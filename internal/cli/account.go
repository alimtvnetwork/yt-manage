package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/oauth"
	"github.com/alimtvnetwork/yt-manage/internal/ytapi"
)

const accountUsage = `Usage: yt-manage account <subcommand> [arguments]

Connect and manage YouTube accounts. Each account is authorised once
through the OAuth consent flow; its refresh token is stored encrypted
(AES-256-GCM) in the local accounts database.

Subcommands:
  add      Connect a new account through the OAuth consent flow
           (--label sets the local label for the account)
  list     List connected accounts and their channels
  remove   Disconnect an account and delete its stored token
           (id as argument or --id; --yes skips the confirmation)

The Google OAuth client is read from the YT_MANAGE_CLIENT_ID and
YT_MANAGE_CLIENT_SECRET environment variables, or from
client_secret.json in the yt-manage home directory (~/.yt-manage).
`

// runAccount handles "yt-manage account ...".
func runAccount(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(accountUsage)
	}
	switch args[0] {
	case "add":
		return runAccountAdd(args[1:])
	case "list":
		return runAccountList(args[1:])
	case "remove":
		return runAccountRemove(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage: unknown account subcommand %q\n\n", args[0])
		fmt.Fprint(os.Stderr, accountUsage)
		return 2
	}
}

// runAccountAdd runs the OAuth consent flow and stores the resulting
// refresh token encrypted under a new or existing account row.
func runAccountAdd(args []string) int {
	fs := flag.NewFlagSet("account add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	label := fs.String("label", "", "local label for the account")
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
	cfg, err := oauth.LoadClientConfig(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		fmt.Fprintf(os.Stderr, "Set YT_MANAGE_CLIENT_ID and YT_MANAGE_CLIENT_SECRET, or place client_secret.json in the data dir at %s.\n", config.ClientSecretPath(home))
		return 1
	}

	ctx := context.Background()
	token, err := oauth.Authorize(ctx, cfg, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	svc, err := ytapi.NewService(ctx, cfg, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	ch, err := ytapi.MyChannel(ctx, svc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	channelTitle := ""
	channelHandle := ""
	if ch.Snippet != nil {
		channelTitle = ch.Snippet.Title
		channelHandle = ch.Snippet.CustomUrl
	}

	ciphertext, nonce, err := cryptox.Encrypt(key, []byte(token.RefreshToken))
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: encrypt refresh token: %v\n", err)
		return 1
	}

	// GoogleSubject holds the channel ID: the youtube.force-ssl
	// scope yields no userinfo subject, and the channel ID is the
	// stable identity of the connected account.
	var accountID uint
	isReconnect := false
	existing, getErr := store.Accounts.GetByGoogleSubject(ch.Id)
	if getErr == nil && existing != nil {
		accountID = existing.ID
		isReconnect = true
		if err := store.Accounts.SaveToken(existing.ID, ciphertext, nonce); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: save refresh token: %v\n", err)
			return 1
		}
	} else {
		accountLabel := *label
		if accountLabel == "" {
			accountLabel = channelTitle
		}
		account := &model.Account{
			Label:         accountLabel,
			GoogleSubject: ch.Id,
			ChannelID:     ch.Id,
			Scopes:        strings.Join(cfg.Scopes, " "),
		}
		if err := store.Accounts.Create(account); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: create account: %v\n", err)
			return 1
		}
		accountID = account.ID
		if err := store.Accounts.SaveToken(account.ID, ciphertext, nonce); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: save refresh token: %v\n", err)
			return 1
		}
	}

	now := time.Now()
	channel := &model.Channel{
		AccountID:    accountID,
		YTChannelID:  ch.Id,
		Title:        channelTitle,
		Handle:       channelHandle,
		LastSyncedAt: &now,
	}
	if ch.Statistics != nil {
		channel.SubscriberCount = int64(ch.Statistics.SubscriberCount)
		channel.VideoCount = int64(ch.Statistics.VideoCount)
	}
	if err := store.Channels.UpsertByYTID(channel); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: save channel: %v\n", err)
		return 1
	}

	displayLabel := *label
	saved, saveErr := store.Accounts.GetByID(accountID)
	if saveErr == nil && saved != nil {
		displayLabel = saved.Label
	}
	if isReconnect {
		fmt.Fprintf(os.Stdout, "Reconnected account %s (%s)\n", displayLabel, channelTitle)
	} else {
		fmt.Fprintf(os.Stdout, "Connected account %s (%s)\n", displayLabel, channelTitle)
	}
	fmt.Fprintln(os.Stdout, "Refresh token stored AES-GCM encrypted in the local accounts database.")
	return 0
}

// runAccountList prints one row per account and channel.
func runAccountList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(accountUsage)
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
		created := account.CreatedAt.Format("2006-01-02 15:04")
		channels, err := store.Channels.ListByAccount(account.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: list channels for account %d: %v\n", account.ID, err)
			return 1
		}
		if len(channels) == 0 {
			rows = append(rows, []string{
				strconv.FormatUint(uint64(account.ID), 10),
				account.Label,
				"-",
				"-",
				created,
			})
			continue
		}
		for _, channel := range channels {
			rows = append(rows, []string{
				strconv.FormatUint(uint64(account.ID), 10),
				account.Label,
				channel.Title,
				channel.YTChannelID,
				created,
			})
		}
	}
	printTable(os.Stdout, []string{"ID", "Label", "Channel", "Channel ID", "Created"}, rows)
	return 0
}

// runAccountRemove deletes an account row and its stored token after
// confirmation, unless --yes is given. Mirrored content rows are
// kept.
func runAccountRemove(args []string) int {
	fs := flag.NewFlagSet("account remove", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	idFlag := fs.Uint("id", 0, "id of the account to remove")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	accountID := *idFlag
	if rest := fs.Args(); len(rest) > 0 {
		parsed, err := strconv.ParseUint(rest[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: invalid account id %q\n\n", rest[0])
			fmt.Fprint(os.Stderr, accountUsage)
			return 2
		}
		accountID = uint(parsed)
	}
	if accountID == 0 {
		fmt.Fprintf(os.Stderr, "yt-manage: account remove requires an account id (argument or --id)\n\n")
		fmt.Fprint(os.Stderr, accountUsage)
		return 2
	}

	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	defer dbs.Close()

	account, err := store.Accounts.GetByID(accountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: load account %d: %v\n", accountID, err)
		return 1
	}
	if !*yes {
		summary := fmt.Sprintf("Remove account %d (%s) and delete its stored token? Mirrored content rows will be kept.", account.ID, account.Label)
		if !confirmChangeSet(os.Stdin, os.Stdout, summary) {
			fmt.Fprintln(os.Stdout, "Aborted; account not removed.")
			return 0
		}
	}
	if err := store.Accounts.Delete(accountID); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: delete account %d: %v\n", accountID, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Removed account %s (%d): the account and its stored token were removed; mirrored content rows were kept.\n", account.Label, account.ID)
	return 0
}
