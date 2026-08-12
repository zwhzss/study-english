package ai

import "context"

// ASR 语音识别接口（音频 → 文本）：硅基流动 SenseVoice 等 OpenAI 兼容 audio/transcriptions
type ASR interface {
	// Transcribe 识别音频文本（audio 为录音字节，format 为 "webm"/"wav"/"mp3"）
	Transcribe(ctx context.Context, audio []byte, format string) (string, error)
}

// mockASR 无 ASR 配置时的占位实现（返回固定提示文本，便于 UI 演示）
type mockASR struct{}

func (m *mockASR) Transcribe(ctx context.Context, audio []byte, format string) (string, error) {
	return "hello, could I see the menu please", nil
}
