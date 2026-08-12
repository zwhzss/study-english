package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OCRProvider 图片文字识别抽象：输入图片 base64，返回识别文本
type OCRProvider interface {
	// Recognize 识别图片中的文字（英文课文场景）
	Recognize(ctx context.Context, imageBase64 string) (string, error)
}

// OpenAIVisionOCR 基于 OpenAI 兼容视觉模型（chat/completions + image_url）
// 硅基流动 Qwen2.5-VL 等；识别质量高、零本地依赖
type OpenAIVisionOCR struct {
	baseURL string
	apiKey  string
	model   string
}

func NewOpenAIVisionOCR(baseURL, apiKey, model string) *OpenAIVisionOCR {
	return &OpenAIVisionOCR{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

const ocrSystemPrompt = "你是专业的 OCR 文字识别引擎。请识别图片中的英文课文内容，原样输出文字，保留段落和标点，不要翻译、不要添加任何解释、不要输出 markdown 代码块。"

// Recognize 视觉模型识别图片文字（data URI 传图）
func (o *OpenAIVisionOCR) Recognize(ctx context.Context, imageBase64 string) (string, error) {
	reqBody := map[string]any{
		"model": o.model,
		"messages": []map[string]any{
			{"role": "system", "content": ocrSystemPrompt},
			{"role": "user", "content": []map[string]any{
				{"type": "text", "text": "请识别这张图片中的英文课文。"},
				{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + imageBase64}},
			}},
		},
		"temperature": 0.1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := string(b)
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "20012") {
			return "", fmt.Errorf("OCR 模型不存在或已下架（%s），请到「⚙️ 模型配置」换用 Qwen/Qwen3-VL 或 deepseek-ai/DeepSeek-OCR，或到硅基流动模型广场确认模型 ID", o.model)
		}
		return "", fmt.Errorf("OCR 返回 %d: %s", resp.StatusCode, msg)
	}

	var cr struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&cr); err != nil {
		return "", fmt.Errorf("解析 OCR 响应失败: %w", err)
	}
	if len(cr.Choices) == 0 || cr.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("OCR 未识别到内容")
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}

// MockOCR 无视觉模型配置时的占位（返回明确错误）
type MockOCR struct{}

func (m *MockOCR) Recognize(_ context.Context, imageBase64 string) (string, error) {
	return "", fmt.Errorf("未配置 OCR 视觉模型，请到「模型配置」页面设置（如 Qwen/Qwen2.5-VL）")
}

// B64Encode 图片字节 → base64（供 OCR 使用）
func B64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
