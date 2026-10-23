package episodes

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// PartAssignment is one video placed inside an episode group, with
// its part number and its 1-based sequence within the episode.
type PartAssignment struct {
	Video      model.Video
	PartNumber int
	Sequence   int
}

// EpisodeGroup is one episode: every parsed part of one
// (channel, series, episode number) triple, sequenced, with
// warnings for missing or duplicated part numbers.
type EpisodeGroup struct {
	ChannelID     uint
	SeriesKey     string
	EpisodeNumber int
	EpisodeKey    string
	Title         string
	Parts         []PartAssignment
	Warnings      []string
}

// GroupResult is the outcome of grouping one channel: the episode
// groups plus the videos whose titles carried no episode number.
// Unparsed videos are reported, never silently dropped.
type GroupResult struct {
	Groups   []EpisodeGroup
	Unparsed []model.Video
}

// groupKey identifies one episode group.
type groupKey struct {
	channelID     uint
	seriesKey     string
	episodeNumber int
}

// GroupVideos groups parseable videos by (channel, series key,
// episode number). Parts are ordered by part number, then by
// publish time (videos without one sort last), then by video ID,
// and sequenced 1..n in that order. Groups are sorted by series
// key, then episode number. Videos with no episode number are
// returned as unparsed.
func GroupVideos(videos []model.Video) (groups []EpisodeGroup, unparsed []model.Video) {
	byKey := make(map[groupKey]*EpisodeGroup)
	for _, video := range videos {
		parsed := ParseTitle(video.Title)
		if !parsed.HasEpisode {
			unparsed = append(unparsed, video)
			continue
		}
		key := groupKey{channelID: video.ChannelID, seriesKey: parsed.SeriesKey, episodeNumber: parsed.EpisodeNumber}
		group, hasGroup := byKey[key]
		if !hasGroup {
			group = &EpisodeGroup{
				ChannelID:     video.ChannelID,
				SeriesKey:     parsed.SeriesKey,
				EpisodeNumber: parsed.EpisodeNumber,
				EpisodeKey:    parsed.SeriesKey + "|ep" + strconv.Itoa(parsed.EpisodeNumber),
			}
			byKey[key] = group
		}
		group.Parts = append(group.Parts, PartAssignment{Video: video, PartNumber: parsed.PartNumber})
	}
	groups = make([]EpisodeGroup, 0, len(byKey))
	for _, group := range byKey {
		sortParts(group.Parts)
		for i := range group.Parts {
			group.Parts[i].Sequence = i + 1
		}
		group.Warnings = partWarnings(group.Parts)
		group.Title = groupDisplayTitle(group.Parts)
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].SeriesKey != groups[j].SeriesKey {
			return groups[i].SeriesKey < groups[j].SeriesKey
		}
		if groups[i].EpisodeNumber != groups[j].EpisodeNumber {
			return groups[i].EpisodeNumber < groups[j].EpisodeNumber
		}
		return groups[i].ChannelID < groups[j].ChannelID
	})
	return groups, unparsed
}

// sortParts orders parts by part number, then by publish time with
// nil publish times last, then by video ID.
func sortParts(parts []PartAssignment) {
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].PartNumber != parts[j].PartNumber {
			return parts[i].PartNumber < parts[j].PartNumber
		}
		publishedI := parts[i].Video.PublishedAt
		publishedJ := parts[j].Video.PublishedAt
		if publishedI == nil && publishedJ != nil {
			return false
		}
		if publishedI != nil && publishedJ == nil {
			return true
		}
		if publishedI != nil && publishedJ != nil && !publishedI.Equal(*publishedJ) {
			return publishedI.Before(*publishedJ)
		}
		return parts[i].Video.ID < parts[j].Video.ID
	})
}

// partWarnings reports the part numbers missing between the lowest
// and highest present part, and every duplicated part number.
func partWarnings(parts []PartAssignment) []string {
	if len(parts) == 0 {
		return nil
	}
	counts := make(map[int]int)
	minPart := parts[0].PartNumber
	maxPart := parts[0].PartNumber
	for _, part := range parts {
		counts[part.PartNumber]++
		if part.PartNumber < minPart {
			minPart = part.PartNumber
		}
		if part.PartNumber > maxPart {
			maxPart = part.PartNumber
		}
	}
	var warnings []string
	for number := minPart + 1; number < maxPart; number++ {
		if counts[number] == 0 {
			warnings = append(warnings, fmt.Sprintf("missing part %d", number))
		}
	}
	numbers := make([]int, 0, len(counts))
	for number := range counts {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	for _, number := range numbers {
		if counts[number] > 1 {
			warnings = append(warnings, fmt.Sprintf("duplicate part %d", number))
		}
	}
	return warnings
}

// groupDisplayTitle derives the episode's display title from the
// first part's original title (parts are already sequenced, so the
// first part is the part-1 video when one exists).
func groupDisplayTitle(parts []PartAssignment) string {
	if len(parts) == 0 {
		return ""
	}
	return displayTitle(parts[0].Video.Title)
}

// GroupChannel groups every mirrored video of a channel and
// materialises the groups into episodes and episode_parts,
// idempotently by episode key: re-running it replaces each
// episode's parts with the freshly parsed set.
func GroupChannel(store *repo.Store, channelID uint) (*GroupResult, error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	groups, unparsed := GroupVideos(videos)
	for _, group := range groups {
		episode := &model.Episode{
			ChannelID:     group.ChannelID,
			EpisodeKey:    group.EpisodeKey,
			EpisodeNumber: group.EpisodeNumber,
			Title:         group.Title,
		}
		if err := store.Episodes.UpsertByKey(episode); err != nil {
			return nil, err
		}
		stored, err := store.Episodes.GetByKey(group.ChannelID, group.EpisodeKey)
		if err != nil {
			return nil, err
		}
		parts := make([]model.EpisodePart, 0, len(group.Parts))
		for _, part := range group.Parts {
			parts = append(parts, model.EpisodePart{
				EpisodeID:  stored.ID,
				VideoID:    part.Video.ID,
				PartNumber: part.PartNumber,
				Sequence:   part.Sequence,
			})
		}
		if err := store.Episodes.ReplaceParts(stored.ID, parts); err != nil {
			return nil, err
		}
	}
	return &GroupResult{Groups: groups, Unparsed: unparsed}, nil
}
