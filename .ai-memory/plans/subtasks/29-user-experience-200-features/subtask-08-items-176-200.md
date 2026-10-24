# Subtask 08 — Items 176–200: Portability, help, delight

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 03-next-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 176. Backup schedule
- [x] 177. Read-only share link (LAN)
- [x] 178. Activity attribution
- [x] 179. Offline queue
- [x] 180. Version-skew warnings
- [x] 181. Contextual help on every screen
- [x] 182. CLI ↔ UI name parity shown
- [x] 183. Interactive walkthroughs
- [x] 184. What's New panel
- [x] 185. Error-to-help linking
- [x] 186. Searchable help
- [x] 187. Tooltips that teach once
- [x] 188. Example library
- [x] 189. Keyboard shortcut trainer
- [x] 190. Feedback button
- [x] 191. Personal home screen
- [x] 192. Custom sidebar order
- [x] 193. Celebration moments, restrained
- [x] 194. Channel-branded accents
- [x] 195. Avatar menu with quick actions
- [x] 196. Time-zone honesty
- [x] 197. Language-ready UI
- [x] 198. Print-friendly views
- [x] 199. "Focus mode" for a publish run
- [x] 200. Annual wrap-up

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Backend: database backup schedule (cadence off/daily/weekly,
retention, tick-driven) with VACUUM INTO snapshots of all
four split databases into <home>/backups/<timestamp>/, list
and back-up-now endpoints, and a stage-restore that copies
files to <home>/restore-staging/ with plain instructions -
the swap itself needs the server stopped, and the response
says so (one panic found during smoke: pruneDBBackups sliced
past the end when fewer sets than the retention existed;
fixed with a length guard before shipping). Read-only share
link (token + expiry in the settings KV, middleware pins
token-bearing requests to read-only and sets the cookie,
revoke endpoint, status reports shareMode). Activity
attribution: ActivityLog.Actor (machine hostname for local
actions, "peer <name>" for inbound peer pushes; old entries
show nothing). Offline queue: bulk applies whose failures
are all connectivity-shaped are queued (KV) with their
change-set; list/retry (caller re-previews from the list
first)/discard endpoints. Peer version skew: payload carries
appVersion + schemaVersion; the peer version endpoint and
pull/conflict responses warn when the peer's schema or app
version differs. What's New feed (four wave entries with
try-it sections). Channel timezone in channel defaults
(IANA-validated). Annual wrap-up endpoint from the mirror.
Verified live: backup create/list/stage, share create/pin/
block/revoke, wrap-up numbers, timezone validation, actor
field, queue empty state, whats-new feed.
Frontend: Settings backup card + share card, History
actor labels, Home offline-queue card, peer version check,
per-section help drawer with verified CLI equivalents,
guided tours, What's New panel + badge, error-to-help
links, palette help search, teach-once tips, example
library, shortcut trainer, feedback report, personal home
layout, custom sidebar, restrained celebrations, channel
accent, avatar menu, dual-zone time display, STRINGS
catalog for new text (legacy UI not retrofitted - stated
honestly), print stylesheet, Focus mode, Wrap-up card with
download-as-page. Honest partials: tour steps fall back to
a centred popover when the target element never renders;
wizard/checklist Home cards keep their own content logic
inside customise mode; only --accent is overridden (hover/
focus rings keep the default); rules examples are copy-only
(the Automation form has no import); language catalog
covers Wave 8 text only.
