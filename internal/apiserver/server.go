// Package apiserver exposes the local REST API and the embedded
// single-page UI. It uses only the standard library HTTP server and
// reaches data exclusively through the repository store.
package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/webhook"
)

// Server is the yt-manage API server.
type Server struct {
	store         *repo.Store
	home          string
	key           []byte
	mux           *http.ServeMux
	mu            sync.Mutex
	plans         map[string]storedPlan
	forceReadOnly bool
	heldWebhooks  []webhook.Event
}

// SetForceReadOnly pins the server into read-only mode regardless
// of the read_only setting (used by `serve --read-only`).
func (s *Server) SetForceReadOnly(force bool) {
	s.forceReadOnly = force
}

// ReadOnly reports whether writes are currently blocked: forced by
// the serve flag or enabled via the read_only setting (item 95).
func (s *Server) ReadOnly() bool {
	if s.forceReadOnly {
		return true
	}
	cfg, err := config.LoadSettings(s.home)
	return err == nil && cfg.ReadOnly
}

// readOnlyMiddleware rejects every non-GET /api call with a plain
// message while read-only mode is on. The UI assets stay readable
// so the app can explain itself.
func (s *Server) readOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead &&
			strings.HasPrefix(r.URL.Path, "/api/") && s.ReadOnly() {
			writeError(w, http.StatusForbidden, fmt.Errorf("read-only mode is on: writes are blocked. Turn it off with: yt-manage settings set read_only false"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// storedPlan is a bulk-edit plan held between the plan and apply
// calls, together with its single-use confirmation token.
type storedPlan struct {
	plan         *batch.Plan
	confirmToken string
	expiresAt    time.Time
}

// New builds a Server over store. The key is the already-loaded
// encryption key used to build per-account YouTube services.
func New(store *repo.Store, home string, key []byte) *Server {
	server := &Server{
		store: store,
		home:  home,
		key:   key,
		mux:   http.NewServeMux(),
		plans: make(map[string]storedPlan),
	}
	server.registerRoutes()
	return server
}

// ListenAndServe serves the API and UI on addr until it fails.
func (s *Server) ListenAndServe(addr string) error {
	s.startAutoLoop()
	return http.ListenAndServe(addr, s.readOnlyMiddleware(s.mux))
}

// registerRoutes wires every route using Go 1.22 method patterns.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/videos", s.handleListVideos)
	s.mux.HandleFunc("GET /api/videos/{id}", s.handleGetVideo)
	s.mux.HandleFunc("PATCH /api/videos/{id}", s.handlePatchVideo)
	s.mux.HandleFunc("POST /api/videos/bulk-plan", s.handleBulkPlan)
	s.mux.HandleFunc("POST /api/videos/bulk-edit", s.handleBulkEdit)
	s.mux.HandleFunc("GET /api/search", s.handleListVideos)
	s.mux.HandleFunc("GET /api/episodes", s.handleListEpisodes)
	s.mux.HandleFunc("POST /api/episodes/{id}/schedule", s.handleScheduleEpisode)
	s.mux.HandleFunc("GET /api/playlists", s.handleListPlaylists)
	s.mux.HandleFunc("POST /api/playlists", s.handleCreatePlaylist)
	s.mux.HandleFunc("PATCH /api/playlists/{id}", s.handlePatchPlaylist)
	s.mux.HandleFunc("GET /api/ab-tests", s.handleListABTests)
	s.mux.HandleFunc("POST /api/ab-tests", s.handleCreateABTest)
	s.mux.HandleFunc("POST /api/ab-tests/{id}/snapshot", s.handleSnapshotABTest)
	s.mux.HandleFunc("POST /api/ab-tests/{id}/pick-winner", s.handlePickWinnerABTest)
	s.mux.HandleFunc("GET /api/accounts", s.handleListAccounts)
	s.mux.HandleFunc("POST /api/sync", s.handleSync)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/saved-searches", s.handleListSavedSearches)
	s.mux.HandleFunc("POST /api/saved-searches", s.handleCreateSavedSearch)
	s.mux.HandleFunc("DELETE /api/saved-searches/{id}", s.handleDeleteSavedSearch)
	s.mux.HandleFunc("GET /api/schedules", s.handleListSchedules)
	s.mux.HandleFunc("GET /api/comments", s.handleListComments)
	s.mux.HandleFunc("POST /api/demo/seed", s.handleDemoSeed)
	s.mux.HandleFunc("POST /api/demo/clear", s.handleDemoClear)
	s.mux.HandleFunc("GET /api/search/suggestions", s.handleSearchSuggestions)
	s.mux.HandleFunc("GET /api/search/all", s.handleSearchAll)
	s.mux.HandleFunc("GET /api/rename-recipes", s.handleListRecipes)
	s.mux.HandleFunc("POST /api/rename-recipes", s.handleCreateRecipe)
	s.mux.HandleFunc("DELETE /api/rename-recipes/{id}", s.handleDeleteRecipe)
	s.mux.HandleFunc("GET /api/videos/{id}/history", s.handleVideoHistory)
	s.mux.HandleFunc("POST /api/episodes/{id}/resequence", s.handleEpisodeResequence)
	s.mux.HandleFunc("POST /api/episodes/{id}/split", s.handleEpisodeSplit)
	s.mux.HandleFunc("POST /api/episodes/merge", s.handleEpisodeMerge)
	s.mux.HandleFunc("POST /api/episodes/{id}/schedule-plan", s.handleSchedulePlan)
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handleSetSetting)
	s.mux.HandleFunc("POST /api/settings/reset", s.handleResetSetting)
	s.mux.HandleFunc("POST /api/settings/export", s.handleExportSettings)
	s.mux.HandleFunc("POST /api/settings/import", s.handleImportSettings)
	s.mux.HandleFunc("GET /api/activity", s.handleListActivity)
	s.mux.HandleFunc("POST /api/activity/{id}/undo", s.handleUndoActivity)
	s.mux.HandleFunc("POST /api/videos/{id}/undo-title", s.handleUndoTitle)
	s.mux.HandleFunc("GET /api/version", s.handleVersion)
	s.mux.HandleFunc("POST /api/preflight", s.handlePreflight)
	s.mux.HandleFunc("GET /api/activity/export", s.handleActivityExport)
	s.mux.HandleFunc("GET /api/backups", s.handleListBackups)
	s.mux.HandleFunc("POST /api/backups/restore", s.handleRestoreBackup)
	s.mux.HandleFunc("GET /api/suggestions", s.handleListSuggestions)
	s.mux.HandleFunc("POST /api/suggestions/state", s.handleSuggestionState)
	s.mux.HandleFunc("GET /api/series", s.handleListSeries)
	s.mux.HandleFunc("GET /api/channels/{id}/defaults", s.handleGetChannelDefaults)
	s.mux.HandleFunc("PUT /api/channels/{id}/defaults", s.handlePutChannelDefaults)
	s.mux.HandleFunc("GET /api/rules", s.handleListRules)
	s.mux.HandleFunc("POST /api/rules", s.handleSaveRule)
	s.mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)
	s.mux.HandleFunc("POST /api/rules/dry-run", s.handleRuleDryRun)
	s.mux.HandleFunc("GET /api/proposals", s.handleListProposals)
	s.mux.HandleFunc("POST /api/proposals/{id}/approve", s.handleApproveProposal)
	s.mux.HandleFunc("POST /api/proposals/{id}/reject", s.handleRejectProposal)
	s.mux.HandleFunc("POST /api/proposals/approve-all", s.handleApproveAllProposals)
	s.mux.HandleFunc("POST /api/autosync/pause", s.handleAutoSyncPause)
	s.mux.HandleFunc("GET /api/workflows/export", s.handleExportWorkflow)
	s.mux.HandleFunc("POST /api/workflows/import", s.handleImportWorkflow)
	s.mux.HandleFunc("GET /api/comment-centre", s.handleCommentCentre)
	s.mux.HandleFunc("POST /api/videos/{id}/first-comment", s.handlePostFirstComment)
	s.mux.HandleFunc("POST /api/comments/{id}/pin", s.handlePinComment)
	s.mux.HandleFunc("PATCH /api/comments/{id}", s.handleEditComment)
	s.mux.HandleFunc("GET /api/comments/{id}/history", s.handleCommentHistory)
	s.mux.HandleFunc("POST /api/comments/{id}/restore", s.handleRestoreComment)
	s.mux.HandleFunc("POST /api/comments/preview", s.handleCommentPreview)
	s.mux.HandleFunc("POST /api/comments/check-links", s.handleCheckCommentLinks)
	s.mux.HandleFunc("GET /api/comment-coverage", s.handleCommentCoverage)
	s.mux.HandleFunc("GET /api/videos/{id}/comments", s.handleListVideoComments)
	s.mux.HandleFunc("POST /api/comments/bulk-preview", s.handleBulkCommentPreview)
	s.mux.HandleFunc("POST /api/comments/bulk-post", s.handleBulkCommentPost)
	s.mux.HandleFunc("POST /api/playlists/{id}/items", s.handleAddPlaylistItem)
	s.mux.HandleFunc("DELETE /api/playlists/{id}/items/{videoId}", s.handleRemovePlaylistItem)
	s.mux.HandleFunc("POST /api/series/playlist", s.handleSeriesPlaylist)
	s.mux.HandleFunc("GET /api/playlists/{id}/gaps", s.handlePlaylistGaps)
	s.mux.HandleFunc("GET /api/playlists/duplicates", s.handlePlaylistDuplicates)
	s.mux.HandleFunc("POST /api/playlists/{id}/reorder-preview", s.handlePlaylistReorderPreview)
	s.mux.HandleFunc("POST /api/playlists/{id}/reorder", s.handlePlaylistReorder)
	s.mux.HandleFunc("POST /api/playlists/{id}/privacy", s.handlePlaylistPrivacy)
	s.mux.HandleFunc("GET /api/playlists/{id}/checklist", s.handlePlaylistChecklist)
	s.mux.HandleFunc("GET /api/orphans", s.handleOrphans)
	s.mux.HandleFunc("GET /api/insights/dashboard", s.handleInsightsDashboard)
	s.mux.HandleFunc("GET /api/insights/scorecards", s.handleInsightsScorecards)
	s.mux.HandleFunc("GET /api/insights/performers", s.handleInsightsPerformers)
	s.mux.HandleFunc("GET /api/insights/heatmap", s.handleInsightsHeatmap)
	s.mux.HandleFunc("GET /api/insights/growth", s.handleInsightsGrowth)
	s.mux.HandleFunc("GET /api/insights/export", s.handleInsightsExport)
	s.mux.HandleFunc("GET /api/goals", s.handleListGoals)
	s.mux.HandleFunc("POST /api/goals", s.handleSaveGoal)
	s.mux.HandleFunc("DELETE /api/goals/{id}", s.handleDeleteGoal)
	s.mux.HandleFunc("GET /api/ab-tests/{id}/results", s.handleABTestResults)
	s.mux.HandleFunc("POST /api/ab-tests/{id}/schedule", s.handleSetABSchedule)
	s.mux.HandleFunc("GET /api/tags", s.handleListTags)
	s.mux.HandleFunc("POST /api/tags/rename", s.handleRenameTag)
	s.mux.HandleFunc("POST /api/peer/push", s.handlePeerPush)
	s.mux.HandleFunc("POST /api/peer/pull", s.handlePeerPull)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /app.css", s.handleCSS)
	s.mux.HandleFunc("GET /app.js", s.handleJS)
}

// envelope is the JSON shape of every /api response.
type envelope struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// writeOK writes a success envelope with data.
func writeOK(w http.ResponseWriter, data interface{}) {
	writeEnvelope(w, http.StatusOK, envelope{OK: true, Data: data})
}

// writeError writes a failure envelope carrying err's message.
func writeError(w http.ResponseWriter, status int, err error) {
	writeEnvelope(w, status, envelope{OK: false, Error: err.Error()})
}

// writeEnvelope writes one envelope as JSON.
func writeEnvelope(w http.ResponseWriter, status int, env envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		fmt.Fprintf(w, "{\"ok\":false,\"error\":%q}\n", err.Error())
	}
}
