package model

import "time"

// Video is a mirrored YouTube video in content.db.
type Video struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ChannelID       uint       `gorm:"index" json:"channel_id"`
	YTVideoID       string     `gorm:"uniqueIndex" json:"yt_video_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Visibility      Visibility `json:"visibility"`
	IsShort         bool       `json:"is_short"`
	PublishedAt     *time.Time `json:"published_at"`
	PublishAt       *time.Time `json:"publish_at"`
	ViewCount       int64      `json:"view_count"`
	LikeCount       int64      `json:"like_count"`
	CommentCount    int64      `json:"comment_count"`
	DurationSeconds int64      `json:"duration_seconds"`
	Tags            string     `json:"tags"` // JSON array of tag strings, mirrored on sync
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (Video) TableName() string {
	return "videos"
}

// Playlist is a mirrored YouTube playlist in content.db.
type Playlist struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ChannelID    uint      `gorm:"index" json:"channel_id"`
	YTPlaylistID string    `gorm:"uniqueIndex" json:"yt_playlist_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Privacy      string    `json:"privacy"`
	ItemCount    int64     `json:"item_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Playlist) TableName() string {
	return "playlists"
}

// PlaylistItem is one entry in a playlist in content.db.
type PlaylistItem struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PlaylistID uint      `gorm:"index" json:"playlist_id"`
	VideoID    uint      `gorm:"index" json:"video_id"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PlaylistItem) TableName() string {
	return "playlist_items"
}

// Comment is a mirrored or managed comment in content.db.
type Comment struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	VideoID       uint       `gorm:"index" json:"video_id"`
	YTCommentID   string     `gorm:"uniqueIndex" json:"yt_comment_id"`
	Author        string     `json:"author"`
	Text          string     `json:"text"`
	IsFirst       bool       `json:"is_first"`
	IsPinnedKnown bool       `json:"is_pinned_known"`
	PinnedText    string     `json:"pinned_text"`
	LikeCount     int64      `json:"like_count"`
	PublishedAt   *time.Time `json:"published_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (Comment) TableName() string {
	return "comments"
}

// CommentHistory is one prior version of a first comment's text in
// content.db (spec 29, item 125): every edit records the text it
// replaced so any version can be restored.
type CommentHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CommentID uint      `gorm:"index" json:"comment_id"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	OldText   string    `json:"old_text"`
	CreatedAt time.Time `json:"created_at"`
}

func (CommentHistory) TableName() string {
	return "comment_histories"
}

// TitleHistory is one recorded title change for a video in content.db.
// It powers the per-video rename history (spec 29, item 38) and the
// undo data for applied renames.
type TitleHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	OldTitle  string    `json:"old_title"`
	NewTitle  string    `json:"new_title"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func (TitleHistory) TableName() string {
	return "title_histories"
}
