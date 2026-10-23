package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

const renameUsage = `Usage: yt-manage rename [--query TEXT] [--channel ID] [--visibility V] [--template T | --regex R [--replacement S] | --description-template T] [--start N] [--limit N] [--apply [--yes]]

Batch rename or edit videos matched by a local-mirror search. The
matches are first shown as a change-set (old -> new); nothing is
written to YouTube until --apply is given and the change-set is
confirmed. Renames are Unicode-safe (NFC normalisation).

Arguments:
  --query TEXT             Search text selecting the videos
  --channel ID             Only videos on the local channel with this ID
  --visibility V           Only videos with this visibility
  --template T             New title template using {title} and {n}
  --regex R                Regex matched against the current title
  --replacement S          Replacement for --regex (default "")
  --description-template T New description template using
                           {description}, {title} and {n}
  --start N                First value for {n} (default 1)
  --limit N                Maximum number of videos (default 500)
  --apply                  Apply the change-set to YouTube
  --yes                    Skip the interactive confirmation
`

// runRename handles "yt-manage rename ...".
func runRename(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(renameUsage)
	}
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	query := fs.String("query", "", "search text")
	channelID := fs.Uint("channel", 0, "local channel ID")
	visibility := fs.String("visibility", "", "visibility filter")
	template := fs.String("template", "", "title template")
	regex := fs.String("regex", "", "title regex")
	replacement := fs.String("replacement", "", "regex replacement")
	descriptionTemplate := fs.String("description-template", "", "description template")
	start := fs.Int("start", 0, "first value for {n}")
	limit := fs.Int("limit", 500, "maximum number of videos")
	apply := fs.Bool("apply", false, "apply the change-set")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	hasPattern := *template != "" || *regex != "" || *descriptionTemplate != ""
	if !hasPattern {
		fmt.Fprint(os.Stderr, renameUsage)
		return 2
	}
	if *visibility != "" {
		if _, err := model.ParseVisibility(*visibility); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
			return 2
		}
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 1
	}
	defer dbs.Close()
	videos, err := searchx.Search(store, searchx.Query{
		Text:       *query,
		ChannelID:  *channelID,
		Visibility: *visibility,
		Limit:      *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 1
	}
	plan, err := batch.BuildPlan(videos, batch.Pattern{
		Template:            *template,
		Regex:               *regex,
		Replacement:         *replacement,
		DescriptionTemplate: *descriptionTemplate,
		StartIndex:          *start,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 2
	}
	fmt.Fprintln(os.Stdout, plan.Summary())
	if len(plan.Changes) == 0 {
		fmt.Fprintln(os.Stdout, "Nothing to change.")
		return 0
	}
	if !*apply {
		fmt.Fprintln(os.Stdout, "Dry run — no changes made. Re-run with --apply to update titles on YouTube.")
		return 0
	}
	if !*yes {
		if !confirmChangeSet(os.Stdin, os.Stdout, plan.Summary()) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	}
	return applyRenamePlan(store, plan)
}

// applyRenamePlan groups the plan changes by account and applies
// each group through that account's YouTube service. It stops at
// the first failure.
func applyRenamePlan(store *repo.Store, plan *batch.Plan) int {
	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 1
	}
	channels, err := store.Channels.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 1
	}
	accountByChannel := make(map[uint]uint, len(channels))
	for _, channel := range channels {
		accountByChannel[channel.ID] = channel.AccountID
	}
	accountOrder := []uint{}
	changesByAccount := make(map[uint][]batch.Change)
	for _, change := range plan.Changes {
		accountID, hasAccount := accountByChannel[change.ChannelID]
		if !hasAccount {
			fmt.Fprintf(os.Stderr, "yt-manage rename: no account found for channel %d\n", change.ChannelID)
			return 1
		}
		if _, hasGroup := changesByAccount[accountID]; !hasGroup {
			accountOrder = append(accountOrder, accountID)
		}
		changesByAccount[accountID] = append(changesByAccount[accountID], change)
	}
	key, err := cryptox.LoadKey(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
		return 1
	}
	ctx := context.Background()
	totalApplied := 0
	for _, accountID := range accountOrder {
		svc, err := ytapi.ServiceForAccount(ctx, home, store, key, accountID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", err)
			return 1
		}
		subPlan := &batch.Plan{Changes: changesByAccount[accountID]}
		applied, applyErr := batch.Apply(ctx, svc, store, subPlan)
		totalApplied += applied
		fmt.Fprintf(os.Stdout, "%d applied for account %d\n", applied, accountID)
		if applyErr != nil {
			fmt.Fprintf(os.Stderr, "yt-manage rename: %v\n", applyErr)
			return 1
		}
	}
	fmt.Fprintf(os.Stdout, "Applied %d/%d rename(s).\n", totalApplied, len(plan.Changes))
	return 0
}
