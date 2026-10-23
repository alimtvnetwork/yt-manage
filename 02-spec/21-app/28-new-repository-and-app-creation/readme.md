# 28 — New Repository and App Creation (yt-manage)

**Status:** Approved (feasibility confirmed by the user, 2026-10-23)
**Repo:** alimtvnetwork/yt-manage
**Language:** Go (module `github.com/alimtvnetwork/yt-manage`)

## 1. Purpose

Create a new repository and a new application, **yt-manage**: a Go CLI in
the spirit of gitmap for full multi-account YouTube management. One tool
connects many YouTube accounts, mirrors their content into a local split
SQLite database, and performs the repetitive channel operations —
search, batch rename, episode grouping, cadence scheduling, comment
automation, playlist editing — that are impractical in YouTube Studio.

The repo-cache blueprint written alongside this spec
(`XX-yt-manage` in repo-cache) generalises the pattern so a future
"new repository in Golang CLI" handover (for example a GitHub or GitLab
manager) reproduces the same shape: installer, self-updater, command
help, REST API + browser UI, split DB, multi-account OAuth.

## 2. Feasibility verdict (confirmed before build)

- **Doable.** The YouTube Data API v3 covers channels, videos (read and
  update), playlists, and comments (insert/update). Scheduling is
  `status.publishAt` on a private video.
- **One hard limit:** the API has **no comment-pin endpoint**. Pinning
  is Studio-only. yt-manage posts/edits the first comment and reports
  the pin as a manual Studio step — never fakes it.
- **"Draft"** is not an API state; it is represented as private +
  unscheduled.
- **Quota posture:** default quota is 10,000 units/day. `search.list`
  costs 100 units, so search runs against the local mirror, never the
  API. Sync uses `channels.list`/`playlistItems.list`/`videos.list`
  (1 unit per page/call).
- **OAuth:** one refresh token per account, encrypted at rest
  (AES-GCM). A Google Cloud OAuth client is supplied by the user; the
  consent screen must leave Testing mode or tokens expire in 7 days.

## 3. Feature summary

1. Multi-account OAuth connect; per-account encrypted token store.
2. Sync engine mirroring channels, videos/shorts (visibility, schedule,
   views, comment counts), playlists, comments into the split DB.
3. AUM-style mass search over the local mirror (FTS5) by title or
   description content.
4. Batch rename/edit by search with an explicit confirmation step;
   Unicode-safe (NFC normalisation, no visual-order corruption).
5. **Episode grouping (the core feature):** automatic grouping of all
   parts of an episode across inconsistent titles, sequencing of parts,
   same-day publishing for an episode's parts, and cadence scheduling
   (every day / 1 / 2 / 3 weeks) across a series of 5–10 videos.
   Detailed in `03-episode-grouping-and-scheduling.md`.
6. Comment automation: post the first comment, edit it later; pin is a
   documented manual step (see §2).
7. Playlist create/edit (title, description) from CLI and UI.
8. Email send/receive around publish events (go-email-reader patterns).
9. REST API + modern CSS3 browser UI: browse and bulk-edit videos,
   pattern finding, A/B title tests.
10. `llm train` export; CLI-to-CLI peer sync of the cache DB.
11. Installer + self-updater following gitmap practices; CI/CD with
    GitHub-verified gates and tag-driven releases.

## 4. Non-negotiables inherited from the task

- Pull before work; spec and plan before code (this folder + the plan
  in `.ai-memory/plans/pending/28-new-repository-and-app-creation.md`).
- Search the codebase only via gitmap (`aum search`, `find`, `cat`).
- Relative git paths only, including release pages and notes.
- One atomic commit + push per task; tests via CI.
- Coding guidelines: repository pattern for all data access, enums with
  String/Parse/IsValid/All, positive boolean naming, simple if/else.

## 5. Document map

- `02-architecture-and-data-model.md` — packages, split-DB schemas,
  repository pattern, sync and quota design.
- `03-episode-grouping-and-scheduling.md` — title parsing, grouping,
  sequencing, cadence rules.
- `97-acceptance-criteria.md` — verifiable completion criteria.
