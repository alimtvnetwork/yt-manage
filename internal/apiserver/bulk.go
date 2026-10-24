package apiserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
)

// patternRequest is the pattern part of a bulk-plan request.
type patternRequest struct {
	Template            string `json:"template"`
	Regex               string `json:"regex"`
	Replacement         string `json:"replacement"`
	DescriptionTemplate string `json:"descriptionTemplate"`
	StartIndex          int    `json:"startIndex"`
}

// bulkPlanRequest is the body of POST /api/videos/bulk-plan.
type bulkPlanRequest struct {
	ChannelID  uint           `json:"channelId"`
	Visibility string         `json:"visibility"`
	Q          string         `json:"q"`
	Pattern    patternRequest `json:"pattern"`
}

// changeDTO is one planned change in a bulk-plan response.
type changeDTO struct {
	VideoID              uint     `json:"videoId"`
	YTVideoID            string   `json:"ytVideoId"`
	ChannelID            uint     `json:"channelId"`
	OldTitle             string   `json:"oldTitle"`
	NewTitle             string   `json:"newTitle"`
	OldDescription       string   `json:"oldDescription"`
	NewDescription       string   `json:"newDescription"`
	HasDescriptionChange bool     `json:"hasDescriptionChange"`
	Warnings             []string `json:"warnings"`
}

// bulkPlanResponse is the data shape of POST /api/videos/bulk-plan.
type bulkPlanResponse struct {
	PlanID       string      `json:"planId"`
	ConfirmToken string      `json:"confirmToken"`
	Summary      string      `json:"summary"`
	Changes      []changeDTO `json:"changes"`
	Warnings     []string    `json:"warnings"`
}

// handleBulkPlan serves POST /api/videos/bulk-plan: it builds the
// change-set and stores it server-side under a plan id and a random
// confirmation token, expiring after ten minutes.
func (s *Server) handleBulkPlan(w http.ResponseWriter, r *http.Request) {
	var body bulkPlanRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	videos, err := searchx.Search(s.store, searchx.Query{
		Text: body.Q, ChannelID: body.ChannelID, Visibility: body.Visibility, Limit: 500,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Template: body.Pattern.Template, Regex: body.Pattern.Regex,
		Replacement: body.Pattern.Replacement, DescriptionTemplate: body.Pattern.DescriptionTemplate,
		StartIndex: body.Pattern.StartIndex,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	planID, err := randomHex(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	confirmToken, err := randomHex(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.mu.Lock()
	s.cleanupPlansLocked()
	s.plans[planID] = storedPlan{plan: plan, confirmToken: confirmToken, expiresAt: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	titleCounts := make(map[string]int)
	for _, change := range plan.Changes {
		titleCounts[change.NewTitle]++
	}
	changes := make([]changeDTO, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		warnings := []string{}
		if titleCounts[change.NewTitle] > 1 {
			warnings = append(warnings, "Another video in this plan gets the same new title")
		}
		if len([]rune(change.NewTitle)) > 90 {
			warnings = append(warnings, "New title is close to YouTube's 100-character limit")
		}
		if change.NewTitle == change.OldTitle && !change.HasDescriptionChange {
			warnings = append(warnings, "Title and description are unchanged")
		}
		changes = append(changes, changeDTO{
			VideoID: change.VideoID, YTVideoID: change.YTVideoID, ChannelID: change.ChannelID,
			OldTitle: change.OldTitle, NewTitle: change.NewTitle,
			OldDescription: change.OldDescription, NewDescription: change.NewDescription,
			HasDescriptionChange: change.HasDescriptionChange, Warnings: warnings,
		})
	}
	warnings := plan.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	writeOK(w, bulkPlanResponse{
		PlanID: planID, ConfirmToken: confirmToken, Summary: plan.Summary(),
		Changes: changes, Warnings: warnings,
	})
}

// cleanupPlansLocked drops expired plans. The caller holds s.mu.
func (s *Server) cleanupPlansLocked() {
	now := time.Now()
	for id, stored := range s.plans {
		if now.After(stored.expiresAt) {
			delete(s.plans, id)
		}
	}
}

// bulkEditRequest is the body of POST /api/videos/bulk-edit.
// OnlyIDs optionally restricts the apply to a subset of the plan's
// changes (staged apply, spec 29 item 35); the rest of the plan is
// kept server-side for a follow-up call.
type bulkEditRequest struct {
	PlanID       string `json:"planId"`
	ConfirmToken string `json:"confirmToken"`
	OnlyIDs      []uint `json:"onlyIds"`
}

// bulkResultDTO is the per-item outcome of a bulk apply.
type bulkResultDTO struct {
	VideoID   uint   `json:"videoId"`
	YTVideoID string `json:"ytVideoId"`
	IsOK      bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

// bulkEditResponse is the data shape of POST /api/videos/bulk-edit.
type bulkEditResponse struct {
	Applied   int             `json:"applied"`
	Total     int             `json:"total"`
	Results   []bulkResultDTO `json:"results"`
	Remaining int             `json:"remaining"`
	PlanKept  bool            `json:"planKept"`
}

// handleBulkEdit serves POST /api/videos/bulk-edit: it applies a
// stored plan's changes (or the OnlyIDs subset) grouped by owning
// account, reports per-item results, and keeps whatever did not
// apply - unattempted or failed changes - in the stored plan so
// the UI can retry them (spec 29, items 35-37).
func (s *Server) handleBulkEdit(w http.ResponseWriter, r *http.Request) {
	var body bulkEditRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	s.cleanupPlansLocked()
	stored, hasPlan := s.plans[body.PlanID]
	s.mu.Unlock()
	if !hasPlan {
		writeError(w, http.StatusNotFound, fmt.Errorf("plan not found or expired: %s", body.PlanID))
		return
	}
	if stored.confirmToken != body.ConfirmToken {
		writeError(w, http.StatusForbidden, fmt.Errorf("confirmation token does not match plan %s", body.PlanID))
		return
	}
	selected, remaining := splitPlanChanges(stored.plan, body.OnlyIDs)
	selectedIDs := make([]uint, 0, len(selected))
	for _, change := range selected {
		selectedIDs = append(selectedIDs, change.VideoID)
	}
	s.snapshotBeforeApply(selectedIDs, fmt.Sprintf("bulk apply of %d videos", len(selected)))
	results, err := s.applyChangesGrouped(r, selected)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	applied := 0
	failedIDs := make(map[uint]bool)
	for _, result := range results {
		if result.IsOK {
			applied++
		} else {
			failedIDs[result.VideoID] = true
		}
	}
	for _, change := range selected {
		if failedIDs[change.VideoID] {
			remaining = append(remaining, change)
		}
	}
	s.mu.Lock()
	if len(remaining) > 0 {
		s.plans[body.PlanID] = storedPlan{
			plan: &batch.Plan{Changes: remaining}, confirmToken: stored.confirmToken,
			expiresAt: time.Now().Add(10 * time.Minute),
		}
	} else {
		delete(s.plans, body.PlanID)
	}
	s.mu.Unlock()
	if applied > 0 {
		s.recordActivity("rename", fmt.Sprintf("Renamed %d of %d videos", applied, len(stored.plan.Changes)), "")
		s.addQuota(50 * applied)
	}
	writeOK(w, bulkEditResponse{
		Applied: applied, Total: len(stored.plan.Changes), Results: results,
		Remaining: len(remaining), PlanKept: len(remaining) > 0,
	})
}

// splitPlanChanges splits a plan into the changes selected by
// onlyIDs (all of them when onlyIDs is empty) and the rest.
func splitPlanChanges(plan *batch.Plan, onlyIDs []uint) ([]batch.Change, []batch.Change) {
	if len(onlyIDs) == 0 {
		return plan.Changes, []batch.Change{}
	}
	wanted := make(map[uint]bool, len(onlyIDs))
	for _, id := range onlyIDs {
		wanted[id] = true
	}
	selected := []batch.Change{}
	remaining := []batch.Change{}
	for _, change := range plan.Changes {
		if wanted[change.VideoID] {
			selected = append(selected, change)
		} else {
			remaining = append(remaining, change)
		}
	}
	return selected, remaining
}

// applyChangesGrouped groups changes by owning account and applies
// each group through that account's YouTube service, collecting
// per-item results and continuing past individual failures.
func (s *Server) applyChangesGrouped(r *http.Request, changes []batch.Change) ([]bulkResultDTO, error) {
	mapping, err := s.channelAccountMap()
	if err != nil {
		return nil, err
	}
	groups := make(map[uint]*batch.Plan)
	var accountOrder []uint
	for _, change := range changes {
		accountID, hasAccount := mapping[change.ChannelID]
		if !hasAccount {
			return nil, fmt.Errorf("no account found for channel %d", change.ChannelID)
		}
		group, hasGroup := groups[accountID]
		if !hasGroup {
			group = &batch.Plan{}
			groups[accountID] = group
			accountOrder = append(accountOrder, accountID)
		}
		group.Changes = append(group.Changes, change)
	}
	results := []bulkResultDTO{}
	for _, accountID := range accountOrder {
		svc, err := s.serviceForAccountID(r, accountID)
		if err != nil {
			for _, change := range groups[accountID].Changes {
				results = append(results, bulkResultDTO{
					VideoID: change.VideoID, YTVideoID: change.YTVideoID, Error: err.Error(),
				})
			}
			continue
		}
		for _, result := range batch.ApplyEach(r.Context(), svc, s.store, groups[accountID]) {
			results = append(results, bulkResultDTO{
				VideoID: result.VideoID, YTVideoID: result.YTVideoID,
				IsOK: result.IsOK, Error: result.Error,
			})
		}
	}
	return results, nil
}
