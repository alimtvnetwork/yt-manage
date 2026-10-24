package model

import "time"

// SyncState is the incremental sync cursor for an account resource in system.db.
type SyncState struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	AccountID uint       `gorm:"uniqueIndex:idx_sync_states_account_resource" json:"account_id"`
	Resource  string     `gorm:"uniqueIndex:idx_sync_states_account_resource" json:"resource"`
	Cursor    string     `json:"cursor"`
	LastRunAt *time.Time `json:"last_run_at"`
	LastError string     `json:"last_error"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (SyncState) TableName() string {
	return "sync_states"
}

// Peer is another yt-manage installation registered for peer sync in system.db.
type Peer struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `json:"name"`
	BaseURL      string     `json:"base_url"`
	SharedSecret string     `json:"-"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (Peer) TableName() string {
	return "peers"
}

// EmailLog is one sent or received publish-event email in system.db.
type EmailLog struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Direction      EmailDirection `json:"direction"`
	Subject        string         `json:"subject"`
	RelatedVideoID uint           `json:"related_video_id"`
	SentAt         *time.Time     `json:"sent_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (EmailLog) TableName() string {
	return "email_logs"
}

// Setting is a key/value setting in system.db.
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex" json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string {
	return "settings"
}

// ActivityLog is one entry in the global activity/history view in
// system.db (spec 29, items 58 and 63): settings changes, syncs,
// applies, and schedules, with an optional JSON detail payload
// that carries what an undo needs.
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Kind      string    `gorm:"index" json:"kind"`
	Summary   string    `json:"summary"`
	Detail    string    `json:"detail"`
	IsUndone  bool      `json:"is_undone"`
	CreatedAt time.Time `json:"created_at"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}
