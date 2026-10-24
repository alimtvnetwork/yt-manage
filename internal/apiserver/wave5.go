package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/comments"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/suggest"
)

// Wave 5 (spec 29, items 101-125) endpoints: the suggestion
// inbox, per-channel smart defaults, series listing, workflow
// bundles, and the comment centre.

// getJSONSetting decodes a JSON settings value; missing keys
// leave target untouched.
func (s *Server) getJSONSetting(key string, target interface{}) {
	raw, found, err := s.store.Settings.Get(key)
	if err != nil || !found {
		return
	}
	_ = json.Unmarshal([]byte(raw), target)
}

// setJSONSetting encodes a value into a settings key.
func (s *Server) setJSONSetting(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.store.Settings.Set(key, string(data))
}

// suggestionDTO adds inbox state to a computed suggestion.
type suggestionDTO struct {
	suggest.Suggestion
	State string `json:"state"` // active | dismissed | snoozed
}

// suggestionStates maps suggestion type to its inbox state.
type suggestionStates map[string]struct {
	State       string `json:"state"`
	SnoozedTill string `json:"snoozedTill"`
}

func (s *Server) loadSuggestionStates() suggestionStates {
	states := suggestionStates{}
	s.getJSONSetting("suggest.states", &states)
	return states
}

// handleListSuggestions serves GET /api/suggestions (items
// 101-106, 109, 110): computed suggestions plus the latest
// anomaly summary, each with its inbox state; dismissed and
// still-snoozed types are included but marked, so the UI can
// offer "show hidden" and reset.
func (s *Server) handleListSuggestions(w http.ResponseWriter, r *http.Request) {
	computed, err := suggest.All(s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	states := s.loadSuggestionStates()
	now := time.Now()
	dtos := make([]suggestionDTO, 0, len(computed))
	for _, item := range computed {
		state := "active"
		if entry, ok := states[item.Type]; ok {
			if entry.State == "dismissed" {
				state = "dismissed"
			}
			if entry.State == "snoozed" {
				if till, parseErr := time.Parse(time.RFC3339, entry.SnoozedTill); parseErr == nil && till.After(now) {
					state = "snoozed"
				}
			}
		}
		dtos = append(dtos, suggestionDTO{Suggestion: item, State: state})
	}
	if entries, listErr := s.store.Activity.List(1, "anomaly"); listErr == nil && len(entries) > 0 {
		state := "active"
		if entry, ok := states["anomalies"]; ok && entry.State == "dismissed" {
			state = "dismissed"
		}
		dtos = append(dtos, suggestionDTO{
			Suggestion: suggest.Suggestion{
				Type: "anomalies", Title: entries[0].Summary, Detail: entries[0].Detail,
			},
			State: state,
		})
	}
	writeOK(w, dtos)
}

// suggestionStateRequest is the body of POST /api/suggestions/state.
type suggestionStateRequest struct {
	Type   string `json:"type"`
	Action string `json:"action"` // dismiss | snooze | reset
	Days   int    `json:"days"`
}

// handleSuggestionState serves POST /api/suggestions/state
// (item 110).
func (s *Server) handleSuggestionState(w http.ResponseWriter, r *http.Request) {
	var body suggestionStateRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	states := s.loadSuggestionStates()
	switch body.Action {
	case "dismiss":
		states[body.Type] = struct {
			State       string `json:"state"`
			SnoozedTill string `json:"snoozedTill"`
		}{State: "dismissed"}
	case "snooze":
		days := body.Days
		if days <= 0 {
			days = 7
		}
		states[body.Type] = struct {
			State       string `json:"state"`
			SnoozedTill string `json:"snoozedTill"`
		}{State: "snoozed", SnoozedTill: time.Now().AddDate(0, 0, days).Format(time.RFC3339)}
	case "reset":
		delete(states, body.Type)
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", body.Action))
		return
	}
	if err := s.setJSONSetting("suggest.states", states); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]bool{"saved": true})
}

// seriesDTO is one detected series for the wizard and watch list.
type seriesDTO struct {
	SeriesKey string   `json:"seriesKey"`
	ChannelID uint     `json:"channelId"`
	Count     int      `json:"count"`
	Grouped   bool     `json:"grouped"`
	Sample    []string `json:"sample"`
	VideoIDs  []uint   `json:"videoIds"`
}

// handleListSeries serves GET /api/series (item 107): series
// detected from title patterns across all channels.
func (s *Server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result := []seriesDTO{}
	for _, channel := range channels {
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		eps, _ := s.store.Episodes.ListByChannel(channel.ID)
		groupedShows := map[string]bool{}
		for _, episode := range eps {
			groupedShows[episodes.ShowKey(episodes.EpisodeBaseKey(episode.EpisodeKey))] = true
		}
		byKey := map[string]*seriesDTO{}
		for _, video := range videos {
			parsed := episodes.ParseTitle(video.Title)
			if parsed.SeriesKey == "" || !parsed.HasEpisode {
				continue
			}
			key := episodes.ShowKey(parsed.SeriesKey)
			entry, ok := byKey[key]
			if !ok {
				entry = &seriesDTO{SeriesKey: key, ChannelID: channel.ID, Grouped: groupedShows[key]}
				byKey[key] = entry
			}
			entry.Count++
			entry.VideoIDs = append(entry.VideoIDs, video.ID)
			if len(entry.Sample) < 3 {
				entry.Sample = append(entry.Sample, video.Title)
			}
		}
		for _, entry := range byKey {
			if entry.Count >= 2 {
				result = append(result, *entry)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	writeOK(w, result)
}

// channelDefaults holds the per-channel remembered defaults
// (item 108) and the post-publish pipeline chain (item 114) and
// the watched (ongoing) series list (item 115).
type channelDefaults struct {
	RenameTemplate     string   `json:"renameTemplate"`
	CadenceDays        int      `json:"cadenceDays"`
	CommentTemplate    string   `json:"commentTemplate"`
	PipelineComment    bool     `json:"pipelineComment"`
	PipelineEmail      bool     `json:"pipelineEmail"`
	PipelinePlaylistID uint     `json:"pipelinePlaylistId"`
	WatchSeries        []string `json:"watchSeries"`
	// Playlist templates (item 135): rendered when a playlist
	// is created from a series.
	PlaylistTitleTemplate       string `json:"playlistTitleTemplate"`
	PlaylistDescriptionTemplate string `json:"playlistDescriptionTemplate"`
	// Timezone (spec 29 item 196): the channel's IANA zone, so
	// publish times can be shown in the channel's zone and the
	// viewer's zone side by side. Empty = the viewer's zone.
	Timezone string `json:"timezone"`
}

func defaultsKey(channelID uint) string {
	return fmt.Sprintf("channel:%d:defaults", channelID)
}

func (s *Server) loadChannelDefaults(channelID uint) channelDefaults {
	defaults := channelDefaults{}
	s.getJSONSetting(defaultsKey(channelID), &defaults)
	return defaults
}

// handleGetChannelDefaults serves GET /api/channels/{id}/defaults.
func (s *Server) handleGetChannelDefaults(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, s.loadChannelDefaults(id))
}

// handlePutChannelDefaults serves PUT /api/channels/{id}/defaults.
func (s *Server) handlePutChannelDefaults(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body channelDefaults
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.CadenceDays < 0 || body.CadenceDays > 365 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cadenceDays must be between 0 and 365"))
		return
	}
	if body.Timezone != "" {
		if _, tzErr := time.LoadLocation(body.Timezone); tzErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("timezone must be an IANA name like Asia/Kuala_Lumpur: %v", tzErr))
			return
		}
	}
	if err := s.setJSONSetting(defaultsKey(id), body); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("setting", fmt.Sprintf("Channel %d defaults updated", id), "")
	writeOK(w, body)
}

// workflowBundle is the export/import shape of item 118: rules,
// rename recipes, saved searches, and per-channel defaults in
// one file. Secrets are never part of a bundle.
type workflowBundle struct {
	Rules           []rule                     `json:"rules"`
	Recipes         []renameRecipe             `json:"recipes"`
	SavedSearches   []savedSearch              `json:"savedSearches"`
	ChannelDefaults map[string]channelDefaults `json:"channelDefaults"`
}

// handleExportWorkflow serves GET /api/workflows/export.
func (s *Server) handleExportWorkflow(w http.ResponseWriter, r *http.Request) {
	bundle := workflowBundle{
		Rules:           s.loadRules(),
		Recipes:         []renameRecipe{},
		SavedSearches:   []savedSearch{},
		ChannelDefaults: map[string]channelDefaults{},
	}
	s.getJSONSetting(settingRenameRecipes, &bundle.Recipes)
	s.getJSONSetting(settingSavedSearches, &bundle.SavedSearches)
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, channel := range channels {
		defaults := s.loadChannelDefaults(channel.ID)
		if defaults.RenameTemplate != "" || defaults.CadenceDays != 0 || defaults.CommentTemplate != "" ||
			defaults.PipelineComment || defaults.PipelineEmail || defaults.PipelinePlaylistID != 0 || len(defaults.WatchSeries) > 0 ||
			defaults.PlaylistTitleTemplate != "" || defaults.PlaylistDescriptionTemplate != "" {
			bundle.ChannelDefaults[strconv.FormatUint(uint64(channel.ID), 10)] = defaults
		}
	}
	writeOK(w, bundle)
}

// handleImportWorkflow serves POST /api/workflows/import: the
// bundle replaces rules, recipes, and saved searches, and merges
// channel defaults per channel.
func (s *Server) handleImportWorkflow(w http.ResponseWriter, r *http.Request) {
	var bundle workflowBundle
	if err := decodeJSON(r, &bundle); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.setJSONSetting(rulesKey, bundle.Rules); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.setJSONSetting(settingRenameRecipes, bundle.Recipes); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.setJSONSetting(settingSavedSearches, bundle.SavedSearches); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for key, defaults := range bundle.ChannelDefaults {
		channelID, parseErr := strconv.ParseUint(key, 10, 64)
		if parseErr != nil {
			continue
		}
		_ = s.setJSONSetting(defaultsKey(uint(channelID)), defaults)
	}
	s.recordActivity("setting", fmt.Sprintf("Workflow bundle imported (%d rules, %d recipes)", len(bundle.Rules), len(bundle.Recipes)), "")
	writeOK(w, map[string]bool{"imported": true})
}

// commentCentreRow is one row of GET /api/comment-centre.
type commentCentreRow struct {
	VideoID     uint        `json:"videoId"`
	YTVideoID   string      `json:"ytVideoId"`
	Title       string      `json:"title"`
	ChannelID   uint        `json:"channelId"`
	Visibility  string      `json:"visibility"`
	PublishedAt string      `json:"publishedAt"`
	Status      string      `json:"status"` // posted | pending | draft
	Comment     *commentDTO `json:"comment,omitempty"`
	NeedsPin    bool        `json:"needsPin"`
}

// handleCommentCentre serves GET /api/comment-centre (item 121):
// every video with its first-comment state. Pending means public
// (or scheduled) with no first comment yet; needs-pin marks a
// posted comment the user has not confirmed as pinned (item 122).
func (s *Server) handleCommentCentre(w http.ResponseWriter, r *http.Request) {
	firstByVideo := map[uint]*model.Comment{}
	firsts, err := s.store.Comments.ListFirstComments(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for i := range firsts {
		firstByVideo[firsts[i].VideoID] = &firsts[i]
	}
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rows := []commentCentreRow{}
	for _, channel := range channels {
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, video := range videos {
			row := commentCentreRow{
				VideoID: video.ID, YTVideoID: video.YTVideoID, Title: video.Title,
				ChannelID: channel.ID, Visibility: video.Visibility.String(),
				PublishedAt: formatTimePtr(video.PublishedAt), Status: "draft",
			}
			if comment, ok := firstByVideo[video.ID]; ok {
				dto := toCommentDTO(*comment)
				row.Comment = &dto
				row.Status = "posted"
				row.NeedsPin = !comment.IsPinnedKnown
			} else if video.Visibility == model.VisibilityPublic || video.Visibility == model.VisibilityScheduled {
				row.Status = "pending"
			}
			rows = append(rows, row)
		}
	}
	writeOK(w, rows)
}

// postFirstCommentRequest is the body of POST /api/videos/{id}/first-comment.
type postFirstCommentRequest struct {
	Text string `json:"text"`
}

// handlePostFirstComment serves POST /api/videos/{id}/first-comment.
func (s *Server) handlePostFirstComment(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body postFirstCommentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("comment text is empty"))
		return
	}
	video, err := s.store.Videos.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	posted, err := comments.PostFirstComment(r.Context(), svc, s.store, *video, body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.addQuota(50)
	s.recordActivity("comment", fmt.Sprintf("Posted first comment on %q", video.Title), "")
	writeOK(w, toCommentDTO(*posted))
}

// pinRequest is the body of POST /api/comments/{id}/pin.
type pinRequest struct {
	Pinned bool `json:"pinned"`
}

// handlePinComment serves POST /api/comments/{id}/pin (items 122
// and 129): the user confirms the manual Studio pin here; the
// flag persists, so cleared items stay cleared.
func (s *Server) handlePinComment(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body pinRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Comments.SetPinned(id, body.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]bool{"pinned": body.Pinned})
}

// editCommentRequest is the body of PATCH /api/comments/{id}.
type editCommentRequest struct {
	Text string `json:"text"`
}

// handleEditComment serves PATCH /api/comments/{id} (item 125:
// the edit records the replaced text in the comment history).
func (s *Server) handleEditComment(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body editCommentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	comment, err := s.store.Comments.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	video, err := s.store.Videos.GetByID(comment.VideoID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := comments.EditFirstComment(r.Context(), svc, s.store, *video, body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.addQuota(50)
	writeOK(w, toCommentDTO(*updated))
}

// commentHistoryDTO is one prior version of a first comment.
type commentHistoryDTO struct {
	ID        uint   `json:"id"`
	OldText   string `json:"oldText"`
	CreatedAt string `json:"createdAt"`
}

// handleCommentHistory serves GET /api/comments/{id}/history.
func (s *Server) handleCommentHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entries, err := s.store.CommentHistory.ListByComment(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]commentHistoryDTO, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, commentHistoryDTO{ID: entry.ID, OldText: entry.OldText, CreatedAt: formatTime(entry.CreatedAt)})
	}
	writeOK(w, dtos)
}

// restoreCommentRequest is the body of POST /api/comments/{id}/restore.
type restoreCommentRequest struct {
	HistoryID uint `json:"historyId"`
}

// handleRestoreComment serves POST /api/comments/{id}/restore:
// the chosen prior version becomes the comment text again,
// through the same edit path (so the current text is itself
// recorded before being replaced).
func (s *Server) handleRestoreComment(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body restoreCommentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entry, err := s.store.CommentHistory.GetByID(body.HistoryID)
	if err != nil || entry.CommentID != id {
		writeError(w, http.StatusNotFound, fmt.Errorf("history entry not found for this comment"))
		return
	}
	comment, err := s.store.Comments.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	video, err := s.store.Videos.GetByID(comment.VideoID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := comments.EditFirstComment(r.Context(), svc, s.store, *video, entry.OldText)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, toCommentDTO(*updated))
}

// commentPreviewRequest is the body of POST /api/comments/preview.
type commentPreviewRequest struct {
	ChannelID uint   `json:"channelId"`
	Template  string `json:"template"`
}

// commentPreviewRow is one rendered template preview.
type commentPreviewRow struct {
	VideoID uint   `json:"videoId"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

// renderCommentTemplate expands {title}, {episode}, {series},
// {channel} for one video (item 123).
func renderCommentTemplate(template string, video model.Video, channelTitle string) string {
	parsed := episodes.ParseTitle(video.Title)
	episodeText := ""
	if parsed.HasEpisode {
		episodeText = fmt.Sprintf("%d", parsed.EpisodeNumber)
	}
	replacer := strings.NewReplacer(
		"{title}", video.Title,
		"{episode}", episodeText,
		"{series}", episodes.ShowKey(parsed.SeriesKey),
		"{channel}", channelTitle,
	)
	return replacer.Replace(template)
}

// toCommentDTO converts a stored comment to its wire shape.
func toCommentDTO(comment model.Comment) commentDTO {
	dto := commentDTO{
		ID: comment.ID, VideoID: comment.VideoID, Text: comment.Text,
		PinnedText:    comment.PinnedText,
		IsPinnedKnown: comment.IsPinnedKnown, PublishedAt: formatTimePtr(comment.PublishedAt),
	}
	return dto
}

// handleCommentPreview serves POST /api/comments/preview:
// the channel's comment template rendered on its newest public
// videos, before anything is posted.
func (s *Server) handleCommentPreview(w http.ResponseWriter, r *http.Request) {
	var body commentPreviewRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	channel, err := s.findChannel(body.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	template := body.Template
	if template == "" {
		template = s.loadChannelDefaults(body.ChannelID).CommentTemplate
	}
	videos, err := s.store.Videos.ListByChannel(body.ChannelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rows := []commentPreviewRow{}
	for _, video := range videos {
		if video.Visibility != model.VisibilityPublic {
			continue
		}
		rows = append(rows, commentPreviewRow{
			VideoID: video.ID, Title: video.Title,
			Text: renderCommentTemplate(template, video, channel.Title),
		})
		if len(rows) >= 5 {
			break
		}
	}
	writeOK(w, rows)
}
