package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/comments"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
)

// Rules engine and approval queue (spec 29, items 112, 113,
// 116). Rules and proposals persist as JSON in the settings
// table, like recipes and saved searches. A rule only ever acts
// on videos it has not processed before: "when a NEW video
// matches X" is the contract, so a rule can never re-fire on the
// same video twice.

const rulesKey = "ui.rules"
const proposalsKey = "ui.proposals"

// ruleMatch selects the videos a rule applies to.
type ruleMatch struct {
	ChannelID  uint   `json:"channelId"`
	Query      string `json:"query"`
	Visibility string `json:"visibility"`
}

// ruleAction is what a rule does to a matched video.
type ruleAction struct {
	Type       string `json:"type"` // rename | comment | playlist
	Template   string `json:"template"`
	Text       string `json:"text"`
	PlaylistID uint   `json:"playlistId"` // local playlist id
}

// rule is one automation rule.
type rule struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Enabled bool       `json:"enabled"`
	Mode    string     `json:"mode"` // apply | propose
	Match   ruleMatch  `json:"match"`
	Action  ruleAction `json:"action"`
}

func (s *Server) loadRules() []rule {
	rules := []rule{}
	s.getJSONSetting(rulesKey, &rules)
	return rules
}

// proposalAction is one concrete, executable step inside a
// proposal.
type proposalAction struct {
	Type       string `json:"type"`
	VideoID    uint   `json:"videoId"`
	VideoTitle string `json:"videoTitle"`
	Detail     string `json:"detail"`
	NewTitle   string `json:"newTitle,omitempty"`
	Text       string `json:"text,omitempty"`
	PlaylistID uint   `json:"playlistId,omitempty"`
	PublishAt  string `json:"publishAt,omitempty"`
}

// proposal is one queued batch of automated actions awaiting a
// decision (item 116).
type proposal struct {
	ID        string           `json:"id"`
	RuleID    string           `json:"ruleId"`
	RuleName  string           `json:"ruleName"`
	CreatedAt string           `json:"createdAt"`
	Status    string           `json:"status"` // pending | approved | rejected
	Actions   []proposalAction `json:"actions"`
	Results   []string         `json:"results,omitempty"`
}

func (s *Server) loadProposals() []proposal {
	proposals := []proposal{}
	s.getJSONSetting(proposalsKey, &proposals)
	return proposals
}

func (s *Server) saveProposals(proposals []proposal) error {
	return s.setJSONSetting(proposalsKey, proposals)
}

// ruleSeenKey is the settings key of one rule's processed set.
func ruleSeenKey(ruleID string) string {
	return "rule:" + ruleID + ":seen"
}

func (s *Server) loadRuleSeen(ruleID string) map[uint]bool {
	ids := []uint{}
	s.getJSONSetting(ruleSeenKey(ruleID), &ids)
	seen := map[uint]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	return seen
}

func (s *Server) saveRuleSeen(ruleID string, seen map[uint]bool) {
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	_ = s.setJSONSetting(ruleSeenKey(ruleID), ids)
}

// matchVideos returns the mirror videos a rule's match selects.
func (s *Server) matchVideos(match ruleMatch) ([]model.Video, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	matched := []model.Video{}
	query := strings.ToLower(match.Query)
	for _, channel := range channels {
		if match.ChannelID != 0 && channel.ID != match.ChannelID {
			continue
		}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		for _, video := range videos {
			if match.Visibility != "" && video.Visibility.String() != match.Visibility {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(video.Title), query) {
				continue
			}
			matched = append(matched, video)
		}
	}
	return matched, nil
}

// planRuleActions expands a rule over videos into concrete
// proposal actions, without writing anything.
func (s *Server) planRuleActions(theRule rule, videos []model.Video) ([]proposalAction, error) {
	actions := []proposalAction{}
	switch theRule.Action.Type {
	case "rename":
		plan, err := batch.BuildPlan(videos, batch.Pattern{Template: theRule.Action.Template})
		if err != nil {
			return nil, err
		}
		for _, change := range plan.Changes {
			if change.NewTitle == change.OldTitle {
				continue
			}
			actions = append(actions, proposalAction{
				Type: "rename", VideoID: change.VideoID, VideoTitle: change.OldTitle,
				Detail: fmt.Sprintf("Rename to %q", change.NewTitle), NewTitle: change.NewTitle,
			})
		}
	case "comment":
		for _, video := range videos {
			if _, err := s.store.Comments.GetFirstByVideo(video.ID); err == nil {
				continue // already has a first comment
			}
			channelTitle := ""
			if channel, findErr := s.findChannel(video.ChannelID); findErr == nil {
				channelTitle = channel.Title
			}
			text := renderCommentTemplate(theRule.Action.Text, video, channelTitle)
			actions = append(actions, proposalAction{
				Type: "comment", VideoID: video.ID, VideoTitle: video.Title,
				Detail: fmt.Sprintf("Post first comment: %s", truncate(text, 80)), Text: text,
			})
		}
	case "playlist":
		playlist, err := s.store.Playlists.GetByID(theRule.Action.PlaylistID)
		if err != nil {
			return nil, fmt.Errorf("playlist %d: %w", theRule.Action.PlaylistID, err)
		}
		for _, video := range videos {
			actions = append(actions, proposalAction{
				Type: "playlist", VideoID: video.ID, VideoTitle: video.Title,
				Detail: fmt.Sprintf("Add to playlist %q", playlist.Title), PlaylistID: playlist.ID,
			})
		}
	default:
		return nil, fmt.Errorf("unknown rule action type %q", theRule.Action.Type)
	}
	return actions, nil
}

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "…"
}

// executeActions performs proposal actions through the YouTube
// API and returns one result line per action. It stops at the
// first error, reporting how far it got.
func (s *Server) executeActions(ctx context.Context, actions []proposalAction) ([]string, error) {
	results := []string{}
	for _, action := range actions {
		video, err := s.store.Videos.GetByID(action.VideoID)
		if err != nil {
			return results, err
		}
		svc, err := s.serviceForChannel(ctx, video.ChannelID)
		if err != nil {
			return results, err
		}
		switch action.Type {
		case "rename":
			plan := &batch.Plan{Changes: []batch.Change{{
				VideoID: video.ID, YTVideoID: video.YTVideoID, ChannelID: video.ChannelID,
				OldTitle: video.Title, NewTitle: action.NewTitle,
				OldDescription: video.Description, NewDescription: video.Description,
			}}}
			if _, err := batch.Apply(ctx, svc, s.store, plan); err != nil {
				return results, err
			}
			s.addQuota(50)
			results = append(results, fmt.Sprintf("renamed %q", video.Title))
		case "comment":
			if _, err := comments.PostFirstComment(ctx, svc, s.store, *video, action.Text); err != nil {
				return results, err
			}
			s.addQuota(50)
			results = append(results, fmt.Sprintf("commented on %q", video.Title))
		case "playlist":
			playlist, err := s.store.Playlists.GetByID(action.PlaylistID)
			if err != nil {
				return results, err
			}
			_, err = svc.PlaylistItems.Insert([]string{"snippet"}, &youtube.PlaylistItem{
				Snippet: &youtube.PlaylistItemSnippet{
					PlaylistId: playlist.YTPlaylistID,
					ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: video.YTVideoID},
				},
			}).Context(ctx).Do()
			if err != nil {
				return results, err
			}
			s.addQuota(50)
			results = append(results, fmt.Sprintf("added %q to %q", video.Title, playlist.Title))
		case "schedule":
			publishAt, err := time.Parse(time.RFC3339, action.PublishAt)
			if err != nil {
				return results, err
			}
			if err := scheduler.RestoreSchedule(ctx, svc, s.store, video.ID, video.YTVideoID, &publishAt, model.VisibilityScheduled); err != nil {
				return results, err
			}
			s.addQuota(50)
			results = append(results, fmt.Sprintf("scheduled %q for %s", video.Title, action.PublishAt))
		default:
			return results, fmt.Errorf("unknown action type %q", action.Type)
		}
	}
	return results, nil
}

// evaluateRule runs one rule over its new matches (post-sync and
// post-autosync trigger). In apply mode the actions execute now;
// in propose mode they queue as one proposal (item 116).
func (s *Server) evaluateRule(ctx context.Context, theRule rule) {
	if !theRule.Enabled {
		return
	}
	matched, err := s.matchVideos(theRule.Match)
	if err != nil {
		return
	}
	seen := s.loadRuleSeen(theRule.ID)
	fresh := []model.Video{}
	for _, video := range matched {
		if !seen[video.ID] {
			fresh = append(fresh, video)
		}
	}
	if len(fresh) == 0 {
		return
	}
	actions, err := s.planRuleActions(theRule, fresh)
	if err != nil {
		s.recordActivity("rule", fmt.Sprintf("Rule %q could not plan: %v", theRule.Name, err), "")
		return
	}
	for _, video := range fresh {
		seen[video.ID] = true
	}
	s.saveRuleSeen(theRule.ID, seen)
	if len(actions) == 0 {
		return
	}
	if theRule.Mode == "propose" {
		proposals := s.loadProposals()
		proposals = append(proposals, proposal{
			ID: fmt.Sprintf("p%d", time.Now().UnixNano()), RuleID: theRule.ID, RuleName: theRule.Name,
			CreatedAt: time.Now().Format(time.RFC3339), Status: "pending", Actions: actions,
		})
		_ = s.saveProposals(proposals)
		s.recordActivity("rule", fmt.Sprintf("Rule %q proposed %d action(s)", theRule.Name, len(actions)), "")
		return
	}
	results, err := s.executeActions(ctx, actions)
	if err != nil {
		s.recordActivity("rule", fmt.Sprintf("Rule %q applied %d of %d action(s), then failed: %v", theRule.Name, len(results), len(actions), err), "")
		return
	}
	s.recordActivity("rule", fmt.Sprintf("Rule %q applied %d action(s)", theRule.Name, len(results)), strings.Join(results, "; "))
}

// evaluateAllRules runs every enabled rule (the automation
// trigger used after syncs).
func (s *Server) evaluateAllRules(ctx context.Context) {
	for _, theRule := range s.loadRules() {
		s.evaluateRule(ctx, theRule)
	}
}

// handleListRules serves GET /api/rules.
func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.loadRules())
}

// handleSaveRule serves POST /api/rules (create or update by id).
func (s *Server) handleSaveRule(w http.ResponseWriter, r *http.Request) {
	var body rule
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("a rule needs a name"))
		return
	}
	if body.Mode != "apply" && body.Mode != "propose" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("mode must be apply or propose"))
		return
	}
	switch body.Action.Type {
	case "rename", "comment", "playlist":
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("action type must be rename, comment or playlist"))
		return
	}
	rules := s.loadRules()
	if body.ID == "" {
		body.ID = fmt.Sprintf("r%d", time.Now().UnixNano())
		rules = append(rules, body)
	} else {
		replaced := false
		for i := range rules {
			if rules[i].ID == body.ID {
				rules[i] = body
				replaced = true
			}
		}
		if !replaced {
			rules = append(rules, body)
		}
	}
	if err := s.setJSONSetting(rulesKey, rules); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, body)
}

// handleDeleteRule serves DELETE /api/rules/{id}.
func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rules := s.loadRules()
	kept := rules[:0]
	found := false
	for _, existing := range rules {
		if existing.ID == id {
			found = true
			continue
		}
		kept = append(kept, existing)
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("rule %s not found", id))
		return
	}
	if err := s.setJSONSetting(rulesKey, kept); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]bool{"deleted": true})
}

// dryRunRequest is the body of POST /api/rules/dry-run.
type dryRunRequest struct {
	Rule rule `json:"rule"`
}

// handleRuleDryRun serves POST /api/rules/dry-run (item 112):
// the rule is expanded over the current mirror, marking which
// matched videos the rule has already processed.
func (s *Server) handleRuleDryRun(w http.ResponseWriter, r *http.Request) {
	var body dryRunRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	matched, err := s.matchVideos(body.Rule.Match)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	seen := s.loadRuleSeen(body.Rule.ID)
	fresh := []model.Video{}
	alreadySeen := 0
	for _, video := range matched {
		if seen[video.ID] {
			alreadySeen++
			continue
		}
		fresh = append(fresh, video)
	}
	actions, err := s.planRuleActions(body.Rule, fresh)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]interface{}{
		"matched": len(matched), "alreadyProcessed": alreadySeen, "actions": actions,
	})
}

// handleListProposals serves GET /api/proposals.
func (s *Server) handleListProposals(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.loadProposals())
}

// decideProposal applies a status change to one proposal,
// executing its actions first when approving.
func (s *Server) decideProposal(r *http.Request, id string, approve bool) error {
	proposals := s.loadProposals()
	for i := range proposals {
		if proposals[i].ID != id {
			continue
		}
		if proposals[i].Status != "pending" {
			return fmt.Errorf("proposal %s is already %s", id, proposals[i].Status)
		}
		if approve {
			results, err := s.executeActions(r.Context(), proposals[i].Actions)
			proposals[i].Results = results
			if err != nil {
				_ = s.saveProposals(proposals)
				return err
			}
			proposals[i].Status = "approved"
			s.recordActivity("rule", fmt.Sprintf("Proposal from %q approved: %d action(s)", proposals[i].RuleName, len(results)), strings.Join(results, "; "))
		} else {
			proposals[i].Status = "rejected"
			s.recordActivity("rule", fmt.Sprintf("Proposal from %q rejected", proposals[i].RuleName), "")
		}
		return s.saveProposals(proposals)
	}
	return fmt.Errorf("proposal %s not found", id)
}

// handleApproveProposal serves POST /api/proposals/{id}/approve.
func (s *Server) handleApproveProposal(w http.ResponseWriter, r *http.Request) {
	if err := s.decideProposal(r, r.PathValue("id"), true); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, s.loadProposals())
}

// handleRejectProposal serves POST /api/proposals/{id}/reject.
func (s *Server) handleRejectProposal(w http.ResponseWriter, r *http.Request) {
	if err := s.decideProposal(r, r.PathValue("id"), false); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, s.loadProposals())
}

// handleApproveAllProposals serves POST /api/proposals/approve-all.
func (s *Server) handleApproveAllProposals(w http.ResponseWriter, r *http.Request) {
	proposals := s.loadProposals()
	approved := 0
	for _, item := range proposals {
		if item.Status != "pending" {
			continue
		}
		if err := s.decideProposal(r, item.ID, true); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("approve %s: %w", item.ID, err))
			return
		}
		approved++
	}
	writeOK(w, map[string]interface{}{"approved": approved, "proposals": s.loadProposals()})
}
