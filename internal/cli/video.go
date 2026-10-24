package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

const videoUsage = `Usage: yt-manage video <subcommand> [arguments]

Inspect and edit videos held in the local mirror.

Subcommands:
  list     List mirrored videos (filter with --channel, --visibility)
  show     Show full details for one video by local ID or YouTube ID
  edit     Edit one video by local ID: plan the change-set, then
           --apply writes to YouTube first and the mirror second

Run "yt-manage video <subcommand> --help" for a subcommand's flags.
`

const videoListUsage = `Usage: yt-manage video list [--channel ID] [--visibility V] [--limit N]

Arguments:
  --channel ID   Only videos on the local channel with this ID
  --visibility V Only videos with this visibility
  --limit N      Maximum number of results (default 50, max 500)
`

const videoShowUsage = `Usage: yt-manage video show <id>

Arguments:
  <id>  Local video ID, or the YouTube video ID when no local ID
        matches
`

const videoEditUsage = `Usage: yt-manage video edit --id ID [--title T] [--description D] [--visibility V] [--publish-at RFC3339] [--apply [--yes]]

Arguments:
  --id ID          Local video ID (required)
  --title T        New title
  --description D  New description
  --visibility V   New visibility (public, unlisted, private,
                   scheduled, draft); scheduled requires --publish-at
  --publish-at T   Scheduled publish time in RFC3339 format; the
                   video is made private with this publishAt
  --apply          Apply the change to YouTube
  --yes            Skip the interactive confirmation
`

// runVideo handles "yt-manage video ...".
func runVideo(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(videoUsage)
	}
	switch args[0] {
	case "list":
		return runVideoList(args[1:])
	case "show":
		return runVideoShow(args[1:])
	case "edit":
		return runVideoEdit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage video: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, videoUsage)
		return 2
	}
}

// runVideoList handles "yt-manage video list ...".
func runVideoList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(videoListUsage)
	}
	wantJSON := hasJSONFlag(args)
	args = stripJSONFlag(args)
	fs := flag.NewFlagSet("video list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	visibility := fs.String("visibility", "", "visibility filter")
	limit := fs.Int("limit", 50, "maximum number of results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	*channelID = withDefaultChannel(*channelID)
	if *visibility != "" {
		if _, err := model.ParseVisibility(*visibility); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage video list: %v\n", err)
			return 2
		}
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video list: %v\n", err)
		return 1
	}
	defer dbs.Close()
	videos, err := searchx.Search(store, searchx.Query{
		ChannelID:  *channelID,
		Visibility: *visibility,
		Limit:      *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video list: %v\n", err)
		return 1
	}
	if wantJSON {
		return printJSON(videos)
	}
	printVideoTable(os.Stdout, videos)
	return 0
}

// runVideoShow handles "yt-manage video show ...".
func runVideoShow(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(videoShowUsage)
	}
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, videoShowUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video show: %v\n", err)
		return 1
	}
	defer dbs.Close()
	video := findVideo(store, args[0])
	if video == nil {
		fmt.Fprintf(os.Stderr, "yt-manage video show: video not found: %s\n", args[0])
		return 1
	}
	printVideoDetails(video)
	return 0
}

// findVideo resolves arg to a mirrored video: when it parses as a
// local ID it tries GetByID first, then falls back to GetByYTID.
// It returns nil when nothing matches.
func findVideo(store *repo.Store, arg string) *model.Video {
	if id, err := strconv.ParseUint(arg, 10, 64); err == nil {
		if video, err := store.Videos.GetByID(uint(id)); err == nil {
			return video
		}
	}
	if video, err := store.Videos.GetByYTID(arg); err == nil {
		return video
	}
	return nil
}

// printVideoDetails writes the full labelled details of one video.
func printVideoDetails(video *model.Video) {
	isShort := "no"
	if video.IsShort {
		isShort = "yes"
	}
	fmt.Fprintf(os.Stdout, "ID: %d\n", video.ID)
	fmt.Fprintf(os.Stdout, "YT ID: %s\n", video.YTVideoID)
	fmt.Fprintf(os.Stdout, "Channel ID: %d\n", video.ChannelID)
	fmt.Fprintf(os.Stdout, "Title: %s\n", video.Title)
	fmt.Fprintf(os.Stdout, "Description: %s\n", video.Description)
	fmt.Fprintf(os.Stdout, "Visibility: %s\n", video.Visibility.String())
	fmt.Fprintf(os.Stdout, "Short: %s\n", isShort)
	fmt.Fprintf(os.Stdout, "Published: %s\n", formatTime(video.PublishedAt))
	fmt.Fprintf(os.Stdout, "PublishAt: %s\n", formatTime(video.PublishAt))
	fmt.Fprintf(os.Stdout, "Views: %d\n", video.ViewCount)
	fmt.Fprintf(os.Stdout, "Likes: %d\n", video.LikeCount)
	fmt.Fprintf(os.Stdout, "Comments: %d\n", video.CommentCount)
	fmt.Fprintf(os.Stdout, "Duration: %d seconds\n", video.DurationSeconds)
	fmt.Fprintf(os.Stdout, "Last synced: %s\n", formatTime(video.LastSyncedAt))
}

// videoEditOptions holds the parsed, validated edit flags together
// with which fields were explicitly set.
type videoEditOptions struct {
	id             uint
	hasTitle       bool
	title          string
	hasDescription bool
	description    string
	hasVisibility  bool
	visibility     model.Visibility
	visibilityRaw  string
	hasPublishAt   bool
	publishAt      time.Time
	publishAtRaw   string
	apply          bool
	yes            bool
}

// runVideoEdit handles "yt-manage video edit ...".
func runVideoEdit(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(videoEditUsage)
	}
	opts, errCode := parseVideoEditFlags(args)
	if errCode != 0 {
		return errCode
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video edit: %v\n", err)
		return 1
	}
	defer dbs.Close()
	video, err := store.Videos.GetByID(opts.id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video edit: %v\n", err)
		return 1
	}
	summary := videoEditSummary(video, opts)
	if !opts.apply {
		fmt.Fprintln(os.Stdout, summary)
		fmt.Fprintln(os.Stdout, "Dry run — no changes made. Re-run with --apply to update the video on YouTube.")
		return 0
	}
	if !opts.yes {
		if !confirmChangeSet(os.Stdin, os.Stdout, summary) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	}
	if err := applyVideoEdit(store, video, opts); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage video edit: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Updated video %d (%s).\n", video.ID, video.YTVideoID)
	return 0
}

// parseVideoEditFlags parses and validates the edit flags. It
// returns a non-zero exit code when parsing or validation fails.
func parseVideoEditFlags(args []string) (*videoEditOptions, int) {
	fs := flag.NewFlagSet("video edit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.Uint("id", 0, "local video ID")
	title := fs.String("title", "", "new title")
	description := fs.String("description", "", "new description")
	visibility := fs.String("visibility", "", "new visibility")
	publishAt := fs.String("publish-at", "", "scheduled publish time (RFC3339)")
	apply := fs.Bool("apply", false, "apply the change")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	opts := &videoEditOptions{
		id:             *id,
		hasTitle:       set["title"],
		title:          *title,
		hasDescription: set["description"],
		description:    *description,
		hasVisibility:  set["visibility"],
		visibilityRaw:  *visibility,
		hasPublishAt:   set["publish-at"],
		publishAtRaw:   *publishAt,
		apply:          *apply,
		yes:            *yes,
	}
	if opts.id == 0 {
		fmt.Fprint(os.Stderr, videoEditUsage)
		return nil, 2
	}
	hasField := opts.hasTitle || opts.hasDescription || opts.hasVisibility || opts.hasPublishAt
	if !hasField {
		fmt.Fprint(os.Stderr, videoEditUsage)
		return nil, 2
	}
	if opts.hasVisibility {
		parsed, err := model.ParseVisibility(opts.visibilityRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage video edit: %v\n", err)
			return nil, 2
		}
		opts.visibility = parsed
		if parsed == model.VisibilityScheduled && !opts.hasPublishAt {
			fmt.Fprintln(os.Stderr, "yt-manage video edit: --visibility scheduled requires --publish-at")
			return nil, 2
		}
	}
	if opts.hasPublishAt {
		parsed, err := time.Parse(time.RFC3339, opts.publishAtRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage video edit: invalid --publish-at: %v\n", err)
			return nil, 2
		}
		opts.publishAt = parsed
	}
	return opts, 0
}

// videoEditSummary builds the human-readable old -> new summary
// covering only the fields the flags set.
func videoEditSummary(video *model.Video, opts *videoEditOptions) string {
	lines := []string{}
	if opts.hasTitle {
		lines = append(lines, fmt.Sprintf("Title: %s -> %s", video.Title, opts.title))
	}
	if opts.hasDescription {
		lines = append(lines, fmt.Sprintf("Description: %s -> %s", video.Description, opts.description))
	}
	if opts.hasVisibility {
		lines = append(lines, fmt.Sprintf("Visibility: %s -> %s", video.Visibility.String(), opts.visibility.String()))
	}
	if opts.hasPublishAt {
		lines = append(lines, fmt.Sprintf("PublishAt: %s -> %s", formatTime(video.PublishAt), opts.publishAt.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

// applyVideoEdit writes the edit to the YouTube API first, then
// updates the mirror rows.
func applyVideoEdit(store *repo.Store, video *model.Video, opts *videoEditOptions) error {
	home, err := config.HomeDir()
	if err != nil {
		return err
	}
	channels, err := store.Channels.List()
	if err != nil {
		return err
	}
	accountByChannel := make(map[uint]uint, len(channels))
	for _, channel := range channels {
		accountByChannel[channel.ID] = channel.AccountID
	}
	accountID, hasAccount := accountByChannel[video.ChannelID]
	if !hasAccount {
		return fmt.Errorf("no account found for channel %d", video.ChannelID)
	}
	key, err := cryptox.LoadKey(home)
	if err != nil {
		return err
	}
	ctx := context.Background()
	svc, err := ytapi.ServiceForAccount(ctx, home, store, key, accountID)
	if err != nil {
		return err
	}
	full, err := ytapi.FetchVideo(ctx, svc, video.YTVideoID)
	if err != nil {
		return err
	}
	if full.Snippet == nil {
		full.Snippet = &youtube.VideoSnippet{}
	}
	if full.Status == nil {
		full.Status = &youtube.VideoStatus{}
	}
	if opts.hasTitle {
		full.Snippet.Title = opts.title
	}
	if opts.hasDescription {
		full.Snippet.Description = opts.description
	}
	if opts.hasPublishAt {
		full.Status.PrivacyStatus = "private"
		full.Status.PublishAt = opts.publishAt.Format(time.RFC3339)
	} else if opts.hasVisibility {
		if opts.visibility == model.VisibilityPublic || opts.visibility == model.VisibilityUnlisted || opts.visibility == model.VisibilityPrivate {
			full.Status.PrivacyStatus = opts.visibility.String()
			full.Status.PublishAt = ""
		}
	}
	if _, err := svc.Videos.Update([]string{"snippet", "status"}, full).Context(ctx).Do(); err != nil {
		return err
	}
	if opts.hasTitle || opts.hasDescription {
		newTitle := video.Title
		if opts.hasTitle {
			newTitle = opts.title
		}
		newDescription := video.Description
		if opts.hasDescription {
			newDescription = opts.description
		}
		if err := store.Videos.UpdateTitleDescription(video.ID, newTitle, newDescription); err != nil {
			return err
		}
	}
	if opts.hasVisibility || opts.hasPublishAt {
		var publishAtPtr *time.Time
		resultingVisibility := opts.visibility
		if opts.hasPublishAt {
			publishAtPtr = &opts.publishAt
			resultingVisibility = model.VisibilityScheduled
		}
		if err := store.Videos.UpdateSchedule(video.ID, publishAtPtr, resultingVisibility); err != nil {
			return err
		}
	}
	return nil
}
