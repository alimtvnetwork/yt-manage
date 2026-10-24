package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/comments"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/mailer"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/syncer"
	"github.com/alimtvnetwork/yt-manage/internal/webhook"
)

// Background automation (spec 29, items 111, 114, 115, 117,
// 120): a one-minute loop inside the serve process that runs due
// auto-syncs, evaluates rules, drafts schedule slots for watched
// series, runs post-publish pipelines, and flushes webhook
// events held back by quiet hours. It only runs while
// `yt-manage serve` is up; the status endpoint says when the
// next auto-sync is due so nothing about it is hidden.

const autoSyncPausedKey = "autosync:paused"
const heldCountKey = "automation:held_count"

// startAutoLoop launches the background loop; it ends when the
// process does.
func (s *Server) startAutoLoop() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.autoTick(context.Background())
		}
	}()
}

func (s *Server) autoTick(ctx context.Context) {
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		return
	}
	if cfg.InQuietHours(time.Now()) {
		// Count what this tick would have done as held; the first
		// tick after the window summarises it (item 117).
		if s.autoSyncDueCount(cfg) > 0 {
			s.bumpHeld(s.autoSyncDueCount(cfg))
		}
		return
	}
	if held, found, _ := s.store.Settings.Get(heldCountKey); found && held != "" && held != "0" {
		s.recordActivity("automation", fmt.Sprintf("%s automation run(s) were held during quiet hours and have now run", held), "")
		_ = s.store.Settings.Set(heldCountKey, "0")
	}
	s.flushHeldWebhooks(cfg)
	s.runDueAutoSyncs(ctx, cfg)
	s.runPipelines(ctx, cfg)
	s.runABTestSchedules(ctx)
	s.runNotifyTick(cfg)
}

// bumpHeld adds to the quiet-hours held counter.
func (s *Server) bumpHeld(count int) {
	current := 0
	if raw, found, err := s.store.Settings.Get(heldCountKey); err == nil && found {
		fmt.Sscanf(raw, "%d", &current)
	}
	_ = s.store.Settings.Set(heldCountKey, fmt.Sprintf("%d", current+count))
}

// autoSyncDueCount counts accounts whose auto-sync is due now.
func (s *Server) autoSyncDueCount(cfg *config.Settings) int {
	if paused, found, _ := s.store.Settings.Get(autoSyncPausedKey); found && paused == "true" {
		return 0
	}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		return 0
	}
	due := 0
	for _, account := range accounts {
		if s.accountAutoSyncDue(account.ID, time.Now()) {
			due++
		}
	}
	return due
}

// accountAutoSyncDue reports whether the account has a sync
// interval configured and its last sync is older than it.
func (s *Server) accountAutoSyncDue(accountID uint, now time.Time) bool {
	settings := s.accountSettings(accountID)
	if settings.SyncIntervalMin <= 0 {
		return false
	}
	state, err := s.store.SyncStates.Get(accountID, "videos")
	if err != nil || state == nil || state.LastRunAt == nil {
		return true
	}
	return now.Sub(*state.LastRunAt) >= time.Duration(settings.SyncIntervalMin)*time.Minute
}

// runDueAutoSyncs syncs every due account, then evaluates rules
// and watched series once (item 111).
func (s *Server) runDueAutoSyncs(ctx context.Context, cfg *config.Settings) {
	if paused, found, _ := s.store.Settings.Get(autoSyncPausedKey); found && paused == "true" {
		return
	}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		return
	}
	ranAny := false
	for _, account := range accounts {
		if !s.accountAutoSyncDue(account.ID, time.Now()) {
			continue
		}
		result, syncErr := syncer.RunAccount(ctx, s.home, s.store, s.key, account.ID, false, cfg.DailyQuotaBudget)
		if syncErr != nil {
			s.recordActivity("sync", fmt.Sprintf("Auto-sync of account %d failed: %v", account.ID, syncErr), "")
			s.alertFailure(fmt.Sprintf("Auto-sync of account %s failed: %v", account.Label, syncErr))
			continue
		}
		ranAny = true
		s.addQuota(result.UnitsEstimate)
		s.recordActivity("sync", fmt.Sprintf("Auto-synced %s: %d videos, %d playlists, %d comments", result.ChannelTitle, result.Videos, result.Playlists, result.Comments), "")
		s.fireWebhook(cfg, "sync.completed", map[string]interface{}{"accountId": account.ID, "videos": result.Videos})
	}
	if ranAny {
		s.evaluateAllRules(ctx)
		s.draftWatchedSeries(ctx)
	}
}

// draftWatchedSeries (item 115): for each channel's watched
// (ongoing) series, group any new parts and draft the next
// schedule slots as a proposal for approval. Drafting never
// applies anything by itself.
func (s *Server) draftWatchedSeries(ctx context.Context) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return
	}
	for _, channel := range channels {
		defaults := s.loadChannelDefaults(channel.ID)
		if len(defaults.WatchSeries) == 0 {
			continue
		}
		if _, err := episodes.GroupChannel(s.store, channel.ID); err != nil {
			continue
		}
		cadence := defaults.CadenceDays
		if cadence <= 0 {
			cadence = 7
		}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			continue
		}
		actions := []proposalAction{}
		slot := time.Now().AddDate(0, 0, 1)
		for _, video := range videos {
			parsed := episodes.ParseTitle(video.Title)
			watched := false
			for _, key := range defaults.WatchSeries {
				if strings.EqualFold(key, episodes.ShowKey(parsed.SeriesKey)) {
					watched = true
				}
			}
			if !watched || video.Visibility == model.VisibilityPublic || video.Visibility == model.VisibilityScheduled {
				continue
			}
			slot = slot.AddDate(0, 0, cadence)
			actions = append(actions, proposalAction{
				Type: "schedule", VideoID: video.ID, VideoTitle: video.Title,
				Detail:    fmt.Sprintf("Schedule for %s", slot.Format("2006-01-02 15:04")),
				PublishAt: slot.Format(time.RFC3339),
			})
		}
		if len(actions) == 0 {
			continue
		}
		proposals := s.loadProposals()
		proposals = append(proposals, proposal{
			ID: fmt.Sprintf("p%d", time.Now().UnixNano()), RuleName: "Watch: " + channel.Title,
			CreatedAt: time.Now().Format(time.RFC3339), Status: "pending", Actions: actions,
		})
		_ = s.saveProposals(proposals)
		s.recordActivity("automation", fmt.Sprintf("Watched series on %s: drafted %d schedule slot(s) for approval", channel.Title, len(actions)), "")
	}
}

// runPipelines (item 114): for channels with a pipeline chain,
// videos that have gone public in the last day and are not yet
// processed get their chain: first comment, playlist add, and
// (once per tick, via the mail watcher) the publish email.
func (s *Server) runPipelines(ctx context.Context, cfg *config.Settings) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return
	}
	done := map[uint]bool{}
	doneIDs := []uint{}
	s.getJSONSetting("pipeline:done", &doneIDs)
	for _, id := range doneIDs {
		done[id] = true
	}
	changed := false
	emailWanted := false
	for _, channel := range channels {
		defaults := s.loadChannelDefaults(channel.ID)
		if !defaults.PipelineComment && !defaults.PipelineEmail && defaults.PipelinePlaylistID == 0 {
			continue
		}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			continue
		}
		for _, video := range videos {
			if done[video.ID] || video.Visibility != model.VisibilityPublic || video.PublishedAt == nil {
				continue
			}
			if time.Since(*video.PublishedAt) > 24*time.Hour {
				done[video.ID] = true
				changed = true
				continue
			}
			svc, svcErr := s.serviceForChannel(ctx, channel.ID)
			if svcErr != nil {
				continue
			}
			acted := []string{}
			if defaults.PipelineComment && defaults.CommentTemplate != "" {
				if _, hasFirst := s.store.Comments.GetFirstByVideo(video.ID); hasFirst != nil {
					text := renderCommentTemplate(defaults.CommentTemplate, video, channel.Title)
					if _, postErr := comments.PostFirstComment(ctx, svc, s.store, video, text); postErr == nil {
						acted = append(acted, "first comment posted")
						s.addQuota(50)
					}
				}
			}
			if defaults.PipelinePlaylistID != 0 {
				playlistAction := proposalAction{Type: "playlist", VideoID: video.ID, PlaylistID: defaults.PipelinePlaylistID}
				if _, execErr := s.executeActions(ctx, []proposalAction{playlistAction}); execErr == nil {
					acted = append(acted, "added to playlist")
				}
			}
			if defaults.PipelineEmail {
				emailWanted = true
			}
			done[video.ID] = true
			changed = true
			if len(acted) > 0 {
				s.recordActivity("automation", fmt.Sprintf("Pipeline for %q: %s", video.Title, strings.Join(acted, ", ")), "")
				s.fireWebhook(cfg, "video.published", map[string]interface{}{"videoId": video.ID, "title": video.Title, "actions": acted})
			}
		}
	}
	if changed {
		ids := make([]uint, 0, len(done))
		for id := range done {
			ids = append(ids, id)
		}
		if len(ids) > 2000 {
			ids = ids[:2000]
		}
		_ = s.setJSONSetting("pipeline:done", ids)
	}
	if emailWanted && cfg.Mail.Username != "" {
		if _, err := mailer.PublishNotifications(ctx, s.store, *cfg, cfg.Mail.Username); err == nil {
			s.recordActivity("automation", "Pipeline: publish notification email sent", "")
		}
	}
}

// fireWebhook delivers an event now, or holds it in memory while
// quiet hours run (item 117 + 120). Held events flush on the
// first tick after the window.
func (s *Server) fireWebhook(cfg *config.Settings, event string, data map[string]interface{}) {
	if cfg.WebhookURL == "" {
		return
	}
	if cfg.InQuietHours(time.Now()) {
		s.mu.Lock()
		s.heldWebhooks = append(s.heldWebhooks, webhook.Event{Event: event, At: time.Now().Format(time.RFC3339), Data: data})
		s.mu.Unlock()
		return
	}
	webhook.Fire(cfg, event, data)
}

// flushHeldWebhooks delivers events held during quiet hours.
func (s *Server) flushHeldWebhooks(cfg *config.Settings) {
	s.mu.Lock()
	held := s.heldWebhooks
	s.heldWebhooks = nil
	s.mu.Unlock()
	for _, event := range held {
		_ = webhook.Deliver(cfg, event)
	}
}

// autoSyncStatusDTO is the status block for auto-sync (item 111).
type autoSyncStatusDTO struct {
	Paused  bool   `json:"paused"`
	NextRun string `json:"nextRunAt"`
	Enabled bool   `json:"enabled"`
}

// autoSyncStatus computes pause state and the earliest next due
// time across accounts.
func (s *Server) autoSyncStatus() autoSyncStatusDTO {
	status := autoSyncStatusDTO{}
	if paused, found, _ := s.store.Settings.Get(autoSyncPausedKey); found && paused == "true" {
		status.Paused = true
	}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		return status
	}
	now := time.Now()
	var earliest *time.Time
	for _, account := range accounts {
		settings := s.accountSettings(account.ID)
		if settings.SyncIntervalMin <= 0 {
			continue
		}
		status.Enabled = true
		state, stateErr := s.store.SyncStates.Get(account.ID, "videos")
		next := now
		if stateErr == nil && state != nil && state.LastRunAt != nil {
			next = state.LastRunAt.Add(time.Duration(settings.SyncIntervalMin) * time.Minute)
		}
		if earliest == nil || next.Before(*earliest) {
			candidate := next
			earliest = &candidate
		}
	}
	if earliest != nil {
		status.NextRun = earliest.Format(time.RFC3339)
	}
	return status
}

// handleAutoSyncPause serves POST /api/autosync/pause {paused}.
func (s *Server) handleAutoSyncPause(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paused bool `json:"paused"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	value := "false"
	if body.Paused {
		value = "true"
	}
	if err := s.store.Settings.Set(autoSyncPausedKey, value); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s.autoSyncStatus())
}
