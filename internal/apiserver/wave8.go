package apiserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alimtvnetwork/yt-manage/internal/batch"
	"github.com/alimtvnetwork/yt-manage/internal/episodes"
	"github.com/alimtvnetwork/yt-manage/internal/peer"
	"github.com/alimtvnetwork/yt-manage/internal/version"
)

// Wave 8 (spec 29 items 176-180, 184, 196, 200 backend): the
// scheduled database backups, the read-only share link, the
// offline apply queue, peer version-skew notes, the What's New
// feed, the channel time zone (in channel defaults, wave5.go),
// and the annual wrap-up. The help/delight items (181-199)
// are UI-side and consume these plus the earlier waves' APIs.

// ----------------------------------------------------------------
// Database backups on a schedule (176)

// backupPrefs is the DB backup schedule of item 176, stored in
// the settings KV as backup.prefs.
type backupPrefs struct {
	Cadence string `json:"cadence"` // off | daily | weekly
	Keep    int    `json:"keep"`    // backup sets to retain
}

func (s *Server) loadBackupPrefs() backupPrefs {
	prefs := backupPrefs{Cadence: "off", Keep: 7}
	s.getJSONSetting("backup.prefs", &prefs)
	if prefs.Keep <= 0 {
		prefs.Keep = 7
	}
	return prefs
}

// handleGetBackupSchedule serves GET /api/backups/schedule.
func (s *Server) handleGetBackupSchedule(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.loadBackupPrefs())
}

// handlePutBackupSchedule serves PUT /api/backups/schedule.
func (s *Server) handlePutBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var prefs backupPrefs
	if err := decodeJSON(r, &prefs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if prefs.Cadence != "off" && prefs.Cadence != "daily" && prefs.Cadence != "weekly" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cadence must be off, daily or weekly"))
		return
	}
	if prefs.Keep < 1 || prefs.Keep > 60 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("keep must be between 1 and 60"))
		return
	}
	if err := s.setJSONSetting("backup.prefs", prefs); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("setting", fmt.Sprintf("Database backup schedule: %s, keep %d", prefs.Cadence, prefs.Keep), "")
	writeOK(w, prefs)
}

// backupsDir is <home>/backups, one subdirectory per backup set.
func (s *Server) backupsDir() string {
	return filepath.Join(s.home, "backups")
}

// dbBackupSet is one backup set of GET /api/backups/db.
type dbBackupSet struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Files     int    `json:"files"`
	SizeBytes int64  `json:"sizeBytes"`
}

// listDBBackups enumerates the backup sets on disk, newest first.
func (s *Server) listDBBackups() ([]dbBackupSet, error) {
	entries, err := os.ReadDir(s.backupsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []dbBackupSet{}, nil
		}
		return nil, err
	}
	sets := []dbBackupSet{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		set := dbBackupSet{Name: entry.Name()}
		if info, statErr := entry.Info(); statErr == nil {
			set.CreatedAt = info.ModTime().Format(time.RFC3339)
		}
		files, readErr := os.ReadDir(filepath.Join(s.backupsDir(), entry.Name()))
		if readErr != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			set.Files++
			if info, statErr := file.Info(); statErr == nil {
				set.SizeBytes += info.Size()
			}
		}
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Name > sets[j].Name })
	return sets, nil
}

// createDBBackup snapshots the four databases into a new set
// and prunes old sets to the configured retention.
func (s *Server) createDBBackup() (*dbBackupSet, error) {
	name := time.Now().Format("2006-01-02-150405")
	dir := filepath.Join(s.backupsDir(), name)
	files, err := s.store.VacuumBackups(dir)
	if err != nil {
		return nil, err
	}
	set := &dbBackupSet{Name: name, CreatedAt: time.Now().Format(time.RFC3339), Files: len(files)}
	for _, file := range files {
		set.SizeBytes += file.SizeBytes
	}
	_ = s.store.Settings.Set("backup:lastDbAt", time.Now().Format(time.RFC3339))
	s.pruneDBBackups()
	return set, nil
}

// pruneDBBackups deletes backup sets beyond the retention count.
func (s *Server) pruneDBBackups() {
	prefs := s.loadBackupPrefs()
	sets, err := s.listDBBackups()
	if err != nil || len(sets) <= prefs.Keep {
		return
	}
	for _, set := range sets[prefs.Keep:] {
		_ = os.RemoveAll(filepath.Join(s.backupsDir(), set.Name))
	}
}

// handleCreateDBBackup serves POST /api/backups/db: back up now.
func (s *Server) handleCreateDBBackup(w http.ResponseWriter, r *http.Request) {
	set, err := s.createDBBackup()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("backup", fmt.Sprintf("Database backup %s created (%d files, %d bytes)", set.Name, set.Files, set.SizeBytes), "")
	writeOK(w, set)
}

// handleListDBBackups serves GET /api/backups/db.
func (s *Server) handleListDBBackups(w http.ResponseWriter, r *http.Request) {
	sets, err := s.listDBBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeOK(w, sets)
}

// handleStageDBRestore serves POST /api/backups/db/stage-restore
// {name}: the set's files are copied to <home>/restore-staging/.
// Swapping live database files under a running server would
// corrupt them, so the restore completes with the server
// stopped; the response says exactly that.
func (s *Server) handleStageDBRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" || strings.ContainsAny(body.Name, "/\\") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("a backup set name is required"))
		return
	}
	source := filepath.Join(s.backupsDir(), body.Name)
	files, err := os.ReadDir(source)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("backup set %q not found", body.Name))
		return
	}
	staging := filepath.Join(s.home, "restore-staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	copied := []string{}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".db") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(source, file.Name()))
		if readErr != nil {
			writeError(w, http.StatusInternalServerError, readErr)
			return
		}
		if writeErr := os.WriteFile(filepath.Join(staging, file.Name()), data, 0o644); writeErr != nil {
			writeError(w, http.StatusInternalServerError, writeErr)
			return
		}
		copied = append(copied, file.Name())
	}
	s.recordActivity("backup", fmt.Sprintf("Restore of backup %s staged (%d files)", body.Name, len(copied)), "")
	writeOK(w, map[string]interface{}{
		"staged": copied,
		"note":   "Staged in the restore-staging folder of the yt-manage home. To finish: stop yt-manage, copy these files over the ones in the data folder (removing any -wal/-shm files alongside them), then start yt-manage again. Nothing was changed yet.",
	})
}

// maybeRunScheduledBackup runs from the serve tick: when the
// cadence is on and the last backup is older than the cadence
// interval, take one.
func (s *Server) maybeRunScheduledBackup() {
	prefs := s.loadBackupPrefs()
	if prefs.Cadence == "off" {
		return
	}
	interval := 24 * time.Hour
	if prefs.Cadence == "weekly" {
		interval = 7 * 24 * time.Hour
	}
	if raw, found, _ := s.store.Settings.Get("backup:lastDbAt"); found {
		if last, err := time.Parse(time.RFC3339, raw); err == nil && time.Since(last) < interval {
			return
		}
	}
	if set, err := s.createDBBackup(); err == nil {
		s.recordActivity("backup", fmt.Sprintf("Scheduled database backup %s created", set.Name), "")
	}
}

// ----------------------------------------------------------------
// Read-only share link (177)

// shareLink is the stored state of the share link, in the
// settings KV as share:link.
type shareLink struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

func (s *Server) loadShareLink() (shareLink, bool) {
	link := shareLink{}
	s.getJSONSetting("share:link", &link)
	if link.Token == "" {
		return link, false
	}
	expires, err := time.Parse(time.RFC3339, link.ExpiresAt)
	if err != nil || time.Now().After(expires) {
		return link, false
	}
	return link, true
}

// shareStateFor reports whether this request arrives through a
// valid share link (query parameter or the cookie it sets),
// and the link's expiry for the banner.
func (s *Server) shareStateFor(r *http.Request) (bool, string) {
	link, valid := s.loadShareLink()
	if !valid {
		return false, ""
	}
	token := r.URL.Query().Get("share")
	if token == "" {
		if cookie, err := r.Cookie("ytm_share"); err == nil {
			token = cookie.Value
		}
	}
	if token != "" && token == link.Token {
		return true, link.ExpiresAt
	}
	return false, ""
}

// handleCreateShare serves POST /api/share {hours}: create (or
// replace) the read-only share link (item 177). The link pins
// every request that carries it to read-only mode until it
// expires or is revoked.
func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hours int `json:"hours"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Hours < 1 || body.Hours > 24*7 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("hours must be between 1 and 168"))
		return
	}
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	link := shareLink{
		Token:     hex.EncodeToString(raw),
		ExpiresAt: time.Now().Add(time.Duration(body.Hours) * time.Hour).Format(time.RFC3339),
	}
	if err := s.setJSONSetting("share:link", link); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordActivity("share", fmt.Sprintf("Read-only share link created, expires %s", link.ExpiresAt), "")
	writeOK(w, map[string]interface{}{
		"path": "/?share=" + link.Token, "expiresAt": link.ExpiresAt,
		"note": "Anyone on the local network with this link sees the app read-only until it expires. The address to share is this server's LAN address plus the path.",
	})
}

// handleGetShare serves GET /api/share: whether a link is live.
func (s *Server) handleGetShare(w http.ResponseWriter, r *http.Request) {
	link, valid := s.loadShareLink()
	writeOK(w, map[string]interface{}{"active": valid, "expiresAt": link.ExpiresAt})
}

// handleDeleteShare serves DELETE /api/share: revoke the link.
func (s *Server) handleDeleteShare(w http.ResponseWriter, r *http.Request) {
	_ = s.store.Settings.Set("share:link", "{}")
	s.recordActivity("share", "Read-only share link revoked", "")
	writeOK(w, map[string]bool{"revoked": true})
}

// ----------------------------------------------------------------
// Offline apply queue (179)

// queuedApply is one queued change-set of the offline queue.
type queuedApply struct {
	ID        uint           `json:"id"`
	Kind      string         `json:"kind"`
	CreatedAt string         `json:"createdAt"`
	Error     string         `json:"error"`
	Changes   []batch.Change `json:"changes"`
}

func (s *Server) loadOfflineQueue() []queuedApply {
	queue := []queuedApply{}
	s.getJSONSetting("offline:queue", &queue)
	return queue
}

func (s *Server) saveOfflineQueue(queue []queuedApply) {
	_ = s.setJSONSetting("offline:queue", queue)
}

// isNetworkError classifies an apply failure as connectivity:
// the machine could not reach YouTube, so the change-set is
// worth queueing for a later retry (item 179).
func isNetworkError(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range []string{"dial", "connection refused", "no such host", "network is unreachable", "i/o timeout", "context deadline exceeded", "tls handshake", "connection reset", "temporary failure"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// queueOfflineApply appends one failed change-set to the queue.
func (s *Server) queueOfflineApply(kind string, changes []batch.Change, errText string) {
	if len(changes) == 0 {
		return
	}
	queue := s.loadOfflineQueue()
	queue = append(queue, queuedApply{
		ID: uint(time.Now().UnixNano() / 1e6), Kind: kind,
		CreatedAt: time.Now().Format(time.RFC3339), Error: errText, Changes: changes,
	})
	if len(queue) > 20 {
		queue = queue[len(queue)-20:]
	}
	s.saveOfflineQueue(queue)
	s.recordActivity("queue", fmt.Sprintf("Queued %d offline change(s) for retry (%s)", len(changes), kind), errText)
}

// handleListOfflineQueue serves GET /api/offline-queue.
func (s *Server) handleListOfflineQueue(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.loadOfflineQueue())
}

// handleRetryOfflineQueue serves POST /api/offline-queue/{id}/retry:
// re-run the queued change-set now. The caller re-previews the
// stored changes from GET /api/offline-queue before retrying,
// so nothing applies unseen. Changes that succeed leave the
// queue; changes that fail again stay, with the fresh error.
func (s *Server) handleRetryOfflineQueue(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	queue := s.loadOfflineQueue()
	index := -1
	for i, entry := range queue {
		if entry.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("queued apply %d not found", id))
		return
	}
	entry := queue[index]
	results, applyErr := s.applyChangesGrouped(r, entry.Changes)
	if applyErr != nil {
		writeError(w, http.StatusBadRequest, applyErr)
		return
	}
	failedIDs := map[uint]string{}
	applied := 0
	for _, result := range results {
		if result.IsOK {
			applied++
		} else {
			failedIDs[result.VideoID] = result.Error
		}
	}
	remaining := []batch.Change{}
	for _, change := range entry.Changes {
		if _, failed := failedIDs[change.VideoID]; failed {
			remaining = append(remaining, change)
		}
	}
	if len(remaining) == 0 {
		s.saveOfflineQueue(append(queue[:index], queue[index+1:]...))
	} else {
		entry.Changes = remaining
		entry.Error = fmt.Sprintf("retry: %d of %d still failing", len(remaining), len(entry.Changes))
		queue[index] = entry
		s.saveOfflineQueue(queue)
	}
	s.recordActivity("queue", fmt.Sprintf("Offline queue retry %d: %d applied, %d remaining", id, applied, len(remaining)), "")
	writeOK(w, map[string]interface{}{"applied": applied, "remaining": len(remaining), "results": results})
}

// handleDeleteOfflineQueue serves DELETE /api/offline-queue/{id}.
func (s *Server) handleDeleteOfflineQueue(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	queue := s.loadOfflineQueue()
	next := queue[:0]
	found := false
	for _, entry := range queue {
		if entry.ID == id {
			found = true
			continue
		}
		next = append(next, entry)
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("queued apply %d not found", id))
		return
	}
	s.saveOfflineQueue(next)
	writeOK(w, map[string]bool{"deleted": true})
}

// ----------------------------------------------------------------
// Peer version skew (180)

// peerRemoteVersion is the last version a peer reported, in
// the settings KV as peer:<id>:remote.
type peerRemoteVersion struct {
	AppVersion    string `json:"appVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	SeenAt        string `json:"seenAt"`
}

// notePeerVersion records what a peer's payload says it runs.
func (s *Server) notePeerVersion(peerID uint, payload *peer.Payload) {
	if payload == nil {
		return
	}
	_ = s.setJSONSetting(fmt.Sprintf("peer:%d:remote", peerID), peerRemoteVersion{
		AppVersion: payload.AppVersion, SchemaVersion: payload.SchemaVersion,
		SeenAt: time.Now().Format(time.RFC3339),
	})
}

// peerVersionWarning compares a peer's recorded version with
// this build and returns "" when they agree (item 180).
func (s *Server) peerVersionWarning(peerID uint) string {
	remote := peerRemoteVersion{}
	s.getJSONSetting(fmt.Sprintf("peer:%d:remote", peerID), &remote)
	return versionWarningText(remote)
}

// versionWarningText renders the skew warning for one recorded
// remote version ("" when there is nothing to warn about).
func versionWarningText(remote peerRemoteVersion) string {
	if remote.AppVersion == "" && remote.SchemaVersion == 0 {
		return "" // never contacted: nothing recorded yet
	}
	if remote.SchemaVersion != 0 && remote.SchemaVersion != peer.PayloadSchemaVersion {
		return fmt.Sprintf("Version skew: the peer speaks payload schema %d, this build uses %d. Update one side before syncing.", remote.SchemaVersion, peer.PayloadSchemaVersion)
	}
	if remote.AppVersion != "" && remote.AppVersion != version.Version {
		return fmt.Sprintf("Version skew: the peer runs yt-manage %s, this machine runs %s. Sync should still work; update when convenient.", remote.AppVersion, version.Version)
	}
	return ""
}

// handlePeerVersion serves GET /api/peers/{id}/version.
func (s *Server) handlePeerVersion(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	remote := peerRemoteVersion{}
	s.getJSONSetting(fmt.Sprintf("peer:%d:remote", id), &remote)
	writeOK(w, map[string]interface{}{
		"localAppVersion": version.Version, "localSchemaVersion": peer.PayloadSchemaVersion,
		"remote": remote, "warning": versionWarningText(remote),
	})
}

// ----------------------------------------------------------------
// What's New (184) and the annual wrap-up (200)

// whatsNewEntry is one release-notes entry of GET /api/whats-new.
type whatsNewEntry struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Points  []string `json:"points"`
	Section string   `json:"section"`
}

// handleWhatsNew serves GET /api/whats-new (item 184): plain-
// words changelog entries for the spec 29 waves, each with the
// screen to try. The UI shows the ones the user has not seen.
func (s *Server) handleWhatsNew(w http.ResponseWriter, r *http.Request) {
	writeOK(w, []whatsNewEntry{
		{ID: "wave8", Title: "Backups, sharing and focus", Section: "settings", Points: []string{
			"Your four databases can now back themselves up on a daily or weekly schedule.",
			"Share a read-only link with someone on your network; it expires on its own.",
			"Focus mode distils a publish run to today's publishes, pins and first comments.",
			"End the year with a per-channel wrap-up, computed entirely from your mirror.",
		}},
		{ID: "wave7", Title: "Many accounts, one place", Section: "accounts", Points: []string{
			"An all-accounts overview with a health centre that names each fix.",
			"Publish digests, failure alerts and a Monday report by email.",
			"Peers can now share only the channels you choose, with a conflict view when both sides edited the same video.",
		}},
		{ID: "wave6", Title: "Playlists and insight", Section: "insights", Points: []string{
			"A playlist board with gap and duplicate finders, plus series scorecards and a publish-time heatmap.",
			"Tags are mirrored now, with rename and merge across a whole channel.",
		}},
		{ID: "wave5", Title: "Suggestions and automation", Section: "suggestions", Points: []string{
			"A suggestion inbox that learns your cadence, plus rules that rename, file and comment for you - with an approval queue when you want the final say.",
		}},
	})
}

// wrapup is the annual summary of GET /api/insights/wrapup
// (item 200), computed from the local mirror only.
type wrapup struct {
	Year            int            `json:"year"`
	ChannelID       uint           `json:"channelId"`
	ChannelTitle    string         `json:"channelTitle"`
	VideosPublished int            `json:"videosPublished"`
	TotalViews      int64          `json:"totalViews"`
	TopTitle        string         `json:"topTitle"`
	TopViews        int64          `json:"topViews"`
	SeriesCompleted []string       `json:"seriesCompleted"`
	SeriesActive    []string       `json:"seriesActive"`
	UploadsByMonth  map[string]int `json:"uploadsByMonth"`
	CoveragePct     float64        `json:"commentCoveragePct"`
	ScheduledAhead  int            `json:"scheduledAhead"`
	GeneratedAt     string         `json:"generatedAt"`
	Note            string         `json:"note"`
}

// handleWrapup serves GET /api/insights/wrapup?channelId=&year=.
func (s *Server) handleWrapup(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseQueryInt(r, "channelId")
	year, yearErr := parseQueryInt(r, "year")
	if err != nil || channelID <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}
	if yearErr != nil || year <= 0 {
		year = time.Now().Year()
	}
	channel, err := s.findChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	videos, err := s.store.Videos.ListByChannel(uint(channelID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result := &wrapup{
		Year: year, ChannelID: uint(channelID), ChannelTitle: channel.Title,
		SeriesCompleted: []string{}, SeriesActive: []string{},
		UploadsByMonth: map[string]int{}, GeneratedAt: time.Now().Format(time.RFC3339),
		Note: "Computed from the local mirror only (synced data, no live API call). A series counts as completed when its latest episode was published in the year and no later episode is waiting unpublished.",
	}
	type showState struct {
		maxPublishedEpisode int
		latestPublished     time.Time
		hasUnpublishedLater bool
		episodes            int
	}
	shows := map[string]*showState{}
	now := time.Now()
	for _, video := range videos {
		if video.PublishedAt != nil && video.PublishedAt.Year() == year {
			result.VideosPublished++
			result.TotalViews += video.ViewCount
			result.UploadsByMonth[video.PublishedAt.Format("2006-01")]++
			if video.ViewCount > result.TopViews {
				result.TopViews, result.TopTitle = video.ViewCount, video.Title
			}
		}
		if video.PublishAt != nil && video.PublishAt.After(now) {
			result.ScheduledAhead++
		}
		parsed := episodes.ParseTitle(video.Title)
		if !parsed.HasEpisode {
			continue
		}
		key := episodes.ShowKey(parsed.SeriesKey)
		state := shows[key]
		if state == nil {
			state = &showState{}
			shows[key] = state
		}
		state.episodes++
		if video.PublishedAt != nil {
			if parsed.EpisodeNumber >= state.maxPublishedEpisode {
				state.maxPublishedEpisode = parsed.EpisodeNumber
				state.latestPublished = *video.PublishedAt
			}
		}
	}
	for _, video := range videos {
		parsed := episodes.ParseTitle(video.Title)
		if !parsed.HasEpisode || video.PublishedAt != nil {
			continue
		}
		key := episodes.ShowKey(parsed.SeriesKey)
		if state := shows[key]; state != nil && parsed.EpisodeNumber > state.maxPublishedEpisode {
			state.hasUnpublishedLater = true
		}
	}
	for key, state := range shows {
		if state.episodes < 2 {
			continue
		}
		if !state.hasUnpublishedLater && state.latestPublished.Year() == year {
			result.SeriesCompleted = append(result.SeriesCompleted, key)
		} else {
			result.SeriesActive = append(result.SeriesActive, key)
		}
	}
	sort.Strings(result.SeriesCompleted)
	sort.Strings(result.SeriesActive)
	coverage, _ := s.commentCoverage(uint(channelID))
	result.CoveragePct = coverage
	writeOK(w, result)
}
