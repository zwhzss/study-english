package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenAIImageGen 用 httptest 模拟 OpenAI 兼容绘图 API：
// 验证 b64_json → 本地存储 → /media/images URL → 缓存命中
func TestOpenAIImageGen(t *testing.T) {
	pngBytes := []byte("fake-png-content-0123456789")
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/images/generations" {
			t.Errorf("路径应为 /images/generations, 实际 %s", r.URL.Path)
		}
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["response_format"] != "b64_json" {
			t.Errorf("应请求 b64_json, 实际 %v", reqBody["response_format"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngBytes)}},
		})
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	gen, err := NewOpenAIImageGen(srv.URL, "test-key", "test-model", "1024x1024", dataDir)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 第一次：调用 API 并落盘
	url1, err := gen.Generate(context.Background(), "a restaurant scene")
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if url1 == "" || url1[:len("/media/images/")] != "/media/images/" {
		t.Fatalf("URL 异常: %s", url1)
	}
	name := filepath.Base(url1)
	saved, err := os.ReadFile(filepath.Join(dataDir, "images", name))
	if err != nil {
		t.Fatalf("图片未落盘: %v", err)
	}
	if string(saved) != string(pngBytes) {
		t.Fatal("落盘内容与 API 返回不一致")
	}

	// 第二次：缓存命中，不再调 API
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
