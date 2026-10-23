package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// EmailLogRepository is the data-access contract for email logs.
type EmailLogRepository interface {
	Create(entry *model.EmailLog) error
	ListRecent(limit int) ([]model.EmailLog, error)
}

type emailLogRepository struct {
	db *gorm.DB
}

func (r *emailLogRepository) Create(entry *model.EmailLog) error {
	return r.db.Create(entry).Error
}

func (r *emailLogRepository) ListRecent(limit int) ([]model.EmailLog, error) {
	query := r.db.Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var entries []model.EmailLog
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
