package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
)

// PronunciationService 发音评测：录音 → ASR 识别文本 vs 目标文本 → 四维评分 + AI 点评
type PronunciationService struct {
	db       *gorm.DB
	reg      *ai.Registry
	audioDir string
}

func NewPronunciationService(db *gorm.DB, reg *ai.Registry, dataDir string) *PronunciationService {
	dir := filepath.Join(dataDir, "pronunciations")
	_ = os.MkdirAll(dir, 0o755)
	return &PronunciationService{db: db, reg: reg, audioDir: dir}
}

// Evaluate 保存录音并评测：
//   - audioBase64 录音文件（webm/wav）
//   - targetText 目标文本；recognizedText 用户实际说的（前端 ASR）
//   - durationMs 录音时长
func (s *PronunciationService) Evaluate(title, targetText, recognizedText string, durationMs int, audioBase64 string) (*model.PronunciationRecord, error) {
	targetText = strings.TrimSpace(targetText)
	recognizedText = strings.TrimSpace(recognizedText)
	if targetText == "" {
		return nil, fmt.Errorf("目标文本不能为空（请填写要朗读的句子）")
	}
	if recognizedText == "" {
		return nil, fmt.Errorf("未识别到语音内容，请重新录音")
	}

	// 保存音频
	audioURL := ""
	if audioBase64 != "" {
		raw, err := base64.StdEncoding.DecodeString(audioBase64)
		if err != nil {
			return nil, fmt.Errorf("音频数据无效: %w", err)
		}
		name := fmt.Sprintf("pron-%d.webm", time.Now().UnixNano())
		if err := os.WriteFile(filepath.Join(s.audioDir, name), raw, 0o644); err != nil {
			return nil, err
		}
		audioURL = "/media/pronunciations/" + name
	}

	// 四维评分（单词级对比）
	accuracy, completeness := wordMatchScore(targetText, recognizedText)
	fluency := fluencyScore(targetText, durationMs)
	intonation := intonationScore(recognizedText, durationMs)
	overall := (accuracy + fluency + completeness + intonation) / 4

	// AI 点评（识别文本 vs 目标文本，LLM 生成改进建议）
	suggestion := s.aiSuggestion(targetText, recognizedText)

	rec := &model.PronunciationRecord{
		Title:          firstNonEmpty(title, "发音评测"),
		TargetText:     targetText,
		RecognizedText: recognizedText,
		Accuracy:       accuracy,
		Fluency:        fluency,
		Completeness:   completeness,
		Intonation:     intonation,
		Score:          overall,
		Suggestion:     suggestion,
		AudioURL:       audioURL,
		DurationMs:     durationMs,
	}
	if err := s.db.Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

// wordMatchScore 单词级对比：准确度（识别词中正确比例）+ 完整度（目标词被读出的比例）
func wordMatchScore(target, recognized string) (accuracy, completeness int) {
	targetWords := normalizeWords(target)
	recWords := normalizeWords(recognized)
	if len(recWords) == 0 {
		return 0, 0
	}
	// 识别词在目标词中的匹配数（支持部分匹配：识别里出现的目标词）
	matched := 0
	targetSet := map[string]int{}
	for _, w := range targetWords {
		targetSet[w]++
	}
	for _, w := range recWords {
		if targetSet[w] > 0 {
			targetSet[w]--
			matched++
		}
	}
	accuracy = matched * 100 / len(recWords)
	if len(targetWords) > 0 {
		completeness = matched * 100 / len(targetWords)
	}
	return accuracy, completeness
}

// fluencyScore 流利度：语速（目标词数/时长），基准 150 WPM 为 100 分
func fluencyScore(target string, durationMs int) int {
	words := len(normalizeWords(target))
	if words == 0 || durationMs <= 0 {
		return 60
	}
	wpm := float64(words) / (float64(durationMs) / 60000.0)
	score := int(wpm / 150.0 * 100)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// intonationScore 语调（近似）：识别词数接近目标词数 → 节奏稳定分高
func intonationScore(recognized string, durationMs int) int {
	rec := len(normalizeWords(recognized))
	if rec == 0 {
		return 50
	}
	if durationMs > 0 {
		// 语速在 80-220 WPM 区间视为自然
		wpm := float64(rec) / (float64(durationMs) / 60000.0)
		if wpm >= 80 && wpm <= 220 {
			return 90
		}
		if wpm >= 50 && wpm <= 300 {
			return 70
		}
	}
	return 60
}

// aiSuggestion 用 LLM 点评发音问题（识别 vs 目标），无 Key/失败时给规则建议
func (s *PronunciationService) aiSuggestion(target, recognized string) string {
	if s.reg == nil {
		return fallbackSuggestion(target, recognized)
	}
	system := "你是英语发音教练。对比用户朗读的目标文本与识别文本，给出简洁的中文改进建议（2-3 条），指出读错/漏读/多读的词，并给发音技巧。"
	user := fmt.Sprintf("目标文本：%s\n识别文本：%s", target, recognized)
	reply, err := s.reg.LLM().Chat(context.Background(), system, user)
	if err != nil || strings.TrimSpace(reply) == "" {
		return fallbackSuggestion(target, recognized)
	}
	return strings.TrimSpace(reply)
}

// fallbackSuggestion 无 LLM 时的规则建议（对比目标/识别词）
func fallbackSuggestion(target, recognized string) string {
	tw := normalizeWords(target)
	rw := normalizeWords(recognized)
	miss := []string{}
	for _, w := range tw {
		found := false
		for _, r := range rw {
			if r == w {
				found = true
				break
			}
		}
		if !found {
			miss = append(miss, w)
		}
	}
	if len(miss) > 0 {
		return "建议重读以下单词：" + strings.Join(miss, ", ") + "。注意放慢语速、清晰发音。"
	}
	return "读得很好！注意保持自然语速和语调起伏，可尝试跟读原声对比。"
}

// normalizeWords 分词（小写、去标点）
func normalizeWords(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '\'')
	})
	out := fields[:0]
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// List 历史评测记录（最新在前）
func (s *PronunciationService) List() ([]model.PronunciationRecord, error) {
	var list []model.PronunciationRecord
	if err := s.db.Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Delete 删除评测记录（含音频文件）
func (s *PronunciationService) Delete(id uint) error {
	var rec model.PronunciationRecord
	if err := s.db.First(&rec, id).Error; err != nil {
		return fmt.Errorf("记录不存在")
	}
	if rec.AudioURL != "" {
		_ = os.Remove(filepath.Join(s.audioDir, filepath.Base(rec.AudioURL)))
	}
	return s.db.Delete(&rec).Error
}
