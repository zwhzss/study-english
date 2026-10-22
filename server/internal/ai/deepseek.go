package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"study-english/server/internal/config"
)

// DeepSeekLLM DeepSeek（OpenAI 兼容协议）实现
type DeepSeekLLM struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int // 单次最大输出 token；0 时取 config.DefaultLLMMaxTokens
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Thinking    *struct {
		Type string `json:"type"`
	} `json:"thinking,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	ResponseFormat  *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
		// FinishReason 区分"模型说完了"(stop) 与"被 max_tokens 截断"(length)——
		// 不读它就只能把截断误判成"模型吐了坏 JSON"，排查方向完全跑偏
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// effectiveMaxTokens 取生效的输出上限（未配置时回落默认值）
func (d *DeepSeekLLM) effectiveMaxTokens() int {
	if d.maxTokens > 0 {
		return d.maxTokens
	}
	return config.DefaultLLMMaxTokens
}

func (d *DeepSeekLLM) ChatJSON(ctx context.Context, system, user string) (string, error) {
	// 不使用 response_format=json_object（DeepSeek v4 兼容性差，会 400 拒绝）：
	// 与 study-math 方案一致——靠 prompt 要求 JSON 输出，解析时容错 markdown 包裹
	reqBody := chatRequest{
		Model:       d.model,
		Temperature: 0.6,
		MaxTokens:   d.effectiveMaxTokens(),
		Messages: []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	d.applyModelTuning(&reqBody)
	return d.doChat(ctx, reqBody)
}

// applyModelTuning 按模型差异化参数：
//   - deepseek-v4-pro：no thinking 模式（直接输出，更快更省；用户要求关闭 thinking）
//   - 其他（flash 等）：直接输出
func (d *DeepSeekLLM) applyModelTuning(req *chatRequest) {
	if d.model == "deepseek-v4-pro" {
		// 显式关闭 thinking：不发送 thinking 字段 + reasoning_effort
		req.Thinking = nil
		req.ReasoningEffort = ""
	}
}

// Chat 普通文本对话（不加 JSON 约束，温度略高更自然）
func (d *DeepSeekLLM) Chat(ctx context.Context, system, user string) (string, error) {
	reqBody := chatRequest{
		Model:       d.model,
		Temperature: 0.9,
		Messages: []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	return d.doChat(ctx, reqBody)
}

// ChatStream 流式对话：解析 OpenAI 兼容 SSE，逐段回调增量
func (d *DeepSeekLLM) ChatStream(ctx context.Context, system, user string, onDelta StreamDelta) error {
	reqBody := chatRequest{
		Model:       d.model,
		Temperature: 0.9,
		Stream:      true,
		Messages: []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("流式调用大模型失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("大模型返回 %d: %s", resp.StatusCode, string(b))
	}

	// 解析 SSE 行：data: {...} 或 data: [DONE]
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 忽略无法解析的行
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := onDelta(chunk.Choices[0].Delta.Content); err != nil {
				return err // 客户端中断
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流失败: %w", err)
	}
	return nil
}

func (d *DeepSeekLLM) doChat(ctx context.Context, reqBody chatRequest) (string, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用大模型失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("大模型返回 %d: %s", resp.StatusCode, string(b))
	}

	// 限制响应体大小（2MB+1 用于区分"响应过大"与正常解析错误）
	const maxRespSize = 2 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRespSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxRespSize {
		return "", fmt.Errorf("大模型响应超过 2MB 限制")
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("大模型返回空 choices")
	}
	ch := cr.Choices[0]
	if ch.FinishReason == "length" {
		return "", fmt.Errorf("模型输出被 max_tokens=%d 截断（已生成 %d token，finish_reason=length）：调大 LLM_MAX_TOKENS 或换上下文更长的模型",
			d.effectiveMaxTokens(), cr.Usage.CompletionTokens)
	}
	return ch.Message.Content, nil
}

// Test 验证 API Key 连通性（普通 chat 测试，与 study-math 一致，无需 json_object）
func (d *DeepSeekLLM) Test(ctx context.Context) (string, error) {
	reply, err := d.ChatJSON(ctx,
		"你是连接测试助手。请只回复：pong",
		"ping",
	)
	if err != nil {
		return "", fmt.Errorf("连接失败: %w", err)
	}
	return fmt.Sprintf("连接成功（模型 %s，模型回复：%s）", d.model, reply), nil
}
