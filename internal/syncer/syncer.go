// Package syncer mirrors a connected account's channel, videos and
// playlists into the local split databases through the repository
// layer. It estimates the quota cost before a run and refuses to
// start a run that would exceed the configured daily budget.
package syncer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

// Result summarises one account sync run.
type Result struct {
	AccountID     uint
	ChannelTitle  string
	Videos        int
	Playlists     int
	Comments      int
	SkippedItems  int
	UnitsEstimate int
	IsIncremental bool
}

// EstimateUnits returns the lower-bound quota cost of a sync run:
// 1 unit for channels.list, one uploads playlistItems page per 50
// videos, one videos.list batch per 50 videos, 1 unit for
// playlists.list, and one playlistItems page per known playlist.
// Comment mirroring (item 97) adds 1 unit per video actually
// fetched, counted into the run's result after the fact, since it
// depends on which videos report having comments.
// It is pure: no network and no database access.
func EstimateUnits(videoCount int64, playlistCount int) int {
	units := 1
	units += ceilDiv50(videoCount)
	units += ceilDiv50(videoCount)
	units += 1
	units += playlistCount
	return units
}

// ceilDiv50 returns ceil(n/50) for n > 0 and 0 otherwise.
func ceilDiv50(n int64) int {
	if n <= 0 {
		return 0
	}
	return int((n + 49) / 50)
}

// RunAccount syncs one account. When isFull is false and a previous
// successful videos run exists, the run is incremental: videos that
// are already mirrored and were published before the last run are
// not fetched again. When budget is positive the run is refused
// before it starts if its estimate exceeds the budget.
func RunAccount(ctx context.Context, home string, store *repo.Store, key []byte, accountID uint, isFull bool, budget int) (*Result, error) {
	svc, err := ytapi.ServiceForAccount(ctx, home, store, key, accountID)
	if err != nil {
		return nil, err
	}
	ch, err := ytapi.MyChannel(ctx, svc)
	if err != nil {
		return nil, fmt.Errorf("sync account %d: %w", accountID, err)
	}

	now := time.Now()
	localChannel, err := ensureLocalChannel(store, accountID, ch, now)
	if err != nil {
		return nil, err
	}

	existingPlaylists, err := store.Playlists.ListByChannel(localChannel.ID)
	if err != nil {
		return nil, fmt.Errorf("list mirrored playlists for channel %d: %w", localChannel.ID, err)
	}
	var channelVideoCount int64
	if ch.Statistics != nil {
		channelVideoCount = int64(ch.Statistics.VideoCount)
	}
	estimate := EstimateUnits(channelVideoCount, len(existingPlaylists))
	result := &Result{
		AccountID:     accountID,
		ChannelTitle:  channelTitle(ch),
		UnitsEstimate: estimate,
	}
	if budget > 0 && estimate > budget {
		budgetErr := fmt.Errorf("sync account %d: estimated quota %d units exceeds daily budget %d units", accountID, estimate, budget)
		return nil, recordSyncError(store, accountID, "videos", budgetErr)
	}

	prev, prevErr := store.SyncStates.Get(accountID, "videos")
	hasPrev := prevErr == nil && prev != nil
	isIncremental := false
	var prevRunAt *time.Time
	if hasPrev {
		prevRunAt = prev.LastRunAt
	}
	if !isFull && hasPrev && prevRunAt != nil {
		isIncremental = true
	}
	result.IsIncremental = isIncremental

	if err := syncVideos(ctx, svc, store, localChannel.ID, ch, isIncremental, prevRunAt, result); err != nil {
		return nil, recordSyncError(store, accountID, "videos", err)
	}
	skipped, playlistCount, err := syncPlaylists(ctx, svc, store, localChannel.ID)
	if err != nil {
		return nil, recordSyncError(store, accountID, "playlists", err)
	}
	result.SkippedItems = skipped
	result.Playlists = playlistCount

	commentCount, commentVideos, err := syncComments(ctx, svc, store, localChannel.ID)
	if err != nil {
		return nil, recordSyncError(store, accountID, "comments", err)
	}
	result.Comments = commentCount
	result.UnitsEstimate += commentVideos

	refreshedAt := time.Now()
	if err := store.Channels.UpsertByYTID(channelModel(accountID, ch, refreshedAt)); err != nil {
		return nil, fmt.Errorf("refresh channel %s: %w", ch.Id, err)
	}
	if err := updateAccountChannel(store, accountID, ch.Id); err != nil {
		return nil, err
	}

	runAt := time.Now()
	if err := recordSyncSuccess(store, accountID, "videos", runAt); err != nil {
		return nil, err
	}
	if err := recordSyncSuccess(store, accountID, "playlists", runAt); err != nil {
		return nil, err
	}
	if err := recordSyncSuccess(store, accountID, "comments", runAt); err != nil {
		return nil, err
	}
	return result, nil
}

// maxCommentVideosPerSync caps the comment mirroring of one run:
// at most 50 videos, one page of top-level comments each, so a
// sync stays within a predictable quota envelope (item 97).
const maxCommentVideosPerSync = 50

// syncComments mirrors top-level comments for the channel's
// videos that report having comments. A video whose comment list
// fails (comments disabled between listing and fetch, quota, ...)
// is skipped without failing the sync. A comment that is already
// stored as the managed first comment keeps that flag.
func syncComments(ctx context.Context, svc *youtube.Service, store *repo.Store, localChannelID uint) (comments int, videosFetched int, err error) {
	videos, err := store.Videos.ListByChannel(localChannelID)
	if err != nil {
		return 0, 0, err
	}
	for _, video := range videos {
		if video.CommentCount <= 0 || videosFetched >= maxCommentVideosPerSync {
			continue
		}
		threads, fetchErr := ytapi.ListTopComments(ctx, svc, video.YTVideoID)
		if fetchErr != nil {
			continue
		}
		videosFetched++
		for _, thread := range threads {
			if thread.Snippet == nil || thread.Snippet.TopLevelComment == nil || thread.Snippet.TopLevelComment.Snippet == nil {
				continue
			}
			top := thread.Snippet.TopLevelComment
			snippet := top.Snippet
			isFirst := false
			if existing, getErr := store.Comments.GetByYTID(top.Id); getErr == nil && existing != nil {
				isFirst = existing.IsFirst
			}
			published, _ := time.Parse(time.RFC3339, snippet.PublishedAt)
			comment := &model.Comment{
				VideoID: video.ID, YTCommentID: top.Id,
				Author: snippet.AuthorDisplayName, Text: snippet.TextOriginal,
				IsFirst: isFirst, PublishedAt: &published,
			}
			if upsertErr := store.Comments.UpsertByYTID(comment); upsertErr != nil {
				return comments, videosFetched, upsertErr
			}
			comments++
		}
	}
	return comments, videosFetched, nil
}

// ensureLocalChannel returns the mirrored channel row for ch,
// creating it from the API data first when it does not exist yet.
func ensureLocalChannel(store *repo.Store, accountID uint, ch *youtube.Channel, now time.Time) (*model.Channel, error) {
	existing, err := store.Channels.GetByYTID(ch.Id)
	if err == nil {
		return existing, nil
	}
	if err := store.Channels.UpsertByYTID(channelModel(accountID, ch, now)); err != nil {
		return nil, fmt.Errorf("create mirrored channel %s: %w", ch.Id, err)
	}
	created, err := store.Channels.GetByYTID(ch.Id)
	if err != nil {
		return nil, fmt.Errorf("load mirrored channel %s after create: %w", ch.Id, err)
	}
	return created, nil
}

// channelModel maps an API channel to its mirrored row.
func channelModel(accountID uint, ch *youtube.Channel, syncedAt time.Time) *model.Channel {
	channel := &model.Channel{
		AccountID:    accountID,
		YTChannelID:  ch.Id,
		Title:        channelTitle(ch),
		LastSyncedAt: &syncedAt,
	}
	if ch.Snippet != nil {
		channel.Handle = ch.Snippet.CustomUrl
	}
	if ch.Statistics != nil {
		channel.SubscriberCount = int64(ch.Statistics.SubscriberCount)
		channel.VideoCount = int64(ch.Statistics.VideoCount)
	}
	return channel
}

// channelTitle returns the API channel's title, tolerating a missing
// snippet.
func channelTitle(ch *youtube.Channel) string {
	if ch.Snippet == nil {
		return ""
	}
	return ch.Snippet.Title
}

// syncVideos fetches the uploads playlist, resolves which videos to
// fetch, and upserts them into the mirror.
func syncVideos(ctx context.Context, svc *youtube.Service, store *repo.Store, localChannelID uint, ch *youtube.Channel, isIncremental bool, prevRunAt *time.Time, result *Result) error {
	if ch.ContentDetails == nil || ch.ContentDetails.RelatedPlaylists == nil || ch.ContentDetails.RelatedPlaylists.Uploads == "" {
		return fmt.Errorf("channel %s has no uploads playlist", ch.Id)
	}
	uploadsID := ch.ContentDetails.RelatedPlaylists.Uploads
	items, err := ytapi.ListPlaylistItems(ctx, svc, uploadsID)
	if err != nil {
		return err
	}
	idsToFetch := collectVideoIDs(store, items, isIncremental, prevRunAt)
	videos, err := ytapi.ListVideosByIDs(ctx, svc, idsToFetch)
	if err != nil {
		return err
	}
	syncedAt := time.Now()
	upserted := 0
	for _, video := range videos {
		mapped, err := videoModel(localChannelID, video, syncedAt)
		if err != nil {
			return err
		}
		if err := store.Videos.UpsertByYTID(mapped); err != nil {
			return fmt.Errorf("upsert video %s: %w", video.Id, err)
		}
		upserted++
	}
	result.Videos = upserted
	return nil
}

// collectVideoIDs returns the YouTube ids to fetch from uploads
// playlist items (newest first). In an incremental run, a video that
// is already mirrored and was published before the previous run is
// known and skipped; every other id is collected.
func collectVideoIDs(store *repo.Store, items []*youtube.PlaylistItem, isIncremental bool, prevRunAt *time.Time) []string {
	var ids []string
	for _, item := range items {
		if item.ContentDetails == nil || item.ContentDetails.VideoId == "" {
			continue
		}
		ytID := item.ContentDetails.VideoId
		if isIncremental && prevRunAt != nil {
			existing, err := store.Videos.GetByYTID(ytID)
			if err == nil && existing != nil && existing.PublishedAt != nil && existing.PublishedAt.Before(*prevRunAt) {
				continue
			}
		}
		ids = append(ids, ytID)
	}
	return ids
}

// videoModel maps an API video to its mirrored row.
func videoModel(localChannelID uint, video *youtube.Video, syncedAt time.Time) (*model.Video, error) {
	mapped := &model.Video{
		ChannelID:    localChannelID,
		YTVideoID:    video.Id,
		Visibility:   model.VisibilityPrivate,
		LastSyncedAt: &syncedAt,
	}
	if video.Snippet != nil {
		mapped.Title = video.Snippet.Title
		mapped.Description = video.Snippet.Description
		if video.Snippet.PublishedAt != "" {
			publishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return nil, fmt.Errorf("parse published_at of video %s: %w", video.Id, err)
			}
			mapped.PublishedAt = &publishedAt
		}
	}
	if video.ContentDetails != nil {
		mapped.DurationSeconds = parseDurationSeconds(video.ContentDetails.Duration)
	}
	// IsShort is a duration-based approximation: YouTube exposes no
	// direct flag on videos.list, and Shorts are at most 60 seconds.
	mapped.IsShort = mapped.DurationSeconds > 0 && mapped.DurationSeconds <= 60
	if video.Status != nil {
		if video.Status.PublishAt != "" {
			publishAt, err := time.Parse(time.RFC3339, video.Status.PublishAt)
			if err != nil {
				return nil, fmt.Errorf("parse publish_at of video %s: %w", video.Id, err)
			}
			mapped.PublishAt = &publishAt
			mapped.Visibility = model.VisibilityScheduled
		} else {
			visibility, err := model.ParseVisibility(video.Status.PrivacyStatus)
			if err == nil {
				mapped.Visibility = visibility
			}
		}
	}
	if video.Statistics != nil {
		// The API client decodes the string-encoded counts into
		// uint64 fields, so they convert directly to int64.
		mapped.ViewCount = int64(video.Statistics.ViewCount)
		mapped.LikeCount = int64(video.Statistics.LikeCount)
		mapped.CommentCount = int64(video.Statistics.CommentCount)
	}
	return mapped, nil
}

// parseDurationSeconds parses an ISO 8601 duration of the form
// PT#H#M#S (the shape YouTube returns for videos) into seconds.
// A malformed value yields 0.
func parseDurationSeconds(duration string) int64 {
	if !strings.HasPrefix(duration, "PT") {
		return 0
	}
	rest := duration[2:]
	var total int64
	number := int64(0)
	hasNumber := false
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			number = number*10 + int64(r-'0')
			hasNumber = true
			continue
		}
		if !hasNumber {
			return 0
		}
		switch r {
		case 'H':
			total += number * 3600
		case 'M':
			total += number * 60
		case 'S':
			total += number
		default:
			return 0
		}
		number = 0
		hasNumber = false
	}
	if hasNumber {
		return 0
	}
	return total
}

// syncPlaylists mirrors every playlist of the channel and replaces
// each playlist's items with the local video ids. Items whose video
// is not mirrored are skipped and counted.
func syncPlaylists(ctx context.Context, svc *youtube.Service, store *repo.Store, localChannelID uint) (skipped int, count int, err error) {
	pageToken := ""
	for {
		call := svc.Playlists.List([]string{"snippet", "contentDetails"}).Mine(true).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return skipped, count, fmt.Errorf("playlists.list mine: %w", err)
		}
		for _, playlist := range resp.Items {
			playlistSkipped, err := syncPlaylist(ctx, svc, store, localChannelID, playlist)
			if err != nil {
				return skipped, count, err
			}
			skipped += playlistSkipped
			count++
		}
		if resp.NextPageToken == "" {
			return skipped, count, nil
		}
		pageToken = resp.NextPageToken
	}
}

// syncPlaylist upserts one playlist and replaces its items.
func syncPlaylist(ctx context.Context, svc *youtube.Service, store *repo.Store, localChannelID uint, playlist *youtube.Playlist) (int, error) {
	mapped := &model.Playlist{
		ChannelID:    localChannelID,
		YTPlaylistID: playlist.Id,
	}
	if playlist.Snippet != nil {
		mapped.Title = playlist.Snippet.Title
		mapped.Description = playlist.Snippet.Description
	}
	if playlist.ContentDetails != nil {
		mapped.ItemCount = playlist.ContentDetails.ItemCount
	}
	if err := store.Playlists.UpsertByYTID(mapped); err != nil {
		return 0, fmt.Errorf("upsert playlist %s: %w", playlist.Id, err)
	}
	local, err := store.Playlists.GetByYTID(playlist.Id)
	if err != nil {
		return 0, fmt.Errorf("load mirrored playlist %s after upsert: %w", playlist.Id, err)
	}
	items, err := ytapi.ListPlaylistItems(ctx, svc, playlist.Id)
	if err != nil {
		return 0, err
	}
	skipped := 0
	var repoItems []model.PlaylistItem
	for index, item := range items {
		if item.ContentDetails == nil || item.ContentDetails.VideoId == "" {
			skipped++
			continue
		}
		video, err := store.Videos.GetByYTID(item.ContentDetails.VideoId)
		if err != nil {
			skipped++
			continue
		}
		repoItems = append(repoItems, model.PlaylistItem{
			PlaylistID: local.ID,
			VideoID:    video.ID,
			Position:   index,
		})
	}
	if err := store.Playlists.ReplaceItems(local.ID, repoItems); err != nil {
		return skipped, fmt.Errorf("replace items of playlist %s: %w", playlist.Id, err)
	}
	return skipped, nil
}

// updateAccountChannel points the account row at ch's YouTube
// channel id when it does not already.
func updateAccountChannel(store *repo.Store, accountID uint, ytChannelID string) error {
	account, err := store.Accounts.GetByID(accountID)
	if err != nil {
		return fmt.Errorf("load account %d: %w", accountID, err)
	}
	if account.ChannelID == ytChannelID {
		return nil
	}
	account.ChannelID = ytChannelID
	if err := store.Accounts.Update(account); err != nil {
		return fmt.Errorf("update channel of account %d: %w", accountID, err)
	}
	return nil
}

// recordSyncSuccess marks a resource as synced at runAt with no
// error.
func recordSyncSuccess(store *repo.Store, accountID uint, resource string, runAt time.Time) error {
	state := &model.SyncState{
		AccountID: accountID,
		Resource:  resource,
		LastRunAt: &runAt,
		LastError: "",
	}
	if err := store.SyncStates.Upsert(state); err != nil {
		return fmt.Errorf("record sync state for account %d resource %s: %w", accountID, resource, err)
	}
	return nil
}

// recordSyncError stores runErr as the resource's last error while
// keeping the previous successful run time, and returns runErr (or a
// combined error when recording itself fails).
func recordSyncError(store *repo.Store, accountID uint, resource string, runErr error) error {
	state := &model.SyncState{
		AccountID: accountID,
		Resource:  resource,
		LastError: runErr.Error(),
	}
	if prev, err := store.SyncStates.Get(accountID, resource); err == nil && prev != nil {
		state.LastRunAt = prev.LastRunAt
		state.Cursor = prev.Cursor
	}
	if err := store.SyncStates.Upsert(state); err != nil {
		return fmt.Errorf("%w; also failed to record sync error for resource %s: %v", runErr, resource, err)
	}
	return runErr
}
