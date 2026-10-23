# First 100 — Make It Feel Good Today

Items 1–100 of spec 29. Each item names the surface it extends. The goal
of this set: a new user reaches a successful first sync in minutes, and a
daily user never waits, never re-types, and never loses work.

## Theme 1 — First run & onboarding (1–10)

1. **Guided first-run wizard (UI + CLI)** — `yt-manage serve` on an empty
   data dir opens a 4-step wizard: connect account → first sync → pick a
   channel → done. The CLI prints the same steps when no account exists.
2. **OAuth connect with live status** — the consent flow shows each stage
   (waiting for browser, exchanging token, fetching channels) instead of
   a silent hang, with a cancel button.
3. **Testing-mode warning in plain words** — if the Google consent screen
   is in Testing mode, onboarding says "tokens expire in 7 days — publish
   the consent screen" with a link, at connect time, not after expiry.
4. **One-click demo channel** — a bundled sample dataset (clearly marked
   as demo) so a user can explore search, episodes, and scheduling
   before connecting a real account.
5. **First-sync progress with counts** — channels, videos, playlists
   counted as they land, with elapsed time and a running total, in both
   CLI and UI.
6. **"What next" card after first sync** — three suggested actions based
   on the data found (e.g. "312 videos look like episodes — group them").
7. **Empty-state screens that teach** — every empty list explains what
   belongs there and offers the one action that fills it; no blank pages.
8. **Setup checklist that persists** — a dismissible checklist (connect,
   sync, first rename, first schedule) that tracks real completion and
   stays out of the way once done.
9. **Import existing settings** — detect a `client_secret.json` or env
   vars already present and confirm them in the wizard instead of asking
   again.
10. **Skip-and-return onboarding** — every wizard step can be skipped and
    resumed later from the checklist; nothing is forced.

## Theme 2 — Navigation & layout (11–20)

11. **One sidebar, seven homes** — Videos, Episodes, Schedules, Playlists,
    Comments, Accounts, Settings as a fixed sidebar with icons, labels,
    and live count badges.
12. **Breadcrumb + page title everywhere** — every screen states where
    the user is and what account/channel scope applies.
13. **Global account/channel switcher** — a header switcher that changes
    scope for the whole UI in one click, with the current scope always
    visible (no hidden filters).
14. **Remembered last screen** — reopening the UI returns to the last
    screen, scope, and scroll position.
15. **Resizable, collapsible sidebar** — collapse to icons; the choice is
    remembered per user.
16. **Dense / comfortable density toggle** — row height switch for video
    and episode tables, remembered.
17. **Sticky table headers + sticky action bar** — headers and the bulk
    action bar stay visible while scrolling long lists.
18. **Right-side detail drawer** — clicking a video opens a drawer with
    details, comments, and history without leaving the list.
19. **Consistent page skeleton** — every screen uses the same header,
    filter row, content, and footer pattern so learned habits transfer.
20. **Deep links for everything** — every screen, filter set, and drawer
    state has a shareable URL that restores exactly that view.

## Theme 3 — Search, find & filter (21–30)

21. **Instant search as you type** — FTS5 results update per keystroke
    from the local mirror, with match counts and no loading spinner.
22. **Highlighted matches** — matched words highlighted in titles and
    descriptions, with a snippet around the first description match.
23. **Filter chips** — visibility, channel, date range, duration,
    has-comments, scheduled/unscheduled as toggle chips above results.
24. **Sort that sticks** — sort by date, title, views, episode number;
    the choice persists per screen.
25. **Saved searches** — name and save any search + filter + sort combo;
    saved searches appear in the sidebar with live counts.
26. **Recent searches** — the search box lists recent queries on focus,
    clearable in one click.
27. **Search suggestions from data** — suggestions drawn from actual
    titles/series names in the mirror (e.g. series prefixes), not generic.
28. **"0 results" recovery** — zero-result screens show which filter
    excluded everything and offer one-click removal of that filter.
29. **Keyboard-first search** — `/` focuses search from anywhere; Enter
    opens the first result; arrows move through results.
30. **Search across everything** — one box optionally searches videos,
    playlists, episodes, and comments together, grouped by type.

## Theme 4 — Bulk edit & rename flow (31–40)

31. **Change-set preview as a table** — old → new side by side, with
    per-row include/exclude checkboxes before anything is applied
    (generalises the CLI dry-run).
32. **Template builder with live preview** — the `{title} EP{n}` template
    edited with a token picker; the preview row updates per keystroke on
    real titles.
33. **Find & replace mode** — plain and regex find/replace with match
    counts shown before applying.
34. **Conflict flags in preview** — duplicate resulting titles, empty
    titles, and over-length titles flagged inline with the count of each.
35. **Apply in stages** — apply to 5, review on YouTube, then apply to
    the rest: a built-in canary step for large change-sets.
36. **Progress bar with per-item results** — during apply, each row flips
    to done/failed live; failures stay visible with the reason.
37. **Retry failures only** — one click re-runs just the failed rows of a
    change-set.
38. **Rename history per video** — every title a video has had, with when
    and by which change-set, in the detail drawer.
39. **Save rename recipes** — named templates + find/replace pairs saved
    and re-usable across channels and accounts.
40. **Description & tags in bulk** — the same preview flow extended to
    description appends/prepends and tag add/remove.

## Theme 5 — Episodes & scheduling (41–50)

41. **Episode board view** — episodes as cards grouped by series, parts
    in sequence, gaps (missing part numbers) shown as empty slots.
42. **Drag to resequence** — reorder parts by dragging; numbering and the
    schedule preview update immediately.
43. **Confidence badges on grouping** — each auto-grouped episode shows
    why (matched markers) and a confidence level; low-confidence groups
    are queued for review first.
44. **One-click split / merge** — split an episode at a part, or merge
    selected videos into an episode, directly on the board.
45. **Calendar view of schedules** — planned publishes on a month
    calendar, colour-coded by channel, with conflicts overlapping.
46. **Cadence picker with real dates** — choosing daily / weekly / 2 / 3
    weeks / custom shows the actual first 10 publish dates before apply.
47. **"What moves if I change this" preview** — editing a rule shows
    which already-scheduled videos would shift, before confirming.
48. **Same-day parts, visibly linked** — parts of one episode share a
    date chip; changing one part's date offers to move them together.
49. **Schedule health warnings** — two episodes same day/channel, gaps
    longer than the cadence, and past-dated plans flagged on the board.
50. **Plan vs applied state** — planned-but-not-applied schedules are
    visually distinct everywhere; nothing looks published that is not.

## Theme 6 — Settings that behave (51–60)

51. **A real Settings screen** — the system.db key/value settings get a
    proper UI (and a matching `settings` CLI group): grouped, labelled,
    searchable. Today they have no user-facing surface at all.
52. **Settings apply instantly** — toggles and selects take effect on
    change with an inline "Saved" tick; no Save button, no restart.
53. **Settings search** — type to filter settings by name or keyword
    (e.g. "quota", "email", "theme").
54. **Per-account settings** — sync cadence, email notifications, and
    defaults can differ per account, with a clear "inherited / overridden"
    marker.
55. **Sensible defaults, shown** — every setting displays its default and
    a "Reset to default" action when changed.
56. **Dangerous settings fenced** — data-dir location, master-key, and
    peer-sync keys sit in an Advanced section with confirmation on change.
57. **Export / import settings** — download settings as JSON, import on
    another machine; secrets are excluded and listed as excluded.
58. **Settings change log** — who/what changed, when, old → new, in an
    activity view (and exportable).
59. **Validation before acceptance** — an invalid port, path, or cadence
    is rejected inline with the reason, never saved then failing later.
60. **CLI/UI parity** — every setting readable and writable from both the
    Settings screen and `yt-manage settings get/set`, same names.

## Theme 7 — Feedback: progress, errors, undo (61–70)

61. **Toasts that inform, then leave** — success toasts state the result
    with counts ("Renamed 42 videos") and auto-dismiss; errors persist
    until dismissed.
62. **Undo for applied changes** — every apply (rename, schedule,
    playlist edit) leaves an Undo in the toast and in History, restoring
    prior values via the API.
63. **Global activity/history view** — a timeline of syncs, applies,
    schedules, and errors across accounts, filterable, with undo where
    available.
64. **Long-task progress centre** — syncs and bulk applies show progress
    in a fixed corner panel; the user can keep working elsewhere in the UI.
65. **Errors in plain language** — every error shows what failed, what
    was NOT changed, and the next step; raw API errors are one click
    away under "Details", never the headline.
66. **Quota meter** — today's YouTube API quota use as a bar in the
    header, with the cost of the next planned action estimated before
    it runs.
67. **Offline / token-expired banners** — a dead network or expired
    token shows one clear banner with a Reconnect action, not scattered
    failures per screen.
68. **Confirmation only where it matters** — destructive or wide actions
    confirm with the exact count and scope; routine single edits do not.
69. **Copyable error reports** — one click copies a sanitised error
    bundle (no tokens) for bug reports.
70. **Sound-off, motion-safe feedback** — no sounds by default; animations
    respect the OS reduced-motion setting.

## Theme 8 — Keyboard & speed (71–80)

71. **Full keyboard map** — a documented shortcut set (`?` opens the
    overlay): navigation, search, select, apply, undo.
72. **Command palette** — Ctrl/Cmd+K fuzzy-searches every action, screen,
    saved search, and setting; Enter runs it.
73. **Multi-select like a file manager** — shift-click ranges, ctrl-click
    toggles, and keyboard range selection in all tables.
74. **Bulk action bar follows selection** — selecting rows reveals
    Rename / Schedule / Playlist / Export with live counts.
75. **Recent items quick-jump** — the palette and header list recently
    opened videos/episodes/playlists.
76. **CLI autocomplete** — shell completion for all 15 command groups,
    subcommands, and flags (bash/zsh/PowerShell), shipped with releases.
77. **CLI aliases for the daily path** — short aliases for the five most
    used flows, shown in help; full names always work.
78. **Remembered CLI defaults** — `yt-manage config`-style defaults
    (account, channel) so daily commands need no repeated flags.
79. **Fast cold start** — UI first paint from the local mirror in under a
    second on a typical library; a perf budget test guards this in CI.
80. **Background prefetch** — after a sync, episode grouping and pattern
    results are precomputed so those screens open instantly.

## Theme 9 — Visual polish & accessibility (81–90)

81. **Light, dark, and system themes** — a theme setting that applies
    instantly across the whole UI, charts included.
82. **Channel avatars & thumbnails everywhere** — lists show thumbnails
    and channel avatars; a broken image degrades to a tidy placeholder,
    never a broken-icon box.
83. **Status at a glance** — visibility and schedule status as coloured
    dots + text labels (never colour alone).
84. **Readable type scale** — one type system: titles, meta, and counts
    visually distinct; tabular numbers for counts and dates.
85. **Focus you can see** — a clear focus ring on every interactive
    element; full tab order follows the visual order.
86. **Screen-reader labels** — icon buttons labelled; tables announced
    with row counts; toasts polite-live.
87. **Contrast-safe palette** — text/background pairs meet WCAG AA in
    both themes; checked in CI with the tokens.
88. **Zoom-friendly layout** — UI remains usable at 200% browser zoom;
    no fixed-height clipping of labels.
89. **Touch targets** — interactive targets ≥ 40px in comfortable
    density; tables still work on a tablet.
90. **Empty, loading, and error art** — one consistent illustration/icon
    style for the three states so the app feels finished everywhere.

## Theme 10 — Trust: preview, safety, quota (91–100)

91. **Dry-run is the default everywhere** — schedule apply, playlist
    edits, and comment posts all preview first, matching rename today.
92. **Scope statement on every apply** — "This will change 42 videos on
    Channel X (Account Y)" spelled out on the confirm step.
93. **Pre-flight checks** — before any apply: token valid, quota enough,
    no conflicting schedule; failures listed before a single write.
94. **Audit trail export** — full history of applied changes exportable
    as CSV/JSON for the user's own records.
95. **Read-only mode** — a setting (and `--read-only` serve flag) that
    disables all writes for safe browsing/demos.
96. **Token health screen** — per-account token age, expiry risk, and
    last-successful-call, with reconnect actions; closes the 7-day
    Testing-mode surprise for good.
97. **Sync actually mirrors comments** — close the known gap: help text
    already promises comment mirroring; the syncer gains it, and the UI
    states the last comment-sync time per channel.
98. **Backup before big writes** — automatic local snapshot of affected
    rows before any apply over a threshold (e.g. 50 items), restorable
    from History.
99. **Honest docs in-product** — README status corrected ("scaffolding
    phase" removed), and the UI footer shows the real version from the
    binary, not a stale file.
100. **Privacy note in Settings** — one screen stating what stays local,
     what is sent to YouTube/Google, and that tokens are encrypted at
     rest, in plain words.
