package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/peer"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

const peerUsage = `Usage: yt-manage peer <subcommand> [arguments]

Exchange local cache rows (videos and playlists, keyed by their
YouTube ids) with another machine's yt-manage over its REST API,
using HMAC-signed requests. This lets a machine with an account
connected feed a machine without one. Both machines must register
the pair under the same peer name and shared secret.

Subcommands:
  add      Register a peer (--name, --url, --secret)
  list     List registered peers
  push     Push local cache rows to a peer (--peer name or id)
  pull     Pull cache rows from a peer (--peer name or id)
`

const peerAddUsage = `Usage: yt-manage peer add --name NAME --url URL --secret SECRET

Arguments:
  --name NAME      Peer name (required; both machines use the same
                   name for the pair)
  --url URL        Peer's base URL, e.g. http://192.168.1.20:8765
                   (required; a trailing slash is trimmed)
  --secret SECRET  Shared secret for HMAC signing (required; both
                   machines use the same secret)
`

const peerPushUsage = `Usage: yt-manage peer push --peer NAME_OR_ID

Arguments:
  --peer NAME_OR_ID   The peer's name, or its numeric local ID
`

const peerPullUsage = `Usage: yt-manage peer pull --peer NAME_OR_ID

Arguments:
  --peer NAME_OR_ID   The peer's name, or its numeric local ID
`

// runPeer handles "yt-manage peer ...".
func runPeer(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		return runGroupUsage(peerUsage)
	}
	switch args[0] {
	case "add":
		return runPeerAdd(args[1:])
	case "list":
		return runPeerList(args[1:])
	case "push":
		return runPeerPush(args[1:])
	case "pull":
		return runPeerPull(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "yt-manage peer: unknown subcommand %q\n", args[0])
		fmt.Fprint(os.Stderr, peerUsage)
		return 2
	}
}

// runPeerAdd handles "yt-manage peer add ...".
func runPeerAdd(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(peerAddUsage)
	}
	fs := flag.NewFlagSet("peer add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "peer name")
	url := fs.String("url", "", "peer base URL")
	secret := fs.String("secret", "", "shared secret")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *name == "" || *url == "" || *secret == "" {
		fmt.Fprint(os.Stderr, peerAddUsage)
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer add: %v\n", err)
		return 1
	}
	defer dbs.Close()
	peerRow := &model.Peer{
		Name:         *name,
		BaseURL:      strings.TrimRight(*url, "/"),
		SharedSecret: *secret,
	}
	if err := store.Peers.Create(peerRow); err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer add: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Peer %s added (id %d). Both machines must use the same name and secret.\n", peerRow.Name, peerRow.ID)
	return 0
}

// runPeerList handles "yt-manage peer list ...".
func runPeerList(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(peerUsage)
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer list: %v\n", err)
		return 1
	}
	defer dbs.Close()
	peers, err := store.Peers.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer list: %v\n", err)
		return 1
	}
	rows := make([][]string, 0, len(peers))
	for _, peerRow := range peers {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(peerRow.ID), 10),
			peerRow.Name,
			peerRow.BaseURL,
			formatTime(peerRow.LastSeenAt),
		})
	}
	printTable(os.Stdout, []string{"ID", "Name", "Base URL", "Last seen"}, rows)
	return 0
}

// resolvePeerArg resolves a --peer value to a stored peer: a
// numeric value is a local ID, anything else is a peer name.
func resolvePeerArg(store *repo.Store, ref string) (*model.Peer, error) {
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		return store.Peers.GetByID(uint(id))
	}
	return store.Peers.GetByName(ref)
}

// parsePeerFlag parses the shared --peer flag of push and pull. It
// returns the peer reference, or "" after printing the usage (in
// which case the caller returns exit code 2).
func parsePeerFlag(name string, usage string, args []string) string {
	if hasHelpFlag(args) {
		return ""
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	ref := fs.String("peer", "", "peer name or numeric ID")
	if err := fs.Parse(args); err != nil {
		return ""
	}
	if *ref == "" {
		fmt.Fprint(os.Stderr, usage)
		return ""
	}
	return *ref
}

// runPeerPush handles "yt-manage peer push ...".
func runPeerPush(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(peerPushUsage)
	}
	ref := parsePeerFlag("peer push", peerPushUsage, args)
	if ref == "" {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer push: %v\n", err)
		return 1
	}
	defer dbs.Close()
	peerRow, err := resolvePeerArg(store, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer push: %v\n", err)
		return 1
	}
	result, err := peer.PushTo(context.Background(), store, peerRow.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer push: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Pushed: %d videos applied, %d skipped, %d playlists, %d items.\n",
		result.VideosApplied, result.VideosSkipped, result.PlaylistsApplied, result.ItemsApplied)
	return 0
}

// runPeerPull handles "yt-manage peer pull ...".
func runPeerPull(args []string) int {
	if hasHelpFlag(args) {
		return runGroupUsage(peerPullUsage)
	}
	ref := parsePeerFlag("peer pull", peerPullUsage, args)
	if ref == "" {
		return 2
	}
	store, dbs, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer pull: %v\n", err)
		return 1
	}
	defer dbs.Close()
	peerRow, err := resolvePeerArg(store, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer pull: %v\n", err)
		return 1
	}
	result, err := peer.PullFrom(context.Background(), store, peerRow.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-manage peer pull: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Pulled: %d videos applied, %d skipped, %d playlists, %d items.\n",
		result.VideosApplied, result.VideosSkipped, result.PlaylistsApplied, result.ItemsApplied)
	return 0
}
