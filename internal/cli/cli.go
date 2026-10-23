// Package cli implements the yt-manage command tree: dispatch, help
// text, and the shared helpers (environment bootstrap, store access,
// confirmation prompts, table output) used by every command group.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/db"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// commandGroup pairs a command name with its handler.
type commandGroup struct {
	name string
	run  func(args []string) int
}

// commandGroups is the full command tree, in help order.
func commandGroups() []commandGroup {
	return []commandGroup{
		{"account", runAccount},
		{"sync", runSync},
		{"video", runVideo},
		{"search", runSearch},
		{"rename", runRename},
		{"episode", runEpisode},
		{"schedule", runSchedule},
		{"playlist", runPlaylist},
		{"comment", runComment},
		{"mail", runMail},
		{"serve", runServe},
		{"peer", runPeer},
		{"llm", runLLM},
		{"update", runUpdate},
		{"version", runVersion},
	}
}

// Run dispatches args to a command group and returns the process exit
// code. With no arguments it prints the help text and returns 0. An
// unknown command prints an error plus the help text to stderr and
// returns 2.
func Run(args []string) int {
	if len(args) == 0 {
		printHelp(os.Stdout)
		return 0
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(os.Stdout)
		return 0
	}
	if err := ensureEnvironment(); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	for _, group := range commandGroups() {
		if group.name == args[0] {
			return group.run(args[1:])
		}
	}
	fmt.Fprintf(os.Stderr, "yt-manage: unknown command %q\n\n", args[0])
	printHelp(os.Stderr)
	return 2
}

// ensureEnvironment makes sure the home directory exists and the four
// split databases can be opened (and are therefore created and
// migrated on first run).
func ensureEnvironment() error {
	home, err := config.HomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("create home directory: %w", err)
	}
	dbs, err := db.Open(home)
	if err != nil {
		return err
	}
	return dbs.Close()
}

// openStore opens the split databases under the home directory and
// returns the repository store over them. The caller owns closing the
// returned DBs.
func openStore() (*repo.Store, *db.DBs, error) {
	home, err := config.HomeDir()
	if err != nil {
		return nil, nil, err
	}
	dbs, err := db.Open(home)
	if err != nil {
		return nil, nil, err
	}
	return repo.NewStore(dbs), dbs, nil
}

// confirmChangeSet prints a change-set summary and asks the user to
// confirm before anything is applied. It returns true only when the
// answer is "y" or "yes" (case-insensitive).
func confirmChangeSet(r io.Reader, w io.Writer, summary string) bool {
	fmt.Fprintln(w, summary)
	fmt.Fprint(w, "Apply? [y/N] ")
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// printTable writes headers and rows as simple aligned columns.
func printTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	printRow := func(cells []string) {
		parts := make([]string, len(cells))
		for i, cell := range cells {
			if i < len(widths)-1 {
				parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
			} else {
				parts[i] = cell
			}
		}
		fmt.Fprintln(w, strings.Join(parts, "  "))
	}
	printRow(headers)
	for _, row := range rows {
		printRow(row)
	}
}

// hasHelpFlag reports whether args ask for help.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			return true
		}
	}
	return false
}

// runGroupUsage prints a group's usage text and returns 0. Wave-1
// feature handlers use it until later waves wire the real behaviour.
func runGroupUsage(usage string) int {
	fmt.Fprint(os.Stdout, usage)
	return 0
}
