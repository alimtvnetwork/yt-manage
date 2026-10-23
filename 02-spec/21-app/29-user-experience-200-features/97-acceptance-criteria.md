# 29 — Acceptance Criteria

Spec: 02-spec/21-app/29-user-experience-200-features/
Plan: .ai-memory/plans/pending/29-user-experience-200-features.md

## Per-item criteria (all 200)

- [ ] The item works in the surface it names (UI, CLI, or both) against a
      real connected account and against the demo dataset (item 4).
- [ ] Any write the item introduces is dry-run/preview first and appears
      in History with undo where the API allows reversal.
- [ ] Any setting the item introduces is in Settings (UI + `settings`
      CLI), applies instantly, and shows its default.
- [ ] Keyboard path exists for every UI action the item adds; the
      shortcut appears in the `?` overlay.
- [ ] Errors from the item follow item 65 (plain language + details).

## First 100 complete when

- [ ] A new user, unaided, goes from install to first successful sync
      and one applied rename in under 15 minutes (wizard path, items
      1–10).
- [ ] UI first paint from the local mirror is under 1 second on a
      5,000-video library (item 79 budget, guarded in CI).
- [ ] Settings screen covers every key in system.db settings; CLI parity
      verified by an automated diff of setting names (items 51, 60).
- [ ] Sync mirrors comments and the UI shows last comment-sync per
      channel (item 97).
- [ ] README no longer claims "scaffolding phase" (item 99).

## Next 100 complete when

- [ ] Suggestion inbox (item 110) is the only place suggestions surface;
      no other screen nags.
- [ ] Every automation (items 111–120) can run in propose-only mode
      (item 116) and every firing is in History with undo.
- [ ] Pin checklist (item 122) reflects reality: no video is shown as
      pinned by the API; the manual step is tracked, not faked.
- [ ] Export bundle (item 174) restores a working setup on a clean
      machine via import preview (item 175), verified end to end.
- [ ] All UI strings come from the localisation catalog (item 197).

## Verification posture

- Behaviour verified through CI (repo rule: no local test-suite runs as
  the gate) plus a scripted UI smoke pass against the demo dataset.
- Each wave of the plan closes only when its items pass the per-item
  criteria above; partial waves stay open in the plan file.
