# Subtask 02 — YouTube connect, sync, search, batch edit (wave 2)

**Goal:** real YouTube Data API v3 integration against the wave-1 core.

## Owned files

- Worker A: `internal/oauth/*.go`, `internal/ytapi/*.go`,
  `internal/syncer/*.go`, CLI wiring for `account` + `sync`
- Worker B: `internal/searchx/*.go`, `internal/batch/*.go`, CLI wiring
  for `search` + `rename` + `video`

## Contract

- OAuth: user supplies a Google OAuth client via
  `YT_MANAGE_CLIENT_ID` / `YT_MANAGE_CLIENT_SECRET` or a client-secret
  JSON in the data dir. Loopback callback server on a random port,
  scopes `youtube.force-ssl` + `youtube.upload` not required —
  `https://www.googleapis.com/auth/youtube.force-ssl` covers read +
  update + comment insert. Refresh token encrypted via cryptox before
  it touches accounts.db.
- ytapi builds a `*youtube.Service` per account from the stored token.
- Syncer: channels.list(mine) → uploads playlist → playlistItems pages
  → videos.list batches of 50 → repo upserts; statistics included;
  quota estimate logged before the run (spec 02 §4).
- Searchx: FTS5 MATCH over `videos_fts` + structured filters
  (channel, visibility, date range); zero API calls.
- Batch: planner produces a change-set (old → new per video) from a
  search result + rename pattern (template with `{title}`, `{n}`,
  regex replace); CLI prints the set and requires `--apply` after an
  interactive yes; titles NFC-normalised via `golang.org/x/text`;
  apply path updates the API first, then the mirror row.

## Exit criteria

- Code paths compile and are wired to real commands; unit-testable
  pieces (planner, quota estimate) have no network dependency.
