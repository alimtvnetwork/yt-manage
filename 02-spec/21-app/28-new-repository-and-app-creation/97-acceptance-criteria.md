# 28 — Acceptance Criteria

Verified against the GitHub check-runs API and the shipped artifacts.

## Repository and scaffolding

- [ ] Repo `alimtvnetwork/yt-manage` exists (private), cloned locally,
      registered in `gitmap scan`, repo-local git identity set.
- [ ] Coding-guideline payload installed (02-spec tree, linters,
      scripts) — installer defects, if hit, are documented and filed.
- [ ] `yt-manage repo` side chat created and seeded.
- [ ] This spec + plan + subtasks committed with relative paths only.

## Core (wave 1)

- [ ] `yt-manage help` lists every command group; `version` prints.
- [ ] Split DBs are created on first run and migrate cleanly; all data
      access goes through `internal/repo`.
- [ ] Token encryption round-trips (test on CI).

## YouTube (wave 2)

- [ ] `account add` completes the OAuth consent flow and stores an
      encrypted refresh token; `account list` shows channels.
- [ ] `sync` mirrors videos with visibility, publishAt, views, and
      comment counts; a second sync is incremental.
- [ ] `search` finds videos by title and by description content from
      the local mirror (zero API quota).
- [ ] `rename` shows a change-set, requires confirmation, applies via
      the API, and is Unicode-safe.

## Features (wave 3)

- [ ] Episode grouper reproduces the §6 worked example from spec 03,
      including the no-part-marker = part 1 rule, as a CI test.
- [ ] Scheduler produces a same-day-per-episode plan on the chosen
      cadence and applies `publishAt` after confirmation.
- [ ] First comment can be posted and edited; the pin limitation is
      surfaced in help and output (never faked).
- [ ] Publish-event email sends; inbox check reads.

## Platform (wave 4)

- [ ] `serve` exposes the REST endpoints in spec 02 §5 and the UI
      supports bulk edit with confirm token.
- [ ] Peer push/pull exchanges rows between two data dirs.
- [ ] `llm train export` emits valid JSONL.
- [ ] `update` verifies checksums before swapping the binary.

## Delivery (wave 5)

- [ ] CI green on GitHub (gofmt, vet, build, test, cross-compile).
- [ ] Tagging `v0.1.0` produces a release with plain-named binaries
      and `checksums.txt`; `install.sh` installs from it.
- [ ] repo-cache blueprint `XX-yt-manage` + README.index committed.
