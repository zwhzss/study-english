package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
)

// truncateForLog 长文本截断，避免日志过大（含省略号标记）
func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + fmt.Sprintf("...[截断,总长%d]", len(runes))
}

// TextbookService 教材业务：目录初始化 + 单元内容拉取 + 查询
// reg 提供 LLM/TTS/绘图 Provider（配置保存后可动态重建）
type TextbookService struct {
	db      *gorm.DB
	reg     *ai.Registry
	dataDir string
	log     *slog.Logger
}

// svcLogger 包级 logger（测试通过 TestMain 替换为静默实现，避免日志刷屏）
var svcLogger = slog.New(slog.NewTextHandler(os.Stderr, nil))

func NewTextbookService(db *gorm.DB, reg *ai.Registry, dataDir string) *TextbookService {
	return &TextbookService{
		db:      db,
		reg:     reg,
		dataDir: dataDir,
		log:     svcLogger.With("svc", "textbook"),
	}
}

// CreateTextbook 创建教材并初始化内置目录（事务保证原子性）
func (s *TextbookService) CreateTextbook(name, publisher, stage, grade, catalogSource string) (*model.Textbook, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("教材名称不能为空")
	}
	if utf8.RuneCountInString(name) > 100 {
		return nil, errors.New("教材名称过长（最多 100 字）")
	}
	if catalogSource == "" {
		catalogSource = "builtin" // 默认内置模板
	}
	if catalogSource != "builtin" && catalogSource != "llm" {
		return nil, errors.New("catalog_source 仅支持 builtin / llm")
	}

	tb := &model.Textbook{
		Name: name, Publisher: publisher, Stage: stage, Grade: grade,
		Version: "v1", CatalogSource: catalogSource, Status: model.StatusPublished,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(tb).Error; err != nil {
			return err
		}
		if err := s.initCatalogTx(tx, tb.ID, name, stage, grade); err != nil {
			return err
		}
		// 新教材排序值 = ID（默认排最后）
		return tx.Model(tb).Update("sort_order", int(tb.ID)).Error
	})
	if err != nil {
		return nil, err
	}
	return tb, nil
}

// initCatalogTx 内置目录模板：预置主流教材的 年级→单元→课时 结构（事务内执行）
// 真实场景可按版本补充更多模板数据；此实现默认按名称关键词匹配，兜底生成通用目录
func (s *TextbookService) initCatalogTx(tx *gorm.DB, textbookID uint, name, stage, grade string) error {
	template := builtinTemplate(name, stage, grade)
	if len(template) == 0 {
		return nil
	}
	items := make([]model.CatalogItem, 0, len(template))
	for i, t := range template {
		items = append(items, model.CatalogItem{
			TextbookID: textbookID,
			Level:      t.Level,
			Unit:       t.Unit,
			Lesson:     t.Lesson,
			Title:      t.Title,
			Scene:      t.Scene,
			OrderNo:    i + 1,
			Status:     model.StatusPending,
		})
	}
	return tx.Create(&items).Error
}

type catalogTemplate struct {
	Level, Unit, Lesson, Title, Scene string
}

// builtinTemplate 内置目录模板（教材骨架先行，按 教材名+年级+册 精确匹配真实目录）
func builtinTemplate(name, stage, grade string) []catalogTemplate {
	// 冀教版（河北教育出版社）：带版本前缀组合匹配，优先于同名年级/册判断
	if contains(name, "冀教版") && contains(name, "三年级英语下册") {
		return jiji3DownTemplate()
	}
	if contains(name, "冀教版") && contains(name, "七年级英语上册") {
		return jiji7UpTemplate()
	}
	if contains(name, "冀教版") && contains(name, "七年级英语下册") {
		return jiji7DownTemplate()
	}
	if contains(name, "冀教版") && contains(name, "九年级英语全一册") {
		return jiji9Template()
	}
	if contains(name, "冀教版") && contains(name, "四年级英语上册") {
		return jiji4UpTemplate()
	}
	if contains(name, "冀教版") && contains(name, "四年级英语下册") {
		return jiji4DownTemplate()
	}
	if contains(name, "冀教版") && contains(name, "五年级英语上册") {
		return jiji5UpTemplate()
	}
	if contains(name, "冀教版") && contains(name, "五年级英语下册") {
		return jiji5DownTemplate()
	}
	if contains(name, "冀教版") && contains(name, "六年级英语上册") {
		return jiji6UpTemplate()
	}
	if contains(name, "冀教版") && contains(name, "六年级英语下册") {
		return jiji6DownTemplate()
	}
	if contains(name, "冀教版") && contains(name, "八年级英语上册") {
		return jiji8UpTemplate()
	}
	if contains(name, "冀教版") && contains(name, "八年级英语下册") {
		return jiji8DownTemplate()
	}
	// PEP 小学（三年级~六年级 × 上下册）精确匹配，杜绝上下册混用
	if contains(name, "三年级英语上册") {
		return pep3UpTemplate()
	}
	if contains(name, "三年级英语下册") {
		return pep3DownTemplate()
	}
	if contains(name, "四年级英语上册") {
		return pep4UpTemplate()
	}
	if contains(name, "四年级英语下册") {
		return pep4DownTemplate()
	}
	if contains(name, "五年级英语上册") {
		return pep5UpTemplate()
	}
	if contains(name, "五年级英语下册") {
		return pep5DownTemplate()
	}
	if contains(name, "六年级英语上册") {
		return pep6UpTemplate()
	}
	if contains(name, "六年级英语下册") {
		return pep6DownTemplate()
	}
	// 初中外研版（带版本前缀，优先于人教版同年级册判断）
	if contains(name, "外研版") && contains(name, "七年级英语下册") {
		return fts7DownTemplate()
	}
	if contains(name, "外研版") && contains(name, "八年级英语上册") {
		return fts8UpTemplate()
	}
	if contains(name, "外研版") && contains(name, "八年级英语下册") {
		return fts8DownTemplate()
	}
	if contains(name, "外研版") && contains(name, "九年级英语") {
		return fts9Template()
	}
	// 初中牛津译林（带版本前缀）
	if contains(name, "牛津") && contains(name, "七年级英语下册") {
		return oxford7DownTemplate()
	}
	if contains(name, "牛津") && contains(name, "八年级英语上册") {
		return oxford8UpTemplate()
	}
	if contains(name, "牛津") && contains(name, "八年级英语下册") {
		return oxford8DownTemplate()
	}
	if contains(name, "牛津") && contains(name, "九年级英语") {
		return oxford9Template()
	}
	// 高中外研版（带版本前缀，优先于人教版高中判断）
	if contains(name, "外研版") && contains(name, "必修一") {
		return ftsHigh1Template()
	}
	if contains(name, "外研版") && contains(name, "必修二") {
		return ftsHigh2Template()
	}
	if contains(name, "外研版") && contains(name, "必修三") {
		return ftsHigh3Template()
	}
	// 初中人教版（按 年级+册）
	if contains(name, "七年级英语下册") {
		return pep7DownTemplate()
	}
	if contains(name, "八年级英语上册") {
		return pep8UpTemplate()
	}
	if contains(name, "八年级英语下册") {
		return pep8DownTemplate()
	}
	if contains(name, "九年级英语全一册") || contains(name, "九年级英语上册") {
		return pep9Template()
	}
	// 高中人教版（必修/选择性必修）
	if contains(name, "高中英语必修一") {
		return pepHigh1Template()
	}
	if contains(name, "高中英语必修二") {
		return pepHigh2Template()
	}
	if contains(name, "高中英语必修三") {
		return pepHigh3Template()
	}
	if contains(name, "高中英语选择性必修一") {
		return pepHighSel1Template()
	}
	if contains(name, "高中英语选择性必修二") {
		return pepHighSel2Template()
	}
	if contains(name, "高中英语选择性必修三") {
		return pepHighSel3Template()
	}
	// 新概念 2-3 册
	if contains(name, "新概念英语第二册") {
		return nce2Template()
	}
	if contains(name, "新概念英语第三册") {
		return nce3Template()
	}
	if contains(name, "新概念") {
		return nceTemplate()
	}
	// 初中（七年级等）人教版/外研版/牛津
	if contains(name, "牛津") || contains(name, "译林") {
		return oxfordTemplate()
	}
	if contains(name, "人教版") || contains(name, "PEP") {
		return pepTemplate()
	}
	if contains(name, "外研版") {
		return ftsTemplate()
	}
	// 兜底：通用 3 单元目录
	return genericTemplate()
}

// ---- PEP 小学各年级真实目录（2024 新版：每单元 6 个，Part A/B 两个课时）----

func pep3UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade3", "Unit 1", "Part A", "Making Friends", "交朋友"},
		{"Grade3", "Unit 1", "Part B", "Hello! Hello!", "问候歌"},
		{"Grade3", "Unit 2", "Part A", "Different Families", "不同家庭"},
		{"Grade3", "Unit 2", "Part B", "This Is My Family", "介绍家人"},
		{"Grade3", "Unit 3", "Part A", "Amazing Animals", "神奇动物"},
		{"Grade3", "Unit 3", "Part B", "What Is It?", "询问动物"},
		{"Grade3", "Unit 4", "Part A", "Plants Around Us", "身边的植物"},
		{"Grade3", "Unit 4", "Part B", "I Like Flowers", "喜欢植物"},
		{"Grade3", "Unit 5", "Part A", "The Colourful World", "多彩世界"},
		{"Grade3", "Unit 5", "Part B", "What Colour Is It?", "颜色问答"},
		{"Grade3", "Unit 6", "Part A", "Useful Numbers", "有用的数字"},
		{"Grade3", "Unit 6", "Part B", "How Many?", "数量表达"},
	}
}

func pep3DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade3", "Unit 1", "Part A", "Meeting New People", "认识新朋友"},
		{"Grade3", "Unit 1", "Part B", "Who's That?", "询问是谁"},
		{"Grade3", "Unit 2", "Part A", "Expressing Yourself", "表达自我"},
		{"Grade3", "Unit 2", "Part B", "I Like to Play", "爱好表达"},
		{"Grade3", "Unit 3", "Part A", "Learning Better", "更好学习"},
		{"Grade3", "Unit 3", "Part B", "Let's Learn Together", "一起学习"},
		{"Grade3", "Unit 4", "Part A", "Healthy Food", "健康食物"},
		{"Grade3", "Unit 4", "Part B", "I Like Apples", "食物喜好"},
		{"Grade3", "Unit 5", "Part A", "Old Toys", "旧玩具"},
		{"Grade3", "Unit 5", "Part B", "My Favourite Toy", "最喜欢的玩具"},
		{"Grade3", "Unit 6", "Part A", "Numbers in Life", "生活中的数字"},
		{"Grade3", "Unit 6", "Part B", "Count with Me", "一起数数"},
	}
}

func pep4UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade4", "Unit 1", "Part A", "My Classroom", "我的教室"},
		{"Grade4", "Unit 1", "Part B", "Clean the Classroom", "打扫教室"},
		{"Grade4", "Unit 2", "Part A", "My Schoolbag", "我的书包"},
		{"Grade4", "Unit 2", "Part B", "What's in It?", "书包里有什么"},
		{"Grade4", "Unit 3", "Part A", "My Friends", "我的朋友"},
		{"Grade4", "Unit 3", "Part B", "Tall and Strong", "外貌描述"},
		{"Grade4", "Unit 4", "Part A", "My Home", "我的家"},
		{"Grade4", "Unit 4", "Part B", "Where Is It?", "位置询问"},
		{"Grade4", "Unit 5", "Part A", "Dinner's Ready", "晚餐好了"},
		{"Grade4", "Unit 5", "Part B", "What Would You Like?", "点餐"},
		{"Grade4", "Unit 6", "Part A", "Meet My Family", "认识我的家人"},
		{"Grade4", "Unit 6", "Part B", "This Is My Uncle", "介绍亲戚"},
	}
}

func pep4DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade4", "Unit 1", "Part A", "My School", "我的学校"},
		{"Grade4", "Unit 1", "Part B", "The Library Is on the First Floor", "校园楼层"},
		{"Grade4", "Unit 2", "Part A", "What Time Is It?", "现在几点"},
		{"Grade4", "Unit 2", "Part B", "Hurry Up!", "时间管理"},
		{"Grade4", "Unit 3", "Part A", "Weather", "天气"},
		{"Grade4", "Unit 3", "Part B", "Is It Sunny?", "天气问答"},
		{"Grade4", "Unit 4", "Part A", "At the Farm", "在农场"},
		{"Grade4", "Unit 4", "Part B", "These Are Tomatoes", "农场蔬菜"},
		{"Grade4", "Unit 5", "Part A", "My Clothes", "我的衣服"},
		{"Grade4", "Unit 5", "Part B", "Whose Coat Is This?", "衣物归属"},
		{"Grade4", "Unit 6", "Part A", "Shopping", "购物"},
		{"Grade4", "Unit 6", "Part B", "How Much Is It?", "问价格"},
	}
}

func pep5UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade5", "Unit 1", "Part A", "What's He Like?", "他是什么样"},
		{"Grade5", "Unit 1", "Part B", "My Teacher", "我的老师"},
		{"Grade5", "Unit 2", "Part A", "My Week", "我的一周"},
		{"Grade5", "Unit 2", "Part B", "What Do You Do on Sundays?", "周末活动"},
		{"Grade5", "Unit 3", "Part A", "What Would You Like?", "你想吃什么"},
		{"Grade5", "Unit 3", "Part B", "Favourite Food", "最喜欢的食物"},
		{"Grade5", "Unit 4", "Part A", "What Can You Do?", "你会做什么"},
		{"Grade5", "Unit 4", "Part B", "I Can Sing English Songs", "才艺表达"},
		{"Grade5", "Unit 5", "Part A", "There Is a Big Bed", "房间里有什么"},
		{"Grade5", "Unit 5", "Part B", "My Room", "我的房间"},
		{"Grade5", "Unit 6", "Part A", "In a Nature Park", "自然公园"},
		{"Grade5", "Unit 6", "Part B", "There Is a River", "公园景物"},
	}
}

func pep5DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade5", "Unit 1", "Part A", "My Day", "我的一天"},
		{"Grade5", "Unit 1", "Part B", "When Do You Get Up?", "作息问答"},
		{"Grade5", "Unit 2", "Part A", "My Favourite Season", "最喜欢的季节"},
		{"Grade5", "Unit 2", "Part B", "Why Do You Like Spring?", "喜欢季节的原因"},
		{"Grade5", "Unit 3", "Part A", "My School Calendar", "校历"},
		{"Grade5", "Unit 3", "Part B", "When Is the Sports Meet?", "活动日期"},
		{"Grade5", "Unit 4", "Part A", "When Is the Art Show?", "画展时间"},
		{"Grade5", "Unit 4", "Part B", "Special Days", "特别的日子"},
		{"Grade5", "Unit 5", "Part A", "Whose Dog Is It?", "这是谁的狗"},
		{"Grade5", "Unit 5", "Part B", "Look at the Dog", "描述宠物"},
		{"Grade5", "Unit 6", "Part A", "Work Quietly!", "安静工作"},
		{"Grade5", "Unit 6", "Part B", "Keep to the Right", "规则表达"},
	}
}

func pep6UpTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade6", "Unit 1", "Part A", "How Can I Get There?", "怎么去那里"},
		{"Grade6", "Unit 1", "Part B", "Turn Left at the Park", "问路指路"},
		{"Grade6", "Unit 2", "Part A", "Ways to Go to School", "上学方式"},
		{"Grade6", "Unit 2", "Part B", "By Bus or by Bike?", "交通方式"},
		{"Grade6", "Unit 3", "Part A", "My Weekend Plan", "周末计划"},
		{"Grade6", "Unit 3", "Part B", "What Are You Going to Do?", "计划问答"},
		{"Grade6", "Unit 4", "Part A", "I Have a Pen Pal", "我有笔友"},
		{"Grade6", "Unit 4", "Part B", "Does He Like Swimming?", "爱好询问"},
		{"Grade6", "Unit 5", "Part A", "What Does He Do?", "职业询问"},
		{"Grade6", "Unit 5", "Part B", "Where Does He Work?", "工作地点"},
		{"Grade6", "Unit 6", "Part A", "How Do You Feel?", "感受表达"},
		{"Grade6", "Unit 6", "Part B", "Don't Be Sad", "情绪安慰"},
	}
}

func pep6DownTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade6", "Unit 1", "Part A", "How Tall Are You?", "你多高"},
		{"Grade6", "Unit 1", "Part B", "I'm Taller Than You", "比较级"},
		{"Grade6", "Unit 2", "Part A", "Last Weekend", "上周末"},
		{"Grade6", "Unit 2", "Part B", "What Did You Do?", "过去时问答"},
		{"Grade6", "Unit 3", "Part A", "Where Did You Go?", "你去哪了"},
		{"Grade6", "Unit 3", "Part B", "My Holiday", "假期"},
		{"Grade6", "Unit 4", "Part A", "Then and Now", "过去与现在"},
		{"Grade6", "Unit 4", "Part B", "Changes in Life", "生活变化"},
	}
}

// pepTemplate 人教版（PEP）七年级英语上册 真实目录（2024 新版：Starter 1-3 + Unit 1-7，每单元 Section A/B）
func pepTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Starter1", "Starter Unit 1", "Section A", "Hello!", "日常问候"},
		{"Starter1", "Starter Unit 1", "Section B", "Letters and Names", "字母与名字"},
		{"Starter2", "Starter Unit 2", "Section A", "What's your name?", "询问姓名"},
		{"Starter2", "Starter Unit 2", "Section B", "Numbers 0-9", "数字表达"},
		{"Starter3", "Starter Unit 3", "Section A", "How old are you?", "询问年龄"},
		{"Starter3", "Starter Unit 3", "Section B", "Personal Information", "个人信息"},
		{"Grade7", "Unit 1", "Section A", "You and Me", "自我介绍"},
		{"Grade7", "Unit 1", "Section B", "Introduce Others", "介绍他人"},
		{"Grade7", "Unit 2", "Section A", "We're Family!", "家庭成员"},
		{"Grade7", "Unit 2", "Section B", "Describe Your Family", "描述家人"},
		{"Grade7", "Unit 3", "Section A", "My School", "校园设施"},
		{"Grade7", "Unit 3", "Section B", "Ask for Directions", "问路指路"},
		{"Grade7", "Unit 4", "Section A", "My Favourite Subject", "课程科目"},
		{"Grade7", "Unit 4", "Section B", "Reasons and Feelings", "喜好理由"},
		{"Grade7", "Unit 5", "Section A", "Fun Clubs", "社团活动"},
		{"Grade7", "Unit 5", "Section B", "Join a Club", "报名社团"},
		{"Grade7", "Unit 6", "Section A", "A Day in the Life", "日常作息"},
		{"Grade7", "Unit 6", "Section B", "Telling the Time", "时间表达"},
		{"Grade7", "Unit 7", "Section A", "Happy Birthday", "生日庆祝"},
		{"Grade7", "Unit 7", "Section B", "Dates and Months", "日期月份"},
	}
}

// ftsTemplate 外研版（新标准）七年级英语上册 真实目录：Module 1-10 × Unit 1-2
func ftsTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade7", "Module 1", "Unit 1", "Nice to meet you", "自我介绍"},
		{"Grade7", "Module 1", "Unit 2", "I'm Wang Lingling", "介绍他人"},
		{"Grade7", "Module 2", "Unit 1", "Is this your mum?", "家庭成员"},
		{"Grade7", "Module 2", "Unit 2", "These are my parents", "描述家人"},
		{"Grade7", "Module 3", "Unit 1", "There are thirty students in my class", "班级描述"},
		{"Grade7", "Module 3", "Unit 2", "The library is on the left", "校园方位"},
		{"Grade7", "Module 4", "Unit 1", "We've got lots of apples", "食物表达"},
		{"Grade7", "Module 4", "Unit 2", "Is your food and drink healthy?", "健康饮食"},
		{"Grade7", "Module 5", "Unit 1", "I love history", "学校课程"},
		{"Grade7", "Module 5", "Unit 2", "We start work at nine o'clock", "日常作息"},
		{"Grade7", "Module 6", "Unit 1", "Does it eat meat?", "动物描述"},
		{"Grade7", "Module 6", "Unit 2", "The tiger lives in Asia", "动物栖息地"},
		{"Grade7", "Module 7", "Unit 1", "How do I write my homework on the computer?", "电脑操作"},
		{"Grade7", "Module 7", "Unit 2", "When do you use a computer?", "使用电脑"},
		{"Grade7", "Module 8", "Unit 1", "I always like birthday parties", "生日派对"},
		{"Grade7", "Module 8", "Unit 2", "She often goes to concerts", "兴趣活动"},
		{"Grade7", "Module 9", "Unit 1", "We're enjoying the school trip a lot", "学校旅行"},
		{"Grade7", "Module 9", "Unit 2", "They're waiting for buses or trains", "进行时描述"},
		{"Grade7", "Module 10", "Unit 1", "Are you getting ready for Spring Festival?", "春节准备"},
		{"Grade7", "Module 10", "Unit 2", "My mother's cleaning our house", "节日活动"},
	}
}

// oxfordTemplate 牛津译林版七年级英语上册 真实目录：Starter 1-4 + Unit 1-8
func oxfordTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Starter", "Starter Unit 1", "Period 1", "Hello", "日常问候"},
		{"Starter", "Starter Unit 2", "Period 1", "I'm Liu Tao", "自我介绍"},
		{"Starter", "Starter Unit 3", "Period 1", "How old are you?", "询问年龄"},
		{"Starter", "Starter Unit 4", "Period 1", "My family", "家庭成员"},
		{"Grade7", "Unit 1", "Reading", "This is me!", "自我介绍"},
		{"Grade7", "Unit 1", "Task", "Introduce Yourself", "介绍自己写作"},
		{"Grade7", "Unit 2", "Reading", "Let's play sports!", "运动爱好"},
		{"Grade7", "Unit 2", "Task", "My Favourite Sport", "描述运动"},
		{"Grade7", "Unit 3", "Reading", "Welcome to our school!", "校园介绍"},
		{"Grade7", "Unit 3", "Task", "Our School", "介绍学校"},
		{"Grade7", "Unit 4", "Reading", "My day", "一天安排"},
		{"Grade7", "Unit 4", "Task", "A Day in My Life", "描述作息"},
		{"Grade7", "Unit 5", "Reading", "Let's celebrate!", "节日庆祝"},
		{"Grade7", "Unit 5", "Task", "My Favourite Festival", "描述节日"},
		{"Grade7", "Unit 6", "Reading", "Food and lifestyle", "饮食生活"},
		{"Grade7", "Unit 6", "Task", "Healthy Living", "健康生活"},
		{"Grade7", "Unit 7", "Reading", "Shopping", "购物"},
		{"Grade7", "Unit 7", "Task", "Shopping List", "购物清单"},
		{"Grade7", "Unit 8", "Reading", "Fashion", "时尚服饰"},
		{"Grade7", "Unit 8", "Task", "My Favourite Clothes", "描述服饰"},
	}
}

// nceTemplate 新概念英语第一册 真实目录（Lesson 1-16，可按需扩展至 144 课）
func nceTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Book1", "Lesson 1", "Lesson 1", "Excuse me!", "道歉与回应"},
		{"Book1", "Lesson 2", "Lesson 2", "Is this your...?", "确认物品"},
		{"Book1", "Lesson 3", "Lesson 3", "Sorry, sir.", "抱歉"},
		{"Book1", "Lesson 4", "Lesson 4", "Is this your pen?", "物主询问"},
		{"Book1", "Lesson 5", "Lesson 5", "Nice to meet you", "见面问候"},
		{"Book1", "Lesson 6", "Lesson 6", "What make is it?", "询问品牌"},
		{"Book1", "Lesson 7", "Lesson 7", "Are you a teacher?", "职业询问"},
		{"Book1", "Lesson 8", "Lesson 8", "What's your job?", "工作问答"},
		{"Book1", "Lesson 9", "Lesson 9", "How are you today?", "近况问候"},
		{"Book1", "Lesson 10", "Lesson 10", "Look at...", "描述物品"},
		{"Book1", "Lesson 11", "Lesson 11", "Is this your shirt?", "衣物物主"},
		{"Book1", "Lesson 12", "Lesson 12", "Whose is this?", "归属询问"},
		{"Book1", "Lesson 13", "Lesson 13", "A new dress", "颜色描述"},
		{"Book1", "Lesson 14", "Lesson 14", "What colour is your...?", "颜色问答"},
		{"Book1", "Lesson 15", "Lesson 15", "Your passports, please.", "出示证件"},
		{"Book1", "Lesson 16", "Lesson 16", "Are you...?", "国籍询问"},
	}
}

func genericTemplate() []catalogTemplate {
	return []catalogTemplate{
		{"Grade7", "Unit 1", "Lesson 1", "Daily Greetings", "日常问候"},
		{"Grade7", "Unit 1", "Lesson 2", "Introduce Myself", "自我介绍"},
		{"Grade7", "Unit 2", "Lesson 1", "At the Restaurant", "餐厅点餐"},
		{"Grade7", "Unit 2", "Lesson 2", "Ordering Food", "点餐练习"},
		{"Grade7", "Unit 3", "Lesson 1", "At the Supermarket", "超市购物"},
		{"Grade7", "Unit 3", "Lesson 2", "At the Airport", "机场出行"},
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ListTextbooks 教材列表
// CountTextbooks 教材总数（启动自检用）
func (s *TextbookService) CountTextbooks(cnt *int64) error {
	return s.db.Model(&model.Textbook{}).Count(cnt).Error
}

func (s *TextbookService) ListTextbooks() ([]model.Textbook, error) {
	var tbs []model.Textbook
	// 按 sort_order 排序（sort_order=0 的历史教材视为创建顺序）
	if err := s.db.Order("sort_order, id").Find(&tbs).Error; err != nil {
		return nil, err
	}
	return tbs, nil
}

// CatalogNode 目录树节点（年级 → 单元 → 课时）
type CatalogNode struct {
	Level   string          `json:"level"`
	Unit    string          `json:"unit"`
	Title   string          `json:"title"`
	Scene   string          `json:"scene"`
	Lessons []CatalogLesson `json:"lessons"`
}

// CatalogLesson 课时
type CatalogLesson struct {
	ID         uint   `json:"id"`
	Lesson     string `json:"lesson"`
	Title      string `json:"title"`
	Scene      string `json:"scene"`
	Status     string `json:"status"`
	HasContent bool   `json:"has_content"`
}

// GetCatalogTree 返回 年级→单元→课时 目录树
func (s *TextbookService) GetCatalogTree(textbookID uint) ([]CatalogNode, error) {
	var tb model.Textbook
	if err := s.db.First(&tb, textbookID).Error; err != nil {
		return nil, errors.New("教材不存在")
	}
	var items []model.CatalogItem
	if err := s.db.Where("textbook_id = ?", textbookID).Order("order_no").Find(&items).Error; err != nil {
		return nil, err
	}

	var nodes []CatalogNode
	unitIdx := map[string]int{}
	for _, it := range items {
		key := it.Level + "|" + it.Unit
		idx, ok := unitIdx[key]
		if !ok {
			nodes = append(nodes, CatalogNode{Level: it.Level, Unit: it.Unit, Title: it.Unit})
			idx = len(nodes) - 1
			unitIdx[key] = idx
		}
		hasContent := false
		var cnt int64
		s.db.Model(&model.AIContent{}).Where("catalog_item_id = ?", it.ID).Count(&cnt)
		if cnt == 0 {
			// AI 内容无，再看导入内容（真实课文导入也算有内容）
			s.db.Model(&model.ImportedContent{}).Where("catalog_item_id = ?", it.ID).Count(&cnt)
		}
		hasContent = cnt > 0
		nodes[idx].Lessons = append(nodes[idx].Lessons, CatalogLesson{
			ID: it.ID, Lesson: it.Lesson, Title: it.Title, Scene: it.Scene,
			Status: it.Status, HasContent: hasContent,
		})
	}
	return nodes, nil
}

// ReorderCatalog 调整教材目录顺序（课时级）：ids 为该教材所有课时的新顺序
func (s *TextbookService) ReorderCatalog(textbookID uint, ids []uint) error {
	if len(ids) == 0 {
		return errors.New("课时列表不能为空")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 校验 ids 都属于该教材
		var tb model.Textbook
		if err := tx.First(&tb, textbookID).Error; err != nil {
			return errors.New("教材不存在")
		}
		var count int64
		tx.Model(&model.CatalogItem{}).Where("textbook_id = ? AND id IN ?", textbookID, ids).Count(&count)
		if int(count) != len(ids) {
			return errors.New("课时列表与该教材目录不一致，请刷新后重试")
		}
		for i, id := range ids {
			if err := tx.Model(&model.CatalogItem{}).Where("id = ?", id).Update("order_no", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateCatalogItem 新增课时（unit 新值即新单元）。Level 沿用该教材已有课时，OrderNo 排末尾
func (s *TextbookService) CreateCatalogItem(textbookID uint, unit, lesson, title, scene string) (*model.CatalogItem, error) {
	unit = strings.TrimSpace(unit)
	title = strings.TrimSpace(title)
	if unit == "" || title == "" {
		return nil, errors.New("单元名和课文标题不能为空")
	}
	level := "Grade1"
	var sample model.CatalogItem
	if err := s.db.Where("textbook_id = ?", textbookID).Order("id").First(&sample).Error; err == nil && sample.Level != "" {
		level = sample.Level
	}
	var maxNo int
	s.db.Model(&model.CatalogItem{}).Where("textbook_id = ?", textbookID).Select("COALESCE(MAX(order_no),0)").Scan(&maxNo)
	item := &model.CatalogItem{
		TextbookID: textbookID, Level: level, Unit: unit,
		Lesson: strings.TrimSpace(lesson), Title: title, Scene: scene,
		OrderNo: maxNo + 1, Status: model.StatusPending,
	}
	if err := s.db.Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateCatalogItem 编辑课时（unit/lesson/title/scene，可移动到别的单元）
func (s *TextbookService) UpdateCatalogItem(id uint, unit, lesson, title, scene string) error {
	updates := map[string]any{}
	if unit != "" {
		updates["unit"] = strings.TrimSpace(unit)
	}
	if lesson != "" {
		updates["lesson"] = strings.TrimSpace(lesson)
	}
	if title != "" {
		updates["title"] = strings.TrimSpace(title)
	}
	if scene != "" {
		updates["scene"] = strings.TrimSpace(scene)
	}
	if len(updates) == 0 {
		return errors.New("没有要修改的内容")
	}
	return s.db.Model(&model.CatalogItem{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteCatalogItem 删除课时（连带其 AI 内容 / 导入内容 / 相关对话）
func (s *TextbookService) DeleteCatalogItem(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("catalog_item_id = ?", id).Delete(&model.AIContent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("catalog_item_id = ?", id).Delete(&model.ImportedContent{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CatalogItem{}, id).Error
	})
}

// RenameUnit 重命名单元：批量改该教材下所有同 Unit 字段的课时
func (s *TextbookService) RenameUnit(textbookID uint, oldUnit, newUnit string) error {
	oldUnit = strings.TrimSpace(oldUnit)
	newUnit = strings.TrimSpace(newUnit)
	if oldUnit == "" || newUnit == "" {
		return errors.New("单元名不能为空")
	}
	return s.db.Model(&model.CatalogItem{}).Where("textbook_id = ? AND unit = ?", textbookID, oldUnit).Update("unit", newUnit).Error
}

// DeleteUnit 删除单元：连带该单元下所有课时及其内容
func (s *TextbookService) DeleteUnit(textbookID uint, unit string) error {
	var ids []uint
	if err := s.db.Model(&model.CatalogItem{}).Where("textbook_id = ? AND unit = ?", textbookID, unit).Pluck("id", &ids).Error; err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			_ = tx.Where("catalog_item_id = ?", id).Delete(&model.AIContent{}).Error
			_ = tx.Where("catalog_item_id = ?", id).Delete(&model.ImportedContent{}).Error
		}
		if len(ids) > 0 {
			if err := tx.Where("id IN ?", ids).Delete(&model.CatalogItem{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// TranscribeASR 语音识别：音频 → 文本（registry 的 ASR Provider，未配置 Key 时 mock 占位）
func (s *TextbookService) TranscribeASR(ctx context.Context, audio []byte, format string) (string, error) {
	return s.reg.ASR().Transcribe(ctx, audio, format)
}

// DeleteTextbook 删除教材（事务）：连带目录项、AI 内容、导入内容、相关对话会话
func (s *TextbookService) DeleteTextbook(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var tb model.Textbook
		if err := tx.First(&tb, id).Error; err != nil {
			return errors.New("教材不存在")
		}
		var items []model.CatalogItem
		tx.Where("textbook_id = ?", id).Find(&items)
		ids := make([]uint, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		if len(ids) > 0 {
			tx.Where("catalog_item_id IN ?", ids).Delete(&model.AIContent{})
			tx.Where("catalog_item_id IN ?", ids).Delete(&model.ImportedContent{})
			var sessions []model.DialogueSession
			tx.Where("lesson_id IN ?", ids).Find(&sessions)
			sids := make([]uint, 0, len(sessions))
			for _, se := range sessions {
				sids = append(sids, se.ID)
			}
			if len(sids) > 0 {
				tx.Where("session_id IN ?", sids).Delete(&model.DialogueMessage{})
				tx.Where("id IN ?", sids).Delete(&model.DialogueSession{})
			}
			tx.Where("textbook_id = ?", id).Delete(&model.CatalogItem{})
		}
		return tx.Delete(&tb).Error
	})
}

// lessonPayload LLM 生成内容的 JSON Schema（与 model 结构对应）
// flexibleString 容错字符串：LLM 偶发把本应字符串的字段输出成数字（如 "unit":1），自动转字符串
type flexibleString string

func (f *flexibleString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) >= 2 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexibleString(s)
		return nil
	}
	*f = flexibleString(string(data))
	return nil
}

type lessonPayload struct {
	Unit          flexibleString       `json:"unit"`
	Title         flexibleString       `json:"title"`
	Scene         flexibleString       `json:"scene"`
	Dialogue      []model.DialogueTurn `json:"dialogue"`
	KeyVocab      []model.VocabItem    `json:"key_vocab"`
	GrammarPoints []model.GrammarPoint `json:"grammar_points"`
	Practice      []model.PracticeItem `json:"practice"`
	Images        []string             `json:"images"`
	ImagePrompts  []string             `json:"image_prompts"` // 每张配图的英文绘图描述（用于 AI 绘图）
	SVGs          []model.SVGImage     `json:"svg_images"`    // 简单示意图（SVG 源码，前端零成本渲染）
}

const contentSystemPrompt = `你是一位专业的少儿英语教材编写专家。请为一个真实生活场景编写一节英语课的内容，要求：
1. 教学理念：能力型培养，非应试刷题。全部内容围绕真实日常沟通场景展开。
2. 语言难度：面向初中生（简单词汇、简短句式），地道且自然。
3. 对话要求：3-7 轮多角色真实对话（角色 role 用英文标识，role_name 用中文），模拟真实沟通节奏，包含开场白、请求、回应、结束语。**角色必须多样化（2-4 个不同角色），role 字段必须且只能从以下角色库中挑选（禁止自创名字如 alex/mia/student1，禁止编号如 student1/student2）**：boy(男孩)、girl(女孩)、student(学生)、teacher(教师)、male_teacher(男教师)、grandpa(爷爷)、grandma(奶奶)、brother(哥哥)、sister(姐姐)、uncle(叔叔)、aunt(阿姨)、mother(妈妈)、father(爸爸)、waiter(服务员)、customer(顾客)、clerk(店员)、driver(司机)、farmer(农民)、worker(工人)、doctor(医生)、nurse(护士)、police(警察)、friend(朋友)。同一段对话中不同说话者必须使用不同的 role（系统会为每个 role 分配不同的语音音色），不要所有对话都用同一个角色。
4. 词汇：5 个左右本课核心词，含音标（IPA）、中文释义、例句。
5. 语法：2 个左右高频语法点/地道表达，含解释与例句。
6. 练习：2-3 个能力型练习（type 为 roleplay/speak/write），无选择题，含示例与提示。
7. 配图：2 张真实场景相关图片 URL（使用 https://images.unsplash.com 的稳定图片地址），并为每张图提供 1 条英文绘图描述 image_prompts（描述场景主体、环境、光线、风格，适合 AI 绘图，如 "a cozy restaurant interior with a waiter taking orders from a customer, warm lighting, realistic illustration style"）。images 与 image_prompts 一一对应。
8. 示意图（可选但推荐）：生成 1-2 张简单 SVG 示意图 svg_images（如场景布局图、单词分类图、对话流程箭头图），每张含 title 和 svg 字段。SVG 要求：viewBox 或宽高明确（建议 400x300 内）、用纯矢量元素（rect/circle/path/text/line）、颜色简洁、可内嵌英文标注；**禁止** script、foreignObject、on* 事件属性、外部链接（href 只能为普通链接）等危险元素。
9. 严格输出合法 JSON，不要输出任何其他文字，不要用 markdown 代码块包裹。字段类型必须严格遵守，尤其注意：
   - **unit、title、scene 必须是字符串**（如 "unit": "Unit 1"），**绝不能是数字**（禁止 "unit": 1，必须 "unit": "Unit 1"）；
   - **role 必须从角色库取值**（boy/girl/student/teacher/male_teacher/grandpa/grandma/brother/sister/uncle/aunt/mother/father/waiter/customer/clerk/driver/farmer/worker/doctor/nurse/police/friend），禁止自创名字（如 alex/mia）或编号（如 student1）；
   - hints 必须是字符串数组（如 "hints": ["提示1", "提示2"]），绝不能输出单个字符串；
   - dialogue、key_vocab、grammar_points、practice、images、image_prompts、svg_images 都必须是 JSON 数组；
   - 字符串内的双引号必须用 \" 转义；禁止未转义的换行符、制表符、控制字符；禁止尾逗号和注释；
   - 输出前请自行检查 JSON 语法完全合法、字段类型正确。
   JSON 字段：unit(字符串), title(字符串), scene(字符串), dialogue(role/role_name/text/image), key_vocab(word/phonetic/meaning/example), grammar_points(point/explanation/example), practice(type/prompt/example/hints数组), images(string数组), image_prompts(string数组), svg_images(title/svg数组)。`

// PullLessonContent 按 单元（目录项）从大模型拉取教材内容并入库
// force=true 时强制重新生成（跳过缓存并覆盖旧内容）；默认命中缓存直接返回
func (s *TextbookService) PullLessonContent(ctx context.Context, catalogItemID uint, force bool) (*model.AIContent, error) {
	var item model.CatalogItem
	if err := s.db.First(&item, catalogItemID).Error; err != nil {
		return nil, fmt.Errorf("目录项不存在: %w", err)
	}
	// 已有 AI 创作内容且非强制 → 直接返回（缓存，不重复调 LLM；兼容旧数据 kind 为空）
	if !force {
		var existing model.AIContent
		if err := s.db.Where("catalog_item_id = ?", item.ID).First(&existing).Error; err == nil &&
			(existing.Status == model.StatusPublished || existing.Status == model.StatusDraft) {
			return s.assembleAIContent(&existing)
		}
	}

	user := fmt.Sprintf("教材单元：%s %s（场景：%s）。请按系统要求生成该课时完整内容。", item.Unit, item.Title, item.Scene)
	// 未配置大模型（非显式选择 Mock）时给出明确提示
	if _, isMock := s.reg.LLM().(*ai.MockLLM); isMock && !s.reg.MockExplicit() {
		return nil, errors.New("尚未配置大模型 API Key，请到「⚙️ 模型配置」页面设置后重试")
	}
	// 日志：记录拉取提示信息，便于调试
	s.log.Info("LLM 拉取课时内容",
		"catalog_item_id", item.ID,
		"unit", item.Unit,
		"title", item.Title,
		"system_prompt", truncateForLog(contentSystemPrompt, 2000),
		"user_prompt", user,
	)
	raw, err := s.reg.LLM().ChatJSON(ctx, contentSystemPrompt, user)
	if err != nil {
		s.log.Error("LLM 拉取失败", "catalog_item_id", item.ID, "error", err.Error())
		return nil, fmt.Errorf("大模型生成失败: %w", err)
	}
	// 日志：记录模型返回的原始数据（便于调试解析问题）
	s.log.Info("LLM 拉取结果",
		"catalog_item_id", item.ID,
		"raw_len", len(raw),
		"raw_preview", truncateForLog(raw, 4000),
	)
	var p lessonPayload
	if err := ai.ParseJSONPayload(raw, &p); err != nil {
		s.log.Error("LLM 结果解析失败", "catalog_item_id", item.ID, "error", err.Error())
		return nil, err
	}
	if len(p.Dialogue) == 0 {
		return nil, errors.New("模型输出缺少对话内容")
	}

	// 强制重新生成：删除旧内容（唯一索引限制，先删后建）
	if force {
		if err := s.db.Where("catalog_item_id = ?", item.ID).Delete(&model.AIContent{}).Error; err != nil {
			return nil, fmt.Errorf("删除旧内容失败: %w", err)
		}
	}

	// 合成对话/词汇音频并持久化到 JSON（audio_url 入库，此后朗读零成本本地播放，不再调 TTS）
	s.synthesizeAndPersistAudio(&p)

	content := &model.AIContent{
		CatalogItemID: item.ID,
		Unit:          item.Unit,
		Title:         string(p.Title),
		Scene:         string(p.Scene),
		DialogueJSON:  mustJSON(p.Dialogue),
		KeyVocabJSON:  mustJSON(p.KeyVocab),
		GrammarJSON:   mustJSON(p.GrammarPoints),
		PracticeJSON:  mustJSON(p.Practice),
		ImagesJSON:    mustJSON(p.Images),
		SVGsJSON:      mustJSON(sanitizeSVGs(p.SVGs)),
		Source:        "llm",
		Status:        model.StatusPublished,
	}
	// 并发防护：唯一索引冲突时（另一请求已生成）不报错，回查已有内容
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "catalog_item_id"}},
		DoNothing: true,
	}).Create(content).Error; err != nil {
		return nil, err
	}
	if content.ID == 0 {
		if err := s.db.Where("catalog_item_id = ?", item.ID).First(content).Error; err != nil {
			return nil, fmt.Errorf("并发拉取后回查失败: %w", err)
		}
		return s.assembleAIContent(content)
	}

	// AI 绘图生成场景配图（未配置 IMAGE_API_KEY 时回退模型返回的 images）
	p.Images = s.generateImages(ctx, p.Images, p.ImagePrompts)
	content.ImagesJSON = mustJSON(p.Images)
	if err := s.db.Model(content).Update("images_json", content.ImagesJSON).Error; err != nil {
		return nil, fmt.Errorf("更新配图失败: %w", err)
	}

	// 更新目录项状态（忽略"已发布"幂等场景；错误则返回）
	if err := s.db.Model(&item).Update("status", model.StatusPublished).Error; err != nil {
		return nil, fmt.Errorf("更新目录状态失败: %w", err)
	}

	return s.assembleAIContent(content)
}

// ReorderTextbooks 调整教材库顺序（ids 为新顺序）
func (s *TextbookService) ReorderTextbooks(ids []uint) error {
	if len(ids) == 0 {
		return errors.New("教材列表不能为空")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&model.Textbook{}).Where("id IN ?", ids).Count(&count)
		if int(count) != len(ids) {
			return errors.New("教材列表与数据库不一致，请刷新后重试")
		}
		for i, id := range ids {
			if err := tx.Model(&model.Textbook{}).Where("id = ?", id).Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetLessonContent 查询课时内容（kind=ai 或 imported，前端 Tab 切换）
func (s *TextbookService) GetLessonContent(ctx context.Context, catalogItemID uint, kind string) (any, error) {
	if kind == "imported" {
		var content model.ImportedContent
		if err := s.db.Where("catalog_item_id = ?", catalogItemID).First(&content).Error; err != nil {
			return nil, fmt.Errorf("该课时尚未导入课文，请先导入: %w", err)
		}
		if content.DialogueJSON != "" {
			_ = json.Unmarshal([]byte(content.DialogueJSON), &content.Dialogue)
		}
		return &content, nil
	}
	// 默认 AI 创作
	var content model.AIContent
	if err := s.db.Where("catalog_item_id = ?", catalogItemID).First(&content).Error; err != nil {
		return nil, fmt.Errorf("该课时 AI 内容尚未生成，请先拉取: %w", err)
	}
	return s.assembleAIContent(&content)
}

// generateImages 用 AI 绘图生成配图：逐个提示词生成，成功替换、失败保留原 URL
func (s *TextbookService) generateImages(ctx context.Context, fallback []string, prompts []string) []string {
	if len(prompts) == 0 {
		return fallback
	}
	result := make([]string, 0, len(prompts))
	for i, prompt := range prompts {
		url, err := s.reg.Image().Generate(ctx, prompt)
		if err == nil && url != "" {
			// AI 绘图返回的多为临时外链（如硅基流动 s3 temporary，几小时后过期）→ 下载到本地持久化
			result = append(result, s.persistImage(ctx, url))
		} else if i < len(fallback) && fallback[i] != "" {
			// 回退：模型给出的配图 URL（unsplash 等外链）→ 同样下载到本地（离线可用 + 统一迁移）
			result = append(result, s.persistImage(ctx, fallback[i]))
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

// persistImage 把 AI 绘图返回的图片下载到 data/images/ 本地持久化（/media/images/{urlHash}.png），
// 失败则保留原 URL（外链仍可临时显示）。按 URL hash 命名实现全局去重（多课时引用同一图不重复下载）
func (s *TextbookService) persistImage(ctx context.Context, url string) string {
	if !strings.HasPrefix(url, "http") {
		return url
	}
	hash := sha1.Sum([]byte(url))
	name := hex.EncodeToString(hash[:10]) + ".png"
	dir := filepath.Join(s.dataDir, "images")
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return "/media/images/" + name // 已下载过
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return url
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return url
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return url
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 上限 8MB
	if err != nil || len(data) < 100 {
		return url
	}
	ext := "png"
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		switch {
		case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
			ext = "jpg"
		case strings.Contains(ct, "webp"):
			ext = "webp"
		case strings.Contains(ct, "gif"):
			ext = "gif"
		}
	}
	if ext != "png" {
		name = hex.EncodeToString(hash[:10]) + "." + ext
		path = filepath.Join(dir, name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return url
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return url
	}
	return "/media/images/" + name
}

// AudioBackfillResult 补合成缺失音频结果
type AudioBackfillResult struct {
	Lessons     int `json:"lessons"`     // 修复的课时数
	Synthesized int `json:"synthesized"` // 补合成的音频句数
}

// BackfillMissingAudio 批量补合成历史课文中 audio_url 为空的对话/词汇（旧数据 TTS 未生成时产生）
// 课时间串行、课时内并发（EdgeTTS 信号量限流），避免并发爆炸
func (s *TextbookService) BackfillMissingAudio() (*AudioBackfillResult, error) {
	var contents []model.AIContent
	if err := s.db.Find(&contents).Error; err != nil {
		return nil, err
	}
	result := &AudioBackfillResult{}
	for i := range contents {
		c := &contents[i]
		var d []model.DialogueTurn
		var v []model.VocabItem
		if c.DialogueJSON != "" {
			_ = json.Unmarshal([]byte(c.DialogueJSON), &d)
		}
		if c.KeyVocabJSON != "" {
			_ = json.Unmarshal([]byte(c.KeyVocabJSON), &v)
		}
		changed := false
		var wg sync.WaitGroup
		synth := func(text, role string, set func(url string)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				url, err := s.reg.TTS().Synthesize(text, role)
				if err == nil && url != "" {
					set(url)
				}
			}()
		}
		for j := range d {
			if d[j].AudioURL == "" {
				idx := j
				synth(d[j].Text, d[j].Role, func(url string) { d[idx].AudioURL = url })
			}
		}
		for j := range v {
			if v[j].AudioURL == "" {
				idx := j
				synth(v[j].Example, "vocab", func(url string) { v[idx].AudioURL = url })
			}
		}
		wg.Wait()
		// 统计实际补上的
		for _, t := range d {
			if t.AudioURL != "" {
				changed = true
				result.Synthesized++
			}
		}
		for _, t := range v {
			if t.AudioURL != "" {
				changed = true
				result.Synthesized++
			}
		}
		if changed {
			if b, err := json.Marshal(d); err == nil {
				s.db.Model(c).Update("dialogue_json", string(b))
			}
			if b, err := json.Marshal(v); err == nil {
				s.db.Model(c).Update("key_vocab_json", string(b))
			}
			result.Lessons++
		}
	}
	return result, nil
}

type ImageMigrateResult struct {
	Lessons       int               `json:"lessons"`                  // 修改的课时数
	Downloaded    int               `json:"downloaded"`               // 成功下载的图片数
	Failed        int               `json:"failed"`                   // 下载失败的图片数（保留原外链）
	FailedDetails []ImageFailDetail `json:"failed_details,omitempty"` // 失败明细（定位需重新生成的课时）
}

// ImageFailDetail 单张图片下载失败明细
type ImageFailDetail struct {
	LessonID    uint   `json:"lesson_id"`
	LessonTitle string `json:"lesson_title"`
	URL         string `json:"url"`
}

// MigrateExternalImages 把历史课文里的外链图片（硅基流动 s3 临时 URL / unsplash）批量下载到本地
func (s *TextbookService) MigrateExternalImages() (*ImageMigrateResult, error) {
	var contents []model.AIContent
	if err := s.db.Find(&contents).Error; err != nil {
		return nil, err
	}
	result := &ImageMigrateResult{}
	ctx := context.Background()
	for i := range contents {
		c := &contents[i]
		var imgs []string
		if err := json.Unmarshal([]byte(c.ImagesJSON), &imgs); err != nil || len(imgs) == 0 {
			continue
		}
		changed := false
		for j, img := range imgs {
			if !strings.HasPrefix(img, "http") {
				continue
			}
			local := s.persistImage(ctx, img)
			if strings.HasPrefix(local, "/media/") {
				imgs[j] = local
				changed = true
				result.Downloaded++
			} else {
				result.Failed++
				result.FailedDetails = append(result.FailedDetails, ImageFailDetail{
					LessonID: c.CatalogItemID, LessonTitle: c.Title, URL: img,
				})
			}
		}
		if changed {
			b, _ := json.Marshal(imgs)
			if err := s.db.Model(c).Update("images_json", string(b)).Error; err != nil {
				continue
			}
			result.Lessons++
		}
	}
	return result, nil
}

// synthesizeAndPersistAudio 为对话/词汇合成 TTS 音频并写入 payload（audio_url 入库持久化）
func (s *TextbookService) synthesizeAndPersistAudio(p *lessonPayload) {
	// 并发合成：对话每句 + 词汇例句并行调用（EdgeTTS 内部信号量限流最多 2 进程），
	// 7 轮对话 + 5 词汇从 ~24s 串行缩短到 ~3s
	type job struct {
		idx    int
		text   string
		role   string
		target func(i int, url string)
	}
	var jobs []job
	for i := range p.Dialogue {
		idx := i
		jobs = append(jobs, job{idx: idx, text: p.Dialogue[i].Text, role: p.Dialogue[i].Role, target: func(i int, url string) { p.Dialogue[i].AudioURL = url }})
	}
	for i := range p.KeyVocab {
		idx := i
		jobs = append(jobs, job{idx: idx, text: p.KeyVocab[i].Example, role: "vocab", target: func(i int, url string) { p.KeyVocab[i].AudioURL = url }})
	}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			url, err := s.reg.TTS().Synthesize(j.text, j.role)
			if err == nil {
				j.target(j.idx, url)
			}
		}(j)
	}
	wg.Wait()
}

// assembleContent 将 JSON 字段反序列化为前端结构并附加角色音色
func (s *TextbookService) assembleAIContent(c *model.AIContent) (*model.AIContent, error) {
	var d []model.DialogueTurn
	var v []model.VocabItem
	var g []model.GrammarPoint
	var p []model.PracticeItem
	var im []string
	var svgs []model.SVGImage
	if c.DialogueJSON != "" {
		_ = json.Unmarshal([]byte(c.DialogueJSON), &d)
	}
	if c.KeyVocabJSON != "" {
		_ = json.Unmarshal([]byte(c.KeyVocabJSON), &v)
	}
	if c.GrammarJSON != "" {
		_ = json.Unmarshal([]byte(c.GrammarJSON), &g)
	}
	if c.PracticeJSON != "" {
		_ = json.Unmarshal([]byte(c.PracticeJSON), &p)
	}
	if c.ImagesJSON != "" {
		_ = json.Unmarshal([]byte(c.ImagesJSON), &im)
	}
	if c.SVGsJSON != "" {
		_ = json.Unmarshal([]byte(c.SVGsJSON), &svgs)
	}
	c.Dialogue = d
	c.KeyVocab = v
	c.Grammar = g
	c.Practice = p
	c.Images = im
	c.SVGs = svgs
	c.RoleVoices = model.DefaultRoleVoices
	return c, nil
}

// sanitizeSVGs 净化 LLM 生成的 SVG：移除危险元素，防止提示注入 XSS
func sanitizeSVGs(svgs []model.SVGImage) []model.SVGImage {
	if len(svgs) == 0 {
		return svgs
	}
	out := make([]model.SVGImage, 0, len(svgs))
	for _, svg := range svgs {
		clean := sanitizeSVG(svg.Svg)
		if strings.Contains(clean, "<svg") {
			out = append(out, model.SVGImage{Title: svg.Title, Svg: clean})
		}
	}
	return out
}

// sanitizeSVG 用正则剥掉危险标签与事件属性（script/foreignObject/on*）
func sanitizeSVG(s string) string {
	s = regexp.MustCompile(`(?is)<\s*script[\s\S]*?<\s*/\s*script\s*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<\s*foreignObject[\s\S]*?<\s*/\s*foreignObject\s*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\son\w+\s*=\s*("[^"]*"|'[^']*')`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\son\w+\s*=\s*[^\s>]+`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\bhref\s*=\s*"(javascript:[^"]*)"`).ReplaceAllString(s, `href="#"`)
	return s
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
