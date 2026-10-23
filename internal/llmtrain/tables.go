package llmtrain

// commandDoc is one static command documentation record.
type commandDoc struct {
	Name        string
	Usage       string
	Description string
}

// commandDocs covers every command group and subcommand of the
// CLI. Descriptions are one line each and contain no user data.
var commandDocs = []commandDoc{
	{"account add", "yt-manage account add [--label NAME]", "Connect a YouTube account through the OAuth consent flow and store its refresh token encrypted."},
	{"account list", "yt-manage account list", "List connected accounts with their channels."},
	{"account remove", "yt-manage account remove <id> [--yes]", "Disconnect an account and delete its stored token; mirrored content rows are kept."},
	{"sync run", "yt-manage sync run [--account ID] [--full]", "Mirror channels, videos, playlists and comments from YouTube into the local databases."},
	{"sync status", "yt-manage sync status", "Show the last sync time and error per account and resource."},
	{"video list", "yt-manage video list [--channel ID] [--visibility V] [--limit N]", "List mirrored videos, optionally filtered by channel and visibility."},
	{"video show", "yt-manage video show <id>", "Show full details for one mirrored video by local ID or YouTube ID."},
	{"video edit", "yt-manage video edit --id ID [--title T] [--description D] [--visibility V] [--publish-at RFC3339] [--apply]", "Plan or apply an edit to one video: title, description, visibility or scheduled publish time."},
	{"search", "yt-manage search [--channel ID] [--visibility V] [--limit N] [--offset N] <query words...>", "Search the local mirror by title and description at zero API quota cost."},
	{"rename", "yt-manage rename [--query TEXT] [--template T | --regex R [--replacement S] | --description-template T] [--apply]", "Batch rename or edit videos matched by a local search, with a confirmed change-set."},
	{"episode group", "yt-manage episode group [--channel ID]", "Parse video titles and materialise episode groups and their parts."},
	{"episode list", "yt-manage episode list --channel ID", "List the episodes of a channel with their part counts."},
	{"episode show", "yt-manage episode show [--channel ID] <episode-key>", "Show one episode and its parts in sequence order."},
	{"schedule plan", "yt-manage schedule plan (--rule ID | --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339) [--include-public]", "Show the publish-date plan a cadence rule would produce, without writing anything."},
	{"schedule apply", "yt-manage schedule apply (--rule ID | --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339) [--include-public] [--yes]", "Apply a cadence plan: set publishAt on each episode part through the API."},
	{"schedule rules list", "yt-manage schedule rules list --channel ID", "List the cadence rules stored for a channel."},
	{"schedule rules add", "yt-manage schedule rules add --channel ID (--cadence C | --cadence-days N) --starts-at RFC3339 [--videos-per-episode N]", "Store a new cadence rule for a channel."},
	{"schedule rules delete", "yt-manage schedule rules delete --id ID [--yes]", "Delete a stored cadence rule after confirmation."},
	{"playlist list", "yt-manage playlist list --channel ID", "List mirrored playlists for a channel."},
	{"playlist create", "yt-manage playlist create --channel ID --title TITLE [--description TEXT] [--privacy V]", "Create a playlist on a connected channel and mirror it."},
	{"playlist edit", "yt-manage playlist edit (--id ID | --yt-id YTID) [--title TITLE] [--description TEXT] [--apply]", "Plan or apply a title or description edit to a playlist."},
	{"comment post", "yt-manage comment post --video ID (--text TEXT | --text-file FILE)", "Post the first comment on a video; pinning is a manual step in YouTube Studio because the API has no pin endpoint."},
	{"comment edit", "yt-manage comment edit --video ID (--text TEXT | --text-file FILE)", "Edit a first comment previously posted by yt-manage; pinning is a manual step in YouTube Studio because the API has no pin endpoint."},
	{"mail send", "yt-manage mail send --to ADDR --subject TEXT (--body TEXT | --body-file FILE)", "Send an email using the SMTP settings in config.json and log it."},
	{"mail inbox", "yt-manage mail inbox [--limit N]", "List recent inbox messages from the configured IMAP account."},
	{"mail watch", "yt-manage mail watch --to ADDR [--interval DURATION]", "Watch for newly published videos and email a notification for each."},
	{"serve", "yt-manage serve [--addr ADDR]", "Start the local REST API and browser UI."},
	{"peer add", "yt-manage peer add --name NAME --url URL --secret SECRET", "Register another yt-manage instance as a peer for cache sync."},
	{"peer list", "yt-manage peer list", "List registered peers with their base URLs and last-seen times."},
	{"peer push", "yt-manage peer push --peer NAME_OR_ID", "Push the local video and playlist cache to a peer over its signed REST API."},
	{"peer pull", "yt-manage peer pull --peer NAME_OR_ID", "Pull the video and playlist cache from a peer and merge it locally, keeping newer rows."},
	{"llm train export", "yt-manage llm train export [--out PATH]", "Export anonymised training data (command docs, schema, usage shapes) as JSONL."},
	{"update check", "yt-manage update check", "Report the current version and whether a newer GitHub release is available."},
	{"update apply", "yt-manage update apply [--yes]", "Download the latest GitHub release, verify its SHA256 checksum, and swap the binary."},
	{"version", "yt-manage version", "Print the yt-manage version."},
}

// schemaDoc is one static table schema record.
type schemaDoc struct {
	Database string
	Table    string
	Columns  []string
}

// schemaDocs mirrors spec 02 section 2: the four split databases
// and their tables, with the columns of the GORM models.
var schemaDocs = []schemaDoc{
	{"accounts.db", "accounts", []string{"id", "label", "google_subject", "channel_id", "token_ciphertext", "token_nonce", "scopes", "created_at", "updated_at"}},
	{"accounts.db", "channels", []string{"id", "account_id", "yt_channel_id", "title", "handle", "subscriber_count", "video_count", "last_synced_at", "created_at", "updated_at"}},
	{"content.db", "videos", []string{"id", "channel_id", "yt_video_id", "title", "description", "visibility", "is_short", "published_at", "publish_at", "view_count", "like_count", "comment_count", "duration_seconds", "last_synced_at", "created_at", "updated_at"}},
	{"content.db", "playlists", []string{"id", "channel_id", "yt_playlist_id", "title", "description", "item_count", "created_at", "updated_at"}},
	{"content.db", "playlist_items", []string{"id", "playlist_id", "video_id", "position", "created_at", "updated_at"}},
	{"content.db", "comments", []string{"id", "video_id", "yt_comment_id", "author", "text", "is_first", "is_pinned_known", "published_at", "created_at", "updated_at"}},
	{"content.db", "videos_fts", []string{"title", "description"}},
	{"planning.db", "episodes", []string{"id", "channel_id", "episode_key", "episode_number", "title", "cadence_days", "first_publish_at", "created_at", "updated_at"}},
	{"planning.db", "episode_parts", []string{"id", "episode_id", "video_id", "part_number", "sequence", "created_at", "updated_at"}},
	{"planning.db", "schedule_rules", []string{"id", "channel_id", "cadence_days", "videos_per_episode", "starts_at", "is_active", "created_at", "updated_at"}},
	{"planning.db", "ab_tests", []string{"id", "video_id", "variant_a", "variant_b", "winner", "is_active", "created_at", "updated_at"}},
	{"planning.db", "ab_test_snapshots", []string{"id", "test_id", "captured_at", "views_a_context", "ctr_note", "created_at", "updated_at"}},
	{"system.db", "sync_states", []string{"id", "account_id", "resource", "cursor", "last_run_at", "last_error", "created_at", "updated_at"}},
	{"system.db", "peers", []string{"id", "name", "base_url", "shared_secret", "last_seen_at", "created_at", "updated_at"}},
	{"system.db", "email_logs", []string{"id", "direction", "subject", "related_video_id", "sent_at", "created_at", "updated_at"}},
	{"system.db", "settings", []string{"id", "key", "value", "created_at", "updated_at"}},
}
