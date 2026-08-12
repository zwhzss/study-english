package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"study-english/server/internal/service"
)

// DialogueHandler AI 实景对话 API
type DialogueHandler struct {
	svc *service.DialogueService
}

func NewDialogueHandler(svc *service.DialogueService) *DialogueHandler {
	return &DialogueHandler{svc: svc}
}

type createSessionReq struct {
	LessonID uint   `json:"lesson_id"`
	UserRole string `json:"user_role"` // 学生扮演的角色（空则默认第一个）
}

type sendMessageReq struct {
	Text string `json:"text" binding:"required"`
}

// CreateSession POST /api/dialogue/sessions 创建对话会话
func (h *DialogueHandler) CreateSession(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	session, err := h.svc.CreateSession(ctx, req.LessonID, req.UserRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": session})
}

// ListMessages GET /api/dialogue/sessions/:id/messages 历史消息
func (h *DialogueHandler) ListMessages(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 无效"})
		return
	}
	msgs, err := h.svc.ListMessages(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": msgs})
}

// StreamMessage POST /api/dialogue/sessions/:id/messages/stream 流式发送（SSE）
// 事件：delta(增量文本) → done(完整 user/ai 消息) / error
func (h *DialogueHandler) StreamMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 无效"})
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息不能为空"})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息不能为空"})
		return
	}

	// SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "当前环境不支持流式输出"})
		return
	}

	writeEvent := func(event string, payload any) error {
		b, _ := json.Marshal(payload)
		if _, err := c.Writer.WriteString("event: " + event + "\n"); err != nil {
			return err // 客户端断开，中断流
		}
		if _, err := c.Writer.WriteString("data: " + string(b) + "\n\n"); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	userMsg, aiMsg, err := h.svc.StreamReply(ctx, uint(id), req.Text, func(delta string) error {
		return writeEvent("delta", gin.H{"delta": delta})
	})
	if err != nil {
		if writeErr := writeEvent("error", gin.H{"message": err.Error()}); writeErr != nil {
			return // 客户端已断开
		}
		return
	}
	_ = writeEvent("done", gin.H{"user": userMsg, "ai": aiMsg})
}
func (h *DialogueHandler) SendMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 无效"})
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息不能为空"})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "消息不能为空"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	userMsg, aiMsg, err := h.svc.SendMessage(ctx, uint(id), req.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": map[string]any{"user": userMsg, "ai": aiMsg}})
}
