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

// ApplyResult is the per-change outcome of ApplyEach.
type ApplyResult struct {
	VideoID   uint
	YTVideoID string
	IsOK      bool
	Error     string
}

// ApplyEach applies plan changes one by one and keeps going past
// failures, returning one result per change. It powers the UI's
// per-item progress and retry-failures flow (spec 29, items 36-37).
func ApplyEach(ctx context.Context, svc *youtube.Service, store *repo.Store, plan *Plan) []ApplyResult {
	results := make([]ApplyResult, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		result := ApplyResult{VideoID: change.VideoID, YTVideoID: change.YTVideoID, IsOK: true}
		if err := applyChange(ctx, svc, store, change); err != nil {
			result.IsOK = false
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
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
	if change.NewTitle != change.OldTitle && store.TitleHistory != nil {
		// Rename history is best-effort: a history write failure
		// must not fail a rename YouTube already accepted.
		_ = store.TitleHistory.Record(change.VideoID, change.OldTitle, change.NewTitle, "batch")
	}
	return nil
}
