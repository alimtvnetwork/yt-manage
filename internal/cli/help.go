package cli

import (
	"fmt"
	"io"
)

// printHelp writes the top-level help text listing every command
// group with a one-line description.
func printHelp(w io.Writer) {
	fmt.Fprint(w, `yt-manage - multi-account YouTube management CLI

Usage:
  yt-manage <command> [arguments]
  yt-manage help

Commands:
  account   Connect and manage YouTube accounts (OAuth)
  sync      Mirror channels, videos, playlists and comments into the local database
  video     List and inspect mirrored videos
  search    Search the local mirror by title or description (no API quota)
  rename    Batch rename or edit videos by search, with confirmation
  episode   Group video parts into episodes and sequence them
  schedule  Apply cadence scheduling rules to episodes
  playlist  Create and edit playlists
  comment   Post and edit the first comment on a video
  mail      Send and read email around publish events
  serve     Start the local REST API and browser UI
  peer      Sync the local cache with another yt-manage instance
  llm       Export training data for LLMs
  settings  Read and change settings (same names as the UI Settings screen)
  update    Check for and apply yt-manage updates
  version   Print the yt-manage version

Run "yt-manage <command> --help" for a command's subcommands.
Data lives in ~/.yt-manage (override with YT_MANAGE_HOME).

Getting started (the same 4 steps the browser UI wizard walks through):
  1. yt-manage account add   Connect a YouTube account (OAuth consent;
                             tokens expire after 7 days while your Google
                             consent screen is in Testing mode - publish it)
  2. yt-manage sync run      Mirror your channels into the local database
  3. yt-manage search <q>    Search the local mirror at no API quota cost
  4. yt-manage serve         Open the browser UI (wizard, checklist, search)
`)
}
