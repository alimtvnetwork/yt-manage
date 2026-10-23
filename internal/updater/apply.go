package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// Apply downloads the latest release archive for this platform,
// verifies its SHA256 against the release checksums.txt, extracts
// the plain binary, and swaps it over the running executable. The
// old binary is kept at <exe>.bak. Downloads are staged under
// <home>/.update-stage, never in the OS temp directory. When the
// running version already is the latest, Apply returns "already
// up to date" and changes nothing.
func Apply(ctx context.Context, home string) (string, error) {
	release, err := LatestRelease(ctx)
	if err != nil {
		return "", err
	}
	if normalizeVersion(release.Tag) == normalizeVersion(version.Version) {
		return "already up to date", nil
	}
	archive := archiveName()
	archiveAsset := release.findAsset(archive)
	if archiveAsset == nil {
		return "", fmt.Errorf("release %s is missing asset %s", release.Tag, archive)
	}
	sumsAsset := release.findAsset(checksumsName)
	if sumsAsset == nil {
		return "", fmt.Errorf("release %s is missing asset %s", release.Tag, checksumsName)
	}
	stageDir := filepath.Join(home, ".update-stage")
	if err := os.RemoveAll(stageDir); err != nil {
		return "", fmt.Errorf("clean update stage: %w", err)
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", fmt.Errorf("create update stage: %w", err)
	}
	archivePath := filepath.Join(stageDir, archive)
	if err := download(ctx, archiveAsset.URL, archivePath); err != nil {
		return "", err
	}
	sumsPath := filepath.Join(stageDir, checksumsName)
	if err := download(ctx, sumsAsset.URL, sumsPath); err != nil {
		return "", err
	}
	if err := verifyChecksum(sumsPath, archivePath, archive); err != nil {
		return "", err
	}
	newBinaryPath := filepath.Join(stageDir, plainBinaryName())
	if err := extractBinary(archivePath, newBinaryPath); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(newBinaryPath, 0o755); err != nil {
			return "", fmt.Errorf("chmod staged binary: %w", err)
		}
	}
	return swapBinary(newBinaryPath, release.Tag)
}

// download fetches url into destPath. A non-200 response is an
// error.
func download(ctx context.Context, url string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request for %s: %w", filepath.Base(destPath), err)
	}
	req.Header.Set("User-Agent", "yt-manage-updater")
	resp, err := httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(destPath), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", filepath.Base(destPath), resp.Status)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destPath, err)
	}
	return nil
}

// verifyChecksum checks the SHA256 of the downloaded archive
// against the checksum recorded for it in checksums.txt. A
// mismatch aborts the update before the binary is touched.
func verifyChecksum(sumsPath string, archivePath string, archive string) error {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(name) == archive {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksums.txt has no entry for %s", archive)
	}
	actual, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", archive, expected, actual)
	}
	return nil
}

// sha256File returns the lowercase hex SHA256 of a file.
func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// extractBinary pulls the plain binary out of the release archive
// into destPath. An archive that does not contain an entry whose
// base name is exactly the plain binary name is refused.
func extractBinary(archivePath string, destPath string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destPath)
	}
	return extractFromTarGz(archivePath, destPath)
}

// extractFromTarGz extracts the plain binary from a .tar.gz
// archive.
func extractFromTarGz(archivePath string, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != plainBinaryName() {
			continue
		}
		return writeFile(destPath, reader)
	}
	return fmt.Errorf("archive %s does not contain the plain binary %s; refusing to update", filepath.Base(archivePath), plainBinaryName())
}

// extractFromZip extracts the plain binary from a .zip archive.
func extractFromZip(archivePath string, destPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(entry.Name) != plainBinaryName() {
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open %s in archive: %w", entry.Name, err)
		}
		writeErr := writeFile(destPath, source)
		_ = source.Close()
		return writeErr
	}
	return fmt.Errorf("archive %s does not contain the plain binary %s; refusing to update", filepath.Base(archivePath), plainBinaryName())
}

// writeFile copies r into a freshly created file at path.
func writeFile(path string, r io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

// swapBinary replaces the running executable with the staged new
// binary, keeping the old one at <exe>.bak. When the replacement
// step fails, the backup is restored before the error is returned.
func swapBinary(newBinaryPath string, newTag string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	backupPath := exe + ".bak"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove stale backup %s: %w", backupPath, err)
	}
	if err := os.Rename(exe, backupPath); err != nil {
		return "", fmt.Errorf("back up current binary: %w", err)
	}
	if err := moveFile(newBinaryPath, exe); err != nil {
		_ = os.Rename(backupPath, exe)
		return "", fmt.Errorf("install new binary: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(exe, 0o755); err != nil {
			return "", fmt.Errorf("chmod installed binary: %w", err)
		}
	}
	return fmt.Sprintf("updated %s -> %s (backup at %s)", version.Version, newTag, backupPath), nil
}

// moveFile moves src to dst, falling back to a copy when a rename
// across filesystems is not possible.
func moveFile(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return err
	}
	return os.Remove(src)
}
