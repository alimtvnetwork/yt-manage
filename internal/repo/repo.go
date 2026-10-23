package repo

import (
	"github.com/alimtvnetwork/yt-manage/internal/db"
)

// Store aggregates every repository. It is the only data-access
// entry point used by services; all GORM access lives in this package.
type Store struct {
	Accounts   AccountRepository
	Channels   ChannelRepository
	Videos     VideoRepository
	Playlists  PlaylistRepository
	Comments   CommentRepository
	Episodes   EpisodeRepository
	Schedules  ScheduleRepository
	ABTests    ABTestRepository
	SyncStates SyncStateRepository
	Peers      PeerRepository
	EmailLogs  EmailLogRepository
	Settings   SettingRepository
}

// NewStore builds a Store backed by the four split databases.
func NewStore(dbs *db.DBs) *Store {
	return &Store{
		Accounts:   &accountRepository{db: dbs.Accounts},
		Channels:   &channelRepository{db: dbs.Accounts},
		Videos:     &videoRepository{db: dbs.Content},
		Playlists:  &playlistRepository{db: dbs.Content},
		Comments:   &commentRepository{db: dbs.Content},
		Episodes:   &episodeRepository{db: dbs.Planning},
		Schedules:  &scheduleRepository{db: dbs.Planning},
		ABTests:    &abTestRepository{db: dbs.Planning},
		SyncStates: &syncStateRepository{db: dbs.System},
		Peers:      &peerRepository{db: dbs.System},
		EmailLogs:  &emailLogRepository{db: dbs.System},
		Settings:   &settingRepository{db: dbs.System},
	}
}
