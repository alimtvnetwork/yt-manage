package model

import "time"

// Episode is a grouped set of videos in planning.db.
type Episode struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ChannelID      uint       `gorm:"index;uniqueIndex:idx_episodes_channel_key" json:"channel_id"`
	EpisodeKey     string     `gorm:"uniqueIndex:idx_episodes_channel_key" json:"episode_key"`
	EpisodeNumber  int        `json:"episode_number"`
	Title          string     `json:"title"`
	CadenceDays    int        `json:"cadence_days"`
	FirstPublishAt *time.Time `json:"first_publish_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Episode) TableName() string {
	return "episodes"
}

// EpisodePart is one video part of an episode in planning.db.
type EpisodePart struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	EpisodeID  uint      `gorm:"index" json:"episode_id"`
	VideoID    uint      `gorm:"index" json:"video_id"`
	PartNumber int       `json:"part_number"`
	Sequence   int       `json:"sequence"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (EpisodePart) TableName() string {
	return "episode_parts"
}

// ScheduleRule is a cadence rule for a channel in planning.db.
type ScheduleRule struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	ChannelID        uint       `gorm:"index" json:"channel_id"`
	CadenceDays      int        `json:"cadence_days"`
	VideosPerEpisode int        `json:"videos_per_episode"`
	StartsAt         *time.Time `json:"starts_at"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (ScheduleRule) TableName() string {
	return "schedule_rules"
}

// ABTest is an A/B title test for a video in planning.db.
type ABTest struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	VariantA  string    `json:"variant_a"`
	VariantB  string    `json:"variant_b"`
	Winner    string    `json:"winner"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ABTest) TableName() string {
	return "ab_tests"
}

// ABTestSnapshot is a captured observation for an A/B test in planning.db.
type ABTestSnapshot struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TestID        uint      `gorm:"index" json:"test_id"`
	CapturedAt    time.Time `json:"captured_at"`
	ViewsAContext string    `json:"views_a_context"`
	CTRNote       string    `json:"ctr_note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ABTestSnapshot) TableName() string {
	return "ab_test_snapshots"
}
