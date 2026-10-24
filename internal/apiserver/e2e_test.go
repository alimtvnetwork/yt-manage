package apiserver

// End-to-end tests for the shipped API surfaces. Every test
// starts from a fresh temp home, seeds the demo dataset, and
// talks to the real mux over HTTP. Nothing here touches the
// network, a YouTube service, or the GitHub API: every
// endpoint under test reads the local mirror only.
//
// The one deliberate boundary: bulk-edit apply, live peer
// HTTP, and real SMTP are NOT tested here. Bulk apply needs
// a YouTube service, peer push/pull need a second server on
// the network, and SMTP needs a mail server; all three are
// covered by their handler-level error paths where those
// paths are reachable offline (see the notify and peer
// tests below).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/db"
	"github.com/alimtvnetwork/yt-manage/internal/repo"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// e2e is one test server plus its temp home.
type e2e struct {
	t    *testing.T
	base string
	home string
	http *http.Client
}

// newE2E builds a fresh server and seeds the demo dataset:
// 1 account, 1 channel ("Demo Channel"), 12 videos,
// 1 playlist. It fails the test on any setup error.
func newE2E(t *testing.T) *e2e {
	t.Helper()
	home := t.TempDir()
	dbs, err := db.Open(home)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	srv := New(repo.NewStore(dbs), home, []byte("test-key"))
	ts := httptest.NewServer(srv.readOnlyMiddleware(srv.mux))
	t.Cleanup(ts.Close)
	env := &e2e{t: t, base: ts.URL, home: home, http: ts.Client()}
	status, body := env.raw("POST", "/api/demo/seed", nil)
	if status != http.StatusOK {
		t.Fatalf("demo seed: status %d, body %s", status, body)
	}
	return env
}

// raw performs one request and returns the status and body
// without interpreting the envelope.
func (e *e2e) raw(method, path string, payload any, headers ...string) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			e.t.Fatalf("marshal %s %s body: %v", method, path, err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, e.base+path, reader)
	if err != nil {
		e.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := e.http.Do(req)
	if err != nil {
		e.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatalf("read %s %s: %v", method, path, err)
	}
	return resp.StatusCode, body
}

// e2eEnvelope is the API wire shape.
type e2eEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

// call performs one request and fails the test unless the
// e2eEnvelope reports ok. It returns the decoded data payload.
func (e *e2e) call(method, path string, payload any, headers ...string) json.RawMessage {
	e.t.Helper()
	status, body := e.raw(method, path, payload, headers...)
	var env e2eEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		e.t.Fatalf("%s %s: status %d, undecodable body %s: %v", method, path, status, body, err)
	}
	if !env.OK {
		e.t.Fatalf("%s %s: status %d, error %q", method, path, status, env.Error)
	}
	return env.Data
}

// callErr performs one request and returns the e2eEnvelope
// without failing on ok=false, for error-path tests.
func (e *e2e) callErr(method, path string, payload any, headers ...string) (int, e2eEnvelope) {
	e.t.Helper()
	status, body := e.raw(method, path, payload, headers...)
	var env e2eEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		e.t.Fatalf("%s %s: status %d, undecodable body %s: %v", method, path, status, body, err)
	}
	return status, env
}

// decode unmarshals data into out, failing the test on error.
func decode(t *testing.T, data json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
}

// settingValue finds one setting by key in a settings response.
func settingValue(t *testing.T, data json.RawMessage, key string) string {
	t.Helper()
	var resp struct {
		Settings []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"settings"`
	}
	decode(t, data, &resp)
	for _, setting := range resp.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	t.Fatalf("setting %q not in response", key)
	return ""
}

func TestE2EStatus(t *testing.T) {
	env := newE2E(t)
	var status struct {
		AccountCount int    `json:"accountCount"`
		VideoCount   int64  `json:"videoCount"`
		AppVersion   string `json:"appVersion"`
	}
	decode(t, env.call("GET", "/api/status", nil), &status)
	if status.AccountCount != 1 {
		t.Errorf("accountCount = %d, want 1", status.AccountCount)
	}
	if status.VideoCount != 12 {
		t.Errorf("videoCount = %d, want 12", status.VideoCount)
	}
	if status.AppVersion == "" {
		t.Error("appVersion is empty")
	}
	if status.AppVersion != version.Version {
		t.Errorf("appVersion = %q, want version.Version %q", status.AppVersion, version.Version)
	}
}

func TestE2ESettings(t *testing.T) {
	env := newE2E(t)

	data := env.call("GET", "/api/settings", nil)
	if got := settingValue(t, data, "daily_quota_budget"); got == "" {
		t.Error("daily_quota_budget missing from settings")
	}

	// Set a key, then read it back.
	env.call("PUT", "/api/settings", map[string]string{"key": "daily_quota_budget", "value": "5000"})
	if got := settingValue(t, env.call("GET", "/api/settings", nil), "daily_quota_budget"); got != "5000" {
		t.Errorf("daily_quota_budget after set = %q, want %q", got, "5000")
	}

	// Export carries the new value; import round-trips it back.
	var exported struct {
		Settings map[string]string `json:"settings"`
		Excluded []string          `json:"excluded"`
	}
	decode(t, env.call("POST", "/api/settings/export", nil), &exported)
	if exported.Settings["daily_quota_budget"] != "5000" {
		t.Errorf("exported daily_quota_budget = %q, want %q", exported.Settings["daily_quota_budget"], "5000")
	}
	for _, key := range exported.Excluded {
		if _, ok := exported.Settings[key]; ok {
			t.Errorf("excluded key %q leaked into the export", key)
		}
	}
	exported.Settings["daily_quota_budget"] = "7000"
	env.call("POST", "/api/settings/import", map[string]any{"settings": exported.Settings})
	if got := settingValue(t, env.call("GET", "/api/settings", nil), "daily_quota_budget"); got != "7000" {
		t.Errorf("daily_quota_budget after import = %q, want %q", got, "7000")
	}

	// An unknown key is rejected, not silently stored.
	if status, env := env.callErr("PUT", "/api/settings", map[string]string{"key": "no_such_key", "value": "1"}); status != http.StatusBadRequest || env.OK {
		t.Errorf("unknown key: status %d ok %v, want 400 + ok false", status, env.OK)
	}
}

func TestE2EVideoSearch(t *testing.T) {
	env := newE2E(t)

	var list struct {
		Videos []struct {
			Title string `json:"title"`
		} `json:"videos"`
		Count int `json:"count"`
	}
	decode(t, env.call("GET", "/api/search?q=Harbour", nil), &list)
	if len(list.Videos) == 0 {
		t.Error("search for Harbour returned no videos")
	}
	if list.Count < len(list.Videos) {
		t.Errorf("search count %d < returned %d", list.Count, len(list.Videos))
	}
	for _, video := range list.Videos {
		if !strings.Contains(video.Title, "Harbour") {
			t.Errorf("search hit %q does not contain Harbour", video.Title)
		}
	}

	var grouped struct {
		Videos    []json.RawMessage `json:"videos"`
		Playlists []json.RawMessage `json:"playlists"`
		Episodes  []json.RawMessage `json:"episodes"`
		Comments  []json.RawMessage `json:"comments"`
	}
	decode(t, env.call("GET", "/api/search/all?q=Harbour", nil), &grouped)
	if len(grouped.Videos) == 0 {
		t.Error("search/all for Harbour returned no videos")
	}
}

func TestE2EInsights(t *testing.T) {
	env := newE2E(t)

	var dashboard struct {
		TotalVideos int   `json:"totalVideos"`
		TotalViews  int64 `json:"totalViews"`
	}
	decode(t, env.call("GET", "/api/insights/dashboard?channelId=1", nil), &dashboard)
	if dashboard.TotalVideos != 12 {
		t.Errorf("dashboard totalVideos = %d, want 12", dashboard.TotalVideos)
	}
	if dashboard.TotalViews <= 0 {
		t.Errorf("dashboard totalViews = %d, want > 0", dashboard.TotalViews)
	}

	var scorecards struct {
		Scorecards []struct {
			SeriesKey string            `json:"seriesKey"`
			Episodes  []json.RawMessage `json:"episodes"`
		} `json:"scorecards"`
	}
	decode(t, env.call("GET", "/api/insights/scorecards?channelId=1", nil), &scorecards)
	if len(scorecards.Scorecards) == 0 {
		t.Error("scorecards empty")
	}

	var performers struct {
		Top    []json.RawMessage `json:"top"`
		Bottom []json.RawMessage `json:"bottom"`
	}
	decode(t, env.call("GET", "/api/insights/performers?channelId=1", nil), &performers)
	if len(performers.Top) == 0 {
		t.Error("performers top empty")
	}

	var heatmap struct {
		Cells []json.RawMessage `json:"cells"`
	}
	decode(t, env.call("GET", "/api/insights/heatmap?channelId=1", nil), &heatmap)
	if len(heatmap.Cells) == 0 {
		t.Error("heatmap cells empty")
	}

	var growth struct {
		Snapshots []json.RawMessage `json:"snapshots"`
		Diff      string            `json:"diff"`
	}
	decode(t, env.call("GET", "/api/insights/growth?accountId=1", nil), &growth)
	if growth.Diff == "" {
		t.Error("growth diff empty (a sentence is expected even with no snapshots)")
	}

	var wrapup struct {
		Year            int    `json:"year"`
		VideosPublished int    `json:"videosPublished"`
		TotalViews      int64  `json:"totalViews"`
		ChannelTitle    string `json:"channelTitle"`
	}
	decode(t, env.call("GET", fmt.Sprintf("/api/insights/wrapup?channelId=1&year=%d", time.Now().Year()), nil), &wrapup)
	if wrapup.Year != time.Now().Year() {
		t.Errorf("wrapup year = %d, want %d", wrapup.Year, time.Now().Year())
	}
	if wrapup.VideosPublished <= 0 {
		t.Errorf("wrapup videosPublished = %d, want > 0", wrapup.VideosPublished)
	}

	var compare struct {
		A struct {
			TotalVideos int `json:"totalVideos"`
		} `json:"a"`
		B struct {
			TotalVideos int `json:"totalVideos"`
		} `json:"b"`
	}
	decode(t, env.call("GET", "/api/insights/compare?a=1&b=1", nil), &compare)
	if compare.A.TotalVideos != compare.B.TotalVideos || compare.A.TotalVideos != 12 {
		t.Errorf("compare a/b = %d/%d, want 12/12", compare.A.TotalVideos, compare.B.TotalVideos)
	}
}

func TestE2ECommentsAndPlaylists(t *testing.T) {
	env := newE2E(t)

	var coverage []struct {
		ChannelID uint    `json:"channelId"`
		Percent   float64 `json:"percent"`
	}
	decode(t, env.call("GET", "/api/comment-coverage", nil), &coverage)
	found := false
	for _, row := range coverage {
		if row.ChannelID == 1 {
			found = true
		}
	}
	if !found {
		t.Error("comment-coverage has no row for channel 1")
	}

	var preview []struct {
		VideoID uint   `json:"videoId"`
		Title   string `json:"title"`
		Text    string `json:"text"`
	}
	decode(t, env.call("POST", "/api/comments/bulk-preview", map[string]any{
		"channelId": 1, "template": "Watching {title}!",
	}), &preview)
	if len(preview) == 0 {
		t.Error("bulk comment preview empty")
	}
	for _, row := range preview {
		if !strings.Contains(row.Text, row.Title) {
			t.Errorf("preview text %q does not render {title} = %q", row.Text, row.Title)
		}
	}

	var gaps struct {
		MissingCount int `json:"missingCount"`
	}
	decode(t, env.call("GET", "/api/playlists/1/gaps?series=harbour%20walks", nil), &gaps)
	if gaps.MissingCount < 0 {
		t.Errorf("gaps missingCount = %d, want >= 0", gaps.MissingCount)
	}

	var duplicates []json.RawMessage
	decode(t, env.call("GET", "/api/playlists/duplicates", nil), &duplicates)

	var checklist []struct {
		Name string `json:"name"`
		OK   bool   `json:"ok"`
	}
	decode(t, env.call("GET", "/api/playlists/1/checklist", nil), &checklist)
	if len(checklist) == 0 {
		t.Error("playlist checklist empty")
	}
	for _, item := range checklist {
		if item.Name == "" {
			t.Error("checklist item without a name")
		}
	}

	var orphans []struct {
		SeriesKey string `json:"seriesKey"`
		Count     int    `json:"count"`
	}
	decode(t, env.call("GET", "/api/orphans?channelId=1", nil), &orphans)
	if len(orphans) == 0 {
		t.Error("orphans empty: demo videos are not all in the single demo playlist")
	}

	var tags []json.RawMessage
	decode(t, env.call("GET", "/api/tags?channelId=1", nil), &tags)
	// Demo videos carry no tags: the honest answer is an empty list.
	if tags == nil {
		t.Error("tags response is null, want an empty list")
	}

	// Goals CRUD.
	var created struct {
		ID       string  `json:"id"`
		Progress float64 `json:"progress"`
	}
	decode(t, env.call("POST", "/api/goals", map[string]any{
		"text": "Reach 12 videos", "type": "count", "channelId": 1, "target": 12,
	}), &created)
	if created.ID == "" {
		t.Fatal("goal create returned no id")
	}
	var goals []struct {
		ID string `json:"id"`
	}
	decode(t, env.call("GET", "/api/goals", nil), &goals)
	seen := false
	for _, goal := range goals {
		if goal.ID == created.ID {
			seen = true
		}
	}
	if !seen {
		t.Errorf("goal %s not in list after create", created.ID)
	}
	env.call("DELETE", "/api/goals/"+created.ID, nil)
	decode(t, env.call("GET", "/api/goals", nil), &goals)
	for _, goal := range goals {
		if goal.ID == created.ID {
			t.Errorf("goal %s still listed after delete", created.ID)
		}
	}
}

func TestE2EBackupsAndShare(t *testing.T) {
	env := newE2E(t)

	// Schedule defaults, then a real schedule.
	var schedule struct {
		Cadence string `json:"cadence"`
		Keep    int    `json:"keep"`
	}
	decode(t, env.call("GET", "/api/backups/schedule", nil), &schedule)
	if schedule.Cadence != "off" {
		t.Errorf("default backup cadence = %q, want off", schedule.Cadence)
	}
	decode(t, env.call("PUT", "/api/backups/schedule", map[string]any{"cadence": "daily", "keep": 3}), &schedule)
	if schedule.Cadence != "daily" || schedule.Keep != 3 {
		t.Errorf("schedule after PUT = %+v, want daily/3", schedule)
	}
	decode(t, env.call("GET", "/api/backups/schedule", nil), &schedule)
	if schedule.Cadence != "daily" || schedule.Keep != 3 {
		t.Errorf("schedule after GET = %+v, want daily/3", schedule)
	}

	// A database backup set: four files on disk.
	var created struct {
		Name      string `json:"name"`
		Files     int    `json:"files"`
		SizeBytes int64  `json:"sizeBytes"`
	}
	decode(t, env.call("POST", "/api/backups/db", nil), &created)
	if created.Files != 4 {
		t.Errorf("backup files = %d, want 4 (accounts, content, planning, system)", created.Files)
	}
	for _, name := range []string{"accounts.db", "content.db", "planning.db", "system.db"} {
		path := filepath.Join(env.home, "backups", created.Name, name)
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Errorf("backup file %s missing or empty: %v", path, err)
		}
	}
	var sets []struct {
		Name string `json:"name"`
	}
	decode(t, env.call("GET", "/api/backups/db", nil), &sets)
	listed := false
	for _, set := range sets {
		if set.Name == created.Name {
			listed = true
		}
	}
	if !listed {
		t.Errorf("backup set %q not listed", created.Name)
	}

	// Stage-restore: bogus name 404s, the real one stages files.
	if status, _ := env.callErr("POST", "/api/backups/db/stage-restore", map[string]string{"name": "2099-01-01-000000"}); status != http.StatusNotFound {
		t.Errorf("stage-restore bogus name: status %d, want 404", status)
	}
	var staged struct {
		Staged []string `json:"staged"`
		Note   string   `json:"note"`
	}
	decode(t, env.call("POST", "/api/backups/db/stage-restore", map[string]string{"name": created.Name}), &staged)
	if len(staged.Staged) != 4 {
		t.Errorf("staged files = %d, want 4", len(staged.Staged))
	}
	if staged.Note == "" {
		t.Error("stage-restore note empty: the stopped-server instruction must be present")
	}
	for _, name := range staged.Staged {
		if _, err := os.Stat(filepath.Join(env.home, "restore-staging", name)); err != nil {
			t.Errorf("staged file %s missing: %v", name, err)
		}
	}

	// Share link lifecycle.
	var share struct {
		Path      string `json:"path"`
		ExpiresAt string `json:"expiresAt"`
	}
	decode(t, env.call("POST", "/api/share", map[string]any{"hours": 2}), &share)
	if !strings.HasPrefix(share.Path, "/?share=") {
		t.Errorf("share path = %q, want /?share=TOKEN", share.Path)
	}
	token := strings.TrimPrefix(share.Path, "/?share=")
	var shareState struct {
		Active    bool   `json:"active"`
		ExpiresAt string `json:"expiresAt"`
	}
	decode(t, env.call("GET", "/api/share", nil), &shareState)
	if !shareState.Active {
		t.Error("share not active after create")
	}
	var status struct {
		ShareMode bool `json:"shareMode"`
		ReadOnly  bool `json:"readOnly"`
	}
	decode(t, env.call("GET", "/api/status?share="+token, nil), &status)
	if !status.ShareMode || !status.ReadOnly {
		t.Errorf("share request: shareMode %v readOnly %v, want true/true", status.ShareMode, status.ReadOnly)
	}
	if code, _ := env.callErr("PUT", "/api/settings", map[string]string{"key": "daily_quota_budget", "value": "1"}, "Cookie", "ytm_share="+token); code != http.StatusForbidden {
		t.Errorf("write with share cookie: status %d, want 403", code)
	}
	env.call("DELETE", "/api/share", nil)
	decode(t, env.call("GET", "/api/share", nil), &shareState)
	if shareState.Active {
		t.Error("share still active after revoke")
	}
}

func TestE2ENotifyAndPeers(t *testing.T) {
	env := newE2E(t)

	// Notification preferences: defaults, valid save, invalid save.
	var prefs struct {
		Digest     string `json:"digest"`
		Failures   string `json:"failures"`
		GoingLive  string `json:"goingLive"`
		Weekly     string `json:"weekly"`
		DigestTime string `json:"digestTime"`
	}
	decode(t, env.call("GET", "/api/notify/prefs", nil), &prefs)
	if prefs.Digest != "digest" || prefs.Failures != "instant" {
		t.Errorf("default prefs = %+v, want digest/instant", prefs)
	}
	decode(t, env.call("PUT", "/api/notify/prefs", map[string]any{
		"digest": "off", "failures": "instant", "goingLive": "off",
		"weekly": "digest", "digestTime": "07:30", "excludedChannelIds": []uint{},
	}), &prefs)
	if prefs.Digest != "off" || prefs.DigestTime != "07:30" {
		t.Errorf("prefs after PUT = %+v, want off/07:30", prefs)
	}
	decode(t, env.call("GET", "/api/notify/prefs", nil), &prefs)
	if prefs.Digest != "off" {
		t.Errorf("prefs after GET = %+v, want digest off", prefs)
	}
	if status, _ := env.callErr("PUT", "/api/notify/prefs", map[string]any{
		"digest": "bogus", "failures": "instant", "goingLive": "off",
		"weekly": "digest", "digestTime": "07:30",
	}); status != http.StatusBadRequest {
		t.Errorf("invalid prefs mode: status %d, want 400", status)
	}

	// Test-send without SMTP configured errors plainly, no crash.
	if status, env2 := env.callErr("POST", "/api/notify/test", map[string]string{"kind": "digest"}); status != http.StatusBadRequest || env2.OK || env2.Error == "" {
		t.Errorf("test-send without SMTP: status %d ok %v error %q, want 400 + a message", status, env2.OK, env2.Error)
	}

	// Peers CRUD.
	var peer struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	decode(t, env.call("POST", "/api/peers", map[string]string{
		"name": "laptop", "baseUrl": "http://localhost:9999", "secret": "test-secret",
	}), &peer)
	if peer.ID == 0 {
		t.Fatal("peer create returned no id")
	}
	var peers []struct {
		ID               uint   `json:"id"`
		Name             string `json:"name"`
		SharedChannelIDs []uint `json:"sharedChannelIds"`
	}
	decode(t, env.call("GET", "/api/peers", nil), &peers)
	peerFound := false
	for _, p := range peers {
		if p.ID == peer.ID {
			peerFound = true
		}
	}
	if !peerFound {
		t.Errorf("peer %d not listed after create", peer.ID)
	}
	env.call("PUT", fmt.Sprintf("/api/peers/%d/sharing", peer.ID), map[string]any{"channelIds": []uint{1}})
	decode(t, env.call("GET", "/api/peers", nil), &peers)
	shared := false
	for _, p := range peers {
		if p.ID == peer.ID {
			for _, id := range p.SharedChannelIDs {
				if id == 1 {
					shared = true
				}
			}
		}
	}
	if !shared {
		t.Error("peer sharing did not persist channel 1")
	}
	var peerVersion struct {
		LocalAppVersion    string `json:"localAppVersion"`
		LocalSchemaVersion int    `json:"localSchemaVersion"`
		Warning            string `json:"warning"`
	}
	decode(t, env.call("GET", fmt.Sprintf("/api/peers/%d/version", peer.ID), nil), &peerVersion)
	if peerVersion.LocalAppVersion != version.Version {
		t.Errorf("peer version local = %q, want %q", peerVersion.LocalAppVersion, version.Version)
	}
	if peerVersion.Warning != "" {
		t.Errorf("peer version warning = %q, want empty for a never-contacted peer", peerVersion.Warning)
	}
	env.call("DELETE", fmt.Sprintf("/api/peers/%d", peer.ID), nil)
	decode(t, env.call("GET", "/api/peers", nil), &peers)
	for _, p := range peers {
		if p.ID == peer.ID {
			t.Errorf("peer %d still listed after delete", peer.ID)
		}
	}

	// Export/import round-trip on identical data.
	status, bundleBody := env.raw("GET", "/api/export/bundle", nil)
	if status != http.StatusOK {
		t.Fatalf("export bundle: status %d", status)
	}
	var bundle struct {
		Kind   string `json:"kind"`
		Mirror struct {
			Videos []json.RawMessage `json:"videos"`
		} `json:"mirror"`
	}
	if err := json.Unmarshal(bundleBody, &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Kind != "export-bundle" {
		t.Errorf("bundle kind = %q, want export-bundle", bundle.Kind)
	}
	var preview struct {
		VideosAdd    int `json:"videosAdd"`
		VideosUpdate int `json:"videosUpdate"`
		VideosSkip   int `json:"videosSkip"`
	}
	decode(t, env.call("POST", "/api/import/preview", json.RawMessage(bundleBody)), &preview)
	if preview.VideosSkip != 12 {
		t.Errorf("import preview videosSkip = %d, want 12 for identical data", preview.VideosSkip)
	}
	var applied struct {
		VideosApplied int `json:"videosApplied"`
		VideosSkipped int `json:"videosSkipped"`
	}
	decode(t, env.call("POST", "/api/import/apply", json.RawMessage(bundleBody)), &applied)
	if applied.VideosApplied != 0 || applied.VideosSkipped != 12 {
		t.Errorf("import apply = %+v, want 0 applied / 12 skipped", applied)
	}
}

func TestE2EOverviewHealthTimeline(t *testing.T) {
	env := newE2E(t)

	var overview struct {
		Accounts []struct {
			Label     string   `json:"label"`
			Attention []string `json:"attention"`
		} `json:"accounts"`
	}
	decode(t, env.call("GET", "/api/overview", nil), &overview)
	if len(overview.Accounts) != 1 {
		t.Fatalf("overview accounts = %d, want 1", len(overview.Accounts))
	}
	noToken := false
	for _, note := range overview.Accounts[0].Attention {
		if strings.Contains(note, "No stored token") {
			noToken = true
		}
	}
	if !noToken {
		t.Errorf("overview attention %v lacks the no-token note for the demo account", overview.Accounts[0].Attention)
	}

	var health []struct {
		Scope string `json:"scope"`
		OK    bool   `json:"ok"`
	}
	decode(t, env.call("GET", "/api/health", nil), &health)
	if len(health) == 0 {
		t.Error("health rows empty")
	}
	anyBad := false
	for _, row := range health {
		if !row.OK {
			anyBad = true
		}
	}
	if !anyBad {
		t.Error("health has no failing row, yet the demo account has no token and SMTP is unset")
	}

	var timeline []struct {
		Status string `json:"status"`
	}
	decode(t, env.call("GET", "/api/timeline?channelId=1&days=365", nil), &timeline)
	published := 0
	for _, item := range timeline {
		if item.Status == "published" {
			published++
		}
	}
	if published == 0 {
		t.Error("timeline has no published items")
	}

	var missed []json.RawMessage
	decode(t, env.call("GET", "/api/missed", nil), &missed)
	if missed == nil {
		t.Error("missed is null, want an empty list")
	}

	// No IMAP configured: an error response, not a panic.
	if status, env2 := env.callErr("GET", "/api/inbox", nil); status != http.StatusBadRequest || env2.OK || env2.Error == "" {
		t.Errorf("inbox without IMAP: status %d ok %v error %q, want 400 + a message", status, env2.OK, env2.Error)
	}
}

func TestE2EBulkPlan(t *testing.T) {
	env := newE2E(t)

	var plan struct {
		PlanID  string `json:"planId"`
		Changes []struct {
			OldTitle string `json:"oldTitle"`
			NewTitle string `json:"newTitle"`
		} `json:"changes"`
	}
	decode(t, env.call("POST", "/api/videos/bulk-plan", map[string]any{
		"channelId": 1,
		"pattern":   map[string]any{"template": "{title} [NEW]"},
	}), &plan)
	if plan.PlanID == "" {
		t.Error("bulk plan returned no plan id")
	}
	if len(plan.Changes) == 0 {
		t.Error("bulk plan returned no changes for the demo videos")
	}
	for _, change := range plan.Changes {
		if !strings.HasSuffix(change.NewTitle, " [NEW]") {
			t.Errorf("bulk change new title %q does not end with the template suffix", change.NewTitle)
		}
	}
}

func TestE2EOfflineQueue(t *testing.T) {
	env := newE2E(t)

	var queue []json.RawMessage
	decode(t, env.call("GET", "/api/offline-queue", nil), &queue)
	if queue == nil {
		t.Error("offline queue is null, want an empty list")
	}
	if status, _ := env.callErr("POST", "/api/offline-queue/123456/retry", nil); status != http.StatusNotFound {
		t.Errorf("retry of missing queue entry: status %d, want 404", status)
	}
	if status, _ := env.callErr("DELETE", "/api/offline-queue/123456", nil); status != http.StatusNotFound {
		t.Errorf("delete of missing queue entry: status %d, want 404", status)
	}
}
