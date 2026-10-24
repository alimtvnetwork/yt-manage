# Subtask 03 — Items 51–75: Settings, feedback, undo, selection

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 02-first-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 51. A real Settings screen
- [x] 52. Settings apply instantly
- [x] 53. Settings search
- [x] 54. Per-account settings
- [x] 55. Sensible defaults, shown
- [x] 56. Dangerous settings fenced
- [x] 57. Export / import settings
- [x] 58. Settings change log
- [x] 59. Validation before acceptance
- [x] 60. CLI/UI parity
- [x] 61. Toasts that inform, then leave
- [x] 62. Undo for applied changes
- [x] 63. Global activity/history view
- [x] 64. Long-task progress centre
- [x] 65. Errors in plain language
- [x] 66. Quota meter
- [x] 67. Offline / token-expired banners
- [x] 68. Confirmation only where it matters
- [x] 69. Copyable error reports
- [x] 70. Sound-off, motion-safe feedback
- [x] 71. Full keyboard map
- [x] 72. Command palette
- [x] 73. Multi-select like a file manager
- [x] 74. Bulk action bar follows selection
- [x] 75. Recent items quick-jump

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Wave 3 shipped: real Settings screen + API (config.SettingKeys is the
single source for UI, API, and the new `settings` CLI group - items
51/60), instant apply with validation-before-save, defaults + reset,
per-account overrides (notify / sync interval) stored as acct:<id>
keys, export/import with secrets excluded and listed, Advanced
fencing for secrets; activity log (new activity_logs table) powers
the History timeline and the settings change log; undo for renames
(per-video, from TitleHistory) and for whole schedule applies
(scheduler.RestoreSchedule restores prior publish state via the API);
toasts, progress centre, quota meter + accounting on sync/apply/
schedule, plain-language error mapping with copyable reports.
Verified: build + vet + gofmt clean, node --check, live smoke of
settings GET/PUT/reset/export/import/per-account and the activity
trail. Honest partials: playlist edits have no undo endpoint and
were not faked; confirmations added only for demo clear, password
change, and settings import.
