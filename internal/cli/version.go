package cli

import (
	"fmt"
	"os"

	"github.com/alimtvnetwork/yt-manage/internal/version"
)

const versionUsage = `Usage: yt-manage version

Print the yt-manage version.
`

// runVersion handles "yt-manage version".
func runVersion(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(versionUsage)
	}
	fmt.Fprintf(os.Stdout, "yt-manage %s\n", version.Version)
	return 0
}
