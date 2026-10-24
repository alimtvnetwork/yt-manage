package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// ActivityRepository is the data-access contract for the activity
// log (system.db).
type ActivityRepository interface {
	Record(kind string, summary string, detail string) (*model.ActivityLog, error)
	List(limit int, kind string) ([]model.ActivityLog, error)
	GetByID(id uint) (*model.ActivityLog, error)
	MarkUndone(id uint) error
}

type activityRepository struct {
	db *gorm.DB
}

func (r *activityRepository) Record(kind string, summary string, detail string) (*model.ActivityLog, error) {
	entry := &model.ActivityLog{Kind: kind, Summary: summary, Detail: detail}
	if err := r.db.Create(entry).Error; err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *activityRepository) List(limit int, kind string) ([]model.ActivityLog, error) {
	var entries []model.ActivityLog
	query := r.db.Order("id DESC")
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *activityRepository) GetByID(id uint) (*model.ActivityLog, error) {
	var entry model.ActivityLog
	if err := r.db.First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *activityRepository) MarkUndone(id uint) error {
	return r.db.Model(&model.ActivityLog{}).Where("id = ?", id).Update("is_undone", true).Error
}
