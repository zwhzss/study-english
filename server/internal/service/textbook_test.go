package service

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/model"
)

// TestMain 测试环境：service 日志静默，避免刷屏
func TestMain(m *testing.M) {
	svcLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

// mockRegistry 全 mock 的 Provider 容器（LLM/TTS/绘图均 mock，显式选择）
func mockRegistry(t *testing.T) *ai.Registry {
	t.Helper()
	return ai.NewRegistry(&config.Config{
		MockContent:   true, // LLM mock
		MockExplicit:  true, // 显式选择 mock（拉取内容不报"未配置"）
		LLMModel:      "mock",
		TTSModel:      "mock",
		ImageProvider: "mock",
		DataDir:       t.TempDir(),
	})
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Textbook{}, &model.CatalogItem{}, &model.AIContent{}, &model.ImportedContent{},
		&model.DialogueSession{}, &model.DialogueMessage{}, &model.AIConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func newTestService(t *testing.T) *TextbookService {
	t.Helper()
	return NewTextbookService(newTestDB(t), mockRegistry(t), "data")
}

// TestCreateTextbookAndCatalog 教材创建 + 内置目录初始化
func TestCreateTextbookAndCatalog(t *testing.T) {
	svc := newTestService(t)
	tb, err := svc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	if err != nil {
		t.Fatalf("创建教材失败: %v", err)
	}
	if tb.ID == 0 {
		t.Fatal("教材 ID 未生成")
	}
	tree, err := svc.GetCatalogTree(tb.ID)
	if err != nil {
		t.Fatalf("获取目录失败: %v", err)
	}
	if len(tree) == 0 {
		t.Fatal("目录树为空，内置模板未生效")
	}
	if len(tree[0].Lessons) == 0 {
		t.Fatal("单元下无课时")
	}
	t.Logf("目录树: %d 个单元, 首个课时 %q (场景: %s)", len(tree), tree[0].Lessons[0].Title, tree[0].Lessons[0].Scene)
}

// TestPullLessonContent 大模型拉取内容（mock）：验证多角色对话 + 图文 + 能力型练习
func TestPullLessonContent(t *testing.T) {
	svc := newTestService(t)
	tb, err := svc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	if err != nil {
		t.Fatalf("创建教材失败: %v", err)
	}
	tree, _ := svc.GetCatalogTree(tb.ID)
	lessonID := tree[0].Lessons[0].ID

	content, err := svc.PullLessonContent(context.Background(), lessonID, false)
	if err != nil {
		t.Fatalf("拉取内容失败: %v", err)
	}
	if len(content.Dialogue) < 3 {
		t.Fatalf("多角色对话轮次过少: %d", len(content.Dialogue))
	}
	roles := map[string]bool{}
	for _, turn := range content.Dialogue {
		roles[turn.Role] = true
	}
	if len(roles) < 2 {
		t.Fatalf("应至少 2 个角色, 实际 %d", len(roles))
	}
	if len(content.KeyVocab) == 0 {
		t.Fatal("词汇为空")
	}
	if len(content.Images) == 0 {
		t.Fatal("配图为空")
	}
	if len(content.RoleVoices) == 0 {
		t.Fatal("角色音色映射为空")
	}
	t.Logf("内容: %d 轮对话(%d 角色), %d 词汇, %d 配图, 场景=%s",
		len(content.Dialogue), len(roles), len(content.KeyVocab), len(content.Images), content.Scene)
}

// TestPullCache 重复拉取命中缓存，不重复调 LLM
func TestPullCache(t *testing.T) {
	svc := newTestService(t)
	tb, _ := svc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	tree, _ := svc.GetCatalogTree(tb.ID)
	lessonID := tree[0].Lessons[0].ID

	c1, err := svc.PullLessonContent(context.Background(), lessonID, false)
	if err != nil {
		t.Fatalf("首次拉取失败: %v", err)
	}
	c2, err := svc.PullLessonContent(context.Background(), lessonID, false)
	if err != nil {
		t.Fatalf("二次拉取失败: %v", err)
	}
	if c1.ID != c2.ID {
		t.Fatalf("缓存失效: 两次拉取产生不同内容 %d vs %d", c1.ID, c2.ID)
	}
	t.Log("重复拉取命中缓存 ✓")
}

// TestPullConcurrent 并发双拉取：唯一索引 + OnConflict 回查，不重复调 LLM、不报冲突
func TestPullConcurrent(t *testing.T) {
	svc := newTestService(t)
	tb, _ := svc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	tree, _ := svc.GetCatalogTree(tb.ID)
	lessonID := tree[0].Lessons[0].ID

	const n = 4
	results := make([]*model.AIContent, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.PullLessonContent(context.Background(), lessonID, false)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("并发拉取 #%d 失败: %v", i, err)
		}
	}
	first := results[0].ID
	for i, r := range results[1:] {
		if r.ID != first {
			t.Fatalf("并发拉取结果不一致: #%d=%d, #0=%d", i+1, r.ID, first)
		}
	}
	t.Logf("并发 %d 路拉取均返回同一内容 id=%d ✓", n, first)
}

// TestGetLessonContent 查询未生成内容应报错
func TestGetLessonContentNotFound(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.GetLessonContent(context.Background(), 999, "ai")
	if err == nil {
		t.Fatal("应返回错误")
	}
	t.Logf("未生成内容正确报错: %v", err)
}
