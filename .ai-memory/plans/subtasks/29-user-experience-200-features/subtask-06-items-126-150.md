# Subtask 06 — Items 126–150: Playlists, analytics, A/B

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 03-next-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 126. Link checker for comments
- [x] 127. Comment coverage report
- [x] 128. Reply triage (read-only mirror)
- [x] 129. Pinned-comment text sync
- [x] 130. Bulk comment post with preview
- [x] 131. Playlist board
- [x] 132. Series → playlist one click
- [x] 133. Playlist gaps finder
- [x] 134. Duplicate detector
- [x] 135. Playlist templates
- [x] 136. Bulk privacy & ordering
- [x] 137. Orphan video finder
- [x] 138. Playlist publish checklist
- [x] 139. Shorts vs long-form views
- [x] 140. Tags manager
- [x] 141. Channel dashboard
- [x] 142. Series scorecards
- [x] 143. A/B tests, visualised
- [x] 144. A/B auto-schedule
- [x] 145. Best/worst performers lists
- [x] 146. Publish-time heatmap
- [x] 147. Growth snapshots
- [x] 148. Export insights
- [x] 149. Goal tracking
- [x] 150. Insight explanations

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Backend: comment link checker (HEAD, capped at 20 unique URLs),
coverage report, drawer triage (comments reordered by like
count), bulk comment preview/post with per-item results,
playlist item add/remove, series→playlist in episode/part
order, gaps, duplicates (>=80% overlap), reorder preview +
rebuild apply, playlist privacy via API, orphans grouped by
show, publish checklist, tags now mirrored on sync with
GET /api/tags + rename/merge/remove via POST /api/tags/rename,
insights package (dashboard, scorecards with >40% drop-off
flags, performers, 7x24 heatmap, growth snapshots appended at
each sync, CSV export) — every insights result carries its
source/window meta (item 150). Goals CRUD with computed
progress. A/B results view data + auto-schedule executed by
the serve tick (switch/snapshot/end via batch.Apply +
RestoreSchedule). DTO fixes: playlistDTO.privacy and
commentDTO.pinnedText now exposed, so items 129/136 render
from list endpoints.
Frontend: Comment Centre link check + coverage header + bulk
post flow, drawer top comments, playlist board with two-pane
move (source items enumerated via reorder-preview — there is
no GET playlist-items endpoint), gaps/duplicates/orphans
panels, checklist + privacy + reorder preview, playlist
templates in channel defaults, tags manager card in Videos,
new Insights section (dashboard, scorecards + sparklines,
performers, heatmap, growth, goals), A/B results + schedule
editor, Videos All/Shorts/Long-form split with counts.
Verified live against the demo dataset (dashboard totals,
scorecards, coverage, orphans, goals, gaps, checklist,
heatmap, performers, CSV export, bulk preview, tags empty
state — demo videos carry no tags). Honest partials: insights
export is CSV only (no PDF route exists, none offered);
growth needs real syncs before snapshots exist; playlist
reorder rebuilds via delete + re-insert (quota cost stated
in the UI before applying).
