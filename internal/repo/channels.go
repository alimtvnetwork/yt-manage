package repo

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// ChannelRepository is the data-access contract for channels.
type ChannelRepository interface {
	UpsertByYTID(channel *model.Channel) error
	GetByYTID(ytChannelID string) (*model.Channel, error)
	ListByAccount(accountID uint) ([]model.Channel, error)
	List() ([]model.Channel, error)
}

type channelRepository struct {
	db *gorm.DB
}

func (r *channelRepository) UpsertByYTID(channel *model.Channel) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "yt_channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"account_id", "title", "handle", "subscriber_count", "video_count", "last_synced_at", "updated_at",
		}),
	}).Create(channel).Error
}

func (r *channelRepository) GetByYTID(ytChannelID string) (*model.Channel, error) {
	var channel model.Channel
	if err := r.db.Where("yt_channel_id = ?", ytChannelID).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepository) ListByAccount(accountID uint) ([]model.Channel, error) {
	var channels []model.Channel
	if err := r.db.Where("account_id = ?", accountID).Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func (r *channelRepository) List() ([]model.Channel, error) {
	var channels []model.Channel
	if err := r.db.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}
