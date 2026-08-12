package ai

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenAITTSTTS 基于 OpenAI 兼容 /audio/speech 接口的真实 TTS
// 兼容 OpenAI / 硅基流动(SiliconFlow) / MiniMax 等提供同一协议的服务商
// 合成结果缓存到本地 data/audio/{hash}.mp3，重复文本不重复调用
type OpenAITTSTTS struct {
	baseURL  string
	apiKey   string
	model    string
	defVoice string
	audioDir string
	platform string // openai / siliconflow（决定音色名风格）
}

type ttsRequest struct {
	Model string `json:"model"`
	Voice string `json:"voice"`
	Input string `json:"input"`
}

// 角色 → 音色映射（OpenAI voices：alloy/echo/fable/onyx/nova/shimmer）
var roleVoices = map[string]string{
	"waiter":   "onyx",    // 男声-成熟
	"customer": "nova",    // 女声-年轻
	"clerk":    "echo",    // 男声-稳重
	"driver":   "onyx",    // 男声-中年
	"teacher":  "shimmer", // 女声-温柔
	"student":  "alloy",   // 童声
	"doctor":   "nova",    // 女声-专业
	"friend":   "fable",   // 女声-活泼
	"vocab":    "nova",    // 词汇朗读
	"default":  "nova",
}

// 角色 → 音色映射（硅基流动 CosyVoice 系统预置音色，需带模型名前缀，如 FunAudioLLM/CosyVoice2-0.5B:alex）
// 男声：alex沉稳 / benjamin低沉 / charles磁性 / david欢快；女声：anna沉稳 / bella激情 / claire温柔 / diana欢快
var roleVoicesSiliconFlow = map[string]string{
	"waiter":   "alex",     // 沉稳男声
	"customer": "claire",   // 温柔女声
	"clerk":    "charles",  // 磁性男声
	"driver":   "benjamin", // 低沉男声
	"teacher":  "anna",     // 沉稳女声
	"student":  "diana",    // 欢快女声
	"doctor":   "anna",     // 沉稳女声
	"friend":   "diana",    // 欢快女声
	"vocab":    "claire",   // 温柔女声
	"default":  "claire",
}

func NewOpenAITTSTTS(baseURL, apiKey, model, defVoice, dataDir string) (*OpenAITTSTTS, error) {
	audioDir := filepath.Join(dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建音频目录失败: %w", err)
	}
	platform := "openai"
	if strings.Contains(baseURL, "siliconflow") || strings.Contains(baseURL, "bigmodel") {
		platform = "siliconflow"
	}
	return &OpenAITTSTTS{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		defVoice: defVoice,
		audioDir: audioDir,
		platform: platform,
	}, nil
}

func (t *OpenAITTSTTS) voiceForRole(role string) string {
	if t.defVoice != "" {
		// 用户自定义音色：若是硅基流动且未带模型名前缀，自动补全（如 alex → 模型:alex）
		if t.platform == "siliconflow" && !strings.Contains(t.defVoice, ":") {
			return t.model + ":" + t.defVoice
		}
		return t.defVoice
	}
	if t.platform == "siliconflow" {
		// 硅基流动要求 voice 带模型名前缀：模型:音色
		return t.model + ":" + roleVoicesSiliconFlow[role]
	}
	if v, ok := roleVoices[role]; ok {
		return v
	}
	return roleVoices["default"]
}

// Synthesize 合成文本音频，返回可访问的 URL（缓存命中直接返回）
func (t *OpenAITTSTTS) Synthesize(text, role string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	voice := t.voiceForRole(role)
	hash := sha1.Sum([]byte(voice + "|" + text))
	name := hex.EncodeToString(hash[:8]) + ".mp3"
	path := filepath.Join(t.audioDir, name)
	url := "/media/audio/" + name

	// 缓存命中
	if _, err := os.Stat(path); err == nil {
		return url, nil
	}

	body, _ := json.Marshal(ttsRequest{Model: t.model, Voice: voice, Input: text})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("TTS 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("TTS 返回 %d: %s", resp.StatusCode, string(b))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB 上限
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return url, nil
}
