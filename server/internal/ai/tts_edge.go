package ai

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EdgeTTS 免费本地 TTS：调用微软 Edge 神经网络语音（edge-tts 命令行），
// 100+ 音色免费无 Key，按角色映射不同音色（男孩/女孩/男女教师/爷爷奶奶等），完全离线合成
type EdgeTTS struct {
	dataDir string
}

// edgeRoleVoices 角色 → edge-tts 英文音色（神经网络语音，自然度高）
var edgeRoleVoices = map[string]string{
	// 儿童
	"boy":     "en-US-SteffanNeural", // 男童
	"girl":    "en-US-AnaNeural",     // 女童
	"student": "en-US-SteffanNeural", // 学生（童声）

	// 教师
	"teacher":        "en-US-JennyNeural",       // 年轻女教师
	"teacher_male":   "en-US-GuyNeural",         // 年轻男教师
	"male_teacher":   "en-US-GuyNeural",         // 年轻男教师
	"female_teacher": "en-US-JennyNeural",       // 年轻女教师
	"teacher_female": "en-US-MichelleNeural",    // 中年女教师
	"teacher_middle": "en-US-ChristopherNeural", // 中年男教师
	"headmaster":     "en-US-ChristopherNeural",

	// 家庭成员
	"grandpa": "en-US-ChristopherNeural", // 爷爷（老年男声）
	"grandma": "en-US-JaneNeural",        // 奶奶（老年女声）
	"brother": "en-US-EricNeural",        // 哥哥
	"sister":  "en-US-AriaNeural",        // 姐姐
	"uncle":   "en-US-RogerNeural",       // 叔叔
	"aunt":    "en-US-CoraNeural",        // 阿姨
	"mother":  "en-US-MichelleNeural",    // 妈妈
	"father":  "en-US-DavisNeural",       // 爸爸
	"mom":     "en-US-MichelleNeural",
	"dad":     "en-US-DavisNeural",

	// 职业
	"farmer":   "en-US-BrianNeural", // 农民（浑厚男声，与男教师区分）
	"worker":   "en-US-DavisNeural", // 工人
	"waiter":   "en-US-DavisNeural", // 服务员
	"customer": "en-US-AnaNeural",   // 顾客
	"clerk":    "en-US-RogerNeural", // 店员
	"driver":   "en-US-DavisNeural", // 司机
	"doctor":   "en-US-MichelleNeural",
	"nurse":    "en-US-JennyNeural",
	"police":   "en-US-DavisNeural",
	"friend":   "en-US-AriaNeural",

	"vocab":   "en-US-JennyNeural",
	"default": "en-US-JennyNeural",
}

func NewEdgeTTS(dataDir string) *EdgeTTS {
	return &EdgeTTS{dataDir: dataDir}
}

// voiceForRole 角色 → edge-tts 音色：先精确匹配，再按关键词（boy/girl/teacher/grandpa...）模糊匹配
func (t *EdgeTTS) voiceForRole(role string) string {
	if v, ok := edgeRoleVoices[role]; ok {
		return v
	}
	r := strings.ToLower(role)
	for _, kw := range []string{"grandpa", "grandma", "teacher", "student", "brother", "sister",
		"uncle", "aunt", "farmer", "worker", "waiter", "customer", "doctor", "nurse", "mother", "father", "boy", "girl"} {
		if strings.Contains(r, kw) {
			if v, ok := edgeRoleVoices[kw]; ok {
				return v
			}
		}
	}
	return edgeRoleVoices["default"]
}

// Synthesize 合成文本音频（本地 edge-tts），返回 /media/audio/{hash}.mp3
func (t *EdgeTTS) Synthesize(text, role string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("文本为空")
	}
	voice := t.voiceForRole(role)
	hash := sha1.Sum([]byte(voice + "|" + text))
	name := hex.EncodeToString(hash[:10]) + ".mp3"
	dir := filepath.Join(t.dataDir, "audio")
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return "/media/audio/" + name, nil // 缓存命中
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	// 调用 edge-tts CLI（首次需 pip install edge-tts）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := edgeTTSCommand(ctx, voice, text, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("edge-tts 合成失败（请先 pip install edge-tts）: %s", strings.TrimSpace(string(out)))
	}
	return "/media/audio/" + name, nil
}

// edgeTTSCommand 构造 edge-tts 命令：优先 edge-tts 可执行，否则 python -m edge_tts
func edgeTTSCommand(ctx context.Context, voice, text, path string) *exec.Cmd {
	args := []string{"--voice", voice, "--text", text, "--write-media", path}
	if _, err := exec.LookPath("edge-tts"); err == nil {
		return exec.CommandContext(ctx, "edge-tts", args...)
	}
	if _, err := exec.LookPath("python"); err == nil {
		return exec.CommandContext(ctx, "python", append([]string{"-m", "edge_tts"}, args...)...)
	}
	return exec.CommandContext(ctx, "python3", append([]string{"-m", "edge_tts"}, args...)...)
}

// EdgeTTSAvailable 检查本机 edge-tts 是否可用
func EdgeTTSAvailable() bool {
	if _, err := exec.LookPath("edge-tts"); err == nil {
		return true
	}
	if _, err := exec.LookPath("python"); err == nil {
		return true // python -m edge_tts 可行（依赖包存在与否在合成时判断）
	}
	return false
}
