# Subtask 05 — CI/CD, installer, tests (wave 5)

**Goal:** proper CI/CD, as explicitly required by the user, plus the
installer and the pure-logic test suite.

## Owned files

- Worker A: `.github/workflows/ci.yml`, `.github/workflows/release.yml`,
  `install.sh`, README install/usage sections
- Worker B: unit tests — `internal/episodes/*_test.go`,
  `internal/scheduler/*_test.go`, `internal/batch/*_test.go`,
  `internal/cryptox/*_test.go`, `internal/model/*_test.go`

## Contract — CI

- `ci.yml` on push + PR to main: checkout → setup-go (stable) →
  `gofmt -l` gate → `go vet ./...` → `go build ./...` →
  `go test ./... -coverprofile=coverage.out` → coverage artifact →
  cross-compile matrix (linux/amd64, windows/amd64, darwin/arm64,
  `CGO_ENABLED=0`).
- Every job must be able to fail the build; no `continue-on-error`.

## Contract — release

- `release.yml` on tag `v*`: build the matrix → package
  `yt-manage_<os>_<arch>.tar.gz` (zip on Windows) with the PLAIN
  binary name inside (`yt-manage` / `yt-manage.exe`) → `checksums.txt`
  (sha256) → `gh release create` with generated notes using relative
  paths only.
- Version source of truth: the git tag; `internal/version` is bumped
  in the release commit.

## Contract — installer + tests

- Root `install.sh`: detect OS/arch, download the latest release
  tarball + checksums, verify SHA256, install to `~/.local/bin`
  (or `$PREFIX/bin`), print a PATH hint. Fails loudly on mismatch.
- Tests cover: episode title parsing (the spec 03 §6 example set,
  zero-padding, word numbers, `(2/3)`), cadence date math (daily /
  7 / 14 / 21 days, same-day parts), rename planner templates,
  crypto round-trip, enum String/Parse round-trips.

## Exit criteria

- CI green on GitHub for the wave-5 push (verified via check-runs
  API); tagging v0.1.0 yields a release whose assets install via
  `install.sh`.
