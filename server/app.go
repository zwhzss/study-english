package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/database"
	"study-english/server/internal/service"
)

// App Wails 桌面应用：内嵌 Go 后端服务，方法绑定给前端调用
type App struct {
	ctx     context.Context
	cfg     *config.Config
	reg     *ai.Registry
	tbSvc   *service.TextbookService
	dgSvc   *service.DialogueService
	abSvc   *service.AbilityService
	cfgSvc  *service.ConfigService
	taskSvc *service.TaskService
	pronSvc *service.PronunciationService
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 启动日志：定位"安装后无反应"类问题（GUI 程序崩溃信息不显示，必须落盘）
	logDir, _ := os.UserConfigDir()
	logPath := filepath.Join(logDir, "StudyEnglish", "app.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	note := func(f string, args ...any) {
		if logf != nil {
			fmt.Fprintf(logf, "%s [startup] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(f, args...))
		}
	}
	defer func() {
		if r := recover(); r != nil {
			note("PANIC: %v", r)
			panic(r) // 保留原始行为，日志已记录
		}
	}()
	note("启动初始化开始")
	// 启动后端服务（与 cmd/server/main.go 一致的初始化链路）
	cfg := config.Load()
	note("配置加载完成 DB=%s", cfg.DBPath)
	db, err := database.Init(cfg.DBPath, cfg.DataDir)
	if err != nil {
		note("数据库初始化失败: %v", err)
		panic("数据库初始化失败: " + err.Error())
	}
	reg := ai.NewRegistry(cfg)
	note("AI Provider 构建完成（LLM=%s ASR=%s）", cfg.LLMModel, cfg.ASRModel)
	a.cfg = cfg
	a.reg = reg
	a.tbSvc = service.NewTextbookService(db, reg)
	a.dgSvc = service.NewDialogueService(db, reg)
	a.abSvc = service.NewAbilityService(db)
	a.cfgSvc = service.NewConfigService(db, reg)
	a.taskSvc = service.NewTaskService()
	a.pronSvc = service.NewPronunciationService(db, reg, cfg.DataDir)
	// 初始化种子教材（仅首次）
	service.SeedTextbooks(db)
}

func (a *App) shutdown(ctx context.Context) {}
