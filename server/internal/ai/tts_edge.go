package ai

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// EdgeTTSMinVersion 微软 Sec-MS-GEC v2 令牌的最低可用版本（7.3.0 起实现）
	EdgeTTSMinVersion = "7.3.0"
	// minAudioBytes 判定合成产物有效的最小字节数；0 字节残壳一律视为失败
	minAudioBytes = 1024
)

// EdgeTTS 免费多音色 TTS：调用微软 Edge「大声朗读」在线服务（edge-tts 命令行）。
// 音色按角色映射（男孩/女孩/男女教师/爷爷奶奶等），无需 Key；该服务是微软未公开接口，需联网。
type EdgeTTS struct {
	dataDir string
	sem     chan struct{} // 合成并发限制（最多 2 个 edge-tts 进程同时跑，防资源占用峰值）
}

// edgeRoleVoices 角色 → edge-tts 英文音色（神经网络语音，自然度高）。
// 音色名已于 2026-10-22 逐个比对线上清单（322 个）校验：en-US-JaneNeural / CoraNeural /
// DavisNeural 均已下架——选中不报错但收不到音频，故换为在列音色。改动此处后请重新校验。
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
	"grandma": "en-US-AvaNeural",         // 奶奶（成人女声）
	"brother": "en-US-EricNeural",        // 哥哥
	"sister":  "en-US-AriaNeural",        // 姐姐
	"uncle":   "en-US-RogerNeural",       // 叔叔
	"aunt":    "en-US-EmmaNeural",        // 阿姨
	"mother":  "en-US-MichelleNeural",    // 妈妈
	"father":  "en-US-AndrewNeural",      // 爸爸
	"mom":     "en-US-MichelleNeural",
	"dad":     "en-US-AndrewNeural",

	// 职业
	"farmer":   "en-US-BrianNeural",  // 农民（浑厚男声，与男教师区分）
	"worker":   "en-US-AndrewNeural", // 工人
	"waiter":   "en-US-AndrewNeural", // 服务员
	"customer": "en-US-AnaNeural",    // 顾客
	"clerk":    "en-US-RogerNeural",  // 店员
	"driver":   "en-US-AndrewNeural", // 司机
	"doctor":   "en-US-MichelleNeural",
	"nurse":    "en-US-JennyNeural",
	"police":   "en-US-AndrewNeural", // 警察
	"friend":   "en-US-AriaNeural",

	"vocab":   "en-US-JennyNeural",
	"default": "en-US-JennyNeural",
}

func NewEdgeTTS(dataDir string) *EdgeTTS {
	return &EdgeTTS{dataDir: dataDir, sem: make(chan struct{}, 2)}
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

// Synthesize 合成文本音频（调 edge-tts CLI），返回 /media/audio/{hash}.mp3。
// 产物按 voice+text 哈希缓存；0 字节残壳一律视为失败并删除，避免污染缓存。
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
	// 缓存命中必须验大小：edge-tts 会先建 0 字节文件再联网握手，失败时留下空壳。
	// 只判存在会把 0 字节文件当有效缓存，之后同一句永远返回静音 URL。
	if st, err := os.Stat(path); err == nil && st.Size() >= minAudioBytes {
		return "/media/audio/" + name, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	// 并发限制：最多 2 个 edge-tts 进程同时合成（防多气泡连点造成资源峰值）
	t.sem <- struct{}{}
	defer func() { <-t.sem }()

	// 调用 edge-tts CLI（首次需 pip install "edge-tts>=7.3.0"）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := edgeTTSCommand(ctx, voice, text, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(path) // 清掉 0 字节残壳，避免污染下次缓存
		return "", fmt.Errorf("edge-tts 合成失败（需 pip install \"edge-tts>=%s\"）: %s", EdgeTTSMinVersion, firstLine(string(out)))
	}
	// 退出码为 0 但没落地有效音频同样算失败（服务侧空响应等）
	if st, serr := os.Stat(path); serr != nil || st.Size() < minAudioBytes {
		os.Remove(path)
		return "", fmt.Errorf("edge-tts 未产出有效音频（%s）: %s", voice, firstLine(string(out)))
	}
	return "/media/audio/" + name, nil
}

// edgeTTSCommand 构造 edge-tts 命令：复用 edgeTTSCmdPrefix 的探测结果，
// 未安装时退回首个 python 候选，让失败原因由子进程如实报出
// Windows 下隐藏子进程控制台窗口（否则每次合成弹出黑窗）
func edgeTTSCommand(ctx context.Context, voice, text, path string) *exec.Cmd {
	args := []string{"--voice", voice, "--text", text, "--write-media", path}
	prefix, ok := edgeTTSCmdPrefix()
	if !ok {
		prefix = []string{pythonCandidates()[0], "-m", "edge_tts"}
	}
	cmd := exec.CommandContext(ctx, prefix[0], append(prefix[1:], args...)...)
	hideWindow(cmd)
	return cmd
}

// edgeTTSProbeCache 缓存探测结果。主程序是 GUI 子系统进程（PE Subsystem=2，无控制台），
// 每 spawn 一个需要控制台的子进程，Windows 都会新建一个控制台窗口——模型配置页每次打开
// 都查一次可用性，不缓存就会连闪多个黑窗。安装/升级 edge-tts 后需 Reset 使缓存失效。
var edgeTTSProbeCache struct {
	mu      sync.Mutex
	prefix  []string
	version string
	probed  bool
}

// edgeTTSProbe 返回调用入口与已安装版本；未安装时 prefix 为 nil
func edgeTTSProbe() ([]string, string) {
	edgeTTSProbeCache.mu.Lock()
	defer edgeTTSProbeCache.mu.Unlock()
	if !edgeTTSProbeCache.probed {
		if prefix, ok := edgeTTSCmdPrefix(); ok {
			edgeTTSProbeCache.prefix = prefix
			edgeTTSProbeCache.version = edgeTTSVersion(prefix)
		}
		edgeTTSProbeCache.probed = true
	}
	return edgeTTSProbeCache.prefix, edgeTTSProbeCache.version
}

// edgeTTSProbeReset 丢弃缓存，让下次探测读到刚装好的版本
func edgeTTSProbeReset() {
	edgeTTSProbeCache.mu.Lock()
	edgeTTSProbeCache.prefix = nil
	edgeTTSProbeCache.version = ""
	edgeTTSProbeCache.probed = false
	edgeTTSProbeCache.mu.Unlock()
}

// EdgeTTSAvailable 本机 edge-tts 是否真正可用：装了且版本够新。
// 微软 2026-10 把握手令牌 Sec-MS-GEC 升级为 v2，仅 7.3.0+ 实现；旧版对**所有**音色
// 一律 403（实测 7.2.8 全音色失败），所以只判 import 成功会把坏环境误报为就绪。
func EdgeTTSAvailable() bool {
	prefix, version := edgeTTSProbe()
	if prefix == nil {
		return false
	}
	return versionAtLeast(version, EdgeTTSMinVersion)
}

// edgeTTSCmdPrefix 返回调用 edge-tts 的命令前缀：优先 CLI（仅 LookPath，不起进程），
// 其次 python -m edge_tts（需起进程探测，必须隐藏控制台窗口）
func edgeTTSCmdPrefix() ([]string, bool) {
	if _, err := exec.LookPath("edge-tts"); err == nil {
		return []string{"edge-tts"}, true
	}
	for _, py := range pythonCandidates() {
		cmd := exec.Command(py, "-c", "import edge_tts")
		hideWindow(cmd)
		if err := cmd.Run(); err == nil {
			return []string{py, "-m", "edge_tts"}, true
		}
	}
	return nil, false
}

// edgeTTSVersion 读取已安装的 edge-tts 版本号；读不到返回空串。
// CLI 入口走 --version（`edge-tts 7.2.8`），模块入口走 python -c——
// 两者参数形态不同，不能拿 CLI 去喂 python 的 -c
func edgeTTSVersion(prefix []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var args []string
	if prefix[0] == "edge-tts" {
		args = []string{"--version"}
	} else {
		args = []string{"-c", "import edge_tts; print(edge_tts.__version__)"}
	}
	cmd := exec.CommandContext(ctx, prefix[0], append(prefix[1:], args...)...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return trimVersion(string(out))
}

// trimVersion 从 "edge-tts 7.2.8" / "7.3.1" 这类输出里取出点分版本号
func trimVersion(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, " \t\n"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimFunc(s, func(r rune) bool {
		return !(r >= '0' && r <= '9') && r != '.'
	})
}

// versionAtLeast 比较点分版本号（忽略预发布后缀），actual 为空视为过旧
func versionAtLeast(actual, minimum string) bool {
	parse := func(s string) []int {
		parts := strings.Split(s, "-")
		nums := strings.Split(parts[0], ".")
		out := make([]int, 3)
		for i := 0; i < 3 && i < len(nums); i++ {
			digits := strings.TrimFunc(nums[i], func(r rune) bool { return r < '0' || r > '9' })
			n, _ := strconv.Atoi(digits)
			out[i] = n
		}
		return out
	}
	a, m := parse(actual), parse(minimum)
	for i := 0; i < 3; i++ {
		if a[i] != m[i] {
			return a[i] > m[i]
		}
	}
	return true
}

// firstLine 取多行输出的首行（edge-tts 失败时是 Python traceback，只留最有信息量的一行）
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "无输出"
	}
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// EnsureEdgeTTS 启动时确保 edge-tts 可用且版本够新（幂等，不阻塞调用方）：
// 已就绪直接返回；未安装则安装，已安装但版本低于 EdgeTTSMinVersion 则升级——
// 否则旧版会永久 403 且因"已安装"而永不修复。logf 可选（写应用日志）
func EnsureEdgeTTS(logf func(format string, args ...any)) {
	prefix, installed := edgeTTSCmdPrefix()
	version := ""
	if installed {
		version = edgeTTSVersion(prefix)
		if versionAtLeast(version, EdgeTTSMinVersion) {
			if logf != nil {
				logf("edge-tts 已就绪（%s）", version)
			}
			return
		}
	}
	spec := "edge-tts>=" + EdgeTTSMinVersion
	action := "pip install"
	if installed {
		action = "pip install -U（版本过旧 " + version + "，需 " + EdgeTTSMinVersion + "+）"
	}
	if logf != nil {
		logf("edge-tts 不可用，尝试自动处理（%s %s）…", action, spec)
	}
	for _, py := range pythonCandidates() {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		out, err := exec.CommandContext(ctx, py, "-m", "pip", "install", "-U", "-q", spec).CombinedOutput()
		cancel()
		if EdgeTTSAvailable() {
			if logf != nil {
				logf("edge-tts 就绪（%s）", py)
			}
			return
		}
		if logf != nil {
			reason := firstLine(string(out))
			if err != nil {
				reason = err.Error() + " " + reason
			}
			logf("edge-tts 安装失败（%s）: %s", py, reason)
		}
	}
}

// pythonCandidates Windows/Unix 常见 Python 命令
func pythonCandidates() []string {
	var out []string
	for _, c := range []string{"python", "python3", "py"} {
		if _, err := exec.LookPath(c); err == nil {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = []string{"python"}
	}
	return out
}
