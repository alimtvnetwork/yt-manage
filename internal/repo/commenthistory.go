package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// CommentHistoryRepository is the data-access contract for first
// comment edit history (content.db).
type CommentHistoryRepository interface {
	Record(commentID uint, videoID uint, oldText string) error
	ListByComment(commentID uint) ([]model.CommentHistory, error)
	GetByID(id uint) (*model.CommentHistory, error)
}

type commentHistoryRepository struct {
	db *gorm.DB
}

func (r *commentHistoryRepository) Record(commentID uint, videoID uint, oldText string) error {
	return r.db.Create(&model.CommentHistory{CommentID: commentID, VideoID: videoID, OldText: oldText}).Error
}

func (r *commentHistoryRepository) ListByComment(commentID uint) ([]model.CommentHistory, error) {
	var entries []model.CommentHistory
	if err := r.db.Where("comment_id = ?", commentID).Order("id DESC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *commentHistoryRepository) GetByID(id uint) (*model.CommentHistory, error) {
	var entry model.CommentHistory
	if err := r.db.First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}
