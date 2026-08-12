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
)

// DeepSeekLLM DeepSeek（OpenAI 兼容协议）实现
type DeepSeekLLM struct {
	baseURL string
	apiKey  string
	model   string
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

// maxOutputTokens 内容生成的最大输出 token（多角色对话+词汇+语法+练习+SVG 量大，防止截断）
const maxOutputTokens = 65535

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

func (d *DeepSeekLLM) ChatJSON(ctx context.Context, system, user string) (string, error) {
	// 不使用 response_format=json_object（DeepSeek v4 兼容性差，会 400 拒绝）：
	// 与 study-math 方案一致——靠 prompt 要求 JSON 输出，解析时容错 markdown 包裹
	reqBody := chatRequest{
		Model:       d.model,
		Temperature: 0.6,
		MaxTokens:   maxOutputTokens,
		Messages: []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	d.applyModelTuning(&reqBody)
	return d.doChat(ctx, reqBody)
}

// applyModelTuning 按模型差异化参数（照搬 study-math）：
//   - deepseek-v4-pro：启用 thinking 模式（质量优先）
//   - 其他（flash 等）：直接输出（速度优先）
func (d *DeepSeekLLM) applyModelTuning(req *chatRequest) {
	if d.model == "deepseek-v4-pro" {
		req.Thinking = &struct {
			Type string `json:"type"`
		}{Type: "enabled"}
		req.ReasoningEffort = "high"
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
	return cr.Choices[0].Message.Content, nil
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
