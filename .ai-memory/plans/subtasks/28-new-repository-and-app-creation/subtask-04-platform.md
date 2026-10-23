# Subtask 04 — REST API + UI, peer sync, llm train, updater (wave 4)

**Goal:** the platform layer from spec 02 §5–§6.

## Owned files

- Worker A: `internal/apiserver/*.go` including embedded UI assets
  (`internal/apiserver/ui/` — index.html, app.css, app.js)
- Worker B: `internal/peer/*.go`, `internal/llmtrain/*.go`,
  `internal/updater/*.go`, CLI wiring for `serve` + `peer` + `llm` +
  `update`

## Contract

- Server: stdlib `net/http`, localhost default `:8765` (flag/env
  override). JSON envelope `{ok, data, error}` on every endpoint.
- Endpoints per spec 02 §5. Bulk edit requires a confirm token issued
  by the plan endpoint (two-step: plan → apply), mirroring CLI
  discipline.
- UI: single-page, dark theme, CSS3 transitions/animations, animated
  table updates, bulk-select with a pattern finder (live highlight),
  A/B test panel (create test, snapshot stats, pick winner → applies
  the winning title through the batch path). No build step; assets are
  embedded vanilla HTML/CSS/JS.
- Peer: HMAC-SHA256 signed requests (`X-YTM-Signature`), push/pull of
  videos + playlists keyed by YouTube ids; conflicts keep the newer
  `last_synced_at`.
- llm train: `export` writes JSONL (one object per line: command docs,
  schema, anonymised examples) to the data dir or `--out` path.
- Updater: latest release via GitHub API, SHA256 verify against
  checksums.txt, atomic binary swap with `.bak` rollback; archive must
  contain the plain binary name.

## Exit criteria

- `yt-manage serve` answers `/api/accounts` and serves the UI;
  peer + llm + update commands run end-to-end locally.
