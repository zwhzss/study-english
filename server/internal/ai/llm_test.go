package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"study-english/server/internal/config"
)

// TestJSONErrorContextPointsAtFault 出错点在 500 字节之外时，仍要能看到出错处内容。
// 旧实现固定打印前 500 字节，本次线上故障的出错点就在窗口之外，完全没给线索。
func TestJSONErrorContextPointsAtFault(t *testing.T) {
	filler := strings.Repeat("x", 900)
	bad := `{"unit":"` + filler + `", "dialogue": [["boy", "hi"]: "oops"]}`

	var out map[string]any
	err := json.Unmarshal([]byte(bad), &out)
	if err == nil {
		t.Fatal("构造的样本应当解析失败")
	}
	ctx := jsonErrorContext(bad, err)
	if !strings.Contains(ctx, "dialogue") {
		t.Errorf("出错上下文应包含出错点附近内容，实得:\n%s", ctx)
	}
	if !strings.Contains(ctx, "出错点") {
		t.Errorf("应标出错点位置，实得:\n%s", ctx)
	}
}

// TestJSONErrorContextRuneSafe 中文内容按字节偏移截取时不能切出半个汉字
func TestJSONErrorContextRuneSafe(t *testing.T) {
	s := `{"scene":"` + strings.Repeat("公园", 300) + `", "d": [["a":1]}`
	var out map[string]any
	err := json.Unmarshal([]byte(s), &out)
	if err == nil {
		t.Fatal("构造的样本应当解析失败")
	}
	ctx := jsonErrorContext(s, err)
	if !utf8.ValidString(ctx) {
		t.Error("出错上下文含被切断的半个字符")
	}
	if !utf8.ValidString(clipRunes(s, 7)) {
		t.Error("clipRunes 在字符中间截断")
	}
}

// TestDoChatReportsTruncation finish_reason=length 必须报成"被 max_tokens 截断"，
// 否则调用方只能看到一句 JSON 解析失败，把长度问题误判成模型输出格式问题
func TestDoChatReportsTruncation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.MaxTokens != 4096 {
			t.Errorf("应把配置的 max_tokens 透传给服务端，实得 %d", req.MaxTokens)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"{\"a\":1"}}],"usage":{"completion_tokens":4096}}`))
	}))
	defer srv.Close()

	d := &DeepSeekLLM{baseURL: srv.URL, apiKey: "k", model: "m", maxTokens: 4096}
	_, err := d.ChatJSON(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("截断应返回错误")
	}
	for _, want := range []string{"max_tokens=4096", "finish_reason=length", "LLM_MAX_TOKENS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应含 %q，实得: %v", want, err)
		}
	}
}

// TestDoChatAcceptsNormalStop 正常结束不得被当成截断
func TestDoChatAcceptsNormalStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"pong"}}]}`))
	}))
	defer srv.Close()

	d := &DeepSeekLLM{baseURL: srv.URL, apiKey: "k", model: "m"}
	got, err := d.Chat(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("正常应答不应报错: %v", err)
	}
	if got != "pong" {
		t.Errorf("实得 %q", got)
	}
	if d.effectiveMaxTokens() != config.DefaultLLMMaxTokens {
		t.Errorf("未配置时应回落默认上限，实得 %d", d.effectiveMaxTokens())
	}
}
