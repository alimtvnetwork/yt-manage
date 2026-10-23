package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alimtvnetwork/yt-manage/internal/config"
	"github.com/alimtvnetwork/yt-manage/internal/llmtrain"
)

const llmUsage = `Usage: yt-manage llm <subcommand> [arguments]

Subcommands:
  train    Export training data (JSONL) of command docs, schema
           descriptions and anonymised usage shapes
`

const llmTrainUsage = `Usage: yt-manage llm train <subcommand> [arguments]

Subcommands:
  export   Write the training data as JSONL
`

const llmTrainExportUsage = `Usage: yt-manage llm train export [--out PATH]

Write JSONL training records (command docs, schema descriptions
and anonymised usage shapes) to a file. The export contains no
account data, tokens, email addresses, titles or YouTube ids.

Arguments:
  --out PATH   Output file (default: <home>/llm-train.jsonl)
`

// runLLM handles "yt-manage llm ...".
func runLLM(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(llmUsage)
	}
	switch args[0] {
	case "train":
		return runLLMTrain(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage llm: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, llmUsage)
		return 2
	}
}

// runLLMTrain handles "yt-manage llm train ...".
func runLLMTrain(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(llmTrainUsage)
	}
	switch args[0] {
	case "export":
		return runLLMTrainExport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage llm train: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, llmTrainUsage)
		return 2
	}
}

// runLLMTrainExport handles "yt-manage llm train export ...".
func runLLMTrainExport(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(llmTrainExportUsage)
	}
	fs := flag.NewFlagSet("llm train export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	home, err := config.HomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage llm train export: %v\n", err)
		return 1
	}
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(home, "llm-train.jsonl")
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage llm train export: %v\n", err)
		return 1
	}
	defer dbs.Close()
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage llm train export: %v\n", err)
		return 1
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		fmt.Fprintf(os.Stderr, "yt-manage llm train export: %v\n", err)
		return 1
	}
	count, err := llmtrain.Export(store, file)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage llm train export: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Wrote %d training records to %s.\n", count, outPath)
	return 0
}
