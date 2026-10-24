package repo

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// EpisodeRepository is the data-access contract for episodes.
type EpisodeRepository interface {
	UpsertByKey(episode *model.Episode) error
	GetByKey(channelID uint, key string) (*model.Episode, error)
	ListByChannel(channelID uint) ([]model.Episode, error)
	ReplaceParts(episodeID uint, parts []model.EpisodePart) error
	ListParts(episodeID uint) ([]model.EpisodePart, error)
	DeleteByChannel(channelID uint) error
}

type episodeRepository struct {
	db *gorm.DB
}

func (r *episodeRepository) UpsertByKey(episode *model.Episode) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "episode_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"episode_number", "title", "cadence_days", "first_publish_at", "updated_at",
		}),
	}).Create(episode).Error
}

func (r *episodeRepository) GetByKey(channelID uint, key string) (*model.Episode, error) {
	var episode model.Episode
	if err := r.db.Where("channel_id = ? AND episode_key = ?", channelID, key).First(&episode).Error; err != nil {
		return nil, err
	}
	return &episode, nil
}

func (r *episodeRepository) ListByChannel(channelID uint) ([]model.Episode, error) {
	var episodes []model.Episode
	if err := r.db.Where("channel_id = ?", channelID).Order("episode_number ASC").Find(&episodes).Error; err != nil {
		return nil, err
	}
	return episodes, nil
}

func (r *episodeRepository) ReplaceParts(episodeID uint, parts []model.EpisodePart) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("episode_id = ?", episodeID).Delete(&model.EpisodePart{}).Error; err != nil {
			return err
		}
		if len(parts) == 0 {
			return nil
		}
		for i := range parts {
			parts[i].ID = 0
			parts[i].EpisodeID = episodeID
		}
		return tx.Create(&parts).Error
	})
}

func (r *episodeRepository) ListParts(episodeID uint) ([]model.EpisodePart, error) {
	var parts []model.EpisodePart
	if err := r.db.Where("episode_id = ?", episodeID).Order("sequence ASC").Find(&parts).Error; err != nil {
		return nil, err
	}
	return parts, nil
}

// DeleteByChannel removes a channel's episodes and their parts. It is
// used to clear the demo dataset.
func (r *episodeRepository) DeleteByChannel(channelID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var episodes []model.Episode
		if err := tx.Where("channel_id = ?", channelID).Find(&episodes).Error; err != nil {
			return err
		}
		for _, episode := range episodes {
			if err := tx.Where("episode_id = ?", episode.ID).Delete(&model.EpisodePart{}).Error; err != nil {
				return err
			}
		}
		return tx.Where("channel_id = ?", channelID).Delete(&model.Episode{}).Error
	})
}
