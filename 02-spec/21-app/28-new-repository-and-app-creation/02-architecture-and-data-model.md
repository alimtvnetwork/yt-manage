# 28 — Architecture and Data Model

## 1. Layout

Standard Go layout, mirroring go-email-reader (root `main.go` +
`internal/`) with gitmap's command discipline.

| Package | Responsibility |
|---|---|
| `main.go` | Entry point; delegates to `internal/cli` |
| `internal/cli` | Command tree, help text, flag parsing, confirmation prompts |
| `internal/version` | App version constant (releases are tag-driven) |
| `internal/config` | Data-dir resolution (`~/.yt-manage`), settings load/save |
| `internal/cryptox` | AES-GCM encrypt/decrypt for OAuth tokens |
| `internal/model` | GORM models + enums (see §3) |
| `internal/db` | Split-DB manager: open/migrate the four databases, FTS5 setup |
| `internal/repo` | Repository pattern — the ONLY layer that touches GORM |
| `internal/ytapi` | YouTube Data API v3 client per account |
| `internal/oauth` | Consent flow with local callback server; token exchange |
| `internal/syncer` | Full + incremental mirror sync |
| `internal/searchx` | AUM-style search over the FTS5 mirror |
| `internal/batch` | Rename/edit planner: change-set, confirmation, apply |
| `internal/episodes` | Title parser, episode grouper, part sequencer |
| `internal/scheduler` | Cadence rules → `publishAt` updates |
| `internal/comments` | First-comment post/edit (pin = manual Studio step) |
| `internal/mailer` | SMTP send + IMAP receive; publish-event mail |
| `internal/apiserver` | REST API + embedded modern CSS3 UI |
| `internal/peer` | CLI-to-CLI forward/receive of cache rows |
| `internal/llmtrain` | Training-data export (JSONL) |
| `internal/updater` | Self-update from GitHub releases |

Dependencies are deliberately lean: GORM with the pure-Go SQLite driver
(`github.com/glebarez/sqlite`, so cross-platform CI builds need no
CGO), `google.golang.org/api/youtube/v3`, `golang.org/x/oauth2`,
`golang.org/x/text` (Unicode normalisation), stdlib HTTP server.

## 2. Split databases (gitmap Db concept)

Data dir `~/.yt-manage/data/`, four normalized SQLite databases:

- **accounts.db** — `accounts` (id, label, google_subject, channel_id,
  token_ciphertext, token_nonce, scopes, created_at, updated_at),
  `channels` (id, account_id FK, yt_channel_id UNIQUE, title,
  handle, subscriber_count, video_count, last_synced_at).
- **content.db** — `videos` (id, channel_id FK, yt_video_id UNIQUE,
  title, description, visibility, is_short, published_at, publish_at,
  view_count, like_count, comment_count, duration_seconds,
  last_synced_at), `playlists` (id, channel_id FK, yt_playlist_id
  UNIQUE, title, description, item_count), `playlist_items`
  (playlist_id FK, video_id FK, position), `comments` (id, video_id
  FK, yt_comment_id UNIQUE, author, text, is_first, is_pinned_known,
  published_at). FTS5 virtual table `videos_fts` over title +
  description powers search.
- **planning.db** — `episodes` (id, channel_id FK, episode_key,
  episode_number, title, cadence_days, first_publish_at),
  `episode_parts` (episode_id FK, video_id FK, part_number,
  sequence), `schedule_rules` (id, channel_id FK, cadence_days,
  videos_per_episode, starts_at, is_active), `ab_tests` (id, video_id
  FK, variant_a, variant_b, winner, is_active),
  `ab_test_snapshots` (test_id FK, captured_at, views_a_context,
  ctr_note).
- **system.db** — `sync_states` (account_id, resource, cursor,
  last_run_at, last_error), `peers` (id, name, base_url, shared_secret,
  last_seen_at), `email_logs` (id, direction, subject, related_video_id,
  sent_at), `settings` (key, value).

Foreign keys on; every table has created_at/updated_at; all queries go
through `internal/repo` (AccountRepository, ChannelRepository,
VideoRepository, PlaylistRepository, CommentRepository,
EpisodeRepository, ScheduleRepository, ABTestRepository,
SyncStateRepository, PeerRepository, EmailLogRepository,
SettingRepository).

## 3. Enums (coding-guideline pattern)

Every enum ships String/Parse/IsValid/All: `Visibility`
(public/unlisted/private/scheduled/draft), `SyncStatus`,
`Cadence` (daily/weekly/two-weeks/three-weeks/custom-days),
`CommentRole`, `ABTestStatus`, `PeerDirection`, `EmailDirection`.

## 4. Sync design

Per account: `channels.list?mine=true` → uploads playlist →
`playlistItems.list` pages → `videos.list` batches of 50 (1 unit per
call) → upsert into content.db; statistics refreshed on the same call.
Playlists via `playlists.list` + items. Comment counts come from video
statistics; first-comment records are reconciled during comment ops.
Incremental runs use `sync_states` cursors and `last_synced_at`.
Quota guard: the syncer estimates unit cost before a run and refuses a
run that would exceed the configured daily budget.

## 5. REST API + UI

`yt-manage serve` starts a localhost server:

- `GET /api/videos`, `GET /api/videos/:id`, `PATCH /api/videos/:id`,
  `POST /api/videos/bulk-edit` (change-set + confirm token),
  `GET /api/search?q=`, `GET /api/episodes`,
  `POST /api/episodes/:id/schedule`, `GET/POST /api/playlists`,
  `GET/POST /api/ab-tests`, `GET /api/accounts`, `POST /api/sync`.
- UI is embedded (`embed.FS`): one-page app, CSS3 animations and
  transitions, dark theme, bulk-select table, pattern finder
  (regex/search highlight), A/B test panel. Deliberately a generation
  ahead of gitmap's docs-viewer UI: no page reloads, animated state
  changes, keyboard-first bulk edit.

## 6. Peer sync, llm train, updater

- **Peer:** `peer add` registers another machine's yt-manage
  (`base_url` + shared secret). `peer push/pull` exchanges content.db
  rows (videos/playlists keyed by yt ids) over the REST API with
  HMAC-signed requests, so a machine with the account connected can
  feed a machine without it — the gitmap forward/receive pattern.
- **llm train:** `llm train export` writes JSONL of command docs,
  schema descriptions, and anonymised usage shapes for LLM training,
  mirroring gitmap's llm-train concept.
- **Updater:** `yt-manage update` checks the latest GitHub release,
  downloads the platform tarball, verifies SHA256 against
  `checksums.txt`, and swaps the binary. Release tarballs ship the
  PLAIN binary name (`yt-manage`) — the gitmap v6.589.4 E1148 defect
  (versioned binary name inside the tar) must not be repeated.
- **Installer:** root `install.sh` downloads the latest release
  tarball for the platform, verifies checksums, installs to
  `~/.local/bin` (or `$PREFIX`).

## 7. CI/CD

- `.github/workflows/ci.yml` — on push/PR: gofmt gate, `go vet`,
  `go build`, `go test ./...` with coverage, plus cross-compile matrix
  (linux/amd64, windows/amd64, darwin/arm64, CGO disabled).
- `.github/workflows/release.yml` — on `v*` tags: build the matrix,
  package tar.gz/zip with the plain binary name, generate
  `checksums.txt`, publish a GitHub release (relative paths only in
  notes).
- CI truth is read from the GitHub check-runs API, never from
  `gitmap pe` alone (known false-green defect in the installed build).
