package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionAtLeast 版本闸门回归：微软 Sec-MS-GEC v2 令牌自 7.3.0 起实现，
// 7.2.8 对所有音色一律 403，必须判为过旧
func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		actual, min string
		want        bool
	}{
		{"7.2.8", "7.3.0", false}, // 本机实测全音色 403 的版本
		{"7.3.0", "7.3.0", true},
		{"7.3.1", "7.3.0", true},
		{"7.10.0", "7.3.0", true},  // 次位数字按整数比，不能按字符串比
		{"8.0.0", "7.3.0", true},   // 跨大版本
		{"7.3", "7.3.0", true},     // 缺省补丁位按 0 处理
		{"7.4.0b1", "7.3.0", true}, // 预发布后缀不参与比较
		{"", "7.3.0", false},       // 读不到版本一律视为不可用
		{"6.1.19", "7.3.0", false},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.actual, c.min); got != c.want {
			t.Errorf("versionAtLeast(%q, %q) = %v，期望 %v", c.actual, c.min, got, c.want)
		}
	}
}

// edgeLiveVoicesSnapshot 2026-10-22 逐个比对线上音色清单（322 个）后确认在列的英文名。
// 微软会静默下架音色——选了不报错但收不到音频，故把校验结果钉进测试：
// 改动 edgeRoleVoices 引入不在本表的音色，或表内音色被再次下架，都会在这里暴露。
var edgeLiveVoicesSnapshot = map[string]bool{
	"en-US-AnaNeural":         true,
	"en-US-AndrewNeural":      true,
	"en-US-AriaNeural":        true,
	"en-US-AvaNeural":         true,
	"en-US-BrianNeural":       true,
	"en-US-ChristopherNeural": true,
	"en-US-EmmaNeural":        true,
	"en-US-EricNeural":        true,
	"en-US-GuyNeural":         true,
	"en-US-JennyNeural":       true,
	"en-US-MichelleNeural":    true,
	"en-US-RogerNeural":       true,
	"en-US-SteffanNeural":     true,
}

// TestEdgeRoleVoicesLive 角色表里的每个音色都必须在实测在列快照中
func TestEdgeRoleVoicesLive(t *testing.T) {
	for role, voice := range edgeRoleVoices {
		if !edgeLiveVoicesSnapshot[voice] {
			t.Errorf("角色 %q 的音色 %q 不在 2026-10-22 实测在列清单中（可能已被微软下架）", role, voice)
		}
	}
}

// TestEdgeRoleVoicesRejectDelistedNames 已实测下架的三个音色不得回流。
// 其中 DavisNeural 曾被 5 个角色引用，一旦回归会造成大面积静默无音频。
func TestEdgeRoleVoicesRejectDelistedNames(t *testing.T) {
	delisted := []string{"en-US-JaneNeural", "en-US-CoraNeural", "en-US-DavisNeural"}
	present := map[string]bool{}
	for _, v := range edgeRoleVoices {
		present[v] = true
	}
	for _, d := range delisted {
		if present[d] {
			t.Errorf("音色 %s 已下架，不应再被引用", d)
		}
	}
}

// TestVoiceForRole 角色解析：精确匹配、关键词模糊匹配、未知角色兜底
func TestVoiceForRole(t *testing.T) {
	tr := &EdgeTTS{}
	if got := tr.voiceForRole("waiter"); got != edgeRoleVoices["waiter"] {
		t.Errorf("精确匹配失败：waiter → %q", got)
	}
	if got := tr.voiceForRole("female_teacher"); got != edgeRoleVoices["female_teacher"] {
		t.Errorf("精确匹配失败：female_teacher → %q", got)
	}
	// 未登记的角色靠关键词命中：my_grandpa_1 → grandpa
	if got := tr.voiceForRole("my_grandpa_1"); got != edgeRoleVoices["grandpa"] {
		t.Errorf("关键词模糊匹配失败：my_grandpa_1 → %q", got)
	}
	if got := tr.voiceForRole("完全没听过的角色"); got != edgeRoleVoices["default"] {
		t.Errorf("未知角色应兜底 default，实得 %q", got)
	}
}

// TestEdgeTTSVersionDetection 本机装了 edge-tts 就必须能读出版本号。
// CLI 入口（edge-tts --version）与模块入口（python -c import…）参数形态不同，
// 任一条走错都会返回空串，进而把正常环境误判为不可用、每次启动重复 pip。
func TestEdgeTTSVersionDetection(t *testing.T) {
	prefix, ok := edgeTTSCmdPrefix()
	if !ok {
		t.Skip("本机未安装 edge-tts，跳过探测测试")
	}
	v := edgeTTSVersion(prefix)
	if v == "" {
		t.Fatalf("已安装但版本探测失败，入口 %v 返回空版本", prefix)
	}
	t.Logf("本机 edge-tts 入口=%v 版本=%s 闸门=%s 判定可用=%v",
		prefix, v, EdgeTTSMinVersion, versionAtLeast(v, EdgeTTSMinVersion))
}

// TestTrimVersion 版本号提取
func TestTrimVersion(t *testing.T) {
	cases := map[string]string{
		"edge-tts 7.2.8\n":        "7.2.8",
		"7.3.1":                   "7.3.1",
		"edge-tts, version 7.3.0": "7.3.0",
		"":                        "",
	}
	for in, want := range cases {
		if got := trimVersion(in); got != want {
			t.Errorf("trimVersion(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestEdgeTTSSynthesizeReal 真实合成冒烟（默认跳过，需显式开启）：
// 走完整代码路径（角色→音色→CLI→产物校验），验证本机 edge-tts 确实能出可播放音频。
// 开启：RUN_EDGE_TTS_SMOKE=1 go test ./internal/ai -run SynthesizeReal -v
func TestEdgeTTSSynthesizeReal(t *testing.T) {
	if os.Getenv("RUN_EDGE_TTS_SMOKE") != "1" {
		t.Skip("设 RUN_EDGE_TTS_SMOKE=1 才跑真实合成（需联网）")
	}
	if !EdgeTTSAvailable() {
		t.Skipf("本机 edge-tts 不满足 >=%s，跳过", EdgeTTSMinVersion)
	}
	tr := NewEdgeTTS(t.TempDir())
	text := "Hello, could you pass me the ruler, please?"
	for _, role := range []string{"teacher", "waiter", "grandma", "aunt", "father", "student"} {
		voice := tr.voiceForRole(role)
		url, err := tr.Synthesize(text, role)
		if err != nil {
			t.Errorf("角色 %s（音色 %s）合成失败: %v", role, voice, err)
			continue
		}
		full := filepath.Join(tr.dataDir, filepath.FromSlash(strings.TrimPrefix(url, "/media/")))
		st, serr := os.Stat(full)
		if serr != nil || st.Size() < minAudioBytes {
			t.Errorf("角色 %s 产物无效: %s err=%v", role, full, serr)
			continue
		}
		t.Logf("OK 角色=%-8s 音色=%-24s %d bytes", role, voice, st.Size())
	}
}

// TestFirstLine edge-tts 失败时输出 Python traceback，错误信息只留首行
func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"a\nb\nc":      "a",
		"  solo  ":     "solo",
		"":             "无输出",
		"line1\nline2": "line1",
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q，期望 %q", in, got, want)
		}
	}
}
