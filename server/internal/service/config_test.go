package service

import (
	"context"
	"strings"
	"testing"

	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/model"
)

// TestConfigSaveGet 配置保存/读取往返（含 Key 保留）
func TestConfigSaveGet(t *testing.T) {
	db := newTestDB(t)
	cfgSvc := NewConfigService(db, mockRegistry(t))

	// 保存配置
	saved := &model.AIConfig{
		LLM:   model.ProviderSetting{Provider: "openai", BaseURL: "https://api.deepseek.com", APIKey: "sk-llm-secret-123", Model: "deepseek-chat"},
		TTS:   model.ProviderSetting{Provider: "openai", BaseURL: "https://api.siliconflow.cn/v1", APIKey: "sk-tts-secret-456", Model: "CosyVoice", Extra: "nova"},
		Image: model.ProviderSetting{Provider: "auto", BaseURL: "https://api.siliconflow.cn/v1", APIKey: "sk-img-secret-789", Model: "FLUX", Extra: "1024x1024"},
	}
	if err := cfgSvc.Save(saved); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	got, source, err := cfgSvc.Get()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if source != "database" {
		t.Fatalf("保存后来源应为 database, 实际 %s", source)
	}
	if got.LLM.APIKey != "sk-llm-secret-123" || got.TTS.APIKey != "sk-tts-secret-456" || got.Image.APIKey != "sk-img-secret-789" {
		t.Fatalf("Key 往返不一致: %+v", got)
	}
	if got.Image.Extra != "1024x1024" {
		t.Fatalf("Extra 字段丢失: %+v", got.Image)
	}
	t.Logf("配置保存/读取 ✓: llm=%s tts=%s image=%s", got.LLM.Model, got.TTS.Model, got.Image.Model)
}

// TestConfigTest 测试连接（mock 模式应返回明确提示而非报错）
func TestConfigTest(t *testing.T) {
	db := newTestDB(t)
	cfgSvc := NewConfigService(db, mockRegistry(t))

	// LLM 未配置 Key → 明确提示
	_, err := cfgSvc.Test(context.Background(), "llm", model.ProviderSetting{Provider: "openai", BaseURL: "https://api.deepseek.com", APIKey: "", Model: "deepseek-v4-flash"})
	if err == nil {
		t.Fatal("llm 无 Key 应提示未配置")
	}
	if !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("应提示配置 Key, 实际: %v", err)
	}
	// LLM mock 选择 → 提示
	if _, err = cfgSvc.Test(context.Background(), "llm", model.ProviderSetting{Provider: "mock"}); err == nil {
		t.Fatal("llm mock 应提示")
	}
	// 非法类型
	if _, err = cfgSvc.Test(context.Background(), "xxx", model.ProviderSetting{}); err == nil {
		t.Fatal("非法类型应报错")
	}
	t.Logf("测试连接 ✓: llm 无Key→%v, mock→%v", err, err)
}

// TestPullWithoutConfig 未配置大模型（非显式 mock）拉取内容应给出提示
func TestPullWithoutConfig(t *testing.T) {
	db := newTestDB(t)
	// 未显式选择 mock（LLM 自动 mock）
	reg := ai.NewRegistry(&config.Config{MockContent: true, MockExplicit: false, DataDir: t.TempDir()})
	svc := NewTextbookService(db, reg)

	tb, err := svc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	if err != nil {
		t.Fatalf("创建教材失败: %v", err)
	}
	tree, _ := svc.GetCatalogTree(tb.ID)
	lessonID := tree[0].Lessons[0].ID

	_, err = svc.PullLessonContent(ctxTest(), lessonID, false)
	if err == nil {
		t.Fatal("未配置大模型时应提示错误")
	}
	if !contains(err.Error(), "模型配置") {
		t.Fatalf("错误信息应引导到模型配置, 实际: %v", err)
	}
	t.Logf("未配置拉取提示 ✓: %v", err)
}

// TestMaskKey 脱敏逻辑
func TestMaskKey(t *testing.T) {
	if got := MaskKey("sk-abcdefgh1234"); got != "sk-abc***1234" {
		t.Fatalf("脱敏异常: %s", got)
	}
	if got := MaskKey("short"); got != "short" {
		t.Fatalf("短 Key 应原样: %s", got)
	}
	if !IsMasked("sk-abc***1234") {
		t.Fatal("脱敏 Key 应被识别")
	}
	if IsMasked("sk-real-key-1234") {
		t.Fatal("真实 Key 不应被识别为脱敏")
	}
	t.Logf("脱敏 ✓: %s", MaskKey("sk-abcdefgh1234"))
}
