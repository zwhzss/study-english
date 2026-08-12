package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"study-english/server/internal/model"
	"study-english/server/internal/service"
)

// ConfigHandler 模型配置 API（页面可视化配置 + 测试连接 + 持久化）
type ConfigHandler struct {
	svc *service.ConfigService
}

func NewConfigHandler(svc *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// GetConfig GET /api/config/ai 返回当前配置（Key 脱敏）
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	cfg, source, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cfg.LLM.APIKey = service.MaskKey(cfg.LLM.APIKey)
	cfg.TTS.APIKey = service.MaskKey(cfg.TTS.APIKey)
	cfg.Image.APIKey = service.MaskKey(cfg.Image.APIKey)
	cfg.OCR.APIKey = service.MaskKey(cfg.OCR.APIKey)
	c.JSON(http.StatusOK, gin.H{"data": cfg, "source": source})
}

// SaveConfig POST /api/config/ai 保存配置并热重建 Provider
func (h *ConfigHandler) SaveConfig(c *gin.Context) {
	var req model.AIConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	// 未修改的脱敏 Key 保留原值
	cur, _, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if service.IsMasked(req.LLM.APIKey) {
		req.LLM.APIKey = cur.LLM.APIKey
	}
	if service.IsMasked(req.TTS.APIKey) {
		req.TTS.APIKey = cur.TTS.APIKey
	}
	if service.IsMasked(req.Image.APIKey) {
		req.Image.APIKey = cur.Image.APIKey
	}
	if service.IsMasked(req.OCR.APIKey) {
		req.OCR.APIKey = cur.OCR.APIKey
	}
	if err := h.svc.Save(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "配置已保存并生效（无需重启）"})
}

// TestConfig POST /api/config/ai/test 测试指定服务连接（使用画面当前输入的值）
// body: {"type":"llm"|"tts"|"image", "config": {provider, base_url, api_key, model, extra}}
func (h *ConfigHandler) TestConfig(c *gin.Context) {
	var req struct {
		Type   string                `json:"type" binding:"required"`
		Config model.ProviderSetting `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 不能为空（llm / tts / image）"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second) // 连通超时：30 秒无响应视为连接失败
	defer cancel()
	// 页面传入脱敏 key（未修改）时，取已保存的真实 key 用于测试
	if service.IsMasked(req.Config.APIKey) {
		cur, _, err := h.svc.Get()
		if err == nil {
			switch req.Type {
			case "llm":
				req.Config.APIKey = cur.LLM.APIKey
			case "tts":
				req.Config.APIKey = cur.TTS.APIKey
			case "image":
				req.Config.APIKey = cur.Image.APIKey
			}
		}
	}
	msg, err := h.svc.Test(ctx, req.Type, req.Config)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "连接超时（30 秒无响应），请检查 Base URL / API Key / 网络"
		}
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "type": req.Type, "message": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "type": req.Type, "message": msg})
}
