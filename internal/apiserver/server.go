// Package apiserver exposes the local REST API and the embedded
// single-page UI. It uses only the standard library HTTP server and
// reaches data exclusively through the repository store.
package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// Server is the yt-manage API server.
type Server struct {
	store *repo.Store
	home  string
	key   []byte
	mux   *http.ServeMux
	mu    sync.Mutex
	plans map[string]storedPlan
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
	return http.ListenAndServe(addr, s.mux)
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
