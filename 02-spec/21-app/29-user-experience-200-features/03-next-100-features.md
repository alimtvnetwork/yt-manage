# Next 100 — Make It Feel Smart

Items 101–200 of spec 29. This set assumes the First 100 have landed:
the surface is fast, safe, and familiar. Now the product starts doing the
thinking with the user — suggesting, automating, warning early, and
teaching — while staying quiet and reversible.

## Theme 11 — Smart suggestions & patterns (101–110)

101. **Pattern finder, promoted** — the existing pattern finder becomes a
     home-screen panel: repeated title structures, series detection, and
     numbering inconsistencies, each with a one-click fix path.
102. **"Videos like this" grouping hints** — ungrouped videos that match
     an existing episode pattern are suggested after every sync.
103. **Title health suggestions** — duplicate titles, missing episode
     numbers, ALL-CAPS runs, and over-length titles listed per channel
     with batch-fix previews.
104. **Best cadence hint** — from a channel's actual publish history,
     suggest the cadence that matches its real rhythm.
105. **Dead-slot detector** — scheduled slots the channel historically
     underperforms in are flagged before applying a plan (labelled as a
     hint, never a block).
106. **Thumbnail/title pairing check** — videos whose titles promise a
     part number the thumbnail lacks are listed for review.
107. **Series rename wizard** — detect a whole series, propose one clean
     naming scheme for all of it, preview, apply (recipe saved).
108. **Smart defaults per channel** — rename templates, cadences, and
     comment text remembered per channel and pre-filled next time.
109. **Anomaly alerts after sync** — view/comment counts that drop,
     videos that vanished, or visibility that changed on YouTube are
     summarised as "3 things changed since last sync".
110. **Suggestion inbox with snooze** — all suggestions collect in one
     inbox; accept, dismiss, or snooze per type. Nothing nags twice.

## Theme 12 — Automation, rules & workflows (111–120)

111. **Auto-sync on a schedule** — per-account background sync interval
     (e.g. hourly) with a visible next-run time and pause control.
112. **Rules engine** — "when a new video matches X, then rename /
     add to playlist / post comment" rules, built in a form, dry-run
     against history before activation.
113. **Rule activity log** — every rule firing listed with what it
     changed and an undo, in the History view.
114. **Post-publish pipeline** — a per-channel chain (post first comment →
     email me → add to playlist) that runs when a scheduled video goes
     live.
115. **Watch folders for series** — a series marked "ongoing" auto-groups
     new parts and drafts the next schedule slots for approval.
116. **Approval queue** — automated actions can be set to "propose, don't
     apply": proposals queue for one-click batch approval.
117. **Quiet hours** — automations and emails respect a user-set quiet
     window; queued items summarise afterwards.
118. **Workflow templates** — shareable bundles (rules + recipes +
     cadence) exportable as one file, importable on another machine.
119. **CLI scripting friendliness** — `--json` output on every read
     command and stable exit codes, so users can build their own
     automations around yt-manage.
120. **Webhook out** — publish/sync/apply events POSTed to a user URL
     (signed), enabling external tools without polling.

## Theme 13 — Comments & community (121–130)

121. **Comment centre** — one screen for first comments across channels:
     posted, pending, needs-pin-in-Studio, with direct Studio links.
122. **Pin checklist** — since pinning is Studio-only, a checklist tracks
     which videos still need the manual pin, deep-linking each video's
     comment in Studio. Cleared items stay cleared.
123. **First-comment templates** — per-channel templates with tokens
     ({title}, {episode}, links), previewed on real videos.
124. **Scheduled first comments** — the first comment posts automatically
     when its video publishes (via the pipeline, item 114).
125. **Comment edit history** — prior versions of each first comment,
     with restore.
126. **Link checker for comments** — URLs in first comments checked for
     404s and listed when dead.
127. **Comment coverage report** — "82% of public videos have a first
     comment" per channel, with the gaps one click away.
128. **Reply triage (read-only mirror)** — once comment sync exists
     (item 97), top comments by likes are readable in the drawer;
     replies still happen in Studio, linked directly.
129. **Pinned-comment text sync** — after the manual pin, the user marks
     it pinned in yt-manage so coverage and history stay truthful.
130. **Bulk comment post with preview** — post the templated first comment
     to a selected set, change-set style, with per-item results.

## Theme 14 — Playlists & organisation (131–140)

131. **Playlist board** — playlists as cards with counts, privacy, and
     last-updated; drag videos between playlists in a two-pane view.
132. **Series → playlist one click** — create/update a playlist from an
     episode series in part order, keeping it in sync as parts publish.
133. **Playlist gaps finder** — videos in a series missing from its
     playlist (and vice versa) listed with one-click repair.
134. **Duplicate detector** — same video twice in a playlist, or near-
     duplicate playlists, flagged with merge/remove actions.
135. **Playlist templates** — title/description templates with tokens
     for new playlists, per channel.
136. **Bulk privacy & ordering** — change playlist privacy, or sort a
     playlist by episode number/date, with preview.
137. **Orphan video finder** — public videos in no playlist, grouped by
     likely series, with bulk add.
138. **Playlist publish checklist** — description present, order correct,
     privacy set: a pre-share checklist per playlist.
139. **Shorts vs long-form views** — every list can split or filter
     shorts and long-form, with separate counts.
140. **Tags manager** — all tags across a channel with counts; rename or
     merge a tag everywhere, preview first.

## Theme 15 — Analytics, insights & A/B (141–150)

141. **Channel dashboard** — totals and trends from the local mirror:
     uploads per month, views by series, comment coverage, schedule
     adherence. Local data, clearly labelled as such.
142. **Series scorecards** — per-series views/comments per episode in a
     table + sparkline, exposing drop-off points between parts.
143. **A/B tests, visualised** — the existing A/B feature gains a results
     view: both titles, duration, views/comments delta, and a "pick
     winner" action that applies and ends the test.
144. **A/B auto-schedule** — tests start/stop on a chosen date with the
     mid-way snapshot saved, so results are comparable.
145. **Best/worst performers lists** — top and bottom videos by views and
     by comments per series, refreshed each sync.
146. **Publish-time heatmap** — a channel's historical publishes by
     weekday/hour with performance overlay; cadence picks consult it.
147. **Growth snapshots** — point-in-time snapshots per sync, diffable:
     "since last month: +38 videos, +12 scheduled".
148. **Export insights** — any dashboard or scorecard exports to CSV/PDF
     for reports.
149. **Goal tracking** — user sets a target (e.g. "all series scheduled
     through December"); progress shows on the dashboard.
150. **Insight explanations** — every chart states its source and window
     ("local mirror, last 90 days"); no mystery numbers.

## Theme 16 — Multi-account mastery (151–160)

151. **All-accounts overview** — one screen: every account, its channels,
     last sync, token health, and quota, colour-coded by attention need.
152. **Cross-account search** — search the whole mirror across accounts
     with account/channel labels on every result.
153. **Compare channels** — side-by-side stats for two channels (any
     accounts): cadence, coverage, series counts.
154. **Account health centre** — expired tokens, quota exhaustion, and
     sync failures aggregated with one Fix action each.
155. **Bulk sync all** — sync every account with combined progress and a
     single summary; failures isolated per account.
156. **Per-account homes** — jumping into an account lands on its own
     dashboard, not a global list.
157. **Channel favourites** — pin key channels to the top of every
     switcher and overview.
158. **Move a series between channels (plan only)** — a planning view
     showing what a cross-channel move would require in Studio, as a
     checklist (the API cannot move videos; the plan says so plainly).
159. **Account nicknames & colours** — user-named, colour-coded accounts
     so scopes are recognisable at a glance everywhere.
160. **Session switch without restart** — switching the active account in
     the CLI via a default-account setting (item 78) and in the UI via
     the switcher (item 13), with no re-auth prompts when tokens are valid.

## Theme 17 — Notifications & publish lifecycle (161–170)

161. **Publish-day digest** — one email (via the existing `mail` group)
     listing what published today per channel, with links; never one
     email per video unless asked.
162. **Failure-first alerts** — schedule misses, failed applies, and token
     expiries email immediately; successes stay in the digest.
163. **Notification preferences per event** — a matrix: event × channel
     (digest / instant / off), in Settings, applied instantly.
164. **"Going live in 1 hour" reminders** — optional pre-publish email so
     the user can do the manual pin and checks on time.
165. **Publish timeline view** — past and future publishes on one
     timeline per channel, with status (published, scheduled, missed).
166. **Missed-publish recovery** — a scheduled video that did not publish
     gets a banner + email with reschedule actions.
167. **Inbox inside the app** — publish emails also land in an in-app
     inbox (the mailer already reads IMAP), searchable, linked to videos.
168. **Weekly channel report** — a Monday email: published, scheduled
     ahead, coverage, top performer, one suggestion (from item 110).
169. **Unsubscribe-safe links** — every automated email carries a
     one-click "stop these" link to the local preferences screen.
170. **Test-send button** — every email template previewable and
     test-sendable to the user's own address from Settings.

## Theme 18 — Collaboration, peer sync & portability (171–180)

171. **Peer status screen** — the existing `peer` sync gains a UI: known
     peers, last sync, what was shared, with pause/remove.
172. **Selective sharing** — choose which channels/accounts a peer
     receives; nothing shares by default, and the choice is visible.
173. **Conflict view for peer sync** — when both sides changed the same
     record, a side-by-side pick-winner screen instead of silent overwrite.
174. **Full export bundle** — one archive: mirror export, settings,
     recipes, rules, history — for moving machines or backup.
175. **Import with preview** — importing a bundle previews what will be
     added/changed/skipped before anything is written.
176. **Backup schedule** — automatic local backups of the 4 DBs on a
     cadence, with restore points listed in Settings.
177. **Read-only share link (LAN)** — serve a read-only UI to a second
     person on the local network with a token link and expiry.
178. **Activity attribution** — history entries record which machine/peer
     made the change, so shared setups stay auditable.
179. **Offline queue** — applies attempted while offline queue with their
     change-sets and run on reconnect, after a re-preview.
180. **Version-skew warnings** — peers on incompatible versions are
     warned before sync, with the update action attached.

## Theme 19 — Help & in-app learning (181–190)

181. **Contextual help on every screen** — a "?" panel explaining the
     screen's job, its key actions, and its CLI equivalent.
182. **CLI ↔ UI name parity shown** — help panels name the exact CLI
     command for the action on screen, teaching the CLI by using the UI.
183. **Interactive walkthroughs** — 2-minute guided tours (first rename,
     first schedule, first A/B) on demo data, replayable anytime.
184. **What's New panel** — after `update apply`, a changelog in plain
     words with "try it" links to the new screens.
185. **Error-to-help linking** — recurring errors link to the relevant
     help section and the setting that fixes them.
186. **Searchable help** — help content indexed in the command palette;
     asking "how do I pin" lands on the pin checklist explanation.
187. **Tooltips that teach once** — first-use tooltips on advanced
     controls, dismissed permanently after acknowledgement.
188. **Example library** — rename templates, cadences, comment templates,
     and rules as copyable examples with explanations.
189. **Keyboard shortcut trainer** — the `?` overlay highlights shortcuts
     the user has never used; gentle, once.
190. **Feedback button** — one-click "this screen confused me" capturing
     screen + sanitised state into a copyable report (item 69).

## Theme 20 — Delight & personalisation (191–200)

191. **Personal home screen** — user pins their own widgets (quota,
     next publishes, suggestion inbox, favourite channels) in their
     order on the home screen.
192. **Custom sidebar order** — reorder/hide sidebar sections; Settings
     stays reachable always.
193. **Celebration moments, restrained** — finishing a series schedule or
     hitting a goal (item 149) earns one tasteful confirmation moment;
     disableable, off in reduced-motion.
194. **Channel-branded accents** — the UI accent colour can follow the
     active channel's colour (nickname colours, item 159) so scope is
     felt, not just read.
195. **Avatar menu with quick actions** — account menu holds sync now,
     token health, settings, and switch account in one place.
196. **Time-zone honesty** — all publish times show the channel's zone
     and the user's zone when they differ; no silent conversions.
197. **Language-ready UI** — all UI strings externalised for translation;
     dates/numbers localised; the CLI help follows the same catalog.
198. **Print-friendly views** — schedules and checklists print cleanly
     (for the Studio pin run), one stylesheet, no app chrome.
199. **"Focus mode" for a publish run** — a distilled screen for publish
     day: today's publishes, pin checklist, first comments, nothing else.
200. **Annual wrap-up** — a year-end summary per channel (videos,
     series completed, top performer) generated locally, shareable as an
     exported page. The product says thank you with the user's own data.
