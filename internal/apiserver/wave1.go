package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/oauth"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// Wave 1 (spec 29, items 1-25) endpoints: app status for onboarding
// and navigation, saved searches, schedule and comment lists, and
// the demo dataset.

const (
	settingSavedSearches = "ui.saved_searches"
	settingDemoSeeded    = "demo.seeded"
	demoYTChannelID      = "UC-DEMO-yt-manage"
	demoGoogleSubject    = "demo-yt-manage"
)

// statusChannel is one channel entry in the status response.
type statusChannel struct {
	ID          uint   `json:"id"`
	AccountID   uint   `json:"accountId"`
	Title       string `json:"title"`
	YTChannelID string `json:"ytChannelId"`
}

// statusSuggestion is one "what next" action computed from the data.
type statusSuggestion struct {
	Text    string `json:"text"`
	Section string `json:"section"`
}

// statusChecklist is the setup checklist state derived from real data.
type statusChecklist struct {
	Connect  bool `json:"connect"`
	Sync     bool `json:"sync"`
	Episodes bool `json:"episodes"`
	Schedule bool `json:"schedule"`
}

// statusResponse is the data shape of GET /api/status.
type statusResponse struct {
	AccountCount  int                `json:"accountCount"`
	ChannelCount  int                `json:"channelCount"`
	VideoCount    int64              `json:"videoCount"`
	EpisodeCount  int                `json:"episodeCount"`
	PlaylistCount int                `json:"playlistCount"`
	ScheduleCount int                `json:"scheduleCount"`
	CommentCount  int64              `json:"commentCount"`
	HasAccount    bool               `json:"hasAccount"`
	HasVideos     bool               `json:"hasVideos"`
	OAuthSource   string             `json:"oauthSource"`
	DataDir       string             `json:"dataDir"`
	DemoSeeded    bool               `json:"demoSeeded"`
	Channels      []statusChannel    `json:"channels"`
	Suggestions   []statusSuggestion `json:"suggestions"`
	Checklist     statusChecklist    `json:"checklist"`
	QuotaUsed     int                `json:"quotaUsed"`
	QuotaBudget   int                `json:"quotaBudget"`
	ReadOnly      bool               `json:"readOnly"`
	AppVersion    string             `json:"appVersion"`
	CommentsSync  string             `json:"commentsLastSyncAt"`
	AutoSync      autoSyncStatusDTO  `json:"autoSync"`
	ShareMode     bool               `json:"shareMode"`
	ShareExpires  string             `json:"shareExpiresAt"`
}

// oauthSource reports where the Google OAuth client comes from:
// the environment, a client_secret.json file, or nowhere yet.
func (s *Server) oauthSource() string {
	if os.Getenv("YT_MANAGE_CLIENT_ID") != "" && os.Getenv("YT_MANAGE_CLIENT_SECRET") != "" {
		return "env"
	}
	if _, err := os.Stat(config.ClientSecretPath(s.home)); err == nil {
		return "file"
	}
	if _, err := oauth.LoadClientConfig(s.home); err == nil {
		return "file"
	}
	return "none"
}

// handleStatus serves GET /api/status: counts, detected setup, the
// onboarding checklist state, and "what next" suggestions, all from
// the local mirror.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.Accounts.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	videoCount, err := s.store.Videos.CountMatching(repo.VideoQuery{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	commentCount, err := s.store.Comments.CountFirstComments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	episodeCount := 0
	playlistCount := 0
	scheduleCount := 0
	episodeLikeCount := 0
	for _, channel := range channels {
		channelEpisodes, err := s.store.Episodes.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		episodeCount += len(channelEpisodes)
		channelPlaylists, err := s.store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		playlistCount += len(channelPlaylists)
		rules, err := s.store.Schedules.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		scheduleCount += len(rules)
		if len(channelEpisodes) == 0 {
			videos, err := s.store.Videos.ListByChannel(channel.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			for _, video := range videos {
				if episodes.ParseTitle(video.Title).HasEpisode {
					episodeLikeCount++
				}
			}
		}
	}
	demoValue, _, err := s.store.Settings.Get(settingDemoSeeded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dataDir, err := config.DataDir()
	if err != nil {
		dataDir = s.home
	}
	response := statusResponse{
		AccountCount:  len(accounts),
		ChannelCount:  len(channels),
		VideoCount:    videoCount,
		EpisodeCount:  episodeCount,
		PlaylistCount: playlistCount,
		ScheduleCount: scheduleCount,
		CommentCount:  commentCount,
		HasAccount:    len(accounts) > 0,
		HasVideos:     videoCount > 0,
		OAuthSource:   s.oauthSource(),
		DataDir:       dataDir,
		DemoSeeded:    demoValue == "true",
		Channels:      make([]statusChannel, 0, len(channels)),
		Suggestions:   []statusSuggestion{},
		Checklist: statusChecklist{
			Connect:  len(accounts) > 0,
			Sync:     videoCount > 0,
			Episodes: episodeCount > 0,
			Schedule: scheduleCount > 0,
		},
	}
	for _, channel := range channels {
		response.Channels = append(response.Channels, statusChannel{
			ID: channel.ID, AccountID: channel.AccountID,
			Title: channel.Title, YTChannelID: channel.YTChannelID,
		})
	}
	response.Suggestions = buildSuggestions(response, episodeLikeCount)
	response.QuotaUsed, _ = s.quotaUsedToday()
	if cfg, cfgErr := config.LoadSettings(s.home); cfgErr == nil {
		response.QuotaBudget = cfg.DailyQuotaBudget
	}
	response.ReadOnly = s.ReadOnly()
	if shareActive, shareExpires := s.shareStateFor(r); shareActive {
		response.ShareMode = true
		response.ShareExpires = shareExpires
		response.ReadOnly = true
	}
	response.AppVersion = version.Version
	response.AutoSync = s.autoSyncStatus()
	for _, account := range accounts {
		state, stateErr := s.store.SyncStates.Get(account.ID, "comments")
		if stateErr == nil && state != nil && state.LastRunAt != nil {
			if response.CommentsSync == "" || state.LastRunAt.Format(time.RFC3339) > response.CommentsSync {
				response.CommentsSync = state.LastRunAt.Format(time.RFC3339)
			}
		}
	}
	writeOK(w, response)
}

// buildSuggestions computes the "what next" cards from the status.
func buildSuggestions(status statusResponse, episodeLikeCount int) []statusSuggestion {
	suggestions := []statusSuggestion{}
	if !status.HasAccount {
		suggestions = append(suggestions, statusSuggestion{
			Text: "Connect your first YouTube account to start mirroring a channel.", Section: "accounts",
		})
		return suggestions
	}
	if !status.HasVideos {
		suggestions = append(suggestions, statusSuggestion{
			Text: "Run your first sync to mirror your channels into the local database.", Section: "accounts",
		})
		return suggestions
	}
	if episodeLikeCount > 0 && status.EpisodeCount == 0 {
		suggestions = append(suggestions, statusSuggestion{
			Text: fmt.Sprintf("%d videos look like episodes - group them into episodes.", episodeLikeCount), Section: "episodes",
		})
	}
	if status.EpisodeCount > 0 && status.ScheduleCount == 0 {
		suggestions = append(suggestions, statusSuggestion{
			Text: "You have episodes but no schedule yet - plan a publish cadence.", Section: "schedules",
		})
	}
	if status.PlaylistCount == 0 {
		suggestions = append(suggestions, statusSuggestion{
			Text: "Create your first playlist to organise the mirrored videos.", Section: "playlists",
		})
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, statusSuggestion{
			Text: "Everything is set up. Search the mirror or plan the next schedule.", Section: "videos",
		})
	}
	return suggestions
}

// savedSearch is one saved search entry, persisted as JSON in the
// settings table under settingSavedSearches.
type savedSearch struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Text       string `json:"q"`
	ChannelID  uint   `json:"channelId"`
	Visibility string `json:"visibility"`
	Sort       string `json:"sort"`
}

// loadSavedSearches reads the saved searches from settings.
func (s *Server) loadSavedSearches() ([]savedSearch, error) {
	raw, found, err := s.store.Settings.Get(settingSavedSearches)
	if err != nil {
		return nil, err
	}
	if !found || raw == "" {
		return []savedSearch{}, nil
	}
	searches := []savedSearch{}
	if err := json.Unmarshal([]byte(raw), &searches); err != nil {
		return nil, fmt.Errorf("parse saved searches: %w", err)
	}
	return searches, nil
}

// storeSavedSearches writes the saved searches back to settings.
func (s *Server) storeSavedSearches(searches []savedSearch) error {
	data, err := json.Marshal(searches)
	if err != nil {
		return err
	}
	return s.store.Settings.Set(settingSavedSearches, string(data))
}

// handleListSavedSearches serves GET /api/saved-searches.
func (s *Server) handleListSavedSearches(w http.ResponseWriter, r *http.Request) {
	searches, err := s.loadSavedSearches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, searches)
}

// handleCreateSavedSearch serves POST /api/saved-searches.
func (s *Server) handleCreateSavedSearch(w http.ResponseWriter, r *http.Request) {
	var body savedSearch
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	searches, err := s.loadSavedSearches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id, err := randomHex(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	body.ID = id
	searches = append(searches, body)
	if err := s.storeSavedSearches(searches); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, searches)
}

// handleDeleteSavedSearch serves DELETE /api/saved-searches/{id}.
func (s *Server) handleDeleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	searches, err := s.loadSavedSearches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	kept := make([]savedSearch, 0, len(searches))
	for _, search := range searches {
		if search.ID != id {
			kept = append(kept, search)
		}
	}
	if err := s.storeSavedSearches(kept); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, kept)
}

// scheduleDTO is the wire shape of one schedule rule.
type scheduleDTO struct {
	ID               uint   `json:"id"`
	ChannelID        uint   `json:"channelId"`
	CadenceDays      int    `json:"cadenceDays"`
	VideosPerEpisode int    `json:"videosPerEpisode"`
	StartsAt         string `json:"startsAt"`
	IsActive         bool   `json:"isActive"`
}

// handleListSchedules serves GET /api/schedules across all channels.
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := []scheduleDTO{}
	for _, channel := range channels {
		rules, err := s.store.Schedules.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, rule := range rules {
			dtos = append(dtos, scheduleDTO{
				ID: rule.ID, ChannelID: rule.ChannelID, CadenceDays: rule.CadenceDays,
				VideosPerEpisode: rule.VideosPerEpisode, StartsAt: formatTimePtr(rule.StartsAt),
				IsActive: rule.IsActive,
			})
		}
	}
	writeOK(w, dtos)
}

// commentDTO is the wire shape of one managed first comment.
type commentDTO struct {
	ID            uint   `json:"id"`
	VideoID       uint   `json:"videoId"`
	VideoTitle    string `json:"videoTitle"`
	YTVideoID     string `json:"ytVideoId"`
	Text          string `json:"text"`
	PinnedText    string `json:"pinnedText,omitempty"`
	IsPinnedKnown bool   `json:"isPinnedKnown"`
	PublishedAt   string `json:"publishedAt"`
}

// handleListComments serves GET /api/comments: the managed first
// comments with their video titles, newest first.
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	comments, err := s.store.Comments.ListFirstComments(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]commentDTO, 0, len(comments))
	for _, comment := range comments {
		dto := commentDTO{
			ID: comment.ID, VideoID: comment.VideoID, Text: comment.Text,
			PinnedText:    comment.PinnedText,
			IsPinnedKnown: comment.IsPinnedKnown, PublishedAt: formatTimePtr(comment.PublishedAt),
		}
		if video, err := s.store.Videos.GetByID(comment.VideoID); err == nil {
			dto.VideoTitle = video.Title
			dto.YTVideoID = video.YTVideoID
		}
		dtos = append(dtos, dto)
	}
	writeOK(w, dtos)
}

// demoSeedResponse is the data shape of POST /api/demo/seed.
type demoSeedResponse struct {
	Videos    int `json:"videos"`
	Episodes  int `json:"episodes"`
	Playlists int `json:"playlists"`
}

// demoVideoSpec is one row of the demo dataset.
type demoVideoSpec struct {
	YTID       string
	Title      string
	Visibility model.Visibility
	Views      int64
	Comments   int64
	DaysAgo    int
	IsShort    bool
}

// demoVideos is the bundled sample dataset: two episode series with
// inconsistent markers, a short, and a scheduled/draft pair.
func demoVideos() []demoVideoSpec {
	return []demoVideoSpec{
		{"DEMOvid001", "Harbour Walks EP1 Part 1 - The Old Jetty", model.VisibilityPublic, 15200, 214, 40, false},
		{"DEMOvid002", "Harbour Walks EP1 Part 2 - Boats at Dawn", model.VisibilityPublic, 11800, 156, 40, false},
		{"DEMOvid003", "Harbour Walks EP 02 part 1 - The Lighthouse", model.VisibilityPublic, 20400, 301, 33, false},
		{"DEMOvid004", "Harbour Walks EP 02 part 2 - Storm Coming", model.VisibilityPublic, 9900, 121, 33, false},
		{"DEMOvid005", "Harbour Walks EP3 - The Ferry Crossing", model.VisibilityPublic, 30100, 402, 26, false},
		{"DEMOvid006", "Mountain Trails EP1 Part 1 - Base Camp", model.VisibilityPublic, 8400, 98, 19, false},
		{"DEMOvid007", "Mountain Trails EP1 Part 2 - The Ridge", model.VisibilityUnlisted, 3100, 44, 19, false},
		{"DEMOvid008", "Mountain Trails EP2 - Summit Day", model.VisibilityScheduled, 0, 0, -7, false},
		{"DEMOvid009", "Harbour Walks - Behind the Scenes", model.VisibilityPrivate, 120, 3, 12, false},
		{"DEMOvid010", "Quick Tip: Reading a Tide Table", model.VisibilityPublic, 45200, 620, 5, true},
		{"DEMOvid011", "Harbour Walks EP4 - Night Ferry (Draft)", model.VisibilityDraft, 0, 0, 0, false},
		{"DEMOvid012", "Channel Update - What Is Coming", model.VisibilityPublic, 6700, 210, 2, false},
	}
}

// handleDemoSeed serves POST /api/demo/seed: it creates the clearly
// labelled demo account, channel, videos, one playlist, and grouped
// episodes, entirely in the local mirror. No YouTube calls are made.
func (s *Server) handleDemoSeed(w http.ResponseWriter, r *http.Request) {
	account, err := s.store.Accounts.GetByGoogleSubject(demoGoogleSubject)
	if err != nil {
		if !isNotFound(err) {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		account = &model.Account{Label: "Demo account", GoogleSubject: demoGoogleSubject, ChannelID: demoYTChannelID}
		if err := s.store.Accounts.Create(account); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	now := time.Now()
	channel := &model.Channel{
		AccountID: account.ID, YTChannelID: demoYTChannelID, Title: "Demo Channel",
		Handle: "@demochannel", SubscriberCount: 12500, VideoCount: 12, LastSyncedAt: &now,
	}
	if err := s.store.Channels.UpsertByYTID(channel); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	storedChannel, err := s.store.Channels.GetByYTID(demoYTChannelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	specs := demoVideos()
	for _, spec := range specs {
		published := now.AddDate(0, 0, -spec.DaysAgo)
		video := &model.Video{
			ChannelID: storedChannel.ID, YTVideoID: spec.YTID, Title: spec.Title,
			Description: "Demo video from the bundled sample dataset. " + spec.Title,
			Visibility:  spec.Visibility, IsShort: spec.IsShort, PublishedAt: &published,
			ViewCount: spec.Views, CommentCount: spec.Comments, LastSyncedAt: &now,
		}
		if spec.Visibility == model.VisibilityScheduled {
			publishAt := now.AddDate(0, 0, 7)
			video.PublishAt = &publishAt
			video.PublishedAt = nil
		}
		if err := s.store.Videos.UpsertByYTID(video); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	playlist := &model.Playlist{
		ChannelID: storedChannel.ID, YTPlaylistID: "PL-DEMO-yt-manage",
		Title: "Demo Playlist - Harbour Walks", Description: "Sample playlist from the demo dataset.", ItemCount: 6,
	}
	if err := s.store.Playlists.UpsertByYTID(playlist); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := episodes.GroupChannel(s.store, storedChannel.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Settings.Set(settingDemoSeeded, "true"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, demoSeedResponse{Videos: len(specs), Episodes: len(result.Groups), Playlists: 1})
}

// handleDemoClear serves POST /api/demo/clear: it removes only the
// demo dataset (account, channel, videos, playlists, episodes).
func (s *Server) handleDemoClear(w http.ResponseWriter, r *http.Request) {
	channel, err := s.store.Channels.GetByYTID(demoYTChannelID)
	if err != nil {
		if isNotFound(err) {
			writeOK(w, map[string]bool{"cleared": true})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Episodes.DeleteByChannel(channel.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Playlists.DeleteByChannel(channel.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Videos.DeleteByChannel(channel.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Channels.Delete(channel.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if account, err := s.store.Accounts.GetByGoogleSubject(demoGoogleSubject); err == nil {
		if err := s.store.Accounts.Delete(account.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if err := s.store.Settings.Set(settingDemoSeeded, "false"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]bool{"cleared": true})
}
