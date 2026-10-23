# Subtask 03 — Episodes, scheduler, comments, email (wave 3)

**Goal:** the differentiating features, per spec 03.

## Owned files

- Worker A: `internal/episodes/*.go`, `internal/scheduler/*.go`, CLI
  wiring for `episode` + `schedule`
- Worker B: `internal/comments/*.go`, `internal/mailer/*.go`, CLI
  wiring for `comment` + `mail` + `playlist`

## Contract

- Episodes: parser exactly per spec 03 §2 (ordered regex patterns,
  word numbers one–ten, `(2/3)` form, no-part = part 1); grouper
  materialises `episodes`/`episode_parts` idempotently by
  `episode_key`; gap warnings on missing part numbers.
- Scheduler: plan first (date table for the whole series), confirm,
  then apply `publishAt` per part — same day per episode, +5 minutes
  per part in sequence; public videos skipped unless
  `--include-public` is confirmed.
- Comments: `commentThreads.insert` for the first comment,
  `comments.update` for edits; records kept in content.db with
  `is_first`. Pin: output states plainly that pinning is a manual
  YouTube Studio step (no API endpoint) — never simulated.
- Mailer: SMTP send + IMAP read (go-email-reader config shape:
  host/port/user/pass in the data dir config, password never logged);
  publish watcher sends one email per newly published video and logs
  to `email_logs`.
- Playlists: create/update title + description via API + mirror.

## Exit criteria

- Episode parser reproduces spec 03 §6 as a table test (wave 5 runs
  it); scheduler plan output matches the cadence rules.
