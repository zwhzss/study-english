package main

import (
	"log"

	"gorm.io/gorm"
	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/database"

	"study-english/server/internal/handler"
	"study-english/server/internal/model"
	"study-english/server/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := database.Init(cfg.DBPath, cfg.DataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 种子数据初始化：常用教材真实目录（幂等，已存在的教材不覆盖）
	service.SeedTextbooks(db)
	// 老数据迁移：sort_order=0 的历史教材回填为 id（按创建顺序排）
	db.Model(&model.Textbook{}).Where("sort_order = 0").Update("sort_order", gorm.Expr("id"))

	// 动态 Provider 容器：支持模型配置页保存后热重建（无需重启）
	reg := ai.NewRegistry(cfg)
	// 若数据库已有保存的模型配置，优先使用（覆盖环境变量）
	service.LoadConfigIntoRegistry(db, reg)

	svc := service.NewTextbookService(db, reg)
	dsvc := service.NewDialogueService(db, reg)
	abSvc := service.NewAbilityService(db)
	cfgSvc := service.NewConfigService(db, reg)
	taskSvc := service.NewTaskService()
	pronSvc := service.NewPronunciationService(db, reg, cfg.DataDir)

	r := handler.NewRouter(cfg, svc, dsvc, abSvc, cfgSvc, taskSvc, pronSvc, reg)

	log.Printf("LLM 模式: %s（可在页面「模型配置」修改）", llmMode(reg))
	log.Printf("服务启动: http://localhost:%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func llmMode(reg *ai.Registry) string {
	if _, ok := reg.LLM().(*ai.MockLLM); ok {
		return "mock（内置示例内容）"
	}
	return "真实模型"
}
