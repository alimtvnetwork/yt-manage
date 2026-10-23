package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/alimtvnetwork/yt-manage/internal/apiserver"
	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
)

const serveUsage = `Usage: yt-manage serve [--addr ADDR]

Start the local REST API and browser UI (default address :8765, set
by server_addr in config.json). The API exposes videos, search,
episodes, playlists, A/B tests, accounts and sync; bulk edits require
a change-set confirmation token, exactly as in the CLI.

Arguments:
  --addr ADDR   Listen address (default: server_addr from
                config.json, built-in default :8765)
`

// runServe handles "yt-manage serve ...".
func runServe(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(serveUsage)
	}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", "", "listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage serve: %v\n", err)
		return 1
	}
	if *addr == "" {
		settings, err := config.LoadSettings(home)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage serve: %v\n", err)
			return 1
		}
		*addr = settings.ServerAddr
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage serve: %v\n", err)
		return 1
	}
	defer dbs.Close()
	key, err := cryptox.LoadKey(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage serve: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "yt-manage API + UI on http://%s (data dir %s)\n", *addr, home)
	if err := apiserver.New(store, home, key).ListenAndServe(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage serve: %v\n", err)
		return 1
	}
	return 0
}
