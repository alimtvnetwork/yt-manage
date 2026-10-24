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
	s.recordActivityAs("peer", fmt.Sprintf("Peer %s pushed: %d videos, %d playlists applied", r.Header.Get("X-YTM-Peer"), result.VideosApplied, result.PlaylistsApplied), "", "peer "+r.Header.Get("X-YTM-Peer"))
	writeOK(w, result)
}

// handlePeerPull serves POST /api/peer/pull: a signed request (the
// body may be "{}" or empty) receives the local mirror payload.
// The signature is verified against whatever bytes arrived.
//
// Selective sharing (spec 29 item 172): once a sharing selection
// has been saved for the requesting peer, the payload is
// limited to those channels. A peer with no saved selection
// keeps the pre-Wave-7 behaviour (the full mirror), so existing
// CLI peer setups are not silently cut off.
func (s *Server) handlePeerPull(w http.ResponseWriter, r *http.Request) {
	_, isAuthenticated := s.authenticatePeer(w, r)
	if !isAuthenticated {
		return
	}
	payload, err := s.peerPullPayload(r.Header.Get("X-YTM-Peer"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, payload)
}

// peerPullPayload builds the payload for one named peer,
// honouring its saved sharing selection when one exists.
func (s *Server) peerPullPayload(peerName string) (*peer.Payload, error) {
	peerRow, err := s.store.Peers.GetByName(peerName)
	if err != nil {
		return peer.BuildPayload(s.store)
	}
	key := fmt.Sprintf("peer:%d:channels", peerRow.ID)
	if _, found, getErr := s.store.Settings.Get(key); getErr != nil || !found {
		return peer.BuildPayload(s.store)
	}
	keep, err := s.peerKeepSet(peerRow.ID)
	if err != nil {
		return nil, err
	}
	return peer.BuildPayloadFor(s.store, keep)
}
