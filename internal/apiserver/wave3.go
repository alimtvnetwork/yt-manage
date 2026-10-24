package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
)

// Wave 3 (spec 29, items 51-75) endpoints: the Settings API with
// validation, per-account settings, export/import, the activity
// log, undo, and quota accounting.

// quotaKey returns today's quota settings key.
func quotaKey() string {
	return "quota:" + time.Now().Format("2006-01-02")
}

// addQuota accumulates today's estimated API usage.
func (s *Server) addQuota(units int) {
	if units <= 0 {
		return
	}
	current, _ := s.quotaUsedToday()
	_ = s.store.Settings.Set(quotaKey(), strconv.Itoa(current+units))
}

// quotaUsedToday reads today's accumulated usage.
func (s *Server) quotaUsedToday() (int, error) {
	raw, found, err := s.store.Settings.Get(quotaKey())
	if err != nil || !found {
		return 0, err
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil
	}
	return parsed, nil
}

// recordActivity writes one activity entry; failures are ignored
// so logging never breaks the action it describes.
func (s *Server) recordActivity(kind string, summary string, detail string) {
	_, _ = s.store.Activity.Record(kind, summary, detail)
}

// settingDTO is one setting in the Settings API response.
type settingDTO struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Default string `json:"default"`
	IsSet   bool   `json:"isSet"`
}

// accountSettingsDTO holds the per-account overrides of one account.
type accountSettingsDTO struct {
	NotifyOnPublish bool `json:"notifyOnPublish"`
	SyncIntervalMin int  `json:"syncIntervalMin"`
}

// settingsResponse is the data shape of GET /api/settings.
type settingsResponse struct {
	Settings    []settingDTO                  `json:"settings"`
	PerAccount  map[string]accountSettingsDTO `json:"perAccount"`
	QuotaUsed   int                           `json:"quotaUsed"`
	QuotaBudget int                           `json:"quotaBudget"`
}

func accountSettingKey(accountID uint, name string) string {
	return fmt.Sprintf("acct:%d:%s", accountID, name)
}

func (s *Server) accountSettings(accountID uint) accountSettingsDTO {
	dto := accountSettingsDTO{}
	if raw, found, err := s.store.Settings.Get(accountSettingKey(accountID, "notify")); err == nil && found {
		dto.NotifyOnPublish = raw == "true"
	}
	if raw, found, err := s.store.Settings.Get(accountSettingKey(accountID, "sync_interval_min")); err == nil && found {
		dto.SyncIntervalMin, _ = strconv.Atoi(raw)
	}
	return dto
}

func (s *Server) buildSettingsResponse() (settingsResponse, error) {
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		return settingsResponse{}, err
	}
	response := settingsResponse{
		Settings:   []settingDTO{},
		PerAccount: map[string]accountSettingsDTO{},
	}
	for _, def := range config.SettingKeys() {
		value, _ := cfg.GetByKey(def.Key)
		isSet := value != def.Default
		if def.Type == "secret" {
			isSet = value == "set"
			value = ""
		}
		response.Settings = append(response.Settings, settingDTO{
			Key: def.Key, Label: def.Label, Group: def.Group, Type: def.Type,
			Value: value, Default: def.Default, IsSet: isSet,
		})
	}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		return settingsResponse{}, err
	}
	for _, account := range accounts {
		response.PerAccount[strconv.FormatUint(uint64(account.ID), 10)] = s.accountSettings(account.ID)
	}
	response.QuotaUsed, _ = s.quotaUsedToday()
	response.QuotaBudget = cfg.DailyQuotaBudget
	return response, nil
}

// handleGetSettings serves GET /api/settings.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	response, err := s.buildSettingsResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, response)
}

// setSettingRequest is the body of PUT /api/settings.
type setSettingRequest struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	AccountID *uint  `json:"accountId"`
}

// handleSetSetting serves PUT /api/settings: validate first, then
// apply instantly and log the change (items 52, 58, 59).
func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	var body setSettingRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.AccountID != nil {
		if err := s.setAccountSetting(*body.AccountID, body.Key, body.Value); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	} else {
		cfg, err := config.LoadSettings(s.home)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		oldValue, _ := cfg.GetByKey(body.Key)
		if err := cfg.SetByKey(body.Key, body.Value); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := cfg.Save(s.home); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		displayOld := oldValue
		displayNew := body.Value
		if def, found := config.FindSettingKey(body.Key); found && def.Type == "secret" {
			displayOld = "(hidden)"
			displayNew = "(hidden)"
		}
		s.recordActivity("setting", fmt.Sprintf("Setting %s changed", body.Key),
			fmt.Sprintf("%s -> %s", displayOld, displayNew))
	}
	response, err := s.buildSettingsResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, response)
}

// setAccountSetting validates and stores one per-account override.
func (s *Server) setAccountSetting(accountID uint, key string, value string) error {
	switch key {
	case "notify_on_publish":
		if value != "true" && value != "false" {
			return fmt.Errorf("notify_on_publish must be true or false")
		}
		return s.store.Settings.Set(accountSettingKey(accountID, "notify"), value)
	case "sync_interval_min":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 1440 {
			return fmt.Errorf("sync_interval_min must be a whole number between 0 and 1440")
		}
		return s.store.Settings.Set(accountSettingKey(accountID, "sync_interval_min"), value)
	}
	return fmt.Errorf("unknown per-account setting %q", key)
}

// resetSettingRequest is the body of POST /api/settings/reset.
type resetSettingRequest struct {
	Key string `json:"key"`
}

// handleResetSetting serves POST /api/settings/reset (item 55).
func (s *Server) handleResetSetting(w http.ResponseWriter, r *http.Request) {
	var body resetSettingRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	def, found := config.FindSettingKey(body.Key)
	if !found {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown setting %q", body.Key))
		return
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := cfg.SetByKey(body.Key, def.Default); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := cfg.Save(s.home); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("setting", fmt.Sprintf("Setting %s reset to default", body.Key), def.Default)
	response, err := s.buildSettingsResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, response)
}

// settingsExport is the JSON shape of POST /api/settings/export.
type settingsExport struct {
	Settings map[string]string `json:"settings"`
	Excluded []string          `json:"excluded"`
}

// handleExportSettings serves POST /api/settings/export (item 57).
func (s *Server) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	values, excluded := cfg.Snapshot()
	writeOK(w, settingsExport{Settings: values, Excluded: excluded})
}

// importSettingsRequest is the body of POST /api/settings/import.
type importSettingsRequest struct {
	Settings map[string]string `json:"settings"`
}

// handleImportSettings serves POST /api/settings/import: every
// value is validated before anything is saved (item 59).
func (s *Server) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	var body importSettingsRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	applied := []string{}
	for key, value := range body.Settings {
		if err := cfg.SetByKey(key, value); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		applied = append(applied, key)
	}
	if err := cfg.Save(s.home); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("setting", fmt.Sprintf("Imported %d settings", len(applied)), strings.Join(applied, ", "))
	response, err := s.buildSettingsResponse()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, response)
}

// activityDTO is one entry of GET /api/activity.
type activityDTO struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Detail    string `json:"detail"`
	IsUndone  bool   `json:"isUndone"`
	CanUndo   bool   `json:"canUndo"`
	CreatedAt string `json:"createdAt"`
}

// handleListActivity serves GET /api/activity (item 63).
func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if limit <= 0 {
		limit = 100
	}
	entries, err := s.store.Activity.List(limit, r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]activityDTO, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, activityDTO{
			ID: entry.ID, Kind: entry.Kind, Summary: entry.Summary, Detail: entry.Detail,
			IsUndone:  entry.IsUndone,
			CanUndo:   entry.Kind == "schedule" && !entry.IsUndone,
			CreatedAt: formatTime(entry.CreatedAt),
		})
	}
	writeOK(w, dtos)
}

// handleUndoTitle serves POST /api/videos/{id}/undo-title: the
// video's title returns to the value before its latest recorded
// rename, through the YouTube API (item 62).
func (s *Server) handleUndoTitle(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	latest, err := s.store.TitleHistory.LatestByVideo(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no recorded rename for video %d", id))
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
	target := latest.OldTitle
	plan := &batch.Plan{Changes: []batch.Change{{
		VideoID: video.ID, YTVideoID: video.YTVideoID, ChannelID: video.ChannelID,
		OldTitle: video.Title, NewTitle: target,
		OldDescription: video.Description, NewDescription: video.Description,
	}}}
	if _, err := batch.Apply(r.Context(), svc, s.store, plan); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.recordActivity("rename", fmt.Sprintf("Undid rename of video %d", id),
		fmt.Sprintf("%s -> %s", video.Title, target))
	s.addQuota(50)
	updated, err := s.store.Videos.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toVideoDTO(*updated))
}

// scheduleUndoEntry is one entry stored in a schedule activity
// entry's detail JSON at apply time.
type scheduleUndoEntry struct {
	VideoID       uint   `json:"videoId"`
	YTVideoID     string `json:"ytVideoId"`
	ChannelID     uint   `json:"channelId"`
	OldPublishAt  string `json:"oldPublishAt"`
	OldVisibility string `json:"oldVisibility"`
}

// handleUndoActivity serves POST /api/activity/{id}/undo for
// schedule entries: every video returns to the publish state it
// had before the schedule was applied (item 62).
func (s *Server) handleUndoActivity(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entry, err := s.store.Activity.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if entry.Kind != "schedule" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("only schedule entries can be undone here; renames undo per video"))
		return
	}
	if entry.IsUndone {
		writeError(w, http.StatusBadRequest, fmt.Errorf("this schedule was already undone"))
		return
	}
	var undoEntries []scheduleUndoEntry
	if err := json.Unmarshal([]byte(entry.Detail), &undoEntries); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("parse schedule detail: %w", err))
		return
	}
	for _, undo := range undoEntries {
		svc, err := s.serviceForChannel(r.Context(), undo.ChannelID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var publishAt *time.Time
		if undo.OldPublishAt != "" {
			parsed, err := time.Parse(time.RFC3339, undo.OldPublishAt)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			publishAt = &parsed
		}
		visibility, err := model.ParseVisibility(undo.OldVisibility)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := scheduler.RestoreSchedule(r.Context(), svc, s.store, undo.VideoID, undo.YTVideoID, publishAt, visibility); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.addQuota(50)
	}
	if err := s.store.Activity.MarkUndone(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("schedule", fmt.Sprintf("Undid schedule #%d", id), "")
	writeOK(w, map[string]bool{"undone": true})
}
