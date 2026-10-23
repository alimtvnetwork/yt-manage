# Subtask 01 — Core (wave 1)

**Goal:** the skeleton every other wave builds on: module, config,
crypto, models, split DB, repositories, CLI shell with full help.

## Owned files

- Worker A: `go.mod`, `internal/model/*.go` (models + enums),
  `internal/db/*.go`, `internal/repo/*.go`
- Worker B: `main.go`, `internal/version/*.go`, `internal/config/*.go`,
  `internal/cryptox/*.go`, `internal/cli/*.go` (command tree + help;
  feature commands may shell out to stubs that later waves fill —
  prefer real wiring against repo interfaces defined here)

## Contract

- Module `github.com/alimtvnetwork/yt-manage`, `go 1.22`.
- GORM via `github.com/glebarez/sqlite` (pure Go, no CGO).
- Four DBs per spec 02 §2, opened by `internal/db.Open(dataDir)`;
  AutoMigrate per DB; FTS5 `videos_fts` created in content.db.
- Enums carry String/Parse/IsValid/All.
- Repositories are the only GORM consumers; services take repository
  interfaces.
- `internal/cli` exposes `Run(args) int`; help lists: account, sync,
  video, search, rename, episode, schedule, playlist, comment, mail,
  serve, peer, llm, update, version.
- Data dir: `~/.yt-manage` (override `YT_MANAGE_HOME`).

## Exit criteria

- `go build ./...` clean; `yt-manage help` and `version` run;
  first run creates the four DBs.
