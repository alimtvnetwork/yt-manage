package peer

// Tests for the peer payload: selective sharing filters,
// version fields, and the newer-wins apply rule. No network:
// everything runs against a temp SQLite mirror.

import (
	"testing"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/db"
	"github.com/alimtvnetwork/yt-manage/internal/model"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// testMirror builds a store with one account, one channel,
// and two videos, and returns the stored channel id.
func testMirror(t *testing.T) (*repo.Store, uint) {
	t.Helper()
	dbs, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	store := repo.NewStore(dbs)
	account := &model.Account{Label: "Test", GoogleSubject: "test-subject"}
	if err := store.Accounts.Create(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	channel := &model.Channel{AccountID: account.ID, YTChannelID: "UC-TEST", Title: "Test Channel"}
	if err := store.Channels.UpsertByYTID(channel); err != nil {
		t.Fatalf("upsert channel: %v", err)
	}
	stored, err := store.Channels.GetByYTID("UC-TEST")
	if err != nil {
		t.Fatalf("get channel: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	for _, spec := range []struct{ yt, title string }{
		{"VID-1", "First Video"},
		{"VID-2", "Second Video"},
	} {
		video := &model.Video{
			ChannelID: stored.ID, YTVideoID: spec.yt, Title: spec.title,
			Visibility: model.VisibilityPublic, LastSyncedAt: &old,
		}
		if err := store.Videos.UpsertByYTID(video); err != nil {
			t.Fatalf("upsert video %s: %v", spec.yt, err)
		}
	}
	return store, stored.ID
}

func TestBuildPayload(t *testing.T) {
	store, _ := testMirror(t)

	payload, err := BuildPayload(store)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	if len(payload.Videos) != 2 {
		t.Errorf("BuildPayload videos = %d, want 2", len(payload.Videos))
	}
	if len(payload.Playlists) != 0 {
		t.Errorf("BuildPayload playlists = %d, want 0", len(payload.Playlists))
	}
	if payload.AppVersion != version.Version {
		t.Errorf("AppVersion = %q, want %q", payload.AppVersion, version.Version)
	}
	if payload.SchemaVersion != PayloadSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", payload.SchemaVersion, PayloadSchemaVersion)
	}
}

func TestBuildPayloadForSelectiveSharing(t *testing.T) {
	store, _ := testMirror(t)

	shared, err := BuildPayloadFor(store, map[string]bool{"UC-TEST": true})
	if err != nil {
		t.Fatalf("BuildPayloadFor shared: %v", err)
	}
	if len(shared.Videos) != 2 {
		t.Errorf("shared payload videos = %d, want 2", len(shared.Videos))
	}

	none, err := BuildPayloadFor(store, map[string]bool{})
	if err != nil {
		t.Fatalf("BuildPayloadFor empty: %v", err)
	}
	if len(none.Videos) != 0 {
		t.Errorf("empty-keep payload videos = %d, want 0", len(none.Videos))
	}

	other, err := BuildPayloadFor(store, map[string]bool{"UC-OTHER": true})
	if err != nil {
		t.Fatalf("BuildPayloadFor other: %v", err)
	}
	if len(other.Videos) != 0 {
		t.Errorf("other-channel payload videos = %d, want 0", len(other.Videos))
	}
}

func TestApplyPayloadNewerWinsAndOlderSkips(t *testing.T) {
	store, _ := testMirror(t)

	// A newer copy renames the video.
	newer := &Payload{
		AppVersion: version.Version, SchemaVersion: PayloadSchemaVersion,
		Videos: []VideoDTO{{
			YtVideoId: "VID-1", ChannelYtId: "UC-TEST",
			Title: "First Video (edited)", Visibility: "public",
			LastSyncedAt: time.Now().Format(time.RFC3339),
		}},
		Playlists: []PlaylistDTO{},
	}
	result, err := ApplyPayload(store, newer)
	if err != nil {
		t.Fatalf("ApplyPayload newer: %v", err)
	}
	if result.VideosApplied != 1 {
		t.Errorf("newer VideosApplied = %d, want 1", result.VideosApplied)
	}
	video, err := store.Videos.GetByYTID("VID-1")
	if err != nil {
		t.Fatalf("get video after apply: %v", err)
	}
	if video.Title != "First Video (edited)" {
		t.Errorf("title after newer apply = %q, want edited title", video.Title)
	}

	// An older copy of the same video is skipped, not applied.
	older := &Payload{
		AppVersion: version.Version, SchemaVersion: PayloadSchemaVersion,
		Videos: []VideoDTO{{
			YtVideoId: "VID-1", ChannelYtId: "UC-TEST",
			Title: "First Video (stale)", Visibility: "public",
			LastSyncedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		}},
		Playlists: []PlaylistDTO{},
	}
	result, err = ApplyPayload(store, older)
	if err != nil {
		t.Fatalf("ApplyPayload older: %v", err)
	}
	if result.VideosSkipped != 1 {
		t.Errorf("older VideosSkipped = %d, want 1", result.VideosSkipped)
	}
	if result.VideosApplied != 0 {
		t.Errorf("older VideosApplied = %d, want 0", result.VideosApplied)
	}
	video, err = store.Videos.GetByYTID("VID-1")
	if err != nil {
		t.Fatalf("get video after stale apply: %v", err)
	}
	if video.Title != "First Video (edited)" {
		t.Errorf("title after stale apply = %q, want the newer title kept", video.Title)
	}
}
