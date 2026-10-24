// Package suggest computes the smart suggestions of spec 29
// (items 101-106, 109) from the local mirror only: no API calls,
// no writes. Every suggestion names its type, channel, the videos
// it concerns, and a plain-language detail.
package suggest

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// Suggestion is one computed suggestion.
type Suggestion struct {
	Type      string            `json:"type"`
	ChannelID uint              `json:"channelId"`
	Title     string            `json:"title"`
	Detail    string            `json:"detail"`
	VideoIDs  []uint            `json:"videoIds"`
	Data      map[string]string `json:"data,omitempty"`
}

// ForChannel computes every suggestion type for one channel.
func ForChannel(store *repo.Store, channelID uint) ([]Suggestion, error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	eps, err := store.Episodes.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	groupedVideoIDs := map[uint]bool{}
	episodeKeys := map[string]bool{}
	for _, episode := range eps {
		episodeKeys[episode.EpisodeKey] = true
		parts, partErr := store.Episodes.ListParts(episode.ID)
		if partErr != nil {
			return nil, partErr
		}
		for _, part := range parts {
			groupedVideoIDs[part.VideoID] = true
		}
	}
	suggestions := []Suggestion{}
	suggestions = append(suggestions, patternIssues(channelID, videos)...)
	suggestions = append(suggestions, groupingHints(channelID, videos, groupedVideoIDs, episodeKeys)...)
	suggestions = append(suggestions, titleHealth(channelID, videos)...)
	if cadence, ok := cadenceHint(channelID, videos); ok {
		suggestions = append(suggestions, cadence)
	}
	suggestions = append(suggestions, deadSlots(channelID, videos)...)
	suggestions = append(suggestions, thumbnailReview(channelID, videos)...)
	return suggestions, nil
}

// All computes suggestions for every channel.
func All(store *repo.Store) ([]Suggestion, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, err
	}
	all := []Suggestion{}
	for _, channel := range channels {
		channelSuggestions, err := ForChannel(store, channel.ID)
		if err != nil {
			return nil, err
		}
		all = append(all, channelSuggestions...)
	}
	return all, nil
}

// patternIssues (item 101) finds series whose episode numbering
// has gaps or duplicates.
func patternIssues(channelID uint, videos []model.Video) []Suggestion {
	bySeries := map[string][]model.Video{}
	for _, video := range videos {
		parsed := episodes.ParseTitle(video.Title)
		if parsed.HasEpisode && parsed.SeriesKey != "" {
			bySeries[parsed.SeriesKey] = append(bySeries[parsed.SeriesKey], video)
		}
	}
	suggestions := []Suggestion{}
	for seriesKey, members := range bySeries {
		if len(members) < 2 {
			continue
		}
		seen := map[int]int{}
		max := 0
		ids := []uint{}
		for _, video := range members {
			parsed := episodes.ParseTitle(video.Title)
			seen[parsed.EpisodeNumber]++
			if parsed.EpisodeNumber > max {
				max = parsed.EpisodeNumber
			}
			ids = append(ids, video.ID)
		}
		problems := []string{}
		duplicates := []int{}
		for number, count := range seen {
			if count > 1 {
				duplicates = append(duplicates, number)
			}
		}
		sort.Ints(duplicates)
		for _, number := range duplicates {
			problems = append(problems, fmt.Sprintf("episode %d appears %d times", number, seen[number]))
		}
		missing := []int{}
		for number := 1; number <= max; number++ {
			if seen[number] == 0 {
				missing = append(missing, number)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("missing episode(s) %s", joinInts(missing)))
		}
		if len(problems) == 0 {
			continue
		}
		suggestions = append(suggestions, Suggestion{
			Type: "patterns", ChannelID: channelID,
			Title:    fmt.Sprintf("Series %q has numbering issues", seriesKey),
			Detail:   strings.Join(problems, "; "),
			VideoIDs: ids,
		})
	}
	return suggestions
}

// groupingHints (item 102) lists ungrouped videos whose show
// matches the show of an existing episode on the channel.
func groupingHints(channelID uint, videos []model.Video, grouped map[uint]bool, episodeKeys map[string]bool) []Suggestion {
	episodeShows := map[string]bool{}
	for key := range episodeKeys {
		episodeShows[episodes.ShowKey(episodes.EpisodeBaseKey(key))] = true
	}
	byKey := map[string][]uint{}
	for _, video := range videos {
		if grouped[video.ID] {
			continue
		}
		parsed := episodes.ParseTitle(video.Title)
		if parsed.SeriesKey == "" || !parsed.HasEpisode {
			continue
		}
		show := episodes.ShowKey(parsed.SeriesKey)
		if episodeShows[show] {
			byKey[show] = append(byKey[show], video.ID)
		}
	}
	suggestions := []Suggestion{}
	for key, ids := range byKey {
		suggestions = append(suggestions, Suggestion{
			Type: "grouping", ChannelID: channelID,
			Title:    fmt.Sprintf("%d ungrouped video(s) look like series %q", len(ids), key),
			Detail:   "They match an existing episode pattern but are not grouped yet. Run episode grouping to add them.",
			VideoIDs: ids,
		})
	}
	return suggestions
}

// titleHealth (item 103) lists duplicate titles, ALL-CAPS titles,
// and over-length titles for the channel.
func titleHealth(channelID uint, videos []model.Video) []Suggestion {
	suggestions := []Suggestion{}
	byTitle := map[string][]uint{}
	caps := []uint{}
	long := []uint{}
	for _, video := range videos {
		byTitle[video.Title] = append(byTitle[video.Title], video.ID)
		if isAllCaps(video.Title) {
			caps = append(caps, video.ID)
		}
		if len([]rune(video.Title)) > 90 {
			long = append(long, video.ID)
		}
	}
	for title, ids := range byTitle {
		if len(ids) > 1 {
			suggestions = append(suggestions, Suggestion{
				Type: "title-health", ChannelID: channelID,
				Title:    fmt.Sprintf("Duplicate title used %d times", len(ids)),
				Detail:   fmt.Sprintf("%q is the title of %d videos.", title, len(ids)),
				VideoIDs: ids,
			})
		}
	}
	if len(caps) > 0 {
		suggestions = append(suggestions, Suggestion{
			Type: "title-health", ChannelID: channelID,
			Title:    fmt.Sprintf("%d title(s) are ALL CAPS", len(caps)),
			Detail:   "ALL-CAPS titles read as shouting and often underperform; consider normal casing.",
			VideoIDs: caps,
		})
	}
	if len(long) > 0 {
		suggestions = append(suggestions, Suggestion{
			Type: "title-health", ChannelID: channelID,
			Title:    fmt.Sprintf("%d title(s) are over 90 characters", len(long)),
			Detail:   "Long titles are truncated in search results and suggested feeds.",
			VideoIDs: long,
		})
	}
	return suggestions
}

// cadenceHint (item 104) derives the channel's real publish rhythm
// from its published videos.
func cadenceHint(channelID uint, videos []model.Video) (Suggestion, bool) {
	published := []time.Time{}
	for _, video := range videos {
		if video.PublishedAt != nil && video.Visibility == model.VisibilityPublic {
			published = append(published, *video.PublishedAt)
		}
	}
	if len(published) < 4 {
		return Suggestion{}, false
	}
	sort.Slice(published, func(i, j int) bool { return published[i].Before(published[j]) })
	gaps := []float64{}
	for i := 1; i < len(published); i++ {
		gaps = append(gaps, published[i].Sub(published[i-1]).Hours()/24)
	}
	sort.Float64s(gaps)
	median := gaps[len(gaps)/2]
	days := int(median + 0.5)
	if days < 1 {
		days = 1
	}
	return Suggestion{
		Type: "cadence", ChannelID: channelID,
		Title:  fmt.Sprintf("This channel really publishes about every %d day(s)", days),
		Detail: fmt.Sprintf("Median gap across the last %d published videos is %.1f days. A cadence that matches the real rhythm is easier to keep.", len(published), median),
		Data:   map[string]string{"cadenceDays": fmt.Sprintf("%d", days)},
	}, true
}

// deadSlots (item 105) flags scheduled videos whose publish
// weekday historically underperforms on this channel. A hint,
// never a block.
func deadSlots(channelID uint, videos []model.Video) []Suggestion {
	viewsByWeekday := map[time.Weekday][]int64{}
	var totalViews int64
	var totalCount int
	scheduled := []model.Video{}
	for _, video := range videos {
		if video.PublishedAt != nil && video.Visibility == model.VisibilityPublic {
			viewsByWeekday[video.PublishedAt.Weekday()] = append(viewsByWeekday[video.PublishedAt.Weekday()], video.ViewCount)
			totalViews += video.ViewCount
			totalCount++
		}
		if video.Visibility == model.VisibilityScheduled && video.PublishAt != nil {
			scheduled = append(scheduled, video)
		}
	}
	if totalCount < 5 || len(scheduled) == 0 {
		return nil
	}
	overallAvg := float64(totalViews) / float64(totalCount)
	flagged := []uint{}
	for _, video := range scheduled {
		dayViews := viewsByWeekday[video.PublishAt.Weekday()]
		if len(dayViews) < 2 {
			continue
		}
		var sum int64
		for _, views := range dayViews {
			sum += views
		}
		if float64(sum)/float64(len(dayViews)) < 0.6*overallAvg {
			flagged = append(flagged, video.ID)
		}
	}
	if len(flagged) == 0 {
		return nil
	}
	return []Suggestion{{
		Type: "dead-slots", ChannelID: channelID,
		Title:    fmt.Sprintf("%d scheduled video(s) land on historically weak days", len(flagged)),
		Detail:   "Videos published on those weekdays average under 60% of this channel's usual views. A hint only - the schedule is unchanged.",
		VideoIDs: flagged,
	}}
}

// thumbnailReview (item 106) lists part-numbered videos so the
// user can visually check the thumbnail shows the same part
// number the title promises. Thumbnail pixels are not machine-
// readable locally, so the check pairs title and thumbnail side
// by side for a human eye.
func thumbnailReview(channelID uint, videos []model.Video) []Suggestion {
	ids := []uint{}
	for _, video := range videos {
		parsed := episodes.ParseTitle(video.Title)
		if parsed.HasPart && parsed.PartNumber >= 1 {
			ids = append(ids, video.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return []Suggestion{{
		Type: "thumbnails", ChannelID: channelID,
		Title:    fmt.Sprintf("%d video(s) promise a part number in the title", len(ids)),
		Detail:   "Check each thumbnail shows the same part number. yt-manage cannot read thumbnail pixels, so this pairs titles with thumbnails for your eye.",
		VideoIDs: ids,
	}}
}

func isAllCaps(title string) bool {
	letters := 0
	for _, r := range title {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return letters >= 8
}

func joinInts(numbers []int) string {
	parts := make([]string, len(numbers))
	for i, number := range numbers {
		parts[i] = fmt.Sprintf("%d", number)
	}
	return strings.Join(parts, ", ")
}
