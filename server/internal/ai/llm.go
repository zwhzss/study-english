package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"study-english/server/internal/config"
)

// ChatMessage 对话消息（OpenAI 兼容）
type ChatMessage struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// LLMProvider 大模型 Provider 抽象：业务层只依赖此接口，可插拔换厂商
type LLMProvider interface {
	// ChatJSON 要求模型返回 JSON 文本（response_format=json_object 强约束）
	ChatJSON(ctx context.Context, system, user string) (string, error)
	// Chat 普通文本对话（用于 AI 语伴/纠错等自然语言场景）
	Chat(ctx context.Context, system, user string) (string, error)
	// ChatStream 流式文本对话（SSE），逐段回调增量
	ChatStream(ctx context.Context, system, user string, onDelta StreamDelta) error
}

// StreamDelta 流式增量回调；返回 error 可中断流
type StreamDelta func(delta string) error

// Tester 连接测试能力（可选实现，用于验证 API Key 是否有效）
type Tester interface {
	// Test 发送最小请求验证连通性，返回描述信息
	Test(ctx context.Context) (string, error)
}

// TTS 文本转语音抽象：mock 实现返回空 URL，前端回退浏览器 SpeechSynthesis
type TTS interface {
	// Synthesize 返回音频 URL（真实服务接入后返回对象存储/网关 URL）
	Synthesize(text, role string) (string, error)
}

// NewLLM 按配置创建 LLM Provider：
//   - 显式 Mock（MockContent）或未配置 Key（非显式 Mock）→ MockLLM（未配置时拉取会提示）
//   - 配置了 Key → 真实模型
func NewLLM(cfg *config.Config) LLMProvider {
	if cfg.MockContent || (cfg.LLMAPIKey == "" && !cfg.MockExplicit) {
		return &MockLLM{}
	}
	return &DeepSeekLLM{
		baseURL:   cfg.LLMBaseURL,
		apiKey:    cfg.LLMAPIKey,
		model:     cfg.LLMModel,
		maxTokens: cfg.LLMMaxTokens,
	}
}

// NewTTS 创建 TTS 实现：
//   - edge：微软 Edge 大声朗读（在线未公开接口，免费多音色，无需 Key，需本机 edge-tts>=7.3.0）
//   - openai（硅基流动等 OpenAI 兼容）：配置 TTS_API_KEY 用真实 TTS
//   - mock（无 Key）：返回空 URL，前端回退浏览器 SpeechSynthesis
func NewTTS(cfg *config.Config) TTS {
	switch cfg.TTSType {
	case "edge":
		return NewEdgeTTS(cfg.DataDir)
	case "mock":
		return &MockTTS{}
	}
	if cfg.TTSAPIKey == "" {
		return &MockTTS{}
	}
	tts, err := NewOpenAITTSTTS(cfg.TTSBaseURL, cfg.TTSAPIKey, cfg.TTSModel, cfg.TTSVoice, cfg.DataDir)
	if err != nil {
		// 目录创建失败时回退 mock，保证服务可用
		return &MockTTS{}
	}
	return tts
}

// ParseJSONPayload 解析 LLM 输出为任意结构（容忍 markdown 代码块包裹）
func ParseJSONPayload[T any](raw string, out *T) error {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return fmt.Errorf("解析模型 JSON 输出失败: %w\n%s", err, jsonErrorContext(s, err))
	}
	return nil
}

// jsonErrorContext 输出解析出错位置前后的片段。
// 原先固定打印前 500 字节，出错点常在窗口之外（本次就是），等于没给任何线索。
func jsonErrorContext(s string, err error) string {
	const span = 120
	off := 0
	switch e := err.(type) {
	case *json.SyntaxError:
		off = int(e.Offset)
	case *json.UnmarshalTypeError:
		off = int(e.Offset)
	}
	if off <= 0 || off > len(s) {
		return fmt.Sprintf("输出共 %d 字节（服务端未给位置），开头：%s", len(s), clipRunes(s, span))
	}
	for off > 0 && !utf8.RuneStart(s[off]) { // 对齐到字符边界，避免切出半个汉字
		off--
	}
	lo, hi := max(0, off-span), min(len(s), off+span)
	for lo > 0 && !utf8.RuneStart(s[lo]) {
		lo++
	}
	for hi < len(s) && !utf8.RuneStart(s[hi]) {
		hi--
	}
	return fmt.Sprintf("输出共 %d 字节，出错位置约第 %d 字节：\n  %s⟪出错点⟫%s",
		len(s), off, s[lo:off], s[off:hi])
}

// clipRunes 按字节截断并对齐字符边界
func clipRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	end := n
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "…"
}
