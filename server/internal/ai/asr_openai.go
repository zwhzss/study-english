package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// OpenAIASR OpenAI 兼容的语音识别（audio/transcriptions）
// 适用：硅基流动（iic/SenseVoiceSmall）等
type OpenAIASR struct {
	baseURL string
	apiKey  string
	model   string
}

func NewOpenAIASR(baseURL, apiKey, model string) *OpenAIASR {
	return &OpenAIASR{baseURL: strings.TrimSuffix(baseURL, "/"), apiKey: apiKey, model: model}
}

func (a *OpenAIASR) Transcribe(ctx context.Context, audio []byte, format string) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("未配置语音识别 API Key（ASR），请到模型配置页设置")
	}
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("file", "audio."+format)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	_ = w.WriteField("model", a.model)
	_ = w.WriteField("language", "en")
	_ = w.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ASR 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	// {"text": "..."}（OpenAI 兼容）；部分厂商返回 {text: [{...}]}
	var r struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Text), nil
}
