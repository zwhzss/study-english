package database

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"study-english/server/internal/model"
)

// migrateLegacyData 数据目录迁移（v1 开发期数据在项目 data/，现统一到 %AppData%\StudyEnglish）：
// 目标库不存在且旧库存在 → 自动复制数据库 + 素材目录（images/audio/pronunciations），保留教材/内容/配置/图片，绝不覆盖
func migrateLegacyData(target, targetDataDir string) {
	if _, err := os.Stat(target); err == nil {
		return // 目标已存在（含已迁移过）
	}
	legacy := filepath.Join("data", "study-english.db")
	if _, err := os.Stat(legacy); err != nil {
		return // 无旧库
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return
	}
	in, err := os.Open(legacy)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, in)
	_ = out.Sync()
	fmt.Println("[migrate] 已把旧数据 data/study-english.db 迁移到", target)

	// 素材目录迁移：图片/音频/发音录音（数据库里存的相对路径 /media/xxx 指向这些目录）
	for _, sub := range []string{"images", "audio", "pronunciations"} {
		src := filepath.Join("data", sub)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(targetDataDir, sub)
		n := copyDirMissing(src, dst)
		if n > 0 {
			fmt.Printf("[migrate] 素材 %s → %s（%d 个文件）\n", src, dst, n)
		}
	}
}

// migrateMedia 无条件补迁移素材目录（幂等：copyDirMissing 只补缺失文件不覆盖）。
// 用于已迁移过数据库、但素材目录没跟上（图片/音频丢失）的情况——每次启动都执行，无旧素材时零开销
func migrateMedia(targetDataDir string) {
	for _, sub := range []string{"images", "audio", "pronunciations"} {
		src := filepath.Join("data", sub)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(targetDataDir, sub)
		n := copyDirMissing(src, dst)
		if n > 0 {
			fmt.Printf("[migrate] 补迁移素材 %s → %s（%d 个文件）\n", src, dst, n)
		}
	}
}

// copyDirMissing 把 src 目录下目标不存在的文件复制到 dst（不覆盖已有文件），返回复制数量
func copyDirMissing(src, dst string) int {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n += copyDirMissing(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
			continue
		}
		dstFile := filepath.Join(dst, e.Name())
		if _, err := os.Stat(dstFile); err == nil {
			continue // 已存在，不覆盖
		}
		in, err := os.Open(filepath.Join(src, e.Name()))
		if err != nil {
			continue
		}
		out, err := os.Create(dstFile)
		if err != nil {
			in.Close()
			continue
		}
		_, _ = io.Copy(out, in)
		in.Close()
		out.Close()
		n++
	}
	return n
}

// Init 打开 SQLite（纯 Go 驱动，免 CGO），启用 WAL + busy_timeout，配置连接池
func Init(dbPath, dataDir string) (*gorm.DB, error) {
	migrateLegacyData(dbPath, dataDir)
	migrateMedia(dataDir) // 素材目录无条件补迁移（幂等：只补缺失，不覆盖），解决"已迁移 db 但图片丢失"
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-20000)", dbPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// 纯 Go 驱动真正支持多连接（不受 mattn 单句柄限制）
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.AutoMigrate(
		&model.Textbook{},
		&model.CatalogItem{},
		&model.AIContent{},
		&model.ImportedContent{},
		&model.DialogueSession{},
		&model.DialogueMessage{},
		&model.AIConfig{},
		&model.PronunciationRecord{},
	); err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}

	// 旧版单表 lesson_contents → ai_contents 数据迁移（一次性，幂等）
	migrateLegacyContents(db)

	return db, nil
}

// migrateLegacyContents 将 v1 的 lesson_contents 迁移到 ai_contents（旧表数据作为 AI 创作内容）
func migrateLegacyContents(db *gorm.DB) {
	var legacy []model.LessonContent
	if err := db.Find(&legacy).Error; err != nil || len(legacy) == 0 {
		return
	}
	// 仅当目标表为空时迁移（幂等）
	var cnt int64
	db.Model(&model.AIContent{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	for _, l := range legacy {
		ai := &model.AIContent{
			CatalogItemID: l.CatalogItemID,
			Unit:          l.Unit,
			Title:         l.Title,
			Scene:         l.Scene,
			DialogueJSON:  l.DialogueJSON,
			KeyVocabJSON:  l.KeyVocabJSON,
			GrammarJSON:   l.GrammarJSON,
			PracticeJSON:  l.PracticeJSON,
			ImagesJSON:    l.ImagesJSON,
			SVGsJSON:      l.SVGsJSON,
			Source:        "llm",
			Status:        "published",
		}
		_ = db.Create(ai).Error
	}
	// 迁移后清空旧表，避免重复迁移
	db.Exec("DELETE FROM lesson_contents")
}
