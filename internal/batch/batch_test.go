package batch_test

import (
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/model"
)

func abVideos() []model.Video {
	return []model.Video{
		{ID: 1, YTVideoID: "yt-a", Title: "A"},
		{ID: 2, YTVideoID: "yt-b", Title: "B"},
	}
}

func TestTemplateNumbering(t *testing.T) {
	plan, err := batch.BuildPlan(abVideos(), batch.Pattern{
		Template:   "{title} [{n}]",
		StartIndex: 5,
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(plan.Changes))
	}
	if plan.Changes[0].NewTitle != "A [5]" {
		t.Errorf("change 0 NewTitle = %q, want %q", plan.Changes[0].NewTitle, "A [5]")
	}
	if plan.Changes[1].NewTitle != "B [6]" {
		t.Errorf("change 1 NewTitle = %q, want %q", plan.Changes[1].NewTitle, "B [6]")
	}
}

func TestTemplateStartIndexZeroDefaultsToOne(t *testing.T) {
	plan, err := batch.BuildPlan(abVideos(), batch.Pattern{
		Template:   "{title} [{n}]",
		StartIndex: 0,
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(plan.Changes))
	}
	if plan.Changes[0].NewTitle != "A [1]" {
		t.Errorf("change 0 NewTitle = %q, want %q", plan.Changes[0].NewTitle, "A [1]")
	}
	if plan.Changes[1].NewTitle != "B [2]" {
		t.Errorf("change 1 NewTitle = %q, want %q", plan.Changes[1].NewTitle, "B [2]")
	}
}

func TestRegexRewriteWithGroupReference(t *testing.T) {
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: "Show Part 2"}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Regex:       `(?i)part\s+(\d+)`,
		Replacement: "pt.$1",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(plan.Changes))
	}
	if plan.Changes[0].NewTitle != "Show pt.2" {
		t.Errorf("NewTitle = %q, want %q", plan.Changes[0].NewTitle, "Show pt.2")
	}
}

func TestRegexThenTemplateOrder(t *testing.T) {
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: "X Show"}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Regex:       "X",
		Replacement: "Y",
		Template:    "{title}!",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(plan.Changes))
	}
	if plan.Changes[0].NewTitle != "Y Show!" {
		t.Errorf("NewTitle = %q, want %q (regex applied before template)", plan.Changes[0].NewTitle, "Y Show!")
	}
}

func TestNFCNormalization(t *testing.T) {
	decomposed := "Cafe\u0301"
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: decomposed}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{Template: "{title}"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %d, want 1 (decomposed title normalizes to a change)", len(plan.Changes))
	}
	newTitle := plan.Changes[0].NewTitle
	if !strings.Contains(newTitle, "Caf\u00e9") {
		t.Errorf("NewTitle = %q, want it to contain composed %q", newTitle, "Caf\u00e9")
	}
	if want := norm.NFC.String(decomposed); newTitle != want {
		t.Errorf("NewTitle = %q, want NFC form %q", newTitle, want)
	}
}

func TestSkipsEmptyTitle(t *testing.T) {
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: "Show"}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Regex:       ".*",
		Replacement: "",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Errorf("changes = %d, want 0", len(plan.Changes))
	}
	if len(plan.Warnings) != 1 {
		t.Errorf("warnings = %d, want 1 (%v)", len(plan.Warnings), plan.Warnings)
	}
}

func TestSkipsTooLongTitle(t *testing.T) {
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: "A"}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Template: "{title}" + strings.Repeat("!", 100),
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Errorf("changes = %d, want 0", len(plan.Changes))
	}
	if len(plan.Warnings) != 1 {
		t.Errorf("warnings = %d, want 1 (%v)", len(plan.Warnings), plan.Warnings)
	}
}

func TestUnchangedVideo(t *testing.T) {
	videos := []model.Video{{ID: 1, YTVideoID: "yt-1", Title: "Show"}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{Template: "{title}"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.UnchangedCount != 1 {
		t.Errorf("UnchangedCount = %d, want 1", plan.UnchangedCount)
	}
	if len(plan.Changes) != 0 {
		t.Errorf("changes = %d, want 0", len(plan.Changes))
	}
}

func TestDescriptionTemplate(t *testing.T) {
	videos := []model.Video{{
		ID:          1,
		YTVideoID:   "yt-1",
		Title:       "Show",
		Description: "hello",
	}}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Template:            "{title}",
		DescriptionTemplate: "Ep: {description}",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(plan.Changes))
	}
	change := plan.Changes[0]
	if !change.HasDescriptionChange {
		t.Error("HasDescriptionChange = false, want true")
	}
	if change.NewDescription != "Ep: hello" {
		t.Errorf("NewDescription = %q, want %q", change.NewDescription, "Ep: hello")
	}
}

func TestPatternErrors(t *testing.T) {
	t.Run("empty pattern", func(t *testing.T) {
		if _, err := batch.BuildPlan(abVideos(), batch.Pattern{}); err == nil {
			t.Error("BuildPlan with empty Pattern{}: expected error, got nil")
		}
	})
	t.Run("invalid regex", func(t *testing.T) {
		if _, err := batch.BuildPlan(abVideos(), batch.Pattern{Regex: "["}); err == nil {
			t.Error("BuildPlan with regex \"[\": expected error, got nil")
		}
	})
}

func TestSummary(t *testing.T) {
	plan, err := batch.BuildPlan(abVideos(), batch.Pattern{
		Template:   "{title} [{n}]",
		StartIndex: 5,
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	summary := plan.Summary()
	lines := strings.Split(summary, "\n")
	if len(lines) == 0 {
		t.Fatal("summary is empty")
	}
	if !strings.Contains(lines[0], "2") {
		t.Errorf("first summary line = %q, want it to report 2 updates", lines[0])
	}
	if want := "2 video(s) to update, 0 unchanged, 0 skipped with warnings"; lines[0] != want {
		t.Errorf("first summary line = %q, want %q", lines[0], want)
	}
	if !strings.Contains(summary, "yt-a  A -> A [5]") {
		t.Errorf("summary missing change line %q:\n%s", "yt-a  A -> A [5]", summary)
	}
	if !strings.Contains(summary, "yt-b  B -> B [6]") {
		t.Errorf("summary missing change line %q:\n%s", "yt-b  B -> B [6]", summary)
	}
}
