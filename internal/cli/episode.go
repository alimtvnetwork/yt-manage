package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

const episodeUsage = `Usage: yt-manage episode <subcommand> [arguments]

Group the parts of an episode together across inconsistent titles
("EP3", "EP 03 - Part 2", "ep03 part three" are one episode). A video
with an episode number and no part marker is part 1. Parts are
sequenced by part number, falling back to upload order.

Subcommands:
  group    Parse titles and materialise episode groups and parts
  list     List episodes with their parts in sequence order
  show     Show one episode and its parts
`

const episodeGroupUsage = `Usage: yt-manage episode group [--channel ID]

Arguments:
  --channel ID   Only group this local channel (default: all
                 channels)
`

const episodeListUsage = `Usage: yt-manage episode list --channel ID

Arguments:
  --channel ID   Local channel ID (required)
`

const episodeShowUsage = `Usage: yt-manage episode show [--channel ID] <episode-key>

Arguments:
  <episode-key>  The episode key, e.g. "robo school|ep3"
  --channel ID   Only search this local channel (default: search
                 every channel)
`

// runEpisode handles "yt-manage episode ...".
func runEpisode(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(episodeUsage)
	}
	switch args[0] {
	case "group":
		return runEpisodeGroup(args[1:])
	case "list":
		return runEpisodeList(args[1:])
	case "show":
		return runEpisodeShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage episode: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, episodeUsage)
		return 2
	}
}

// runEpisodeGroup handles "yt-manage episode group ...": it parses
// and materialises the episode groups of one channel, or of every
// channel when --channel is 0. A channel that fails is reported on
// stderr and sets the exit code, but the remaining channels are
// still processed.
func runEpisodeGroup(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(episodeGroupUsage)
	}
	fs := flag.NewFlagSet("episode group", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID (0 = all channels)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode group: %v\n", err)
		return 1
	}
	defer dbs.Close()
	channelIDs, err := episodeGroupChannels(store, *channelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode group: %v\n", err)
		return 1
	}
	exitCode := 0
	for _, id := range channelIDs {
		result, err := episodes.GroupChannel(store, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage episode group: channel %d: %v\n", id, err)
			exitCode = 1
			continue
		}
		printEpisodeGroupResult(id, result)
	}
	return exitCode
}

// episodeGroupChannels resolves the --channel flag to the channel
// IDs to group: the flag itself when set, otherwise every channel.
func episodeGroupChannels(store *repo.Store, channelID uint) ([]uint, error) {
	if channelID != 0 {
		return []uint{channelID}, nil
	}
	channels, err := store.Channels.List()
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}
	return ids, nil
}

// printEpisodeGroupResult prints one channel's grouping outcome:
// a table of episodes, then the unparsed titles.
func printEpisodeGroupResult(channelID uint, result *episodes.GroupResult) {
	fmt.Fprintf(os.Stdout, "Channel %d:\n", channelID)
	rows := make([][]string, 0, len(result.Groups))
	for _, group := range result.Groups {
		warnings := "-"
		if len(group.Warnings) > 0 {
			warnings = strings.Join(group.Warnings, "; ")
		}
		rows = append(rows, []string{
			episodeDisplayName(group),
			group.EpisodeKey,
			strconv.Itoa(len(group.Parts)),
			warnings,
		})
	}
	printTable(os.Stdout, []string{"Episode", "Key", "Parts", "Warnings"}, rows)
	if len(result.Unparsed) > 0 {
		fmt.Fprintln(os.Stdout, "Unparsed (no episode number found):")
		for _, video := range result.Unparsed {
			fmt.Fprintf(os.Stdout, "  - %s (%s)\n", video.Title, video.YTVideoID)
		}
	}
}

// episodeDisplayName labels a group by its display title, falling
// back to the episode number when the title carried no series name.
func episodeDisplayName(group episodes.EpisodeGroup) string {
	if group.Title != "" {
		return group.Title
	}
	return fmt.Sprintf("Episode %d", group.EpisodeNumber)
}

// runEpisodeList handles "yt-manage episode list ...".
func runEpisodeList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(episodeListUsage)
	}
	wantJSON := hasJSONFlag(args)
	args = stripJSONFlag(args)
	fs := flag.NewFlagSet("episode list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *channelID == 0 {
		fmt.Fprint(os.Stderr, episodeListUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode list: %v\n", err)
		return 1
	}
	defer dbs.Close()
	episodeList, err := store.Episodes.ListByChannel(*channelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode list: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(episodeList))
	for _, episode := range episodeList {
		parts, err := store.Episodes.ListParts(episode.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage episode list: %v\n", err)
			return 1
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(episode.ID), 10),
			episode.EpisodeKey,
			strconv.Itoa(episode.EpisodeNumber),
			episode.Title,
			strconv.Itoa(len(parts)),
		})
	}
	if wantJSON {
		return printJSON(episodeList)
	}
	printTable(os.Stdout, []string{"ID", "Key", "Number", "Title", "Parts"}, rows)
	return 0
}

// runEpisodeShow handles "yt-manage episode show ...".
func runEpisodeShow(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(episodeShowUsage)
	}
	fs := flag.NewFlagSet("episode show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	channelID := fs.Uint("channel", 0, "local channel ID (0 = search all channels)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 1 {
		fmt.Fprint(os.Stderr, episodeShowUsage)
		return 2
	}
	key := fs.Args()[0]
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode show: %v\n", err)
		return 1
	}
	defer dbs.Close()
	episode := findEpisode(store, *channelID, key)
	if episode == nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode show: episode not found: %s\n", key)
		return 1
	}
	return printEpisodeDetails(store, episode)
}

// findEpisode resolves an episode key to a stored episode: on the
// given channel when channelID is set, otherwise on the first
// channel that has the key. It returns nil when nothing matches.
func findEpisode(store *repo.Store, channelID uint, key string) *model.Episode {
	if channelID != 0 {
		episode, err := store.Episodes.GetByKey(channelID, key)
		if err != nil {
			return nil
		}
		return episode
	}
	channels, err := store.Channels.List()
	if err != nil {
		return nil
	}
	for _, channel := range channels {
		if episode, err := store.Episodes.GetByKey(channel.ID, key); err == nil {
			return episode
		}
	}
	return nil
}

// printEpisodeDetails writes the episode's fields and its parts
// table, resolving each part's video through the mirror.
func printEpisodeDetails(store *repo.Store, episode *model.Episode) int {
	fmt.Fprintf(os.Stdout, "ID: %d\n", episode.ID)
	fmt.Fprintf(os.Stdout, "Channel ID: %d\n", episode.ChannelID)
	fmt.Fprintf(os.Stdout, "Key: %s\n", episode.EpisodeKey)
	fmt.Fprintf(os.Stdout, "Number: %d\n", episode.EpisodeNumber)
	fmt.Fprintf(os.Stdout, "Title: %s\n", episode.Title)
	fmt.Fprintf(os.Stdout, "Cadence days: %d\n", episode.CadenceDays)
	fmt.Fprintf(os.Stdout, "First publish: %s\n", formatTime(episode.FirstPublishAt))
	parts, err := store.Episodes.ListParts(episode.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage episode show: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(parts))
	for _, part := range parts {
		video, err := store.Videos.GetByID(part.VideoID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage episode show: %v\n", err)
			return 1
		}
		rows = append(rows, []string{
			strconv.Itoa(part.Sequence),
			strconv.Itoa(part.PartNumber),
			strconv.FormatUint(uint64(video.ID), 10),
			video.YTVideoID,
			video.Title,
		})
	}
	printTable(os.Stdout, []string{"Sequence", "Part", "Video ID", "YT ID", "Title"}, rows)
	return 0
}
