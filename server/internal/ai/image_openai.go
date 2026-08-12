package ai

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
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

// OpenAIImageGen 基于 OpenAI 兼容 /images/generations 的 AI 绘图
// 兼容 OpenAI (gpt-image-1/dall-e-3) / 硅基流动 (FLUX.1-schnell) 等
// 结果优先 b64_json 存本地 data/images/{hash}.png，返回 /media/images/ URL
type OpenAIImageGen struct {
	baseURL  string
	apiKey   string
	model    string
	size     string
	imageDir string
}

type imageGenRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size,omitempty"`       // OpenAI 兼容（dall-e-3/gpt-image-1）
	ImageSize      string `json:"image_size,omitempty"` // 硅基流动参数名（size 会被忽略/报错）
	ResponseFormat string `json:"response_format,omitempty"`
}

type imageGenResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

// downloadTo 下载图片到本地 path（限制 8MB），成功返回 true
func (g *OpenAIImageGen) downloadTo(ctx context.Context, src, path string) bool {
	if !strings.HasPrefix(src, "http") {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || len(data) < 100 {
		return false
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return false
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return false
	}
	return true
}

func NewOpenAIImageGen(baseURL, apiKey, model, size, dataDir string) (*OpenAIImageGen, error) {
	imageDir := filepath.Join(dataDir, "images")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建图片目录失败: %w", err)
	}
	if model == "" {
		model = "black-forest-labs/FLUX.1-schnell"
	}
	if size == "" {
		size = "1024x1024"
	}
	return &OpenAIImageGen{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey, model: model, size: size, imageDir: imageDir,
	}, nil
}

// Generate 生成图片：缓存命中直接返回；否则调绘图 API 并本地存储
func (g *OpenAIImageGen) Generate(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("绘图提示词为空")
	}

	hash := sha1.Sum([]byte(g.model + "|" + g.size + "|" + prompt))
	name := hex.EncodeToString(hash[:10]) + ".png"
	path := filepath.Join(g.imageDir, name)
	url := "/media/images/" + name
	if _, err := os.Stat(path); err == nil {
		return url, nil // 缓存命中
	}

	body, _ := json.Marshal(imageGenRequest{
		Model: g.model, Prompt: prompt, N: 1, Size: g.size, ImageSize: g.size,
		// 注意：不带 response_format=b64_json —— 硅基流动不支持该参数会 400 报错；
		// 响应里的 url（临时外链）由 service 层 persistImage 下载本地持久化
	})

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey) // 本地适配服务不校验 Key
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("绘图 API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("绘图 API 返回 %d: %s", resp.StatusCode, string(b))
	}

	var gr imageGenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 20<<20)).Decode(&gr); err != nil {
		return "", fmt.Errorf("解析绘图响应失败: %w", err)
	}
	if len(gr.Data) == 0 {
		return "", fmt.Errorf("绘图 API 返回空 data")
	}
	item := gr.Data[0]

	// 优先 b64_json 存本地（临时文件 + rename 原子落盘，防并发半写）；
	// url（硅基流动返回的临时外链）同样下载落盘，保证缓存命中 + 本地持久化（s3 临时地址会过期）
	if item.B64JSON != "" {
		raw, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return "", fmt.Errorf("解码图片失败: %w", err)
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o644); err != nil {
			return "", err
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			return "", err
		}
		return url, nil
	}
	if item.URL != "" {
		// 下载外链到本地（失败则原样返回外链，由 service 层 persistImage 兜底）
		if g.downloadTo(ctx, item.URL, path) {
			return url, nil
		}
		return item.URL, nil
	}
	return "", fmt.Errorf("绘图 API 未返回图片数据")
}
