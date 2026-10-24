package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/syncer"
)

const syncUsage = `Usage: yt-manage sync <subcommand> [arguments]

Mirror YouTube data into the local split databases. Sync uses
channels.list, playlistItems.list and videos.list (1 quota unit per
call) and refuses a run that would exceed the configured daily quota
budget.

Subcommands:
  run      Run a sync for one account (--account) or all accounts
           (--full forces a full run instead of an incremental one)
  status   Show the last sync time and error per account and resource
`

// runSync handles "yt-manage sync ...".
func runSync(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(syncUsage)
	}
	switch args[0] {
	case "run":
		return runSyncRun(args[1:])
	case "status":
		return runSyncStatus(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage: unknown sync subcommand %q\n\n", args[0])
		fmt.Fprint(os.Stderr, syncUsage)
		return 2
	}
}

// runSyncRun syncs one account (--account) or every account, prints
// a per-account summary table, and returns 1 when any account failed
// (failures are printed to stderr as they happen, never swallowed).
func runSyncRun(args []string) int {
	fs := flag.NewFlagSet("sync run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	accountFlag := fs.Uint("account", 0, "id of the account to sync (0 = all accounts)")
	isFull := fs.Bool("full", false, "run a full sync instead of an incremental one")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	defer dbs.Close()

	key, err := cryptox.LoadKey(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	settings, err := config.LoadSettings(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}

	var targets []model.Account
	*accountFlag = withDefaultAccount(*accountFlag)
	if *accountFlag != 0 {
		account, err := store.Accounts.GetByID(*accountFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: load account %d: %v\n", *accountFlag, err)
			return 1
		}
		targets = []model.Account{*account}
	} else {
		accounts, err := store.Accounts.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: list accounts: %v\n", err)
			return 1
		}
		targets = accounts
	}

	ctx := context.Background()
	hasFailure := false
	var rows [][]string
	for _, target := range targets {
		result, err := syncer.RunAccount(ctx, home, store, key, target.ID, *isFull, settings.DailyQuotaBudget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: sync account %d (%s): %v\n", target.ID, target.Label, err)
			hasFailure = true
			continue
		}
		mode := "full"
		if result.IsIncremental {
			mode = "incremental"
		}
		rows = append(rows, []string{
			target.Label,
			result.ChannelTitle,
			strconv.Itoa(result.Videos),
			strconv.Itoa(result.Playlists),
			strconv.Itoa(result.Comments),
			strconv.Itoa(result.UnitsEstimate),
			mode,
		})
	}
	printTable(os.Stdout, []string{"Account", "Channel", "Videos", "Playlists", "Comments", "Units est.", "Mode"}, rows)
	if hasFailure {
		return 1
	}
	return 0
}

// runSyncStatus prints the stored sync state of every account.
func runSyncStatus(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(syncUsage)
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	defer dbs.Close()

	accounts, err := store.Accounts.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: list accounts: %v\n", err)
		return 1
	}
	var rows [][]string
	for _, account := range accounts {
		states, err := store.SyncStates.ListByAccount(account.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: list sync states for account %d: %v\n", account.ID, err)
			return 1
		}
		for _, state := range states {
			lastRun := "-"
			if state.LastRunAt != nil {
				lastRun = state.LastRunAt.Format("2006-01-02 15:04")
			}
			lastError := state.LastError
			if lastError == "" {
				lastError = "-"
			}
			rows = append(rows, []string{
				account.Label,
				state.Resource,
				lastRun,
				lastError,
			})
		}
	}
	printTable(os.Stdout, []string{"Account", "Resource", "Last run", "Last error"}, rows)
	return 0
}
