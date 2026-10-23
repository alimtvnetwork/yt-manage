package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"google.golang.org/api/youtube/v3"

	"github.com/alimtvnetwork/yt-manage/internal/comments"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

const commentUsage = `Usage: yt-manage comment <subcommand> [arguments]

Manage the first comment on a video.

Subcommands:
  post     Post the first comment on a video
  edit     Edit a comment previously posted by yt-manage

Pinning: the YouTube Data API has NO endpoint for pinning a comment.
Pinning is a manual step in YouTube Studio — yt-manage posts or edits
the comment and reminds you to pin it there; it never fakes a pin.
`

const commentPostUsage = `Usage: yt-manage comment post --video ID (--text TEXT | --text-file FILE)

Arguments:
  --video ID      Local video ID, or YouTube video ID (required)
  --text TEXT     Comment text (exactly one of --text / --text-file)
  --text-file F   Read the comment text from file F
`

const commentEditUsage = `Usage: yt-manage comment edit --video ID (--text TEXT | --text-file FILE)

Arguments:
  --video ID      Local video ID, or YouTube video ID (required)
  --text TEXT     New comment text (exactly one of --text / --text-file)
  --text-file F   Read the new comment text from file F
`

// runComment handles "yt-manage comment ...".
func runComment(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(commentUsage)
	}
	switch args[0] {
	case "post":
		return runCommentPost(args[1:])
	case "edit":
		return runCommentEdit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage comment: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, commentUsage)
		return 2
	}
}

// commentTextOptions holds the parsed post/edit flags.
type commentTextOptions struct {
	videoRef string
	text     string
}

// parseCommentTextFlags parses the shared post/edit flags and
// resolves the comment text, requiring exactly one of --text and
// --text-file. On success it returns the options and -1; on failure
// or help it returns nil options and the exit code to return
// (0 for help).
func parseCommentTextFlags(name, usage string, args []string) (*commentTextOptions, int) {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, usage)
		return nil, 0
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	videoRef := fs.String("video", "", "local video ID or YouTube video ID")
	text := fs.String("text", "", "comment text")
	textFile := fs.String("text-file", "", "file holding the comment text")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	if *videoRef == "" {
		fmt.Fprint(os.Stderr, usage)
		return nil, 2
	}
	hasText := set["text"]
	hasTextFile := set["text-file"]
	if hasText == hasTextFile {
		fmt.Fprintf(os.Stderr, "yt-manage %s: exactly one of --text and --text-file is required\n", name)
		return nil, 2
	}
	opts := &commentTextOptions{videoRef: *videoRef, text: *text}
	if hasTextFile {
		data, err := os.ReadFile(*textFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage %s: read text file: %v\n", name, err)
			return nil, 1
		}
		opts.text = string(data)
	}
	return opts, -1
}

// resolveCommentVideo resolves a --video value: a numeric value is
// a local ID (Videos.GetByID); anything else is a YouTube video ID
// (Videos.GetByYTID).
func resolveCommentVideo(store *repo.Store, ref string) (*model.Video, error) {
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		video, err := store.Videos.GetByID(uint(id))
		if err != nil {
			return nil, fmt.Errorf("video not found: %s", ref)
		}
		return video, nil
	}
	video, err := store.Videos.GetByYTID(ref)
	if err != nil {
		return nil, fmt.Errorf("video not found: %s", ref)
	}
	return video, nil
}

// ytServiceForLocalChannel resolves the account owning a local
// channel and builds its YouTube service.
func ytServiceForLocalChannel(store *repo.Store, channelID uint) (*youtube.Service, error) {
	channels, err := store.Channels.List()
	if err != nil {
		return nil, err
	}
	accountByChannel := make(map[uint]uint, len(channels))
	for _, channel := range channels {
		accountByChannel[channel.ID] = channel.AccountID
	}
	accountID, hasAccount := accountByChannel[channelID]
	if !hasAccount {
		return nil, fmt.Errorf("no account found for channel %d", channelID)
	}
	home, err := config.HomeDir()
	if err != nil {
		return nil, err
	}
	key, err := cryptox.LoadKey(home)
	if err != nil {
		return nil, err
	}
	return ytapi.ServiceForAccount(context.Background(), home, store, key, accountID)
}

// runCommentPost handles "yt-manage comment post ...".
func runCommentPost(args []string) int {
	opts, code := parseCommentTextFlags("comment post", commentPostUsage, args)
	if opts == nil {
		if code == 0 {
			return 0
		}
		return code
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment post: %v\n", err)
		return 1
	}
	defer dbs.Close()
	video, err := resolveCommentVideo(store, opts.videoRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment post: %v\n", err)
		return 1
	}
	svc, err := ytServiceForLocalChannel(store, video.ChannelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment post: %v\n", err)
		return 1
	}
	posted, err := comments.PostFirstComment(context.Background(), svc, store, *video, opts.text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment post: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Posted first comment %s on %s.\n", posted.YTCommentID, video.Title)
	fmt.Fprintln(os.Stdout, comments.PinGuidance(video.YTVideoID))
	return 0
}

// runCommentEdit handles "yt-manage comment edit ...".
func runCommentEdit(args []string) int {
	opts, code := parseCommentTextFlags("comment edit", commentEditUsage, args)
	if opts == nil {
		if code == 0 {
			return 0
		}
		return code
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment edit: %v\n", err)
		return 1
	}
	defer dbs.Close()
	video, err := resolveCommentVideo(store, opts.videoRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment edit: %v\n", err)
		return 1
	}
	svc, err := ytServiceForLocalChannel(store, video.ChannelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment edit: %v\n", err)
		return 1
	}
	if _, err := comments.EditFirstComment(context.Background(), svc, store, *video, opts.text); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage comment edit: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Updated first comment on %s.\n", video.Title)
	fmt.Fprintln(os.Stdout, comments.PinGuidance(video.YTVideoID))
	return 0
}
