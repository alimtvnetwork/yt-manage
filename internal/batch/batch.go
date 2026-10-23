// Package batch plans and applies batch title and description
// edits. Planning is pure: BuildPlan turns mirrored videos and a
// rename pattern into a change-set that the CLI shows for
// confirmation before Apply writes anything to YouTube.
package batch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// maxTitleRunes is the YouTube title length limit, in runes.
const maxTitleRunes = 100

// Pattern describes how new titles and descriptions are derived
// from the existing ones. Regex is applied first, then Template.
// The placeholders are {title} and {n} for titles, and
// {description}, {title} and {n} for description templates.
type Pattern struct {
	Template            string
	Regex               string
	Replacement         string
	DescriptionTemplate string
	StartIndex          int
}

// Change is one planned update: old and new title (and description
// when it changes) for a single video.
type Change struct {
	VideoID              uint
	YTVideoID            string
	ChannelID            uint
	OldTitle             string
	NewTitle             string
	OldDescription       string
	NewDescription       string
	HasDescriptionChange bool
}

// Plan is a change-set produced by BuildPlan: the changes to apply,
// how many videos were left unchanged, and warnings for the videos
// that were skipped.
type Plan struct {
	Changes        []Change
	UnchangedCount int
	Warnings       []string
}

// BuildPlan builds the change-set for videos under pattern p, in
// input order. An error is returned only when the pattern itself is
// invalid: the regex does not compile, or no template, regex or
// description template is set at all. Videos whose new title would
// be empty or longer than 100 runes are skipped with a warning.
func BuildPlan(videos []model.Video, p Pattern) (*Plan, error) {
	hasTitlePattern := p.Template != "" || p.Regex != ""
	if !hasTitlePattern && p.DescriptionTemplate == "" {
		return nil, fmt.Errorf("pattern is empty: set a template, a regex, or a description template")
	}
	var compiled *regexp.Regexp
	if p.Regex != "" {
		var err error
		compiled, err = regexp.Compile(p.Regex)
		if err != nil {
			return nil, fmt.Errorf("compile regex: %w", err)
		}
	}
	startIndex := p.StartIndex
	if startIndex == 0 {
		startIndex = 1
	}
	plan := &Plan{}
	for i, video := range videos {
		change, isChanged, warning := planVideo(video, p, compiled, startIndex+i)
		if warning != "" {
			plan.Warnings = append(plan.Warnings, warning)
			continue
		}
		if !isChanged {
			plan.UnchangedCount++
			continue
		}
		plan.Changes = append(plan.Changes, change)
	}
	return plan, nil
}

// planVideo computes the change for one video. It returns the
// change, whether anything actually changes, and a warning when
// the video must be skipped.
func planVideo(video model.Video, p Pattern, compiled *regexp.Regexp, n int) (Change, bool, string) {
	working := video.Title
	if compiled != nil {
		working = compiled.ReplaceAllString(working, p.Replacement)
	}
	if p.Template != "" {
		replacer := strings.NewReplacer(
			"{title}", working,
			"{n}", strconv.Itoa(n),
		)
		working = replacer.Replace(p.Template)
	}
	newTitle := norm.NFC.String(working)

	newDescription := video.Description
	hasDescriptionChange := false
	if p.DescriptionTemplate != "" {
		replacer := strings.NewReplacer(
			"{description}", video.Description,
			"{title}", newTitle,
			"{n}", strconv.Itoa(n),
		)
		newDescription = norm.NFC.String(replacer.Replace(p.DescriptionTemplate))
		hasDescriptionChange = newDescription != video.Description
	}

	if newTitle == "" {
		return Change{}, false, fmt.Sprintf("video %d (%s): new title is empty", video.ID, video.YTVideoID)
	}
	if runeCount := len([]rune(newTitle)); runeCount > maxTitleRunes {
		return Change{}, false, fmt.Sprintf("video %d (%s): new title is %d runes, exceeds %d", video.ID, video.YTVideoID, runeCount, maxTitleRunes)
	}
	if newTitle == video.Title && !hasDescriptionChange {
		return Change{}, false, ""
	}
	change := Change{
		VideoID:              video.ID,
		YTVideoID:            video.YTVideoID,
		ChannelID:            video.ChannelID,
		OldTitle:             video.Title,
		NewTitle:             newTitle,
		OldDescription:       video.Description,
		NewDescription:       newDescription,
		HasDescriptionChange: hasDescriptionChange,
	}
	return change, true, ""
}

// Summary renders the plan as a human-readable change-set: a
// headline count line, one line per change, then the warnings.
func (p *Plan) Summary() string {
	lines := []string{
		fmt.Sprintf("%d video(s) to update, %d unchanged, %d skipped with warnings", len(p.Changes), p.UnchangedCount, len(p.Warnings)),
	}
	for _, change := range p.Changes {
		line := fmt.Sprintf("%s  %s -> %s", change.YTVideoID, change.OldTitle, change.NewTitle)
		if change.HasDescriptionChange {
			line += " (+description)"
		}
		lines = append(lines, line)
	}
	for _, warning := range p.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	return strings.Join(lines, "\n")
}
