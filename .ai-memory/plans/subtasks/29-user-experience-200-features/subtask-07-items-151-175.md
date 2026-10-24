# Subtask 07 — Items 151–175: Multi-account, notifications, peer & export

**Plan:** .ai-memory/plans/pending/29-user-experience-200-features.md
**Spec:** 02-spec/21-app/29-user-experience-200-features/ 03-next-100-features.md
**Status:** completed (2026-10-24)

## Items

- [x] 151. All-accounts overview
- [x] 152. Cross-account search
- [x] 153. Compare channels
- [x] 154. Account health centre
- [x] 155. Bulk sync all
- [x] 156. Per-account homes
- [x] 157. Channel favourites
- [x] 158. Move a series between channels (plan only)
- [x] 159. Account nicknames & colours
- [x] 160. Session switch without restart
- [x] 161. Publish-day digest
- [x] 162. Failure-first alerts
- [x] 163. Notification preferences per event
- [x] 164. "Going live in 1 hour" reminders
- [x] 165. Publish timeline view
- [x] 166. Missed-publish recovery
- [x] 167. Inbox inside the app
- [x] 168. Weekly channel report
- [x] 169. Unsubscribe-safe links
- [x] 170. Test-send button
- [x] 171. Peer status screen
- [x] 172. Selective sharing
- [x] 173. Conflict view for peer sync
- [x] 174. Full export bundle
- [x] 175. Import with preview

## Rules

- Preview-first writes, History entry, undo where the API allows.
- New settings land in Settings UI + `settings` CLI together.
- Verify via CI; UI work also passes the demo-dataset smoke pass.

## Closure note (2026-10-24)

Backend: all-accounts overview + health centre (token,
staleness, quota, mail-config checks with named fixes),
channel favourites, account nickname/colour profiles,
channel compare (dashboard reuse + series count + median
publish gap), move-series plan (plan only - the API cannot
move videos, the checklist says so), notification prefs
matrix in the settings KV, publish digest + Monday weekly
report builders, failure alerts wired into manual and auto
sync (quiet hours hold them; held alerts flush as one summary
after the window), going-live reminders from the serve tick,
publish timeline + missed-publish queue, in-app inbox via the
existing IMAP reader, peer management API (add/list/remove),
selective sharing (peer payload filtered by the saved channel
selection on both push and pull - pull enforces the requesting
peer's saved selection; peers with no saved selection keep
the legacy full payload), conflict detection by fetching the
peer payload without applying, pick-a-winner resolve in both
directions, and the full export bundle (mirror + non-secret
settings + UI state) with preview-before-apply import
(newer-wins mirror semantics make re-import safe).
Verified live: overview/health/compare/move-plan/profiles/
prefs validation/test-send error paths/timeline/missed/inbox
against the demo dataset; a real two-instance peer test
(push 12 videos + 1 playlist, edited title on the far side,
conflict found and resolved remote-wins, selective pull
returning an empty payload when nothing is shared); bundle
round-trip preview 0 add / 0 update / 12 skip with an
idempotent apply.
Frontend: overview cards + health panel + sync-all summary,
search-everywhere results labelled with channel + account,
compare table, favourites starring everywhere channels are
listed, focused-account Home strip, header account switcher
persisting default_account, Notifications card (matrix,
digest time, per-channel inclusion, send-now, test sends),
publish timeline, missed banner + panel, Inbox section,
Peers section (cards, sharing editor, push/pull with
progress, conflict side-by-side with keep mine/theirs),
Settings portability card (bundle download + import
preview/apply). Honest partials: item 156 scopes via the
focused channel rather than filtering every select; item 166
reschedule opens the containing episode's schedule form
(there is no single-video schedule endpoint; nothing applies
until the user submits); comment results in search-everywhere
carry labels only when their video is in the result set
(comment DTOs have no channelId).
