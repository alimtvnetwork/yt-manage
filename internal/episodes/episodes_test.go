package episodes_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/db"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

func TestParseTitle(t *testing.T) {
	cases := []struct {
		title      string
		episode    int
		hasEpisode bool
		part       int
		hasPart    bool
		seriesKey  string
	}{
		{"Robo School EP3", 3, true, 1, false, "robo school"},
		{"Robo School EP 03 - Part 2", 3, true, 2, true, "robo school"},
		{"Robo School ep03 part three", 3, true, 3, true, "robo school"},
		{"Robo School - Episode 3 (Part 03)", 3, true, 3, true, "robo school"},
		{"EP4", 4, true, 1, false, ""},
		{"EP 04 - Part 2", 4, true, 2, true, ""},
		{"Robo School EP5 (2/3)", 5, true, 2, true, "robo school"},
		{"Robo School EP10 Part Ten", 10, true, 10, true, "robo school"},
		{"Random Vlog Day", 0, false, 0, false, "random vlog day"},
	}
	for _, tc := range cases {
		got := episodes.ParseTitle(tc.title)
		if got.EpisodeNumber != tc.episode || got.HasEpisode != tc.hasEpisode ||
			got.PartNumber != tc.part || got.HasPart != tc.hasPart ||
			got.SeriesKey != tc.seriesKey {
			t.Errorf("ParseTitle(%q) = {episode:%d hasEpisode:%v part:%d hasPart:%v series:%q}, want {episode:%d hasEpisode:%v part:%d hasPart:%v series:%q}",
				tc.title,
				got.EpisodeNumber, got.HasEpisode, got.PartNumber, got.HasPart, got.SeriesKey,
				tc.episode, tc.hasEpisode, tc.part, tc.hasPart, tc.seriesKey)
		}
	}
}

func TestParseTitleZeroPaddingEquivalence(t *testing.T) {
	padded := episodes.ParseTitle("Show EP03")
	plain := episodes.ParseTitle("Show EP3")
	if padded.EpisodeNumber != plain.EpisodeNumber {
		t.Errorf("episode numbers differ: EP03 -> %d, EP3 -> %d", padded.EpisodeNumber, plain.EpisodeNumber)
	}
	if padded.SeriesKey != plain.SeriesKey {
		t.Errorf("series keys differ: EP03 -> %q, EP3 -> %q", padded.SeriesKey, plain.SeriesKey)
	}
}

// specTitles are the worked-example titles of spec 03 §6: episode 3
// with three parts and episode 4 with two parts (five titles).
var specTitles = []string{
	"EP3",
	"EP 03 Part 2",
	"ep03 part three",
	"EP4",
	"EP 04 - Part 2",
}

func specVideos() []model.Video {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	videos := make([]model.Video, 0, len(specTitles))
	for i, title := range specTitles {
		published := base.Add(time.Duration(i) * time.Hour)
		videos = append(videos, model.Video{
			ID:          uint(i + 1),
			ChannelID:   1,
			YTVideoID:   fmt.Sprintf("yt-%d", i+1),
			Title:       title,
			PublishedAt: &published,
		})
	}
	return videos
}

func findGroup(t *testing.T, groups []episodes.EpisodeGroup, episodeNumber int) episodes.EpisodeGroup {
	t.Helper()
	for _, group := range groups {
		if group.EpisodeNumber == episodeNumber {
			return group
		}
	}
	t.Fatalf("no group for episode %d in %d groups", episodeNumber, len(groups))
	return episodes.EpisodeGroup{}
}

func TestGroupVideosSpecExample(t *testing.T) {
	groups, unparsed := episodes.GroupVideos(specVideos())
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if len(unparsed) != 0 {
		t.Fatalf("unparsed = %d, want 0", len(unparsed))
	}

	ep3 := findGroup(t, groups, 3)
	wantEP3Titles := []string{"EP3", "EP 03 Part 2", "ep03 part three"}
	if len(ep3.Parts) != len(wantEP3Titles) {
		t.Fatalf("ep3 parts = %d, want %d", len(ep3.Parts), len(wantEP3Titles))
	}
	for i, part := range ep3.Parts {
		if part.Sequence != i+1 {
			t.Errorf("ep3 part %d sequence = %d, want %d", i, part.Sequence, i+1)
		}
		if part.Video.Title != wantEP3Titles[i] {
			t.Errorf("ep3 sequence %d title = %q, want %q", part.Sequence, part.Video.Title, wantEP3Titles[i])
		}
	}

	ep4 := findGroup(t, groups, 4)
	wantEP4Titles := []string{"EP4", "EP 04 - Part 2"}
	if len(ep4.Parts) != len(wantEP4Titles) {
		t.Fatalf("ep4 parts = %d, want %d", len(ep4.Parts), len(wantEP4Titles))
	}
	for i, part := range ep4.Parts {
		if part.Sequence != i+1 {
			t.Errorf("ep4 part %d sequence = %d, want %d", i, part.Sequence, i+1)
		}
		if part.Video.Title != wantEP4Titles[i] {
			t.Errorf("ep4 sequence %d title = %q, want %q", part.Sequence, part.Video.Title, wantEP4Titles[i])
		}
	}

	t.Run("unparsed video reported", func(t *testing.T) {
		published := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
		videos := append(specVideos(), model.Video{
			ID:          6,
			ChannelID:   1,
			YTVideoID:   "yt-6",
			Title:       "Random Vlog Day",
			PublishedAt: &published,
		})
		groups, unparsed := episodes.GroupVideos(videos)
		if len(unparsed) != 1 {
			t.Fatalf("unparsed = %d, want 1", len(unparsed))
		}
		if unparsed[0].Title != "Random Vlog Day" {
			t.Errorf("unparsed title = %q, want %q", unparsed[0].Title, "Random Vlog Day")
		}
		if len(groups) != 2 {
			t.Fatalf("groups = %d, want 2 (unchanged)", len(groups))
		}
		if got := len(findGroup(t, groups, 3).Parts); got != 3 {
			t.Errorf("ep3 parts = %d, want 3 (unchanged)", got)
		}
		if got := len(findGroup(t, groups, 4).Parts); got != 2 {
			t.Errorf("ep4 parts = %d, want 2 (unchanged)", got)
		}
	})
}

func TestGroupVideosWarnings(t *testing.T) {
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	makeVideo := func(id uint, title string) model.Video {
		published := base.Add(time.Duration(id) * time.Hour)
		return model.Video{ID: id, ChannelID: 1, Title: title, PublishedAt: &published}
	}

	t.Run("missing part", func(t *testing.T) {
		groups, _ := episodes.GroupVideos([]model.Video{
			makeVideo(1, "Show EP1 Part 1"),
			makeVideo(2, "Show EP1 Part 3"),
		})
		if len(groups) != 1 {
			t.Fatalf("groups = %d, want 1", len(groups))
		}
		found := false
		for _, warning := range groups[0].Warnings {
			if strings.Contains(warning, "missing part 2") {
				found = true
			}
		}
		if !found {
			t.Errorf("warnings = %v, want one containing %q", groups[0].Warnings, "missing part 2")
		}
	})

	t.Run("duplicate part", func(t *testing.T) {
		groups, _ := episodes.GroupVideos([]model.Video{
			makeVideo(1, "Show EP1 Part 1"),
			makeVideo(2, "Show EP01 Part 1"),
		})
		if len(groups) != 1 {
			t.Fatalf("groups = %d, want 1", len(groups))
		}
		found := false
		for _, warning := range groups[0].Warnings {
			if strings.Contains(warning, "duplicate part 1") {
				found = true
			}
		}
		if !found {
			t.Errorf("warnings = %v, want one containing %q", groups[0].Warnings, "duplicate part 1")
		}
	})
}

func openTestStore(t *testing.T) (*repo.Store, uint) {
	t.Helper()
	dbs, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	store := repo.NewStore(dbs)

	channel := &model.Channel{
		AccountID:   1,
		YTChannelID: "chan-episodes-test",
		Title:       "Episodes Test Channel",
	}
	if err := store.Channels.UpsertByYTID(channel); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	stored, err := store.Channels.GetByYTID("chan-episodes-test")
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	return store, stored.ID
}

func seedSpecVideos(t *testing.T, store *repo.Store, channelID uint) {
	t.Helper()
	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	for i, title := range specTitles {
		published := base.Add(time.Duration(i) * time.Hour)
		video := &model.Video{
			ChannelID:   channelID,
			YTVideoID:   fmt.Sprintf("yt-spec-%d", i+1),
			Title:       title,
			Visibility:  model.VisibilityPrivate,
			PublishedAt: &published,
		}
		if err := store.Videos.UpsertByYTID(video); err != nil {
			t.Fatalf("seed video %q: %v", title, err)
		}
	}
}

func partsCountByEpisode(t *testing.T, store *repo.Store, channelID uint) map[int]int {
	t.Helper()
	eps, err := store.Episodes.ListByChannel(channelID)
	if err != nil {
		t.Fatalf("ListByChannel episodes: %v", err)
	}
	counts := make(map[int]int, len(eps))
	for _, ep := range eps {
		parts, err := store.Episodes.ListParts(ep.ID)
		if err != nil {
			t.Fatalf("ListParts episode %d: %v", ep.EpisodeNumber, err)
		}
		counts[ep.EpisodeNumber] = len(parts)
	}
	return counts
}

func TestGroupChannelIdempotent(t *testing.T) {
	store, channelID := openTestStore(t)
	seedSpecVideos(t, store, channelID)

	result, err := episodes.GroupChannel(store, channelID)
	if err != nil {
		t.Fatalf("GroupChannel: %v", err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(result.Groups))
	}
	if len(result.Unparsed) != 0 {
		t.Fatalf("unparsed = %d, want 0", len(result.Unparsed))
	}
	counts := partsCountByEpisode(t, store, channelID)
	if counts[3] != 3 || counts[4] != 2 {
		t.Fatalf("parts counts = %v, want ep3:3 ep4:2", counts)
	}

	second, err := episodes.GroupChannel(store, channelID)
	if err != nil {
		t.Fatalf("second GroupChannel: %v", err)
	}
	if len(second.Groups) != 2 {
		t.Fatalf("second run groups = %d, want 2", len(second.Groups))
	}
	eps, err := store.Episodes.ListByChannel(channelID)
	if err != nil {
		t.Fatalf("ListByChannel episodes: %v", err)
	}
	if len(eps) != 2 {
		t.Fatalf("episode rows after re-run = %d, want 2", len(eps))
	}
	countsAfter := partsCountByEpisode(t, store, channelID)
	if countsAfter[3] != 3 || countsAfter[4] != 2 {
		t.Errorf("parts counts after re-run = %v, want ep3:3 ep4:2 (unchanged)", countsAfter)
	}
}
