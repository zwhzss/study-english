package main

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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

func (a *App) TextbookMeta() (map[string]any, error) {
	return map[string]any{
		"textbooks": service.TextbookPresets(),
		"stages":    model.Stages,
		"grades":    model.GradesByStage,
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

// StartStreamDialogue 流式发送：逐字通过 Wails 事件 ai:delta 推送，结束推送 ai:done
func (a *App) StartStreamDialogue(sessionID uint, text string) error {
	go func() {
		_, aiMsg, err := a.dgSvc.StreamReply(context.Background(), sessionID, text, func(delta string) error {
			runtime.EventsEmit(a.ctx, "ai:delta", map[string]any{"session_id": sessionID, "delta": delta})
			return nil
		})
		if err != nil {
			runtime.EventsEmit(a.ctx, "ai:done", map[string]any{"session_id": sessionID, "error": err.Error()})
			return
		}
		runtime.EventsEmit(a.ctx, "ai:done", map[string]any{"session_id": sessionID, "message": aiMsg})
	}()
	return nil
}

// Transcribe 语音识别（音频 base64 → 文本）
func (a *App) Transcribe(audioBase64 string, format string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return "", errors.New("音频数据无效")
	}
	return a.reg.ASR().Transcribe(context.Background(), data, format)
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

// ========== 媒体文件 ==========
// MediaURL 把 /media/xxx 相对路径转为 file:// 绝对路径（Wails 下前端直接播放本地音频/图片）

func (a *App) MediaURL(rel string) string {
	rel = strings.TrimPrefix(rel, "/media/")
	abs := filepath.Join(a.cfg.DataDir, filepath.FromSlash(rel))
	return "file:///" + filepath.ToSlash(abs)
}
