package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"study-english/server/internal/service"
)

// PronunciationHandler 发音评测 API
type PronunciationHandler struct {
	svc *service.PronunciationService
}

func NewPronunciationHandler(svc *service.PronunciationService) *PronunciationHandler {
	return &PronunciationHandler{svc: svc}
}

// Evaluate POST /api/pronunciations 提交录音评测
// body: {title, target_text, recognized_text, duration_ms, audio_base64}
func (h *PronunciationHandler) Evaluate(c *gin.Context) {
	var req struct {
		Title          string `json:"title"`
		TargetText     string `json:"target_text"`
		RecognizedText string `json:"recognized_text"`
		DurationMs     int    `json:"duration_ms"`
		AudioBase64    string `json:"audio_base64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	rec, err := h.svc.Evaluate(req.Title, req.TargetText, req.RecognizedText, req.DurationMs, req.AudioBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rec})
}

// List GET /api/pronunciations 历史评测记录
func (h *PronunciationHandler) List(c *gin.Context) {
	list, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// Delete DELETE /api/pronunciations/:id
func (h *PronunciationHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记录 ID 无效"})
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已删除"})
}
