# Plan 28 — New Repository and App Creation (yt-manage)

**Spec:** 02-spec/21-app/28-new-repository-and-app-creation/
**Execution:** execute-parent-task-with-n-steps-v6 (A=2 workers per wave,
disjoint file ownership, workers never run git; the lead commits).
**Status:** in progress

## Setup (done outside the waves)

- [x] Feasibility confirmed with the user (pin = API-impossible,
      documented; draft = private + unscheduled; local-mirror search).
- [x] Repo created (private), cloned, gitmap-registered, identity set.
- [x] `yt-manage repo` side chat created and seeded.
- [x] Coding-guideline payload installed. DEVIATION: `gitmap cg
      install` fails on Linux — install.sh has 3 defects (counter
      `((x++))` under `set -e` kills the first merge; `local` at top
      level line 620 blocks the manifest; required-files check
      contradicts the current payload). Ran a scratch copy with the
      counter sites patched to `+=1`; all merges landed; remaining
      top-level files copied manually; `coding-guidelines.json`
      manifest not generated. Defects filed to the
      coding-guidelines-v24 repo chat 2026-10-23.
- [x] Spec + this plan + subtasks written.

## Waves

- [x] Subtask 01 — Core: module, config, cryptox, models/enums, split
      DB, repositories, CLI skeleton + help.
- [x] Subtask 02 — YouTube: OAuth connect, API client, sync engine,
      mirror search, batch rename/edit with confirmation.
- [x] Subtask 03 — Features: episode grouper, cadence scheduler,
      comment automation, publish email.
- [x] Subtask 04 — Platform: REST API + CSS3 UI, peer sync, llm train
      export, self-updater.
- [x] Subtask 05 — Delivery: CI workflow, release workflow,
      install.sh, unit tests for pure logic, README completion.
      (Lead fix on consolidation: the vendored
      linter-scripts/validate-guidelines.go carried a vet finding —
      a Println argument ending in a redundant newline — so CI scoped
      vet/test around it. The lead fixed that one line in-repo and
      restored repo-wide `go vet ./...` / `go test ./...` gates.)

## Closeout

- [ ] Atomic commits per subtask, pushed; CI verified via GitHub
      check-runs API (not `gitmap pe` alone).
- [ ] Tag v0.1.0; release assets verified (plain binary name,
      checksums.txt).
- [ ] repo-cache blueprint `XX-yt-manage` + README.index (separate
      repo, separate commit).
- [ ] This plan moved to completed with a closure note.
