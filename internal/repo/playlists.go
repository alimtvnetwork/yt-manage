package repo

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// PlaylistRepository is the data-access contract for playlists.
type PlaylistRepository interface {
	UpsertByYTID(playlist *model.Playlist) error
	GetByYTID(ytPlaylistID string) (*model.Playlist, error)
	GetByID(id uint) (*model.Playlist, error)
	ListByChannel(channelID uint) ([]model.Playlist, error)
	ReplaceItems(playlistID uint, items []model.PlaylistItem) error
	ListItems(playlistID uint) ([]model.PlaylistItem, error)
	DeleteByChannel(channelID uint) error
}

type playlistRepository struct {
	db *gorm.DB
}

func (r *playlistRepository) UpsertByYTID(playlist *model.Playlist) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "yt_playlist_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"channel_id", "title", "description", "item_count", "updated_at",
		}),
	}).Create(playlist).Error
}

func (r *playlistRepository) GetByID(id uint) (*model.Playlist, error) {
	var playlist model.Playlist
	if err := r.db.First(&playlist, id).Error; err != nil {
		return nil, err
	}
	return &playlist, nil
}

func (r *playlistRepository) GetByYTID(ytPlaylistID string) (*model.Playlist, error) {
	var playlist model.Playlist
	if err := r.db.Where("yt_playlist_id = ?", ytPlaylistID).First(&playlist).Error; err != nil {
		return nil, err
	}
	return &playlist, nil
}

func (r *playlistRepository) ListByChannel(channelID uint) ([]model.Playlist, error) {
	var playlists []model.Playlist
	if err := r.db.Where("channel_id = ?", channelID).Order("id ASC").Find(&playlists).Error; err != nil {
		return nil, err
	}
	return playlists, nil
}

func (r *playlistRepository) ReplaceItems(playlistID uint, items []model.PlaylistItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("playlist_id = ?", playlistID).Delete(&model.PlaylistItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			items[i].ID = 0
			items[i].PlaylistID = playlistID
		}
		return tx.Create(&items).Error
	})
}

func (r *playlistRepository) ListItems(playlistID uint) ([]model.PlaylistItem, error) {
	var items []model.PlaylistItem
	if err := r.db.Where("playlist_id = ?", playlistID).Order("position ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteByChannel removes a channel's playlists and their items. It is
// used to clear the demo dataset.
func (r *playlistRepository) DeleteByChannel(channelID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var playlists []model.Playlist
		if err := tx.Where("channel_id = ?", channelID).Find(&playlists).Error; err != nil {
			return err
		}
		for _, playlist := range playlists {
			if err := tx.Where("playlist_id = ?", playlist.ID).Delete(&model.PlaylistItem{}).Error; err != nil {
				return err
			}
		}
		return tx.Where("channel_id = ?", channelID).Delete(&model.Playlist{}).Error
	})
}
