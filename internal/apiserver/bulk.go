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
	VideoID              uint   `json:"videoId"`
	YTVideoID            string `json:"ytVideoId"`
	ChannelID            uint   `json:"channelId"`
	OldTitle             string `json:"oldTitle"`
	NewTitle             string `json:"newTitle"`
	OldDescription       string `json:"oldDescription"`
	NewDescription       string `json:"newDescription"`
	HasDescriptionChange bool   `json:"hasDescriptionChange"`
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
	changes := make([]changeDTO, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		changes = append(changes, changeDTO{
			VideoID: change.VideoID, YTVideoID: change.YTVideoID, ChannelID: change.ChannelID,
			OldTitle: change.OldTitle, NewTitle: change.NewTitle,
			OldDescription: change.OldDescription, NewDescription: change.NewDescription,
			HasDescriptionChange: change.HasDescriptionChange,
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
type bulkEditRequest struct {
	PlanID       string `json:"planId"`
	ConfirmToken string `json:"confirmToken"`
}

// bulkEditResponse is the data shape of POST /api/videos/bulk-edit.
type bulkEditResponse struct {
	Applied int `json:"applied"`
	Total   int `json:"total"`
}

// handleBulkEdit serves POST /api/videos/bulk-edit (spec 02 §5's
// bulk-edit endpoint): it consumes a stored plan exactly once and
// applies its changes grouped by owning account.
func (s *Server) handleBulkEdit(w http.ResponseWriter, r *http.Request) {
	var body bulkEditRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	s.cleanupPlansLocked()
	stored, hasPlan := s.plans[body.PlanID]
	if hasPlan && stored.confirmToken == body.ConfirmToken {
		delete(s.plans, body.PlanID)
	}
	s.mu.Unlock()
	if !hasPlan {
		writeError(w, http.StatusNotFound, fmt.Errorf("plan not found or expired: %s", body.PlanID))
		return
	}
	if stored.confirmToken != body.ConfirmToken {
		writeError(w, http.StatusForbidden, fmt.Errorf("confirmation token does not match plan %s", body.PlanID))
		return
	}
	applied, err := s.applyPlanGrouped(r, stored.plan)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, bulkEditResponse{Applied: applied, Total: len(stored.plan.Changes)})
}

// applyPlanGrouped groups a plan's changes by owning account and
// applies each group through that account's YouTube service.
func (s *Server) applyPlanGrouped(r *http.Request, plan *batch.Plan) (int, error) {
	mapping, err := s.channelAccountMap()
	if err != nil {
		return 0, err
	}
	groups := make(map[uint]*batch.Plan)
	var accountOrder []uint
	for _, change := range plan.Changes {
		accountID, hasAccount := mapping[change.ChannelID]
		if !hasAccount {
			return 0, fmt.Errorf("no account found for channel %d", change.ChannelID)
		}
		group, hasGroup := groups[accountID]
		if !hasGroup {
			group = &batch.Plan{}
			groups[accountID] = group
			accountOrder = append(accountOrder, accountID)
		}
		group.Changes = append(group.Changes, change)
	}
	applied := 0
	for _, accountID := range accountOrder {
		svc, err := s.serviceForAccountID(r, accountID)
		if err != nil {
			return applied, err
		}
		count, err := batch.Apply(r.Context(), svc, s.store, groups[accountID])
		applied += count
		if err != nil {
			return applied, err
		}
	}
	return applied, nil
}
