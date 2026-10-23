package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/searchx"
)

const searchUsage = `Usage: yt-manage search [--channel ID] [--visibility V] [--limit N] [--offset N] <query words...>

Search the local mirror (FTS5 over video titles and descriptions).
Search never calls the YouTube API, so it costs zero quota — unlike
search.list, which costs 100 units per call.

Arguments:
  <query>        Words to match against titles and descriptions of
                 mirrored videos
  --channel ID   Only videos on the local channel with this ID
  --visibility V Only videos with this visibility
                 (public, unlisted, private, scheduled, draft)
  --limit N      Maximum number of results (default 50, max 500)
  --offset N     Number of results to skip
`

// runSearch handles "yt-manage search ...".
func runSearch(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(searchUsage)
	}
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	visibility := fs.String("visibility", "", "visibility filter")
	limit := fs.Int("limit", 50, "maximum number of results")
	offset := fs.Int("offset", 0, "number of results to skip")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *visibility != "" {
		if _, err := model.ParseVisibility(*visibility); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage search: %v\n", err)
			return 2
		}
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage search: %v\n", err)
		return 1
	}
	defer dbs.Close()
	videos, err := searchx.Search(store, searchx.Query{
		Text:       strings.Join(fs.Args(), " "),
		ChannelID:  *channelID,
		Visibility: *visibility,
		Limit:      *limit,
		Offset:     *offset,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage search: %v\n", err)
		return 1
	}
	printVideoTable(os.Stdout, videos)
	fmt.Fprintf(os.Stdout, "%d result(s) from the local mirror (0 API units)\n", len(videos))
	return 0
}

// printVideoTable writes videos as the shared video table used by
// search and video list.
func printVideoTable(w io.Writer, videos []model.Video) {
	rows := make([][]string, 0, len(videos))
	for _, video := range videos {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(video.ID), 10),
			video.YTVideoID,
			video.Title,
			video.Visibility.String(),
			strconv.FormatInt(video.ViewCount, 10),
			formatDate(video.PublishedAt),
		})
	}
	printTable(w, []string{"ID", "YT ID", "Title", "Visibility", "Views", "Published"}, rows)
}

// formatDate formats a nullable time as a calendar date, or "-"
// when it is nil.
func formatDate(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}

// formatTime formats a nullable time as RFC3339, or "-" when it
// is nil.
func formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format(time.RFC3339)
}
