package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/updater"
)

const updateUsage = `Usage: yt-manage update <subcommand> [arguments]

Self-update yt-manage from GitHub releases. Downloads verify their
SHA256 against the release checksums.txt before the binary is
swapped; release tarballs ship the plain binary name (yt-manage).

Subcommands:
  check    Report whether a newer release is available
  apply    Download, verify and install the latest release
           (--yes skips the confirmation)
`

// runUpdate handles "yt-manage update ...".
func runUpdate(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(updateUsage)
	}
	switch args[0] {
	case "check":
		return runUpdateCheck(args[1:])
	case "apply":
		return runUpdateApply(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage update: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, updateUsage)
		return 2
	}
}

// runUpdateCheck handles "yt-manage update check ...".
func runUpdateCheck(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(updateUsage)
	}
	current, latest, isUpdateAvailable, err := updater.Check(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage update check: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "current: %s\n", current)
	fmt.Fprintf(os.Stdout, "latest: %s\n", latest)
	if isUpdateAvailable {
		fmt.Fprintln(os.Stdout, "update available")
	} else {
		fmt.Fprintln(os.Stdout, "up to date")
	}
	return 0
}

// runUpdateApply handles "yt-manage update apply ...".
func runUpdateApply(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(updateUsage)
	}
	fs := flag.NewFlagSet("update apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ctx := context.Background()
	current, latest, isUpdateAvailable, err := updater.Check(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage update apply: %v\n", err)
		return 1
	}
	if !isUpdateAvailable {
		fmt.Fprintf(os.Stdout, "current: %s\n", current)
		fmt.Fprintf(os.Stdout, "latest: %s\n", latest)
		fmt.Fprintln(os.Stdout, "up to date")
		return 0
	}
	plan := fmt.Sprintf("Update yt-manage %s -> %s from GitHub releases (checksum-verified)", current, latest)
	if !*yes {
		if !confirmChangeSet(os.Stdin, os.Stdout, plan) {
			fmt.Fprintln(os.Stdout, "Aborted.")
			return 0
		}
	} else {
		fmt.Fprintln(os.Stdout, plan)
	}
	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage update apply: %v\n", err)
		return 1
	}
	summary, err := updater.Apply(ctx, home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage update apply: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, summary)
	return 0
}
