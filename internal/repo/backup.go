package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// DBBackupFile is one database file written by a backup run.
type DBBackupFile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

// VacuumBackups writes a consistent copy of each of the four
// split databases into dir using SQLite's VACUUM INTO, which
// snapshots a live database safely (no torn WAL copies). It
// backs the scheduled database backups of spec 29 item 176.
// dir must be a path this process generated itself; it is
// inlined into the SQL, so quotes are rejected outright.
func (s *Store) VacuumBackups(dir string) ([]DBBackupFile, error) {
	if strings.ContainsAny(dir, "'\";") {
		return nil, fmt.Errorf("backup directory contains unsafe characters")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	targets := []struct {
		name string
		db   *gorm.DB
	}{
		{"accounts.db", s.dbs.Accounts},
		{"content.db", s.dbs.Content},
		{"planning.db", s.dbs.Planning},
		{"system.db", s.dbs.System},
	}
	files := []DBBackupFile{}
	for _, target := range targets {
		path := filepath.Join(dir, target.name)
		if err := target.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", path)).Error; err != nil {
			return files, fmt.Errorf("backup %s: %w", target.name, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return files, fmt.Errorf("stat backup %s: %w", target.name, err)
		}
		files = append(files, DBBackupFile{Name: target.name, Path: path, SizeBytes: info.Size()})
	}
	return files, nil
}
