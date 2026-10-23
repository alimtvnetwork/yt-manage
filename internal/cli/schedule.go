package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/scheduler"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

const scheduleUsage = `Usage: yt-manage schedule <subcommand> [arguments]

Apply cadence scheduling to a series of episodes. Every part of an
episode gets the same publish date (staggered by minutes in sequence
order); episodes are spaced by the rule's cadence — every day, or
every 1, 2 or 3 weeks. The full plan is shown for confirmation before
any video is updated. Already-public videos are skipped unless
--include-public is given and confirmed.

Subcommands:
  plan     Show the publish-date plan for a channel's episodes
  apply    Apply a plan: set publishAt on each part via the API
  rules    Manage the cadence rules stored per channel
           (rules list|add|delete)

Run "yt-manage schedule <subcommand> --help" for a subcommand's flags.
`

const scheduleRulesUsage = `Usage: yt-manage schedule rules <subcommand> [arguments]

Subcommands:
  list     List the cadence rules stored for a channel (--channel)
  add      Store a new cadence rule (--channel, --cadence or
           --cadence-days, --starts-at, --videos-per-episode)
  delete   Delete a stored rule (--id; asks for confirmation
           unless --yes is given)
`

const scheduleRulesListUsage = `Usage: yt-manage schedule rules list --channel ID

Arguments:
  --channel ID   Local channel ID (required)
`

const scheduleRulesAddUsage = `Usage: yt-manage schedule rules add --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339 [--videos-per-episode N]

Arguments:
  --channel ID            Local channel ID (required)
  --cadence C             daily, weekly, two-weeks or three-weeks
  --cadence-days N        Custom cadence in days (wins over
                          --cadence when set)
  --starts-at RFC3339     First episode's publish date/time
                          (required)
  --videos-per-episode N  Expected videos per episode (default 0)
`

const scheduleRulesDeleteUsage = `Usage: yt-manage schedule rules delete --id ID [--yes]

Arguments:
  --id ID   ID of the rule to delete (required)
  --yes     Skip the interactive confirmation
`

const schedulePlanUsage = `Usage: yt-manage schedule plan (--rule ID | --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339) [--include-public]

Show the publish-date plan for a channel's episodes. Nothing is
written. Already-public videos are skipped unless --include-public
is given.

Arguments:
  --rule ID               Use a stored schedule rule
  --channel ID            Local channel ID (for an unsaved rule)
  --cadence C             daily, weekly, two-weeks or three-weeks
  --cadence-days N        Custom cadence in days (wins over
                          --cadence when set)
  --starts-at RFC3339     First episode's publish date/time
  --include-public        Also plan already-public videos (they
                          are skipped by default)
`

const scheduleApplyUsage = `Usage: yt-manage schedule apply (--rule ID | --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339) [--include-public] [--yes]

Build the plan, show it, ask for confirmation (unless --yes), then
set publishAt on each part through the YouTube API and mirror the
schedule locally. Already-public videos are skipped unless
--include-public is given.

Arguments:
  --rule ID               Use a stored schedule rule
  --channel ID            Local channel ID (for an unsaved rule)
  --cadence C             daily, weekly, two-weeks or three-weeks
  --cadence-days N        Custom cadence in days (wins over
                          --cadence when set)
  --starts-at RFC3339     First episode's publish date/time
  --include-public        Also schedule already-public videos
                          (they are skipped by default)
  --yes                   Skip the interactive confirmation
`

// scheduleRuleFlags holds the flags shared by the plan, apply and
// rules add commands for describing a schedule rule.
type scheduleRuleFlags struct {
	ruleID           uint
	channelID        uint
	cadence          string
	cadenceDays      int
	startsAt         string
	videosPerEpisode int
	includePublic    bool
	yes              bool
}

// addScheduleRuleFlags registers the shared rule flags on fs.
func addScheduleRuleFlags(fs *flag.FlagSet) *scheduleRuleFlags {
	opts := &scheduleRuleFlags{}
	fs.UintVar(&opts.ruleID, "rule", 0, "stored schedule rule ID")
	fs.UintVar(&opts.channelID, "channel", 0, "local channel ID")
	fs.StringVar(&opts.cadence, "cadence", "", "cadence name (daily, weekly, two-weeks, three-weeks)")
	fs.IntVar(&opts.cadenceDays, "cadence-days", 0, "cadence in days (overrides --cadence)")
	fs.StringVar(&opts.startsAt, "starts-at", "", "first publish time (RFC3339)")
	fs.IntVar(&opts.videosPerEpisode, "videos-per-episode", 0, "expected videos per episode")
	fs.BoolVar(&opts.includePublic, "include-public", false, "include already-public videos")
	fs.BoolVar(&opts.yes, "yes", false, "skip confirmation")
	return opts
}

// resolveRule resolves the shared flags to a schedule rule: the
// stored rule when --rule is set, otherwise an unsaved rule built
// from --channel, a cadence source and --starts-at.
func resolveRule(store *repo.Store, opts *scheduleRuleFlags) (model.ScheduleRule, error) {
	if opts.ruleID > 0 {
		rule, err := store.Schedules.GetByID(opts.ruleID)
		if err != nil {
			return model.ScheduleRule{}, err
		}
		return *rule, nil
	}
	if opts.channelID == 0 {
		return model.ScheduleRule{}, fmt.Errorf("--channel is required when --rule is not given")
	}
	if opts.startsAt == "" {
		return model.ScheduleRule{}, fmt.Errorf("--starts-at is required when --rule is not given")
	}
	startsAt, err := time.Parse(time.RFC3339, opts.startsAt)
	if err != nil {
		return model.ScheduleRule{}, fmt.Errorf("invalid --starts-at: %w", err)
	}
	cadenceDays, err := resolveCadenceDays(opts)
	if err != nil {
		return model.ScheduleRule{}, err
	}
	return model.ScheduleRule{
		ChannelID:        opts.channelID,
		CadenceDays:      cadenceDays,
		VideosPerEpisode: opts.videosPerEpisode,
		StartsAt:         &startsAt,
		IsActive:         true,
	}, nil
}

// resolveCadenceDays resolves the cadence flags to a day count:
// --cadence-days wins when set, otherwise the named --cadence.
func resolveCadenceDays(opts *scheduleRuleFlags) (int, error) {
	if opts.cadenceDays > 0 {
		return opts.cadenceDays, nil
	}
	if opts.cadence == "" {
		return 0, fmt.Errorf("a cadence source is required: --cadence or --cadence-days")
	}
	cadence, err := model.ParseCadence(opts.cadence)
	if err != nil {
		return 0, err
	}
	days := cadence.Days()
	if days == 0 {
		return 0, fmt.Errorf("--cadence %q needs an explicit --cadence-days value", opts.cadence)
	}
	return days, nil
}

// runSchedule handles "yt-manage schedule ...".
func runSchedule(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(scheduleUsage)
	}
	switch args[0] {
	case "plan":
		return runSchedulePlan(args[1:])
	case "apply":
		return runScheduleApply(args[1:])
	case "rules":
		return runScheduleRules(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage schedule: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, scheduleUsage)
		return 2
	}
}

// runScheduleRules handles "yt-manage schedule rules ...".
func runScheduleRules(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(scheduleRulesUsage)
	}
	switch args[0] {
	case "list":
		return runScheduleRulesList(args[1:])
	case "add":
		return runScheduleRulesAdd(args[1:])
	case "delete":
		return runScheduleRulesDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, scheduleRulesUsage)
		return 2
	}
}

// runScheduleRulesList handles "yt-manage schedule rules list ...".
func runScheduleRulesList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(scheduleRulesListUsage)
	}
	fs := flag.NewFlagSet("schedule rules list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *channelID == 0 {
		fmt.Fprint(os.Stderr, scheduleRulesListUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules list: %v\n", err)
		return 1
	}
	defer dbs.Close()
	rules, err := store.Schedules.ListByChannel(*channelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules list: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(rules))
	for _, rule := range rules {
		active := "no"
		if rule.IsActive {
			active = "yes"
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(rule.ID), 10),
			strconv.FormatUint(uint64(rule.ChannelID), 10),
			strconv.Itoa(rule.CadenceDays),
			strconv.Itoa(rule.VideosPerEpisode),
			formatTime(rule.StartsAt),
			active,
		})
	}
	printTable(os.Stdout, []string{"ID", "Channel", "Cadence days", "Videos/Episode", "Starts", "Active"}, rows)
	return 0
}

// runScheduleRulesAdd handles "yt-manage schedule rules add ...".
func runScheduleRulesAdd(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(scheduleRulesAddUsage)
	}
	fs := flag.NewFlagSet("schedule rules add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := addScheduleRuleFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules add: %v\n", err)
		return 1
	}
	defer dbs.Close()
	resolved, err := resolveRule(store, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules add: %v\n", err)
		return 2
	}
	rule := resolved
	if err := store.Schedules.Create(&rule); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules add: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Created schedule rule %d.\n", rule.ID)
	return 0
}

// runScheduleRulesDelete handles "yt-manage schedule rules delete ...".
func runScheduleRulesDelete(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(scheduleRulesDeleteUsage)
	}
	fs := flag.NewFlagSet("schedule rules delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.Uint("id", 0, "schedule rule ID")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == 0 {
		fmt.Fprint(os.Stderr, scheduleRulesDeleteUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules delete: %v\n", err)
		return 1
	}
	defer dbs.Close()
	if !*yes {
		summary := fmt.Sprintf("Delete schedule rule %d?", *id)
		if !confirmChangeSet(os.Stdin, os.Stdout, summary) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	}
	if err := store.Schedules.Delete(*id); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule rules delete: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Deleted schedule rule %d.\n", *id)
	return 0
}

// buildSchedulePlan resolves the rule from the flags and builds
// its plan. It returns a non-zero exit code when either step fails.
func buildSchedulePlan(store *repo.Store, opts *scheduleRuleFlags, command string) (*scheduler.Plan, int) {
	rule, err := resolveRule(store, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule %s: %v\n", command, err)
		if opts.ruleID > 0 {
			return nil, 1
		}
		return nil, 2
	}
	plan, err := scheduler.BuildPlan(store, rule, opts.includePublic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule %s: %v\n", command, err)
		return nil, 1
	}
	return plan, 0
}

// printSchedulePlan prints the plan's date table, the skipped
// already-public section, and the would-schedule footer.
func printSchedulePlan(plan *scheduler.Plan) {
	rows := make([][]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		rows = append(rows, []string{
			entry.PublishAt.Format("2006-01-02 15:04"),
			entry.EpisodeKey,
			strconv.Itoa(entry.PartNumber),
			entry.Title,
			entry.YTVideoID,
		})
	}
	printTable(os.Stdout, []string{"Publish at", "Episode", "Part", "Title", "YT ID"}, rows)
	if len(plan.SkippedPublic) > 0 {
		fmt.Fprintln(os.Stdout, "Skipped (already public):")
		for _, entry := range plan.SkippedPublic {
			fmt.Fprintf(os.Stdout, "  - %s (%s)\n", entry.Title, entry.YTVideoID)
		}
	}
	fmt.Fprintf(os.Stdout, "%d video(s) would be scheduled, %d skipped.\n", len(plan.Entries), len(plan.SkippedPublic))
}

// runSchedulePlan handles "yt-manage schedule plan ...": it builds
// and prints the plan without writing anything.
func runSchedulePlan(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(schedulePlanUsage)
	}
	fs := flag.NewFlagSet("schedule plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := addScheduleRuleFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule plan: %v\n", err)
		return 1
	}
	defer dbs.Close()
	plan, exitCode := buildSchedulePlan(store, opts, "plan")
	if exitCode != 0 {
		return exitCode
	}
	printSchedulePlan(plan)
	return 0
}

// runScheduleApply handles "yt-manage schedule apply ...": it
// builds and prints the plan, confirms it (unless --yes), then
// applies it through the account that owns the rule's channel.
func runScheduleApply(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(scheduleApplyUsage)
	}
	fs := flag.NewFlagSet("schedule apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := addScheduleRuleFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule apply: %v\n", err)
		return 1
	}
	defer dbs.Close()
	plan, exitCode := buildSchedulePlan(store, opts, "apply")
	if exitCode != 0 {
		return exitCode
	}
	printSchedulePlan(plan)
	if len(plan.Entries) == 0 {
		fmt.Fprintln(os.Stdout, "Nothing to schedule.")
		return 0
	}
	if !opts.yes {
		if !confirmChangeSet(os.Stdin, os.Stdout, plan.Summary()) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	}
	applied, err := applySchedulePlan(store, plan)
	fmt.Fprintf(os.Stdout, "Scheduled %d/%d video(s).\n", applied, len(plan.Entries))
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage schedule apply: %v\n", err)
		return 1
	}
	return 0
}

// applySchedulePlan resolves the account for the plan's channel
// and applies the plan through that account's YouTube service.
func applySchedulePlan(store *repo.Store, plan *scheduler.Plan) (int, error) {
	home, err := config.HomeDir()
	if err != nil {
		return 0, err
	}
	channels, err := store.Channels.List()
	if err != nil {
		return 0, err
	}
	accountByChannel := make(map[uint]uint, len(channels))
	for _, channel := range channels {
		accountByChannel[channel.ID] = channel.AccountID
	}
	accountID, hasAccount := accountByChannel[plan.Rule.ChannelID]
	if !hasAccount {
		return 0, fmt.Errorf("no account found for channel %d", plan.Rule.ChannelID)
	}
	key, err := cryptox.LoadKey(home)
	if err != nil {
		return 0, err
	}
	ctx := context.Background()
	svc, err := ytapi.ServiceForAccount(ctx, home, store, key, accountID)
	if err != nil {
		return 0, err
	}
	return scheduler.Apply(ctx, svc, store, plan)
}
