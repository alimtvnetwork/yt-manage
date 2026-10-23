package mailer

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// watchWatermarkKey is the settings key holding the RFC3339 time
// of the last publish-notification pass.
const watchWatermarkKey = "mail.watch.last_check"

// PublishNotifications runs one pass of the publish watcher: it
// emails one notification per video published since the stored
// watermark and logs each send. On the very first run it only sets
// the watermark, so the whole back catalogue is never mailed.
func PublishNotifications(ctx context.Context, store *repo.Store, s config.Settings, to string) (int, error) {
	now := time.Now()
	value, hasWatermark, err := store.Settings.Get(watchWatermarkKey)
	if err != nil {
		return 0, fmt.Errorf("read mail watch watermark: %w", err)
	}
	if !hasWatermark || value == "" {
		if err := store.Settings.Set(watchWatermarkKey, now.Format(time.RFC3339)); err != nil {
			return 0, fmt.Errorf("set mail watch watermark: %w", err)
		}
		return 0, nil
	}
	watermark, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, fmt.Errorf("parse mail watch watermark: %w", err)
	}
	if watermark.IsZero() {
		if err := store.Settings.Set(watchWatermarkKey, now.Format(time.RFC3339)); err != nil {
			return 0, fmt.Errorf("set mail watch watermark: %w", err)
		}
		return 0, nil
	}
	candidates, err := store.Videos.Search(repo.VideoQuery{
		Text:           "",
		Visibility:     string(model.VisibilityPublic),
		PublishedAfter: &watermark,
		Limit:          500,
	})
	if err != nil {
		return 0, fmt.Errorf("search newly published videos: %w", err)
	}
	sent := 0
	for _, video := range candidates {
		subject := "Published: " + video.Title
		body := publishNotificationBody(video)
		if err := Send(s, to, subject, body); err != nil {
			return sent, err
		}
		entry := &model.EmailLog{
			Direction:      model.EmailDirectionSent,
			Subject:        subject,
			RelatedVideoID: video.ID,
			SentAt:         &now,
		}
		if err := store.EmailLogs.Create(entry); err != nil {
			return sent, fmt.Errorf("log publish notification for video %d: %w", video.ID, err)
		}
		sent++
	}
	if err := store.Settings.Set(watchWatermarkKey, now.Format(time.RFC3339)); err != nil {
		return sent, fmt.Errorf("set mail watch watermark: %w", err)
	}
	return sent, nil
}

// publishNotificationBody renders the notification body for one
// newly published video.
func publishNotificationBody(video model.Video) string {
	published := "unknown"
	if video.PublishedAt != nil {
		published = video.PublishedAt.Format(time.RFC3339)
	}
	return fmt.Sprintf("A video was published.\n\nTitle: %s\nWatch: https://www.youtube.com/watch?v=%s\nPublished: %s\n", video.Title, video.YTVideoID, published)
}

// Watch runs PublishNotifications on a ticker until ctx is done.
// Each pass reports its sent count (or its error) to out with a
// timestamp; a failed pass never stops the loop.
func Watch(ctx context.Context, store *repo.Store, s config.Settings, to string, interval time.Duration, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	runWatchPass(ctx, store, s, to, out)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runWatchPass(ctx, store, s, to, out)
		}
	}
}

// runWatchPass performs one watcher pass and reports its outcome.
func runWatchPass(ctx context.Context, store *repo.Store, s config.Settings, to string, out io.Writer) {
	stamp := time.Now().Format(time.RFC3339)
	sent, err := PublishNotifications(ctx, store, s, to)
	if err != nil {
		fmt.Fprintf(out, "%s mail watch: %v\n", stamp, err)
		return
	}
	fmt.Fprintf(out, "%s mail watch: sent %d notification(s)\n", stamp, sent)
}
