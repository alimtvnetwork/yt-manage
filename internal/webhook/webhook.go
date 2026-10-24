// Package webhook sends signed event notifications to a
// user-configured URL (spec 29, item 120). Delivery is
// best-effort and always asynchronous from the caller's point of
// view: an unreachable endpoint never blocks or fails the action
// that produced the event.
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
)

// Event is one webhook payload.
type Event struct {
	Event string      `json:"event"`
	At    string      `json:"at"`
	Data  interface{} `json:"data"`
}

// Fire sends the event in a goroutine when a webhook URL is
// configured. During quiet hours the caller is responsible for
// holding events; Fire itself delivers immediately.
func Fire(settings *config.Settings, event string, data interface{}) {
	if settings == nil || settings.WebhookURL == "" {
		return
	}
	payload := Event{Event: event, At: time.Now().Format(time.RFC3339), Data: data}
	go func() {
		_ = Deliver(settings, payload)
	}()
}

// Deliver posts one event synchronously. The body is the JSON
// payload; when a secret is configured the hex HMAC-SHA256 of the
// body travels in X-YTM-Signature so receivers can verify the
// sender.
func Deliver(settings *config.Settings, payload Event) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, settings.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "yt-manage-webhook")
	if settings.WebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(settings.WebhookSecret))
		mac.Write(body)
		request.Header.Set("X-YTM-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint answered %s", response.Status)
	}
	return nil
}
