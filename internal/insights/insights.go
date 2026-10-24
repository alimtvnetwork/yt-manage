// Package insights computes the analytics of spec 29 (items
// 141, 142, 145, 146) from the local mirror only. Every result
// carries its source and window so no number is a mystery
// (item 150).
package insights

import (
	"fmt"
	"sort"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// Meta describes where the numbers came from.
type Meta struct {
	Source      string `json:"source"`
	GeneratedAt string `json:"generatedAt"`
	WindowDays  int    `json:"windowDays"`
}

func meta(windowDays int) Meta {
	return Meta{Source: "local mirror (synced data only, no live API call)", GeneratedAt: time.Now().Format(time.RFC3339), WindowDays: windowDays}
}

// MonthCount is one uploads-by-month bucket.
type MonthCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

// SeriesViews is one show's totals.
type SeriesViews struct {
	SeriesKey string `json:"seriesKey"`
	Videos    int    `json:"videos"`
	Views     int64  `json:"views"`
	Comments  int64  `json:"comments"`
}

// Dashboard is the channel dashboard payload (item 141).
type Dashboard struct {
	Meta               Meta          `json:"meta"`
	ChannelID          uint          `json:"channelId"`
	TotalVideos        int           `json:"totalVideos"`
	TotalViews         int64         `json:"totalViews"`
	TotalComments      int64         `json:"totalComments"`
	UploadsByMonth     []MonthCount  `json:"uploadsByMonth"`
	ViewsBySeries      []SeriesViews `json:"viewsBySeries"`
	CommentCoveragePct float64       `json:"commentCoveragePct"`
	ScheduledTotal     int           `json:"scheduledTotal"`
	ScheduledPublished int           `json:"scheduledPublished"`
	AdherencePct       float64       `json:"scheduleAdherencePct"`
}

// DashboardFor builds the dashboard for one channel.
func DashboardFor(store *repo.Store, channelID uint) (*Dashboard, error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, err
	}
	dashboard := &Dashboard{Meta: meta(0), ChannelID: channelID, UploadsByMonth: []MonthCount{}, ViewsBySeries: []SeriesViews{}}
	months := map[string]int{}
	series := map[string]*SeriesViews{}
	public := 0
	withFirst := 0
	for _, video := range videos {
		dashboard.TotalVideos++
		dashboard.TotalViews += video.ViewCount
		dashboard.TotalComments += video.CommentCount
		if video.PublishedAt != nil {
			month := video.PublishedAt.Format("2006-01")
			months[month]++
		}
		parsed := episodes.ParseTitle(video.Title)
		if parsed.SeriesKey != "" {
			key := episodes.ShowKey(parsed.SeriesKey)
			entry, ok := series[key]
			if !ok {
				entry = &SeriesViews{SeriesKey: key}
				series[key] = entry
			}
			entry.Videos++
			entry.Views += video.ViewCount
			entry.Comments += video.CommentCount
		}
		if video.Visibility == model.VisibilityPublic {
			public++
		}
		if video.Visibility == model.VisibilityScheduled {
			dashboard.ScheduledTotal++
		}
		if video.PublishAt != nil && video.PublishAt.Before(time.Now()) {
			dashboard.ScheduledTotal++
			if video.Visibility == model.VisibilityPublic {
				dashboard.ScheduledPublished++
			}
		}
	}
	firsts, err := store.Comments.ListFirstComments(0)
	if err != nil {
		return nil, err
	}
	firstVideoIDs := map[uint]bool{}
	for _, first := range firsts {
		firstVideoIDs[first.VideoID] = true
	}
	for _, video := range videos {
		if video.Visibility == model.VisibilityPublic && firstVideoIDs[video.ID] {
			withFirst++
		}
	}
	if public > 0 {
		dashboard.CommentCoveragePct = round1(float64(withFirst) * 100 / float64(public))
	}
	if dashboard.ScheduledTotal > 0 {
		dashboard.AdherencePct = round1(float64(dashboard.ScheduledPublished) * 100 / float64(dashboard.ScheduledTotal))
	}
	monthKeys := make([]string, 0, len(months))
	for month := range months {
		monthKeys = append(monthKeys, month)
	}
	sort.Strings(monthKeys)
	for _, month := range monthKeys {
		dashboard.UploadsByMonth = append(dashboard.UploadsByMonth, MonthCount{Month: month, Count: months[month]})
	}
	for _, entry := range series {
		dashboard.ViewsBySeries = append(dashboard.ViewsBySeries, *entry)
	}
	sort.Slice(dashboard.ViewsBySeries, func(i, j int) bool { return dashboard.ViewsBySeries[i].Views > dashboard.ViewsBySeries[j].Views })
	return dashboard, nil
}

// EpisodeScore is one episode's numbers in a scorecard.
type EpisodeScore struct {
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
	VideoIDs      []uint `json:"videoIds"`
	Views         int64  `json:"views"`
	Comments      int64  `json:"comments"`
	Dropoff       bool   `json:"dropoff"`
}

// Scorecard is one series' per-episode table (item 142).
type Scorecard struct {
	SeriesKey string         `json:"seriesKey"`
	Episodes  []EpisodeScore `json:"episodes"`
}

// ScorecardsFor builds per-series scorecards for a channel.
func ScorecardsFor(store *repo.Store, channelID uint) ([]Scorecard, Meta, error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, Meta{}, err
	}
	bySeries := map[string]map[int]*EpisodeScore{}
	for _, video := range videos {
		parsed := episodes.ParseTitle(video.Title)
		if !parsed.HasEpisode || parsed.SeriesKey == "" {
			continue
		}
		show := episodes.ShowKey(parsed.SeriesKey)
		if bySeries[show] == nil {
			bySeries[show] = map[int]*EpisodeScore{}
		}
		entry, ok := bySeries[show][parsed.EpisodeNumber]
		if !ok {
			entry = &EpisodeScore{EpisodeNumber: parsed.EpisodeNumber, Title: video.Title}
			bySeries[show][parsed.EpisodeNumber] = entry
		}
		entry.VideoIDs = append(entry.VideoIDs, video.ID)
		entry.Views += video.ViewCount
		entry.Comments += video.CommentCount
	}
	cards := []Scorecard{}
	for show, byEpisode := range bySeries {
		card := Scorecard{SeriesKey: show}
		numbers := make([]int, 0, len(byEpisode))
		for number := range byEpisode {
			numbers = append(numbers, number)
		}
		sort.Ints(numbers)
		var previousViews int64 = -1
		for _, number := range numbers {
			entry := byEpisode[number]
			if previousViews > 0 && entry.Views < previousViews*3/5 {
				entry.Dropoff = true
			}
			if entry.Views > 0 {
				previousViews = entry.Views
			}
			card.Episodes = append(card.Episodes, *entry)
		}
		if len(card.Episodes) >= 2 {
			cards = append(cards, card)
		}
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].SeriesKey < cards[j].SeriesKey })
	return cards, meta(0), nil
}

// Performer is one ranked video.
type Performer struct {
	VideoID  uint   `json:"videoId"`
	Title    string `json:"title"`
	Views    int64  `json:"views"`
	Comments int64  `json:"comments"`
}

// Performers returns the top and bottom public videos of a
// channel by views or comments (item 145).
func Performers(store *repo.Store, channelID uint, by string) (top []Performer, bottom []Performer, resultMeta Meta, err error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, nil, Meta{}, err
	}
	public := []model.Video{}
	for _, video := range videos {
		if video.Visibility == model.VisibilityPublic {
			public = append(public, video)
		}
	}
	value := func(video model.Video) int64 {
		if by == "comments" {
			return video.CommentCount
		}
		return video.ViewCount
	}
	sort.Slice(public, func(i, j int) bool { return value(public[i]) > value(public[j]) })
	convert := func(list []model.Video) []Performer {
		out := []Performer{}
		for _, video := range list {
			out = append(out, Performer{VideoID: video.ID, Title: video.Title, Views: video.ViewCount, Comments: video.CommentCount})
		}
		return out
	}
	limit := 10
	if len(public) < limit {
		limit = len(public)
	}
	top = convert(public[:limit])
	bottomList := []model.Video{}
	for i := len(public) - 1; i >= 0 && len(bottomList) < limit; i-- {
		bottomList = append(bottomList, public[i])
	}
	bottom = convert(bottomList)
	return top, bottom, meta(0), nil
}

// HeatCell is one weekday/hour bucket of the publish heatmap.
type HeatCell struct {
	Weekday  int     `json:"weekday"` // 0 = Sunday
	Hour     int     `json:"hour"`
	Count    int     `json:"count"`
	AvgViews float64 `json:"avgViews"`
}

// Heatmap builds the publish-time heatmap (item 146) from the
// channel's published videos of the last windowDays days.
func Heatmap(store *repo.Store, channelID uint, windowDays int) ([]HeatCell, Meta, error) {
	videos, err := store.Videos.ListByChannel(channelID)
	if err != nil {
		return nil, Meta{}, err
	}
	if windowDays <= 0 {
		windowDays = 365
	}
	cutoff := time.Now().AddDate(0, 0, -windowDays)
	type bucket struct {
		count int
		views int64
	}
	buckets := map[[2]int]*bucket{}
	for _, video := range videos {
		if video.PublishedAt == nil || video.PublishedAt.Before(cutoff) {
			continue
		}
		key := [2]int{int(video.PublishedAt.Weekday()), video.PublishedAt.Hour()}
		entry, ok := buckets[key]
		if !ok {
			entry = &bucket{}
			buckets[key] = entry
		}
		entry.count++
		entry.views += video.ViewCount
	}
	cells := []HeatCell{}
	for key, entry := range buckets {
		cells = append(cells, HeatCell{
			Weekday: key[0], Hour: key[1], Count: entry.count,
			AvgViews: round1(float64(entry.views) / float64(entry.count)),
		})
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Weekday != cells[j].Weekday {
			return cells[i].Weekday < cells[j].Weekday
		}
		return cells[i].Hour < cells[j].Hour
	})
	return cells, meta(windowDays), nil
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

// Describe formats a meta block for CSV export headers.
func (m Meta) Describe() string {
	return fmt.Sprintf("source: %s; generated: %s; window: %d days", m.Source, m.GeneratedAt, m.WindowDays)
}
