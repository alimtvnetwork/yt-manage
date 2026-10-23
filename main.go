// Command yt-manage is a Go CLI for full multi-account YouTube
// management: it mirrors channels into a local split SQLite database
// and performs search, batch rename, episode grouping, cadence
// scheduling, comment automation, and playlist editing.
package main

import (
	"os"

	"github.com/alimtvnetwork/yt-manage/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
