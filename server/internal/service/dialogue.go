package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
)

// DialogueService AI 实景对话：学生扮演场景中的一角色，AI 扮演其余角色
type DialogueService struct {
	db  *gorm.DB
	reg *ai.Registry
	log *slog.Logger
}

func NewDialogueService(db *gorm.DB, reg *ai.Registry) *DialogueService {
	return &DialogueService{
		db:  db,
		reg: reg,
		log: svcLogger.With("svc", "dialogue"),
	}
}

// CreateSession 创建会话：从课时内容提取角色，用户扮演 userRole，其余为 AI 角色
func (s *DialogueService) CreateSession(ctx context.Context, lessonID uint, userRole string) (*model.DialogueSession, error) {
	scene, title, allRoles, err := s.rolesFromLesson(lessonID)
	if err != nil {
		return nil, err
	}
	if userRole == "" {
		// 默认扮演第一个角色
		if len(allRoles) == 0 {
			return nil, errors.New("该课时没有可扮演的角色")
		}
		userRole = allRoles[0].Role
	}
	// 校验用户角色存在
	var userRoleName string
	userFound := false
	var aiList []model.AIRole
	seen := map[string]bool{}
	for _, r := range allRoles {
		if r.Role == userRole {
			userRoleName = r.RoleName
			userFound = true
			continue
		}
		if !seen[r.Role] {
			seen[r.Role] = true
			aiList = append(aiList, r)
		}
	}
	if !userFound {
		// 自由对话（lessonID=0）允许扮演任意角色；课时对话必须来自剧本角色
		if lessonID == 0 {
			userRoleName = userRole
			userFound = true
		} else {
			return nil, fmt.Errorf("角色 %s 不在该课时的对话角色中", userRole)
		}
	}
	if len(aiList) == 0 {
		return nil, errors.New("没有可扮演的 AI 角色（至少需要 2 个角色）")
	}

	aiJSON, _ := json.Marshal(aiList)
	session := &model.DialogueSession{
		LessonID:     lessonID,
		Scene:        scene,
		Title:        title,
		UserRole:     userRole,
		UserRoleName: userRoleName,
		AICharacters: string(aiJSON),
		Status:       "active",
		AIList:       aiList,
	}
	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

// rolesFromLesson 从课时内容提取对话角色列表（lessonID=0 时为自由对话场景）
func (s *DialogueService) rolesFromLesson(lessonID uint) (scene, title string, roles []model.AIRole, err error) {
	if lessonID == 0 {
		return "自由对话", "Free Talk", []model.AIRole{
			{Role: "teacher", RoleName: "英语老师"},
			{Role: "friend", RoleName: "外国朋友"},
		}, nil
	}
	var content model.AIContent
	if err := s.db.Where("catalog_item_id = ?", lessonID).First(&content).Error; err != nil {
		return "", "", nil, errors.New("该课时内容尚未生成，请先拉取内容")
	}
	var dialogue []model.DialogueTurn
	if content.DialogueJSON != "" {
		_ = json.Unmarshal([]byte(content.DialogueJSON), &dialogue)
	}
	seen := map[string]bool{}
	for _, turn := range dialogue {
		if !seen[turn.Role] {
			seen[turn.Role] = true
			roles = append(roles, model.AIRole{Role: turn.Role, RoleName: turn.RoleName})
		}
	}
	if len(roles) == 0 {
		return "", "", nil, errors.New("课时内容中没有对话角色")
	}
	return content.Scene, content.Title, roles, nil
}

// 对话 System Prompt：能力型教学约束（不直接纠错、简单英语、不跑题）
const dialogueSystemPrompt = `你是一位英语教学场景中的 AI 语伴，正在扮演「%s」场景中的 %s（英文角色 %s）。
教学约束（必须严格遵守）：
1. 全程使用简单初中英语，短句为主，地道自然。
2. 用户出现语法或用词错误时，不要直接指出或打断，在下一句回复中自然地复述正确说法，让他潜移默化地吸收。
3. 每次回复 1-3 句，可以追问，保持真实对话节奏感。
4. 不跑题，始终围绕「%s」场景交流。
5. 用户说再见/结束对话时，友好道别并总结今天用到的表达。`

// SendMessage 发送用户消息，AI 角色回复（保存两条消息）
func (s *DialogueService) SendMessage(ctx context.Context, sessionID uint, text string) (*model.DialogueMessage, *model.DialogueMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, errors.New("消息不能为空")
	}
	system, history, aiRole, err := s.buildContext(sessionID)
	if err != nil {
		return nil, nil, err
	}

	// 用户消息入库
	userMsg, err := s.saveUserMessage(sessionID, text)
	if err != nil {
		return nil, nil, err
	}

	// 调用 LLM 生成 AI 回复
	messages := append(history, ai.ChatMessage{Role: "user", Content: text})
	reply, err := s.reg.LLM().Chat(ctx, system, joinUserMessages(messages))
	if err != nil {
		return nil, nil, fmt.Errorf("AI 回复失败: %w", err)
	}
	s.log.Info("对话 LLM 调用",
		"session_id", sessionID, "user", truncateForLog(text, 500),
		"system_prompt", truncateForLog(system, 1000),
		"reply", truncateForLog(reply, 1000),
	)
	aiMsg, err := s.saveAIMessage(sessionID, strings.TrimSpace(reply), aiRole)
	if err != nil {
		return nil, nil, err
	}
	return userMsg, aiMsg, nil
}

// StreamReply 流式发送：用户消息入库 → AI 增量回调 → 完成后 AI 消息入库
func (s *DialogueService) StreamReply(ctx context.Context, sessionID uint, text string, onDelta ai.StreamDelta) (*model.DialogueMessage, *model.DialogueMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, errors.New("消息不能为空")
	}
	system, history, aiRole, err := s.buildContext(sessionID)
	if err != nil {
		return nil, nil, err
	}

	userMsg, err := s.saveUserMessage(sessionID, text)
	if err != nil {
		return nil, nil, err
	}

	messages := append(history, ai.ChatMessage{Role: "user", Content: text})
	var sb strings.Builder
	streamErr := s.reg.LLM().ChatStream(ctx, system, joinUserMessages(messages), func(delta string) error {
		sb.WriteString(delta)
		return onDelta(delta)
	})
	if streamErr != nil {
		return nil, nil, fmt.Errorf("AI 流式回复失败: %w", streamErr)
	}
	s.log.Info("对话 LLM 流式调用",
		"session_id", sessionID, "user", truncateForLog(text, 500),
		"reply", truncateForLog(sb.String(), 1000),
	)

	aiMsg, err := s.saveAIMessage(sessionID, strings.TrimSpace(sb.String()), aiRole)
	if err != nil {
		return nil, nil, err
	}
	return userMsg, aiMsg, nil
}

// buildContext 组装 System Prompt + 历史上下文 + AI 角色
func (s *DialogueService) buildContext(sessionID uint) (string, []ai.ChatMessage, model.AIRole, error) {
	var session model.DialogueSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return "", nil, model.AIRole{}, errors.New("会话不存在")
	}
	var aiList []model.AIRole
	_ = json.Unmarshal([]byte(session.AICharacters), &aiList)
	aiRole := model.AIRole{Role: "teacher", RoleName: "老师"}
	if len(aiList) > 0 {
		aiRole = aiList[0] // MVP：固定与第一个 AI 角色对话
	}

	system := fmt.Sprintf(dialogueSystemPrompt, session.Scene, aiRole.RoleName, aiRole.Role, session.Scene)
	var history []ai.ChatMessage
	var recent []model.DialogueMessage
	s.db.Where("session_id = ?", sessionID).Order("id desc").Limit(10).Find(&recent)
	for i := len(recent) - 1; i >= 0; i-- {
		msg := recent[i]
		if msg.IsUser {
			history = append(history, ai.ChatMessage{Role: "user", Content: fmt.Sprintf("%s(用户): %s", msg.SpeakerName, msg.Content)})
		} else {
			history = append(history, ai.ChatMessage{Role: "assistant", Content: fmt.Sprintf("%s: %s", msg.SpeakerName, msg.Content)})
		}
	}
	return system, history, aiRole, nil
}

func (s *DialogueService) saveUserMessage(sessionID uint, text string) (*model.DialogueMessage, error) {
	var session model.DialogueSession
	if err := s.db.First(&session, sessionID).Error; err != nil {
		return nil, errors.New("会话不存在")
	}
	userMsg := &model.DialogueMessage{
		SessionID: sessionID, Speaker: session.UserRole,
		SpeakerName: session.UserRoleName, IsUser: true, Content: text,
	}
	if err := s.db.Create(userMsg).Error; err != nil {
		return nil, err
	}
	return userMsg, nil
}

func (s *DialogueService) saveAIMessage(sessionID uint, reply string, aiRole model.AIRole) (*model.DialogueMessage, error) {
	if reply == "" {
		reply = "I see. Please go on!"
	}
	audioURL, _ := s.reg.TTS().Synthesize(reply, aiRole.Role)
	aiMsg := &model.DialogueMessage{
		SessionID: sessionID, Speaker: aiRole.Role,
		SpeakerName: aiRole.RoleName, IsUser: false, Content: reply, AudioURL: audioURL,
	}
	if err := s.db.Create(aiMsg).Error; err != nil {
		return nil, err
	}
	return aiMsg, nil
}

// joinUserMessages 将历史 user/assistant 消息拼接为单条用户输入（简化流式前的实现）
func joinUserMessages(msgs []ai.ChatMessage) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ListMessages 会话历史消息
func (s *DialogueService) ListMessages(sessionID uint) ([]model.DialogueMessage, error) {
	var msgs []model.DialogueMessage
	if err := s.db.Where("session_id = ?", sessionID).Order("id").Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}
