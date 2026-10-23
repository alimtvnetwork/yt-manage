// Package searchx provides AUM-style search over the local video
// mirror. It never calls the YouTube API: every query is served by
// the repository layer over the mirrored data.
package searchx

import (
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// Query filters a search over the local mirror.
type Query struct {
	Text            string
	ChannelID       uint
	Visibility      string
	PublishedAfter  *time.Time
	PublishedBefore *time.Time
	Limit           int
	Offset          int
}

// Search returns the mirrored videos matching q. The limit is
// normalised: a limit of zero or below falls back to 50, and any
// limit above 500 is capped at 500.
func Search(store *repo.Store, q Query) ([]model.Video, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	videos, err := store.Videos.Search(repo.VideoQuery{
		Text:            q.Text,
		ChannelID:       q.ChannelID,
		Visibility:      q.Visibility,
		PublishedAfter:  q.PublishedAfter,
		PublishedBefore: q.PublishedBefore,
		Limit:           limit,
		Offset:          q.Offset,
	})
	if err != nil {
		return nil, err
	}
	return videos, nil
}
