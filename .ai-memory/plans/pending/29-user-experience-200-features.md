# Plan 29 — User Experience: 200 Features (First 100 + Next 100)

**Spec:** 02-spec/21-app/29-user-experience-200-features/
**Execution:** execute-parent-task-with-n-steps-v6 (A=2 workers per wave,
disjoint file ownership, workers never run git; the lead commits).
**Status:** pending (proposed 2026-10-24 at the user's request)
**Depends on:** Plan 28 (completed 2026-10-23) — the working product.

## Why

The engine is done (15 CLI groups, local mirror, browser UI, REST API);
the surface is thin (UI ≈ 760 lines, settings have no user-facing
surface, no onboarding, no undo). This plan adds 200 UX features in two
sets so the product feels fluid: the First 100 make it feel good today,
the Next 100 make it feel smart tomorrow. Full item text lives in the
spec; this plan is the execution tracker.

## Set 1 — First 100: make it feel good today

Spec file: 02-spec/21-app/29-user-experience-200-features/02-first-100-features.md

- [x] Wave 1 (Subtask 01) — Items 1–25: first-run wizard, OAuth status,
      demo channel, first-sync progress, sidebar navigation, scope
      switcher, deep links, instant search, filter chips.
- [x] Wave 2 (Subtask 02) — Items 26–50: saved/recent searches,
      change-set preview table, template builder, staged apply, episode
      board, calendar schedules, cadence previews.
- [x] Wave 3 (Subtask 03) — Items 51–75: real Settings screen + `settings`
      CLI (instant apply, per-account, export/import), toasts, undo,
      history, progress centre, quota meter, multi-select, bulk bar.
- [x] Wave 4 (Subtask 04) — Items 76–100: command palette, CLI
      autocomplete/aliases/defaults, themes, accessibility pass, dry-run
      everywhere, pre-flight checks, token health, comment sync (closes
      the help-text gap), honest docs.

## Set 2 — Next 100: make it feel smart tomorrow

Spec file: 02-spec/21-app/29-user-experience-200-features/03-next-100-features.md
Starts only after Set 1 closes (suggestions and automation build on
preview/undo/settings).

- [x] Wave 5 (Subtask 05) — Items 101–125: pattern panel, suggestion
      inbox, rules engine, approval queue, post-publish pipeline,
      comment centre, pin checklist.
- [x] Wave 6 (Subtask 06) — Items 126–150: comment coverage, playlist
      board + gap/duplicate finders, tags manager, channel dashboard,
      series scorecards, A/B results view, publish heatmap.
- [ ] Wave 7 (Subtask 07) — Items 151–175: all-accounts overview,
      cross-account search, account health centre, publish digests and
      alerts, notification matrix, peer status screen, export bundle.
- [ ] Wave 8 (Subtask 08) — Items 176–200: import preview, backups,
      contextual help + walkthroughs, What's New, personal home screen,
      focus mode, time-zone honesty, localisation-ready strings,
      annual wrap-up.

## Rules for every wave

- Spec before code; item numbers are the unit of work and of reporting
  ("items 31–40 done" — never vague percentages).
- Every write: preview first, History entry, undo where the API allows.
- Every new setting lands in Settings UI + `settings` CLI together.
- Verify via CI only (repo rule); UI waves also run the demo-dataset
  smoke pass from the spec's acceptance criteria.
- One atomic commit per wave, pushed; this plan's checkboxes updated in
  the same commit.

## Closeout

- [ ] All 200 items checked against
      02-spec/21-app/29-user-experience-200-features/97-acceptance-criteria.md.
- [ ] README status and feature list updated to match shipped UX.
- [ ] This plan moved to .ai-memory/plans/completed/ with a closure note.
