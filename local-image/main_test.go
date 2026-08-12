package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig() *config {
	return &config{
		port:      "17000",
		llamaCli:  "llama-cli",
		fluxModel: "models/flux1-schnell-Q4_K_S.gguf",
		clipModel: "models/clip-l.gguf",
		t5Model:   "models/t5xxl-encoder.gguf",
		vaeModel:  "models/flux-vae.gguf",
		steps:     "4",
		seed:      "-1",
		extraArgs: "--threads 4",
		outDir:    filepath.Join(os.TempDir(), "local-image-test"),
	}
}

// TestBuildArgs 参数组装：模型/编码器/VAE/提示词/输出/步数/额外参数
func TestBuildArgs(t *testing.T) {
	cfg := testConfig()
	args := buildArgs(cfg, "a restaurant scene", "out/test.png")

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-m models/flux1-schnell-Q4_K_S.gguf",
		"--clip-model models/clip-l.gguf",
		"--t5-model models/t5xxl-encoder.gguf",
		"--vae-model models/flux-vae.gguf",
		"-p a restaurant scene",
		"--image-out out/test.png",
		"--steps 4",
		"--seed -1",
		"--threads 4",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("参数缺少 %q\n实际: %s", want, joined)
		}
	}
	t.Logf("参数组装 ✓: %s", joined)
}

// TestHandleGenerate 请求解析：合法请求 → 调用子进程（用编译的假 llama-cli 验证链路）
func TestHandleGenerate(t *testing.T) {
	// 假 llama-cli：解析 os.Args，把 -p 后的提示词写入 PROMPT_LOG，并生成假 PNG
	script := `package main
import ("os"; "strings")
func main() {
	args := os.Args[1:]
	for i, a := range args {
		if a == "--image-out" && i+1 < len(args) {
			os.WriteFile(args[i+1], []byte("FAKE-PNG"), 0o644)
		}
		if a == "-p" && i+1 < len(args) {
			os.WriteFile(os.Getenv("PROMPT_LOG"), []byte(strings.Join(args[i+1:], " ")), 0o644)
		}
	}
}`
	tmp := t.TempDir()
	scriptFile := filepath.Join(tmp, "fakecli.go")
	exe := filepath.Join(tmp, "fakecli.exe")
	if err := os.WriteFile(scriptFile, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("go", "build", "-o", exe, scriptFile).Run(); err != nil {
		t.Fatalf("编译假 llama-cli 失败: %v", err)
	}

	cfg := testConfig()
	cfg.outDir = tmp
	cfg.llamaCli = exe
	cfg.extraArgs = ""
	t.Setenv("PROMPT_LOG", filepath.Join(tmp, "prompt.log"))

	h := handleGenerate(cfg)
	body, _ := json.Marshal(map[string]any{"prompt": "a restaurant scene", "n": 1})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d: %s", rec.Code, rec.Body.String())
	}
	var resp genResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].B64JSON == "" {
		t.Fatalf("应返回 1 张 b64 图, 实际 %d 张", len(resp.Data))
	}
	// 验证 b64 解码 == FAKE-PNG
	raw, _ := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if string(raw) != "FAKE-PNG" {
		t.Fatalf("b64 内容异常: %s", raw)
	}
	// 验证提示词确实传给子进程
	if b, err := os.ReadFile(filepath.Join(tmp, "prompt.log")); err != nil || !strings.Contains(string(b), "a restaurant scene") {
		t.Fatalf("提示词未传给子进程: %v %s", err, b)
	}
	t.Logf("请求→子进程→b64 响应 全链路 ✓")
}

// TestHandleGenerateErrors 错误处理：空 prompt → 400
func TestHandleGenerateErrors(t *testing.T) {
	h := handleGenerate(testConfig())
	body, _ := json.Marshal(map[string]any{"prompt": ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空 prompt 应 400, 实际 %d", rec.Code)
	}
	// 非 POST → 405
	req2 := httptest.NewRequest(http.MethodGet, "/v1/images/generations", nil)
	rec2 := httptest.NewRecorder()
	h(rec2, req2)
	if rec2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 应 405, 实际 %d", rec2.Code)
	}
	t.Logf("错误处理 ✓: 空 prompt→400, 非 POST→405")
}

// TestHandleModels 模型列表端点
func TestHandleModels(t *testing.T) {
	h := handleModels(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if !strings.Contains(rec.Body.String(), "local-flux") {
		t.Fatalf("模型列表异常: %s", rec.Body.String())
	}
	t.Logf("模型列表 ✓: %s", rec.Body.String())
}

func base64Std(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
