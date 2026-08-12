package ai

import (
	"testing"

	"study-english/server/internal/config"
)

// TestNewImageProviderModes 验证 IMAGE_PROVIDER 三种模式：
// openai（无 Key 也走本地端点）/ auto（有 Key 才真实）/ mock（强制）
func TestNewImageProviderModes(t *testing.T) {
	base := &config.Config{
		DataDir:      t.TempDir(),
		ImageBaseURL: "http://localhost:17000/v1",
		ImageModel:   "local-flux",
		ImageSize:    "1024x1024",
	}

	// openai：无 Key 也必须返回真实实现（本地适配服务场景）
	cfg := *base
	cfg.ImageProvider = "openai"
	if _, ok := NewImageProvider(&cfg).(*OpenAIImageGen); !ok {
		t.Fatal("IMAGE_PROVIDER=openai 应返回 OpenAIImageGen（本地服务无 Key）")
	}

	// auto + 无 Key → mock
	cfg2 := *base
	cfg2.ImageProvider = "auto"
	if _, ok := NewImageProvider(&cfg2).(*MockImageGen); !ok {
		t.Fatal("auto 模式无 Key 应返回 MockImageGen")
	}

	// auto + 有 Key → openai
	cfg3 := *base
	cfg3.ImageProvider = "auto"
	cfg3.ImageAPIKey = "sk-test"
	if _, ok := NewImageProvider(&cfg3).(*OpenAIImageGen); !ok {
		t.Fatal("auto 模式有 Key 应返回 OpenAIImageGen")
	}

	// mock：强制
	cfg4 := *base
	cfg4.ImageProvider = "mock"
	cfg4.ImageAPIKey = "sk-test"
	if _, ok := NewImageProvider(&cfg4).(*MockImageGen); !ok {
		t.Fatal("mock 模式即使有 Key 也应返回 MockImageGen")
	}
	t.Log("IMAGE_PROVIDER 三种模式 ✓")
}
