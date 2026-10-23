package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"google.golang.org/api/youtube/v3"
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

// parsePathID parses the {id} path value as an unsigned id.
func parsePathID(r *http.Request) (uint, error) {
	return parseUintString(r.PathValue("id"))
}

// parseUintString parses s as an unsigned id.
func parseUintString(s string) (uint, error) {
	value, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	if value == 0 {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return uint(value), nil
}

// parseQueryUint parses an optional unsigned query parameter. The
// second result reports whether a value was supplied.
func parseQueryUint(r *http.Request, name string) (uint, bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, true, fmt.Errorf("invalid %s %q", name, raw)
	}
	return uint(value), true, nil
}

// parseQueryInt parses an optional integer query parameter.
func parseQueryInt(r *http.Request, name string) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	return value, nil
}

// decodeJSON decodes the request body into target.
func decodeJSON(r *http.Request, target interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

// decodeOptionalJSON decodes the request body when one is present;
// an empty body leaves target untouched.
func decodeOptionalJSON(r *http.Request, target interface{}) error {
	if r.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

// isNotFound reports whether err is a repository not-found error.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// randomHex returns n random bytes encoded as hex.
func randomHex(n int) (string, error) {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// channelAccountMap builds the local channel id to account id map.
func (s *Server) channelAccountMap() (map[uint]uint, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	mapping := make(map[uint]uint, len(channels))
	for _, channel := range channels {
		mapping[channel.ID] = channel.AccountID
	}
	return mapping, nil
}

// findChannel returns a channel by its local id.
func (s *Server) findChannel(channelID uint) (*model.Channel, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		if channel.ID == channelID {
			found := channel
			return &found, nil
		}
	}
	return nil, fmt.Errorf("channel not found: %d", channelID)
}

// serviceForChannel builds the YouTube service for the account that
// owns the given local channel.
func (s *Server) serviceForChannel(ctx context.Context, channelID uint) (*youtube.Service, error) {
	mapping, err := s.channelAccountMap()
	if err != nil {
		return nil, err
	}
	accountID, hasAccount := mapping[channelID]
	if !hasAccount {
		return nil, fmt.Errorf("no account found for channel %d", channelID)
	}
	return ytapi.ServiceForAccount(ctx, s.home, s.store, s.key, accountID)
}

// serviceForAccountID builds the YouTube service for an account.
func (s *Server) serviceForAccountID(r *http.Request, accountID uint) (*youtube.Service, error) {
	return ytapi.ServiceForAccount(r.Context(), s.home, s.store, s.key, accountID)
}

// singleChangePlan builds a one-change batch plan from optional
// title and description replacements for a video.
func singleChangePlan(video *model.Video, title *string, description *string) *batch.Plan {
	newTitle := video.Title
	if title != nil {
		newTitle = *title
	}
	newDescription := video.Description
	hasDescriptionChange := false
	if description != nil {
		newDescription = *description
		hasDescriptionChange = newDescription != video.Description
	}
	change := batch.Change{
		VideoID: video.ID, YTVideoID: video.YTVideoID, ChannelID: video.ChannelID,
		OldTitle: video.Title, NewTitle: newTitle,
		OldDescription: video.Description, NewDescription: newDescription,
		HasDescriptionChange: hasDescriptionChange,
	}
	return &batch.Plan{Changes: []batch.Change{change}}
}
