# yt-manage

A Go CLI for full multi-account YouTube management, built following gitmap
practices: split SQLite databases, a repository-pattern data layer, mass
search, a REST API with a modern browser UI, CLI-to-CLI machine sync, an
installer and a self-updater.

## What it does

- Connect multiple YouTube accounts with OAuth (one refresh token per
  account, encrypted at rest) and keep a local mirror of every channel.
- Read videos and shorts with visibility (public, unlisted, private,
  scheduled, draft), view counts and comment counts.
- Mass search the local mirror AUM-style by title or description content,
  then batch-rename or batch-edit the matches with an explicit
  confirmation step. All edits are Unicode-safe.
- Group videos into episodes automatically, even when titles are
  inconsistent (`EP3`, `EP 03`, `part 2` in different places), sequence
  the parts, and schedule a whole episode on a cadence (every day, every
  week, every 2 weeks, every 3 weeks). All parts of an episode publish
  on the same day.
- Comment automation: post the first comment on a video and edit it
  later. Pinning is a YouTube Studio-only action — the YouTube Data API
  v3 has no pin endpoint, so yt-manage prepares the comment and reports
  the pin as a manual Studio step.
- Create and edit playlists (title and description) from the CLI or UI.
- Send and receive email around publish events (SMTP/IMAP, following the
  go-email-reader patterns).
- REST API plus a modern CSS3 browser UI for browsing and bulk-editing
  the database, running A/B title tests, and finding patterns.
- `llm train` export and CLI-to-CLI peer sync so two machines can
  forward and share one cache database, like gitmap.

## Data layout

Split SQLite databases (gitmap Db concept), normalized schemas, GORM as
the ORM, and one repository per aggregate:

- `accounts.db` — connected accounts and encrypted OAuth tokens
- `content.db` — channels, videos, playlists, comments
- `planning.db` — episodes, parts, schedule rules, A/B tests
- `system.db` — sync state, peers, email log, settings

## Status

Scaffolding phase: repository, coding-guideline install, spec and plan
land first; implementation follows in waves (core CLI and database,
OAuth and sync, search and batch edit, episodes and scheduling,
API/UI/peer/installer).

## Install

Installer and self-updater follow the gitmap pattern and ship with the
first release.
