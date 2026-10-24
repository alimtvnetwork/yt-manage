package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// DBs holds the four split databases.
type DBs struct {
	Accounts *gorm.DB
	Content  *gorm.DB
	Planning *gorm.DB
	System   *gorm.DB
}

// Open creates <homeDir>/data/, opens the four split databases with
// foreign keys and WAL enabled, migrates each one, and sets up the
// FTS5 index for videos in content.db.
func Open(homeDir string) (*DBs, error) {
	dataDir := filepath.Join(homeDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	accounts, err := openOne(filepath.Join(dataDir, "accounts.db"), &model.Account{}, &model.Channel{})
	if err != nil {
		return nil, err
	}
	content, err := openOne(filepath.Join(dataDir, "content.db"), &model.Video{}, &model.Playlist{}, &model.PlaylistItem{}, &model.Comment{}, &model.TitleHistory{})
	if err != nil {
		closeDB(accounts)
		return nil, err
	}
	planning, err := openOne(filepath.Join(dataDir, "planning.db"), &model.Episode{}, &model.EpisodePart{}, &model.ScheduleRule{}, &model.ABTest{}, &model.ABTestSnapshot{})
	if err != nil {
		closeDB(accounts)
		closeDB(content)
		return nil, err
	}
	system, err := openOne(filepath.Join(dataDir, "system.db"), &model.SyncState{}, &model.Peer{}, &model.EmailLog{}, &model.Setting{}, &model.ActivityLog{})
	if err != nil {
		closeDB(accounts)
		closeDB(content)
		closeDB(planning)
		return nil, err
	}

	dbs := &DBs{
		Accounts: accounts,
		Content:  content,
		Planning: planning,
		System:   system,
	}

	if err := setupVideosFTS(content); err != nil {
		_ = dbs.Close()
		return nil, err
	}

	return dbs, nil
}

// Close closes all four databases.
func (d *DBs) Close() error {
	var firstErr error
	for _, gdb := range []*gorm.DB{d.Accounts, d.Content, d.Planning, d.System} {
		if err := closeDB(gdb); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func openOne(path string, models ...interface{}) (*gorm.DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := gdb.AutoMigrate(models...); err != nil {
		_ = closeDB(gdb)
		return nil, fmt.Errorf("migrate %s: %w", filepath.Base(path), err)
	}
	return gdb, nil
}

func closeDB(gdb *gorm.DB) error {
	if gdb == nil {
		return nil
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func setupVideosFTS(content *gorm.DB) error {
	statements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS videos_fts USING fts5(title, description, content='videos', content_rowid='id')`,
		`CREATE TRIGGER IF NOT EXISTS videos_fts_insert AFTER INSERT ON videos BEGIN
			INSERT INTO videos_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS videos_fts_update AFTER UPDATE ON videos BEGIN
			INSERT INTO videos_fts(videos_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
			INSERT INTO videos_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS videos_fts_delete AFTER DELETE ON videos BEGIN
			INSERT INTO videos_fts(videos_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
		END`,
	}
	for _, stmt := range statements {
		if err := content.Exec(stmt).Error; err != nil {
			return fmt.Errorf("setup videos_fts: %w", err)
		}
	}
	return nil
}
