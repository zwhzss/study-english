package service

import (
	"context"
	"encoding/json"
	"testing"

	"study-english/server/internal/model"
)

func ctxTest() context.Context { return context.Background() }

// TestAbilityProfile 能力画像：初始零活动 → 基础分；有活动后递增且封顶 100
func TestAbilityProfile(t *testing.T) {
	db := newTestDB(t)
	abSvc := NewAbilityService(db)

	// 零活动：全 20 基础分
	p, err := abSvc.GetProfile()
	if err != nil {
		t.Fatalf("获取画像失败: %v", err)
	}
	if p.Listening != 20 || p.Speaking != 20 || p.Reading != 20 || p.Writing != 20 {
		t.Fatalf("零活动应为 20 基础分, 实际 %+v", p)
	}
	if p.Weakest == "" {
		t.Fatal("最弱维度不应为空")
	}
	t.Logf("零活动画像: %+v | 建议: %s", p, p.Advice)

	// 模拟活动：创建教材+内容（读），对话若干轮（说/听）
	tbSvc := NewTextbookService(db, mockRegistry(t), "data")
	dlgSvc := NewDialogueService(db, mockRegistry(t))

	tb, _ := tbSvc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	tree, _ := tbSvc.GetCatalogTree(tb.ID)
	lessonID := tree[0].Lessons[0].ID
	content, err := tbSvc.PullLessonContent(ctxTest(), lessonID, false)
	if err != nil {
		t.Fatalf("拉取内容失败: %v", err)
	}
	sess, _ := dlgSvc.CreateSession(ctxTest(), lessonID, "customer")
	dlgSvc.SendMessage(ctxTest(), sess.ID, "Hello! I'd like a menu please.")

	p2, err := abSvc.GetProfile()
	if err != nil {
		t.Fatalf("二次获取失败: %v", err)
	}
	if p2.Speaking <= p.Speaking {
		t.Fatalf("对话后口语应提升: 前 %d 后 %d", p.Speaking, p2.Speaking)
	}
	if p2.Reading <= p.Reading {
		t.Fatalf("拉取内容后阅读应提升: 前 %d 后 %d", p.Reading, p2.Reading)
	}
	if p2.Speaking > 100 || p2.Reading > 100 {
		t.Fatal("能力值不应超过 100")
	}
	t.Logf("活动后画像: 听%d 说%d 读%d 写%d | 总活动%d", p2.Listening, p2.Speaking, p2.Reading, p2.Writing, p2.TotalActivities)

	// SVG 内容应已入库并解析
	if len(content.SVGs) == 0 {
		t.Log("提示：当前 mock 场景无 SVG（可接受）")
	} else if content.SVGs[0].Svg == "" {
		t.Fatal("SVG 内容为空")
	}
}

// TestSanitizeSVG 净化：script/foreignObject/on* 事件应被移除
func TestSanitizeSVG(t *testing.T) {
	dirty := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script><rect onclick="evil()" width="10"/><circle onload='x()' r="5"/><foreignObject><div>hi</div></foreignObject><text>ok</text></svg>`
	clean := sanitizeSVG(dirty)
	for _, bad := range []string{"script", "foreignObject", "onclick", "onload"} {
		if contains(clean, bad) {
			t.Fatalf("净化不彻底: 仍含 %q\n%s", bad, clean)
		}
	}
	if !contains(clean, "<text>ok</text>") {
		t.Fatalf("合法内容被误删: %s", clean)
	}
	t.Logf("SVG 净化 ✓: %s", clean)
}

// 确保类型引用（防未使用导入）
var _ = model.SVGImage{}

// TestPracticeHintsFlexible hints 容错：单字符串与数组都接受（LLM 常见输出偏差）
func TestPracticeHintsFlexible(t *testing.T) {
	var p1 model.PracticeItem
	_ = json.Unmarshal([]byte(`{"type":"speak","prompt":"P","hints":["a","b"]}`), &p1)
	if len(p1.Hints) != 2 {
		t.Fatalf("数组形式解析失败: %v", p1.Hints)
	}
	var p2 model.PracticeItem
	if err := json.Unmarshal([]byte(`{"type":"write","prompt":"P","hints":"I'd like..."}`), &p2); err != nil {
		t.Fatalf("单字符串形式应容错, 实际报错: %v", err)
	}
	if len(p2.Hints) != 1 || p2.Hints[0] != "I'd like..." {
		t.Fatalf("单字符串应转成单元素数组, 实际: %v", p2.Hints)
	}
	t.Logf("hints 容错 ✓: 数组=%v, 字符串=%v", p1.Hints, p2.Hints)
}
