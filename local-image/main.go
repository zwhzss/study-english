// local-image：本地 AI 绘图适配服务
// 把 OpenAI 兼容的 POST /images/generations 请求，转成 llama.cpp 子进程调用，
// 让主系统（AI 英语课堂）能用本地下载的 FLUX GGUF 模型生成场景配图。
//
// 用法（先按 README/start.bat 配好环境变量，再启动）：
//
//	go run .  或  local-image.exe
//
// 主系统侧配置：
//
//	IMAGE_PROVIDER=openai
//	IMAGE_BASE_URL=http://localhost:17000/v1
//	（无需 IMAGE_API_KEY，本地服务不校验）
package main

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	cfg := loadConfig()

	if _, err := exec.LookPath(cfg.llamaCli); err != nil {
		log.Fatalf("找不到 llama-cli：%s（请安装新版 llama.cpp 或设置 LLAMA_CLI 环境变量指向 llama-cli.exe）", cfg.llamaCli)
	}
	for name, p := range map[string]string{"主模型": cfg.fluxModel, "CLIP 编码器": cfg.clipModel, "T5 编码器": cfg.t5Model, "VAE": cfg.vaeModel} {
		if _, err := os.Stat(p); err != nil {
			log.Fatalf("缺少 %s 文件：%s（%v）", name, p, err)
		}
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		log.Fatalf("创建输出目录失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/images/generations", handleGenerate(cfg))
	mux.HandleFunc("/v1/models", handleModels(cfg))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("本地绘图服务启动: http://localhost:%s", cfg.port)
	log.Printf("模型: %s | 步数: %s | 输出目录: %s", cfg.fluxModel, cfg.steps, cfg.outDir)
	if err := http.ListenAndServe(":"+cfg.port, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// config 本地绘图服务配置（全部环境变量可覆盖）
type config struct {
	port      string // 监听端口
	llamaCli  string // llama-cli 可执行文件路径
	fluxModel string // FLUX 主模型 GGUF
	clipModel string // CLIP 文本编码器 GGUF
	t5Model   string // T5 文本编码器 GGUF
	vaeModel  string // VAE GGUF
	steps     string // 采样步数（FLUX schnell 建议 4）
	seed      string // 随机种子（-1 随机）
	extraArgs string // 额外命令行参数（追加到 llama-cli 末尾）
	outDir    string // 生成图片临时输出目录
}

func loadConfig() *config {
	wd, _ := os.Getwd()
	return &config{
		port:      getenv("LOCAL_IMAGE_PORT", "17000"),
		llamaCli:  getenv("LLAMA_CLI", "llama-cli"),
		fluxModel: getenv("FLUX_MODEL", filepath.Join(wd, "models", "flux1-schnell-Q4_K_S.gguf")),
		clipModel: getenv("FLUX_CLIP", filepath.Join(wd, "models", "clip-l.gguf")),
		t5Model:   getenv("FLUX_T5", filepath.Join(wd, "models", "t5xxl-encoder.gguf")),
		vaeModel:  getenv("FLUX_VAE", filepath.Join(wd, "models", "flux-vae.gguf")),
		steps:     getenv("FLUX_STEPS", "4"),
		seed:      getenv("FLUX_SEED", "-1"),
		extraArgs: os.Getenv("FLUX_EXTRA_ARGS"),
		outDir:    getenv("LOCAL_IMAGE_OUT", filepath.Join(wd, "out")),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---- OpenAI 兼容请求/响应 ----

type genRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	ResponseFormat string `json:"response_format"`
}

type genResponse struct {
	Data []genItem `json:"data"`
}

type genItem struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

// handleGenerate 处理 /images/generations：解析请求 → llama.cpp 子进程 → 返回图片
func handleGenerate(cfg *config) http.HandlerFunc {
	var mu sync.Mutex // llama.cpp 单进程串行，避免并发抢显存
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
			return
		}
		var req genRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}
		if strings.TrimSpace(req.Prompt) == "" {
			writeError(w, http.StatusBadRequest, "prompt 不能为空")
			return
		}
		n := req.N
		if n <= 0 {
			n = 1
		}
		if n > 4 {
			n = 4 // 单机显存有限，限制并发张数
		}

		mu.Lock() // 串行生成
		defer mu.Unlock()

		resp := genResponse{Data: make([]genItem, 0, n)}
		for i := 0; i < n; i++ {
			img, err := generateOne(r.Context(), cfg, req.Prompt, req.Size, i)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "图片生成失败: "+err.Error())
				return
			}
			// 主系统（OpenAIImageGen）请求带 response_format=b64_json，按 b64 返回
			resp.Data = append(resp.Data, genItem{B64JSON: img})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// generateOne 调 llama.cpp 生成一张图，返回 base64 PNG
func generateOne(ctx context.Context, cfg *config, prompt, size string, idx int) (string, error) {
	if size != "" && size != "1024x1024" {
		log.Printf("本地 FLUX 固定 1024x1024，忽略请求尺寸 %s", size)
	}
	outFile := filepath.Join(cfg.outDir, "flux-"+hashString(prompt)+".png")
	if _, err := os.Stat(outFile); err == nil {
		return readB64(outFile) // 缓存命中
	}

	args := buildArgs(cfg, prompt, outFile)
	cmd := exec.CommandContext(ctx, cfg.llamaCli, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("llama-cli 输出: %s", strings.TrimSpace(string(output)))
		return "", errors.New("llama-cli 执行失败: " + err.Error())
	}
	return readB64(outFile)
}

// buildArgs 组装 llama-cli 命令行参数（独立函数便于单测）
func buildArgs(cfg *config, prompt, outFile string) []string {
	args := []string{
		"-m", cfg.fluxModel,
		"--clip-model", cfg.clipModel,
		"--t5-model", cfg.t5Model,
		"--vae-model", cfg.vaeModel,
		"-p", prompt,
		"--image-out", outFile,
		"--steps", cfg.steps,
		"--seed", cfg.seed,
	}
	if cfg.extraArgs != "" {
		args = append(args, strings.Fields(cfg.extraArgs)...)
	}
	return args
}

func hashString(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:8])
}

func readB64(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// handleModels OpenAI 兼容模型列表（便于客户端探测）
func handleModels(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "local-flux", "object": "model", "owned_by": "local-image"},
			},
		})
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": msg}})
}
