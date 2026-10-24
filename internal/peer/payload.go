package peer

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// VideoDTO is the wire form of one mirrored video. Times are
// RFC3339 strings; an empty string means the time is unset.
type VideoDTO struct {
	YtVideoId       string `json:"ytVideoId"`
	ChannelYtId     string `json:"channelYtId"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Visibility      string `json:"visibility"`
	IsShort         bool   `json:"isShort"`
	PublishedAt     string `json:"publishedAt"`
	PublishAt       string `json:"publishAt"`
	ViewCount       int64  `json:"viewCount"`
	LikeCount       int64  `json:"likeCount"`
	CommentCount    int64  `json:"commentCount"`
	DurationSeconds int64  `json:"durationSeconds"`
	LastSyncedAt    string `json:"lastSyncedAt"`
}

// PlaylistItemDTO is the wire form of one playlist entry: the
// YouTube video id and its position in the playlist.
type PlaylistItemDTO struct {
	YtVideoId string `json:"ytVideoId"`
	Position  int    `json:"position"`
}

// PlaylistDTO is the wire form of one mirrored playlist with its
// items.
type PlaylistDTO struct {
	YtPlaylistId string            `json:"ytPlaylistId"`
	ChannelYtId  string            `json:"channelYtId"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	ItemCount    int64             `json:"itemCount"`
	Items        []PlaylistItemDTO `json:"items"`
}

// Payload is the full cache exchange body: every mirrored video and
// playlist of the sender.
// PayloadSchemaVersion is the wire schema of Payload. It is
// bumped when the shape changes incompatibly, so a peer on a
// different schema can be warned about before sync (spec 29
// item 180: version-skew warnings).
const PayloadSchemaVersion = 1

type Payload struct {
	AppVersion    string        `json:"appVersion"`
	SchemaVersion int           `json:"schemaVersion"`
	Videos        []VideoDTO    `json:"videos"`
	Playlists     []PlaylistDTO `json:"playlists"`
}

// Result summarises one applied payload: how many videos and
// playlists were written, how many videos were skipped because the
// local copy is newer, and how many playlist items were written.
type Result struct {
	VideosApplied    int `json:"videosApplied"`
	VideosSkipped    int `json:"videosSkipped"`
	PlaylistsApplied int `json:"playlistsApplied"`
	ItemsApplied     int `json:"itemsApplied"`
}

// formatTime renders a nullable time as RFC3339, or "" when nil.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// parseTime parses an RFC3339 wire time; "" yields nil.
func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// BuildPayload collects every mirrored video and playlist into a
// wire payload. Playlist items are resolved to YouTube video ids;
// an item whose video is no longer in the mirror is skipped.
func BuildPayload(store *repo.Store) (*Payload, error) {
	return BuildPayloadFor(store, nil)
}

// BuildPayloadFor is BuildPayload restricted to the channels
// whose YouTube ids are in keep (nil keeps every channel). It
// backs selective peer sharing (spec 29 item 172): a peer only
// ever receives the channels the user chose to share with it.
func BuildPayloadFor(store *repo.Store, keep map[string]bool) (*Payload, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	payload := &Payload{
		AppVersion:    version.Version,
		SchemaVersion: PayloadSchemaVersion,
		Videos:        []VideoDTO{},
		Playlists:     []PlaylistDTO{},
	}
	for _, channel := range channels {
		if keep != nil && !keep[channel.YTChannelID] {
			continue
		}
		videos, err := store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return nil, fmt.Errorf("list videos for channel %d: %w", channel.ID, err)
		}
		for _, video := range videos {
			payload.Videos = append(payload.Videos, videoToDTO(video, channel.YTChannelID))
		}
		playlists, err := store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			return nil, fmt.Errorf("list playlists for channel %d: %w", channel.ID, err)
		}
		for _, playlist := range playlists {
			dto, err := playlistToDTO(store, playlist, channel.YTChannelID)
			if err != nil {
				return nil, err
			}
			payload.Playlists = append(payload.Playlists, dto)
		}
	}
	return payload, nil
}

// videoToDTO converts one mirrored video to its wire form.
func videoToDTO(video model.Video, channelYTID string) VideoDTO {
	return VideoDTO{
		YtVideoId:       video.YTVideoID,
		ChannelYtId:     channelYTID,
		Title:           video.Title,
		Description:     video.Description,
		Visibility:      video.Visibility.String(),
		IsShort:         video.IsShort,
		PublishedAt:     formatTime(video.PublishedAt),
		PublishAt:       formatTime(video.PublishAt),
		ViewCount:       video.ViewCount,
		LikeCount:       video.LikeCount,
		CommentCount:    video.CommentCount,
		DurationSeconds: video.DurationSeconds,
		LastSyncedAt:    formatTime(video.LastSyncedAt),
	}
}

// playlistToDTO converts one mirrored playlist and its items to
// wire form, skipping items whose video cannot be resolved.
func playlistToDTO(store *repo.Store, playlist model.Playlist, channelYTID string) (PlaylistDTO, error) {
	dto := PlaylistDTO{
		YtPlaylistId: playlist.YTPlaylistID,
		ChannelYtId:  channelYTID,
		Title:        playlist.Title,
		Description:  playlist.Description,
		ItemCount:    playlist.ItemCount,
		Items:        []PlaylistItemDTO{},
	}
	items, err := store.Playlists.ListItems(playlist.ID)
	if err != nil {
		return PlaylistDTO{}, fmt.Errorf("list items for playlist %d: %w", playlist.ID, err)
	}
	for _, item := range items {
		video, err := store.Videos.GetByID(item.VideoID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return PlaylistDTO{}, fmt.Errorf("resolve video %d for playlist %d: %w", item.VideoID, playlist.ID, err)
		}
		dto.Items = append(dto.Items, PlaylistItemDTO{
			YtVideoId: video.YTVideoID,
			Position:  item.Position,
		})
	}
	return dto, nil
}

// ApplyPayload merges a received payload into the local mirror and
// reports what was written. A video whose local last_synced_at is
// not before the incoming one is kept and counted as skipped, so
// conflicts always keep the newer copy.
func ApplyPayload(store *repo.Store, p *Payload) (Result, error) {
	var result Result
	if p == nil {
		return result, nil
	}
	for _, dto := range p.Videos {
		isApplied, err := applyVideo(store, dto)
		if err != nil {
			return result, err
		}
		if isApplied {
			result.VideosApplied++
		} else {
			result.VideosSkipped++
		}
	}
	for _, dto := range p.Playlists {
		itemsApplied, err := applyPlaylist(store, dto)
		if err != nil {
			return result, err
		}
		result.PlaylistsApplied++
		result.ItemsApplied += itemsApplied
	}
	return result, nil
}

// applyVideo writes one video DTO into the mirror. It reports
// false when the local copy is newer and the DTO was skipped.
func applyVideo(store *repo.Store, dto VideoDTO) (bool, error) {
	channel, err := resolveChannel(store, dto.ChannelYtId)
	if err != nil {
		return false, err
	}
	publishedAt, err := parseTime(dto.PublishedAt)
	if err != nil {
		return false, fmt.Errorf("parse published_at for video %s: %w", dto.YtVideoId, err)
	}
	publishAt, err := parseTime(dto.PublishAt)
	if err != nil {
		return false, fmt.Errorf("parse publish_at for video %s: %w", dto.YtVideoId, err)
	}
	lastSyncedAt, err := parseTime(dto.LastSyncedAt)
	if err != nil {
		return false, fmt.Errorf("parse last_synced_at for video %s: %w", dto.YtVideoId, err)
	}
	existing, err := store.Videos.GetByYTID(dto.YtVideoId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("load video %s: %w", dto.YtVideoId, err)
		}
		existing = nil
	}
	if existing != nil && !isBefore(existing.LastSyncedAt, lastSyncedAt) {
		return false, nil
	}
	visibility, err := model.ParseVisibility(dto.Visibility)
	if err != nil {
		visibility = model.VisibilityPrivate
	}
	video := &model.Video{
		ChannelID:       channel.ID,
		YTVideoID:       dto.YtVideoId,
		Title:           dto.Title,
		Description:     dto.Description,
		Visibility:      visibility,
		IsShort:         dto.IsShort,
		PublishedAt:     publishedAt,
		PublishAt:       publishAt,
		ViewCount:       dto.ViewCount,
		LikeCount:       dto.LikeCount,
		CommentCount:    dto.CommentCount,
		DurationSeconds: dto.DurationSeconds,
		LastSyncedAt:    lastSyncedAt,
	}
	if err := store.Videos.UpsertByYTID(video); err != nil {
		return false, fmt.Errorf("save video %s: %w", dto.YtVideoId, err)
	}
	return true, nil
}

// applyPlaylist writes one playlist DTO and replaces its items
// with the DTO's items resolved to local videos. It returns the
// number of items written; items whose video is unknown locally
// are skipped.
func applyPlaylist(store *repo.Store, dto PlaylistDTO) (int, error) {
	channel, err := resolveChannel(store, dto.ChannelYtId)
	if err != nil {
		return 0, err
	}
	itemCount := dto.ItemCount
	existing, err := store.Playlists.GetByYTID(dto.YtPlaylistId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("load playlist %s: %w", dto.YtPlaylistId, err)
		}
		existing = nil
	}
	if existing != nil && existing.ItemCount > itemCount {
		itemCount = existing.ItemCount
	}
	playlist := &model.Playlist{
		ChannelID:    channel.ID,
		YTPlaylistID: dto.YtPlaylistId,
		Title:        dto.Title,
		Description:  dto.Description,
		ItemCount:    itemCount,
	}
	if err := store.Playlists.UpsertByYTID(playlist); err != nil {
		return 0, fmt.Errorf("save playlist %s: %w", dto.YtPlaylistId, err)
	}
	stored, err := store.Playlists.GetByYTID(dto.YtPlaylistId)
	if err != nil {
		return 0, fmt.Errorf("reload playlist %s: %w", dto.YtPlaylistId, err)
	}
	items := []model.PlaylistItem{}
	for _, item := range dto.Items {
		video, err := store.Videos.GetByYTID(item.YtVideoId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return 0, fmt.Errorf("resolve video %s for playlist %s: %w", item.YtVideoId, dto.YtPlaylistId, err)
		}
		items = append(items, model.PlaylistItem{
			VideoID:  video.ID,
			Position: item.Position,
		})
	}
	if err := store.Playlists.ReplaceItems(stored.ID, items); err != nil {
		return 0, fmt.Errorf("save items for playlist %s: %w", dto.YtPlaylistId, err)
	}
	return len(items), nil
}

// resolveChannel returns the local channel for a YouTube channel
// id, creating a stub row (no account, empty title) when the
// channel is not mirrored yet, so peer content for an unconnected
// channel still has somewhere to land.
func resolveChannel(store *repo.Store, ytChannelID string) (*model.Channel, error) {
	channel, err := store.Channels.GetByYTID(ytChannelID)
	if err == nil {
		return channel, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load channel %s: %w", ytChannelID, err)
	}
	stub := &model.Channel{
		YTChannelID: ytChannelID,
		AccountID:   0,
		Title:       "",
	}
	if err := store.Channels.UpsertByYTID(stub); err != nil {
		return nil, fmt.Errorf("create stub channel %s: %w", ytChannelID, err)
	}
	channel, err = store.Channels.GetByYTID(ytChannelID)
	if err != nil {
		return nil, fmt.Errorf("reload channel %s: %w", ytChannelID, err)
	}
	return channel, nil
}

// isBefore reports whether a is strictly before b. A nil time is
// older than any set time; two nil times are equal.
func isBefore(a *time.Time, b *time.Time) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil {
		return true
	}
	if b == nil {
		return false
	}
	return a.Before(*b)
}
