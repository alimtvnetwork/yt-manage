package cli

import (
	"fmt"
	"os"
	"strings"
)

const completionUsage = `Usage: yt-manage completion <bash|zsh|powershell>

Print a shell completion script for yt-manage (spec 29, item 76).

Examples:
  bash:       yt-manage completion bash > /etc/bash_completion.d/yt-manage
  zsh:        yt-manage completion zsh > "${fpath[1]}/_yt-manage"
  powershell: yt-manage completion powershell >> $PROFILE
`

// runCompletion handles "yt-manage completion ...". The scripts are
// generated from the live command tree so they cannot drift from
// the binary.
func runCompletion(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(completionUsage)
	}
	names := make([]string, 0, len(commandGroups()))
	for _, group := range commandGroups() {
		names = append(names, group.name)
	}
	for alias := range commandAliases {
		names = append(names, alias)
	}
	joined := strings.Join(names, " ")
	switch args[0] {
	case "bash":
		fmt.Fprintf(os.Stdout, `# bash completion for yt-manage (generated)
_yt_manage() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
  fi
}
complete -F _yt_manage yt-manage
`, joined)
	case "zsh":
		fmt.Fprintf(os.Stdout, `#compdef yt-manage
# zsh completion for yt-manage (generated)
_yt_manage() {
  local -a commands
  commands=(%s)
  _describe 'command' commands
}
_yt_manage "$@"
`, quoteEach(names))
	case "powershell":
		fmt.Fprintf(os.Stdout, `# PowerShell completion for yt-manage (generated)
Register-ArgumentCompleter -CommandName yt-manage -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  if ($commandAst.CommandElements.Count -eq 2) {
    @(%s) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
      [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
  }
}
`, quoteEachPS(names))
	default:
		fmt.Fprintf(os.Stderr, "yt-manage: unknown shell %q (want bash, zsh or powershell)\n", args[0])
		return 2
	}
	return 0
}

func quoteEach(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + name + "'"
	}
	return strings.Join(quoted, " ")
}

func quoteEachPS(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = "'" + name + "'"
	}
	return strings.Join(quoted, ", ")
}
