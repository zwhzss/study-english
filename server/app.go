package main

import (
	"context"

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
	// 启动后端服务（与 cmd/server/main.go 一致的初始化链路）
	cfg := config.Load()
	db, err := database.Init(cfg.DBPath)
	if err != nil {
		panic("数据库初始化失败: " + err.Error())
	}
	reg := ai.NewRegistry(cfg)
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
