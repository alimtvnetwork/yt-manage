package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/config"
)

const settingsUsage = `Usage: yt-manage settings <subcommand> [arguments]

Read and change the same settings the browser UI's Settings screen
manages (spec 29, item 60 - one set of names, both surfaces).

Subcommands:
  list            List every setting with its value and default
  get <key>       Print one setting's value (secrets are never printed)
  set <key> <val> Validate and save one setting
  reset <key>     Reset one setting to its default
  export          Print the settings as JSON (secrets excluded)
  import <file>   Import settings JSON produced by export ('-' = stdin)

Keys: daily_quota_budget, server_addr, default_account, default_channel,
read_only, mail.smtp_host, mail.smtp_port,
mail.imap_host, mail.imap_port, mail.username, mail.password.
`

// runSettings handles "yt-manage settings ...".
func runSettings(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(settingsUsage)
	}
	switch args[0] {
	case "list":
		return runSettingsList()
	case "get":
		return runSettingsGet(args[1:])
	case "set":
		return runSettingsSet(args[1:])
	case "reset":
		return runSettingsReset(args[1:])
	case "export":
		return runSettingsExport()
	case "import":
		return runSettingsImport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage: unknown settings subcommand %q\n\n", args[0])
		fmt.Fprint(os.Stderr, settingsUsage)
		return 2
	}
}

func loadCLISettings() (*config.Settings, string, error) {
	home, err := config.HomeDir()
	if err != nil {
		return nil, "", err
	}
	settings, err := config.LoadSettings(home)
	return settings, home, err
}

func runSettingsList() int {
	settings, _, err := loadCLISettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	rows := [][]string{}
	for _, def := range config.SettingKeys() {
		value, _ := settings.GetByKey(def.Key)
		if def.Type == "secret" {
			if value == "set" {
				value = "(set, hidden)"
			} else {
				value = "(not set)"
			}
		}
		rows = append(rows, []string{def.Key, value, def.Default, def.Label})
	}
	printTable(os.Stdout, []string{"Key", "Value", "Default", "Label"}, rows)
	return 0
}

func runSettingsGet(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "yt-manage: settings get needs exactly one key")
		return 2
	}
	settings, _, err := loadCLISettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	def, found := config.FindSettingKey(args[0])
	if !found {
		fmt.Fprintf(os.Stderr, "yt-manage: unknown setting %q\n", args[0])
		return 1
	}
	if def.Type == "secret" {
		fmt.Fprintln(os.Stdout, "(secret values are never printed)")
		return 0
	}
	value, _ := settings.GetByKey(args[0])
	fmt.Fprintln(os.Stdout, value)
	return 0
}

func runSettingsSet(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "yt-manage: settings set needs a key and a value")
		return 2
	}
	settings, home, err := loadCLISettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	if err := settings.SetByKey(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	if err := settings.Save(home); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Saved %s\n", args[0])
	return 0
}

func runSettingsReset(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "yt-manage: settings reset needs exactly one key")
		return 2
	}
	def, found := config.FindSettingKey(args[0])
	if !found {
		fmt.Fprintf(os.Stderr, "yt-manage: unknown setting %q\n", args[0])
		return 1
	}
	return runSettingsSet([]string{args[0], def.Default})
}

func runSettingsExport() int {
	settings, _, err := loadCLISettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	values, excluded := settings.Snapshot()
	payload := map[string]interface{}{"settings": values, "excluded": excluded}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, string(data))
	return 0
}

func runSettingsImport(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "yt-manage: settings import needs a file path (or - for stdin)")
		return 2
	}
	var data []byte
	var err error
	if args[0] == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	var payload struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: parse import file: %v\n", err)
		return 1
	}
	settings, home, err := loadCLISettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	keys := make([]string, 0, len(payload.Settings))
	for key := range payload.Settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := settings.SetByKey(key, payload.Settings[key]); err != nil {
			fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
			return 1
		}
	}
	if err := settings.Save(home); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Imported %d setting(s): %s\n", len(keys), strings.Join(keys, ", "))
	return 0
}
