package main

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
	"study-english/server/internal/service"
)

// ========== 教材 ==========

func (a *App) ListTextbooks() ([]model.Textbook, error) {
	return a.tbSvc.ListTextbooks()
}

func (a *App) CreateTextbook(name, publisher, stage, grade string) (*model.Textbook, error) {
	return a.tbSvc.CreateTextbook(name, publisher, stage, grade, "builtin")
}

func (a *App) DeleteTextbook(id uint) error {
	return a.tbSvc.DeleteTextbook(id)
}

func (a *App) ReorderTextbooks(ids []uint) error {
	return a.tbSvc.ReorderTextbooks(ids)
}

func (a *App) GetCatalog(textbookID uint) ([]service.CatalogNode, error) {
	return a.tbSvc.GetCatalogTree(textbookID)
}

func (a *App) ReorderCatalog(textbookID uint, ids []uint) error {
	return a.tbSvc.ReorderCatalog(textbookID, ids)
}

// StartPullLesson 异步拉取 AI 内容：返回 taskID，完成后自动入库，轮询 GetTask
func (a *App) StartPullLesson(lessonID uint, force bool) (string, error) {
	return a.taskSvc.StartPull(lessonID, func() (*model.AIContent, error) {
		return a.tbSvc.PullLessonContent(context.Background(), lessonID, force)
	}), nil
}

func (a *App) GetTask(taskID string) (*service.Task, error) {
	t, ok := a.taskSvc.Get(taskID)
	if !ok {
		return nil, errors.New("任务不存在")
	}
	return t, nil
}

func (a *App) GetLessonContent(lessonID uint, kind string) (*model.AIContent, error) {
	c, err := a.tbSvc.GetLessonContent(context.Background(), lessonID, kind)
	if err != nil {
		return nil, err
	}
	ac, ok := c.(*model.AIContent)
	if !ok || ac == nil {
		return nil, nil
	}
	return ac, nil
}

func (a *App) GetImportedContent(lessonID uint) (*model.ImportedContent, error) {
	c, err := a.tbSvc.GetLessonContent(context.Background(), lessonID, "imported")
	if err != nil {
		return nil, err
	}
	ic, ok := c.(*model.ImportedContent)
	if !ok || ic == nil {
		return nil, nil
	}
	return ic, nil
}

// ImportReq 导入课文参数（文本 / PDF base64 / 图片 base64 三选一）
type ImportReq struct {
	Title       string `json:"title"`
	Text        string `json:"text"`
	PDFBase64   string `json:"pdf_base64"`
	ImageBase64 string `json:"image_base64"`
}

func (a *App) ImportLesson(lessonID uint, req ImportReq) (*model.ImportedContent, error) {
	switch {
	case req.Text != "":
		return a.tbSvc.ImportLessonText(lessonID, req.Title, req.Text)
	case req.PDFBase64 != "":
		data, err := base64.StdEncoding.DecodeString(req.PDFBase64)
		if err != nil {
			return nil, errors.New("PDF 数据无效")
		}
		return a.tbSvc.ImportLessonPDF(lessonID, req.Title, data)
	case req.ImageBase64 != "":
		data, err := base64.StdEncoding.DecodeString(req.ImageBase64)
		if err != nil {
			return nil, errors.New("图片数据无效")
		}
		return a.tbSvc.ImportLessonImage(lessonID, req.Title, data)
	}
	return nil, errors.New("请提供文本、PDF 或图片")
}

// TextbookMetaData 教材预设（Wails 绑定不支持顶层 map 返回值，必须用结构体）
type TextbookMetaData struct {
	Textbooks []model.TextbookPreset `json:"textbooks"`
	Stages    []string               `json:"stages"`
	Grades    map[string][]string    `json:"grades"`
}

func (a *App) TextbookMeta() (*TextbookMetaData, error) {
	return &TextbookMetaData{
		Textbooks: service.TextbookPresets(),
		Stages:    model.Stages,
		Grades:    model.GradesByStage,
	}, nil
}

// ========== 对话 ==========

func (a *App) CreateDialogueSession(lessonID uint, userRole string) (*model.DialogueSession, error) {
	return a.dgSvc.CreateSession(context.Background(), lessonID, userRole)
}

func (a *App) ListDialogueMessages(sessionID uint) ([]model.DialogueMessage, error) {
	return a.dgSvc.ListMessages(sessionID)
}

// DialogueReply 对话往返结果
type DialogueReply struct {
	User *model.DialogueMessage `json:"user"`
	AI   *model.DialogueMessage `json:"ai"`
}

// SendDialogueMessage 非流式发送
func (a *App) SendDialogueMessage(sessionID uint, text string) (*DialogueReply, error) {
	userMsg, aiMsg, err := a.dgSvc.SendMessage(context.Background(), sessionID, text)
	if err != nil {
		return nil, err
	}
	return &DialogueReply{User: userMsg, AI: aiMsg}, nil
}

// StartStreamDialogue 对话流式入口（前端主动调用，Go 返回绑定结果——符合"请求-响应走绑定、事件只用于广播"架构）
// 回复通过绑定返回值一次性返回（前端模拟打字机）；TTS 音频异步合成后由前端轮询 ListDialogueMessages 获取
func (a *App) StartStreamDialogue(sessionID uint, text string) error {
	a.logf("对话流式开始 session=%d text=%q", sessionID, text)
	go func() {
		t0 := time.Now()
		_, aiMsg, err := a.dgSvc.StreamReply(context.Background(), sessionID, text, nil)
		a.logf("对话流式完成 session=%d 耗时 %.1fs err=%v", sessionID, time.Since(t0).Seconds(), err)
		if err != nil {
			a.logf("对话失败: %v", err)
		}
		_ = aiMsg
	}()
	return nil
}

// Transcribe 语音识别（音频 base64 → 文本）
func (a *App) Transcribe(audioBase64 string, format string) (string, error) {
	a.logf("Transcribe 收到 %d 字节, format=%s", len(audioBase64), format)
	data, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		a.logf("Transcribe base64 解码失败: %v", err)
		return "", errors.New("音频数据无效")
	}
	text, err := a.reg.ASR().Transcribe(context.Background(), data, format)
	if err != nil {
		a.logf("Transcribe 识别失败: %v", err)
		return "", err
	}
	a.logf("Transcribe 成功: %q", text)
	return text, nil
}

// ========== 能力画像 ==========

func (a *App) GetAbilityProfile() (*service.AbilityProfile, error) {
	return a.abSvc.GetProfile()
}

// ========== 模型配置 ==========

func (a *App) GetAIConfig() (*model.AIConfig, error) {
	cfg, _, err := a.cfgSvc.Get()
	return cfg, err
}

func (a *App) SaveAIConfig(cfg *model.AIConfig) error {
	if cfg == nil {
		return errors.New("配置不能为空")
	}
	return a.cfgSvc.Save(cfg)
}

// TestProvider 测试连接（用画面输入的配置，不落库）
func (a *App) TestProvider(service string, ps model.ProviderSetting) (string, error) {
	return a.cfgSvc.Test(context.Background(), service, ps)
}

// ========== 发音评测 ==========

type EvalReq struct {
	Title          string `json:"title"`
	TargetText     string `json:"target_text"`
	RecognizedText string `json:"recognized_text"`
	DurationMs     int    `json:"duration_ms"`
	AudioBase64    string `json:"audio_base64"`
}

func (a *App) EvaluatePronunciation(req EvalReq) (*model.PronunciationRecord, error) {
	return a.pronSvc.Evaluate(req.Title, req.TargetText, req.RecognizedText, req.DurationMs, req.AudioBase64)
}

func (a *App) ListPronunciations() ([]model.PronunciationRecord, error) {
	return a.pronSvc.List()
}

func (a *App) DeletePronunciation(id uint) error {
	return a.pronSvc.Delete(id)
}

// TTSStatusData 当前 TTS 生效通道状态
type TTSStatusData struct {
	Provider      string `json:"provider"` // edge / openai / mock
	Model         string `json:"model"`
	EdgeAvailable bool   `json:"edge_available"` // 本机 edge-tts 是否可用
	VoiceLabel    string `json:"voice_label"`    // 中文说明
}

// TTSStatus 返回当前 TTS 通道状态（配置页显示）
func (a *App) TTSStatus() (*TTSStatusData, error) {
	cfg, _, err := a.cfgSvc.Get()
	if err != nil {
		return nil, err
	}
	edgeAvail := ai.EdgeTTSAvailable()
	st := &TTSStatusData{
		Provider:      cfg.TTS.Provider,
		Model:         cfg.TTS.Model,
		EdgeAvailable: edgeAvail,
	}
	switch cfg.TTS.Provider {
	case "edge":
		label := "Edge TTS 在线（免费多音色，按角色自动选声）"
		if !edgeAvail {
			label += " ⚠️ edge-tts 未安装或版本过旧（需 ≥" + ai.EdgeTTSMinVersion + "），将自动处理"
		}
		st.VoiceLabel = label
	case "mock":
		st.VoiceLabel = "Mock（浏览器朗读兜底）"
	case "openai":
		if cfg.TTS.APIKey != "" {
			st.VoiceLabel = "在线 TTS（" + cfg.TTS.Model + "）"
		} else {
			st.VoiceLabel = "未填 API Key → 浏览器朗读兜底"
		}
	default:
		st.VoiceLabel = "未配置 → 浏览器朗读兜底"
	}
	return st, nil
}

// BackfillAudio 批量补合成历史课文缺失音频
func (a *App) BackfillAudio() (*service.AudioBackfillResult, error) {
	return a.tbSvc.BackfillMissingAudio()
}

// MigrateImages 批量把历史课文外链图片下载到本地
func (a *App) MigrateImages() (*service.ImageMigrateResult, error) {
	return a.tbSvc.MigrateExternalImages()
}

// ========== 目录 CRUD ==========
func (a *App) CreateCatalogItem(textbookID uint, unit, lesson, title, scene string) (*model.CatalogItem, error) {
	return a.tbSvc.CreateCatalogItem(textbookID, unit, lesson, title, scene)
}

func (a *App) UpdateCatalogItem(id uint, unit, lesson, title, scene string) error {
	return a.tbSvc.UpdateCatalogItem(id, unit, lesson, title, scene)
}

func (a *App) DeleteCatalogItem(id uint) error {
	return a.tbSvc.DeleteCatalogItem(id)
}

func (a *App) RenameUnit(textbookID uint, oldUnit, newUnit string) error {
	return a.tbSvc.RenameUnit(textbookID, oldUnit, newUnit)
}

func (a *App) DeleteUnit(textbookID uint, unit string) error {
	return a.tbSvc.DeleteUnit(textbookID, unit)
}

// ========== 媒体文件 ==========
// MediaURL 把 /media/xxx 相对路径转为 file:// 绝对路径（Wails 下前端直接播放本地音频/图片）

func (a *App) MediaURL(rel string) string {
	rel = strings.TrimPrefix(rel, "/media/")
	abs := filepath.Join(a.cfg.DataDir, filepath.FromSlash(rel))
	return "file:///" + filepath.ToSlash(abs)
}
