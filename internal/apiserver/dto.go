package apiserver

import (
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// DTO structs are the API wire shapes. GORM models are never
// marshalled directly.

type videoDTO struct {
	ID              uint   `json:"id"`
	YTVideoID       string `json:"ytVideoId"`
	ChannelID       uint   `json:"channelId"`
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
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type channelDTO struct {
	ID              uint   `json:"id"`
	YTChannelID     string `json:"ytChannelId"`
	Title           string `json:"title"`
	Handle          string `json:"handle"`
	SubscriberCount int64  `json:"subscriberCount"`
	VideoCount      int64  `json:"videoCount"`
	LastSyncedAt    string `json:"lastSyncedAt"`
}

type accountDTO struct {
	ID        uint         `json:"id"`
	Label     string       `json:"label"`
	ChannelID string       `json:"channelId"`
	CreatedAt string       `json:"createdAt"`
	Channels  []channelDTO `json:"channels"`
}

type episodePartDTO struct {
	Sequence    int    `json:"sequence"`
	PartNumber  int    `json:"partNumber"`
	VideoID     uint   `json:"videoId"`
	YTVideoID   string `json:"ytVideoId"`
	Title       string `json:"title"`
	Visibility  string `json:"visibility"`
	PublishAt   string `json:"publishAt"`
	PublishedAt string `json:"publishedAt"`
}

type episodeDTO struct {
	ID             uint             `json:"id"`
	ChannelID      uint             `json:"channelId"`
	EpisodeKey     string           `json:"episodeKey"`
	EpisodeNumber  int              `json:"episodeNumber"`
	Title          string           `json:"title"`
	CadenceDays    int              `json:"cadenceDays"`
	FirstPublishAt string           `json:"firstPublishAt"`
	Parts          []episodePartDTO `json:"parts"`
	GapParts       []int            `json:"gapParts"`
	Warnings       []string         `json:"warnings"`
}

type playlistDTO struct {
	ID           uint   `json:"id"`
	ChannelID    uint   `json:"channelId"`
	YTPlaylistID string `json:"ytPlaylistId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ItemCount    int64  `json:"itemCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type snapshotDTO struct {
	ID            uint   `json:"id"`
	TestID        uint   `json:"testId"`
	CapturedAt    string `json:"capturedAt"`
	ViewsAContext string `json:"viewsAContext"`
	CTRNote       string `json:"ctrNote"`
}

type abTestDTO struct {
	ID            uint          `json:"id"`
	VideoID       uint          `json:"videoId"`
	VariantA      string        `json:"variantA"`
	VariantB      string        `json:"variantB"`
	Winner        string        `json:"winner"`
	IsActive      bool          `json:"isActive"`
	SnapshotCount int           `json:"snapshotCount"`
	Snapshots     []snapshotDTO `json:"snapshots"`
	CreatedAt     string        `json:"createdAt"`
	UpdatedAt     string        `json:"updatedAt"`
}

// formatTime renders t as RFC3339.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatTimePtr renders a time pointer as RFC3339, empty when nil.
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func toVideoDTO(video model.Video) videoDTO {
	return videoDTO{
		ID: video.ID, YTVideoID: video.YTVideoID, ChannelID: video.ChannelID,
		Title: video.Title, Description: video.Description,
		Visibility: video.Visibility.String(), IsShort: video.IsShort,
		PublishedAt: formatTimePtr(video.PublishedAt), PublishAt: formatTimePtr(video.PublishAt),
		ViewCount: video.ViewCount, LikeCount: video.LikeCount, CommentCount: video.CommentCount,
		DurationSeconds: video.DurationSeconds, LastSyncedAt: formatTimePtr(video.LastSyncedAt),
		CreatedAt: formatTime(video.CreatedAt), UpdatedAt: formatTime(video.UpdatedAt),
	}
}

func toChannelDTO(channel model.Channel) channelDTO {
	return channelDTO{
		ID: channel.ID, YTChannelID: channel.YTChannelID, Title: channel.Title,
		Handle: channel.Handle, SubscriberCount: channel.SubscriberCount,
		VideoCount: channel.VideoCount, LastSyncedAt: formatTimePtr(channel.LastSyncedAt),
	}
}

func toPlaylistDTO(playlist model.Playlist) playlistDTO {
	return playlistDTO{
		ID: playlist.ID, ChannelID: playlist.ChannelID, YTPlaylistID: playlist.YTPlaylistID,
		Title: playlist.Title, Description: playlist.Description, ItemCount: playlist.ItemCount,
		CreatedAt: formatTime(playlist.CreatedAt), UpdatedAt: formatTime(playlist.UpdatedAt),
	}
}

func toSnapshotDTO(snapshot model.ABTestSnapshot) snapshotDTO {
	return snapshotDTO{
		ID: snapshot.ID, TestID: snapshot.TestID, CapturedAt: formatTime(snapshot.CapturedAt),
		ViewsAContext: snapshot.ViewsAContext, CTRNote: snapshot.CTRNote,
	}
}

func toABTestDTO(test model.ABTest, snapshots []model.ABTestSnapshot) abTestDTO {
	dto := abTestDTO{
		ID: test.ID, VideoID: test.VideoID, VariantA: test.VariantA, VariantB: test.VariantB,
		Winner: test.Winner, IsActive: test.IsActive, SnapshotCount: len(snapshots),
		Snapshots: make([]snapshotDTO, 0, len(snapshots)),
		CreatedAt: formatTime(test.CreatedAt), UpdatedAt: formatTime(test.UpdatedAt),
	}
	for _, snapshot := range snapshots {
		dto.Snapshots = append(dto.Snapshots, toSnapshotDTO(snapshot))
	}
	return dto
}
