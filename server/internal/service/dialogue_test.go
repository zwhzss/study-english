package service

import (
	"context"
	"testing"

	"study-english/server/internal/model"
)

func newDialogueTestService(t *testing.T) (*TextbookService, *DialogueService, uint) {
	t.Helper()
	db := newTestDB(t)
	tbSvc := NewTextbookService(db, mockRegistry(t))
	dlgSvc := NewDialogueService(db, mockRegistry(t))

	tb, err := tbSvc.CreateTextbook("人教版七年级英语上册", "人民教育出版社", "初中", "七年级", "builtin")
	if err != nil {
		t.Fatalf("创建教材失败: %v", err)
	}
	tree, _ := tbSvc.GetCatalogTree(tb.ID)
	// 用餐厅课时（Unit 3 Lesson 1，mock 返回餐厅场景多角色对话）
	lessonID := tree[0].Lessons[0].ID
	if _, err := tbSvc.PullLessonContent(context.Background(), lessonID, false); err != nil {
		t.Fatalf("拉取内容失败: %v", err)
	}
	return tbSvc, dlgSvc, lessonID
}

// TestDialogueCreateSession 创建会话：扮演 customer，AI 角色应为 waiter
func TestDialogueCreateSession(t *testing.T) {
	_, dlg, lessonID := newDialogueTestService(t)
	s, err := dlg.CreateSession(context.Background(), lessonID, "customer")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if s.UserRole != "customer" {
		t.Fatalf("用户角色应为 customer, 实际 %s", s.UserRole)
	}
	if len(s.AIList) == 0 {
		t.Fatal("AI 角色列表为空")
	}
	hasWaiter := false
	for _, r := range s.AIList {
		if r.Role == "waiter" {
			hasWaiter = true
		}
		if r.Role == s.UserRole {
			t.Fatalf("AI 角色不应包含用户角色 %s", s.UserRole)
		}
	}
	if !hasWaiter {
		t.Fatalf("AI 角色应包含 waiter, 实际 %+v", s.AIList)
	}
	t.Logf("会话: 用户扮演 %s(顾客), AI 角色 %+v, 场景=%s", s.UserRole, s.AIList, s.Scene)
}

// TestDialogueSendMessage 发送消息：AI 回复非空，两条消息入库
func TestDialogueSendMessage(t *testing.T) {
	_, dlg, lessonID := newDialogueTestService(t)
	s, err := dlg.CreateSession(context.Background(), lessonID, "customer")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	userMsg, aiMsg, err := dlg.SendMessage(context.Background(), s.ID, "Good evening! I'd like to order some food.")
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	if !userMsg.IsUser || userMsg.Content == "" {
		t.Fatal("用户消息入库异常")
	}
	if aiMsg.IsUser || aiMsg.Content == "" {
		t.Fatal("AI 回复为空")
	}
	if aiMsg.Speaker != "waiter" {
		t.Fatalf("AI 说话者应为 waiter, 实际 %s", aiMsg.Speaker)
	}
	msgs, err := dlg.ListMessages(s.ID)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("历史消息应为 2 条, 实际 %d (%v)", len(msgs), err)
	}
	t.Logf("对话: 用户「%s」→ AI(waiter)「%s」", userMsg.Content[:20], aiMsg.Content[:30])
}

// TestDialogueInvalidRole 非法角色应报错
func TestDialogueInvalidRole(t *testing.T) {
	_, dlg, lessonID := newDialogueTestService(t)
	_, err := dlg.CreateSession(context.Background(), lessonID, "king")
	if err == nil {
		t.Fatal("非法角色应报错")
	}
	t.Logf("非法角色正确报错: %v", err)
}

// TestDialogueFreeTalk 自由对话（lessonID=0）
func TestDialogueFreeTalk(t *testing.T) {
	_, dlg, _ := newDialogueTestService(t)
	s, err := dlg.CreateSession(context.Background(), 0, "student")
	if err != nil {
		t.Fatalf("创建自由对话失败: %v", err)
	}
	if s.Scene != "自由对话" {
		t.Fatalf("场景应为自由对话, 实际 %s", s.Scene)
	}
	_, aiMsg, err := dlg.SendMessage(context.Background(), s.ID, "Hello! How are you today?")
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	if aiMsg.Content == "" {
		t.Fatal("AI 回复为空")
	}
	t.Logf("自由对话 AI 回复: %s", aiMsg.Content)
}

// 确保 model 类型被引用（防止未使用导入）
var _ = model.DialogueSession{}
