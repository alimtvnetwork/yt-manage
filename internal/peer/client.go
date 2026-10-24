package peer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// Client is the HTTP client side of peer sync. A nil HTTP field
// uses a default client with a 60 second timeout.
type Client struct {
	HTTP *http.Client
}

// httpClient returns the configured HTTP client, or the default.
func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// envelope is the JSON envelope every API endpoint answers with.
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

// PushTo builds the local payload and posts it, HMAC-signed, to
// the peer's /api/peer/push endpoint. The peer applies it and its
// apply Result is returned.
func PushTo(ctx context.Context, store *repo.Store, peerID uint) (Result, error) {
	return (&Client{}).PushTo(ctx, store, peerID)
}

// PushTo is PushTo on this client.
func (c *Client) PushTo(ctx context.Context, store *repo.Store, peerID uint) (Result, error) {
	payload, err := BuildPayload(store)
	if err != nil {
		return Result{}, err
	}
	return c.PushPayload(ctx, store, peerID, payload)
}

// PushSelected pushes only the given channels' data (by YouTube
// channel id) to the peer: selective sharing, spec 29 item 172.
func (c *Client) PushSelected(ctx context.Context, store *repo.Store, peerID uint, keep map[string]bool) (Result, error) {
	payload, err := BuildPayloadFor(store, keep)
	if err != nil {
		return Result{}, err
	}
	return c.PushPayload(ctx, store, peerID, payload)
}

// PushPayload sends one already-built payload to the peer's
// /api/peer/push endpoint and returns its apply Result.
func (c *Client) PushPayload(ctx context.Context, store *repo.Store, peerID uint, payload *Payload) (Result, error) {
	peerRow, err := store.Peers.GetByID(peerID)
	if err != nil {
		return Result{}, fmt.Errorf("load peer %d: %w", peerID, err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("encode payload: %w", err)
	}
	data, err := c.post(ctx, peerRow, "/api/peer/push", body)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if len(data) > 0 {
		if err := json.Unmarshal(data, &result); err != nil {
			return Result{}, fmt.Errorf("decode push result from peer %s: %w", peerRow.Name, err)
		}
	}
	if err := touchPeer(store, peerRow); err != nil {
		return Result{}, err
	}
	return result, nil
}

// PullFrom asks the peer for its payload via /api/peer/pull,
// applies it locally, and returns the local apply Result.
func PullFrom(ctx context.Context, store *repo.Store, peerID uint) (Result, error) {
	return (&Client{}).PullFrom(ctx, store, peerID)
}

// PullFrom is PullFrom on this client.
func (c *Client) PullFrom(ctx context.Context, store *repo.Store, peerID uint) (Result, error) {
	peerRow, err := store.Peers.GetByID(peerID)
	if err != nil {
		return Result{}, fmt.Errorf("load peer %d: %w", peerID, err)
	}
	data, err := c.post(ctx, peerRow, "/api/peer/pull", []byte("{}"))
	if err != nil {
		return Result{}, err
	}
	var payload Payload
	if len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return Result{}, fmt.Errorf("decode payload from peer %s: %w", peerRow.Name, err)
		}
	}
	result, err := ApplyPayload(store, &payload)
	if err != nil {
		return Result{}, err
	}
	if err := touchPeer(store, peerRow); err != nil {
		return Result{}, err
	}
	return result, nil
}

// FetchPayload asks the peer for its payload without applying
// anything locally: the read-only half of conflict detection
// (spec 29 item 173).
func (c *Client) FetchPayload(ctx context.Context, store *repo.Store, peerID uint) (*Payload, error) {
	peerRow, err := store.Peers.GetByID(peerID)
	if err != nil {
		return nil, fmt.Errorf("load peer %d: %w", peerID, err)
	}
	data, err := c.post(ctx, peerRow, "/api/peer/pull", []byte("{}"))
	if err != nil {
		return nil, err
	}
	payload := &Payload{Videos: []VideoDTO{}, Playlists: []PlaylistDTO{}}
	if len(data) > 0 {
		if err := json.Unmarshal(data, payload); err != nil {
			return nil, fmt.Errorf("decode payload from peer %s: %w", peerRow.Name, err)
		}
	}
	if err := touchPeer(store, peerRow); err != nil {
		return nil, err
	}
	return payload, nil
}

// post sends one signed request to the peer and returns the data
// field of its envelope. The sender identifies itself with
// X-YTM-Peer set to the shared pair name (see the package comment)
// and signs the exact body bytes with the shared secret.
func (c *Client) post(ctx context.Context, peerRow *model.Peer, path string, body []byte) (json.RawMessage, error) {
	url := strings.TrimRight(peerRow.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request to peer %s: %w", peerRow.Name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YTM-Peer", peerRow.Name)
	req.Header.Set("X-YTM-Signature", Sign(peerRow.SharedSecret, body))
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to peer %s: %w", peerRow.Name, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response from peer %s: %w", peerRow.Name, err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("peer %s answered with status %s and an unreadable body: %w", peerRow.Name, resp.Status, err)
	}
	if !env.OK {
		if env.Error != "" {
			return nil, fmt.Errorf("peer %s: %s", peerRow.Name, env.Error)
		}
		return nil, fmt.Errorf("peer %s answered with status %s and no error detail", peerRow.Name, resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s answered with status %s", peerRow.Name, resp.Status)
	}
	return env.Data, nil
}

// touchPeer records a successful exchange in the peer's
// last_seen_at.
func touchPeer(store *repo.Store, peerRow *model.Peer) error {
	now := time.Now()
	peerRow.LastSeenAt = &now
	if err := store.Peers.Update(peerRow); err != nil {
		return fmt.Errorf("update last seen for peer %s: %w", peerRow.Name, err)
	}
	return nil
}
