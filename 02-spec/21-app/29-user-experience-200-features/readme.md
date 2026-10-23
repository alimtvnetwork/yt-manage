# 29 — User Experience: 200 Features (First 100 + Next 100)

**Status:** Proposed (user request, 2026-10-24)
**Repo:** alimtvnetwork/yt-manage
**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Builds on:** 02-spec/21-app/28-new-repository-and-app-creation/

## 1. Purpose

yt-manage already has a strong engine and a thin surface. This spec adds
the surface: 200 user-experience features, in two sets of 100, whose only
job is to make the product feel fluid — fast feedback, no lost work, no
surprises, settings that behave the way the user expects, and help that
arrives before the user has to ask for it.

- **First 100 (02-first-100-features.md)** — make it feel good today:
  onboarding, navigation, search, bulk edit, episodes/scheduling,
  settings, feedback, keyboard, visual/accessibility, trust.
- **Next 100 (03-next-100-features.md)** — make it feel smart tomorrow:
  suggestions, automation, comments, playlists, analytics, multi-account
  mastery, notifications, collaboration, in-app learning, delight.

Nothing here replaces an existing feature. Every item extends a surface
that already exists (a CLI group, the browser UI, the REST API, or the
settings table) so the work lands as polish on a working product, not a
rewrite.

## 2. Current state (verified 2026-10-24, commit d35d94a)

What the user has today:

- 15 CLI command groups: `account`, `sync`, `video`, `search`, `rename`,
  `episode`, `schedule`, `playlist`, `comment`, `mail`, `serve`, `peer`,
  `llm`, `update`, `version`.
- Multi-account OAuth with encrypted tokens; local mirror in 4 split
  SQLite DBs (accounts, content, planning, system); FTS5 search over the
  mirror at zero API quota.
- Batch rename/edit, dry-run by default, with a single y/N confirmation.
- Episode grouping + cadence scheduling (daily / weekly / 2 / 3 weeks).
- Comment post/edit (pin is a documented manual Studio step).
- Browser UI served by `yt-manage serve`: a single page of ~760 lines
  total (index.html 151, app.css 172, app.js 439) covering browse,
  bulk edit, pattern finder, and A/B tests.
- Settings exist only as a key/value table in system.db — no CLI command
  and no UI screen reads or writes them for the user.
- No onboarding flow, no progress reporting on long syncs, no undo,
  no keyboard shortcuts, no theming, no saved views.

Known gaps this spec also closes on the way: README still says
"scaffolding phase"; help text claims sync mirrors comments but the
syncer does not sync comments yet.

## 3. UX principles (apply to all 200)

1. **Instant local feedback.** Anything served from the local mirror
   responds on the keystroke or click — never a spinner for local data.
2. **Never lose work.** Drafts, filters, half-typed renames, and scroll
   positions survive navigation, refresh, and crashes.
3. **Dry-run first, undo always.** Every write previews as a change-set
   (the existing rename behaviour, generalised) and every applied change
   can be undone from history.
4. **Settings apply live.** Changing a setting takes effect immediately,
   with no restart and no re-login, and the user sees the effect before
   leaving the screen.
5. **Plain language, real numbers.** Errors say what happened, what was
   not changed, and what to do next. Quota, counts, and dates are shown
   as numbers, not adjectives.
6. **Keyboard is a first-class path.** Every mouse action in the UI has
   a keyboard equivalent; the CLI mirrors the same names.
7. **One place per job.** Videos, episodes, schedules, comments,
   playlists, accounts, and settings each have exactly one home screen;
   everything else links there.
8. **Quiet by default.** Notifications report publish events and
   failures, never routine success noise the user did not ask for.

## 4. Theme map

First 100:

| Theme | Items | Area |
|---|---|---|
| 1. First run & onboarding | 1–10 | CLI + UI |
| 2. Navigation & layout | 11–20 | UI |
| 3. Search, find & filter | 21–30 | UI + `search` |
| 4. Bulk edit & rename flow | 31–40 | UI + `rename` |
| 5. Episodes & scheduling | 41–50 | UI + `episode`/`schedule` |
| 6. Settings that behave | 51–60 | UI + new `settings` surface |
| 7. Feedback: progress, errors, undo | 61–70 | UI + CLI |
| 8. Keyboard & speed | 71–80 | UI + CLI |
| 9. Visual polish & accessibility | 81–90 | UI |
| 10. Trust: preview, safety, quota | 91–100 | UI + CLI |

Next 100:

| Theme | Items | Area |
|---|---|---|
| 11. Smart suggestions & patterns | 101–110 | UI + API |
| 12. Automation, rules & workflows | 111–120 | CLI + UI |
| 13. Comments & community | 121–130 | UI + `comment` |
| 14. Playlists & organisation | 131–140 | UI + `playlist` |
| 15. Analytics, insights & A/B | 141–150 | UI + API |
| 16. Multi-account mastery | 151–160 | UI + `account` |
| 17. Notifications & publish lifecycle | 161–170 | `mail` + UI |
| 18. Collaboration, peer sync & portability | 171–180 | `peer` + UI |
| 19. Help & in-app learning | 181–190 | UI + CLI |
| 20. Delight & personalisation | 191–200 | UI + CLI |

## 5. Document map

- `02-first-100-features.md` — items 1–100, the fluidity set.
- `03-next-100-features.md` — items 101–200, the intelligence set.
- `97-acceptance-criteria.md` — how completion of each set is verified.
