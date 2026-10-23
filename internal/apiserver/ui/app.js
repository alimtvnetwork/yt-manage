"use strict";

const state = {
  videos: [],
  selected: new Set(),
  bulkPlan: null,
  currentEpisodeId: null,
};

function el(id) { return document.getElementById(id); }

function setStatus(id, text, kind) {
  const node = el(id);
  node.textContent = text || "";
  node.className = "status-line" + (kind ? " " + kind : "");
}

async function api(method, path, body) {
  const options = { method, headers: {} };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  const response = await fetch(path, options);
  const payload = await response.json();
  if (!payload.ok) {
    throw new Error(payload.error || ("HTTP " + response.status));
  }
  return payload.data;
}

function escapeHTML(text) {
  return String(text == null ? "" : text)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function highlightTitle(title) {
  const pattern = el("pattern-finder").value;
  if (!pattern) return escapeHTML(title);
  let regex = null;
  try { regex = new RegExp(pattern, "gi"); } catch (err) { return escapeHTML(title); }
  const escaped = escapeHTML(title);
  let plainPattern = pattern;
  try { new RegExp(plainPattern); } catch (err) { return escaped; }
  // Highlight on the escaped text only when the pattern itself is plain
  // text-safe; regex matches are found on the raw title and rebuilt.
  const raw = String(title);
  let out = "";
  let last = 0;
  let match = regex.exec(raw);
  let guard = 0;
  while (match && guard < 200) {
    guard++;
    out += escapeHTML(raw.slice(last, match.index));
    out += "<mark>" + escapeHTML(match[0]) + "</mark>";
    last = match.index + match[0].length;
    if (match[0].length === 0) regex.lastIndex++;
    match = regex.exec(raw);
  }
  out += escapeHTML(raw.slice(last));
  return out;
}

// Tabs
el("tabs").addEventListener("click", (event) => {
  const button = event.target.closest(".tab");
  if (!button) return;
  document.querySelectorAll(".tab").forEach((t) => t.classList.remove("is-active"));
  document.querySelectorAll(".panel").forEach((p) => p.classList.remove("is-active"));
  button.classList.add("is-active");
  el("section-" + button.dataset.section).classList.add("is-active");
});

// Videos
async function loadVideos() {
  setStatus("video-status", "Loading...");
  const params = new URLSearchParams();
  const q = el("video-q").value.trim();
  const channel = el("video-channel").value;
  const visibility = el("video-visibility").value;
  if (q) params.set("q", q);
  if (channel) params.set("channel", channel);
  if (visibility) params.set("visibility", visibility);
  params.set("limit", "200");
  try {
    const data = await api("GET", "/api/videos?" + params.toString());
    state.videos = data.videos || [];
    state.selected.clear();
    renderVideos();
    setStatus("video-status", data.count + " video(s)", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
}

function renderVideos() {
  const tbody = el("video-rows");
  tbody.classList.remove("refreshing");
  void tbody.offsetWidth;
  tbody.innerHTML = state.videos.map((video) => {
    const checked = state.selected.has(video.id) ? " checked" : "";
    return "<tr data-id=\"" + video.id + "\">" +
      "<td><input type=\"checkbox\" class=\"video-check\" data-id=\"" + video.id + "\"" + checked + "></td>" +
      "<td>" + video.id + "</td>" +
      "<td>" + highlightTitle(video.title) + "</td>" +
      "<td>" + escapeHTML(video.visibility) + "</td>" +
      "<td>" + video.viewCount + "</td>" +
      "<td>" + escapeHTML(video.publishedAt || "") + "</td></tr>";
  }).join("");
  tbody.classList.add("refreshing");
  el("video-select-all").checked = state.videos.length > 0 && state.selected.size === state.videos.length;
}

el("video-rows").addEventListener("change", (event) => {
  const box = event.target.closest(".video-check");
  if (!box) return;
  const id = Number(box.dataset.id);
  if (box.checked) state.selected.add(id); else state.selected.delete(id);
  box.closest("tr").classList.toggle("selected", box.checked);
  el("video-select-all").checked = state.videos.length > 0 && state.selected.size === state.videos.length;
});

el("video-rows").addEventListener("click", (event) => {
  if (event.target.closest(".video-check")) return;
  const row = event.target.closest("tr[data-id]");
  if (!row) return;
  const video = state.videos.find((v) => v.id === Number(row.dataset.id));
  if (!video) return;
  el("video-edit-form").classList.remove("hidden");
  el("edit-video-id").textContent = "#" + video.id + " (" + video.ytVideoId + ")";
  el("edit-title").value = video.title;
  el("edit-description").value = video.description;
  el("video-edit-form").dataset.videoId = String(video.id);
  setStatus("edit-status", "");
});

el("video-select-all").addEventListener("change", (event) => {
  state.selected.clear();
  if (event.target.checked) state.videos.forEach((v) => state.selected.add(v.id));
  renderVideos();
});

el("pattern-finder").addEventListener("input", renderVideos);
el("video-search-btn").addEventListener("click", loadVideos);
el("video-q").addEventListener("keydown", (event) => { if (event.key === "Enter") loadVideos(); });

el("edit-cancel").addEventListener("click", () => el("video-edit-form").classList.add("hidden"));

el("video-edit-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const id = el("video-edit-form").dataset.videoId;
  setStatus("edit-status", "Saving...");
  try {
    const updated = await api("PATCH", "/api/videos/" + id, {
      title: el("edit-title").value,
      description: el("edit-description").value,
    });
    setStatus("edit-status", "Saved: " + updated.title, "ok");
    await loadVideos();
  } catch (err) {
    setStatus("edit-status", err.message, "error");
  }
});

// Bulk plan / apply
el("bulk-plan-btn").addEventListener("click", async () => {
  setStatus("video-status", "Building plan...");
  const body = {
    q: el("video-q").value.trim(),
    visibility: el("video-visibility").value,
    pattern: {
      template: el("bulk-template").value,
      regex: el("bulk-regex").value,
      replacement: el("bulk-replacement").value,
      descriptionTemplate: el("bulk-desc-template").value,
      startIndex: Number(el("bulk-start").value || 1),
    },
  };
  const channel = el("video-channel").value;
  if (channel) body.channelId = Number(channel);
  try {
    const data = await api("POST", "/api/videos/bulk-plan", body);
    state.bulkPlan = data;
    el("bulk-plan").classList.remove("hidden");
    el("bulk-summary").textContent = data.summary || "";
    el("bulk-changes").innerHTML = (data.changes || []).map((change) =>
      "<tr><td>" + escapeHTML(change.ytVideoId) + "</td><td>" + escapeHTML(change.oldTitle) +
      "</td><td>" + escapeHTML(change.newTitle) + "</td><td>" +
      (change.hasDescriptionChange ? "changes" : "") + "</td></tr>").join("");
    setStatus("video-status", (data.changes || []).length + " change(s) planned", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

el("bulk-apply-btn").addEventListener("click", async () => {
  if (!state.bulkPlan) return;
  setStatus("video-status", "Applying...");
  try {
    const data = await api("POST", "/api/videos/bulk-edit", {
      planId: state.bulkPlan.planId,
      confirmToken: state.bulkPlan.confirmToken,
    });
    setStatus("video-status", "Applied " + data.applied + " of " + data.total, "ok");
    state.bulkPlan = null;
    el("bulk-plan").classList.add("hidden");
    await loadVideos();
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

// Episodes
async function loadEpisodes() {
  const channel = el("episode-channel").value;
  if (!channel) { setStatus("episode-status", "Channel ID is required", "error"); return; }
  setStatus("episode-status", "Loading...");
  try {
    const episodes = await api("GET", "/api/episodes?channel=" + encodeURIComponent(channel));
    el("episode-list").innerHTML = episodes.map((episode) => {
      const parts = (episode.parts || []).map((part) =>
        "<li>Part " + part.partNumber + " (seq " + part.sequence + "): " +
        escapeHTML(part.title) + " <span class=\"pill\">" + escapeHTML(part.ytVideoId) + "</span></li>").join("");
      return "<div class=\"card episode\"><h3>" + escapeHTML(episode.title || episode.episodeKey) +
        " <span class=\"pill\">episode " + episode.episodeNumber + "</span></h3>" +
        "<ul class=\"parts\">" + parts + "</ul>" +
        "<button data-schedule=\"" + episode.id + "\">Schedule this series</button></div>";
    }).join("") || "<p>No episodes for this channel.</p>";
    setStatus("episode-status", episodes.length + " episode(s)", "ok");
  } catch (err) {
    setStatus("episode-status", err.message, "error");
  }
}

el("episode-load-btn").addEventListener("click", loadEpisodes);

el("episode-list").addEventListener("click", (event) => {
  const button = event.target.closest("[data-schedule]");
  if (!button) return;
  state.currentEpisodeId = button.dataset.schedule;
  el("schedule-form").classList.remove("hidden");
  el("schedule-episode-id").textContent = "#" + state.currentEpisodeId;
  setStatus("schedule-status", "");
  el("schedule-result").innerHTML = "";
});

el("schedule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const local = el("schedule-starts").value;
  if (!local) { setStatus("schedule-status", "Starts-at is required", "error"); return; }
  const startsAt = new Date(local).toISOString();
  setStatus("schedule-status", "Scheduling...");
  try {
    const data = await api("POST", "/api/episodes/" + state.currentEpisodeId + "/schedule", {
      cadenceDays: Number(el("schedule-cadence").value || 7),
      startsAt,
      includePublic: el("schedule-public").checked,
    });
    setStatus("schedule-status", "Applied " + data.applied + ", skipped public " + data.skippedPublic, "ok");
    el("schedule-result").innerHTML = (data.entries || []).map((entry) =>
      "<li>" + escapeHTML(entry.ytVideoId) + " → " + escapeHTML(entry.publishAt) + "</li>").join("");
  } catch (err) {
    setStatus("schedule-status", err.message, "error");
  }
});

// Playlists
async function loadPlaylists() {
  const channel = el("playlist-channel").value;
  if (!channel) { setStatus("playlist-status", "Channel ID is required", "error"); return; }
  setStatus("playlist-status", "Loading...");
  try {
    const playlists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(channel));
    el("playlist-list").innerHTML = playlists.map((playlist) =>
      "<div class=\"card playlist\"><h3>" + escapeHTML(playlist.title) +
      " <span class=\"pill\">" + playlist.itemCount + " items</span></h3>" +
      "<p>" + escapeHTML(playlist.description || "") + "</p>" +
      "<div class=\"inline-form\"><input data-pl-title=\"" + playlist.id + "\" value=\"" + escapeHTML(playlist.title) + "\">" +
      "<input data-pl-desc=\"" + playlist.id + "\" value=\"" + escapeHTML(playlist.description || "") + "\">" +
      "<button data-pl-save=\"" + playlist.id + "\">Save</button></div></div>").join("") ||
      "<p>No playlists for this channel.</p>";
    setStatus("playlist-status", playlists.length + " playlist(s)", "ok");
  } catch (err) {
    setStatus("playlist-status", err.message, "error");
  }
}

el("playlist-load-btn").addEventListener("click", loadPlaylists);

el("playlist-list").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-pl-save]");
  if (!button) return;
  const id = button.dataset.plSave;
  setStatus("playlist-status", "Saving...");
  try {
    await api("PATCH", "/api/playlists/" + id, {
      title: document.querySelector("[data-pl-title=\"" + id + "\"]").value,
      description: document.querySelector("[data-pl-desc=\"" + id + "\"]").value,
    });
    setStatus("playlist-status", "Saved", "ok");
    await loadPlaylists();
  } catch (err) {
    setStatus("playlist-status", err.message, "error");
  }
});

el("playlist-create-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  setStatus("playlist-create-status", "Creating...");
  try {
    const created = await api("POST", "/api/playlists", {
      channelId: Number(el("playlist-new-channel").value),
      title: el("playlist-new-title").value,
      description: el("playlist-new-description").value,
      privacy: el("playlist-new-privacy").value,
    });
    setStatus("playlist-create-status", "Created " + created.ytPlaylistId, "ok");
    el("playlist-channel").value = el("playlist-new-channel").value;
    await loadPlaylists();
  } catch (err) {
    setStatus("playlist-create-status", err.message, "error");
  }
});

// A/B tests
async function loadABTests() {
  setStatus("ab-status", "Loading...");
  const videoId = el("ab-video-id").value;
  const path = videoId ? "/api/ab-tests?videoId=" + encodeURIComponent(videoId) : "/api/ab-tests";
  try {
    const tests = await api("GET", path);
    el("ab-list").innerHTML = tests.map((test) => {
      const snapshots = (test.snapshots || []).map((snap) =>
        "<li>" + escapeHTML(snap.capturedAt) + ": views " + escapeHTML(snap.viewsAContext) +
        (snap.ctrNote ? " — " + escapeHTML(snap.ctrNote) : "") + "</li>").join("");
      return "<div class=\"card abtest\"><h3>Test #" + test.id + " for video " + test.videoId +
        " <span class=\"pill\">" + (test.isActive ? "active" : "done") + "</span></h3>" +
        "<p>A: " + escapeHTML(test.variantA) + "<br>B: " + escapeHTML(test.variantB) +
        (test.winner ? "<br>Winner: " + escapeHTML(test.winner) : "") + "</p>" +
        "<p>" + test.snapshotCount + " snapshot(s)</p><ul class=\"parts\">" + snapshots + "</ul>" +
        "<div class=\"inline-form\"><input data-ab-note=\"" + test.id + "\" placeholder=\"CTR note\">" +
        "<button data-ab-snap=\"" + test.id + "\">Snapshot</button>" +
        "<button data-ab-win=\"a\" data-ab-id=\"" + test.id + "\">Pick A</button>" +
        "<button data-ab-win=\"b\" data-ab-id=\"" + test.id + "\">Pick B</button></div></div>";
    }).join("") || "<p>No tests.</p>";
    setStatus("ab-status", tests.length + " test(s)", "ok");
  } catch (err) {
    setStatus("ab-status", err.message, "error");
  }
}

el("ab-load-btn").addEventListener("click", loadABTests);

el("ab-list").addEventListener("click", async (event) => {
  const snap = event.target.closest("[data-ab-snap]");
  const win = event.target.closest("[data-ab-win]");
  try {
    if (snap) {
      const note = document.querySelector("[data-ab-note=\"" + snap.dataset.abSnap + "\"]").value;
      await api("POST", "/api/ab-tests/" + snap.dataset.abSnap + "/snapshot", { ctrNote: note });
      setStatus("ab-status", "Snapshot recorded", "ok");
      await loadABTests();
    } else if (win) {
      await api("POST", "/api/ab-tests/" + win.dataset.abId + "/pick-winner", { winner: win.dataset.abWin });
      setStatus("ab-status", "Winner applied", "ok");
      await loadABTests();
    }
  } catch (err) {
    setStatus("ab-status", err.message, "error");
  }
});

el("ab-create-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  setStatus("ab-create-status", "Creating...");
  try {
    const created = await api("POST", "/api/ab-tests", {
      videoId: Number(el("ab-new-video").value),
      variantA: el("ab-new-a").value,
      variantB: el("ab-new-b").value,
    });
    setStatus("ab-create-status", "Created test #" + created.id, "ok");
    el("ab-video-id").value = el("ab-new-video").value;
    await loadABTests();
  } catch (err) {
    setStatus("ab-create-status", err.message, "error");
  }
});

// Accounts and sync
async function loadAccounts() {
  setStatus("account-status", "Loading...");
  try {
    const accounts = await api("GET", "/api/accounts");
    const rows = [];
    accounts.forEach((account) => {
      if (!account.channels || account.channels.length === 0) {
        rows.push("<tr><td>" + account.id + "</td><td>" + escapeHTML(account.label) +
          "</td><td colspan=\"4\">No channels</td></tr>");
        return;
      }
      account.channels.forEach((channel) => {
        rows.push("<tr><td>" + account.id + "</td><td>" + escapeHTML(account.label) + "</td><td>" +
          escapeHTML(channel.title) + "</td><td>" + escapeHTML(channel.ytChannelId) + "</td><td>" +
          channel.subscriberCount + "</td><td>" + channel.videoCount + "</td></tr>");
      });
    });
    el("account-rows").innerHTML = rows.join("");
    setStatus("account-status", accounts.length + " account(s)", "ok");
  } catch (err) {
    setStatus("account-status", err.message, "error");
  }
}

el("accounts-load-btn").addEventListener("click", loadAccounts);

el("sync-all-btn").addEventListener("click", async () => {
  const button = el("sync-all-btn");
  button.disabled = true;
  el("sync-spinner").classList.remove("hidden");
  setStatus("account-status", "Syncing...");
  try {
    const data = await api("POST", "/api/sync", {});
    el("sync-result").classList.remove("hidden");
    el("sync-result").textContent = JSON.stringify(data, null, 2);
    const hasErrors = data.errors && data.errors.length > 0;
    setStatus("account-status", hasErrors ? "Sync finished with errors" : "Sync complete", hasErrors ? "error" : "ok");
  } catch (err) {
    setStatus("account-status", err.message, "error");
  } finally {
    button.disabled = false;
    el("sync-spinner").classList.add("hidden");
  }
});

// Initial load
loadVideos();
loadAccounts();
