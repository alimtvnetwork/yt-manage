# yt-manage

A Go CLI for full multi-account YouTube management, built following gitmap
practices: split SQLite databases, a repository-pattern data layer, mass
search, a REST API with a modern browser UI, CLI-to-CLI machine sync, an
installer and a self-updater.

## Installation

On Linux or macOS, install the latest release with the installer
script:

```sh
curl -fsSL https://raw.githubusercontent.com/alimtvnetwork/yt-manage/main/install.sh | sh
```

The script detects your OS and architecture, downloads the matching
release archive and `checksums.txt`, verifies the archive's SHA256
against `checksums.txt` (aborting before it installs anything if the
checksum does not match), and installs the plain `yt-manage` binary
into `~/.local/bin` — or into `$PREFIX/bin` when `PREFIX` is set. If
that directory is not on your `PATH`, the script prints the line to
add.

Manual install: download the archive for your platform from the
Releases page — `yt-manage_linux_amd64.tar.gz`,
`yt-manage_darwin_arm64.tar.gz`, or `yt-manage_windows_amd64.zip` —
together with `checksums.txt`. Verify the archive against
`checksums.txt` (for example with `sha256sum --check checksums.txt`
on Linux, or `shasum -a 256` on macOS), unpack it, and place the
plain `yt-manage` binary it contains (`yt-manage.exe` on Windows)
somewhere on your `PATH`.

Build from source: with Go 1.22 or newer, check out this repository
and run:

```sh
go build ./...
go build -o yt-manage .
```

Once installed, keep yt-manage current with its self-updater, which
downloads the latest release archive, verifies its SHA256 against
`checksums.txt`, and swaps the binary:

```sh
yt-manage update check   # report whether a newer release exists
yt-manage update apply   # download, verify and install it
```

## Usage

Quick start, in order:

1. **Connect an account** (OAuth). First configure a Google OAuth
   client: either set the `YT_MANAGE_CLIENT_ID` and
   `YT_MANAGE_CLIENT_SECRET` environment variables, or place a
   `client_secret.json` file in the data dir (see below). Then run:

   ```sh
   yt-manage account add
   ```

   This opens the Google consent flow; the resulting refresh token is
   stored encrypted at rest. Note: if your Google Cloud project's
   consent screen is still in Testing mode, Google expires refresh
   tokens after 7 days — publish the consent screen (leave Testing
   mode) so connected accounts stay connected.

2. **Mirror your channels locally:**

   ```sh
   yt-manage sync run
   ```

3. **Search the local mirror** — full-text search over titles and
   descriptions, served entirely from the local databases, so it
   costs no YouTube API quota:

   ```sh
   yt-manage search <words>
   ```

4. **Batch-rename the matches.** Without `--apply` this is a dry
   run: it prints the change-set (old -> new) and writes nothing.
   Add `--apply` to write the same change-set to YouTube after
   confirmation:

   ```sh
   yt-manage rename --query "old series" --template "{title} EP{n}"
   yt-manage rename --query "old series" --template "{title} EP{n}" --apply
   ```

5. **Group videos into episodes** — titles are parsed for episode
   and part markers, even when they are inconsistent:

   ```sh
   yt-manage episode group
   ```

6. **Schedule episodes on a cadence.** `plan` shows the publish
   dates a rule would produce without writing anything; `apply`
   writes them. All parts of an episode publish on the same day:

   ```sh
   yt-manage schedule plan --channel ID --cadence three-weeks --starts-at 2026-11-01T09:00:00Z
   yt-manage schedule apply --channel ID --cadence three-weeks --starts-at 2026-11-01T09:00:00Z
   ```

   Cadences are `daily`, `weekly`, `two-weeks` and `three-weeks`
   (or a custom `--cadence-days N`).

7. **Serve the REST API and browser UI** (bulk edit, pattern
   finder, A/B tests) on a local address:

   ```sh
   yt-manage serve
   ```

8. **Export LLM training data** (command docs, schema descriptions
   and anonymised usage shapes, as JSONL):

   ```sh
   yt-manage llm train export
   ```

Run any command with `--help` for its full flag list.

### Data dir and secrets

All local state lives in the data dir `~/.yt-manage` (the databases
sit in its `data/` subdirectory). Set the `YT_MANAGE_HOME`
environment variable to use a different location.

OAuth refresh tokens are encrypted with a master key. Set
`YT_MANAGE_MASTER_KEY` to a base64-encoded 32-byte key to supply
your own; when it is unset, yt-manage generates a key file inside
the data dir on first use and reuses it from then on. Keep the key
safe: without it, stored tokens cannot be decrypted.

### Comment pinning

The YouTube Data API has no endpoint for pinning comments. yt-manage
posts and edits the first comment and tells you the manual pin step
in YouTube Studio; it never fakes a pin.

## What it does

- Connect multiple YouTube accounts with OAuth (one refresh token per
  account, encrypted at rest) and keep a local mirror of every channel.
- Read videos and shorts with visibility (public, unlisted, private,
  scheduled, draft), view counts and comment counts.
- Mass search the local mirror AUM-style by title or description content,
  then batch-rename or batch-edit the matches with an explicit
  confirmation step. All edits are Unicode-safe.
- Group videos into episodes automatically, even when titles are
  inconsistent (`EP3`, `EP 03`, `part 2` in different places), sequence
  the parts, and schedule a whole episode on a cadence (every day, every
  week, every 2 weeks, every 3 weeks). All parts of an episode publish
  on the same day.
- Comment automation: post the first comment on a video and edit it
  later. Pinning is a YouTube Studio-only action — the YouTube Data API
  v3 has no pin endpoint, so yt-manage prepares the comment and reports
  the pin as a manual Studio step.
- Create and edit playlists (title and description) from the CLI or UI.
- Send and receive email around publish events (SMTP/IMAP, following the
  go-email-reader patterns).
- REST API plus a modern CSS3 browser UI for browsing and bulk-editing
  the database, running A/B title tests, and finding patterns.
- `llm train` export and CLI-to-CLI peer sync so two machines can
  forward and share one cache database, like gitmap.

## Data layout

Split SQLite databases (gitmap Db concept), normalized schemas, GORM as
the ORM, and one repository per aggregate:

- `accounts.db` — connected accounts and encrypted OAuth tokens
- `content.db` — channels, videos, playlists, comments
- `planning.db` — episodes, parts, schedule rules, A/B tests
- `system.db` — sync state, peers, email log, settings

## Status

Scaffolding phase: repository, coding-guideline install, spec and plan
land first; implementation follows in waves (core CLI and database,
OAuth and sync, search and batch edit, episodes and scheduling,
API/UI/peer/installer).

## Install

Installer and self-updater follow the gitmap pattern and ship with the
first release.
