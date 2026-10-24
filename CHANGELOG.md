# Changelog

All notable changes to yt-manage are recorded here. The version in
`internal/version` (mirrored in `version.json`) is the in-binary
source of truth; releases are tag-driven (`vX.Y.Z` triggers the
release workflow, which publishes archives + checksums).

## [0.2.0] — 2026-10-25

Spec 29 campaign complete: 200 UX features in 8 waves, all
CI-green and live-verified against the demo dataset.

- Waves 1–4 (items 1–100): status + checklist, saved searches,
  schedules and comment lists, demo seed/clear, search sort and
  filters, a rewritten UI shell, title histories, bulk plans with
  warnings and staged retry, search suggestions, rename recipes,
  the episode board, settings with validation and undo, activity
  history, quota accounting, read-only mode, pre-flight checks,
  audit CSV export, automatic backups, keyboard map, command
  palette, multi-select bulk actions, themes, and staged playlist
  edits.
- Waves 5–8 (items 101–200): pattern and title-health
  suggestions, a suggestion inbox with snooze, a rules engine
  with an approval queue, auto-sync with quiet hours, post-publish
  pipelines, a comment centre with history and pin tracking,
  playlist board with gap/duplicate/orphan finders, tags
  mirroring with rename/merge, channel dashboard, series
  scorecards, A/B results with auto-scheduling, publish-time
  heatmap, growth snapshots, goals, multi-account overview and
  health, publish digests and failure alerts, publish timeline
  and missed-publish recovery, in-app inbox, peer sync with
  selective sharing and conflict resolution, a full export/import
  bundle, scheduled database backups, a read-only share link,
  an offline apply queue, contextual help with CLI parity,
  guided walkthroughs, personal home layout, focus mode, and an
  annual wrap-up.
- Packaging: a Windows installer (`install.ps1`, checksum-verified,
  user PATH, uninstall) alongside `install.sh`; release archives
  now cover Linux amd64, Windows amd64/arm64, and macOS
  amd64/arm64 (the Intel Mac archive was previously missing,
  which install.sh would have requested and failed to find).
- Tests: end-to-end API tests over a seeded demo server plus
  unit tests for the peer payload, the updater, and config.

Known residuals from the campaign closeout (see the acceptance
file in `02-spec/21-app/29-user-experience-200-features/`):
real-account verification needs Google OAuth consent; the CI
perf-budget test, the settings-CLI parity diff, and the
15-minute unaided wizard timing were not measured; the
localisation catalog covers Wave 8 strings only; UI-state prefs
are UI-only, not settings CLI keys.

## [0.1.0] — 2026-10-23

First release: multi-account YouTube management CLI with loopback
OAuth and AES-256-GCM token storage, split SQLite databases with
an FTS5 search mirror, dry-run batch edits, episode grouping with
cadence scheduling, comment mirroring on sync, a REST API with an
embedded browser UI, CLI-to-CLI peer sync, an installer, and a
self-updater.
