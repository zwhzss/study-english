package service

import (
	"encoding/json"
	"math"

	"gorm.io/gorm"

	"study-english/server/internal/model"
)

// AbilityProfile 能力画像（四维，0-100）
type AbilityProfile struct {
	Listening       int    `json:"listening"`        // 听
	Speaking        int    `json:"speaking"`         // 说
	Reading         int    `json:"reading"`          // 读
	Writing         int    `json:"writing"`          // 写
	TotalActivities int    `json:"total_activities"` // 总学习活动数
	Weakest         string `json:"weakest"`          // 最薄弱维度
	Advice          string `json:"advice"`           // 学习建议
}

// AbilityService 能力画像：从用户学习活动统计推导四维能力值（MVP，无主观打分）
// 原则：能力型教学——记录"能力值"而非考试分，渐进曲线避免线性爆表
type AbilityService struct {
	db *gorm.DB
}

func NewAbilityService(db *gorm.DB) *AbilityService {
	return &AbilityService{db: db}
}

// GetProfile 统计现有活动并推导四维能力值
// 数据源（无新表，从现有活动反推）：
//   - 说：用户主动发出的对话消息数（AI 实景对话）
//   - 听：AI 角色回复消息数 + 会话数（听 AI 说话）
//   - 读：已生成/学习的课时内容数（含多角色对话剧本）
//   - 写：所有课时的能力型练习条目数（roleplay/speak/write 的总和，作为写作实践）
func (s *AbilityService) GetProfile() (*AbilityProfile, error) {
	var userMsgs, aiMsgs, sessions int64
	s.db.Model(&model.DialogueMessage{}).Where("is_user = ?", true).Count(&userMsgs)
	s.db.Model(&model.DialogueMessage{}).Where("is_user = ?", false).Count(&aiMsgs)
	s.db.Model(&model.DialogueSession{}).Count(&sessions)

	var contents []model.AIContent
	if err := s.db.Find(&contents).Error; err != nil {
		return nil, err
	}
	var practiceCount int64
	for _, c := range contents {
		var practice []model.PracticeItem
		if c.PracticeJSON != "" {
			_ = json.Unmarshal([]byte(c.PracticeJSON), &practice)
		}
		practiceCount += int64(len(practice))
	}

	listening := score(int(aiMsgs) + int(sessions))
	speaking := score(int(userMsgs) + int(sessions))
	reading := score(int(len(contents)) * 2)
	writing := score(int(practiceCount))

	p := &AbilityProfile{
		Listening:       listening,
		Speaking:        speaking,
		Reading:         reading,
		Writing:         writing,
		TotalActivities: int(userMsgs + aiMsgs + sessions + int64(len(contents)) + practiceCount),
	}

	// 最薄弱维度 + 建议
	dims := []struct {
		name  string
		score int
		adv   string
	}{
		{"听力", listening, "多听真实场景对话，用「聆听模式」跟读原声"},
		{"口语", speaking, "多用「AI 实景对话」开口练习，试试语音输入"},
		{"阅读", reading, "多学新课时，用「穿透式阅读」积累句型"},
		{"写作", writing, "完成课时的「写一写」练习，积累地道表达"},
	}
	minScore, minIdx := dims[0].score, 0
	for i, d := range dims {
		if d.score < minScore {
			minScore, minIdx = d.score, i
		}
	}
	p.Weakest = dims[minIdx].name
	p.Advice = "建议优先提升「" + dims[minIdx].name + "」：" + dims[minIdx].adv
	return p, nil
}

// score 渐进能力曲线：20 基础分 + 15*log2(1+n)，封顶 100
func score(activity int) int {
	if activity <= 0 {
		return 20
	}
	v := 20 + 15*math.Log2(float64(1+activity))
	if v > 100 {
		return 100
	}
	return int(v)
}
