package ai

import (
	"sync/atomic"

	"study-english/server/internal/config"
)

// Registry 运行时 AI Provider 容器：支持配置保存后动态重建，无需重启服务
type Registry struct {
	llm          atomic.Pointer[LLMProvider]
	tts          atomic.Pointer[TTS]
	img          atomic.Pointer[ImageProvider]
	mockExplicit atomic.Bool // LLM 是否为显式选择 mock（未配置 Key 自动 mock 时为 false）
}

func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{}
	r.Rebuild(cfg)
	return r
}

// Rebuild 用最新配置重建三个 Provider（配置保存后调用）
func (r *Registry) Rebuild(cfg *config.Config) {
	llm := NewLLM(cfg)
	tts := NewTTS(cfg)
	img := NewImageProvider(cfg)
	r.llm.Store(&llm)
	r.tts.Store(&tts)
	r.img.Store(&img)
	r.mockExplicit.Store(cfg.MockExplicit)
}

// MockExplicit LLM 是否为显式 mock 选择（配置页选 Mock / MOCK_CONTENT=1）
func (r *Registry) MockExplicit() bool { return r.mockExplicit.Load() }

func (r *Registry) LLM() LLMProvider { return *r.llm.Load() }
func (r *Registry) TTS() TTS         { return *r.tts.Load() }
func (r *Registry) Image() ImageProvider {
	// 未配置 Image 时返回 mock（防御）
	if p := r.img.Load(); p != nil {
		return *p
	}
	return &MockImageGen{}
}

func (r *Registry) ImageRaw() *ImageProvider { return r.img.Load() }
