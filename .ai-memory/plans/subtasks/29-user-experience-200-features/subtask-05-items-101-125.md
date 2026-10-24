# Subtask 05 — Items 101–125: Suggestions, automation, comment centre

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 03-next-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 101. Pattern finder, promoted
- [x] 102. "Videos like this" grouping hints
- [x] 103. Title health suggestions
- [x] 104. Best cadence hint
- [x] 105. Dead-slot detector
- [x] 106. Thumbnail/title pairing check
- [x] 107. Series rename wizard
- [x] 108. Smart defaults per channel
- [x] 109. Anomaly alerts after sync
- [x] 110. Suggestion inbox with snooze
- [x] 111. Auto-sync on a schedule
- [x] 112. Rules engine
- [x] 113. Rule activity log
- [x] 114. Post-publish pipeline
- [x] 115. Watch folders for series
- [x] 116. Approval queue
- [x] 117. Quiet hours
- [x] 118. Workflow templates
- [x] 119. CLI scripting friendliness
- [x] 120. Webhook out
- [x] 121. Comment centre
- [x] 122. Pin checklist
- [x] 123. First-comment templates
- [x] 124. Scheduled first comments
- [x] 125. Comment edit history

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Wave 5 shipped - Set 2 opened. Suggestion engine (internal/suggest)
computes patterns/numbering issues, grouping hints, title health,
real cadence, dead slots, and thumbnail review lists from the mirror
only; series detection groups at show level (episodes.ShowKey).
Anomaly summaries come from a per-account stats snapshot compared
at each sync (item 109). Inbox with dismiss/snooze/reset per type.
Automation: rules engine (match -> rename/comment/playlist, apply
or propose, seen-set so a rule never re-fires on a video), dry-run,
approval queue, rule activity in History, auto-sync loop in serve
(per-account interval from Wave 3 settings, pause control, next-run
in /api/status), post-publish pipeline (comment -> playlist ->
email via the existing mail watcher), watched-series schedule
drafts as proposals, quiet hours (held-run summary + held
webhooks), signed webhooks (HMAC-SHA256), workflow bundle
export/import. Comment centre with pin tracking (manual Studio
pin confirmed in-app, persisted), templates with tokens + real
preview, scheduled first comments via the pipeline, and comment
edit history with restore (recorded inside EditFirstComment).
CLI: --json on the read commands. Verified live against the demo
dataset (suggestions, series, dry-run matched 7, defaults,
workflow export, snooze state, --json outputs). Honest partials:
automations run only while `serve` is up; grouping from a
grouping suggestion still uses the CLI group command (no group
endpoint); the series wizard narrows filter-built plans by
include-set like item 74.
