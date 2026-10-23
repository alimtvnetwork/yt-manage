package model

import "time"

// Account is a connected Google/YouTube account in accounts.db.
type Account struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Label           string    `json:"label"`
	GoogleSubject   string    `gorm:"uniqueIndex" json:"google_subject"`
	ChannelID       string    `json:"channel_id"`
	TokenCiphertext []byte    `json:"-"`
	TokenNonce      []byte    `json:"-"`
	Scopes          string    `json:"scopes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Account) TableName() string {
	return "accounts"
}

// Channel is a YouTube channel owned by an account in accounts.db.
type Channel struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	AccountID       uint       `gorm:"index" json:"account_id"`
	YTChannelID     string     `gorm:"uniqueIndex" json:"yt_channel_id"`
	Title           string     `json:"title"`
	Handle          string     `json:"handle"`
	SubscriberCount int64      `json:"subscriber_count"`
	VideoCount      int64      `json:"video_count"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (Channel) TableName() string {
	return "channels"
}
