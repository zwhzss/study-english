package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
)

// ImportLessonText 导入课文原文（文本粘贴/JSON）→ imported_contents 表（版权合规通道）
func (s *TextbookService) ImportLessonText(catalogItemID uint, title, text string) (*model.ImportedContent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("课文内容不能为空")
	}
	var item model.CatalogItem
	if err := s.db.First(&item, catalogItemID).Error; err != nil {
		return nil, fmt.Errorf("目录项不存在: %w", err)
	}

	content := &model.ImportedContent{
		CatalogItemID: item.ID,
		Unit:          item.Unit,
		Title:         firstNonEmpty(title, item.Title),
		Text:          text,
		Source:        "imported",
		Status:        "published",
	}
	// 已有导入内容则更新，否则创建
	var existing model.ImportedContent
	if err := s.db.Where("catalog_item_id = ?", item.ID).First(&existing).Error; err == nil {
		existing.Title = content.Title
		existing.Text = content.Text
		if err := s.db.Save(&existing).Error; err != nil {
			return nil, err
		}
		// 更新后重新结构化 + 配音
		s.structureAndVoice(&existing)
		return &existing, nil
	}
	if err := s.db.Create(content).Error; err != nil {
		return nil, err
	}
	// 更新目录项状态
	s.db.Model(&item).Update("status", model.StatusPublished)
	// 异步：LLM 结构化课文（识别角色+拆分对话）+ 按角色 TTS 配音
	s.structureAndVoice(content)
	return content, nil
}

// structureAndVoice 把导入的课文原文用 LLM 拆分为多角色对话，并为每句按角色合成 TTS 音频
func (s *TextbookService) structureAndVoice(content *model.ImportedContent) {
	go func() {
		system := `你是英语课文分析专家。下面是导入的英语课文原文。请识别其中的说话角色，把课文拆分为多角色对话。
每个对话轮次：role 用英文角色标识（必须从以下角色库选择：boy/girl/student/teacher/male_teacher/grandpa/grandma/brother/sister/uncle/aunt/mother/father/waiter/customer/clerk/driver/farmer/worker/doctor/nurse/police/friend/narrator），role_name 用中文，text 为该角色说的原句。
若原文是纯叙述、没有明显对话角色，则全部用 narrator（旁白，中文名"旁白"），并尽量按句拆分。
严格输出 JSON，不要任何其他文字：{"dialogue":[{"role":"...","role_name":"...","text":"..."}]}`
		raw, err := s.reg.LLM().ChatJSON(context.Background(), system, content.Text)
		if err != nil {
			s.log.Warn("课文结构化失败", "catalog_item_id", content.CatalogItemID, "error", err.Error())
			return
		}
		var payload struct {
			Dialogue []model.DialogueTurn `json:"dialogue"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil || len(payload.Dialogue) == 0 {
			s.log.Warn("课文结构化解析失败", "catalog_item_id", content.CatalogItemID, "raw", truncateForLog(raw, 200))
			return
		}
		// 每句按角色合成 TTS（edge-tts 多角色音色）
		for i := range payload.Dialogue {
			url, err := s.reg.TTS().Synthesize(payload.Dialogue[i].Text, payload.Dialogue[i].Role)
			if err == nil {
				payload.Dialogue[i].AudioURL = url
			}
		}
		b, _ := json.Marshal(payload.Dialogue)
		s.db.Model(content).Update("dialogue_json", string(b))
		s.log.Info("课文结构化完成", "catalog_item_id", content.CatalogItemID, "rounds", len(payload.Dialogue))
	}()
}

// ImportLessonPDF 导入 PDF 课文：纯 Go 提取文本（免 CGO），无 OCR
func (s *TextbookService) ImportLessonPDF(catalogItemID uint, title string, data []byte) (*model.ImportedContent, error) {
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("PDF 解析失败（可能加密或非文本型扫描件）: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= pdfReader.NumPage(); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err == nil {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}
	extracted := strings.TrimSpace(sb.String())
	if extracted == "" {
		return nil, errors.New("PDF 中未提取到文本（可能是扫描图片版，请使用图片导入或直接粘贴文字）")
	}
	if title == "" {
		title = firstNonEmpty(strings.Split(extracted, "\n")[0], "导入课文")
	}
	return s.ImportLessonText(catalogItemID, title, extracted)
}

// ImportLessonImage 导入图片课文：视觉大模型 OCR 识别 → imported_contents（版权合规通道）
func (s *TextbookService) ImportLessonImage(catalogItemID uint, title string, imgData []byte) (*model.ImportedContent, error) {
	// 从已保存配置读取 OCR 视觉模型设置（未配置则明确提示）
	var cfg model.AIConfig
	if err := s.db.First(&cfg, 1).Error; err != nil || cfg.OCR.Model == "" {
		return nil, errors.New("未配置 OCR 视觉模型，请到「⚙️ 模型配置」→「👁️ OCR 识别」设置（如 Qwen/Qwen3-VL-8B-Instruct 或 deepseek-ai/DeepSeek-OCR）")
	}
	ocr := ai.NewOpenAIVisionOCR(cfg.OCR.BaseURL, cfg.OCR.APIKey, cfg.OCR.Model)
	text, err := ocr.Recognize(context.Background(), ai.B64Encode(imgData))
	if err != nil {
		return nil, fmt.Errorf("OCR 识别失败: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("OCR 未识别到课文文字，请尝试更清晰的图片")
	}
	return s.ImportLessonText(catalogItemID, title, text)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
