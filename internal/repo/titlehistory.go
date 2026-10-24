package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// TitleHistoryRepository is the data-access contract for title
// history entries (content.db).
type TitleHistoryRepository interface {
	Record(videoID uint, oldTitle string, newTitle string, source string) error
	ListByVideo(videoID uint, limit int) ([]model.TitleHistory, error)
	LatestByVideo(videoID uint) (*model.TitleHistory, error)
}

type titleHistoryRepository struct {
	db *gorm.DB
}

func (r *titleHistoryRepository) Record(videoID uint, oldTitle string, newTitle string, source string) error {
	entry := &model.TitleHistory{
		VideoID: videoID, OldTitle: oldTitle, NewTitle: newTitle, Source: source,
	}
	return r.db.Create(entry).Error
}

func (r *titleHistoryRepository) ListByVideo(videoID uint, limit int) ([]model.TitleHistory, error) {
	var entries []model.TitleHistory
	query := r.db.Where("video_id = ?", videoID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *titleHistoryRepository) LatestByVideo(videoID uint) (*model.TitleHistory, error) {
	var entry model.TitleHistory
	if err := r.db.Where("video_id = ?", videoID).Order("id DESC").First(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}
