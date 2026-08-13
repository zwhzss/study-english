package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"study-english/server/internal/ai"
	"study-english/server/internal/config"
	"study-english/server/internal/model"
)

// ConfigService 模型配置管理：读取/保存/测试连接，保存后动态重建 Provider
type ConfigService struct {
	db  *gorm.DB
	reg *ai.Registry
}

func NewConfigService(db *gorm.DB, reg *ai.Registry) *ConfigService {
	return &ConfigService{db: db, reg: reg}
}

const aiConfigRowID = 1

// Get 当前配置：数据库优先，无则返回环境变量默认值；source 标记配置来源
func (s *ConfigService) Get() (*model.AIConfig, string, error) {
	var c model.AIConfig
	if err := s.db.First(&c, aiConfigRowID).Error; err == nil {
		// 旧配置可能缺 OCR 字段，补默认（不落库，保存时才持久化）
		if c.OCR.Model == "" {
			c.OCR = model.ProviderSetting{
				Provider: "openai", BaseURL: config.Load().ImageBaseURL, Model: "Qwen/Qwen3-VL-8B-Instruct",
			}
		}
		return &c, "database", nil
	}
	// 默认配置（无数据库配置时：默认 URL/模型 + 空 Key）
	env := config.Load()
	return &model.AIConfig{
		ID: aiConfigRowID,
		LLM: model.ProviderSetting{
			Provider: "openai", BaseURL: env.LLMBaseURL, APIKey: "", Model: env.LLMModel,
		},
		TTS: model.ProviderSetting{
			Provider: "openai", BaseURL: env.TTSBaseURL, APIKey: "", Model: env.TTSModel, Extra: env.TTSVoice,
		},
		Image: model.ProviderSetting{
			Provider: "auto", BaseURL: env.ImageBaseURL, APIKey: "", Model: env.ImageModel, Extra: env.ImageSize,
		},
		OCR: model.ProviderSetting{
			Provider: "openai", BaseURL: env.ImageBaseURL, APIKey: "", Model: "Qwen/Qwen3-VL-8B-Instruct",
		},
		ASR: model.ProviderSetting{
			Provider: "openai", BaseURL: env.ASRBaseURL, APIKey: "", Model: env.ASRModel,
		},
	}, "default", nil
}

// Save 保存配置并立即重建 Provider（无需重启）
func (s *ConfigService) Save(cfg *model.AIConfig) error {
	if cfg == nil {
		return errors.New("配置不能为空")
	}
	cfg.ID = aiConfigRowID
	if err := s.db.Save(cfg).Error; err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	// 重建 Provider
	env := cfgToConfig(cfg)
	s.reg.Rebuild(env)
	return nil
}

// Test 测试指定服务连接（llm / tts / image）
// 使用画面传入的 ProviderSetting 构建临时 Provider 测试，与已保存/环境变量配置无关
func (s *ConfigService) Test(ctx context.Context, service string, ps model.ProviderSetting) (string, error) {
	// 日志：记录测试参数（Key 脱敏）与结果，便于排查连接问题
	logParams := slog.Group("test",
		"service", service,
		"provider", ps.Provider,
		"base_url", ps.BaseURL,
		"model", ps.Model,
		"api_key", MaskKey(ps.APIKey),
	)
	env := config.Load() // 仅作 BaseURL 兜底
	run := func(fn func() (string, error)) (string, error) {
		msg, err := fn()
		if err != nil {
			slog.Error("模型测试连接失败", "error", err.Error(), logParams)
			return "", err
		}
		slog.Info("模型测试连接成功", "message", msg, logParams)
		return msg, nil
	}
	switch service {
	case "llm":
		if ps.Provider == "mock" || ps.APIKey == "" {
			return "", errors.New("未配置大模型 API Key（或选择了 Mock），无法测试连接")
		}
		tmp := &config.Config{
			LLMBaseURL: or(ps.BaseURL, env.LLMBaseURL),
			LLMAPIKey:  ps.APIKey,
			LLMModel:   or(ps.Model, env.LLMModel),
		}
		return run(func() (string, error) {
			t, ok := ai.NewLLM(tmp).(ai.Tester)
			if !ok {
				return "", errors.New("当前大模型 Provider 不支持连接测试")
			}
			return t.Test(ctx)
		})
	case "tts":
		if ps.Provider == "mock" || ps.APIKey == "" {
			return "", errors.New("未配置 TTS API Key（或选择了 Mock），使用浏览器朗读")
		}
		return run(func() (string, error) {
			tts, err := ai.NewOpenAITTSTTS(or(ps.BaseURL, env.TTSBaseURL), ps.APIKey, or(ps.Model, env.TTSModel), ps.Extra, env.DataDir)
			if err != nil {
				return "", err
			}
			url, err := tts.Synthesize("Hello! This is a test of the voice service.", "customer")
			if err != nil {
				return "", err
			}
			return "语音合成成功: " + url, nil
		})
	case "image":
		if ps.Provider == "mock" || (ps.Provider != "openai" && ps.APIKey == "") {
			return "", errors.New("未配置绘图 API Key（或选择了 Mock），无法测试连接")
		}
		return run(func() (string, error) {
			tmp := &config.Config{
				ImageBaseURL:  or(ps.BaseURL, env.ImageBaseURL),
				ImageAPIKey:   ps.APIKey,
				ImageModel:    or(ps.Model, env.ImageModel),
				ImageSize:     or(ps.Extra, env.ImageSize),
				ImageProvider: "openai",
				DataDir:       env.DataDir, // 测试图落盘到数据目录（否则 CWD/images）
			}
			url, err := ai.NewImageProvider(tmp).Generate(ctx, "a simple colorful test scene with a house and a tree")
			if err != nil {
				return "", err
			}
			return "绘图成功: " + url, nil
		})
	case "ocr":
		if ps.Provider == "mock" || ps.APIKey == "" {
			return "", errors.New("未配置 OCR 视觉模型 API Key（或选择了 Mock），无法测试连接")
		}
		return run(func() (string, error) {
			ocr := ai.NewOpenAIVisionOCR(or(ps.BaseURL, env.ImageBaseURL), ps.APIKey, or(ps.Model, "Qwen/Qwen3-VL-8B-Instruct"))
			// 生成 64x64 测试图验证连接（Qwen3-VL 要求图片 ≥28px）
			b64, err := makeTestPNG(64, 64)
			if err != nil {
				return "", err
			}
			if _, err := ocr.Recognize(ctx, b64); err != nil {
				return "", err
			}
			return "OCR 连接成功（视觉模型已响应）", nil
		})
	case "asr":
		if ps.Provider == "mock" || ps.APIKey == "" {
			return "", errors.New("未配置语音识别 API Key（或选择了 Mock），无法测试连接")
		}
		return run(func() (string, error) {
			asr := ai.NewOpenAIASR(or(ps.BaseURL, env.ASRBaseURL), ps.APIKey, or(ps.Model, "iic/SenseVoiceSmall"))
			// 用 1 秒静音 WAV 验证连接（SenseVoice 接受任意音频）
			wav := makeTestWAV()
			if _, err := asr.Transcribe(ctx, wav, "wav"); err != nil {
				return "", err
			}
			return "语音识别连接成功", nil
		})
	default:
		return "", fmt.Errorf("未知服务类型: %s（支持 llm / tts / image / ocr / asr）", service)
	}
}

// makeTestWAV 生成 0.1 秒静音 WAV（16bit 单声道 8kHz），用于 ASR 连接测试
func makeTestWAV() []byte {
	var buf bytes.Buffer
	rate := 8000
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+rate/10*2))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // mono
	_ = binary.Write(&buf, binary.LittleEndian, uint32(rate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(rate*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(rate/10*2))
	for i := 0; i < rate/10; i++ {
		_ = binary.Write(&buf, binary.LittleEndian, int16(0))
	}
	return buf.Bytes()
}

// makeTestPNG 生成指定尺寸的纯色 PNG（base64），用于 OCR 连接测试
func makeTestPNG(w, h int) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{245, 245, 245, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func or(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// MaskKey API Key 脱敏：sk-ab12...wxyz，避免明文回传前端
func MaskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:6] + "***" + key[len(key)-4:]
}

// IsMasked 判断提交的 Key 是否为脱敏值（未修改）
func IsMasked(key string) bool {
	return strings.Contains(key, "***")
}

// cfgToConfig 将 DB 配置映射为运行时 Config 并重建 Provider
func cfgToConfig(cfg *model.AIConfig) *config.Config {
	c := config.Load() // 环境默认兜底
	c.LLMBaseURL = cfg.LLM.BaseURL
	c.LLMAPIKey = cfg.LLM.APIKey
	c.LLMModel = cfg.LLM.Model
	c.MockContent = false
	c.MockExplicit = false
	if cfg.LLM.Provider == "mock" {
		c.LLMAPIKey = ""
		c.MockContent = true
		c.MockExplicit = true // 配置页显式选择 Mock
	}
	if c.LLMAPIKey == "" && !c.MockExplicit {
		c.MockContent = true // 未配置 Key：自动 mock（拉取时提示）
	}
	c.TTSBaseURL = cfg.TTS.BaseURL
	c.TTSAPIKey = cfg.TTS.APIKey
	c.TTSModel = cfg.TTS.Model
	c.TTSVoice = cfg.TTS.Extra
	c.TTSType = cfg.TTS.Provider // edge（本地免费）/ openai（硅基等） / mock
	if c.TTSType == "" {
		c.TTSType = "openai"
	}
	if cfg.TTS.Provider == "mock" {
		c.TTSAPIKey = ""
		c.TTSType = "mock"
	}
	c.ImageBaseURL = cfg.Image.BaseURL
	c.ImageAPIKey = cfg.Image.APIKey
	c.ImageModel = cfg.Image.Model
	c.ImageSize = cfg.Image.Extra
	c.ImageProvider = cfg.Image.Provider // auto / openai / mock
	if c.ImageProvider == "" {
		c.ImageProvider = "auto"
	}
	c.OCRBaseURL = cfg.OCR.BaseURL
	c.OCRAPIKey = cfg.OCR.APIKey
	c.OCRModel = cfg.OCR.Model
	c.ASRBaseURL = cfg.ASR.BaseURL
	c.ASRAPIKey = cfg.ASR.APIKey
	c.ASRModel = cfg.ASR.Model
	return c
}

// LoadConfigIntoRegistry 启动时从 DB 加载配置并重建（无 DB 配置时用环境变量）
func LoadConfigIntoRegistry(db *gorm.DB, reg *ai.Registry) {
	var c model.AIConfig
	if err := db.First(&c, aiConfigRowID).Error; err == nil {
		reg.Rebuild(cfgToConfig(&c))
	}
}
