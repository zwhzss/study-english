package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"study-english/server/internal/model"
	"study-english/server/internal/service"
)

// AbilityHandler 能力画像 API
type AbilityHandler struct {
	svc *service.AbilityService
}

func NewAbilityHandler(svc *service.AbilityService) *AbilityHandler {
	return &AbilityHandler{svc: svc}
}

// GetProfile GET /api/ability-profile 四维能力画像（雷达图数据源）
func (h *AbilityHandler) GetProfile(c *gin.Context) {
	profile, err := h.svc.GetProfile()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": profile})
}

// TextbookHandler 教材相关 API
type TextbookHandler struct {
	svc     *service.TextbookService
	taskSvc *service.TaskService
}

func NewTextbookHandler(svc *service.TextbookService, taskSvc *service.TaskService) *TextbookHandler {
	return &TextbookHandler{svc: svc, taskSvc: taskSvc}
}

// ImportLesson POST /api/lessons/:id/import 导入课文（版权合规通道）
// 支持两种方式：
//   - JSON: {"title":"...", "text":"课文原文"}（粘贴文本）
//   - multipart/form-data: file=xx.pdf（PDF 解析）+ title 字段
func (h *TextbookHandler) ImportLesson(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "课时 ID 无效"})
		return
	}

	// 文件上传（PDF 或图片）
	if file, header, err := c.Request.FormFile("file"); err == nil {
		defer file.Close()
		if header.Size > 20<<20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件过大（限 20MB）"})
			return
		}
		data := make([]byte, header.Size)
		if _, err := file.Read(data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "读取文件失败: " + err.Error()})
			return
		}
		name := strings.ToLower(header.Filename)
		var content *model.ImportedContent
		switch {
		case strings.HasSuffix(name, ".pdf"):
			content, err = h.svc.ImportLessonPDF(uint(id), c.PostForm("title"), data)
		case strings.HasSuffix(name, ".png"), strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"), strings.HasSuffix(name, ".webp"):
			content, err = h.svc.ImportLessonImage(uint(id), c.PostForm("title"), data)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 PDF / PNG / JPG 图片"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": content})
		return
	}

	// JSON 文本导入
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 text 字段（课文原文）或上传 PDF 文件"})
		return
	}
	content, err := h.svc.ImportLessonText(uint(id), req.Title, req.Text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": content})
}

// MigrateImages POST /api/images/migrate 批量把历史课文外链图片下载到本地
func (h *TextbookHandler) MigrateImages(c *gin.Context) {
	result, err := h.svc.MigrateExternalImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// TranscribeASR POST /api/asr/transcribe 语音识别（multipart: file=音频, format=webm/wav）
func (h *TextbookHandler) TranscribeASR(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传音频文件"})
		return
	}
	fh, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取音频"})
		return
	}
	defer fh.Close()
	data, err := io.ReadAll(fh)
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "音频内容为空"})
		return
	}
	format := c.DefaultPostForm("format", "webm")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	text, err := h.svc.TranscribeASR(ctx, data, format)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "语音识别失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}

func (h *TextbookHandler) TextbookMeta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"textbooks": service.TextbookPresets(), // 种子教材清单（按阶段过滤）
			"stages":    model.Stages,
			"grades":    model.GradesByStage,
		},
	})
}

type createTextbookReq struct {
	Name          string `json:"name" binding:"required"`
	Publisher     string `json:"publisher"`
	Stage         string `json:"stage"`          // 小学/初中/高中/成人
	Grade         string `json:"grade"`          // 如 七年级
	CatalogSource string `json:"catalog_source"` // builtin(默认) / llm
}

// CreateTextbook POST /api/textbooks 创建教材并初始化目录
func (h *TextbookHandler) CreateTextbook(c *gin.Context) {
	var req createTextbookReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	// 输入校验（非法输入返回 400，而非 500）
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教材名称不能为空"})
		return
	}
	if utf8.RuneCountInString(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教材名称过长（最多 100 字）"})
		return
	}
	if req.CatalogSource != "" && req.CatalogSource != "builtin" && req.CatalogSource != "llm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "catalog_source 仅支持 builtin / llm"})
		return
	}
	tb, err := h.svc.CreateTextbook(req.Name, req.Publisher, req.Stage, req.Grade, req.CatalogSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tb})
}

// ListTextbooks GET /api/textbooks
func (h *TextbookHandler) ListTextbooks(c *gin.Context) {
	tbs, err := h.svc.ListTextbooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tbs})
}

// BackfillAudio POST /api/audio/backfill 批量补合成历史课文缺失的音频
func (h *TextbookHandler) BackfillAudio(c *gin.Context) {
	result, err := h.svc.BackfillMissingAudio()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ReorderTextbooks PUT /api/textbooks/order 调整教材库顺序
// CatalogItemCRUD 目录 CRUD 接口

// CreateCatalogItem POST /api/catalog-items 新增课时（unit 新值即新单元）
func (h *TextbookHandler) CreateCatalogItem(c *gin.Context) {
	var req struct {
		TextbookID uint   `json:"textbook_id"`
		Unit       string `json:"unit"`
		Lesson     string `json:"lesson"`
		Title      string `json:"title"`
		Scene      string `json:"scene"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	item, err := h.svc.CreateCatalogItem(req.TextbookID, req.Unit, req.Lesson, req.Title, req.Scene)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// UpdateCatalogItem PUT /api/catalog-items/:id 编辑课时
func (h *TextbookHandler) UpdateCatalogItem(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Unit   string `json:"unit"`
		Lesson string `json:"lesson"`
		Title  string `json:"title"`
		Scene  string `json:"scene"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.UpdateCatalogItem(uint(id), req.Unit, req.Lesson, req.Title, req.Scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "课时已更新"})
}

// DeleteCatalogItem DELETE /api/catalog-items/:id 删除课时
func (h *TextbookHandler) DeleteCatalogItem(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteCatalogItem(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "课时已删除"})
}

// RenameUnit PUT /api/catalog-items/rename-unit 重命名单元
func (h *TextbookHandler) RenameUnit(c *gin.Context) {
	var req struct {
		TextbookID uint   `json:"textbook_id"`
		OldUnit    string `json:"old_unit"`
		NewUnit    string `json:"new_unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := h.svc.RenameUnit(req.TextbookID, req.OldUnit, req.NewUnit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "单元已重命名"})
}

// DeleteUnit DELETE /api/catalog-items/unit 删除单元（连带课时）
func (h *TextbookHandler) DeleteUnit(c *gin.Context) {
	var req struct {
		TextbookID uint   `json:"textbook_id"`
		Unit       string `json:"unit"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.DeleteUnit(req.TextbookID, req.Unit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "单元已删除"})
}

func (h *TextbookHandler) ReorderTextbooks(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 ids 教材顺序列表"})
		return
	}
	if err := h.svc.ReorderTextbooks(req.IDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "教材库顺序已更新"})
}

// DeleteTextbook DELETE /api/textbooks/:id 删除教材（连带目录与内容）
func (h *TextbookHandler) DeleteTextbook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教材 ID 无效"})
		return
	}
	if err := h.svc.DeleteTextbook(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "教材已删除"})
}

// GetCatalog GET /api/textbooks/:id/catalog 目录树（年级→单元→课时）
func (h *TextbookHandler) GetCatalog(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教材 ID 无效"})
		return
	}
	tree, err := h.svc.GetCatalogTree(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}

// PullLessonContent POST /api/lessons/:id/pull 从大模型拉取单元内容
// body: {"force": true} 可选——强制重新生成覆盖旧内容
func (h *TextbookHandler) PullLessonContent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "课时 ID 无效"})
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	_ = c.ShouldBindJSON(&req) // 空 body 允许

	taskID := h.taskSvc.StartPull(uint(id), func() (*model.AIContent, error) {
		// 后台 goroutine 执行：与前端连接解耦，切页面/刷新都不中断
		return h.svc.PullLessonContent(context.Background(), uint(id), req.Force)
	})
	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID, "status": "running", "message": "AI 生成已开始，完成后自动保存（可切到其他页面，稍后回来即可见）"})
}

// GetTask GET /api/tasks/:id 查询异步任务状态与结果
func (h *TextbookHandler) GetTask(c *gin.Context) {
	task, ok := h.taskSvc.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

// ReorderCatalog PUT /api/textbooks/:id/catalog/order 调整目录顺序
// body: {"ids": [课时id新顺序...]}（该教材所有课时）
func (h *TextbookHandler) ReorderCatalog(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教材 ID 无效"})
		return
	}
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 ids 课时顺序列表"})
		return
	}
	if err := h.svc.ReorderCatalog(uint(id), req.IDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "目录顺序已更新"})
}

// GetLessonContent GET /api/lessons/:id/content 查询课时内容
// query: kind=ai(默认) | imported（前端 Tab 切换）
func (h *TextbookHandler) GetLessonContent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "课时 ID 无效"})
		return
	}
	kind := c.DefaultQuery("kind", "ai")
	content, err := h.svc.GetLessonContent(c.Request.Context(), uint(id), kind)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": content})
}
