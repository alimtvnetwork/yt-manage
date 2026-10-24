package apiserver

import (
	"fmt"
	"net/http"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/syncer"
)

// handleListAccounts serves GET /api/accounts.
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.Accounts.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]accountDTO, 0, len(accounts))
	for _, account := range accounts {
		channels, err := s.store.Channels.ListByAccount(account.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		dto := accountDTO{
			ID: account.ID, Label: account.Label, ChannelID: account.ChannelID,
			CreatedAt: formatTime(account.CreatedAt), UpdatedAt: formatTime(account.UpdatedAt),
			Channels: make([]channelDTO, 0, len(channels)),
		}
		for _, channel := range channels {
			dto.Channels = append(dto.Channels, toChannelDTO(channel))
		}
		dtos = append(dtos, dto)
	}
	writeOK(w, dtos)
}

// syncRequest is the body of POST /api/sync.
type syncRequest struct {
	AccountID *uint `json:"accountId"`
	IsFull    bool  `json:"full"`
}

// syncResultDTO is one account's sync outcome. Error is set when
// that account's run failed; the other fields are then zero.
type syncResultDTO struct {
	AccountID     uint   `json:"accountId"`
	ChannelTitle  string `json:"channelTitle"`
	Videos        int    `json:"videos"`
	Playlists     int    `json:"playlists"`
	Comments      int    `json:"comments"`
	UnitsEstimate int    `json:"unitsEstimate"`
	IsIncremental bool   `json:"isIncremental"`
	Error         string `json:"error,omitempty"`
}

// syncResponse is the data shape of POST /api/sync.
type syncResponse struct {
	Results []syncResultDTO `json:"results"`
	Errors  []string        `json:"errors"`
}

// handleSync serves POST /api/sync for one account or all accounts.
//
// Sync is long-running batch work, so the envelope reports partial
// success honestly: the response stays 200 with one result entry per
// account, failed accounts carry their error in the entry and in
// the top-level errors list, and ok stays true as long as the
// request itself was well-formed.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var body syncRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings, err := config.LoadSettings(s.home)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	accountIDs, err := s.syncAccountIDs(body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	response := syncResponse{Results: []syncResultDTO{}, Errors: []string{}}
	for _, accountID := range accountIDs {
		result, err := syncer.RunAccount(r.Context(), s.home, s.store, s.key, accountID, body.IsFull, settings.DailyQuotaBudget)
		if err != nil {
			response.Results = append(response.Results, syncResultDTO{AccountID: accountID, Error: err.Error()})
			response.Errors = append(response.Errors, err.Error())
			continue
		}
		response.Results = append(response.Results, syncResultDTO{
			AccountID: result.AccountID, ChannelTitle: result.ChannelTitle,
			Videos: result.Videos, Playlists: result.Playlists, Comments: result.Comments,
			UnitsEstimate: result.UnitsEstimate, IsIncremental: result.IsIncremental,
		})
		s.addQuota(result.UnitsEstimate)
		s.recordActivity("sync", fmt.Sprintf("Synced %s: %d videos, %d playlists, %d comments", result.ChannelTitle, result.Videos, result.Playlists, result.Comments), "")
	}
	writeOK(w, response)
}

// syncAccountIDs resolves a sync request to the account ids to run:
// the single requested account, or every stored account.
func (s *Server) syncAccountIDs(body syncRequest) ([]uint, error) {
	if body.AccountID != nil {
		return []uint{*body.AccountID}, nil
	}
	accounts, err := s.store.Accounts.List()
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids, nil
}
