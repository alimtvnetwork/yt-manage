// Package scheduler turns cadence rules into publish plans and
// applies them. Planning is pure over the local mirror: episode k
// publishes at starts_at + k × cadence_days, and every part of the
// episode shares that date, staggered by five minutes per part in
// sequence order so playlist order stays stable (spec 03 §5).
package scheduler

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// partStagger is the gap between consecutive parts of one episode.
const partStagger = 5 * time.Minute

// PlanEntry is one video's planned publish time.
type PlanEntry struct {
	EpisodeID     uint
	EpisodeKey    string
	EpisodeNumber int
	VideoID       uint
	YTVideoID     string
	Title         string
	PartNumber    int
	Sequence      int
	PublishAt     time.Time
	IsPublic      bool
}

// Plan is a built schedule: the entries to apply, in publish
// order, and the already-public entries that were skipped.
type Plan struct {
	Rule          model.ScheduleRule
	Entries       []PlanEntry
	SkippedPublic []PlanEntry
}

// BuildPlan builds the publish plan for a rule over the channel's
// materialised episodes, ordered by episode number. Already-public
// videos land in SkippedPublic unless isIncludePublic is set, in
// which case they are planned like any other entry.
func BuildPlan(store *repo.Store, rule model.ScheduleRule, isIncludePublic bool) (*Plan, error) {
	if rule.StartsAt == nil {
		return nil, fmt.Errorf("schedule rule for channel %d has no starts_at time", rule.ChannelID)
	}
	episodes, err := store.Episodes.ListByChannel(rule.ChannelID)
	if err != nil {
		return nil, err
	}
	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].EpisodeNumber < episodes[j].EpisodeNumber
	})
	plan := &Plan{Rule: rule}
	for k, episode := range episodes {
		base := rule.StartsAt.AddDate(0, 0, k*rule.CadenceDays)
		parts, err := store.Episodes.ListParts(episode.ID)
		if err != nil {
			return nil, err
		}
		sort.Slice(parts, func(i, j int) bool {
			return parts[i].Sequence < parts[j].Sequence
		})
		for _, part := range parts {
			video, err := store.Videos.GetByID(part.VideoID)
			if err != nil {
				return nil, err
			}
			entry := PlanEntry{
				EpisodeID:     episode.ID,
				EpisodeKey:    episode.EpisodeKey,
				EpisodeNumber: episode.EpisodeNumber,
				VideoID:       video.ID,
				YTVideoID:     video.YTVideoID,
				Title:         video.Title,
				PartNumber:    part.PartNumber,
				Sequence:      part.Sequence,
				PublishAt:     base.Add(time.Duration(part.Sequence-1) * partStagger),
				IsPublic:      video.Visibility == model.VisibilityPublic,
			}
			if entry.IsPublic && !isIncludePublic {
				plan.SkippedPublic = append(plan.SkippedPublic, entry)
			} else {
				plan.Entries = append(plan.Entries, entry)
			}
		}
	}
	return plan, nil
}

// Summary renders the plan as one line per entry, followed by the
// skipped already-public entries when there are any. It is the
// text shown for confirmation before a plan is applied.
func (p *Plan) Summary() string {
	lines := make([]string, 0, len(p.Entries)+len(p.SkippedPublic)+1)
	for _, entry := range p.Entries {
		lines = append(lines, summaryLine(entry))
	}
	if len(p.SkippedPublic) > 0 {
		lines = append(lines, "skipped (already public):")
		for _, entry := range p.SkippedPublic {
			lines = append(lines, summaryLine(entry))
		}
	}
	return strings.Join(lines, "\n")
}

// summaryLine renders one plan entry.
func summaryLine(entry PlanEntry) string {
	return fmt.Sprintf("%s  %s part %d  %s", entry.PublishAt.Format("2006-01-02 15:04"), entry.EpisodeKey, entry.PartNumber, entry.Title)
}
