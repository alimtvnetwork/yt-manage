# Subtask 01 — Items 1–25: Onboarding, navigation, instant search

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 02-first-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 1. Guided first-run wizard (UI + CLI)
- [x] 2. OAuth connect with live status
- [x] 3. Testing-mode warning in plain words
- [x] 4. One-click demo channel
- [x] 5. First-sync progress with counts
- [x] 6. "What next" card after first sync
- [x] 7. Empty-state screens that teach
- [x] 8. Setup checklist that persists
- [x] 9. Import existing settings
- [x] 10. Skip-and-return onboarding
- [x] 11. One sidebar, seven homes
- [x] 12. Breadcrumb + page title everywhere
- [x] 13. Global account/channel switcher
- [x] 14. Remembered last screen
- [x] 15. Resizable, collapsible sidebar
- [x] 16. Dense / comfortable density toggle
- [x] 17. Sticky table headers + sticky action bar
- [x] 18. Right-side detail drawer
- [x] 19. Consistent page skeleton
- [x] 20. Deep links for everything
- [x] 21. Instant search as you type
- [x] 22. Highlighted matches
- [x] 23. Filter chips
- [x] 24. Sort that sticks
- [x] 25. Saved searches

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Wave 1 shipped: /api/status (counts, OAuth detection, checklist,
suggestions), saved-searches API, /api/schedules + /api/comments lists,
demo seed/clear (12 videos, grouped episodes, 1 playlist), video search
sort + shorts/has-comments filters + total counts; CLI getting-started
help and staged `account add` with the Testing-mode warning; UI rebuilt
with sidebar + badges, Home wizard/checklist/what-next, hash deep links,
filter chips, instant search with highlights/snippets, saved searches,
detail drawer, density/collapse preferences. Verified: local build +
vet + gofmt clean, live smoke against the demo dataset (seed, search,
filters, saved-search CRUD, clear), CI green on push.
