package service

import (
	"log"

	"gorm.io/gorm"

	"study-english/server/internal/model"
)

// TextbookSeed 教材种子（真实目录由 builtinTemplate 按名称自动匹配对应模板）
type TextbookSeed struct {
	Name      string
	Publisher string
	Stage     string
	Grade     string
}

// TextbookPresets 种子教材清单（下拉列表数据源，与系统初始化教材一致）
func TextbookPresets() []model.TextbookPreset {
	out := make([]model.TextbookPreset, 0, len(TextbookSeeds))
	for _, s := range TextbookSeeds {
		out = append(out, model.TextbookPreset{Name: s.Name, Publisher: s.Publisher, Stage: s.Stage, Grade: s.Grade})
	}
	return out
}

// TextbookSeeds 系统初始化种子：常见教材（全部为真实目录，模板见 textbook.go）
// 注意：新增模板后在此追加对应教材种子即可
var TextbookSeeds = []TextbookSeed{
	// PEP 小学（三年级~六年级 × 上下册）
	{Name: "人教版PEP三年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "三年级"},
	{Name: "人教版PEP三年级英语下册", Publisher: "人民教育出版社", Stage: "小学", Grade: "三年级"},
	{Name: "人教版PEP四年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "四年级"},
	{Name: "人教版PEP四年级英语下册", Publisher: "人民教育出版社", Stage: "小学", Grade: "四年级"},
	{Name: "人教版PEP五年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "五年级"},
	{Name: "人教版PEP五年级英语下册", Publisher: "人民教育出版社", Stage: "小学", Grade: "五年级"},
	{Name: "人教版PEP六年级英语上册", Publisher: "人民教育出版社", Stage: "小学", Grade: "六年级"},
	{Name: "人教版PEP六年级英语下册", Publisher: "人民教育出版社", Stage: "小学", Grade: "六年级"},
	// 初中
	{Name: "人教版七年级英语上册", Publisher: "人民教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "人教版七年级英语下册", Publisher: "人民教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "人教版八年级英语上册", Publisher: "人民教育出版社", Stage: "初中", Grade: "八年级"},
	{Name: "人教版八年级英语下册", Publisher: "人民教育出版社", Stage: "初中", Grade: "八年级"},
	{Name: "人教版九年级英语全一册", Publisher: "人民教育出版社", Stage: "初中", Grade: "九年级"},
	{Name: "外研版七年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "七年级"},
	{Name: "外研版七年级英语下册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "七年级"},
	{Name: "外研版八年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "八年级"},
	{Name: "外研版八年级英语下册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "八年级"},
	{Name: "外研版九年级英语上册", Publisher: "外语教学与研究出版社", Stage: "初中", Grade: "九年级"},
	{Name: "牛津译林版七年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "七年级"},
	{Name: "牛津译林版七年级英语下册", Publisher: "译林出版社", Stage: "初中", Grade: "七年级"},
	{Name: "牛津译林版八年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "八年级"},
	{Name: "牛津译林版八年级英语下册", Publisher: "译林出版社", Stage: "初中", Grade: "八年级"},
	{Name: "牛津译林版九年级英语上册", Publisher: "译林出版社", Stage: "初中", Grade: "九年级"},
	// 高中
	// 冀教版（河北教育出版社）
	{Name: "冀教版三年级英语下册", Publisher: "河北教育出版社", Stage: "小学", Grade: "三年级"},
	{Name: "冀教版七年级英语上册", Publisher: "河北教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "冀教版七年级英语下册", Publisher: "河北教育出版社", Stage: "初中", Grade: "七年级"},
	{Name: "冀教版九年级英语全一册", Publisher: "河北教育出版社", Stage: "初中", Grade: "九年级"},
	{Name: "人教版高中英语必修一", Publisher: "人民教育出版社", Stage: "高中", Grade: "高一"},
	{Name: "人教版高中英语必修二", Publisher: "人民教育出版社", Stage: "高中", Grade: "高一"},
	{Name: "人教版高中英语必修三", Publisher: "人民教育出版社", Stage: "高中", Grade: "高二"},
	{Name: "人教版高中英语选择性必修一", Publisher: "人民教育出版社", Stage: "高中", Grade: "高二"},
	{Name: "人教版高中英语选择性必修二", Publisher: "人民教育出版社", Stage: "高中", Grade: "高二"},
	{Name: "人教版高中英语选择性必修三", Publisher: "人民教育出版社", Stage: "高中", Grade: "高三"},
	{Name: "外研版高中英语必修一", Publisher: "外语教学与研究出版社", Stage: "高中", Grade: "高一"},
	{Name: "外研版高中英语必修二", Publisher: "外语教学与研究出版社", Stage: "高中", Grade: "高一"},
	{Name: "外研版高中英语必修三", Publisher: "外语教学与研究出版社", Stage: "高中", Grade: "高二"},
	// 成人
	{Name: "新概念英语第一册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "初级"},
	{Name: "新概念英语第二册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "中级"},
	{Name: "新概念英语第三册", Publisher: "外语教学与研究出版社", Stage: "成人", Grade: "中高级"},
}

// SeedTextbooks 初始化种子教材（幂等）：
// 仅当教材表为空（首次使用）时批量创建种子真实目录；
// 一旦系统已有教材，绝不再自动创建/修改任何教材——用户删除的教材不会复活、定制目录永久保留
func SeedTextbooks(db *gorm.DB) {
	var cnt int64
	db.Model(&model.Textbook{}).Count(&cnt)
	if cnt > 0 {
		return // 已初始化过，尊重用户的所有删除与定制
	}
	for _, s := range TextbookSeeds {
		svc := NewTextbookService(db, nil, "data")
		if _, err := svc.CreateTextbook(s.Name, s.Publisher, s.Stage, s.Grade, "builtin"); err != nil {
			log.Printf("[seed] 教材创建失败 %s: %v", s.Name, err)
			continue
		}
		log.Printf("[seed] 已初始化教材: %s（真实目录）", s.Name)
	}
}
