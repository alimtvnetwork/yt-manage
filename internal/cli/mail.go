package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/mailer"
	"github.com/alimtvnetwork/yt-manage/internal/model"
)

const mailUsage = `Usage: yt-manage mail <subcommand> [arguments]

Send and receive email around publish events, using the SMTP/IMAP
settings stored in config.json in the yt-manage home directory.

Subcommands:
  send     Send an email (--to, --subject, --body)
  inbox    List recent inbox messages
  watch    Watch the inbox for publish-event mail
`

const mailSendUsage = `Usage: yt-manage mail send --to ADDR --subject TEXT (--body TEXT | --body-file FILE)

Arguments:
  --to ADDR       Recipient address (required)
  --subject TEXT  Subject line (required)
  --body TEXT     Message body
  --body-file F   Read the message body from file F
`

const mailInboxUsage = `Usage: yt-manage mail inbox [--limit N]

Arguments:
  --limit N  Maximum number of messages to list (default 20)
`

const mailWatchUsage = `Usage: yt-manage mail watch --to ADDR [--interval DURATION]

Arguments:
  --to ADDR        Recipient for publish notifications (required)
  --interval D     Time between watcher passes (default 5m)
`

// runMail handles "yt-manage mail ...".
func runMail(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(mailUsage)
	}
	switch args[0] {
	case "send":
		return runMailSend(args[1:])
	case "inbox":
		return runMailInbox(args[1:])
	case "watch":
		return runMailWatch(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage mail: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, mailUsage)
		return 2
	}
}

// loadMailSettings loads the application settings from the home
// directory.
func loadMailSettings() (*config.Settings, string, error) {
	home, err := config.HomeDir()
	if err != nil {
		return nil, "", err
	}
	settings, err := config.LoadSettings(home)
	if err != nil {
		return nil, "", err
	}
	return settings, home, nil
}

// runMailSend handles "yt-manage mail send ...".
func runMailSend(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(mailSendUsage)
	}
	fs := flag.NewFlagSet("mail send", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	to := fs.String("to", "", "recipient address")
	subject := fs.String("subject", "", "subject line")
	body := fs.String("body", "", "message body")
	bodyFile := fs.String("body-file", "", "file holding the message body")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	if *to == "" || *subject == "" {
		fmt.Fprint(os.Stderr, mailSendUsage)
		return 2
	}
	if set["body"] && set["body-file"] {
		fmt.Fprintln(os.Stderr, "yt-manage mail send: only one of --body and --body-file may be given")
		return 2
	}
	bodyText := *body
	if set["body-file"] {
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage mail send: read body file: %v\n", err)
			return 1
		}
		bodyText = string(data)
	}
	settings, _, err := loadMailSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail send: %v\n", err)
		return 1
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail send: %v\n", err)
		return 1
	}
	defer dbs.Close()
	if err := mailer.Send(*settings, *to, *subject, bodyText); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail send: %v\n", err)
		return 1
	}
	now := time.Now()
	entry := &model.EmailLog{
		Direction:      model.EmailDirectionSent,
		Subject:        *subject,
		RelatedVideoID: 0,
		SentAt:         &now,
	}
	if err := store.EmailLogs.Create(entry); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail send: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "Sent.")
	return 0
}

// runMailInbox handles "yt-manage mail inbox ...".
func runMailInbox(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(mailInboxUsage)
	}
	fs := flag.NewFlagSet("mail inbox", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 20, "maximum number of messages")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	settings, _, err := loadMailSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail inbox: %v\n", err)
		return 1
	}
	messages, err := mailer.FetchInbox(*settings, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail inbox: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(messages))
	for _, message := range messages {
		rows = append(rows, []string{
			message.Date.Format("2006-01-02 15:04"),
			message.From,
			message.Subject,
		})
	}
	printTable(os.Stdout, []string{"Date", "From", "Subject"}, rows)
	return 0
}

// runMailWatch handles "yt-manage mail watch ...".
func runMailWatch(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(mailWatchUsage)
	}
	fs := flag.NewFlagSet("mail watch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	to := fs.String("to", "", "recipient for publish notifications")
	interval := fs.Duration("interval", 5*time.Minute, "time between watcher passes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *to == "" {
		fmt.Fprint(os.Stderr, mailWatchUsage)
		return 2
	}
	settings, _, err := loadMailSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail watch: %v\n", err)
		return 1
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail watch: %v\n", err)
		return 1
	}
	defer dbs.Close()
	fmt.Fprintf(os.Stdout, "Watching for newly published videos; notifications go to %s every %s. Press Ctrl+C to stop.\n", *to, interval.String())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := mailer.Watch(ctx, store, *settings, *to, *interval, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage mail watch: %v\n", err)
		return 1
	}
	return 0
}
