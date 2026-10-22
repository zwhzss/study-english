package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// 启动日志：定位"安装后无反应"类问题（GUI 程序崩溃信息不显示，必须落盘）。
	// 必须落在 cfg.DataDir 而非写死 UserConfigDir——否则 DATA_DIR 指定的隔离环境
	// 仍会往生产日志里写，也无法用副本做沙箱复现。
	cfg := config.Load()
	logPath := filepath.Join(cfg.DataDir, "app.log")
	_ = os.MkdirAll(cfg.DataDir, 0o755)
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
	note("配置加载完成 DB=%s", cfg.DBPath)
	// 异步确保 edge-tts 可用（不阻塞启动；已安装则秒返回）
	go ai.EnsureEdgeTTS(func(f string, args ...any) { note(f, args...) })
	db, err := database.Init(cfg.DBPath, cfg.DataDir)
	if err != nil {
		note("数据库初始化失败: %v", err)
		panic("数据库初始化失败: " + err.Error())
	}
	reg := ai.NewRegistry(cfg)
	note("AI Provider 初始构建完成（LLM=%s ASR=%s）", cfg.LLMModel, cfg.ASRModel)
	// 从数据库加载已保存的模型配置并重建 Provider（LLM/TTS/绘图/OCR/ASR 真实 Key 来自表）
	service.LoadConfigIntoRegistry(db, reg)
	note("已从数据库加载模型配置（LLM=%s ASR=%s）", cfg.LLMModel, cfg.ASRModel)
	a.cfg = cfg
	a.reg = reg
	a.tbSvc = service.NewTextbookService(db, reg, cfg.DataDir)
	a.dgSvc = service.NewDialogueService(db, reg)
	a.abSvc = service.NewAbilityService(db)
	a.cfgSvc = service.NewConfigService(db, reg)
	a.taskSvc = service.NewTaskService()
	a.pronSvc = service.NewPronunciationService(db, reg, cfg.DataDir)
	// 初始化种子教材（仅首次）
	service.SeedTextbooks(db)
	note("启动初始化完成")
	// 启动自检：自动验证核心链路（LLM 对话 / TTS 合成 / 数据库），结果写 app.log
	a.runSelfCheck(note)
}

// runSelfCheck 启动自检：真实调用 LLM（deepseek）+ TTS（edge）+ DB 读，排查"桌面版功能异常"
func (a *App) runSelfCheck(note func(f string, args ...any)) {
	go func() {
		ctx := context.Background()
		t0 := time.Now()
		reply, err := a.reg.LLM().Chat(ctx, "你是测试助手，只回复 OK 两个字。", "回复 OK 两个字")
		note("自检 LLM: err=%v 耗时%.1fs 回复=%q", err, time.Since(t0).Seconds(), reply)
		t0 = time.Now()
		// 文本必须每次唯一：固定文本会永久命中缓存，自检就再也测不出 edge-tts 是否真的坏了
		probe := fmt.Sprintf("Edge TTS startup check at %s.", time.Now().Format("15:04:05"))
		url, terr := a.reg.TTS().Synthesize(probe, "teacher")
		switch {
		case terr != nil:
			note("自检 TTS: 失败 耗时%.1fs err=%v", time.Since(t0).Seconds(), terr)
		case url == "":
			note("自检 TTS: 未生效（当前为 Mock，走浏览器朗读）耗时%.1fs", time.Since(t0).Seconds())
		default:
			full := filepath.Join(a.cfg.DataDir, filepath.FromSlash(strings.TrimPrefix(url, "/media/")))
			if st, serr := os.Stat(full); serr != nil || st.Size() < 1024 {
				note("自检 TTS: 产物无效 耗时%.1fs url=%s err=%v", time.Since(t0).Seconds(), url, serr)
			} else {
				note("自检 TTS: OK 耗时%.1fs %d bytes url=%s", time.Since(t0).Seconds(), st.Size(), url)
			}
		}
		var cnt int64
		a.tbSvc.CountTextbooks(&cnt)
		note("自检 DB: 教材 %d 本", cnt)
		note("自检完成")
	}()
}

func (a *App) shutdown(ctx context.Context) {}

// logf 追加写运行日志（cfg.DataDir\app.log，桌面版即 %AppData%\StudyEnglish\app.log），
// 用于排查"无反应/卡住"类问题；与 startup 的启动日志同一文件
func (a *App) logf(format string, args ...any) {
	if a.cfg == nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(a.cfg.DataDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s [app] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}
