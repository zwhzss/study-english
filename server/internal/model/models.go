package model

import "encoding/json"

import "time"

// 内容/目录节点状态机
const (
	StatusPending   = "pending"   // 未初始化
	StatusReady     = "ready"     // 目录已生成/内容已生成
	StatusDraft     = "draft"     // 草稿（待审核）
	StatusReviewed  = "reviewed"  // 已审核
	StatusPublished = "published" // 已发布
)

// Textbook 教材（一套教材 = 年级 → 单元 → 课时 目录树）
type Textbook struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Name          string    `gorm:"size:100;not null;index" json:"name"` // 教材名称，如 人教版七年级英语上册
	Publisher     string    `gorm:"size:50" json:"publisher"`            // 出版社
	Stage         string    `gorm:"size:20" json:"stage"`                // 小学/初中/高中/成人
	Grade         string    `gorm:"size:50" json:"grade"`                // 年级
	Version       string    `gorm:"size:20" json:"version"`              // 版本号
	CatalogSource string    `gorm:"size:20" json:"catalog_source"`       // builtin 内置模板 / llm 大模型生成
	Status        string    `gorm:"size:20;default:pending" json:"status"`
	SortOrder     int       `gorm:"default:0" json:"sort_order"` // 教材库排序（可手动调整）
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CatalogItem 教材目录节点（年级 → 单元 → 课时）
type CatalogItem struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	TextbookID uint      `gorm:"index;not null" json:"textbook_id"`
	Level      string    `gorm:"size:20" json:"level"`  // 年级，如 Grade7
	Unit       string    `gorm:"size:50" json:"unit"`   // 单元，如 Unit 1
	Lesson     string    `gorm:"size:50" json:"lesson"` // 课时标题
	Title      string    `gorm:"size:200" json:"title"` // 展示名
	Scene      string    `gorm:"size:100" json:"scene"` // 关联真实场景，如 餐厅点餐
	OrderNo    int       `json:"order_no"`
	Status     string    `gorm:"size:20;default:pending" json:"status"` // pending=未拉取内容
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SVGImage LLM 生成的简单示意图（SVG 源码，前端以 data URL 安全渲染）
type SVGImage struct {
	Title string `json:"title"`
	Svg   string `json:"svg"`
}

// AIContent AI 创作课时内容（能力型拓展：对话/词汇/语法/练习/配图/SVG）
// 独立表：与导入课文分开存储，同一课时可两者并存（前端 Tab 切换）
type AIContent struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	CatalogItemID uint   `gorm:"uniqueIndex:uk_ai_item;not null" json:"catalog_item_id"` // 每课时一条 AI 内容
	Unit          string `gorm:"size:50" json:"unit"`
	Title         string `gorm:"size:200" json:"title"`
	Scene         string `gorm:"size:100" json:"scene"`
	// 结构化内容（见 LessonPayload）
	DialogueJSON string    `gorm:"type:text" json:"-"`    // 多角色对话脚本
	KeyVocabJSON string    `gorm:"type:text" json:"-"`    // 核心词汇
	GrammarJSON  string    `gorm:"type:text" json:"-"`    // 语法点
	PracticeJSON string    `gorm:"type:text" json:"-"`    // 练习
	ImagesJSON   string    `gorm:"type:text" json:"-"`    // 配图
	SVGsJSON     string    `gorm:"type:text" json:"-"`    // LLM 生成的 SVG 示意图
	Source       string    `gorm:"size:20" json:"source"` // llm
	Status       string    `gorm:"size:20;default:published" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 便捷字段（前端展示）
	Dialogue   []DialogueTurn    `gorm:"-" json:"dialogue"`
	KeyVocab   []VocabItem       `gorm:"-" json:"key_vocab"`
	Grammar    []GrammarPoint    `gorm:"-" json:"grammar_points"`
	Practice   []PracticeItem    `gorm:"-" json:"practice"`
	Images     []string          `gorm:"-" json:"images"`
	SVGs       []SVGImage        `gorm:"-" json:"svg_images"`
	RoleVoices map[string]string `gorm:"-" json:"role_voices"` // 角色 → 音色/头像
}

// ImportedContent 导入课文内容（版权合规：教材原文经 PDF/文本录入）
type ImportedContent struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CatalogItemID uint      `gorm:"uniqueIndex:uk_imported_item;not null" json:"catalog_item_id"` // 每课时一条导入内容
	Unit          string    `gorm:"size:50" json:"unit"`
	Title         string    `gorm:"size:200" json:"title"`
	Text          string    `gorm:"type:text" json:"imported_text"` // 课文原文
	Source        string    `gorm:"size:20" json:"source"`          // imported
	Status        string    `gorm:"size:20;default:published" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LessonContent 旧版课时内容（v1 单表，仅用于数据迁移到 AIContent，不再用于新逻辑）
type LessonContent struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	CatalogItemID uint   `gorm:"uniqueIndex:uk_lesson_item_kind;not null" json:"catalog_item_id"`
	Kind          string `gorm:"size:20;uniqueIndex:uk_lesson_item_kind;default:ai" json:"kind"` // ai(AI创作) / imported(导入原文)
	Unit          string `gorm:"size:50" json:"unit"`
	Title         string `gorm:"size:200" json:"title"`
	Scene         string `gorm:"size:100" json:"scene"`
	// 结构化内容（见 LessonPayload）
	DialogueJSON string    `gorm:"type:text" json:"-"`    // 多角色对话脚本
	KeyVocabJSON string    `gorm:"type:text" json:"-"`    // 核心词汇
	GrammarJSON  string    `gorm:"type:text" json:"-"`    // 语法点
	PracticeJSON string    `gorm:"type:text" json:"-"`    // 练习
	ImagesJSON   string    `gorm:"type:text" json:"-"`    // 配图
	SVGsJSON     string    `gorm:"type:text" json:"-"`    // LLM 生成的 SVG 示意图
	ImportedText string    `gorm:"type:text" json:"-"`    // 导入的课文原文（source=imported 时展示）
	Source       string    `gorm:"size:20" json:"source"` // llm(创作) / imported(导入) / builtin / manual
	Status       string    `gorm:"size:20;default:draft" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 便捷字段（前端展示）
	Dialogue   []DialogueTurn    `gorm:"-" json:"dialogue"`
	KeyVocab   []VocabItem       `gorm:"-" json:"key_vocab"`
	Grammar    []GrammarPoint    `gorm:"-" json:"grammar_points"`
	Practice   []PracticeItem    `gorm:"-" json:"practice"`
	Images     []string          `gorm:"-" json:"images"`
	SVGs       []SVGImage        `gorm:"-" json:"svg_images"`
	Imported   string            `gorm:"-" json:"imported_text"` // 课文原文（导入模式）
	RoleVoices map[string]string `gorm:"-" json:"role_voices"`   // 角色 → 音色/头像
}

// DialogueTurn 多角色对话轮次
type DialogueTurn struct {
	Role     string `json:"role"`      // waiter / customer ...
	RoleName string `json:"role_name"` // 中文角色名，如 服务员
	Text     string `json:"text"`
	AudioURL string `json:"audio_url,omitempty"`
	Image    string `json:"image,omitempty"`
}

// VocabItem 核心词汇
type VocabItem struct {
	Word     string `json:"word"`
	Phonetic string `json:"phonetic"`
	Meaning  string `json:"meaning"`
	Example  string `json:"example"`
	AudioURL string `json:"audio_url,omitempty"`
}

// GrammarPoint 语法点
type GrammarPoint struct {
	Point       string `json:"point"`
	Explanation string `json:"explanation"`
	Example     string `json:"example"`
}

// PracticeItem 练习（能力型：开放式问题，无选择题）
type PracticeItem struct {
	Type    string   `json:"type"` // speak / write / roleplay
	Prompt  string   `json:"prompt"`
	Example string   `json:"example,omitempty"`
	Hints   []string `json:"hints,omitempty"`
}

// UnmarshalJSON 容错：LLM 可能把 hints 输出为单个字符串而非数组（如 "hints":"..."）
func (p *PracticeItem) UnmarshalJSON(data []byte) error {
	type alias PracticeItem
	aux := &struct {
		Hints json.RawMessage `json:"hints"`
		*alias
	}{alias: (*alias)(p)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.Hints) > 0 && string(aux.Hints) != "null" {
		var single string
		if err := json.Unmarshal(aux.Hints, &single); err == nil {
			p.Hints = []string{single}
		} else {
			var arr []string
			if err := json.Unmarshal(aux.Hints, &arr); err == nil {
				p.Hints = arr
			}
		}
	}
	return nil
}

// 内置角色音色映射（真实 TTS 接入后按角色换音色；浏览器回退用 SpeechSynthesis）
var DefaultRoleVoices = map[string]string{
	"waiter":   "男声-成熟",
	"customer": "女声-年轻",
	"teacher":  "女声-温柔",
	"student":  "童声",
	"clerk":    "男声-稳重",
	"driver":   "男声-中年",
	"doctor":   "女声-专业",
	"friend":   "女声-活泼",
}

// AIConfig 模型配置（数据库持久化，单行存储整份 JSON）
type AIConfig struct {
	ID        uint            `gorm:"primarykey" json:"id"`
	LLM       ProviderSetting `gorm:"type:text;serializer:json" json:"llm"`   // 大模型
	TTS       ProviderSetting `gorm:"type:text;serializer:json" json:"tts"`   // 语音
	Image     ProviderSetting `gorm:"type:text;serializer:json" json:"image"` // 绘图
	OCR       ProviderSetting `gorm:"type:text;serializer:json" json:"ocr"`   // OCR 视觉模型（图片导入课文）
	ASR       ProviderSetting `gorm:"type:text;serializer:json" json:"asr"`   // 语音识别（对话语音输入/发音评测）
	UpdatedAt time.Time       `json:"updated_at"`
}

// ProviderSetting 单个模型服务配置
type ProviderSetting struct {
	Provider string `json:"provider"` // openai（OpenAI 兼容）/ mock
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Extra    string `json:"extra,omitempty"` // TTS 音色 / 图片尺寸 等附加参数
}

// TextbookPreset 常用教材预设（消除手工输入歧义）
type TextbookPreset struct {
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
	Stage     string `json:"stage"` // 小学/初中/高中/成人
	Grade     string `json:"grade"` // 如 七年级
}

// CommonTextbooks 内置常用教材清单
var CommonTextbooks = []TextbookPreset{
	// 初中（人教版）
	{Name: "人教版七年级英语上册", Publisher: "人民教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "人教版七年级英语下册", Publisher: "人民教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "人教版八年级英语上册", Publisher: "人民教育出版社", Stage: "初中", Grade: "八年级"},
	{Name: "人教版八年级英语下册", Publisher: "人民教育出版社", Stage: "初中", Grade: "八年级"},
	{Name: "人教版九年级英语全一册", Publisher: "人民教育出版社", Stage: "初中", Grade: "九年级"},
	// 初中（外研版）
	{Name: "外研版七年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "七年级"},
	{Name: "外研版七年级英语下册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "七年级"},
	{Name: "外研版八年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "八年级"},
	{Name: "外研版九年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "九年级"},
	// 初中（牛津版）
	{Name: "牛津译林版七年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "七年级"},
	{Name: "牛津译林版八年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "八年级"},
	{Name: "牛津译林版九年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "九年级"},
	// 初中（北师大版）
	{Name: "北师大版七年级英语上册", Publisher: "北京师范大学出版社", Stage: "初中", Grade: "七年级"},
	{Name: "北师大版八年级英语上册", Publisher: "北京师范大学出版社", Stage: "初中", Grade: "八年级"},
	// 小学（人教版PEP）
	{Name: "人教版PEP三年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "三年级"},
	{Name: "人教版PEP四年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "四年级"},
	{Name: "人教版PEP五年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "五年级"},
	{Name: "人教版PEP六年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "六年级"},
	// 小学（外研版）
	{Name: "外研版三年级英语上册", Publisher: "外语教学与研究出版社", Stage: "小学", Grade: "三年级"},
	{Name: "外研版四年级英语上册", Publisher: "外语教学与研究出版社", Stage: "小学", Grade: "四年级"},
	// 高中（人教版）
	{Name: "人教版高中英语必修一", Publisher: "人民教育出版社", Stage: "高中", Grade: "高一"},
	{Name: "人教版高中英语必修二", Publisher: "人民教育出版社", Stage: "高中", Grade: "高一"},
	{Name: "人教版高中英语必修三", Publisher: "人民教育出版社", Stage: "高中", Grade: "高二"},
	{Name: "人教版高中英语选择性必修一", Publisher: "人民教育出版社", Stage: "高中", Grade: "高二"},
	// 高中（外研版）
	{Name: "外研版高中英语必修一", Publisher: "外语教学与研究出版社", Stage: "高中", Grade: "高一"},
	// 成人
	{Name: "新概念英语第一册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "初级"},
	{Name: "新概念英语第二册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "中级"},
	{Name: "新概念英语第三册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "中高级"},
}

// Stages 可选阶段
var Stages = []string{"小学", "初中", "高中", "成人"}

// GradesByStage 各阶段可选年级
var GradesByStage = map[string][]string{
	"小学": {"一年级", "二年级", "三年级", "四年级", "五年级", "六年级"},
	"初中": {"七年级", "八年级", "九年级"},
	"高中": {"高一", "高二", "高三"},
	"成人": {"初级", "中级", "中高级", "高级"},
}

// PronunciationRecord 发音评测记录（录音 → AI 评测 → 四维评分 → 回看）
type PronunciationRecord struct {
	ID             uint   `gorm:"primarykey" json:"id"`
	Title          string `gorm:"size:200" json:"title"`            // 评测标题（如 课文句子）
	TargetText     string `gorm:"type:text" json:"target_text"`     // 目标文本（要读的句子）
	RecognizedText string `gorm:"type:text" json:"recognized_text"` // 识别文本（用户实际说的，ASR）
	// 四维评分（0-100）
	Accuracy     int       `json:"accuracy"`                    // 准确度
	Fluency      int       `json:"fluency"`                     // 流利度
	Completeness int       `json:"completeness"`                // 完整度
	Intonation   int       `json:"intonation"`                  // 语调（语速稳定性近似）
	Score        int       `json:"score"`                       // 综合分
	Suggestion   string    `gorm:"type:text" json:"suggestion"` // AI 点评建议
	AudioURL     string    `gorm:"size:255" json:"audio_url"`   // 录音文件（/media/pronunciations/）
	DurationMs   int       `json:"duration_ms"`                 // 录音时长
	CreatedAt    time.Time `json:"created_at"`
}

// AIRole 对话中的 AI 角色
type AIRole struct {
	Role     string `json:"role"`
	RoleName string `json:"role_name"`
}

// DialogueSession AI 实景对话会话（学生扮演一角色，AI 扮演其余角色）
type DialogueSession struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	LessonID     uint      `json:"lesson_id"` // 关联课时（0=自由对话）
	Scene        string    `gorm:"size:100" json:"scene"`
	Title        string    `gorm:"size:200" json:"title"`
	UserRole     string    `gorm:"size:50" json:"user_role"`      // 学生扮演的角色
	UserRoleName string    `gorm:"size:50" json:"user_role_name"` // 中文名
	AICharacters string    `gorm:"type:text" json:"-"`            // JSON: []AIRole
	Status       string    `gorm:"size:20;default:active" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	AIList []AIRole `gorm:"-" json:"ai_roles"`
}

// DialogueMessage 对话消息
type DialogueMessage struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	SessionID   uint      `gorm:"index;not null" json:"session_id"`
	Speaker     string    `gorm:"size:50" json:"speaker"`      // user 或 AI 角色 role
	SpeakerName string    `gorm:"size:50" json:"speaker_name"` // 中文名
	IsUser      bool      `json:"is_user"`
	Content     string    `gorm:"type:text" json:"content"`
	AudioURL    string    `gorm:"size:255" json:"audio_url"`
	CreatedAt   time.Time `json:"created_at"`
}
