package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// printJSON writes a value as indented JSON to stdout (spec 29,
// item 119: read commands accept --json so scripts get stable,
// parseable output with the same exit codes).
func printJSON(value interface{}) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage: encode JSON: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, string(data))
	return 0
}

// hasJSONFlag reports whether --json appears anywhere in args.
func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

// stripJSONFlag removes --json so flag parsers that do not know
// it do not choke on it.
func stripJSONFlag(args []string) []string {
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--json" {
			kept = append(kept, arg)
		}
	}
	return kept
}
