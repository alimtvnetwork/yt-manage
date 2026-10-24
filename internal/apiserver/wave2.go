package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
)

// Wave 2 (spec 29, items 26-50) endpoints: search suggestions and
// cross-entity search, rename recipes, per-video title history,
// episode board operations, and schedule previews.

const settingRenameRecipes = "ui.rename_recipes"

// handleSearchSuggestions serves GET /api/search/suggestions?q=:
// completions drawn from the mirror itself - matching titles and
// the series they belong to (item 27).
func (s *Server) handleSearchSuggestions(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("q")
	videos, err := searchx.Search(s.store, searchx.Query{Text: text, Limit: 50, Sort: "views"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	seen := make(map[string]bool)
	suggestions := []string{}
	add := func(value string) {
		if value != "" && !seen[value] && len(suggestions) < 10 {
			seen[value] = true
			suggestions = append(suggestions, value)
		}
	}
	for _, video := range videos {
		if parsed := episodes.ParseTitle(video.Title); parsed.HasEpisode {
			add(parsed.SeriesKey)
		}
	}
	for _, video := range videos {
		add(video.Title)
	}
	writeOK(w, map[string][]string{"suggestions": suggestions})
}

// searchAllResponse is the data shape of GET /api/search/all.
type searchAllResponse struct {
	Videos    []videoDTO    `json:"videos"`
	Playlists []playlistDTO `json:"playlists"`
	Episodes  []episodeDTO  `json:"episodes"`
	Comments  []commentDTO  `json:"comments"`
}

// handleSearchAll serves GET /api/search/all?q=: one query across
// videos, playlists, episodes, and first comments (item 30).
func (s *Server) handleSearchAll(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	response := searchAllResponse{
		Videos: []videoDTO{}, Playlists: []playlistDTO{},
		Episodes: []episodeDTO{}, Comments: []commentDTO{},
	}
	if text == "" {
		writeOK(w, response)
		return
	}
	videos, err := searchx.Search(s.store, searchx.Query{Text: text, Limit: 10})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, video := range videos {
		response.Videos = append(response.Videos, toVideoDTO(video))
	}
	lower := strings.ToLower(text)
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, channel := range channels {
		playlists, err := s.store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, playlist := range playlists {
			if strings.Contains(strings.ToLower(playlist.Title), lower) && len(response.Playlists) < 10 {
				response.Playlists = append(response.Playlists, toPlaylistDTO(playlist))
			}
		}
		channelEpisodes, err := s.store.Episodes.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, episode := range channelEpisodes {
			if strings.Contains(strings.ToLower(episode.Title), lower) && len(response.Episodes) < 10 {
				dto, err := s.episodeWithParts(episode)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				response.Episodes = append(response.Episodes, dto)
			}
		}
	}
	comments, err := s.store.Comments.ListFirstComments(200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, comment := range comments {
		if strings.Contains(strings.ToLower(comment.Text), lower) && len(response.Comments) < 10 {
			dto := commentDTO{
				ID: comment.ID, VideoID: comment.VideoID, Text: comment.Text,
				IsPinnedKnown: comment.IsPinnedKnown, PublishedAt: formatTimePtr(comment.PublishedAt),
			}
			if video, err := s.store.Videos.GetByID(comment.VideoID); err == nil {
				dto.VideoTitle = video.Title
				dto.YTVideoID = video.YTVideoID
			}
			response.Comments = append(response.Comments, dto)
		}
	}
	writeOK(w, response)
}

// renameRecipe is one saved rename recipe (item 39), persisted as
// JSON in the settings table.
type renameRecipe struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Template            string `json:"template"`
	Regex               string `json:"regex"`
	Replacement         string `json:"replacement"`
	DescriptionTemplate string `json:"descriptionTemplate"`
	StartIndex          int    `json:"startIndex"`
}

func (s *Server) loadRecipes() ([]renameRecipe, error) {
	raw, found, err := s.store.Settings.Get(settingRenameRecipes)
	if err != nil {
		return nil, err
	}
	if !found || raw == "" {
		return []renameRecipe{}, nil
	}
	recipes := []renameRecipe{}
	if err := json.Unmarshal([]byte(raw), &recipes); err != nil {
		return nil, fmt.Errorf("parse rename recipes: %w", err)
	}
	return recipes, nil
}

func (s *Server) storeRecipes(recipes []renameRecipe) error {
	data, err := json.Marshal(recipes)
	if err != nil {
		return err
	}
	return s.store.Settings.Set(settingRenameRecipes, string(data))
}

// handleListRecipes serves GET /api/rename-recipes.
func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	recipes, err := s.loadRecipes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, recipes)
}

// handleCreateRecipe serves POST /api/rename-recipes.
func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	var body renameRecipe
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	recipes, err := s.loadRecipes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id, err := randomHex(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	body.ID = id
	recipes = append(recipes, body)
	if err := s.storeRecipes(recipes); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, recipes)
}

// handleDeleteRecipe serves DELETE /api/rename-recipes/{id}.
func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	recipes, err := s.loadRecipes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	kept := make([]renameRecipe, 0, len(recipes))
	for _, recipe := range recipes {
		if recipe.ID != id {
			kept = append(kept, recipe)
		}
	}
	if err := s.storeRecipes(kept); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, kept)
}

// titleHistoryDTO is one entry of GET /api/videos/{id}/history.
type titleHistoryDTO struct {
	OldTitle  string `json:"oldTitle"`
	NewTitle  string `json:"newTitle"`
	Source    string `json:"source"`
	ChangedAt string `json:"changedAt"`
}

// handleVideoHistory serves GET /api/videos/{id}/history (item 38).
func (s *Server) handleVideoHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entries, err := s.store.TitleHistory.ListByVideo(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]titleHistoryDTO, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, titleHistoryDTO{
			OldTitle: entry.OldTitle, NewTitle: entry.NewTitle,
			Source: entry.Source, ChangedAt: formatTime(entry.CreatedAt),
		})
	}
	writeOK(w, dtos)
}

// resequenceRequest is the body of POST /api/episodes/{id}/resequence.
type resequenceRequest struct {
	VideoIDs []uint `json:"videoIds"`
}

// handleEpisodeResequence serves POST /api/episodes/{id}/resequence:
// the given video order becomes the new sequence, and part numbers
// are renumbered to match (item 42).
func (s *Server) handleEpisodeResequence(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body resequenceRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	episode, err := s.findEpisodeByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	parts, err := s.store.Episodes.ListParts(episode.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(body.VideoIDs) != len(parts) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("videoIds must list every part of the episode exactly once"))
		return
	}
	known := make(map[uint]bool, len(parts))
	for _, part := range parts {
		known[part.VideoID] = true
	}
	newParts := make([]model.EpisodePart, 0, len(body.VideoIDs))
	for i, videoID := range body.VideoIDs {
		if !known[videoID] {
			writeError(w, http.StatusBadRequest, fmt.Errorf("video %d is not a part of episode %d", videoID, id))
			return
		}
		newParts = append(newParts, model.EpisodePart{VideoID: videoID, PartNumber: i + 1, Sequence: i + 1})
	}
	if err := s.store.Episodes.ReplaceParts(episode.ID, newParts); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dto, err := s.episodeWithParts(*episode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, dto)
}

// splitRequest is the body of POST /api/episodes/{id}/split.
type splitRequest struct {
	VideoID uint `json:"videoId"`
}

// handleEpisodeSplit serves POST /api/episodes/{id}/split: parts
// from the given video onward move into a new episode (item 44).
func (s *Server) handleEpisodeSplit(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body splitRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	episode, err := s.findEpisodeByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	parts, err := s.store.Episodes.ListParts(episode.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Sequence < parts[j].Sequence })
	splitAt := -1
	for i, part := range parts {
		if part.VideoID == body.VideoID {
			splitAt = i
		}
	}
	if splitAt <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("split video must be a part of the episode and not its first part"))
		return
	}
	head := parts[:splitAt]
	tail := parts[splitAt:]
	newEpisode := &model.Episode{
		ChannelID:     episode.ChannelID,
		EpisodeKey:    fmt.Sprintf("%s-split-%d", episode.EpisodeKey, body.VideoID),
		EpisodeNumber: episode.EpisodeNumber, Title: episode.Title + " (split)",
	}
	if err := s.store.Episodes.UpsertByKey(newEpisode); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	stored, err := s.store.Episodes.GetByKey(episode.ChannelID, newEpisode.EpisodeKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Episodes.ReplaceParts(episode.ID, head); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.Episodes.ReplaceParts(stored.ID, tail); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]uint{"episodeId": episode.ID, "newEpisodeId": stored.ID})
}

// mergeRequest is the body of POST /api/episodes/merge.
type mergeRequest struct {
	EpisodeIDs []uint `json:"episodeIds"`
}

// handleEpisodeMerge serves POST /api/episodes/merge: the parts of
// every listed episode move into the first one, renumbered in
// order; the emptied episodes are deleted (item 44).
func (s *Server) handleEpisodeMerge(w http.ResponseWriter, r *http.Request) {
	var body mergeRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(body.EpisodeIDs) < 2 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("at least two episodeIds are required"))
		return
	}
	target, err := s.findEpisodeByID(body.EpisodeIDs[0])
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	combined := []model.EpisodePart{}
	for _, episodeID := range body.EpisodeIDs {
		episode, err := s.findEpisodeByID(episodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if episode.ChannelID != target.ChannelID {
			writeError(w, http.StatusBadRequest, fmt.Errorf("episodes must belong to the same channel"))
			return
		}
		parts, err := s.store.Episodes.ListParts(episode.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].Sequence < parts[j].Sequence })
		combined = append(combined, parts...)
	}
	for i := range combined {
		combined[i].ID = 0
		combined[i].PartNumber = i + 1
		combined[i].Sequence = i + 1
	}
	if err := s.store.Episodes.ReplaceParts(target.ID, combined); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, episodeID := range body.EpisodeIDs[1:] {
		if err := s.store.Episodes.Delete(episodeID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	dto, err := s.episodeWithParts(*target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, dto)
}

// schedulePlanEntryDTO is one entry of a schedule preview.
type schedulePlanEntryDTO struct {
	VideoID          uint   `json:"videoId"`
	YTVideoID        string `json:"ytVideoId"`
	Title            string `json:"title"`
	EpisodeNumber    int    `json:"episodeNumber"`
	PartNumber       int    `json:"partNumber"`
	PublishAt        string `json:"publishAt"`
	CurrentPublishAt string `json:"currentPublishAt"`
	IsPublic         bool   `json:"isPublic"`
}

// schedulePlanResponse is the data shape of the schedule-plan endpoint.
type schedulePlanResponse struct {
	Entries       []schedulePlanEntryDTO `json:"entries"`
	SkippedPublic int                    `json:"skippedPublic"`
}

// handleSchedulePlan serves POST /api/episodes/{id}/schedule-plan:
// the publish dates a cadence rule would produce, with each video's
// current publish date alongside, and nothing written (items 46-47).
func (s *Server) handleSchedulePlan(w http.ResponseWriter, r *http.Request) {
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
	entries := make([]schedulePlanEntryDTO, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		dto := schedulePlanEntryDTO{
			VideoID: entry.VideoID, YTVideoID: entry.YTVideoID, Title: entry.Title,
			EpisodeNumber: entry.EpisodeNumber, PartNumber: entry.PartNumber,
			PublishAt: entry.PublishAt.Format(time.RFC3339), IsPublic: entry.IsPublic,
		}
		if video, err := s.store.Videos.GetByID(entry.VideoID); err == nil {
			dto.CurrentPublishAt = formatTimePtr(video.PublishAt)
		}
		entries = append(entries, dto)
	}
	writeOK(w, schedulePlanResponse{Entries: entries, SkippedPublic: len(plan.SkippedPublic)})
}
