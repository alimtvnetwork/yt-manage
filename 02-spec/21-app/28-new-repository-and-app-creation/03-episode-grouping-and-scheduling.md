# 28 — Episode Grouping and Scheduling

This is the core feature — the reason yt-manage exists.

## 1. The problem

A series is uploaded with inconsistent titles. The same episode appears
as, for example:

- `Robo School EP3` — no part marker at all (it is part 1)
- `Robo School EP 03 - Part 2`
- `Robo School ep03 part three`
- `Robo School - Episode 3 (Part 03)`

Titles differ in spacing, zero-padding, word-vs-digit part markers, and
placement. A human sees one episode with three parts; tooling must too.

## 2. Title parser

`internal/episodes` extracts, per title:

- **Series key:** the title with episode/part markers, digits, and
  separators removed, lowercased, whitespace-collapsed. Videos sharing
  a series key are candidates for one series.
- **Episode number:** first match of the ordered patterns
  `(?i)\bep(?:isode)?\s*[-.]?\s*0*(\d+)`, `(?i)\be\s*0*(\d+)\b`,
  or a trailing bare number when no other marker exists.
- **Part number:** `(?i)\bpart\s*[-.]?\s*0*(\d+)`,
  `(?i)\bpart\s+(one|two|three|four|five|six|seven|eight|nine|ten)\b`,
  `(?i)\((\d+)\s*/\s*\d+\)` (e.g. `(2/3)`), or `nil`.
- A video with an episode number and **no part marker is part 1**
  (the `EP3` case). Parts are ordered by part number; ties and
  part-less duplicates fall back to upload order (`published_at`,
  then video id for determinism).

Parsing is pure and table-tested: digits, zero-padding (`03` == `3`),
word numbers, and mixed placement are all covered.

## 3. Grouping

Group key = (channel, series key, episode number). Every video with a
parseable episode number joins exactly one episode group; groups are
materialised into `episodes` + `episode_parts` with a stable
`episode_key` (`series|ep<number>`) so re-runs are idempotent.
Unparseable videos are reported, never silently dropped.

## 4. Sequencing

Within an episode, parts are sequenced 1..n by part number (then
upload order). The sequence is stored and shown in CLI and UI; a
sequence warning is raised when part numbers have gaps (e.g. parts
1 and 3 with no 2).

## 5. Cadence scheduling

A `schedule_rule` per channel/series sets:

- `cadence_days`: 1 (daily), 7, 14, or 21 (or custom),
- `starts_at`: the first episode's publish date/time,
- episode ordering: by episode number.

Applying a rule sets, for episode *k* (0-based), the publish date
`starts_at + k × cadence_days`, and assigns **every part of the
episode the same `publishAt`** (same day, staggered by minutes in
sequence order so playlist order is stable). Each video is updated
through the API as private + `status.publishAt` (ISO 8601 UTC).
Already-public videos are skipped and reported — scheduling never
unpublishes without an explicit `--include-public` confirmation.

Series size is 5–10 videos per scheduling run in the UI sense: the
scheduler pages through episodes in order and shows the full plan
(date table) for confirmation before any API write — the same
plan-then-confirm discipline as batch rename.

## 6. Worked example

Titles: `EP3`, `EP 03 Part 2`, `ep03 part three`, `EP4`, `EP 04 - Part 2`,
rule = every 3 weeks from 2026-11-02 10:00.

- Episode 3 → parts [EP3, EP 03 Part 2, ep03 part three], all
  `publishAt` 2026-11-02 (10:00, 10:05, 10:10).
- Episode 4 → parts [EP4, EP 04 - Part 2], `publishAt` 2026-11-23.
