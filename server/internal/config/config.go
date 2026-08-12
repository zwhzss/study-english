package config

import (
	"os"
)

// Config 应用配置，全部支持环境变量覆盖（默认值面向本地开发）
type Config struct {
	Port          string // Gin 监听端口
	DBPath        string // SQLite 数据文件路径
	DataDir       string // 静态/素材目录（教材配图、占位音频）
	LLMBaseURL    string // 大模型 API 地址（OpenAI 兼容）
	LLMAPIKey     string // 大模型 API Key
	LLMModel      string // 对话模型名
	TTSType       string // tts 实现类型: mock | openai（自动按 TTS_API_KEY 判断）
	TTSBaseURL    string // OpenAI 兼容 TTS 地址（如 OpenAI / 硅基流动）
	TTSAPIKey     string // TTS API Key
	TTSModel      string // TTS 模型
	TTSVoice      string // 默认音色（空则按角色自动映射）
	ImageBaseURL  string // OpenAI 兼容绘图地址（OpenAI / 硅基流动 / 本地适配服务）
	ImageAPIKey   string // 绘图 API Key（本地服务可空）
	ImageModel    string // 绘图模型
	ImageSize     string // 图片尺寸
	ImageProvider string // 绘图 provider: auto(默认,按 Key 判断) / openai / mock
	MockContent   bool   // 是否使用 mock 内容
	MockExplicit  bool   // mock 是否为显式选择（配置页选 Mock / MOCK_CONTENT=1）——未配置 Key 自动 mock 时为 false
	AllowOrigin   string // CORS 允许来源
	DistDir       string // Angular 构建产物目录（存在则 Gin 托管静态文件，单端口部署）
}

// Load 加载配置：系统配置（端口/路径等）支持环境变量覆盖；
// 模型配置（URL/Key/模型）不读环境变量，仅提供默认占位值——真实值从数据库 ai_configs 表获得
func Load() *Config {
	return &Config{
		Port:        getenv("APP_PORT", "8090"),
		DBPath:      getenv("DB_PATH", "data/study-english.db"),
		DataDir:     getenv("DATA_DIR", "data"),
		AllowOrigin: getenv("ALLOW_ORIGIN", "*"),
		DistDir:     getenv("DIST_DIR", "../frontend/dist/frontend/browser"),
		// 模型配置默认占位（不读环境变量，由数据库 ai_configs 表提供真实值）
		LLMBaseURL:    "https://api.deepseek.com",
		LLMAPIKey:     "",
		LLMModel:      "deepseek-v4-flash",
		TTSBaseURL:    "https://api.siliconflow.cn/v1",
		TTSAPIKey:     "",
		TTSModel:      "FunAudioLLM/CosyVoice2-0.5B",
		TTSVoice:      "",
		ImageBaseURL:  "https://api.siliconflow.cn/v1",
		ImageAPIKey:   "",
		ImageModel:    "Kwai-Kolors/Kolors",
		ImageSize:     "1024x1024",
		ImageProvider: "auto",
		MockContent:   false,
		MockExplicit:  false,
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
