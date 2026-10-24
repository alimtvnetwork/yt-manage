package apiserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
)

// handleListEpisodes serves GET /api/episodes?channel=ID.
func (s *Server) handleListEpisodes(w http.ResponseWriter, r *http.Request) {
	channelID, hasChannel, err := parseQueryUint(r, "channel")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !hasChannel || channelID == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channel query parameter is required"))
		return
	}
	episodes, err := s.store.Episodes.ListByChannel(channelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]episodeDTO, 0, len(episodes))
	for _, episode := range episodes {
		dto, err := s.episodeWithParts(episode)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		dtos = append(dtos, dto)
	}
	writeOK(w, dtos)
}

// episodeWithParts builds an episode DTO including its parts, the
// missing part numbers (gap slots for the board, spec 29 item 41),
// and health warnings (item 49).
func (s *Server) episodeWithParts(episode model.Episode) (episodeDTO, error) {
	dto := episodeDTO{
		ID: episode.ID, ChannelID: episode.ChannelID, EpisodeKey: episode.EpisodeKey,
		EpisodeNumber: episode.EpisodeNumber, Title: episode.Title, CadenceDays: episode.CadenceDays,
		FirstPublishAt: formatTimePtr(episode.FirstPublishAt), Parts: []episodePartDTO{},
		GapParts: []int{}, Warnings: []string{},
	}
	parts, err := s.store.Episodes.ListParts(episode.ID)
	if err != nil {
		return episodeDTO{}, err
	}
	seenParts := make(map[int]int)
	maxPart := 0
	for _, part := range parts {
		partDTO := episodePartDTO{Sequence: part.Sequence, PartNumber: part.PartNumber, VideoID: part.VideoID}
		video, err := s.store.Videos.GetByID(part.VideoID)
		if err != nil {
			return episodeDTO{}, err
		}
		partDTO.YTVideoID = video.YTVideoID
		partDTO.Title = video.Title
		partDTO.Visibility = video.Visibility.String()
		partDTO.PublishAt = formatTimePtr(video.PublishAt)
		partDTO.PublishedAt = formatTimePtr(video.PublishedAt)
		dto.Parts = append(dto.Parts, partDTO)
		seenParts[part.PartNumber]++
		if part.PartNumber > maxPart {
			maxPart = part.PartNumber
		}
	}
	for number := 1; number <= maxPart; number++ {
		if seenParts[number] == 0 {
			dto.GapParts = append(dto.GapParts, number)
			dto.Warnings = append(dto.Warnings, fmt.Sprintf("Missing part %d", number))
		}
		if seenParts[number] > 1 {
			dto.Warnings = append(dto.Warnings, fmt.Sprintf("Part %d appears %d times", number, seenParts[number]))
		}
	}
	return dto, nil
}

// findEpisodeByID finds an episode by scanning every channel's
// episode list (the repository has no GetByID for episodes).
func (s *Server) findEpisodeByID(id uint) (*model.Episode, error) {
	channels, err := s.store.Channels.List()
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		episodes, err := s.store.Episodes.ListByChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		for _, episode := range episodes {
			if episode.ID == id {
				found := episode
				return &found, nil
			}
		}
	}
	return nil, fmt.Errorf("episode not found: %d", id)
}

// scheduleRequest is the body of POST /api/episodes/{id}/schedule.
// CadenceDays is a pointer so an omitted value can fall back to the
// episode's stored cadence; Cadence is the named alternative.
type scheduleRequest struct {
	CadenceDays   *int   `json:"cadenceDays"`
	Cadence       string `json:"cadence"`
	StartsAt      string `json:"startsAt"`
	IncludePublic bool   `json:"includePublic"`
}

// scheduleEntryDTO is one applied entry in a schedule response.
type scheduleEntryDTO struct {
	VideoID   uint   `json:"videoId"`
	YTVideoID string `json:"ytVideoId"`
	PublishAt string `json:"publishAt"`
}

// scheduleResponse is the data shape of the schedule endpoint.
type scheduleResponse struct {
	Applied       int                `json:"applied"`
	SkippedPublic int                `json:"skippedPublic"`
	Entries       []scheduleEntryDTO `json:"entries"`
}

// resolveCadenceDays picks the cadence for a schedule request:
// explicit days win, then a named cadence, then the episode's
// stored cadence, then a seven-day default.
func resolveScheduleCadence(body scheduleRequest, episode *model.Episode) (int, error) {
	if body.CadenceDays != nil && *body.CadenceDays > 0 {
		return *body.CadenceDays, nil
	}
	if body.Cadence != "" {
		cadence, err := model.ParseCadence(body.Cadence)
		if err != nil {
			return 0, err
		}
		days := cadence.Days()
		if days == 0 {
			return 0, fmt.Errorf("cadence %q needs an explicit cadenceDays value", body.Cadence)
		}
		return days, nil
	}
	if episode.CadenceDays > 0 {
		return episode.CadenceDays, nil
	}
	return 7, nil
}

// handleScheduleEpisode serves POST /api/episodes/{id}/schedule.
// NOTE: scheduling is channel/series-wide per the data model
// (schedule rules carry no series key); the {id} episode anchors
// the channel whose episodes are scheduled.
func (s *Server) handleScheduleEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body scheduleRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.StartsAt == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("startsAt is required"))
		return
	}
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid startsAt: %w", err))
		return
	}
	episode, err := s.findEpisodeByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	cadenceDays, err := resolveScheduleCadence(body, episode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rule := model.ScheduleRule{
		ChannelID: episode.ChannelID, CadenceDays: cadenceDays,
		StartsAt: &startsAt, IsActive: true,
	}
	plan, err := scheduler.BuildPlan(s.store, rule, body.IncludePublic)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), episode.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	applied, err := scheduler.Apply(r.Context(), svc, s.store, plan)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entries := make([]scheduleEntryDTO, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		entries = append(entries, scheduleEntryDTO{
			VideoID: entry.VideoID, YTVideoID: entry.YTVideoID,
			PublishAt: entry.PublishAt.Format(time.RFC3339),
		})
	}
	writeOK(w, scheduleResponse{
		Applied: applied, SkippedPublic: len(plan.SkippedPublic), Entries: entries,
	})
}
