package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/service"
)

// NewRouter 注册全部路由
func NewRouter(cfg *config.Config, svc *service.TextbookService, dsvc *service.DialogueService, abSvc *service.AbilityService, cfgSvc *service.ConfigService, taskSvc *service.TaskService, pronSvc *service.PronunciationService, reg *ai.Registry) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.AllowOrigin},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * 3600,
	}))

	h := NewTextbookHandler(svc, taskSvc)
	dh := NewDialogueHandler(dsvc)
	ah := NewAbilityHandler(abSvc)
	ch := NewConfigHandler(cfgSvc)
	ph := NewPronunciationHandler(pronSvc)

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 大模型连接测试：验证 LLM_API_KEY 是否可用（mock 模式亦返回模式说明）
	r.GET("/api/ai/test", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		start := time.Now()
		llm := reg.LLM()
		mode := "mock"
		if _, ok := llm.(*ai.DeepSeekLLM); ok {
			mode = "deepseek"
		}
		if t, ok := llm.(ai.Tester); ok {
			msg, err := t.Test(ctx)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"ok": false, "mode": mode, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "mode": mode, "message": msg, "latency_ms": time.Since(start).Milliseconds()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "mode": mode, "message": "provider 不支持测试"})
	})

	api := r.Group("/api")
	{
		api.POST("/textbooks", h.CreateTextbook)                  // 创建教材 + 初始化目录
		api.PUT("/textbooks/order", h.ReorderTextbooks)           // 调整教材库顺序
		api.GET("/textbooks", h.ListTextbooks)                    // 教材列表
		api.DELETE("/textbooks/:id", h.DeleteTextbook)            // 删除教材（连带内容）
		api.GET("/textbooks/meta", h.TextbookMeta)                // 常用教材预设（下拉列表）
		api.GET("/textbooks/:id/catalog", h.GetCatalog)           // 目录树
		api.PUT("/textbooks/:id/catalog/order", h.ReorderCatalog) // 调整目录顺序（拖拽保存）
		api.POST("/lessons/:id/pull", h.PullLessonContent)        // 异步拉取课文（返回 task_id）
		api.GET("/lessons/:id/content", h.GetLessonContent)       // 查询课时内容
		api.POST("/lessons/:id/import", h.ImportLesson)           // 导入课文（PDF/文本，版权合规）
		api.GET("/tasks/:id", h.GetTask)                          // 异步任务状态

		// AI 实景对话
		api.POST("/dialogue/sessions", dh.CreateSession)                     // 创建会话（选角色）
		api.GET("/dialogue/sessions/:id/messages", dh.ListMessages)          // 历史消息
		api.POST("/dialogue/sessions/:id/messages", dh.SendMessage)          // 发送消息
		api.POST("/dialogue/sessions/:id/messages/stream", dh.StreamMessage) // 流式发送（SSE）

		// 能力画像（雷达图数据）
		api.GET("/ability-profile", ah.GetProfile)

		// 模型配置（页面可视化 + 测试连接 + 持久化）
		api.GET("/config/ai", ch.GetConfig)
		api.POST("/config/ai", ch.SaveConfig)
		api.POST("/config/ai/test", ch.TestConfig)

		// 发音评测
		api.POST("/pronunciations", ph.Evaluate)
		api.GET("/pronunciations", ph.List)
		api.DELETE("/pronunciations/:id", ph.Delete)

		// 语音识别（对话语音输入 / 发音评测）
		api.POST("/asr/transcribe", h.TranscribeASR)
	}

	// 若 Angular 构建产物存在，由 Gin 托管静态文件（单端口部署 + SPA fallback）
	serveStatic(r, cfg.DistDir)
	// 托管 TTS 合成音频（data/audio/）：存在返回文件，不存在 404（避免回退 HTML）
	audioDir := filepath.Clean(filepath.Join(cfg.DataDir, "audio"))
	serveDir(r, "/media/audio/", audioDir)
	// 托管 AI 绘图生成图片（data/images/）
	serveDir(r, "/media/images/", filepath.Clean(filepath.Join(cfg.DataDir, "images")))
	// 托管发音评测录音（data/pronunciations/）
	serveDir(r, "/media/pronunciations/", filepath.Clean(filepath.Join(cfg.DataDir, "pronunciations")))

	return r
}

// serveDir 安全托管目录内文件（前缀检查含分隔符边界，防 ../ 穿越）
func serveDir(r *gin.Engine, route, dir string) {
	prefix := dir + string(filepath.Separator)
	r.GET(route+"*filepath", func(c *gin.Context) {
		p := filepath.Clean(filepath.Join(dir, filepath.FromSlash(c.Param("filepath"))))
		if strings.HasPrefix(p, prefix) { // 边界检查：拒绝 dir 的兄弟目录（如 images-extra）
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				c.File(p)
				return
			}
		}
		c.Status(http.StatusNotFound)
	})
}

// serveStatic 托管前端 dist 目录：真实文件直接返回，SPA 路由回退 index.html
func serveStatic(r *gin.Engine, dist string) {
	info, err := os.Stat(dist)
	if err != nil || !info.IsDir() {
		return
	}
	// 统一为本机路径风格（Windows 反斜杠），保证前缀匹配正确
	distRoot := filepath.Clean(dist)
	r.Static("/assets", distRoot+string(filepath.Separator)+"assets")
	r.StaticFile("/favicon.ico", distRoot+string(filepath.Separator)+"favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// 先尝试命中真实静态文件（main-*.js / styles-*.css / polyfills-*.js），
		// 避免这些资源请求被错误回退成 index.html（HTML）导致前端白屏
		p := filepath.Clean(filepath.Join(distRoot, filepath.FromSlash(c.Request.URL.Path)))
		if strings.HasPrefix(p, distRoot) {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				c.File(p)
				return
			}
		}
		// SPA 路由（/textbooks 等）回退到入口页
		c.File(filepath.Join(distRoot, "index.html"))
	})
}
