package ai

import (
	"context"
	"errors"

	"study-english/server/internal/config"
)

// ImageProvider AI 绘图抽象：输入英文绘图提示词，返回可访问图片 URL
type ImageProvider interface {
	// Generate 生成图片并返回 URL（本地上传 /media/images/ 或外链）；未配置 Key 时返回空+错误
	Generate(ctx context.Context, prompt string) (string, error)
}

// MockImageGen 无 IMAGE_API_KEY 时的占位实现（service 回退模型返回的配图）
type MockImageGen struct{}

var errImageNotConfigured = errors.New("未配置 IMAGE_API_KEY，使用回退配图")

func (m *MockImageGen) Generate(_ context.Context, prompt string) (string, error) {
	return "", errImageNotConfigured
}

// NewImageProvider 创建绘图 Provider：
//   - auto（默认）：配置 IMAGE_API_KEY 用真实 API，否则 mock
//   - openai：强制走 OpenAI 兼容端点（本地适配服务场景，Key 可为空）
//   - mock：强制 mock
func NewImageProvider(cfg *config.Config) ImageProvider {
	switch cfg.ImageProvider {
	case "mock":
		return &MockImageGen{}
	case "openai":
		gen, err := NewOpenAIImageGen(cfg.ImageBaseURL, cfg.ImageAPIKey, cfg.ImageModel, cfg.ImageSize, cfg.DataDir)
		if err != nil {
			return &MockImageGen{}
		}
		return gen
	default: // auto
		if cfg.ImageAPIKey == "" {
			return &MockImageGen{}
		}
		gen, err := NewOpenAIImageGen(cfg.ImageBaseURL, cfg.ImageAPIKey, cfg.ImageModel, cfg.ImageSize, cfg.DataDir)
		if err != nil {
			return &MockImageGen{}
		}
		return gen
	}
}
