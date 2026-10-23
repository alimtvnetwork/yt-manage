package repo

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// VideoQuery filters a video search over the local mirror.
type VideoQuery struct {
	Text            string
	ChannelID       uint
	Visibility      string
	PublishedAfter  *time.Time
	PublishedBefore *time.Time
	Limit           int
	Offset          int
}

// VideoRepository is the data-access contract for videos.
type VideoRepository interface {
	UpsertByYTID(video *model.Video) error
	GetByID(id uint) (*model.Video, error)
	GetByYTID(ytVideoID string) (*model.Video, error)
	ListByChannel(channelID uint) ([]model.Video, error)
	Search(q VideoQuery) ([]model.Video, error)
	UpdateTitleDescription(id uint, title string, description string) error
	UpdateSchedule(id uint, publishAt *time.Time, visibility model.Visibility) error
	UpdateStats(id uint, views int64, likes int64, comments int64) error
}

type videoRepository struct {
	db *gorm.DB
}

func (r *videoRepository) UpsertByYTID(video *model.Video) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "yt_video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"channel_id", "title", "description", "visibility", "is_short",
			"published_at", "publish_at", "view_count", "like_count",
			"comment_count", "duration_seconds", "last_synced_at", "updated_at",
		}),
	}).Create(video).Error
}

func (r *videoRepository) GetByID(id uint) (*model.Video, error) {
	var video model.Video
	if err := r.db.First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *videoRepository) GetByYTID(ytVideoID string) (*model.Video, error) {
	var video model.Video
	if err := r.db.Where("yt_video_id = ?", ytVideoID).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *videoRepository) ListByChannel(channelID uint) ([]model.Video, error) {
	var videos []model.Video
	if err := r.db.Where("channel_id = ?", channelID).Order("id ASC").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *videoRepository) Search(q VideoQuery) ([]model.Video, error) {
	query := r.db.Model(&model.Video{})
	if q.Text != "" {
		query = query.Joins("JOIN videos_fts ON videos_fts.rowid = videos.id").
			Where("videos_fts MATCH ?", q.Text)
	}
	if q.ChannelID != 0 {
		query = query.Where("videos.channel_id = ?", q.ChannelID)
	}
	if q.Visibility != "" {
		query = query.Where("videos.visibility = ?", q.Visibility)
	}
	if q.PublishedAfter != nil {
		query = query.Where("videos.published_at >= ?", q.PublishedAfter)
	}
	if q.PublishedBefore != nil {
		query = query.Where("videos.published_at <= ?", q.PublishedBefore)
	}
	query = query.Order("videos.id ASC")
	if q.Limit > 0 {
		query = query.Limit(q.Limit)
	}
	if q.Offset > 0 {
		query = query.Offset(q.Offset)
	}
	var videos []model.Video
	if err := query.Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *videoRepository) UpdateTitleDescription(id uint, title string, description string) error {
	return r.db.Model(&model.Video{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":       title,
		"description": description,
	}).Error
}

func (r *videoRepository) UpdateSchedule(id uint, publishAt *time.Time, visibility model.Visibility) error {
	return r.db.Model(&model.Video{}).Where("id = ?", id).Updates(map[string]interface{}{
		"publish_at": publishAt,
		"visibility": visibility,
	}).Error
}

func (r *videoRepository) UpdateStats(id uint, views int64, likes int64, comments int64) error {
	return r.db.Model(&model.Video{}).Where("id = ?", id).Updates(map[string]interface{}{
		"view_count":    views,
		"like_count":    likes,
		"comment_count": comments,
	}).Error
}
