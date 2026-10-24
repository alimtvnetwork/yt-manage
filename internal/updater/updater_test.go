package updater

// Tests for the self-updater's pure helpers. No network:
// LatestRelease and the download path are deliberately not
// covered here; they need the GitHub API and a release server.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v1.2.3", "1.2.3"},
		{" 1.0 ", "1.0"},
		{"1.2.3", "1.2.3"},
		{"vv1.0", "v1.0"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeVersion(tc.in); got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	want := "yt-manage_" + runtime.GOOS + "_" + runtime.GOARCH + ext
	if got := archiveName(); got != want {
		t.Errorf("archiveName() = %q, want %q", got, want)
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	archive := "yt-manage_linux_amd64.tar.gz"
	archivePath := filepath.Join(dir, archive)
	content := []byte("fake archive bytes")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])
	sumsPath := filepath.Join(dir, "checksums.txt")

	writeSums := func(t *testing.T, lines string) {
		t.Helper()
		if err := os.WriteFile(sumsPath, []byte(lines), 0o644); err != nil {
			t.Fatalf("write checksums: %v", err)
		}
	}

	writeSums(t, fmt.Sprintf("%s  %s\n", good, archive))
	if err := verifyChecksum(sumsPath, archivePath, archive); err != nil {
		t.Errorf("matching checksum: %v", err)
	}

	writeSums(t, fmt.Sprintf("%s  %s\n", good, archive))
	// Starred (binary-mode) entries are also accepted.
	writeSums(t, fmt.Sprintf("%s *%s\n", good, archive))
	if err := verifyChecksum(sumsPath, archivePath, archive); err != nil {
		t.Errorf("starred matching checksum: %v", err)
	}

	writeSums(t, fmt.Sprintf("%s  %s\n", "0000000000000000000000000000000000000000000000000000000000000000", archive))
	if err := verifyChecksum(sumsPath, archivePath, archive); err == nil {
		t.Error("mismatching checksum: want an error, got nil")
	}

	writeSums(t, fmt.Sprintf("%s  some-other-file.tar.gz\n", good))
	if err := verifyChecksum(sumsPath, archivePath, archive); err == nil {
		t.Error("missing checksum entry: want an error, got nil")
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{
		Tag: "v0.2.0",
		Assets: []Asset{
			{Name: "yt-manage_linux_amd64.tar.gz", URL: "https://example.invalid/a"},
			{Name: "checksums.txt", URL: "https://example.invalid/sums"},
		},
	}
	got := release.findAsset("yt-manage_linux_amd64.tar.gz")
	if got == nil {
		t.Fatal("findAsset present: got nil")
	}
	if got.URL != "https://example.invalid/a" {
		t.Errorf("findAsset URL = %q, want the archive URL", got.URL)
	}
	if got := release.findAsset("yt-manage_windows_amd64.zip"); got != nil {
		t.Errorf("findAsset missing: got %+v, want nil", got)
	}
	if got := (&Release{}).findAsset("anything"); got != nil {
		t.Errorf("findAsset on empty release: got %+v, want nil", got)
	}
}
