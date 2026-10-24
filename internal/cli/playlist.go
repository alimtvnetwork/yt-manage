package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

const playlistUsage = `Usage: yt-manage playlist <subcommand> [arguments]

Create and edit playlists on a connected channel.

Subcommands:
  list     List mirrored playlists for a channel
  create   Create a playlist (--channel, --title, --description)
  edit     Edit a playlist's title or description
`

const playlistListUsage = `Usage: yt-manage playlist list --channel ID

Arguments:
  --channel ID  Local channel ID (required)
`

const playlistCreateUsage = `Usage: yt-manage playlist create --channel ID --title TITLE [--description TEXT] [--privacy V]

Arguments:
  --channel ID      Local channel ID (required)
  --title TITLE     Playlist title (required)
  --description TEXT Playlist description
  --privacy V       Privacy status: public, unlisted or private
                    (default private)
`

const playlistEditUsage = `Usage: yt-manage playlist edit (--id ID | --yt-id YTID) [--title TITLE] [--description TEXT] [--apply [--yes]]

Arguments:
  --id ID            Local playlist ID
  --yt-id YTID       YouTube playlist ID (exactly one of --id / --yt-id)
  --title TITLE      New title
  --description TEXT New description (at least one of --title /
                     --description is required)
  --apply            Apply the change to YouTube
  --yes              Skip the interactive confirmation
`

// runPlaylist handles "yt-manage playlist ...".
func runPlaylist(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(playlistUsage)
	}
	switch args[0] {
	case "list":
		return runPlaylistList(args[1:])
	case "create":
		return runPlaylistCreate(args[1:])
	case "edit":
		return runPlaylistEdit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage playlist: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, playlistUsage)
		return 2
	}
}

// runPlaylistList handles "yt-manage playlist list ...".
func runPlaylistList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(playlistListUsage)
	}
	wantJSON := hasJSONFlag(args)
	args = stripJSONFlag(args)
	fs := flag.NewFlagSet("playlist list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *channelID == 0 {
		fmt.Fprint(os.Stderr, playlistListUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist list: %v\n", err)
		return 1
	}
	defer dbs.Close()
	playlists, err := store.Playlists.ListByChannel(*channelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist list: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(playlists))
	for _, playlist := range playlists {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(playlist.ID), 10),
			playlist.YTPlaylistID,
			playlist.Title,
			strconv.FormatInt(playlist.ItemCount, 10),
		})
	}
	if wantJSON {
		return printJSON(playlists)
	}
	printTable(os.Stdout, []string{"ID", "YT ID", "Title", "Items"}, rows)
	return 0
}

// runPlaylistCreate handles "yt-manage playlist create ...".
func runPlaylistCreate(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(playlistCreateUsage)
	}
	fs := flag.NewFlagSet("playlist create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	title := fs.String("title", "", "playlist title")
	description := fs.String("description", "", "playlist description")
	privacy := fs.String("privacy", "private", "privacy status")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *channelID == 0 || *title == "" {
		fmt.Fprint(os.Stderr, playlistCreateUsage)
		return 2
	}
	visibility, err := model.ParseVisibility(*privacy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 2
	}
	if visibility != model.VisibilityPublic && visibility != model.VisibilityUnlisted && visibility != model.VisibilityPrivate {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: invalid privacy %q: must be public, unlisted or private\n", *privacy)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 1
	}
	defer dbs.Close()
	channel, err := findLocalChannel(store, *channelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 1
	}
	svc, err := ytServiceForLocalChannel(store, channel.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 1
	}
	ctx := context.Background()
	created, err := svc.Playlists.Insert([]string{"snippet", "status"}, &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title:       *title,
			Description: *description,
			ChannelId:   channel.YTChannelID,
		},
		Status: &youtube.PlaylistStatus{PrivacyStatus: visibility.String()},
	}).Context(ctx).Do()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 1
	}
	record := &model.Playlist{
		ChannelID:    channel.ID,
		YTPlaylistID: created.Id,
		Title:        *title,
		Description:  *description,
		ItemCount:    0,
	}
	if created.Snippet != nil {
		record.Title = created.Snippet.Title
		record.Description = created.Snippet.Description
	}
	if err := store.Playlists.UpsertByYTID(record); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist create: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Created playlist %s.\n", created.Id)
	return 0
}

// findLocalChannel returns the channel with the given local ID by
// scanning the mirrored channels.
func findLocalChannel(store *repo.Store, channelID uint) (*model.Channel, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		if channel.ID == channelID {
			found := channel
			return &found, nil
		}
	}
	return nil, fmt.Errorf("channel not found: %d", channelID)
}

// findPlaylistByLocalID returns the playlist with the given local
// ID by scanning every channel's playlists (the repository has no
// GetByID for playlists).
func findPlaylistByLocalID(store *repo.Store, playlistID uint) (*model.Playlist, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		playlists, err := store.Playlists.ListByChannel(channel.ID)
		if err != nil {
			return nil, err
		}
		for _, playlist := range playlists {
			if playlist.ID == playlistID {
				found := playlist
				return &found, nil
			}
		}
	}
	return nil, fmt.Errorf("playlist not found: %d", playlistID)
}

// playlistEditOptions holds the parsed, validated edit flags.
type playlistEditOptions struct {
	id             uint
	ytID           string
	hasTitle       bool
	title          string
	hasDescription bool
	description    string
	apply          bool
	yes            bool
}

// parsePlaylistEditFlags parses and validates the edit flags. On
// success it returns the options and -1; on failure or help it
// returns nil options and the exit code the caller should return
// (0 for help).
func parsePlaylistEditFlags(args []string) (*playlistEditOptions, int) {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, playlistEditUsage)
		return nil, 0
	}
	fs := flag.NewFlagSet("playlist edit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.Uint("id", 0, "local playlist ID")
	ytID := fs.String("yt-id", "", "YouTube playlist ID")
	title := fs.String("title", "", "new title")
	description := fs.String("description", "", "new description")
	apply := fs.Bool("apply", false, "apply the change")
	yes := fs.Bool("yes", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	opts := &playlistEditOptions{
		id:             *id,
		ytID:           *ytID,
		hasTitle:       set["title"],
		title:          *title,
		hasDescription: set["description"],
		description:    *description,
		apply:          *apply,
		yes:            *yes,
	}
	hasID := set["id"]
	hasYTID := set["yt-id"] && *ytID != ""
	if hasID == hasYTID {
		fmt.Fprintf(os.Stderr, "yt-manage playlist edit: exactly one of --id and --yt-id is required\n")
		return nil, 2
	}
	if !opts.hasTitle && !opts.hasDescription {
		fmt.Fprintf(os.Stderr, "yt-manage playlist edit: at least one of --title and --description is required\n")
		return nil, 2
	}
	return opts, -1
}

// runPlaylistEdit handles "yt-manage playlist edit ...".
func runPlaylistEdit(args []string) int {
	opts, code := parsePlaylistEditFlags(args)
	if opts == nil {
		if code == 0 {
			return 0
		}
		return code
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist edit: %v\n", err)
		return 1
	}
	defer dbs.Close()
	var playlist *model.Playlist
	if opts.ytID != "" {
		playlist, err = store.Playlists.GetByYTID(opts.ytID)
	} else {
		playlist, err = findPlaylistByLocalID(store, opts.id)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist edit: %v\n", err)
		return 1
	}
	summary := playlistEditSummary(playlist, opts)
	if !opts.apply {
		fmt.Fprintln(os.Stdout, summary)
		fmt.Fprintln(os.Stdout, "Dry run — no changes made. Re-run with --apply to update the playlist on YouTube.")
		return 0
	}
	if !opts.yes {
		if !confirmChangeSet(os.Stdin, os.Stdout, summary) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	}
	if err := applyPlaylistEdit(store, playlist, opts); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage playlist edit: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Updated playlist %s.\n", playlist.YTPlaylistID)
	return 0
}

// playlistEditSummary builds the old -> new summary covering only
// the fields the flags set.
func playlistEditSummary(playlist *model.Playlist, opts *playlistEditOptions) string {
	lines := []string{}
	if opts.hasTitle {
		lines = append(lines, fmt.Sprintf("Title: %s -> %s", playlist.Title, opts.title))
	}
	if opts.hasDescription {
		lines = append(lines, fmt.Sprintf("Description: %s -> %s", playlist.Description, opts.description))
	}
	return strings.Join(lines, "\n")
}

// applyPlaylistEdit writes the edit to the YouTube API first, then
// updates the mirror row preserving its item count.
func applyPlaylistEdit(store *repo.Store, playlist *model.Playlist, opts *playlistEditOptions) error {
	svc, err := ytServiceForLocalChannel(store, playlist.ChannelID)
	if err != nil {
		return err
	}
	ctx := context.Background()
	resp, err := svc.Playlists.List([]string{"snippet", "status"}).Id(playlist.YTPlaylistID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("playlists.list %s: %w", playlist.YTPlaylistID, err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("playlists.list %s: no playlist returned", playlist.YTPlaylistID)
	}
	full := resp.Items[0]
	if full.Snippet == nil {
		full.Snippet = &youtube.PlaylistSnippet{}
	}
	if full.Status == nil {
		full.Status = &youtube.PlaylistStatus{}
	}
	newTitle := playlist.Title
	if opts.hasTitle {
		full.Snippet.Title = opts.title
		newTitle = opts.title
	}
	newDescription := playlist.Description
	if opts.hasDescription {
		full.Snippet.Description = opts.description
		newDescription = opts.description
	}
	if _, err := svc.Playlists.Update([]string{"snippet", "status"}, full).Context(ctx).Do(); err != nil {
		return fmt.Errorf("playlists.update %s: %w", playlist.YTPlaylistID, err)
	}
	record := &model.Playlist{
		ChannelID:    playlist.ChannelID,
		YTPlaylistID: playlist.YTPlaylistID,
		Title:        newTitle,
		Description:  newDescription,
		ItemCount:    playlist.ItemCount,
	}
	if err := store.Playlists.UpsertByYTID(record); err != nil {
		return fmt.Errorf("mirror playlist %s: %w", playlist.YTPlaylistID, err)
	}
	return nil
}
