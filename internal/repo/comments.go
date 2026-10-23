package repo

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// CommentRepository is the data-access contract for comments.
type CommentRepository interface {
	UpsertByYTID(comment *model.Comment) error
	GetFirstByVideo(videoID uint) (*model.Comment, error)
	ListByVideo(videoID uint) ([]model.Comment, error)
	UpdateText(id uint, text string) error
}

type commentRepository struct {
	db *gorm.DB
}

func (r *commentRepository) UpsertByYTID(comment *model.Comment) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "yt_comment_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"video_id", "author", "text", "is_first", "is_pinned_known", "published_at", "updated_at",
		}),
	}).Create(comment).Error
}

func (r *commentRepository) GetFirstByVideo(videoID uint) (*model.Comment, error) {
	var comment model.Comment
	if err := r.db.Where("video_id = ? AND is_first = ?", videoID, true).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *commentRepository) ListByVideo(videoID uint) ([]model.Comment, error) {
	var comments []model.Comment
	if err := r.db.Where("video_id = ?", videoID).Order("id ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *commentRepository) UpdateText(id uint, text string) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).Update("text", text).Error
}
