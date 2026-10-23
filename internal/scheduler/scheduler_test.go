package scheduler_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/db"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
)

type videoSpec struct {
	ytID       string
	title      string
	episode    int
	partNumber int
	sequence   int
}

var specVideos = []videoSpec{
	{"v-ep3-p1", "EP3", 3, 1, 1},
	{"v-ep3-p2", "EP 03 Part 2", 3, 2, 2},
	{"v-ep3-p3", "ep03 part three", 3, 3, 3},
	{"v-ep4-p1", "EP4", 4, 1, 1},
	{"v-ep4-p2", "EP 04 - Part 2", 4, 2, 2},
}

// seedStore builds a temp store holding one channel, the five spec
// 03 §6 videos (private, except ep3 part 2 when publicPart2 is set),
// and episodes ep3/ep4 with their parts, materialised directly
// through the repos.
func seedStore(t *testing.T, publicPart2 bool) (*repo.Store, uint) {
	t.Helper()
	dbs, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	store := repo.NewStore(dbs)

	channel := &model.Channel{
		AccountID:   1,
		YTChannelID: "chan-scheduler-test",
		Title:       "Scheduler Test Channel",
	}
	if err := store.Channels.UpsertByYTID(channel); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	storedChannel, err := store.Channels.GetByYTID("chan-scheduler-test")
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	channelID := storedChannel.ID

	videoIDs := make(map[string]uint, len(specVideos))
	for _, spec := range specVideos {
		visibility := model.VisibilityPrivate
		if publicPart2 && spec.ytID == "v-ep3-p2" {
			visibility = model.VisibilityPublic
		}
		video := &model.Video{
			ChannelID:  channelID,
			YTVideoID:  spec.ytID,
			Title:      spec.title,
			Visibility: visibility,
		}
		if err := store.Videos.UpsertByYTID(video); err != nil {
			t.Fatalf("seed video %q: %v", spec.ytID, err)
		}
		stored, err := store.Videos.GetByYTID(spec.ytID)
		if err != nil {
			t.Fatalf("get video %q: %v", spec.ytID, err)
		}
		videoIDs[spec.ytID] = stored.ID
	}

	for _, episodeNumber := range []int{3, 4} {
		episode := &model.Episode{
			ChannelID:     channelID,
			EpisodeKey:    fmt.Sprintf("|ep%d", episodeNumber),
			EpisodeNumber: episodeNumber,
		}
		if err := store.Episodes.UpsertByKey(episode); err != nil {
			t.Fatalf("seed episode %d: %v", episodeNumber, err)
		}
		stored, err := store.Episodes.GetByKey(channelID, episode.EpisodeKey)
		if err != nil {
			t.Fatalf("get episode %d: %v", episodeNumber, err)
		}
		var parts []model.EpisodePart
		for _, spec := range specVideos {
			if spec.episode != episodeNumber {
				continue
			}
			parts = append(parts, model.EpisodePart{
				EpisodeID:  stored.ID,
				VideoID:    videoIDs[spec.ytID],
				PartNumber: spec.partNumber,
				Sequence:   spec.sequence,
			})
		}
		if err := store.Episodes.ReplaceParts(stored.ID, parts); err != nil {
			t.Fatalf("seed parts episode %d: %v", episodeNumber, err)
		}
	}
	return store, channelID
}

func specStartsAt() time.Time {
	return time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
}

func TestBuildPlanSpecExample(t *testing.T) {
	store, channelID := seedStore(t, false)
	startsAt := specStartsAt()
	rule := model.ScheduleRule{
		ChannelID:   channelID,
		CadenceDays: 21,
		StartsAt:    &startsAt,
	}
	plan, err := scheduler.BuildPlan(store, rule, false)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.SkippedPublic) != 0 {
		t.Fatalf("SkippedPublic = %d, want 0", len(plan.SkippedPublic))
	}
	if len(plan.Entries) != 5 {
		t.Fatalf("entries = %d, want 5", len(plan.Entries))
	}
	wantTimes := []time.Time{
		time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 10, 5, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 10, 10, 0, 0, time.UTC),
		time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 23, 10, 5, 0, 0, time.UTC),
	}
	wantEpisodes := []int{3, 3, 3, 4, 4}
	for i, entry := range plan.Entries {
		if !entry.PublishAt.Equal(wantTimes[i]) {
			t.Errorf("entry %d PublishAt = %s, want %s", i,
				entry.PublishAt.Format(time.RFC3339), wantTimes[i].Format(time.RFC3339))
		}
		if entry.EpisodeNumber != wantEpisodes[i] {
			t.Errorf("entry %d EpisodeNumber = %d, want %d", i, entry.EpisodeNumber, wantEpisodes[i])
		}
	}
}

func TestBuildPlanCadenceVariants(t *testing.T) {
	cases := []struct {
		name        string
		cadenceDays int
		wantEP4     time.Time
	}{
		{"daily", 1, time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)},
		{"weekly", 7, time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, channelID := seedStore(t, false)
			startsAt := specStartsAt()
			rule := model.ScheduleRule{
				ChannelID:   channelID,
				CadenceDays: tc.cadenceDays,
				StartsAt:    &startsAt,
			}
			plan, err := scheduler.BuildPlan(store, rule, false)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}
			var ep4First *scheduler.PlanEntry
			for i := range plan.Entries {
				if plan.Entries[i].EpisodeNumber == 4 {
					ep4First = &plan.Entries[i]
					break
				}
			}
			if ep4First == nil {
				t.Fatal("no ep4 entry in plan")
			}
			if !ep4First.PublishAt.Equal(tc.wantEP4) {
				t.Errorf("ep4 first PublishAt = %s, want %s",
					ep4First.PublishAt.Format(time.RFC3339), tc.wantEP4.Format(time.RFC3339))
			}
		})
	}
}

func TestBuildPlanPublicHandling(t *testing.T) {
	store, channelID := seedStore(t, true)
	startsAt := specStartsAt()
	rule := model.ScheduleRule{
		ChannelID:   channelID,
		CadenceDays: 21,
		StartsAt:    &startsAt,
	}

	plan, err := scheduler.BuildPlan(store, rule, false)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Entries) != 4 {
		t.Errorf("entries = %d, want 4", len(plan.Entries))
	}
	if len(plan.SkippedPublic) != 1 {
		t.Fatalf("SkippedPublic = %d, want 1", len(plan.SkippedPublic))
	}
	skipped := plan.SkippedPublic[0]
	if skipped.EpisodeNumber != 3 || skipped.PartNumber != 2 {
		t.Errorf("skipped entry = episode %d part %d, want episode 3 part 2",
			skipped.EpisodeNumber, skipped.PartNumber)
	}
	if !skipped.IsPublic {
		t.Error("skipped entry IsPublic = false, want true")
	}

	included, err := scheduler.BuildPlan(store, rule, true)
	if err != nil {
		t.Fatalf("BuildPlan (include public): %v", err)
	}
	if len(included.Entries) != 5 {
		t.Errorf("entries with includePublic = %d, want 5", len(included.Entries))
	}
	if len(included.SkippedPublic) != 0 {
		t.Errorf("SkippedPublic with includePublic = %d, want 0", len(included.SkippedPublic))
	}
}

func TestBuildPlanNilStartsAt(t *testing.T) {
	store, channelID := seedStore(t, false)
	rule := model.ScheduleRule{
		ChannelID:   channelID,
		CadenceDays: 21,
		StartsAt:    nil,
	}
	if _, err := scheduler.BuildPlan(store, rule, false); err == nil {
		t.Error("BuildPlan with nil StartsAt: expected error, got nil")
	}
}
