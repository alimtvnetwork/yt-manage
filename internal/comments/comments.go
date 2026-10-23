// Package comments posts and edits the first comment on a video
// through the YouTube Data API and mirrors the result in the local
// store. The API has no endpoint for pinning a comment, so pinning
// is always surfaced as a manual YouTube Studio step.
package comments

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// PostFirstComment posts text as the first comment on video. It
// refuses when a first comment is already recorded for the video;
// use EditFirstComment in that case. The comment is created through
// the API first and mirrored second.
func PostFirstComment(ctx context.Context, svc *youtube.Service, store *repo.Store, video model.Video, text string) (*model.Comment, error) {
	if _, err := store.Comments.GetFirstByVideo(video.ID); err == nil {
		return nil, fmt.Errorf("a first comment already exists for video %d: use edit instead", video.ID)
	}
	resp, err := svc.CommentThreads.Insert([]string{"snippet"}, &youtube.CommentThread{
		Snippet: &youtube.CommentThreadSnippet{
			VideoId: video.YTVideoID,
			TopLevelComment: &youtube.Comment{
				Snippet: &youtube.CommentSnippet{TextOriginal: text},
			},
		},
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("post first comment on video %s: %w", video.YTVideoID, err)
	}
	if resp.Snippet == nil || resp.Snippet.TopLevelComment == nil || resp.Snippet.TopLevelComment.Snippet == nil {
		return nil, fmt.Errorf("post first comment on video %s: API returned no top-level comment", video.YTVideoID)
	}
	now := time.Now()
	record := &model.Comment{
		VideoID:       video.ID,
		YTCommentID:   resp.Snippet.TopLevelComment.Id,
		Author:        resp.Snippet.TopLevelComment.Snippet.AuthorDisplayName,
		Text:          text,
		IsFirst:       true,
		IsPinnedKnown: false,
		PublishedAt:   &now,
	}
	if err := store.Comments.UpsertByYTID(record); err != nil {
		return nil, fmt.Errorf("mirror first comment on video %d: %w", video.ID, err)
	}
	return record, nil
}

// EditFirstComment updates the text of the first comment previously
// recorded for video, through the API first and the mirror second.
func EditFirstComment(ctx context.Context, svc *youtube.Service, store *repo.Store, video model.Video, newText string) (*model.Comment, error) {
	existing, err := store.Comments.GetFirstByVideo(video.ID)
	if err != nil {
		return nil, fmt.Errorf("no first comment recorded for video %d: %w", video.ID, err)
	}
	if _, err := svc.Comments.Update([]string{"snippet"}, &youtube.Comment{
		Id:      existing.YTCommentID,
		Snippet: &youtube.CommentSnippet{TextOriginal: newText},
	}).Context(ctx).Do(); err != nil {
		return nil, fmt.Errorf("edit first comment %s: %w", existing.YTCommentID, err)
	}
	if err := store.Comments.UpdateText(existing.ID, newText); err != nil {
		return nil, fmt.Errorf("mirror edited comment %d: %w", existing.ID, err)
	}
	existing.Text = newText
	return existing, nil
}

// PinGuidance returns the manual pinning instructions for a video.
// The YouTube Data API has no comment-pin endpoint, so this guidance
// is printed after every post or edit instead of a simulated pin.
func PinGuidance(ytVideoID string) string {
	return fmt.Sprintf("Note: the YouTube Data API has no endpoint for pinning a comment, so pinning is a manual step: open https://www.youtube.com/watch?v=%s in YouTube Studio or your browser, find this comment, open its ⋮ menu and choose 'Pin'. yt-manage never fakes a pin.", ytVideoID)
}
