# Subtask 02 — Items 26–50: Search depth, bulk edit, episodes & scheduling

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 02-first-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 26. Recent searches
- [x] 27. Search suggestions from data
- [x] 28. "0 results" recovery
- [x] 29. Keyboard-first search
- [x] 30. Search across everything
- [x] 31. Change-set preview as a table
- [x] 32. Template builder with live preview
- [x] 33. Find & replace mode
- [x] 34. Conflict flags in preview
- [x] 35. Apply in stages
- [x] 36. Progress bar with per-item results
- [x] 37. Retry failures only
- [x] 38. Rename history per video
- [x] 39. Save rename recipes
- [x] 40. Description & tags in bulk
- [x] 41. Episode board view
- [x] 42. Drag to resequence
- [x] 43. Confidence badges on grouping
- [x] 44. One-click split / merge
- [x] 45. Calendar view of schedules
- [x] 46. Cadence picker with real dates
- [x] 47. "What moves if I change this" preview
- [x] 48. Same-day parts, visibly linked
- [x] 49. Schedule health warnings
- [x] 50. Plan vs applied state

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Wave 2 shipped: search suggestions + cross-entity search, recent
searches, zero-result filter recovery, keyboard-first search; bulk
plan per-change warnings, include/exclude rows, token template
builder with live preview, staged apply (onlyIds) with per-item
results and retry-failures via plan retention; rename recipes CRUD
and per-video title history (new title_histories table, recorded in
batch.Apply); episode board with gap slots and warnings, drag
resequence, split/merge endpoints, schedule-plan previews with
current-vs-planned dates, month calendar. Verified: build + vet +
gofmt clean, node --check, live smoke (suggestions, search-all,
merge/resequence/split, schedule-plan, recipes, duplicate warnings).
Honest partials: item 40 covers description bulk only - tags are not
mirrored by sync, so no tags feature was faked; item 43 surfaces
API warnings as review cues - there is no numeric confidence score.
