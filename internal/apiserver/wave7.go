package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/insights"
	"github.com/alimtvnetwork/yt-manage/internal/mailer"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/peer"
	"github.com/alimtvnetwork/yt-manage/internal/suggest"
)

// Wave 7 (spec 29 items 151-175): multi-account mastery,
// notifications and the publish lifecycle, and peer/export
// portability. Everything here reads or writes the local
// mirror and the settings KV; the only outward effects are the
// notification emails the user configured and the signed peer
// traffic the user explicitly triggers.

// ----------------------------------------------------------------
// Overview, health, favourites, account profiles (151, 154, 157, 159)

// accountProfile is the per-account nickname/colour of item 159,
// stored in the settings KV as acct:<id>:profile.
type accountProfile struct {
	Nickname string `json:"nickname"`
	Colour   string `json:"colour"`
}

func (s *Server) loadAccountProfile(accountID uint) accountProfile {
	profile := accountProfile{}
	s.getJSONSetting(fmt.Sprintf("acct:%d:profile", accountID), &profile)
	return profile
}

// overviewAccount is one account's row of GET /api/overview.
type overviewAccount struct {
	ID           uint         `json:"id"`
	Label        string       `json:"label"`
	Nickname     string       `json:"nickname"`
	Colour       string       `json:"colour"`
	ChannelCount int          `json:"channelCount"`
	VideoCount   int64        `json:"videoCount"`
	LastSyncAt   string       `json:"lastSyncAt"`
	Attention    []string     `json:"attention"`
	Channels     []channelDTO `json:"channels"`
}

// handleOverview serves GET /api/overview (item 151): every
// account with its channels, last sync, and the reasons it
// needs attention, colour-coded by the frontend from the
// attention list (empty = healthy).
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.Accounts.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rows := []overviewAccount{}
	for _, account := range accounts {
		channels, err := s.store.Channels.ListByAccount(account.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		row := overviewAccount{
			ID: account.ID, Label: account.Label, Attention: []string{},
			Channels: []channelDTO{},
		}
		profile := s.loadAccountProfile(account.ID)
		row.Nickname, row.Colour = profile.Nickname, profile.Colour
		if len(account.TokenCiphertext) == 0 {
			row.Attention = append(row.Attention, "No stored token - sign in again")
		}
		if len(channels) == 0 {
			row.Attention = append(row.Attention, "No channels mirrored yet")
		}
		var lastSync *time.Time
		for _, channel := range channels {
			row.ChannelCount++
			row.VideoCount += channel.VideoCount
			row.Channels = append(row.Channels, toChannelDTO(channel))
			if channel.LastSyncedAt == nil {
				row.Attention = append(row.Attention, fmt.Sprintf("Channel %s has never synced", channel.Title))
			} else {
				if lastSync == nil || channel.LastSyncedAt.After(*lastSync) {
					lastSync = channel.LastSyncedAt
				}
				if time.Since(*channel.LastSyncedAt) > 48*time.Hour {
					row.Attention = append(row.Attention, fmt.Sprintf("Channel %s last synced %s ago", channel.Title, humanDuration(time.Since(*channel.LastSyncedAt))))
				}
			}
		}
		row.LastSyncAt = formatTimePtr(lastSync)
		rows = append(rows, row)
	}
	used, _ := s.quotaUsedToday()
	cfg, _ := config.LoadSettings(s.home)
	writeOK(w, map[string]interface{}{
		"accounts": rows, "quotaUsedToday": used,
		"quotaBudget": cfg.DailyQuotaBudget, "readOnly": s.ReadOnly(),
	})
}

// humanDuration renders a duration as a coarse "3d"/"5h" string.
func humanDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// healthCheck is one row of GET /api/health.
type healthCheck struct {
	Scope   string `json:"scope"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// handleHealth serves GET /api/health (item 154): the account
// health centre. Every problem names the fix the UI can offer.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	checks := []healthCheck{}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(accounts) == 0 {
		checks = append(checks, healthCheck{Scope: "accounts", OK: false,
			Message: "No accounts connected", Fix: "Connect an account from the Accounts screen"})
	}
	for _, account := range accounts {
		scope := fmt.Sprintf("Account %s", account.Label)
		if len(account.TokenCiphertext) == 0 {
			checks = append(checks, healthCheck{Scope: scope, OK: false,
				Message: "No stored token - the account cannot sync", Fix: "Sign in again from the Accounts screen"})
		} else {
			checks = append(checks, healthCheck{Scope: scope, OK: true, Message: "Token stored"})
		}
		channels, _ := s.store.Channels.ListByAccount(account.ID)
		for _, channel := range channels {
			cs := fmt.Sprintf("Channel %s", channel.Title)
			switch {
			case channel.LastSyncedAt == nil:
				checks = append(checks, healthCheck{Scope: cs, OK: false,
					Message: "Never synced", Fix: "Run a sync"})
			case time.Since(*channel.LastSyncedAt) > 48*time.Hour:
				checks = append(checks, healthCheck{Scope: cs, OK: false,
					Message: "Last synced " + humanDuration(time.Since(*channel.LastSyncedAt)) + " ago", Fix: "Run a sync"})
			default:
				checks = append(checks, healthCheck{Scope: cs, OK: true,
					Message: "Synced " + humanDuration(time.Since(*channel.LastSyncedAt)) + " ago"})
			}
		}
	}
	cfg, cfgErr := config.LoadSettings(s.home)
	if cfgErr == nil {
		used, _ := s.quotaUsedToday()
		switch {
		case cfg.DailyQuotaBudget > 0 && used >= cfg.DailyQuotaBudget:
			checks = append(checks, healthCheck{Scope: "Quota", OK: false,
				Message: fmt.Sprintf("Daily quota budget exhausted (%d of %d units)", used, cfg.DailyQuotaBudget),
				Fix:     "Raise the daily quota budget in Settings, or wait for the UTC reset"})
		case cfg.DailyQuotaBudget > 0 && used >= cfg.DailyQuotaBudget*9/10:
			checks = append(checks, healthCheck{Scope: "Quota", OK: false,
				Message: fmt.Sprintf("Quota at %d of %d units (over 90%%)", used, cfg.DailyQuotaBudget),
				Fix:     "Slow down applies, or raise the budget in Settings"})
		default:
			checks = append(checks, healthCheck{Scope: "Quota", OK: true,
				Message: fmt.Sprintf("%d units used today", used)})
		}
		prefs := s.loadNotifyPrefs()
		if (prefs.Digest != "off" || prefs.Weekly != "off" || prefs.Failures == "instant" || prefs.GoingLive == "instant") && cfg.Mail.SMTPHost == "" {
			checks = append(checks, healthCheck{Scope: "Mail", OK: false,
				Message: "Notifications are on but SMTP is not configured, so no email can be sent",
				Fix:     "Add the mail settings in Settings, or switch notifications off"})
		}
		if cfg.Mail.Username == "" && cfg.Mail.SMTPHost != "" {
			checks = append(checks, healthCheck{Scope: "Mail", OK: false,
				Message: "SMTP is set but no mail username: notification emails have no recipient",
				Fix:     "Set the mail username in Settings"})
		}
	}
	writeOK(w, checks)
}

// handleGetFavourites serves GET /api/favourites (item 157).
func (s *Server) handleGetFavourites(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{"channelIds": s.loadFavourites()})
}

// handlePutFavourites serves PUT /api/favourites {channelIds}.
func (s *Server) handlePutFavourites(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChannelIDs []uint `json:"channelIds"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.setJSONSetting("ui.favourites", body.ChannelIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]interface{}{"channelIds": body.ChannelIDs})
}

func (s *Server) loadFavourites() []uint {
	ids := []uint{}
	s.getJSONSetting("ui.favourites", &ids)
	return ids
}

// handleGetAccountProfile serves GET /api/accounts/{id}/profile.
func (s *Server) handleGetAccountProfile(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, s.loadAccountProfile(uint(id)))
}

// handlePutAccountProfile serves PUT /api/accounts/{id}/profile
// (item 159): nickname and accent colour for one account.
func (s *Server) handlePutAccountProfile(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var profile accountProfile
	if err := decodeJSON(r, &profile); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if profile.Colour != "" && !validHexColour(profile.Colour) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("colour must look like #3b82f6"))
		return
	}
	if err := s.setJSONSetting(fmt.Sprintf("acct:%d:profile", id), profile); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, profile)
}

// validHexColour reports whether value is a #rrggbb colour.
func validHexColour(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, c := range value[1:] {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------
// Compare channels (153) and the move-series plan (158)

// compareSide is one channel's half of GET /api/insights/compare.
type compareSide struct {
	ChannelID      uint    `json:"channelId"`
	Title          string  `json:"title"`
	TotalVideos    int     `json:"totalVideos"`
	TotalViews     int64   `json:"totalViews"`
	TotalComments  int64   `json:"totalComments"`
	SeriesCount    int     `json:"seriesCount"`
	CoveragePct    float64 `json:"commentCoveragePct"`
	AdherencePct   float64 `json:"scheduleAdherencePct"`
	UploadsLast90  int     `json:"uploadsLast90"`
	MedianGapDays  float64 `json:"medianGapDays"`
	ScheduledAhead int     `json:"scheduledAhead"`
}

// handleCompareChannels serves GET /api/insights/compare?a=&b=
// (item 153): two channels side by side, computed from the
// mirror by the same code as the dashboard.
func (s *Server) handleCompareChannels(w http.ResponseWriter, r *http.Request) {
	aID, errA := parseQueryInt(r, "a")
	bID, errB := parseQueryInt(r, "b")
	if errA != nil || errB != nil || aID <= 0 || bID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("a and b channel ids are required"))
		return
	}
	a, err := s.compareSide(uint(aID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	b, err := s.compareSide(uint(bID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]interface{}{"a": a, "b": b})
}

func (s *Server) compareSide(channelID uint) (*compareSide, error) {
	channel, err := s.findChannel(channelID)
	if err != nil {
		return nil, err
	}
	dash, err := insights.DashboardFor(s.store, channelID)
	if err != nil {
		return nil, err
	}
	videos, err := s.store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	side := &compareSide{
		ChannelID: channelID, Title: channel.Title,
		TotalVideos: dash.TotalVideos, TotalViews: dash.TotalViews, TotalComments: dash.TotalComments,
		CoveragePct: dash.CommentCoveragePct, AdherencePct: dash.AdherencePct,
	}
	shows := map[string]bool{}
	published := []time.Time{}
	now := time.Now()
	for _, video := range videos {
		if video.PublishedAt != nil {
			published = append(published, *video.PublishedAt)
			if now.Sub(*video.PublishedAt) <= 90*24*time.Hour {
				side.UploadsLast90++
			}
			if parsed := episodes.ParseTitle(video.Title); parsed.HasEpisode {
				shows[episodes.ShowKey(parsed.SeriesKey)] = true
			}
		}
		if video.PublishAt != nil && video.PublishAt.After(now) {
			side.ScheduledAhead++
		}
	}
	side.SeriesCount = len(shows)
	sort.Slice(published, func(i, j int) bool { return published[i].Before(published[j]) })
	if len(published) >= 2 {
		gaps := []float64{}
		for i := 1; i < len(published); i++ {
			gaps = append(gaps, published[i].Sub(published[i-1]).Hours()/24)
		}
		sort.Float64s(gaps)
		side.MedianGapDays = gaps[len(gaps)/2]
	}
	return side, nil
}

// handleSeriesMovePlan serves GET /api/series/move-plan
// (item 158): a plan only. The YouTube API cannot move a video
// between channels, so the answer is the honest checklist a
// human follows in Studio, with the series' videos listed.
func (s *Server) handleSeriesMovePlan(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	series := r.URL.Query().Get("series")
	targetID, targetErr := parseQueryInt(r, "targetChannelId")
	if err != nil || channelID <= 0 || series == "" || targetErr != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId, series and targetChannelId are required"))
		return
	}
	from, err := s.findChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	to, err := s.findChannel(uint(targetID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	videos, err := s.store.Videos.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	type moveVideo struct {
		ID        uint   `json:"id"`
		Title     string `json:"title"`
		YTVideoID string `json:"ytVideoId"`
	}
	moved := []moveVideo{}
	for _, video := range videos {
		if showOf(video) == strings.ToLower(series) {
			moved = append(moved, moveVideo{ID: video.ID, Title: video.Title, YTVideoID: video.YTVideoID})
		}
	}
	steps := []string{
		"The YouTube API cannot move videos between channels - this is a re-upload, done by hand in Studio.",
		fmt.Sprintf("Download the original files for the %d video(s) below (YouTube Studio > Content, or Google Takeout).", len(moved)),
		fmt.Sprintf("Upload each original to %s in episode/part order, reusing the mirrored titles, descriptions and tags.", to.Title),
		"Recreate the series playlist on the target channel (Episodes > Create/update playlist can rebuild it after the next sync).",
		"Re-post each first comment on the new uploads and pin it in Studio.",
		"Update descriptions, cards and end screens anywhere that link to the old video URLs.",
		"Post a community update on the old channel pointing viewers at the new home of the series.",
		fmt.Sprintf("Only after the replacements are live: unlist (do not delete) the originals on %s, so existing links keep working.", from.Title),
		"Sync both accounts so the mirror matches the new reality.",
	}
	writeOK(w, map[string]interface{}{
		"seriesKey": series, "fromChannel": from.Title, "toChannel": to.Title,
		"videos": moved, "steps": steps,
		"note": "Plan only - nothing was changed. View counts, comments and watch history do not transfer between channels.",
	})
}

// showOf returns a video's show-level key ("" when ungrouped).
func showOf(video model.Video) string {
	parsed := episodes.ParseTitle(video.Title)
	if !parsed.HasEpisode {
		return ""
	}
	return episodes.ShowKey(parsed.SeriesKey)
}

// ----------------------------------------------------------------
// Notifications: preferences, digest, weekly, alerts (161-164, 168-170)

// notifyPrefs is the notification matrix of item 163, stored in
// the settings KV as notify.prefs. Modes: digest events are
// "digest" (send on schedule) or "off"; instant events are
// "instant" or "off".
type notifyPrefs struct {
	Digest             string `json:"digest"`     // digest | off
	Failures           string `json:"failures"`   // instant | off
	GoingLive          string `json:"goingLive"`  // instant | off
	Weekly             string `json:"weekly"`     // digest | off
	DigestTime         string `json:"digestTime"` // HH:MM local
	ExcludedChannelIDs []uint `json:"excludedChannelIds"`
}

func (s *Server) loadNotifyPrefs() notifyPrefs {
	prefs := notifyPrefs{Digest: "digest", Failures: "instant", GoingLive: "instant", Weekly: "digest", DigestTime: "08:00"}
	s.getJSONSetting("notify.prefs", &prefs)
	if prefs.DigestTime == "" {
		prefs.DigestTime = "08:00"
	}
	return prefs
}

// handleGetNotifyPrefs serves GET /api/notify/prefs.
func (s *Server) handleGetNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.loadNotifyPrefs())
}

// handlePutNotifyPrefs serves PUT /api/notify/prefs (item 163),
// validated before saving like every other settings write.
func (s *Server) handlePutNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	var prefs notifyPrefs
	if err := decodeJSON(r, &prefs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for name, mode := range map[string]string{"digest": prefs.Digest, "weekly": prefs.Weekly} {
		if mode != "digest" && mode != "off" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("%s must be \"digest\" or \"off\"", name))
			return
		}
	}
	for name, mode := range map[string]string{"failures": prefs.Failures, "goingLive": prefs.GoingLive} {
		if mode != "instant" && mode != "off" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("%s must be \"instant\" or \"off\"", name))
			return
		}
	}
	if _, err := time.Parse("15:04", prefs.DigestTime); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("digestTime must look like 08:00"))
		return
	}
	if err := s.setJSONSetting("notify.prefs", prefs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("setting", "Notification preferences updated", "")
	writeOK(w, prefs)
}

// notifyRecipient is where notification email goes: the mail
// account itself, as with the publish pipeline (item 114).
func (s *Server) notifyRecipient(cfg *config.Settings) string {
	return cfg.Mail.Username
}

// notifyFooter (item 169): every automated email explains how
// to stop it, pointing at the local preferences screen.
func (s *Server) notifyFooter(cfg *config.Settings, event string) string {
	addr := cfg.ServerAddr
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	return fmt.Sprintf("\n\n--\nYou are receiving this %s email from yt-manage.\nTo stop these emails, open http://%s/ and go to Settings > Notifications, or run: yt-manage settings set notify off.\n", event, addr)
}

// sendNotify sends one notification email now, appending the
// stop-these-emails footer. It reports configuration problems
// plainly instead of failing silently.
func (s *Server) sendNotify(cfg *config.Settings, event, subject, body string) error {
	to := s.notifyRecipient(cfg)
	if cfg.Mail.SMTPHost == "" {
		return fmt.Errorf("SMTP is not configured (Settings > Mail)")
	}
	if to == "" {
		return fmt.Errorf("no notification recipient: set the mail username in Settings")
	}
	return mailer.Send(*cfg, to, subject, body+s.notifyFooter(cfg, event))
}

func (s *Server) digestExcluded(prefs notifyPrefs) map[uint]bool {
	excluded := map[uint]bool{}
	for _, id := range prefs.ExcludedChannelIDs {
		excluded[id] = true
	}
	return excluded
}

// buildDigest renders the publish-day digest (item 161): one
// email for the whole installation, one section per channel,
// listing what published on the given day with links. It
// returns the body and whether anything published at all.
func (s *Server) buildDigest(day time.Time) (string, bool, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return "", false, err
	}
	prefs := s.loadNotifyPrefs()
	excluded := s.digestExcluded(prefs)
	var sections []string
	any := false
	for _, channel := range channels {
		if excluded[channel.ID] {
			continue
		}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return "", false, err
		}
		lines := []string{}
		for _, video := range videos {
			if video.PublishedAt == nil || video.Visibility != model.VisibilityPublic {
				continue
			}
			published := *video.PublishedAt
			if published.Year() == day.Year() && published.YearDay() == day.YearDay() {
				lines = append(lines, fmt.Sprintf("- %s\n  https://www.youtube.com/watch?v=%s", video.Title, video.YTVideoID))
			}
		}
		if len(lines) > 0 {
			any = true
			sections = append(sections, fmt.Sprintf("%s (%d published)\n%s", channel.Title, len(lines), strings.Join(lines, "\n")))
		}
	}
	if !any {
		return fmt.Sprintf("Nothing published on %s.", day.Format("2006-01-02")), false, nil
	}
	return fmt.Sprintf("Published on %s:\n\n%s", day.Format("2006-01-02"), strings.Join(sections, "\n\n")), true, nil
}

// handleSendDigest serves POST /api/notify/digest (item 161):
// send today's digest now.
func (s *Server) handleSendDigest(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	body, _, err := s.buildDigest(time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.sendNotify(cfg, "publish digest", "yt-manage publish digest "+time.Now().Format("2006-01-02"), body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.recordActivity("email", "Publish digest sent", "")
	writeOK(w, map[string]bool{"sent": true})
}

// buildWeekly renders the Monday report (item 168) per channel:
// published last 7 days, scheduled ahead, comment coverage,
// top performer, and one suggestion.
func (s *Server) buildWeekly() (string, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return "", err
	}
	prefs := s.loadNotifyPrefs()
	excluded := s.digestExcluded(prefs)
	now := time.Now()
	sections := []string{}
	for _, channel := range channels {
		if excluded[channel.ID] {
			continue
		}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return "", err
		}
		published := 0
		scheduledAhead := 0
		var top *model.Video
		for i := range videos {
			video := &videos[i]
			if video.PublishedAt != nil && now.Sub(*video.PublishedAt) <= 7*24*time.Hour {
				published++
			}
			if video.PublishAt != nil && video.PublishAt.After(now) {
				scheduledAhead++
			}
			if top == nil || video.ViewCount > top.ViewCount {
				top = video
			}
		}
		coverage, _ := s.commentCoverage(channel.ID)
		lines := []string{
			fmt.Sprintf("%s", channel.Title),
			fmt.Sprintf("  Published in the last 7 days: %d", published),
			fmt.Sprintf("  Scheduled ahead: %d", scheduledAhead),
			fmt.Sprintf("  First-comment coverage: %.0f%%", coverage),
		}
		if top != nil {
			lines = append(lines, fmt.Sprintf("  Top performer: %s (%d views)", top.Title, top.ViewCount))
		}
		if suggestions, sErr := suggest.ForChannel(s.store, channel.ID); sErr == nil && len(suggestions) > 0 {
			lines = append(lines, fmt.Sprintf("  One suggestion: %s - %s", suggestions[0].Title, suggestions[0].Detail))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if len(sections) == 0 {
		return "No channels are included in the weekly report.", nil
	}
	return "Weekly channel report:\n\n" + strings.Join(sections, "\n\n"), nil
}

// commentCoverage is one channel's first-comment coverage
// percentage, shared by the weekly report (same definition as
// the coverage report of item 127).
func (s *Server) commentCoverage(channelID uint) (float64, error) {
	firsts, err := s.store.Comments.ListFirstComments(0)
	if err != nil {
		return 0, err
	}
	withFirst := map[uint]bool{}
	for _, first := range firsts {
		withFirst[first.VideoID] = true
	}
	videos, err := s.store.Videos.ListByChannel(channelID)
	if err != nil {
		return 0, err
	}
	public, covered := 0, 0
	for _, video := range videos {
		if video.Visibility != model.VisibilityPublic {
			continue
		}
		public++
		if withFirst[video.ID] {
			covered++
		}
	}
	if public == 0 {
		return 0, nil
	}
	return float64(covered) * 100 / float64(public), nil
}

// handleSendWeekly serves POST /api/notify/weekly (item 168).
func (s *Server) handleSendWeekly(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	body, err := s.buildWeekly()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.sendNotify(cfg, "weekly report", "yt-manage weekly channel report", body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.recordActivity("email", "Weekly channel report sent", "")
	writeOK(w, map[string]bool{"sent": true})
}

// handleNotifyTest serves POST /api/notify/test {kind}
// (item 170): every email kind is test-sendable to the user's
// own address from Settings, marked as a test.
func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var subject, text string
	switch body.Kind {
	case "digest":
		digest, _, dErr := s.buildDigest(time.Now())
		if dErr != nil {
			writeError(w, http.StatusInternalServerError, dErr)
			return
		}
		subject, text = "[test] yt-manage publish digest", digest
	case "weekly":
		weekly, wErr := s.buildWeekly()
		if wErr != nil {
			writeError(w, http.StatusInternalServerError, wErr)
			return
		}
		subject, text = "[test] yt-manage weekly channel report", weekly
	case "failure":
		subject, text = "[test] yt-manage alert", "This is a test failure alert. Real alerts are sent immediately when a sync or apply fails."
	case "going-live":
		subject, text = "[test] yt-manage: going live soon", "This is a test going-live reminder. Real reminders arrive about an hour before a scheduled video publishes."
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("kind must be digest, weekly, failure or going-live"))
		return
	}
	if err := s.sendNotify(cfg, body.Kind, subject, text); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]bool{"sent": true})
}

// alertFailure (item 162) emails a failure immediately when the
// failures event is set to instant, and always leaves an
// in-app trace in History. Quiet hours hold the email; held
// alerts flush as one summary when the window ends.
func (s *Server) alertFailure(summary string) {
	s.recordActivity("alert", summary, "")
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		return
	}
	prefs := s.loadNotifyPrefs()
	if prefs.Failures != "instant" {
		return
	}
	if cfg.InQuietHours(time.Now()) {
		held := []string{}
		s.getJSONSetting("notify.heldAlerts", &held)
		held = append(held, summary)
		_ = s.setJSONSetting("notify.heldAlerts", held)
		return
	}
	_ = s.sendNotify(cfg, "failure alert", "yt-manage alert: "+truncate(summary, 80), summary)
}

// runNotifyTick runs once a minute from the serve loop: the
// daily digest, the Monday weekly report, going-live
// reminders, and the post-quiet-hours flush of held alerts.
func (s *Server) runNotifyTick(cfg *config.Settings) {
	now := time.Now()
	if cfg.InQuietHours(now) {
		return
	}
	// Flush held failure alerts as one summary (item 117).
	held := []string{}
	s.getJSONSetting("notify.heldAlerts", &held)
	if len(held) > 0 {
		_ = s.setJSONSetting("notify.heldAlerts", []string{})
		_ = s.sendNotify(cfg, "failure alert", fmt.Sprintf("yt-manage: %d alert(s) held during quiet hours", len(held)),
			"The following failures happened during quiet hours:\n\n- "+strings.Join(held, "\n- "))
	}
	prefs := s.loadNotifyPrefs()
	digestAt, err := time.Parse("15:04", prefs.DigestTime)
	if err != nil {
		return
	}
	scheduledToday := time.Date(now.Year(), now.Month(), now.Day(), digestAt.Hour(), digestAt.Minute(), 0, 0, now.Location())
	if now.Before(scheduledToday) {
		return
	}
	if prefs.Digest == "digest" {
		key := "notify:digest:" + now.Format("2006-01-02")
		if _, sent, _ := s.store.Settings.Get(key); !sent {
			_ = s.store.Settings.Set(key, "1")
			if body, any, bErr := s.buildDigest(now); bErr == nil && any {
				if sendErr := s.sendNotify(cfg, "publish digest", "yt-manage publish digest "+now.Format("2006-01-02"), body); sendErr == nil {
					s.recordActivity("email", "Publish digest sent", "")
				}
			}
		}
	}
	if prefs.Weekly == "digest" && now.Weekday() == time.Monday {
		year, week := now.ISOWeek()
		key := fmt.Sprintf("notify:weekly:%d-%02d", year, week)
		if _, sent, _ := s.store.Settings.Get(key); !sent {
			_ = s.store.Settings.Set(key, "1")
			if body, bErr := s.buildWeekly(); bErr == nil {
				if sendErr := s.sendNotify(cfg, "weekly report", "yt-manage weekly channel report", body); sendErr == nil {
					s.recordActivity("email", "Weekly channel report sent", "")
				}
			}
		}
	}
	s.sendGoingLiveReminders(cfg, prefs, now)
}

// sendGoingLiveReminders (item 164): one email listing the
// videos that publish within the next hour, at most once per
// video (remembered in the settings KV).
func (s *Server) sendGoingLiveReminders(cfg *config.Settings, prefs notifyPrefs, now time.Time) {
	if prefs.GoingLive != "instant" {
		return
	}
	channels, err := s.store.Channels.List()
	if err != nil {
		return
	}
	lines := []string{}
	reminded := []uint{}
	for _, channel := range channels {
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			continue
		}
		for _, video := range videos {
			if video.PublishAt == nil {
				continue
			}
			until := video.PublishAt.Sub(now)
			if until <= 0 || until > time.Hour {
				continue
			}
			key := fmt.Sprintf("notify:reminded:%d", video.ID)
			if _, done, _ := s.store.Settings.Get(key); done {
				continue
			}
			_ = s.store.Settings.Set(key, "1")
			lines = append(lines, fmt.Sprintf("- %s (%s) publishes at %s\n  https://www.youtube.com/watch?v=%s",
				video.Title, channel.Title, video.PublishAt.Format("15:04"), video.YTVideoID))
			reminded = append(reminded, video.ID)
		}
	}
	if len(lines) == 0 {
		return
	}
	body := "Going live within the hour - time for the manual pin and final checks:\n\n" + strings.Join(lines, "\n")
	if err := s.sendNotify(cfg, "going-live reminder", "yt-manage: going live within the hour", body); err == nil {
		s.recordActivity("email", fmt.Sprintf("Going-live reminder sent for %d video(s)", len(reminded)), "")
	}
}

// ----------------------------------------------------------------
// Publish timeline and missed publishes (165, 166)

// timelineItem is one publish event of GET /api/timeline.
type timelineItem struct {
	VideoID    uint   `json:"videoId"`
	Title      string `json:"title"`
	YTVideoID  string `json:"ytVideoId"`
	ChannelID  uint   `json:"channelId"`
	At         string `json:"at"`
	Status     string `json:"status"` // published | scheduled | missed
	Visibility string `json:"visibility"`
}

// videoPublishStatus classifies one video for the timeline:
// published when it has a publish timestamp in the past,
// scheduled when its slot is ahead, missed when the slot
// passed but the video never went public.
func videoPublishStatus(video model.Video, now time.Time) (string, *time.Time) {
	if video.PublishedAt != nil {
		return "published", video.PublishedAt
	}
	if video.PublishAt != nil {
		if video.PublishAt.After(now) {
			return "scheduled", video.PublishAt
		}
		return "missed", video.PublishAt
	}
	return "", nil
}

// handleTimeline serves GET /api/timeline?channelId=&days=
// (item 165): past and future publishes on one timeline.
func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	days, daysErr := parseQueryInt(r, "days")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	if daysErr != nil || days <= 0 {
		days = 30
	}
	videos, err := s.store.Videos.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	from := now.Add(-time.Duration(days) * 24 * time.Hour)
	to := now.Add(time.Duration(days) * 24 * time.Hour)
	items := []timelineItem{}
	for _, video := range videos {
		status, at := videoPublishStatus(video, now)
		if at == nil || at.Before(from) || at.After(to) {
			continue
		}
		items = append(items, timelineItem{
			VideoID: video.ID, Title: video.Title, YTVideoID: video.YTVideoID,
			ChannelID: video.ChannelID, At: formatTimePtr(at), Status: status,
			Visibility: string(video.Visibility),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].At < items[j].At })
	writeOK(w, items)
}

// missedItem is one row of GET /api/missed.
type missedItem struct {
	VideoID   uint    `json:"videoId"`
	Title     string  `json:"title"`
	YTVideoID string  `json:"ytVideoId"`
	ChannelID uint    `json:"channelId"`
	PublishAt string  `json:"publishAt"`
	HoursLate float64 `json:"hoursLate"`
}

// missedVideos finds the missed publishes (item 166): the slot
// passed over an hour ago and the video is still not public.
// Shared by the endpoint and any banner the UI draws.
func (s *Server) missedVideos(channelID uint) ([]missedItem, error) {
	var channels []model.Channel
	var err error
	if channelID > 0 {
		channel, findErr := s.findChannel(channelID)
		if findErr != nil {
			return nil, findErr
		}
		channels = []model.Channel{*channel}
	} else {
		channels, err = s.store.Channels.List()
		if err != nil {
			return nil, err
		}
	}
	now := time.Now()
	missed := []missedItem{}
	for _, channel := range channels {
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		for _, video := range videos {
			status, at := videoPublishStatus(video, now)
			if status != "missed" || now.Sub(*at) < time.Hour {
				continue
			}
			missed = append(missed, missedItem{
				VideoID: video.ID, Title: video.Title, YTVideoID: video.YTVideoID,
				ChannelID: video.ChannelID, PublishAt: formatTimePtr(at),
				HoursLate: float64(int(now.Sub(*at).Hours()*10)) / 10,
			})
		}
	}
	sort.Slice(missed, func(i, j int) bool { return missed[i].PublishAt < missed[j].PublishAt })
	return missed, nil
}

// handleMissed serves GET /api/missed?channelId= (item 166).
// Rescheduling uses the existing schedule endpoints; this list
// is the recovery queue.
func (s *Server) handleMissed(w http.ResponseWriter, r *http.Request) {
	channelID, _ := parseQueryInt(r, "channelId")
	missed, err := s.missedVideos(uint(channelID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, missed)
}

// ----------------------------------------------------------------
// In-app inbox (167)

// handleInbox serves GET /api/inbox?limit= (item 167): the
// mail account's inbox envelopes via the existing IMAP reader,
// so publish emails are visible inside the app too.
func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit")
	if err != nil || limit <= 0 {
		limit = 30
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	messages, err := mailer.FetchInbox(*cfg, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	type inboxDTO struct {
		UID     uint32 `json:"uid"`
		From    string `json:"from"`
		Subject string `json:"subject"`
		Date    string `json:"date"`
	}
	dtos := []inboxDTO{}
	for _, message := range messages {
		dtos = append(dtos, inboxDTO{
			UID: message.UID, From: message.From, Subject: message.Subject,
			Date: message.Date.Format(time.RFC3339),
		})
	}
	writeOK(w, dtos)
}

// ----------------------------------------------------------------
// Peers: status screen, selective sharing, conflicts (171-173)

// peerDTO is one peer of GET /api/peers. The shared secret is
// never returned.
type peerDTO struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	BaseURL          string `json:"baseUrl"`
	LastSeenAt       string `json:"lastSeenAt"`
	SharedChannelIDs []uint `json:"sharedChannelIds"`
}

func (s *Server) peerSharing(peerID uint) []uint {
	ids := []uint{}
	s.getJSONSetting(fmt.Sprintf("peer:%d:channels", peerID), &ids)
	return ids
}

func (s *Server) toPeerDTO(peerRow model.Peer) peerDTO {
	return peerDTO{
		ID: peerRow.ID, Name: peerRow.Name, BaseURL: peerRow.BaseURL,
		LastSeenAt: formatTimePtr(peerRow.LastSeenAt), SharedChannelIDs: s.peerSharing(peerRow.ID),
	}
}

// handleListPeers serves GET /api/peers (item 171).
func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.store.Peers.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := []peerDTO{}
	for _, peerRow := range peers {
		dtos = append(dtos, s.toPeerDTO(peerRow))
	}
	writeOK(w, dtos)
}

// handleCreatePeer serves POST /api/peers {name, baseUrl,
// secret}: the same registration the CLI's peer add performs.
func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		BaseURL string `json:"baseUrl"`
		Secret  string `json:"secret"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" || body.BaseURL == "" || body.Secret == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name, baseUrl and secret are all required"))
		return
	}
	if !strings.HasPrefix(body.BaseURL, "http://") && !strings.HasPrefix(body.BaseURL, "https://") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("baseUrl must start with http:// or https://"))
		return
	}
	peerRow := &model.Peer{Name: body.Name, BaseURL: strings.TrimRight(body.BaseURL, "/"), SharedSecret: body.Secret}
	if err := s.store.Peers.Create(peerRow); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.recordActivity("peer", fmt.Sprintf("Peer %s added (%s)", peerRow.Name, peerRow.BaseURL), "")
	writeOK(w, s.toPeerDTO(*peerRow))
}

// handleDeletePeer serves DELETE /api/peers/{id}.
func (s *Server) handleDeletePeer(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Peers.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.store.Settings.Set(fmt.Sprintf("peer:%d:channels", id), "[]")
	s.recordActivity("peer", fmt.Sprintf("Peer %d removed", id), "")
	writeOK(w, map[string]bool{"deleted": true})
}

// handlePutPeerSharing serves PUT /api/peers/{id}/sharing
// {channelIds} (item 172): the channels this peer may receive.
// Nothing is shared until the user picks channels here.
func (s *Server) handlePutPeerSharing(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.store.Peers.GetByID(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var body struct {
		ChannelIDs []uint `json:"channelIds"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.setJSONSetting(fmt.Sprintf("peer:%d:channels", id), body.ChannelIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("peer", fmt.Sprintf("Peer %d now receives %d channel(s)", id, len(body.ChannelIDs)), "")
	writeOK(w, map[string]interface{}{"sharedChannelIds": body.ChannelIDs})
}

// peerKeepSet resolves a peer's shared channel ids to the
// YouTube-id keep set the payload builder filters by.
func (s *Server) peerKeepSet(peerID uint) (map[string]bool, error) {
	keep := map[string]bool{}
	for _, channelID := range s.peerSharing(peerID) {
		channel, err := s.findChannel(channelID)
		if err != nil {
			continue
		}
		keep[channel.YTChannelID] = true
	}
	return keep, nil
}

// handlePeerPushTo serves POST /api/peers/{id}/push: push only
// the shared channels to the peer (item 172).
func (s *Server) handlePeerPushTo(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	keep, err := s.peerKeepSet(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(keep) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("no channels are shared with this peer yet - choose them under Sharing first"))
		return
	}
	client := &peer.Client{}
	result, err := client.PushSelected(r.Context(), s.store, id, keep)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.recordActivity("peer", fmt.Sprintf("Pushed %d channel(s) to peer %d: %d videos applied there", len(keep), id, result.VideosApplied), "")
	writeOK(w, result)
}

// handlePeerPullFrom serves POST /api/peers/{id}/pull: fetch
// the peer's payload and apply it locally.
func (s *Server) handlePeerPullFrom(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := &peer.Client{}
	payload, err := client.FetchPayload(r.Context(), s.store, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.notePeerVersion(id, payload)
	result, err := peer.ApplyPayload(s.store, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("peer", fmt.Sprintf("Pulled from peer %d: %d videos, %d playlists applied", id, result.VideosApplied, result.PlaylistsApplied), "")
	writeOK(w, map[string]interface{}{
		"videosApplied": result.VideosApplied, "videosSkipped": result.VideosSkipped,
		"playlistsApplied": result.PlaylistsApplied, "itemsApplied": result.ItemsApplied,
		"versionNote": s.peerVersionWarning(id),
	})
}

// peerConflict is one differing record of POST
// /api/peers/{id}/conflicts (item 173).
type peerConflict struct {
	YTVideoID   string `json:"ytVideoId"`
	ChannelYTID string `json:"channelYtId"`
	Local       string `json:"local"`
	Remote      string `json:"remote"`
}

// describeVideo renders the compared fields of one side.
func describeVideo(title, description, visibility string) string {
	return fmt.Sprintf("Title: %s\nVisibility: %s\nDescription: %s", title, visibility, description)
}

// handlePeerConflicts serves POST /api/peers/{id}/conflicts:
// fetch the peer's payload (nothing applied) and list every
// video both sides hold with different title, description or
// visibility - the pick-a-winner queue of item 173.
func (s *Server) handlePeerConflicts(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := &peer.Client{}
	remote, err := client.FetchPayload(r.Context(), s.store, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.notePeerVersion(id, remote)
	conflicts := []peerConflict{}
	for _, dto := range remote.Videos {
		local, getErr := s.store.Videos.GetByYTID(dto.YtVideoId)
		if getErr != nil {
			continue // only the peer has it: a pull applies it, no conflict
		}
		if local.Title == dto.Title && local.Description == dto.Description && string(local.Visibility) == dto.Visibility {
			continue
		}
		conflicts = append(conflicts, peerConflict{
			YTVideoID: dto.YtVideoId, ChannelYTID: dto.ChannelYtId,
			Local:  describeVideo(local.Title, local.Description, string(local.Visibility)),
			Remote: describeVideo(dto.Title, dto.Description, dto.Visibility),
		})
	}
	writeOK(w, conflicts)
}

// handlePeerResolveConflict serves POST
// /api/peers/{id}/conflicts/resolve {ytVideoId, winner}
// (item 173): remote wins applies the peer's copy of that one
// video; local wins pushes the local copy to the peer.
func (s *Server) handlePeerResolveConflict(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		YTVideoID string `json:"ytVideoId"`
		Winner    string `json:"winner"` // local | remote
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client := &peer.Client{}
	switch body.Winner {
	case "remote":
		remote, fetchErr := client.FetchPayload(r.Context(), s.store, id)
		if fetchErr != nil {
			writeError(w, http.StatusBadRequest, fetchErr)
			return
		}
		single := &peer.Payload{Videos: []peer.VideoDTO{}, Playlists: []peer.PlaylistDTO{}}
		for _, dto := range remote.Videos {
			if dto.YtVideoId == body.YTVideoID {
				single.Videos = append(single.Videos, dto)
			}
		}
		if len(single.Videos) == 0 {
			writeError(w, http.StatusNotFound, fmt.Errorf("the peer no longer has video %s", body.YTVideoID))
			return
		}
		// ApplyPayload skips copies whose local sync time is
		// newer; a deliberate remote-wins pick must not be
		// skipped, so align the incoming sync time first.
		single.Videos[0].LastSyncedAt = time.Now().Format(time.RFC3339)
		if _, applyErr := peer.ApplyPayload(s.store, single); applyErr != nil {
			writeError(w, http.StatusInternalServerError, applyErr)
			return
		}
	case "local":
		localPayload, buildErr := peer.BuildPayload(s.store)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, buildErr)
			return
		}
		single := &peer.Payload{Videos: []peer.VideoDTO{}, Playlists: []peer.PlaylistDTO{}}
		for _, dto := range localPayload.Videos {
			if dto.YtVideoId == body.YTVideoID {
				single.Videos = append(single.Videos, dto)
			}
		}
		if len(single.Videos) == 0 {
			writeError(w, http.StatusNotFound, fmt.Errorf("video %s is not in the local mirror", body.YTVideoID))
			return
		}
		single.Videos[0].LastSyncedAt = time.Now().Format(time.RFC3339)
		if _, pushErr := client.PushPayload(r.Context(), s.store, id, single); pushErr != nil {
			writeError(w, http.StatusBadRequest, pushErr)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("winner must be \"local\" or \"remote\""))
		return
	}
	s.recordActivity("peer", fmt.Sprintf("Conflict on %s resolved in favour of the %s copy (peer %d)", body.YTVideoID, body.Winner, id), "")
	writeOK(w, map[string]bool{"resolved": true})
}

// ----------------------------------------------------------------
// Full export bundle and import with preview (174, 175)

// exportBundle is the JSON shape of GET /api/export/bundle
// (item 174): the mirror, the non-secret settings, and the UI
// state (rules, recipes, saved searches, channel defaults,
// favourites, notification preferences) in one file.
type exportBundle struct {
	App        string                 `json:"app"`
	Kind       string                 `json:"kind"`
	Version    int                    `json:"version"`
	ExportedAt string                 `json:"exportedAt"`
	Mirror     *peer.Payload          `json:"mirror"`
	Settings   map[string]string      `json:"settings"`
	Excluded   []string               `json:"excludedSettings"`
	UIState    map[string]interface{} `json:"uiState"`
}

// buildBundle assembles the export bundle from the stores.
func (s *Server) buildBundle() (*exportBundle, error) {
	payload, err := peer.BuildPayload(s.store)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		return nil, err
	}
	values, excluded := cfg.Snapshot()
	bundle := &exportBundle{
		App: "yt-manage", Kind: "export-bundle", Version: 1,
		ExportedAt: time.Now().Format(time.RFC3339),
		Mirror:     payload, Settings: values, Excluded: excluded,
		UIState: map[string]interface{}{},
	}
	for _, key := range []string{"ui.saved_searches", "ui.rename_recipes", "ui.rules", "ui.favourites", "notify.prefs"} {
		var value interface{}
		s.getJSONSetting(key, &value)
		if value != nil {
			bundle.UIState[key] = value
		}
	}
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	defaults := map[string]channelDefaults{}
	for _, channel := range channels {
		stored := s.loadChannelDefaults(channel.ID)
		if stored.RenameTemplate != "" || stored.CadenceDays != 0 || stored.CommentTemplate != "" ||
			stored.PipelineComment || stored.PipelineEmail || stored.PipelinePlaylistID != 0 || len(stored.WatchSeries) > 0 ||
			stored.PlaylistTitleTemplate != "" || stored.PlaylistDescriptionTemplate != "" {
			defaults[fmt.Sprintf("%d", channel.ID)] = stored
		}
	}
	if len(defaults) > 0 {
		bundle.UIState["channelDefaults"] = defaults
	}
	return bundle, nil
}

// handleExportBundle serves GET /api/export/bundle (item 174)
// as a file download.
func (s *Server) handleExportBundle(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.buildBundle()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("export", fmt.Sprintf("Export bundle downloaded (%d videos, %d playlists)", len(bundle.Mirror.Videos), len(bundle.Mirror.Playlists)), "")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=yt-manage-bundle.json")
	_, _ = w.Write(data)
}

// importPreview is the response of POST /api/import/preview
// (item 175): what an apply would add, update and skip per
// section, computed without writing anything.
type importPreview struct {
	VideosAdd    int      `json:"videosAdd"`
	VideosUpdate int      `json:"videosUpdate"`
	VideosSkip   int      `json:"videosSkip"`
	Playlists    int      `json:"playlists"`
	SettingsKeys int      `json:"settingsKeys"`
	UISections   []string `json:"uiSections"`
	ExportedAt   string   `json:"exportedAt"`
	MachineNote  string   `json:"machineNote"`
}

// decodeBundle validates the envelope of an import request.
func decodeBundle(r *http.Request) (*exportBundle, error) {
	var bundle exportBundle
	if err := decodeJSON(r, &bundle); err != nil {
		return nil, err
	}
	if bundle.Kind != "export-bundle" || bundle.Mirror == nil {
		return nil, fmt.Errorf("this file is not a yt-manage export bundle")
	}
	return &bundle, nil
}

// previewBundle computes the import preview for a bundle.
func (s *Server) previewBundle(bundle *exportBundle) (*importPreview, error) {
	preview := &importPreview{ExportedAt: bundle.ExportedAt, UISections: []string{}}
	for _, dto := range bundle.Mirror.Videos {
		local, err := s.store.Videos.GetByYTID(dto.YtVideoId)
		if err != nil {
			preview.VideosAdd++
			continue
		}
		incoming, _ := time.Parse(time.RFC3339, dto.LastSyncedAt)
		if local.LastSyncedAt == nil || local.LastSyncedAt.Before(incoming) {
			preview.VideosUpdate++
		} else {
			preview.VideosSkip++
		}
	}
	preview.Playlists = len(bundle.Mirror.Playlists)
	preview.SettingsKeys = len(bundle.Settings)
	for key := range bundle.UIState {
		preview.UISections = append(preview.UISections, key)
	}
	sort.Strings(preview.UISections)
	preview.MachineNote = "Channel defaults in the bundle are keyed by the exporting machine's channel ids; they apply only where the same channel id exists here."
	return preview, nil
}

// handleImportPreview serves POST /api/import/preview.
func (s *Server) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	bundle, err := decodeBundle(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	preview, err := s.previewBundle(bundle)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, preview)
}

// handleImportApply serves POST /api/import/apply (item 175):
// apply a bundle whose preview the user has seen. Settings
// values are validated through the same SetByKey path as the
// settings import; the mirror applies through the peer apply
// (newer-wins), so re-importing is safe.
func (s *Server) handleImportApply(w http.ResponseWriter, r *http.Request) {
	bundle, err := decodeBundle(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for key, value := range bundle.Settings {
		if err := cfg.SetByKey(key, value); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("setting %s: %w", key, err))
			return
		}
	}
	if err := cfg.Save(s.home); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := peer.ApplyPayload(s.store, bundle.Mirror)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	appliedSections := []string{}
	for key, value := range bundle.UIState {
		if key == "channelDefaults" {
			continue // applied below, keyed by channel id
		}
		if err := s.setJSONSetting(key, value); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		appliedSections = append(appliedSections, key)
	}
	if raw, ok := bundle.UIState["channelDefaults"]; ok {
		if byID, isMap := raw.(map[string]interface{}); isMap {
			for idText, value := range byID {
				channelID, parseErr := strconv.ParseUint(idText, 10, 64)
				if parseErr != nil {
					continue
				}
				if _, findErr := s.findChannel(uint(channelID)); findErr != nil {
					continue // a channel this machine does not have
				}
				data, marshalErr := json.Marshal(value)
				if marshalErr != nil {
					continue
				}
				var defaults channelDefaults
				if unmarshalErr := json.Unmarshal(data, &defaults); unmarshalErr != nil {
					continue
				}
				_ = s.setJSONSetting(defaultsKey(uint(channelID)), defaults)
			}
			appliedSections = append(appliedSections, "channelDefaults")
		}
	}
	s.recordActivity("import", fmt.Sprintf("Imported bundle from %s: %d videos applied, %d skipped, %d settings, sections %s",
		bundle.ExportedAt, result.VideosApplied, result.VideosSkipped, len(bundle.Settings), strings.Join(appliedSections, ", ")), "")
	writeOK(w, map[string]interface{}{
		"videosApplied": result.VideosApplied, "videosSkipped": result.VideosSkipped,
		"playlistsApplied": result.PlaylistsApplied, "settingsApplied": len(bundle.Settings),
		"uiSections": appliedSections,
	})
}
