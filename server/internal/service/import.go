package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"

	"study-english/server/internal/ai"
	"study-english/server/internal/model"
)

// ImportLessonText 导入课文原文（文本粘贴/JSON）→ imported_contents 表（版权合规通道）
func (s *TextbookService) ImportLessonText(catalogItemID uint, title, text string) (*model.ImportedContent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("课文内容不能为空")
	}
	var item model.CatalogItem
	if err := s.db.First(&item, catalogItemID).Error; err != nil {
		return nil, fmt.Errorf("目录项不存在: %w", err)
	}

	content := &model.ImportedContent{
		CatalogItemID: item.ID,
		Unit:          item.Unit,
		Title:         firstNonEmpty(title, item.Title),
		Text:          text,
		Source:        "imported",
		Status:        "published",
	}
	// 已有导入内容则更新，否则创建
	var existing model.ImportedContent
	if err := s.db.Where("catalog_item_id = ?", item.ID).First(&existing).Error; err == nil {
		existing.Title = content.Title
		existing.Text = content.Text
		if err := s.db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err := s.db.Create(content).Error; err != nil {
		return nil, err
	}
	// 更新目录项状态
	s.db.Model(&item).Update("status", model.StatusPublished)
	return content, nil
}

// ImportLessonPDF 导入 PDF 课文：纯 Go 提取文本（免 CGO），无 OCR
func (s *TextbookService) ImportLessonPDF(catalogItemID uint, title string, data []byte) (*model.ImportedContent, error) {
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("PDF 解析失败（可能加密或非文本型扫描件）: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= pdfReader.NumPage(); i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err == nil {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}
	extracted := strings.TrimSpace(sb.String())
	if extracted == "" {
		return nil, errors.New("PDF 中未提取到文本（可能是扫描图片版，请使用图片导入或直接粘贴文字）")
	}
	if title == "" {
		title = firstNonEmpty(strings.Split(extracted, "\n")[0], "导入课文")
	}
	return s.ImportLessonText(catalogItemID, title, extracted)
}

// ImportLessonImage 导入图片课文：视觉大模型 OCR 识别 → imported_contents（版权合规通道）
func (s *TextbookService) ImportLessonImage(catalogItemID uint, title string, imgData []byte) (*model.ImportedContent, error) {
	// 从已保存配置读取 OCR 视觉模型设置（未配置则明确提示）
	var cfg model.AIConfig
	if err := s.db.First(&cfg, 1).Error; err != nil || cfg.OCR.Model == "" {
		return nil, errors.New("未配置 OCR 视觉模型，请到「⚙️ 模型配置」→「👁️ OCR 识别」设置（如 Qwen/Qwen3-VL-8B-Instruct 或 deepseek-ai/DeepSeek-OCR）")
	}
	ocr := ai.NewOpenAIVisionOCR(cfg.OCR.BaseURL, cfg.OCR.APIKey, cfg.OCR.Model)
	text, err := ocr.Recognize(context.Background(), ai.B64Encode(imgData))
	if err != nil {
		return nil, fmt.Errorf("OCR 识别失败: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("OCR 未识别到课文文字，请尝试更清晰的图片")
	}
	return s.ImportLessonText(catalogItemID, title, text)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
