package batch

import (
	"context"
	"fmt"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

// Apply applies plan changes in order. Each change is written to
// the YouTube API first and only then mirrored locally. It stops
// at the first error and returns how many changes were applied
// before it.
func Apply(ctx context.Context, svc *youtube.Service, store *repo.Store, plan *Plan) (int, error) {
	applied := 0
	for _, change := range plan.Changes {
		if err := applyChange(ctx, svc, store, change); err != nil {
			return applied, fmt.Errorf("apply %s: %w", change.YTVideoID, err)
		}
		applied++
	}
	return applied, nil
}

// applyChange applies a single change: fetch the current video,
// update its snippet on YouTube, then update the mirror row.
func applyChange(ctx context.Context, svc *youtube.Service, store *repo.Store, change Change) error {
	full, err := ytapi.FetchVideo(ctx, svc, change.YTVideoID)
	if err != nil {
		return err
	}
	if full.Snippet == nil {
		full.Snippet = &youtube.VideoSnippet{}
	}
	full.Snippet.Title = change.NewTitle
	if change.HasDescriptionChange {
		full.Snippet.Description = change.NewDescription
	}
	resultingDescription := full.Snippet.Description
	if _, err := svc.Videos.Update([]string{"snippet", "status"}, full).Context(ctx).Do(); err != nil {
		return err
	}
	if err := store.Videos.UpdateTitleDescription(change.VideoID, change.NewTitle, resultingDescription); err != nil {
		return err
	}
	return nil
}
