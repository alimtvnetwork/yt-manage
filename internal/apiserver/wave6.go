package apiserver

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/comments"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/insights"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

// Wave 6 (spec 29, items 126-150) endpoints: comment link
// checking, coverage, bulk comment posting, playlist operations,
// and the insights surface.

var urlPattern = regexp.MustCompile(`https?://[^\s<>"')]+`)

// linkCheckResult is one checked URL from a first comment.
type linkCheckResult struct {
	URL       string `json:"url"`
	CommentID uint   `json:"commentId"`
	VideoID   uint   `json:"videoId"`
	Status    int    `json:"status"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

// handleCheckCommentLinks serves POST /api/comments/check-links
// (item 126): every distinct URL in the managed first comments
// is fetched once; dead links (non-2xx/3xx or unreachable) are
// reported with the comment they live in.
func (s *Server) handleCheckCommentLinks(w http.ResponseWriter, r *http.Request) {
	firsts, err := s.store.Comments.ListFirstComments(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	seen := map[string]linkCheckResult{}
	order := []string{}
	for _, comment := range firsts {
		for _, url := range urlPattern.FindAllString(comment.Text, -1) {
			url = strings.TrimRight(url, ".,")
			if _, done := seen[url]; done {
				continue
			}
			seen[url] = linkCheckResult{URL: url, CommentID: comment.ID, VideoID: comment.VideoID}
			order = append(order, url)
			if len(order) >= 20 {
				break
			}
		}
		if len(order) >= 20 {
			break
		}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	results := []linkCheckResult{}
	for _, url := range order {
		result := seen[url]
		response, fetchErr := client.Head(url)
		if fetchErr != nil {
			result.Error = fetchErr.Error()
		} else {
			result.Status = response.StatusCode
			result.OK = response.StatusCode >= 200 && response.StatusCode < 400
			response.Body.Close()
		}
		results = append(results, result)
	}
	writeOK(w, results)
}

// coverageDTO is one channel's comment coverage (item 127).
type coverageDTO struct {
	ChannelID   uint    `json:"channelId"`
	Public      int     `json:"public"`
	WithFirst   int     `json:"withFirst"`
	Percent     float64 `json:"percent"`
	GapVideoIDs []uint  `json:"gapVideoIds"`
	GapCount    int     `json:"gapCount"`
}

// handleCommentCoverage serves GET /api/comment-coverage.
func (s *Server) handleCommentCoverage(w http.ResponseWriter, r *http.Request) {
	firsts, err := s.store.Comments.ListFirstComments(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	withFirst := map[uint]bool{}
	for _, first := range firsts {
		withFirst[first.VideoID] = true
	}
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	reports := []coverageDTO{}
	for _, channel := range channels {
		report := coverageDTO{ChannelID: channel.ID, GapVideoIDs: []uint{}}
		videos, err := s.store.Videos.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, video := range videos {
			if video.Visibility != model.VisibilityPublic {
				continue
			}
			report.Public++
			if withFirst[video.ID] {
				report.WithFirst++
			} else {
				report.GapCount++
				if len(report.GapVideoIDs) < 200 {
					report.GapVideoIDs = append(report.GapVideoIDs, video.ID)
				}
			}
		}
		if report.Public > 0 {
			report.Percent = float64(int(float64(report.WithFirst)*1000/float64(report.Public))) / 10
		}
		reports = append(reports, report)
	}
	writeOK(w, reports)
}

// videoCommentDTO is one mirrored comment in the triage view.
type videoCommentDTO struct {
	ID        uint   `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	LikeCount int64  `json:"likeCount"`
	IsFirst   bool   `json:"isFirst"`
}

// handleListVideoComments serves GET /api/videos/{id}/comments
// (item 128): the mirrored top comments, most liked first.
// Read-only: replies happen in Studio.
func (s *Server) handleListVideoComments(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	list, err := s.store.Comments.ListByVideo(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]videoCommentDTO, 0, len(list))
	for _, comment := range list {
		dtos = append(dtos, videoCommentDTO{
			ID: comment.ID, Author: comment.Author, Text: comment.Text,
			LikeCount: comment.LikeCount, IsFirst: comment.IsFirst,
		})
	}
	writeOK(w, dtos)
}

// bulkCommentPreviewRequest is the body of POST /api/comments/bulk-preview.
type bulkCommentPreviewRequest struct {
	ChannelID uint   `json:"channelId"`
	Template  string `json:"template"`
	VideoIDs  []uint `json:"videoIds"`
}

// bulkCommentRow is one row of the bulk post preview/apply.
type bulkCommentRow struct {
	VideoID uint   `json:"videoId"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Error   string `json:"error,omitempty"`
}

// pendingCommentVideos returns the channel's public videos with
// no first comment yet, optionally narrowed to the given ids.
func (s *Server) pendingCommentVideos(channelID uint, onlyIDs []uint) ([]model.Video, error) {
	videos, err := s.store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	only := map[uint]bool{}
	for _, id := range onlyIDs {
		only[id] = true
	}
	pending := []model.Video{}
	for _, video := range videos {
		if video.Visibility != model.VisibilityPublic {
			continue
		}
		if len(only) > 0 && !only[video.ID] {
			continue
		}
		if _, err := s.store.Comments.GetFirstByVideo(video.ID); err == nil {
			continue
		}
		pending = append(pending, video)
	}
	return pending, nil
}

// handleBulkCommentPreview serves POST /api/comments/bulk-preview
// (item 130): the templated first comment rendered per pending
// video, change-set style, before anything is posted.
func (s *Server) handleBulkCommentPreview(w http.ResponseWriter, r *http.Request) {
	var body bulkCommentPreviewRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	channel, err := s.findChannel(body.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	template := body.Template
	if template == "" {
		template = s.loadChannelDefaults(body.ChannelID).CommentTemplate
	}
	pending, err := s.pendingCommentVideos(body.ChannelID, body.VideoIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rows := make([]bulkCommentRow, 0, len(pending))
	for _, video := range pending {
		rows = append(rows, bulkCommentRow{
			VideoID: video.ID, Title: video.Title,
			Text: renderCommentTemplate(template, video, channel.Title),
		})
	}
	writeOK(w, rows)
}

// bulkCommentPostRequest is the body of POST /api/comments/bulk-post.
type bulkCommentPostRequest struct {
	Items []bulkCommentRow `json:"items"`
}

// handleBulkCommentPost serves POST /api/comments/bulk-post:
// posts each row's text as the video's first comment, with
// per-item results.
func (s *Server) handleBulkCommentPost(w http.ResponseWriter, r *http.Request) {
	var body bulkCommentPostRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rows := make([]bulkCommentRow, 0, len(body.Items))
	posted := 0
	for _, item := range body.Items {
		row := item
		video, err := s.store.Videos.GetByID(item.VideoID)
		if err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		svc, err := s.serviceForChannel(r.Context(), video.ChannelID)
		if err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		if _, err := comments.PostFirstComment(r.Context(), svc, s.store, *video, item.Text); err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		posted++
		s.addQuota(50)
		rows = append(rows, row)
	}
	if posted > 0 {
		s.recordActivity("comment", fmt.Sprintf("Bulk-posted %d first comment(s)", posted), "")
	}
	writeOK(w, map[string]interface{}{"posted": posted, "results": rows})
}

// livePlaylistItems fetches a playlist's items from the API,
// following pages up to a sane cap.
func (s *Server) livePlaylistItems(ctx *http.Request, svc *youtube.Service, ytPlaylistID string) ([]*youtube.PlaylistItem, error) {
	items := []*youtube.PlaylistItem{}
	pageToken := ""
	for {
		call := svc.PlaylistItems.List([]string{"snippet", "contentDetails"}).PlaylistId(ytPlaylistID).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		response, err := call.Context(ctx.Context()).Do()
		if err != nil {
			return nil, err
		}
		items = append(items, response.Items...)
		if response.NextPageToken == "" || len(items) >= 500 {
			return items, nil
		}
		pageToken = response.NextPageToken
	}
}

// playlistItemAddRequest is the body of POST /api/playlists/{id}/items.
type playlistItemAddRequest struct {
	VideoID uint `json:"videoId"`
}

// handleAddPlaylistItem serves POST /api/playlists/{id}/items
// (board moves, item 131).
func (s *Server) handleAddPlaylistItem(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body playlistItemAddRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	video, err := s.store.Videos.GetByID(body.VideoID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	item := proposalAction{Type: "playlist", VideoID: video.ID, PlaylistID: playlist.ID}
	if _, err := s.executeActions(r.Context(), []proposalAction{item}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeOK(w, map[string]bool{"added": true})
}

// handleRemovePlaylistItem serves DELETE
// /api/playlists/{id}/items/{videoId}: the live item for that
// video is deleted through the API and the mirror refreshed on
// the next sync.
func (s *Server) handleRemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	videoID, err := strconv.ParseUint(r.PathValue("videoId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	video, err := s.store.Videos.GetByID(uint(videoID))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), playlist.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.livePlaylistItems(r, svc, playlist.YTPlaylistID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, item := range items {
		if item.Snippet != nil && item.Snippet.ResourceId != nil && item.Snippet.ResourceId.VideoId == video.YTVideoID {
			if err := svc.PlaylistItems.Delete(item.Id).Context(r.Context()).Do(); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			s.addQuota(50)
			writeOK(w, map[string]bool{"removed": true})
			return
		}
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("video %d is not in playlist %d", videoID, id))
}

// seriesVideosInOrder returns a show's videos sorted by episode
// then part.
func (s *Server) seriesVideosInOrder(channelID uint, seriesKey string) ([]model.Video, error) {
	videos, err := s.store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	type scored struct {
		video   model.Video
		episode int
		part    int
	}
	scoredList := []scored{}
	for _, video := range videos {
		parsed := episodes.ParseTitle(video.Title)
		if parsed.SeriesKey == "" || !parsed.HasEpisode {
			continue
		}
		if episodes.ShowKey(parsed.SeriesKey) != seriesKey {
			continue
		}
		scoredList = append(scoredList, scored{video: video, episode: parsed.EpisodeNumber, part: parsed.PartNumber})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].episode != scoredList[j].episode {
			return scoredList[i].episode < scoredList[j].episode
		}
		return scoredList[i].part < scoredList[j].part
	})
	ordered := make([]model.Video, 0, len(scoredList))
	for _, entry := range scoredList {
		ordered = append(ordered, entry.video)
	}
	return ordered, nil
}

// seriesPlaylistRequest is the body of POST /api/series/playlist.
type seriesPlaylistRequest struct {
	ChannelID  uint   `json:"channelId"`
	SeriesKey  string `json:"seriesKey"`
	PlaylistID uint   `json:"playlistId"`
}

// handleSeriesPlaylist serves POST /api/series/playlist (item
// 132): create the series' playlist when missing, add every
// missing part in episode order, and set the order to match.
// The mirror catches up on the next sync; the response reports
// exactly what was done live.
func (s *Server) handleSeriesPlaylist(w http.ResponseWriter, r *http.Request) {
	var body seriesPlaylistRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ordered, err := s.seriesVideosInOrder(body.ChannelID, body.SeriesKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(ordered) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("no videos found for series %q", body.SeriesKey))
		return
	}
	svc, err := s.serviceForChannel(r.Context(), body.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ytPlaylistID := ""
	playlistTitle := ""
	if body.PlaylistID != 0 {
		playlist, getErr := s.store.Playlists.GetByID(body.PlaylistID)
		if getErr != nil {
			writeError(w, http.StatusNotFound, getErr)
			return
		}
		ytPlaylistID = playlist.YTPlaylistID
		playlistTitle = playlist.Title
	} else {
		defaults := s.loadChannelDefaults(body.ChannelID)
		playlistTitle = renderPlaylistTemplate(defaults.PlaylistTitleTemplate, body.SeriesKey, "", len(ordered))
		if playlistTitle == "" {
			playlistTitle = titleCase(body.SeriesKey)
		}
		created, createErr := svc.Playlists.Insert([]string{"snippet", "status"}, &youtube.Playlist{
			Snippet: &youtube.PlaylistSnippet{Title: playlistTitle},
			Status:  &youtube.PlaylistStatus{PrivacyStatus: "private"},
		}).Context(r.Context()).Do()
		if createErr != nil {
			writeError(w, http.StatusBadRequest, createErr)
			return
		}
		ytPlaylistID = created.Id
		s.addQuota(50)
	}
	live, err := s.livePlaylistItems(r, svc, ytPlaylistID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	present := map[string]bool{}
	for _, item := range live {
		if item.Snippet != nil && item.Snippet.ResourceId != nil {
			present[item.Snippet.ResourceId.VideoId] = true
		}
	}
	added := 0
	for _, video := range ordered {
		if present[video.YTVideoID] {
			continue
		}
		_, insertErr := svc.PlaylistItems.Insert([]string{"snippet"}, &youtube.PlaylistItem{
			Snippet: &youtube.PlaylistItemSnippet{
				PlaylistId: ytPlaylistID,
				ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: video.YTVideoID},
			},
		}).Context(r.Context()).Do()
		if insertErr != nil {
			writeError(w, http.StatusBadRequest, insertErr)
			return
		}
		added++
		s.addQuota(50)
	}
	s.recordActivity("playlist", fmt.Sprintf("Series %q playlist %q: %d added, %d total", body.SeriesKey, playlistTitle, added, len(ordered)), "")
	writeOK(w, map[string]interface{}{"playlistTitle": playlistTitle, "ytPlaylistId": ytPlaylistID, "added": added, "total": len(ordered)})
}

func titleCase(key string) string {
	words := strings.Fields(key)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

// renderPlaylistTemplate expands playlist template tokens.
func renderPlaylistTemplate(template string, seriesKey string, channelTitle string, count int) string {
	if template == "" {
		return ""
	}
	return strings.NewReplacer(
		"{series}", titleCase(seriesKey),
		"{channel}", channelTitle,
		"{count}", strconv.Itoa(count),
	).Replace(template)
}

// gapsDTO is the playlist gaps report (item 133).
type gapsDTO struct {
	PlaylistID   uint   `json:"playlistId"`
	SeriesKey    string `json:"seriesKey"`
	MissingIDs   []uint `json:"missingVideoIds"`
	ExtraIDs     []uint `json:"extraVideoIds"`
	MissingCount int    `json:"missingCount"`
	ExtraCount   int    `json:"extraCount"`
}

// handlePlaylistGaps serves GET /api/playlists/{id}/gaps?series=key.
func (s *Server) handlePlaylistGaps(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	seriesKey := r.URL.Query().Get("series")
	if seriesKey == "" {
		seriesKey = episodes.ShowKey(episodes.ParseTitle(playlist.Title).SeriesKey)
		if parsed := episodes.ParseTitle(playlist.Title); parsed.SeriesKey != "" {
			seriesKey = episodes.ShowKey(parsed.SeriesKey)
		} else {
			seriesKey = strings.ToLower(playlist.Title)
		}
	}
	ordered, err := s.seriesVideosInOrder(playlist.ChannelID, seriesKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := s.store.Playlists.ListItems(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	inPlaylist := map[uint]bool{}
	for _, item := range items {
		inPlaylist[item.VideoID] = true
	}
	inSeries := map[uint]bool{}
	report := gapsDTO{PlaylistID: id, SeriesKey: seriesKey, MissingIDs: []uint{}, ExtraIDs: []uint{}}
	for _, video := range ordered {
		inSeries[video.ID] = true
		if !inPlaylist[video.ID] {
			report.MissingIDs = append(report.MissingIDs, video.ID)
		}
	}
	for _, item := range items {
		if !inSeries[item.VideoID] {
			report.ExtraIDs = append(report.ExtraIDs, item.VideoID)
		}
	}
	report.MissingCount = len(report.MissingIDs)
	report.ExtraCount = len(report.ExtraIDs)
	writeOK(w, report)
}

// duplicateGroup is one duplicate finding (item 134).
type duplicateGroup struct {
	Kind       string `json:"kind"` // video-in-playlist | playlists
	PlaylistID uint   `json:"playlistId"`
	Detail     string `json:"detail"`
	VideoIDs   []uint `json:"videoIds,omitempty"`
}

// handlePlaylistDuplicates serves GET /api/playlists/duplicates.
func (s *Server) handlePlaylistDuplicates(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.Channels.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	groups := []duplicateGroup{}
	for _, channel := range channels {
		playlists, err := s.store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		videoSets := map[uint]map[uint]bool{}
		for _, playlist := range playlists {
			items, err := s.store.Playlists.ListItems(playlist.ID)
			if err != nil {
				continue
			}
			seen := map[uint]int{}
			videoSets[playlist.ID] = map[uint]bool{}
			for _, item := range items {
				seen[item.VideoID]++
				videoSets[playlist.ID][item.VideoID] = true
			}
			for videoID, count := range seen {
				if count > 1 {
					groups = append(groups, duplicateGroup{
						Kind: "video-in-playlist", PlaylistID: playlist.ID,
						Detail:   fmt.Sprintf("Video %d appears %d times in %q", videoID, count, playlist.Title),
						VideoIDs: []uint{videoID},
					})
				}
			}
		}
		for i := 0; i < len(playlists); i++ {
			for j := i + 1; j < len(playlists); j++ {
				a, b := playlists[i], playlists[j]
				shared := 0
				for videoID := range videoSets[a.ID] {
					if videoSets[b.ID][videoID] {
						shared++
					}
				}
				bigger := len(videoSets[a.ID])
				if len(videoSets[b.ID]) > bigger {
					bigger = len(videoSets[b.ID])
				}
				if bigger > 0 && float64(shared)/float64(bigger) >= 0.8 {
					groups = append(groups, duplicateGroup{
						Kind: "playlists", PlaylistID: a.ID,
						Detail: fmt.Sprintf("%q and %q share %d of %d videos - possible duplicates", a.Title, b.Title, shared, bigger),
					})
				}
			}
		}
	}
	writeOK(w, groups)
}

// reorderRequest is the body of the reorder endpoints.
type reorderRequest struct {
	Mode string `json:"mode"` // episode | date
}

// reorderRow is one row of a reorder preview.
type reorderRow struct {
	VideoID     uint   `json:"videoId"`
	Title       string `json:"title"`
	CurrentPos  int    `json:"currentPos"`
	ProposedPos int    `json:"proposedPos"`
}

// proposedOrder computes the desired order of a playlist's
// mirrored items.
func (s *Server) proposedOrder(playlistID uint, mode string) ([]reorderRow, []model.PlaylistItem, error) {
	items, err := s.store.Playlists.ListItems(playlistID)
	if err != nil {
		return nil, nil, err
	}
	type entry struct {
		item    model.PlaylistItem
		episode int
		part    int
		date    time.Time
	}
	entries := []entry{}
	for _, item := range items {
		video, videoErr := s.store.Videos.GetByID(item.VideoID)
		if videoErr != nil {
			continue
		}
		parsed := episodes.ParseTitle(video.Title)
		date := time.Time{}
		if video.PublishedAt != nil {
			date = *video.PublishedAt
		}
		entries = append(entries, entry{item: item, episode: parsed.EpisodeNumber, part: parsed.PartNumber, date: date})
	}
	sort.Slice(entries, func(i, j int) bool {
		if mode == "date" {
			return entries[i].date.Before(entries[j].date)
		}
		if entries[i].episode != entries[j].episode {
			return entries[i].episode < entries[j].episode
		}
		return entries[i].part < entries[j].part
	})
	rows := []reorderRow{}
	orderedItems := []model.PlaylistItem{}
	for position, e := range entries {
		title := ""
		if video, videoErr := s.store.Videos.GetByID(e.item.VideoID); videoErr == nil {
			title = video.Title
		}
		rows = append(rows, reorderRow{VideoID: e.item.VideoID, Title: title, CurrentPos: e.item.Position, ProposedPos: position})
		orderedItems = append(orderedItems, e.item)
	}
	return rows, orderedItems, nil
}

// handlePlaylistReorderPreview serves POST /api/playlists/{id}/reorder-preview.
func (s *Server) handlePlaylistReorderPreview(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body reorderRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rows, _, err := s.proposedOrder(id, body.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, rows)
}

// handlePlaylistReorder serves POST /api/playlists/{id}/reorder
// (item 136): rebuilds the live playlist in the proposed order
// (delete all items, re-insert in order), capped at 200 items.
func (s *Server) handlePlaylistReorder(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body reorderRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	_, orderedItems, err := s.proposedOrder(id, body.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(orderedItems) > 200 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("reorder is capped at 200 items; this playlist has %d", len(orderedItems)))
		return
	}
	svc, err := s.serviceForChannel(r.Context(), playlist.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	live, err := s.livePlaylistItems(r, svc, playlist.YTPlaylistID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, item := range live {
		if err := svc.PlaylistItems.Delete(item.Id).Context(r.Context()).Do(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.addQuota(50)
	}
	for _, item := range orderedItems {
		video, videoErr := s.store.Videos.GetByID(item.VideoID)
		if videoErr != nil {
			continue
		}
		_, insertErr := svc.PlaylistItems.Insert([]string{"snippet"}, &youtube.PlaylistItem{
			Snippet: &youtube.PlaylistItemSnippet{
				PlaylistId: playlist.YTPlaylistID,
				ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: video.YTVideoID},
			},
		}).Context(r.Context()).Do()
		if insertErr != nil {
			writeError(w, http.StatusBadRequest, insertErr)
			return
		}
		s.addQuota(50)
	}
	s.recordActivity("playlist", fmt.Sprintf("Reordered playlist %q by %s", playlist.Title, body.Mode), "")
	writeOK(w, map[string]interface{}{"reordered": len(orderedItems)})
}

// privacyRequest is the body of POST /api/playlists/{id}/privacy.
type privacyRequest struct {
	Privacy string `json:"privacy"`
}

// handlePlaylistPrivacy serves POST /api/playlists/{id}/privacy
// (item 136).
func (s *Server) handlePlaylistPrivacy(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body privacyRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := model.ParseVisibility(body.Privacy); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("privacy must be public, unlisted or private"))
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), playlist.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	response, err := svc.Playlists.List([]string{"snippet", "status"}).Id(playlist.YTPlaylistID).Context(r.Context()).Do()
	if err != nil || len(response.Items) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("fetch playlist: %v", err))
		return
	}
	full := response.Items[0]
	if full.Status == nil {
		full.Status = &youtube.PlaylistStatus{}
	}
	full.Status.PrivacyStatus = body.Privacy
	if _, err := svc.Playlists.Update([]string{"snippet", "status"}, full).Context(r.Context()).Do(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.addQuota(50)
	s.recordActivity("playlist", fmt.Sprintf("Playlist %q privacy set to %s", playlist.Title, body.Privacy), "")
	writeOK(w, map[string]string{"privacy": body.Privacy})
}

// orphanGroup is one group of orphan videos (item 137).
type orphanGroup struct {
	SeriesKey string `json:"seriesKey"`
	VideoIDs  []uint `json:"videoIds"`
	Count     int    `json:"count"`
}

// handleOrphans serves GET /api/orphans?channelId=.
func (s *Server) handleOrphans(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	playlists, err := s.store.Playlists.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	inPlaylist := map[uint]bool{}
	for _, playlist := range playlists {
		items, err := s.store.Playlists.ListItems(playlist.ID)
		if err != nil {
			continue
		}
		for _, item := range items {
			inPlaylist[item.VideoID] = true
		}
	}
	videos, err := s.store.Videos.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	groups := map[string]*orphanGroup{}
	for _, video := range videos {
		if video.Visibility != model.VisibilityPublic || inPlaylist[video.ID] {
			continue
		}
		parsed := episodes.ParseTitle(video.Title)
		key := "(no series)"
		if parsed.SeriesKey != "" {
			key = episodes.ShowKey(parsed.SeriesKey)
		}
		group, ok := groups[key]
		if !ok {
			group = &orphanGroup{SeriesKey: key}
			groups[key] = group
		}
		group.VideoIDs = append(group.VideoIDs, video.ID)
		group.Count++
	}
	result := []orphanGroup{}
	for _, group := range groups {
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	writeOK(w, result)
}

// checklistItem is one publish-checklist line (item 138).
type checklistItem struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// handlePlaylistChecklist serves GET /api/playlists/{id}/checklist.
func (s *Server) handlePlaylistChecklist(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	playlist, err := s.store.Playlists.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	items, err := s.store.Playlists.ListItems(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	checks := []checklistItem{
		{Name: "Description", OK: strings.TrimSpace(playlist.Description) != "", Message: "Playlist has a description"},
		{Name: "Privacy", OK: playlist.Privacy != "", Message: "Privacy is recorded as " + playlist.Privacy},
		{Name: "Items", OK: len(items) > 0, Message: fmt.Sprintf("%d item(s)", len(items))},
	}
	if playlist.Privacy == "" {
		checks[1].Message = "Privacy unknown - sync the account to record it"
	}
	ordered := true
	lastEpisode, lastPart := -1, -1
	for _, item := range items {
		video, videoErr := s.store.Videos.GetByID(item.VideoID)
		if videoErr != nil {
			continue
		}
		parsed := episodes.ParseTitle(video.Title)
		if !parsed.HasEpisode {
			continue
		}
		if parsed.EpisodeNumber < lastEpisode || (parsed.EpisodeNumber == lastEpisode && parsed.PartNumber < lastPart) {
			ordered = false
		}
		lastEpisode, lastPart = parsed.EpisodeNumber, parsed.PartNumber
	}
	checks = append(checks, checklistItem{Name: "Order", OK: ordered, Message: "Items follow episode/part order"})
	writeOK(w, checks)
}

// tagCount is one tag's usage on a channel (item 140).
type tagCount struct {
	Tag      string `json:"tag"`
	Count    int    `json:"count"`
	VideoIDs []uint `json:"videoIds"`
}

// videoTags decodes a video's mirrored tag list.
func videoTags(video model.Video) []string {
	if video.Tags == "" {
		return nil
	}
	tags := []string{}
	_ = json.Unmarshal([]byte(video.Tags), &tags)
	return tags
}

// handleListTags serves GET /api/tags?channelId=: every tag on
// the channel with its usage count, from the mirror.
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	videos, err := s.store.Videos.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	byTag := map[string]*tagCount{}
	for _, video := range videos {
		for _, tag := range videoTags(video) {
			entry, ok := byTag[tag]
			if !ok {
				entry = &tagCount{Tag: tag}
				byTag[tag] = entry
			}
			entry.Count++
			entry.VideoIDs = append(entry.VideoIDs, video.ID)
		}
	}
	result := []tagCount{}
	for _, entry := range byTag {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	writeOK(w, result)
}

// tagRenameRequest is the body of POST /api/tags/rename.
type tagRenameRequest struct {
	ChannelID uint   `json:"channelId"`
	From      string `json:"from"`
	To        string `json:"to"` // empty To with merge=false removes the tag; merge into To when To is an existing tag
}

// handleRenameTag serves POST /api/tags/rename (item 140):
// rename (or merge, when the target already exists) a tag on
// every mirrored video of the channel that carries it, through
// the API, preview-style results per video. The mirror updates
// locally on success so the manager stays truthful without
// waiting for a sync.
func (s *Server) handleRenameTag(w http.ResponseWriter, r *http.Request) {
	var body tagRenameRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.From == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("from is required"))
		return
	}
	videos, err := s.store.Videos.ListByChannel(body.ChannelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	svc, err := s.serviceForChannel(r.Context(), body.ChannelID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	changed := 0
	failures := []string{}
	for _, video := range videos {
		tags := videoTags(video)
		found := false
		next := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tag == body.From {
				found = true
				if body.To != "" {
					next = append(next, body.To)
				}
				continue
			}
			next = append(next, tag)
		}
		if !found {
			continue
		}
		// De-duplicate (merge case).
		seen := map[string]bool{}
		unique := next[:0]
		for _, tag := range next {
			if !seen[tag] {
				seen[tag] = true
				unique = append(unique, tag)
			}
		}
		full, fetchErr := ytapi.FetchVideo(r.Context(), svc, video.YTVideoID)
		if fetchErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", video.Title, fetchErr))
			continue
		}
		if full.Snippet == nil {
			failures = append(failures, fmt.Sprintf("%s: no snippet", video.Title))
			continue
		}
		full.Snippet.Tags = unique
		if _, updateErr := svc.Videos.Update([]string{"snippet"}, full).Context(r.Context()).Do(); updateErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", video.Title, updateErr))
			continue
		}
		data, _ := json.Marshal(unique)
		_ = s.store.Videos.UpdateTags(video.ID, string(data))
		s.addQuota(50)
		changed++
	}
	if changed > 0 {
		s.recordActivity("rename", fmt.Sprintf("Tag %q renamed to %q on %d video(s)", body.From, body.To, changed), "")
	}
	writeOK(w, map[string]interface{}{"changed": changed, "failures": failures})
}

// handleInsightsDashboard serves GET /api/insights/dashboard.
func (s *Server) handleInsightsDashboard(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	dashboard, err := insights.DashboardFor(s.store, uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, dashboard)
}

// handleInsightsScorecards serves GET /api/insights/scorecards.
func (s *Server) handleInsightsScorecards(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	cards, meta, err := insights.ScorecardsFor(s.store, uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]interface{}{"meta": meta, "scorecards": cards})
}

// handleInsightsPerformers serves GET /api/insights/performers.
func (s *Server) handleInsightsPerformers(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	top, bottom, meta, err := insights.Performers(s.store, uint(channelID), r.URL.Query().Get("by"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]interface{}{"meta": meta, "top": top, "bottom": bottom})
}

// handleInsightsHeatmap serves GET /api/insights/heatmap.
func (s *Server) handleInsightsHeatmap(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	windowDays, _ := parseQueryInt(r, "windowDays")
	cells, meta, err := insights.Heatmap(s.store, uint(channelID), windowDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]interface{}{"meta": meta, "cells": cells})
}

// growthDTO is the growth snapshot history + diff (item 147).
type growthDTO struct {
	Snapshots []growthPoint `json:"snapshots"`
	Diff      string        `json:"diff"`
}

type growthPoint struct {
	At        string `json:"at"`
	Videos    int    `json:"videos"`
	Scheduled int    `json:"scheduled"`
	Views     int64  `json:"views"`
	Comments  int64  `json:"comments"`
}

// handleInsightsGrowth serves GET /api/insights/growth?accountId=.
func (s *Server) handleInsightsGrowth(w http.ResponseWriter, r *http.Request) {
	accountID, err := parseQueryInt(r, "accountId")
	if err != nil || accountID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("accountId is required"))
		return
	}
	points := []growthPoint{}
	s.getJSONSetting(fmt.Sprintf("growthsnap:%d", accountID), &points)
	diff := "Not enough snapshots yet - growth is recorded at each sync."
	if len(points) >= 2 {
		first, last := points[0], points[len(points)-1]
		diff = fmt.Sprintf("Since %s: %+d videos, %+d scheduled, %+d views, %+d comments",
			first.At, last.Videos-first.Videos, last.Scheduled-first.Scheduled, last.Views-first.Views, last.Comments-first.Comments)
	}
	writeOK(w, growthDTO{Snapshots: points, Diff: diff})
}

// handleInsightsExport serves GET /api/insights/export as CSV
// (item 148). Kinds: performers, scorecards.
func (s *Server) handleInsightsExport(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	kind := r.URL.Query().Get("kind")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=yt-manage-insights.csv")
	writer := csv.NewWriter(w)
	defer writer.Flush()
	switch kind {
	case "scorecards":
		cards, meta, err := insights.ScorecardsFor(s.store, uint(channelID))
		if err != nil {
			return
		}
		_ = writer.Write([]string{"# " + meta.Describe()})
		_ = writer.Write([]string{"series", "episode", "title", "views", "comments", "dropoff"})
		for _, card := range cards {
			for _, episode := range card.Episodes {
				_ = writer.Write([]string{card.SeriesKey, strconv.Itoa(episode.EpisodeNumber), episode.Title,
					strconv.FormatInt(episode.Views, 10), strconv.FormatInt(episode.Comments, 10), strconv.FormatBool(episode.Dropoff)})
			}
		}
	default:
		top, bottom, meta, err := insights.Performers(s.store, uint(channelID), r.URL.Query().Get("by"))
		if err != nil {
			return
		}
		_ = writer.Write([]string{"# " + meta.Describe()})
		_ = writer.Write([]string{"list", "video_id", "title", "views", "comments"})
		for _, performer := range top {
			_ = writer.Write([]string{"top", strconv.FormatUint(uint64(performer.VideoID), 10), performer.Title, strconv.FormatInt(performer.Views, 10), strconv.FormatInt(performer.Comments, 10)})
		}
		for _, performer := range bottom {
			_ = writer.Write([]string{"bottom", strconv.FormatUint(uint64(performer.VideoID), 10), performer.Title, strconv.FormatInt(performer.Views, 10), strconv.FormatInt(performer.Comments, 10)})
		}
	}
}

// goal is one user goal (item 149).
type goal struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Type       string `json:"type"` // coverage | count | scheduled-through
	ChannelID  uint   `json:"channelId"`
	Target     int    `json:"target"`     // percent or count; unused for scheduled-through
	TargetDate string `json:"targetDate"` // for scheduled-through
}

// goalDTO adds computed progress to a goal.
type goalDTO struct {
	goal
	Progress float64 `json:"progress"`
	Detail   string  `json:"detail"`
}

const goalsKey = "ui.goals"

func (s *Server) loadGoals() []goal {
	goals := []goal{}
	s.getJSONSetting(goalsKey, &goals)
	return goals
}

// goalProgress computes one goal's progress from the mirror.
func (s *Server) goalProgress(g goal) goalDTO {
	dto := goalDTO{goal: g}
	videos, err := s.store.Videos.ListByChannel(g.ChannelID)
	if err != nil {
		dto.Detail = "channel not found"
		return dto
	}
	switch g.Type {
	case "count":
		dto.Progress = percentOf(len(videos), g.Target)
		dto.Detail = fmt.Sprintf("%d of %d videos", len(videos), g.Target)
	case "coverage":
		firsts, _ := s.store.Comments.ListFirstComments(0)
		withFirst := map[uint]bool{}
		for _, first := range firsts {
			withFirst[first.VideoID] = true
		}
		public, covered := 0, 0
		for _, video := range videos {
			if video.Visibility == model.VisibilityPublic {
				public++
				if withFirst[video.ID] {
					covered++
				}
			}
		}
		pct := 0.0
		if public > 0 {
			pct = float64(covered) * 100 / float64(public)
		}
		dto.Progress = percentOf(int(pct+0.5), g.Target)
		dto.Detail = fmt.Sprintf("%.0f%% of public videos have a first comment (target %d%%)", pct, g.Target)
	case "scheduled-through":
		var latest *time.Time
		for _, video := range videos {
			if video.PublishAt != nil && (latest == nil || video.PublishAt.After(*latest)) {
				latest = video.PublishAt
			}
		}
		target, parseErr := time.Parse("2006-01-02", g.TargetDate)
		if latest == nil {
			dto.Detail = "Nothing is scheduled yet"
		} else if parseErr == nil && !latest.Before(target) {
			dto.Progress = 100
			dto.Detail = fmt.Sprintf("Scheduled through %s", latest.Format("2006-01-02"))
		} else {
			dto.Detail = fmt.Sprintf("Scheduled through %s; target %s", latest.Format("2006-01-02"), g.TargetDate)
			if parseErr == nil {
				total := target.Sub(time.Now().Truncate(24 * time.Hour))
				remaining := target.Sub(*latest)
				if total > 0 {
					dto.Progress = 100 - float64(remaining)*100/float64(total)
					if dto.Progress < 0 {
						dto.Progress = 0
					}
				}
			}
		}
	}
	return dto
}

func percentOf(value int, target int) float64 {
	if target <= 0 {
		return 0
	}
	pct := float64(value) * 100 / float64(target)
	if pct > 100 {
		return 100
	}
	return float64(int(pct*10)) / 10
}

// handleListGoals serves GET /api/goals.
func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	dtos := []goalDTO{}
	for _, g := range s.loadGoals() {
		dtos = append(dtos, s.goalProgress(g))
	}
	writeOK(w, dtos)
}

// handleSaveGoal serves POST /api/goals.
func (s *Server) handleSaveGoal(w http.ResponseWriter, r *http.Request) {
	var body goal
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Text == "" || body.ChannelID == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("a goal needs text and a channel"))
		return
	}
	goals := s.loadGoals()
	if body.ID == "" {
		body.ID = fmt.Sprintf("g%d", time.Now().UnixNano())
		goals = append(goals, body)
	} else {
		replaced := false
		for i := range goals {
			if goals[i].ID == body.ID {
				goals[i] = body
				replaced = true
			}
		}
		if !replaced {
			goals = append(goals, body)
		}
	}
	if err := s.setJSONSetting(goalsKey, goals); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, s.goalProgress(body))
}

// handleDeleteGoal serves DELETE /api/goals/{id}.
func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	goals := s.loadGoals()
	kept := goals[:0]
	for _, g := range goals {
		if g.ID != id {
			kept = append(kept, g)
		}
	}
	if err := s.setJSONSetting(goalsKey, kept); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, map[string]bool{"deleted": true})
}

// abTestResultsDTO is the visualised A/B result (item 143).
type abTestResultsDTO struct {
	Test       abTestDTO       `json:"test"`
	Video      videoDTO        `json:"video"`
	Snapshots  []snapshotDTO   `json:"snapshots"`
	FirstViews int64           `json:"firstSnapshotViews"`
	LastViews  int64           `json:"lastSnapshotViews"`
	DeltaViews int64           `json:"deltaViews"`
	Schedule   *abTestSchedule `json:"schedule,omitempty"`
}

// abTestSchedule is the auto-schedule of a test (item 144).
type abTestSchedule struct {
	SwitchAt    string `json:"switchAt"`
	SnapshotAt  string `json:"snapshotAt"`
	EndAt       string `json:"endAt"`
	Switched    bool   `json:"switched"`
	Snapshotted bool   `json:"snapshotted"`
	Ended       bool   `json:"ended"`
}

func abScheduleKey(testID uint) string {
	return fmt.Sprintf("abtest:%d:schedule", testID)
}

func (s *Server) loadABSchedule(testID uint) *abTestSchedule {
	var schedule abTestSchedule
	raw, found, err := s.store.Settings.Get(abScheduleKey(testID))
	if err != nil || !found {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &schedule); err != nil {
		return nil
	}
	return &schedule
}

// handleABTestResults serves GET /api/ab-tests/{id}/results.
func (s *Server) handleABTestResults(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	test, err := s.store.ABTests.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	dto, err := s.abTestWithSnapshots(*test)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	video, err := s.store.Videos.GetByID(test.VideoID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	result := abTestResultsDTO{Test: dto, Video: toVideoDTO(*video), Snapshots: dto.Snapshots, Schedule: s.loadABSchedule(id)}
	if len(dto.Snapshots) > 0 {
		first, _ := strconv.ParseInt(dto.Snapshots[0].ViewsAContext, 10, 64)
		last, _ := strconv.ParseInt(dto.Snapshots[len(dto.Snapshots)-1].ViewsAContext, 10, 64)
		result.FirstViews = first
		result.LastViews = last
		result.DeltaViews = last - first
	}
	writeOK(w, result)
}

// handleSetABSchedule serves POST /api/ab-tests/{id}/schedule.
func (s *Server) handleSetABSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var body abTestSchedule
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, value := range []string{body.SwitchAt, body.SnapshotAt, body.EndAt} {
		if value == "" {
			continue
		}
		if _, parseErr := time.Parse(time.RFC3339, value); parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("dates must be RFC3339 timestamps"))
			return
		}
	}
	if err := s.setJSONSetting(abScheduleKey(id), body); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, body)
}

// runABTestSchedules executes due A/B schedule steps (item
// 144): switch the live title to variant B, take the mid-way
// snapshot, and close the test at the end date with a final
// snapshot. Called from the automation tick.
func (s *Server) runABTestSchedules(ctx context.Context) {
	tests, err := s.store.ABTests.ListActive()
	if err != nil {
		return
	}
	now := time.Now()
	for _, test := range tests {
		schedule := s.loadABSchedule(test.ID)
		if schedule == nil {
			continue
		}
		video, videoErr := s.store.Videos.GetByID(test.VideoID)
		if videoErr != nil {
			continue
		}
		changed := false
		due := func(value string, done bool) bool {
			if value == "" || done {
				return false
			}
			parsed, parseErr := time.Parse(time.RFC3339, value)
			return parseErr == nil && !parsed.After(now)
		}
		if due(schedule.SwitchAt, schedule.Switched) {
			svc, svcErr := s.serviceForChannel(ctx, video.ChannelID)
			if svcErr == nil {
				plan := singleChangePlan(video, &test.VariantB, nil)
				if _, applyErr := batch.Apply(ctx, svc, s.store, plan); applyErr == nil {
					schedule.Switched = true
					changed = true
					s.recordActivity("automation", fmt.Sprintf("A/B test %d: switched title to variant B", test.ID), "")
				}
			}
		}
		if due(schedule.SnapshotAt, schedule.Snapshotted) {
			_ = s.store.ABTests.AddSnapshot(&model.ABTestSnapshot{
				TestID: test.ID, CapturedAt: now,
				ViewsAContext: strconv.FormatInt(video.ViewCount, 10), CTRNote: "auto mid-way snapshot",
			})
			schedule.Snapshotted = true
			changed = true
		}
		if due(schedule.EndAt, schedule.Ended) {
			_ = s.store.ABTests.AddSnapshot(&model.ABTestSnapshot{
				TestID: test.ID, CapturedAt: now,
				ViewsAContext: strconv.FormatInt(video.ViewCount, 10), CTRNote: "auto final snapshot",
			})
			test.IsActive = false
			_ = s.store.ABTests.Update(&test)
			schedule.Ended = true
			changed = true
			s.recordActivity("automation", fmt.Sprintf("A/B test %d ended on schedule - pick a winner from the results view", test.ID), "")
		}
		if changed {
			_ = s.setJSONSetting(abScheduleKey(test.ID), schedule)
		}
	}
}
