package apiserver

import (
	"fmt"
	"net/http"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// handleListPlaylists serves GET /api/playlists?channel=ID.
func (s *Server) handleListPlaylists(w http.ResponseWriter, r *http.Request) {
	channelID, hasChannel, err := parseQueryUint(r, "channel")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !hasChannel || channelID == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channel query parameter is required"))
		return
	}
	playlists, err := s.store.Playlists.ListByChannel(channelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]playlistDTO, 0, len(playlists))
	for _, playlist := range playlists {
		dtos = append(dtos, toPlaylistDTO(playlist))
	}
	writeOK(w, dtos)
}

// createPlaylistRequest is the body of POST /api/playlists.
type createPlaylistRequest struct {
	ChannelID   uint   `json:"channelId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Privacy     string `json:"privacy"`
}

// handleCreatePlaylist serves POST /api/playlists: it creates the
// playlist through the channel account's YouTube service and then
// mirrors it locally.
func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var body createPlaylistRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.ChannelID == 0 || body.Title == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId and title are required"))
		return
	}
	privacy := body.Privacy
	if privacy == "" {
		privacy = "private"
	}
	channel, err := s.findChannel(body.ChannelID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), channel.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := svc.Playlists.Insert([]string{"snippet", "status"}, &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title: body.Title, Description: body.Description, ChannelId: channel.YTChannelID,
		},
		Status: &youtube.PlaylistStatus{PrivacyStatus: privacy},
	}).Context(r.Context()).Do()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record := &model.Playlist{
		ChannelID: channel.ID, YTPlaylistID: created.Id,
		Title: body.Title, Description: body.Description, ItemCount: 0,
	}
	if created.Snippet != nil {
		record.Title = created.Snippet.Title
		record.Description = created.Snippet.Description
	}
	if created.ContentDetails != nil {
		record.ItemCount = created.ContentDetails.ItemCount
	}
	if err := s.store.Playlists.UpsertByYTID(record); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	saved, err := s.store.Playlists.GetByYTID(created.Id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toPlaylistDTO(*saved))
}

// findPlaylistByID finds a playlist by scanning every channel's
// playlists (the repository has no GetByID for playlists).
func (s *Server) findPlaylistByID(id uint) (*model.Playlist, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		playlists, err := s.store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		for _, playlist := range playlists {
			if playlist.ID == id {
				found := playlist
				return &found, nil
			}
		}
	}
	return nil, fmt.Errorf("playlist not found: %d", id)
}

// patchPlaylistRequest is the body of PATCH /api/playlists/{id}.
type patchPlaylistRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// handlePatchPlaylist serves PATCH /api/playlists/{id}: it fetches
// the playlist through the API, updates the requested fields, writes
// it back, and mirrors the result preserving the item count.
func (s *Server) handlePatchPlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body patchPlaylistRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Title == nil && body.Description == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("at least one of title or description is required"))
		return
	}
	playlist, err := s.findPlaylistByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), playlist.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := svc.Playlists.List([]string{"snippet", "status"}).Id(playlist.YTPlaylistID).Context(r.Context()).Do()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(resp.Items) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("playlists.list %s: no playlist returned", playlist.YTPlaylistID))
		return
	}
	full := resp.Items[0]
	if full.Snippet == nil {
		full.Snippet = &youtube.PlaylistSnippet{}
	}
	if full.Status == nil {
		full.Status = &youtube.PlaylistStatus{}
	}
	newTitle := playlist.Title
	if body.Title != nil {
		full.Snippet.Title = *body.Title
		newTitle = *body.Title
	}
	newDescription := playlist.Description
	if body.Description != nil {
		full.Snippet.Description = *body.Description
		newDescription = *body.Description
	}
	if _, err := svc.Playlists.Update([]string{"snippet", "status"}, full).Context(r.Context()).Do(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record := &model.Playlist{
		ChannelID: playlist.ChannelID, YTPlaylistID: playlist.YTPlaylistID,
		Title: newTitle, Description: newDescription, ItemCount: playlist.ItemCount,
	}
	if err := s.store.Playlists.UpsertByYTID(record); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	saved, err := s.store.Playlists.GetByYTID(playlist.YTPlaylistID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toPlaylistDTO(*saved))
}
