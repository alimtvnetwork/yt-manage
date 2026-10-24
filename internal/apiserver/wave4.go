package apiserver

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// Wave 4 (spec 29, items 76-100) trust endpoints: pre-flight
// checks, the audit-trail export, automatic backups before big
// writes with a mirror restore, and the version endpoint.

// handleVersion serves GET /api/version (item 99: the UI footer
// shows the real binary version).
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"version": version.Version})
}

// preflightRequest is the body of POST /api/preflight.
type preflightRequest struct {
	Kind      string           `json:"kind"` // "bulk" or "schedule"
	PlanID    string           `json:"planId"`
	ChannelID uint             `json:"channelId"`
	Entries   []preflightEntry `json:"entries"`
}

type preflightEntry struct {
	VideoID   uint   `json:"videoId"`
	PublishAt string `json:"publishAt"`
}

// preflightCheck is one named check result.
type preflightCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// handlePreflight serves POST /api/preflight (item 93): before a
// big apply, verify the sign-in can be built, the quota estimate
// fits today's remaining budget, and (for schedules) no two
// entries collide with each other or with already-scheduled
// videos on the channel.
func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	var body preflightRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checks := []preflightCheck{}
	used, _ := s.quotaUsedToday()
	budget := 0
	if cfg, err := config.LoadSettings(s.home); err == nil {
		budget = cfg.DailyQuotaBudget
	}
	switch body.Kind {
	case "bulk":
		s.mu.Lock()
		stored, hasPlan := s.plans[body.PlanID]
		s.mu.Unlock()
		if !hasPlan {
			writeError(w, http.StatusNotFound, fmt.Errorf("plan not found or expired: %s", body.PlanID))
			return
		}
		accounts := map[uint]bool{}
		if mapping, mapErr := s.channelAccountMap(); mapErr == nil {
			for _, change := range stored.plan.Changes {
				video, err := s.store.Videos.GetByID(change.VideoID)
				if err == nil {
					if accountID, ok := mapping[video.ChannelID]; ok {
						accounts[accountID] = true
					}
				}
			}
		}
		tokenOK := true
		tokenMessage := "Sign-in could not be verified for every account"
		if len(accounts) > 0 {
			tokenOK = true
			tokenMessage = fmt.Sprintf("Sign-in ready for %d account(s)", len(accounts))
			for accountID := range accounts {
				if _, err := s.serviceForAccountID(r, accountID); err != nil {
					tokenOK = false
					tokenMessage = fmt.Sprintf("Account %d: %v", accountID, err)
					break
				}
			}
		}
		checks = append(checks, preflightCheck{Name: "Sign-in", OK: tokenOK, Message: tokenMessage})
		estimate := 50 * len(stored.plan.Changes)
		quotaOK := budget == 0 || used+estimate <= budget
		checks = append(checks, preflightCheck{
			Name: "Quota", OK: quotaOK,
			Message: fmt.Sprintf("This apply costs about %d units; %d of %d used today", estimate, used, budget),
		})
	case "schedule":
		if _, err := s.findChannel(body.ChannelID); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := s.serviceForChannel(r.Context(), body.ChannelID); err != nil {
			checks = append(checks, preflightCheck{Name: "Sign-in", OK: false, Message: err.Error()})
		} else {
			checks = append(checks, preflightCheck{Name: "Sign-in", OK: true, Message: "Sign-in ready"})
		}
		estimate := 50 * len(body.Entries)
		quotaOK := budget == 0 || used+estimate <= budget
		checks = append(checks, preflightCheck{
			Name: "Quota", OK: quotaOK,
			Message: fmt.Sprintf("This schedule costs about %d units; %d of %d used today", estimate, used, budget),
		})
		conflicts := []string{}
		seen := map[string]bool{}
		for _, entry := range body.Entries {
			if seen[entry.PublishAt] {
				conflicts = append(conflicts, "two videos share publish time "+entry.PublishAt)
			}
			seen[entry.PublishAt] = true
		}
		videos, err := s.store.Videos.ListByChannel(body.ChannelID)
		if err == nil {
			entryIDs := map[uint]bool{}
			for _, entry := range body.Entries {
				entryIDs[entry.VideoID] = true
			}
			for _, video := range videos {
				if video.PublishAt == nil || entryIDs[video.ID] {
					continue
				}
				if seen[video.PublishAt.Format(time.RFC3339)] {
					conflicts = append(conflicts, fmt.Sprintf("clashes with already-scheduled %q", video.Title))
				}
			}
		}
		checks = append(checks, preflightCheck{
			Name: "Conflicts", OK: len(conflicts) == 0,
			Message: strings.Join(conflicts, "; "),
		})
		if len(conflicts) == 0 {
			checks[len(checks)-1].Message = "No publish-time clashes"
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown preflight kind %q", body.Kind))
		return
	}
	allOK := true
	for _, check := range checks {
		if !check.OK {
			allOK = false
		}
	}
	writeOK(w, map[string]interface{}{"ok": allOK, "checks": checks})
}

// handleActivityExport serves GET /api/activity/export as a CSV
// download (item 94). It intentionally bypasses the JSON envelope:
// the response IS the audit file.
func (s *Server) handleActivityExport(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.Activity.List(10000, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=yt-manage-audit.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "created_at", "kind", "summary", "detail", "undone"})
	for _, entry := range entries {
		_ = writer.Write([]string{
			strconv.FormatUint(uint64(entry.ID), 10),
			entry.CreatedAt.Format(time.RFC3339),
			entry.Kind, entry.Summary, entry.Detail,
			strconv.FormatBool(entry.IsUndone),
		})
	}
	writer.Flush()
}

// backupThreshold is the apply size above which a snapshot is
// taken first (item 98).
const backupThreshold = 50

// backupVideo is one video's pre-apply state in a snapshot.
type backupVideo struct {
	VideoID     uint   `json:"videoId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	PublishAt   string `json:"publishAt"`
}

// snapshotBeforeApply stores the current state of the given videos
// under a backup:<unix> settings key and logs it. Best effort: a
// failed snapshot never blocks the apply, but the activity log
// says so when it happens.
func (s *Server) snapshotBeforeApply(videoIDs []uint, reason string) {
	if len(videoIDs) < backupThreshold {
		return
	}
	snapshot := make([]backupVideo, 0, len(videoIDs))
	for _, id := range videoIDs {
		video, err := s.store.Videos.GetByID(id)
		if err != nil {
			continue
		}
		snapshot = append(snapshot, backupVideo{
			VideoID: video.ID, Title: video.Title, Description: video.Description,
			Visibility: video.Visibility.String(), PublishAt: formatTimePtr(video.PublishAt),
		})
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	key := fmt.Sprintf("backup:%d", time.Now().Unix())
	if err := s.store.Settings.Set(key, string(data)); err != nil {
		s.recordActivity("backup", fmt.Sprintf("Backup before %s FAILED to save", reason), err.Error())
		return
	}
	s.recordActivity("backup", fmt.Sprintf("Backup taken before %s (%d videos)", reason, len(snapshot)), key)
}

// backupSummary is one entry of GET /api/backups.
type backupSummary struct {
	Key        string `json:"key"`
	VideoCount int    `json:"videoCount"`
}

// handleListBackups serves GET /api/backups.
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	all, err := s.store.Settings.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	summaries := []backupSummary{}
	for _, setting := range all {
		if !strings.HasPrefix(setting.Key, "backup:") {
			continue
		}
		var snapshot []backupVideo
		count := 0
		if json.Unmarshal([]byte(setting.Value), &snapshot) == nil {
			count = len(snapshot)
		}
		summaries = append(summaries, backupSummary{Key: setting.Key, VideoCount: count})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Key > summaries[j].Key })
	writeOK(w, summaries)
}

// restoreBackupRequest is the body of POST /api/backups/restore.
type restoreBackupRequest struct {
	Key string `json:"key"`
}

// handleRestoreBackup serves POST /api/backups/restore: the local
// mirror returns to the snapshot. This restores yt-manage's own
// records; titles already changed on YouTube itself are reverted
// per video with "Undo last rename" (item 62). The response says
// exactly that.
func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	var body restoreBackupRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	raw, found, err := s.store.Settings.Get(body.Key)
	if err != nil || !found || !strings.HasPrefix(body.Key, "backup:") {
		writeError(w, http.StatusNotFound, fmt.Errorf("backup %q not found", body.Key))
		return
	}
	var snapshot []backupVideo
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	restored := 0
	for _, entry := range snapshot {
		if err := s.store.Videos.UpdateTitleDescription(entry.VideoID, entry.Title, entry.Description); err != nil {
			continue
		}
		visibility, visErr := model.ParseVisibility(entry.Visibility)
		if visErr == nil {
			var publishAt *time.Time
			if entry.PublishAt != "" {
				if parsed, parseErr := time.Parse(time.RFC3339, entry.PublishAt); parseErr == nil {
					publishAt = &parsed
				}
			}
			_ = s.store.Videos.UpdateSchedule(entry.VideoID, publishAt, visibility)
		}
		restored++
	}
	s.recordActivity("backup", fmt.Sprintf("Restored local mirror from %s (%d videos)", body.Key, restored), body.Key)
	writeOK(w, map[string]interface{}{
		"restored": restored,
		"note":     "Local records restored. Titles already changed on YouTube are reverted per video with Undo last rename.",
	})
}
