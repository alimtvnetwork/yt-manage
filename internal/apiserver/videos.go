package apiserver

import (
	"fmt"
	"net/http"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
)

// videosResponse is the data shape of the video list endpoints.
type videosResponse struct {
	Videos []videoDTO `json:"videos"`
	Count  int        `json:"count"`
}

// handleListVideos serves GET /api/videos and GET /api/search.
func (s *Server) handleListVideos(w http.ResponseWriter, r *http.Request) {
	channelID, _, err := parseQueryUint(r, "channel")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	query := searchx.Query{
		Text: r.URL.Query().Get("q"), ChannelID: channelID,
		Visibility:  r.URL.Query().Get("visibility"),
		Sort:        r.URL.Query().Get("sort"),
		OnlyShorts:  r.URL.Query().Get("shorts") == "1",
		HasComments: r.URL.Query().Get("hasComments") == "1",
		Limit:       limit, Offset: offset,
	}
	videos, err := searchx.Search(s.store, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	total, err := s.store.Videos.CountMatching(repo.VideoQuery{
		Text: query.Text, ChannelID: query.ChannelID, Visibility: query.Visibility,
		Sort: query.Sort, OnlyShorts: query.OnlyShorts, HasComments: query.HasComments,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]videoDTO, 0, len(videos))
	for _, video := range videos {
		dtos = append(dtos, toVideoDTO(video))
	}
	writeOK(w, videosResponse{Videos: dtos, Count: int(total)})
}

// handleGetVideo serves GET /api/videos/{id}.
func (s *Server) handleGetVideo(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	video, err := s.store.Videos.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toVideoDTO(*video))
}

// patchVideoRequest is the body of PATCH /api/videos/{id}.
type patchVideoRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// handlePatchVideo serves PATCH /api/videos/{id}: it applies the
// requested title and/or description through a one-change batch plan.
func (s *Server) handlePatchVideo(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body patchVideoRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Title == nil && body.Description == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("at least one of title or description is required"))
		return
	}
	video, err := s.store.Videos.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	plan := singleChangePlan(video, body.Title, body.Description)
	if _, err := batch.Apply(r.Context(), svc, s.store, plan); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.store.Videos.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, toVideoDTO(*updated))
}
