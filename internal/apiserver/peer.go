package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/alimtvnetwork/yt-manage/internal/peer"
)

// maxPeerBodyBytes is the largest accepted peer request body.
const maxPeerBodyBytes = 32 << 20

// authenticatePeer reads the request body and verifies its HMAC
// signature against the named peer's shared secret. On failure it
// writes the error response and returns hasPeer false.
func (s *Server) authenticatePeer(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPeerBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read peer body: %w", err))
		return nil, false
	}
	peerName := r.Header.Get("X-YTM-Peer")
	secret, hasPeer, err := peer.FindPeerSecret(s.store, peerName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return nil, false
	}
	if !hasPeer {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("unknown peer %q", peerName))
		return nil, false
	}
	if !peer.Verify(secret, body, r.Header.Get("X-YTM-Signature")) {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid signature for peer %q", peerName))
		return nil, false
	}
	return body, true
}

// handlePeerPush serves POST /api/peer/push: a signed payload from
// another yt-manage installation is applied to the local mirror.
func (s *Server) handlePeerPush(w http.ResponseWriter, r *http.Request) {
	body, isAuthenticated := s.authenticatePeer(w, r)
	if !isAuthenticated {
		return
	}
	var payload peer.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode peer payload: %w", err))
		return
	}
	result, err := peer.ApplyPayload(s.store, &payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, result)
}

// handlePeerPull serves POST /api/peer/pull: a signed request (the
// body may be "{}" or empty) receives the local mirror payload.
// The signature is verified against whatever bytes arrived.
func (s *Server) handlePeerPull(w http.ResponseWriter, r *http.Request) {
	_, isAuthenticated := s.authenticatePeer(w, r)
	if !isAuthenticated {
		return
	}
	payload, err := peer.BuildPayload(s.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, payload)
}
