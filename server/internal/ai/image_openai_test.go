package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenAIImageGen 用 httptest 模拟 OpenAI 兼容绘图 API：
// 验证 b64_json → 本地存储 → /media/images URL → 缓存命中
func TestOpenAIImageGen(t *testing.T) {
	pngBytes := []byte("fake-png-content-" + strings.Repeat("0123456789", 15)) // 150 字节，满足下载下限
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/img/") {
			// 图片下载端点（Generate 把外链下载到本地）
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
			return
		}
		calls++
		if r.URL.Path != "/images/generations" {
			t.Errorf("路径应为 /images/generations, 实际 %s", r.URL.Path)
		}
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["image_size"] != "1024x1024" {
			t.Errorf("应请求 image_size=1024x1024（硅基流动参数名）, 实际 %v", reqBody["image_size"])
		}
		if v, ok := reqBody["response_format"]; ok && v != nil && v != "" {
			t.Errorf("不应请求 response_format（硅基流动会 400）, 实际 %v", v)
		}
		w.Header().Set("Content-Type", "application/json")
		// 模拟硅基流动：返回指向 mock 服务器的可下载外链（本地持久化链路）
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "http://" + r.Host + "/img/" + fmt.Sprint(calls) + ".png"}},
		})
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	gen, err := NewOpenAIImageGen(srv.URL, "test-key", "test-model", "1024x1024", dataDir)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 第一次：调用 API，下载外链落盘 → 返回本地 /media/images/
	url1, err := gen.Generate(context.Background(), "a restaurant scene")
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if !strings.HasPrefix(url1, "/media/images/") {
		t.Fatalf("应返回本地 /media/images/ URL, 实际: %s", url1)
	}
	name := filepath.Base(url1)
	saved, err := os.ReadFile(filepath.Join(dataDir, "images", name))
	if err != nil {
		t.Fatalf("图片未落盘: %v", err)
	}
	if string(saved) != string(pngBytes) {
		t.Fatal("落盘内容与下载不一致")
	}

	// 第二次：缓存命中，不再调 API（同一 prompt → 同一 hash）
	url2, err := gen.Generate(context.Background(), "a restaurant scene")
	if err != nil {
		t.Fatalf("缓存生成失败: %v", err)
	}
	if url1 != url2 {
		t.Fatalf("缓存 URL 不一致: %s vs %s", url1, url2)
	}
	if calls != 1 {
		t.Fatalf("缓存未生效: 应只调 1 次 API, 实际 %d", calls)
	}
	t.Logf("绘图链路 ✓: API 1 次调用, 落盘 %s, 缓存命中 URL=%s", name, url2)
}

// TestOpenAIImageGenURLFallback API 只返回 url 时直接使用外链
func TestOpenAIImageGenURLFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.com/img.png"}},
		})
	}))
	defer srv.Close()

	gen, err := NewOpenAIImageGen(srv.URL, "test-key", "m", "512x512", t.TempDir())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	url, err := gen.Generate(context.Background(), "scene")
	if err != nil || url != "https://cdn.example.com/img.png" {
		t.Fatalf("url 回退失败: %v %s", err, url)
	}
	t.Logf("url 外链回退 ✓: %s", url)
}

// TestOpenAIImageGenError API 错误时返回可读错误
func TestOpenAIImageGenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"quota exceeded"}`, http.StatusPaymentRequired)
	}))
	defer srv.Close()

	gen, _ := NewOpenAIImageGen(srv.URL, "bad-key", "m", "512x512", t.TempDir())
	_, err := gen.Generate(context.Background(), "scene")
	if err == nil {
		t.Fatal("应返回错误")
	}
	t.Logf("API 错误可读 ✓: %v", err)
}
