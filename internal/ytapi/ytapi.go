// Package ytapi builds a YouTube Data API v3 service for a connected
// account and provides the small read helpers the syncer builds on.
// Every service is backed by an OAuth token source, so access tokens
// are refreshed automatically from the stored refresh token.
package ytapi

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/oauth"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// videoParts are the parts fetched for every video read.
var videoParts = []string{"snippet", "status", "contentDetails", "statistics"}

// NewService builds a YouTube service whose HTTP client refreshes
// access tokens from token through cfg automatically.
func NewService(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*youtube.Service, error) {
	client := oauth2.NewClient(ctx, cfg.TokenSource(ctx, token))
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create youtube service: %w", err)
	}
	return svc, nil
}

// ServiceForAccount builds a YouTube service for a stored account:
// it loads the account row, decrypts its refresh token with key, and
// builds the service from that refresh token.
func ServiceForAccount(ctx context.Context, home string, store *repo.Store, key []byte, accountID uint) (*youtube.Service, error) {
	account, err := store.Accounts.GetByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("load account %d: %w", accountID, err)
	}
	cfg, err := oauth.LoadClientConfig(home)
	if err != nil {
		return nil, err
	}
	if len(account.TokenCiphertext) == 0 || len(account.TokenNonce) == 0 {
		return nil, fmt.Errorf("account %d has no stored token: reconnect it with 'yt-manage account add'", accountID)
	}
	plaintext, err := cryptox.Decrypt(key, account.TokenCiphertext, account.TokenNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt token for account %d: %w", accountID, err)
	}
	refreshToken := string(plaintext)
	if refreshToken == "" {
		return nil, fmt.Errorf("account %d has an empty stored refresh token: reconnect it with 'yt-manage account add'", accountID)
	}
	svc, err := NewService(ctx, cfg, &oauth2.Token{RefreshToken: refreshToken})
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// MyChannel returns the channel owned by the authorised account
// (channels.list with mine=true, 1 quota unit).
func MyChannel(ctx context.Context, svc *youtube.Service) (*youtube.Channel, error) {
	resp, err := svc.Channels.List([]string{"snippet", "statistics", "contentDetails"}).Mine(true).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("channels.list mine: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("channels.list mine: no channel returned for this account")
	}
	return resp.Items[0], nil
}

// FetchVideo returns one video by its YouTube id (videos.list,
// 1 quota unit).
func FetchVideo(ctx context.Context, svc *youtube.Service, ytVideoID string) (*youtube.Video, error) {
	resp, err := svc.Videos.List(videoParts).Id(ytVideoID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("videos.list %s: %w", ytVideoID, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("videos.list %s: no video returned", ytVideoID)
	}
	return resp.Items[0], nil
}

// ListPlaylistItems returns every item of a playlist, paging through
// playlistItems.list at 50 items per page (1 quota unit per page).
// ListTopComments returns one page of top-level comment threads
// for a video (spec 29, item 97: sync mirrors comments). Replies
// are not fetched; one page keeps the quota cost at 1 unit.
func ListTopComments(ctx context.Context, svc *youtube.Service, ytVideoID string) ([]*youtube.CommentThread, error) {
	response, err := svc.CommentThreads.List([]string{"snippet"}).
		VideoId(ytVideoID).MaxResults(20).TextFormat("plainText").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return response.Items, nil
}

func ListPlaylistItems(ctx context.Context, svc *youtube.Service, playlistID string) ([]*youtube.PlaylistItem, error) {
	var items []*youtube.PlaylistItem
	pageToken := ""
	for {
		call := svc.PlaylistItems.List([]string{"snippet", "contentDetails"}).PlaylistId(playlistID).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("playlistItems.list %s: %w", playlistID, err)
		}
		items = append(items, resp.Items...)
		if resp.NextPageToken == "" {
			return items, nil
		}
		pageToken = resp.NextPageToken
	}
}

// ListVideosByIDs returns the videos for ids, fetched in batches of
// 50 (videos.list, 1 quota unit per batch).
func ListVideosByIDs(ctx context.Context, svc *youtube.Service, ids []string) ([]*youtube.Video, error) {
	var videos []*youtube.Video
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		resp, err := svc.Videos.List(videoParts).Id(batch...).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("videos.list batch of %d: %w", len(batch), err)
		}
		videos = append(videos, resp.Items...)
	}
	return videos, nil
}
