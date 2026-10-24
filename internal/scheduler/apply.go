package scheduler

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

// Apply applies a plan in entry order. Each video is written to the
// YouTube API first (private with its publishAt) and only then
// mirrored locally as scheduled. It stops at the first error and
// returns how many videos were scheduled before it. Once every
// entry of an episode is applied, the episode row records the
// rule's cadence and its first publish time.
func Apply(ctx context.Context, svc *youtube.Service, store *repo.Store, plan *Plan) (int, error) {
	applied := 0
	for i, entry := range plan.Entries {
		if err := applyEntry(ctx, svc, store, entry); err != nil {
			return applied, fmt.Errorf("schedule %s: %w", entry.YTVideoID, err)
		}
		applied++
		isLastOfEpisode := i == len(plan.Entries)-1 || plan.Entries[i+1].EpisodeID != entry.EpisodeID
		if isLastOfEpisode {
			if err := markEpisodeScheduled(store, plan, entry); err != nil {
				return applied, err
			}
		}
	}
	return applied, nil
}

// applyEntry schedules one video: fetch it, set it private with
// its publishAt through the API, then mirror the schedule.
func applyEntry(ctx context.Context, svc *youtube.Service, store *repo.Store, entry PlanEntry) error {
	full, err := ytapi.FetchVideo(ctx, svc, entry.YTVideoID)
	if err != nil {
		return err
	}
	if full.Status == nil {
		full.Status = &youtube.VideoStatus{}
	}
	publishAt := entry.PublishAt
	full.Status.PrivacyStatus = "private"
	full.Status.PublishAt = publishAt.UTC().Format(time.RFC3339)
	if _, err := svc.Videos.Update([]string{"snippet", "status"}, full).Context(ctx).Do(); err != nil {
		return err
	}
	return store.Videos.UpdateSchedule(entry.VideoID, &publishAt, model.VisibilityScheduled)
}

// RestoreSchedule puts one video back to a previous publish state:
// a non-nil publishAt re-schedules it (private with that publish
// time); a nil publishAt clears the schedule and restores the
// given visibility. It is the undo path for an applied schedule
// (spec 29, item 62) and mirrors applyEntry in reverse.
func RestoreSchedule(ctx context.Context, svc *youtube.Service, store *repo.Store, videoID uint, ytVideoID string, publishAt *time.Time, visibility model.Visibility) error {
	full, err := ytapi.FetchVideo(ctx, svc, ytVideoID)
	if err != nil {
		return err
	}
	if full.Status == nil {
		full.Status = &youtube.VideoStatus{}
	}
	if publishAt != nil {
		full.Status.PrivacyStatus = "private"
		full.Status.PublishAt = publishAt.UTC().Format(time.RFC3339)
	} else {
		privacy := visibility.String()
		if privacy == "draft" || privacy == "scheduled" {
			// Draft and scheduled are local concepts; on the API
			// both are a private video with no publish time.
			privacy = "private"
		}
		full.Status.PrivacyStatus = privacy
		full.Status.PublishAt = ""
	}
	if _, err := svc.Videos.Update([]string{"snippet", "status"}, full).Context(ctx).Do(); err != nil {
		return err
	}
	storedVisibility := visibility
	if publishAt == nil && storedVisibility == model.VisibilityScheduled {
		storedVisibility = model.VisibilityPrivate
	}
	if publishAt != nil {
		storedVisibility = model.VisibilityScheduled
	}
	return store.Videos.UpdateSchedule(videoID, publishAt, storedVisibility)
}

// markEpisodeScheduled records the applied cadence and the first
// publish time on the episode row of the just-finished episode.
func markEpisodeScheduled(store *repo.Store, plan *Plan, lastEntry PlanEntry) error {
	stored, err := store.Episodes.GetByKey(plan.Rule.ChannelID, lastEntry.EpisodeKey)
	if err != nil {
		return err
	}
	firstPublishAt := firstEntryPublishAt(plan, lastEntry.EpisodeID)
	stored.CadenceDays = plan.Rule.CadenceDays
	stored.FirstPublishAt = &firstPublishAt
	return store.Episodes.UpsertByKey(stored)
}

// firstEntryPublishAt returns the publish time of an episode's
// first plan entry (its lowest sequence).
func firstEntryPublishAt(plan *Plan, episodeID uint) time.Time {
	for _, entry := range plan.Entries {
		if entry.EpisodeID == episodeID {
			return entry.PublishAt
		}
	}
	return time.Time{}
}
