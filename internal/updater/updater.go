// Package updater implements self-update from GitHub releases:
// it queries the latest release of the yt-manage repository,
// verifies the platform archive against the release checksums,
// and swaps the running binary, keeping a .bak backup for
// rollback.
//
// Release archives MUST contain the plain binary name (yt-manage
// or yt-manage.exe). An archive without it is refused: this is the
// gitmap E1148 lesson, where the updater looked for a plain name
// inside a tarball that shipped a versioned one and every update
// failed.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// RepoSlug is the GitHub repository releases are read from.
const RepoSlug = "alimtvnetwork/yt-manage"

// BinaryName is the plain binary name shipped inside release
// archives (with a .exe suffix on Windows).
const BinaryName = "yt-manage"

// checksumsName is the name of the release checksums asset.
const checksumsName = "checksums.txt"

// Asset is one downloadable file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Release is the subset of a GitHub release the updater needs.
type Release struct {
	Tag    string  `json:"tag"`
	Assets []Asset `json:"assets"`
}

// githubRelease is the wire form of the GitHub release API.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// plainBinaryName returns the binary name for the current
// platform: yt-manage, or yt-manage.exe on Windows.
func plainBinaryName() string {
	if runtime.GOOS == "windows" {
		return BinaryName + ".exe"
	}
	return BinaryName
}

// archiveName returns the release archive name for the current
// platform: yt-manage_<goos>_<goarch>.tar.gz, or .zip on Windows.
func archiveName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return BinaryName + "_" + runtime.GOOS + "_" + runtime.GOARCH + ext
}

// normalizeVersion strips surrounding space and one leading "v"
// so "v1.2.3" and "1.2.3" compare equal.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// httpClient returns the client used for GitHub requests.
func httpClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute}
}

// LatestRelease fetches the latest published release of RepoSlug
// from the GitHub API.
func LatestRelease(ctx context.Context) (*Release, error) {
	url := "https://api.github.com/repos/" + RepoSlug + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yt-manage-updater")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch latest release: unexpected status %s", resp.Status)
	}
	var wire githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	release := &Release{Tag: wire.TagName, Assets: []Asset{}}
	for _, asset := range wire.Assets {
		release.Assets = append(release.Assets, Asset{
			Name: asset.Name,
			URL:  asset.BrowserDownloadURL,
		})
	}
	return release, nil
}

// findAsset returns the asset with the given name, or nil.
func (r *Release) findAsset(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// Check reports the running version, the latest release tag, and
// whether the latest release differs from the running version.
func Check(ctx context.Context) (current string, latest string, isUpdateAvailable bool, err error) {
	release, err := LatestRelease(ctx)
	if err != nil {
		return "", "", false, err
	}
	current = version.Version
	latest = release.Tag
	isUpdateAvailable = normalizeVersion(latest) != normalizeVersion(current)
	return current, latest, isUpdateAvailable, nil
}
