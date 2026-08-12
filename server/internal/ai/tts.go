package ai

// MockTTS 占位 TTS：返回空 URL，前端回退浏览器 SpeechSynthesis 朗读
// 接入真实 TTS（Azure/讯飞/阿里云）时替换为真实实现：异步合成 → 对象存储 URL
type MockTTS struct{}

func (m *MockTTS) Synthesize(text, role string) (string, error) {
	return "", nil
}
