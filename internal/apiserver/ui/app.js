"use strict";

// ---------------------------------------------------------------- state
const state = {
  videos: [],
  selected: new Set(),
  bulkPlan: null,
  currentEpisodeId: null,
  status: null,
  savedSearches: [],
  section: "home",
  videosLoaded: false,
  syncing: false,
};

const SECTIONS = {
  home: "Home",
  videos: "Videos",
  episodes: "Episodes",
  schedules: "Schedules",
  playlists: "Playlists",
  comments: "Comments",
  abtests: "A/B Tests",
  accounts: "Accounts",
  settings: "Settings",
};

const LS = {
  get(key, fallback) {
    try {
      const raw = localStorage.getItem("ytm." + key);
      return raw === null ? fallback : JSON.parse(raw);
    } catch (err) { return fallback; }
  },
  set(key, value) {
    try { localStorage.setItem("ytm." + key, JSON.stringify(value)); } catch (err) { /* storage unavailable */ }
  },
};

function el(id) { return document.getElementById(id); }

function setStatus(id, text, kind) {
  const node = el(id);
  if (!node) return;
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

// ------------------------------------------------------- text highlight
// Plain-text, case-insensitive, escape-safe highlighting (item 22).
function highlightPlain(text, query) {
  const raw = String(text == null ? "" : text);
  const needle = String(query || "").trim().toLowerCase();
  if (!needle) return escapeHTML(raw);
  const lower = raw.toLowerCase();
  let out = "";
  let last = 0;
  let idx = lower.indexOf(needle, last);
  let guard = 0;
  while (idx !== -1 && guard < 200) {
    guard++;
    out += escapeHTML(raw.slice(last, idx));
    out += "<mark>" + escapeHTML(raw.slice(idx, idx + needle.length)) + "</mark>";
    last = idx + needle.length;
    idx = lower.indexOf(needle, last);
  }
  out += escapeHTML(raw.slice(last));
  return out;
}

function descriptionSnippet(description, query) {
  const raw = String(description || "");
  const needle = String(query || "").trim().toLowerCase();
  if (!needle || !raw) return "";
  const idx = raw.toLowerCase().indexOf(needle);
  if (idx === -1) return "";
  const start = Math.max(0, idx - 60);
  const end = Math.min(raw.length, idx + needle.length + 60);
  const slice = raw.slice(start, end).replace(/\s+/g, " ");
  return (start > 0 ? "…" : "") + highlightPlain(slice, query) + (end < raw.length ? "…" : "");
}

// Pattern-finder regex highlighting (pre-existing behaviour, kept).
function highlightTitle(title) {
  const q = currentVideoState().q;
  if (q) return highlightPlain(title, q);
  const pattern = el("pattern-finder").value;
  if (!pattern) return escapeHTML(title);
  let regex = null;
  try { regex = new RegExp(pattern, "gi"); } catch (err) { return escapeHTML(title); }
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

// ------------------------------------------------------------ preferences
function applyDensity(density) {
  const value = density === "dense" ? "dense" : "comfortable";
  document.body.classList.toggle("density-dense", value === "dense");
  el("density-toggle").textContent = "Density: " + value;
  el("set-density").value = value;
  LS.set("density", value);
}

function applySidebar(collapsed) {
  document.body.classList.toggle("sidebar-collapsed", !!collapsed);
  el("set-sidebar-collapsed").checked = !!collapsed;
  LS.set("sidebarCollapsed", !!collapsed);
}

el("density-toggle").addEventListener("click", () => {
  applyDensity(document.body.classList.contains("density-dense") ? "comfortable" : "dense");
});
el("set-density").addEventListener("change", () => applyDensity(el("set-density").value));
el("set-sidebar-collapsed").addEventListener("change", () => applySidebar(el("set-sidebar-collapsed").checked));
el("sidebar-collapse").addEventListener("click", () => {
  applySidebar(!document.body.classList.contains("sidebar-collapsed"));
});

// --------------------------------------------------------------- routing
// Hash deep links: #/<section>?<videos state> (items 14, 20).
function currentVideoState() {
  return {
    q: el("video-q").value.trim(),
    channel: el("video-channel").value,
    visibility: state.visibility || "",
    sort: el("video-sort").value,
    shorts: !!state.shorts,
    hasComments: !!state.hasComments,
  };
}

function videosHash() {
  const vs = currentVideoState();
  const params = new URLSearchParams();
  if (vs.q) params.set("q", vs.q);
  if (vs.channel) params.set("channel", vs.channel);
  if (vs.visibility) params.set("visibility", vs.visibility);
  if (vs.sort && vs.sort !== "newest") params.set("sort", vs.sort);
  if (vs.shorts) params.set("shorts", "1");
  if (vs.hasComments) params.set("hasComments", "1");
  const qs = params.toString();
  return "#/videos" + (qs ? "?" + qs : "");
}

function updateHash() {
  const hash = state.section === "videos" ? videosHash() : "#/" + state.section;
  history.replaceState(null, "", hash);
}

function parseHash() {
  const raw = location.hash.replace(/^#\/?/, "");
  if (!raw) return null;
  const parts = raw.split("?");
  const section = parts[0];
  if (!SECTIONS[section]) return null;
  const params = new URLSearchParams(parts.slice(1).join("?"));
  return { section, params };
}

function applyVideoParams(params) {
  el("video-q").value = params.get("q") || "";
  el("video-channel").value = params.get("channel") || el("video-channel").value;
  el("video-sort").value = params.get("sort") || LS.get("videoSort", "newest");
  setVisibility(params.get("visibility") || "", false);
  setToggleChips(params.get("shorts") === "1", params.get("hasComments") === "1", false);
}

function saveScroll() {
  LS.set("scroll." + state.section, window.scrollY);
}

let scrollTimer = null;
window.addEventListener("scroll", () => {
  if (scrollTimer) return;
  scrollTimer = setTimeout(() => { scrollTimer = null; saveScroll(); }, 200);
}, { passive: true });

function navigate(section, skipHash) {
  if (!SECTIONS[section]) section = "home";
  saveScroll();
  state.section = section;
  document.querySelectorAll(".nav-item").forEach((item) =>
    item.classList.toggle("is-active", item.dataset.section === section));
  document.querySelectorAll(".panel").forEach((panel) =>
    panel.classList.toggle("is-active", panel.id === "section-" + section));
  el("page-title").textContent = SECTIONS[section];
  renderScopeLabel();
  LS.set("lastSection", section);
  if (!skipHash) updateHash();
  onEnterSection(section);
  const y = LS.get("scroll." + section, 0);
  requestAnimationFrame(() => window.scrollTo(0, y));
}

el("side-nav").addEventListener("click", (event) => {
  const button = event.target.closest(".nav-item");
  if (button) navigate(button.dataset.section);
});

function onEnterSection(section) {
  if (section === "videos" && !state.videosLoaded) loadVideos();
  if (section === "episodes" && el("episode-channel").value) loadEpisodes();
  if (section === "playlists" && el("playlist-channel").value) loadPlaylists();
  if (section === "schedules") loadSchedules();
  if (section === "comments") loadComments();
  if (section === "accounts") loadAccounts();
}

window.addEventListener("hashchange", () => {
  const parsed = parseHash();
  if (!parsed) return;
  if (parsed.section === "videos") {
    applyVideoParams(parsed.params);
    state.videosLoaded = false;
  }
  navigate(parsed.section, true);
});

// --------------------------------------------------------- scope switcher
function channelTitle(id) {
  if (!state.status || !state.status.channels) return "";
  const found = state.status.channels.find((c) => String(c.id) === String(id));
  return found ? found.title : "";
}

function renderScopeLabel() {
  const scope = LS.get("scopeChannel", "");
  el("scope-label").textContent = scope
    ? "· Channel: " + (channelTitle(scope) || ("#" + scope))
    : (state.status && state.status.channels && state.status.channels.length > 1 ? "· All channels" : "");
}

function applyScope(channelId, reload) {
  LS.set("scopeChannel", channelId || "");
  el("scope-channel").value = channelId || "";
  el("wizard-channel").value = channelId || "";
  if (channelId) {
    el("video-channel").value = channelId;
    el("episode-channel").value = channelId;
    el("playlist-channel").value = channelId;
  }
  renderScopeLabel();
  markWizardStep(3, !!channelId);
  if (state.section === "videos") updateHash();
  if (reload && state.section === "videos") loadVideos();
}

el("scope-channel").addEventListener("change", () => applyScope(el("scope-channel").value, true));
el("wizard-channel").addEventListener("change", () => applyScope(el("wizard-channel").value, true));

// ---------------------------------------------------------------- status
function oauthText(source) {
  if (source === "env") return "Detected from environment variables (YT_MANAGE_CLIENT_ID / YT_MANAGE_CLIENT_SECRET)";
  if (source === "file") return "Detected from client_secret.json in the data directory";
  return "Not configured yet — set YT_MANAGE_CLIENT_ID and YT_MANAGE_CLIENT_SECRET, or place a client_secret.json file in the data directory";
}

async function loadStatus() {
  try {
    const status = await api("GET", "/api/status");
    state.status = status;
    renderStatus(status);
    return status;
  } catch (err) {
    return null;
  }
}

function setBadge(id, count) {
  const node = el(id);
  node.textContent = count ? String(count) : "";
}

function renderStatus(status) {
  setBadge("badge-videos", status.videoCount);
  setBadge("badge-episodes", status.episodeCount);
  setBadge("badge-schedules", status.scheduleCount);
  setBadge("badge-playlists", status.playlistCount);
  setBadge("badge-comments", status.commentCount);
  setBadge("badge-accounts", status.accountCount);

  const options = (status.channels || []).map((channel) =>
    "<option value=\"" + channel.id + "\">" + escapeHTML(channel.title) + "</option>").join("");
  el("scope-channel").innerHTML = "<option value=\"\">All channels</option>" + options;
  el("wizard-channel").innerHTML = "<option value=\"\">All channels</option>" + options;
  const savedScope = LS.get("scopeChannel", "");
  if (savedScope && (status.channels || []).some((c) => String(c.id) === String(savedScope))) {
    applyScope(savedScope, false);
  }

  el("detected-oauth").textContent = oauthText(status.oauthSource);
  el("detected-datadir").textContent = status.dataDir || "";
  el("set-oauth").textContent = oauthText(status.oauthSource);
  el("set-datadir").textContent = status.dataDir || "";

  const stats = [
    ["Accounts", status.accountCount, "accounts"],
    ["Channels", status.channelCount, "accounts"],
    ["Videos", status.videoCount, "videos"],
    ["Episodes", status.episodeCount, "episodes"],
    ["Playlists", status.playlistCount, "playlists"],
    ["Schedules", status.scheduleCount, "schedules"],
    ["Comments", status.commentCount, "comments"],
  ];
  el("home-stats").innerHTML = stats.map((row) =>
    "<button class=\"stat\" data-goto=\"" + row[2] + "\"><span class=\"stat-num\">" + (row[1] || 0) +
    "</span><span class=\"stat-label\">" + row[0] + "</span></button>").join("");

  renderWizard(status);
  renderChecklist(status);
  renderWhatNext(status);
  renderScopeLabel();
}

el("home-stats").addEventListener("click", (event) => {
  const button = event.target.closest("[data-goto]");
  if (button) navigate(button.dataset.goto);
});

// ------------------------------------------------------ wizard & home
function wizardSkipped() { return LS.get("wizardSkipped", []); }

function markWizardStep(step, done) {
  const node = document.querySelector(".wizard-step[data-step=\"" + step + "\"]");
  if (node) node.classList.toggle("is-done", !!done);
}

function renderWizard(status) {
  const card = el("wizard-card");
  if (status.hasAccount) {
    card.classList.add("hidden");
    return;
  }
  card.classList.remove("hidden");
  el("wizard-oauth-state").textContent = oauthText(status.oauthSource);
  const skipped = wizardSkipped();
  document.querySelectorAll(".wizard-step").forEach((node) => {
    node.classList.toggle("is-skipped", skipped.indexOf(Number(node.dataset.step)) !== -1);
  });
  markWizardStep(1, status.hasAccount);
  markWizardStep(2, status.hasVideos);
  markWizardStep(3, !!LS.get("scopeChannel", ""));
  markWizardStep(4, status.hasAccount && status.hasVideos);
}

document.querySelectorAll(".wiz-skip").forEach((button) => {
  button.addEventListener("click", () => {
    const skipped = wizardSkipped();
    const step = Number(button.dataset.wizSkip);
    if (skipped.indexOf(step) === -1) skipped.push(step);
    LS.set("wizardSkipped", skipped);
    if (state.status) renderWizard(state.status);
  });
});

el("wizard-copy-cmd").addEventListener("click", async () => {
  const button = el("wizard-copy-cmd");
  try {
    await navigator.clipboard.writeText("yt-manage account add");
    button.textContent = "Copied";
  } catch (err) {
    button.textContent = "Select & copy: yt-manage account add";
  }
  setTimeout(() => { button.textContent = "Copy"; }, 2000);
});

el("wizard-sync-btn").addEventListener("click", () => {
  runSync(el("wizard-sync-btn"), "wizard-sync-progress", "wizard-sync-result");
});
el("wizard-finish-btn").addEventListener("click", () => navigate("videos"));

const CHECKLIST_ITEMS = [
  ["connect", "Connect an account", "home"],
  ["sync", "Run your first sync", "accounts"],
  ["episodes", "Group videos into episodes", "episodes"],
  ["schedule", "Schedule an episode", "schedules"],
];

function renderChecklist(status) {
  const card = el("checklist-card");
  if (LS.get("checklistDismissed", false)) { card.classList.add("hidden"); return; }
  const checklist = status.checklist || {};
  card.classList.remove("hidden");
  el("checklist-list").innerHTML = CHECKLIST_ITEMS.map((item) => {
    const done = !!checklist[item[0]];
    return "<li class=\"" + (done ? "done" : "") + "\"><span class=\"box\">" + (done ? "✓" : "○") +
      "</span><button class=\"check-label\" data-goto=\"" + item[2] + "\">" + item[1] + "</button></li>";
  }).join("");
}

el("checklist-dismiss").addEventListener("click", () => {
  LS.set("checklistDismissed", true);
  el("checklist-card").classList.add("hidden");
});

el("checklist-list").addEventListener("click", (event) => {
  const button = event.target.closest("[data-goto]");
  if (button) navigate(button.dataset.goto);
});

function renderWhatNext(status) {
  const card = el("whatnext-card");
  const suggestions = status.suggestions || [];
  if (suggestions.length === 0) { card.classList.add("hidden"); return; }
  card.classList.remove("hidden");
  el("whatnext-list").innerHTML = suggestions.map((suggestion, index) => {
    const section = SECTIONS[suggestion.section] ? suggestion.section : "videos";
    return "<div class=\"whatnext-item\"><span>" + escapeHTML(suggestion.text) +
      "</span><button data-next=\"" + index + "\" data-section-target=\"" + section + "\">Open " +
      SECTIONS[section] + "</button></div>";
  }).join("");
}

el("whatnext-list").addEventListener("click", (event) => {
  const button = event.target.closest("[data-section-target]");
  if (button) navigate(button.dataset.sectionTarget);
});

// ------------------------------------------------------------------ demo
el("demo-seed-btn").addEventListener("click", async () => {
  setStatus("demo-status", "Loading demo channel...");
  try {
    const data = await api("POST", "/api/demo/seed", {});
    setStatus("demo-status",
      "Demo loaded: " + data.videos + " videos, " + data.episodes + " episodes, " + data.playlists + " playlists.", "ok");
    state.videosLoaded = false;
    await loadStatus();
  } catch (err) {
    setStatus("demo-status", err.message, "error");
  }
});

el("demo-clear-btn").addEventListener("click", async () => {
  setStatus("demo-status", "Removing demo data...");
  try {
    await api("POST", "/api/demo/clear", {});
    setStatus("demo-status", "Demo data removed.", "ok");
    state.videosLoaded = false;
    await loadStatus();
    if (state.section === "videos") loadVideos();
  } catch (err) {
    setStatus("demo-status", err.message, "error");
  }
});

// ------------------------------------------------------------------ sync
function formatElapsed(ms) {
  const seconds = ms / 1000;
  return seconds < 60 ? seconds.toFixed(1) + "s" : Math.floor(seconds / 60) + "m " + Math.round(seconds % 60) + "s";
}

function syncStage(elapsedMs) {
  return elapsedMs < 2500 ? "Contacting YouTube…" : "Mirroring videos & playlists…";
}

function pickField(row, names) {
  for (let i = 0; i < names.length; i++) {
    if (row[names[i]] !== undefined && row[names[i]] !== null) return row[names[i]];
  }
  return "";
}

function renderSyncResult(container, data) {
  let rows = [];
  const lists = [data.accounts, data.results, data.perAccount];
  for (let i = 0; i < lists.length; i++) {
    if (Array.isArray(lists[i])) { rows = lists[i]; break; }
  }
  if (rows.length === 0 && (data.videos !== undefined || data.channels !== undefined)) {
    rows = [{ label: "All accounts", channels: data.channels, videos: data.videos, playlists: data.playlists, units: data.units }];
  }
  let html = "";
  if (rows.length > 0) {
    html += "<table class=\"grid sync-table\"><thead><tr><th>Account</th><th>Channels</th><th>Videos</th><th>Playlists</th><th>API units</th></tr></thead><tbody>" +
      rows.map((row) =>
        "<tr><td>" + escapeHTML(pickField(row, ["label", "account", "accountLabel", "accountId", "id"]) || "Account") + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["channels", "channelCount", "channel"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["videos", "videoCount"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["playlists", "playlistCount"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["units", "quotaUnits", "unitsUsed", "apiUnits"])) + "</td></tr>").join("") +
      "</tbody></table>";
  } else {
    html += "<p>Sync finished.</p>";
  }
  if (data.errors && data.errors.length > 0) {
    html += "<ul>" + data.errors.map((err) => "<li>" + escapeHTML(typeof err === "string" ? err : JSON.stringify(err)) + "</li>").join("") + "</ul>";
  }
  container.innerHTML = html;
  container.classList.remove("hidden");
}

async function runSync(button, progressId, resultId) {
  if (state.syncing) return;
  state.syncing = true;
  if (button) button.disabled = true;
  const spinner = el("sync-spinner");
  if (spinner && progressId === "sync-progress") spinner.classList.remove("hidden");
  const started = Date.now();
  const progress = el(progressId);
  const tick = () => {
    const elapsed = Date.now() - started;
    if (progress) progress.textContent = "Syncing… " + formatElapsed(elapsed) + " — " + syncStage(elapsed);
  };
  tick();
  const timer = setInterval(tick, 150);
  try {
    const data = await api("POST", "/api/sync", {});
    const result = el(resultId);
    if (result) renderSyncResult(result, data);
    const hasErrors = data.errors && data.errors.length > 0;
    if (progress) {
      progress.textContent = (hasErrors ? "Sync finished with errors in " : "Sync complete in ") + formatElapsed(Date.now() - started);
      progress.className = "status-line " + (hasErrors ? "error" : "ok");
    }
    if (resultId === "sync-result") {
      setStatus("account-status", hasErrors ? "Sync finished with errors" : "Sync complete", hasErrors ? "error" : "ok");
    }
    state.videosLoaded = false;
    await loadStatus();
    if (state.section === "accounts") loadAccounts();
  } catch (err) {
    if (progress) { progress.textContent = err.message; progress.className = "status-line error"; }
    if (resultId === "sync-result") setStatus("account-status", err.message, "error");
  } finally {
    clearInterval(timer);
    state.syncing = false;
    if (button) button.disabled = false;
    if (spinner) spinner.classList.add("hidden");
  }
}

el("sync-all-btn").addEventListener("click", () => {
  runSync(el("sync-all-btn"), "sync-progress", "sync-result");
});

// ---------------------------------------------------------------- videos
state.visibility = "";
state.shorts = false;
state.hasComments = false;

function setVisibility(value, reload) {
  state.visibility = value || "";
  document.querySelectorAll("#visibility-chips .chip[data-vis]").forEach((chip) =>
    chip.classList.toggle("is-active", chip.dataset.vis === state.visibility));
  if (state.section === "videos") updateHash();
  if (reload) loadVideos();
}

function setToggleChips(shorts, hasComments, reload) {
  state.shorts = !!shorts;
  state.hasComments = !!hasComments;
  el("chip-shorts").setAttribute("aria-pressed", String(state.shorts));
  el("chip-has-comments").setAttribute("aria-pressed", String(state.hasComments));
  if (state.section === "videos") updateHash();
  if (reload) loadVideos();
}

el("visibility-chips").addEventListener("click", (event) => {
  const chip = event.target.closest(".chip");
  if (!chip) return;
  if (chip.id === "chip-shorts") { setToggleChips(!state.shorts, state.hasComments, true); return; }
  if (chip.id === "chip-has-comments") { setToggleChips(state.shorts, !state.hasComments, true); return; }
  if (chip.dataset.vis !== undefined) setVisibility(chip.dataset.vis, true);
});

async function loadVideos() {
  const vs = currentVideoState();
  const params = new URLSearchParams();
  if (vs.q) params.set("q", vs.q);
  if (vs.channel) params.set("channel", vs.channel);
  if (vs.visibility) params.set("visibility", vs.visibility);
  if (vs.sort) params.set("sort", vs.sort);
  if (vs.shorts) params.set("shorts", "1");
  if (vs.hasComments) params.set("hasComments", "1");
  params.set("limit", "200");
  try {
    const data = await api("GET", "/api/videos?" + params.toString());
    state.videos = data.videos || [];
    state.selected.clear();
    state.videosLoaded = true;
    renderVideos();
    renderVideoEmpty(data.count || 0, vs);
    setStatus("video-status", data.count + " video(s)", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
}

function activeFilterLabels(vs) {
  const labels = [];
  if (vs.q) labels.push("search “" + vs.q + "”");
  if (vs.channel) labels.push("channel #" + vs.channel);
  if (vs.visibility) labels.push("visibility: " + vs.visibility);
  if (vs.shorts) labels.push("shorts only");
  if (vs.hasComments) labels.push("has comments");
  return labels;
}

function renderVideoEmpty(count, vs) {
  const box = el("video-empty");
  if (count > 0) { box.classList.add("hidden"); box.innerHTML = ""; return; }
  const libraryEmpty = state.status && !state.status.hasVideos;
  if (libraryEmpty) {
    box.innerHTML = "<h3>No videos yet</h3>" +
      "<p>Your local mirror is empty. Sync a connected account to mirror its videos here — after that, search is instant and costs no YouTube API quota.</p>" +
      "<div class=\"row\"><button id=\"empty-sync-btn\" class=\"primary\">Sync now</button>" +
      "<button id=\"empty-demo-btn\">Load demo channel</button></div>";
  } else {
    const labels = activeFilterLabels(vs);
    box.innerHTML = "<h3>No videos match</h3>" +
      "<p>" + (labels.length ? "Active filters: " + escapeHTML(labels.join(", ")) + ". One of them may be excluding everything." : "Nothing matches the current view.") + "</p>" +
      "<div class=\"row\"><button id=\"empty-clear-btn\" class=\"primary\">Clear filters</button></div>";
  }
  box.classList.remove("hidden");
  const syncBtn = el("empty-sync-btn");
  if (syncBtn) syncBtn.addEventListener("click", () => navigate("accounts"));
  const demoBtn = el("empty-demo-btn");
  if (demoBtn) demoBtn.addEventListener("click", () => el("demo-seed-btn").click());
  const clearBtn = el("empty-clear-btn");
  if (clearBtn) clearBtn.addEventListener("click", clearVideoFilters);
}

function clearVideoFilters() {
  el("video-q").value = "";
  el("video-channel").value = "";
  setVisibility("", false);
  setToggleChips(false, false, false);
  updateHash();
  loadVideos();
}

function renderVideos() {
  const tbody = el("video-rows");
  tbody.classList.remove("refreshing");
  void tbody.offsetWidth;
  const q = currentVideoState().q;
  tbody.innerHTML = state.videos.map((video) => {
    const checked = state.selected.has(video.id) ? " checked" : "";
    const snippet = descriptionSnippet(video.description, q);
    return "<tr data-id=\"" + video.id + "\">" +
      "<td><input type=\"checkbox\" class=\"video-check\" data-id=\"" + video.id + "\"" + checked + "></td>" +
      "<td>" + video.id + "</td>" +
      "<td>" + highlightTitle(video.title) + (snippet ? "<div class=\"snippet\">" + snippet + "</div>" : "") + "</td>" +
      "<td>" + escapeHTML(video.visibility) + "</td>" +
      "<td>" + video.viewCount + "</td>" +
      "<td>" + (video.commentCount || 0) + "</td>" +
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
  if (video) openDrawer(video);
});

el("video-select-all").addEventListener("change", (event) => {
  state.selected.clear();
  if (event.target.checked) state.videos.forEach((v) => state.selected.add(v.id));
  renderVideos();
});

el("pattern-finder").addEventListener("input", renderVideos);
el("video-search-btn").addEventListener("click", () => { updateHash(); loadVideos(); });
el("video-channel").addEventListener("change", () => { updateHash(); loadVideos(); });
el("video-sort").addEventListener("change", () => {
  LS.set("videoSort", el("video-sort").value);
  updateHash();
  loadVideos();
});

// Instant search: 150ms debounce, no button press needed (item 21).
let searchTimer = null;
el("video-q").addEventListener("input", () => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => { updateHash(); loadVideos(); }, 150);
});

// Detail drawer (item 18)
function openDrawer(video) {
  el("edit-video-id").textContent = "#" + video.id + " (" + video.ytVideoId + ")";
  el("drawer-visibility").textContent = video.visibility || "";
  el("drawer-views").textContent = String(video.viewCount || 0);
  el("drawer-comments").textContent = String(video.commentCount || 0);
  el("drawer-published").textContent = video.publishedAt || "";
  el("drawer-yt-link").href = "https://www.youtube.com/watch?v=" + encodeURIComponent(video.ytVideoId || "");
  el("edit-title").value = video.title || "";
  el("edit-description").value = video.description || "";
  el("video-edit-form").dataset.videoId = String(video.id);
  setStatus("edit-status", "");
  el("video-drawer").classList.add("open");
  el("video-drawer").setAttribute("aria-hidden", "false");
  el("drawer-backdrop").classList.remove("hidden");
}

function closeDrawer() {
  el("video-drawer").classList.remove("open");
  el("video-drawer").setAttribute("aria-hidden", "true");
  el("drawer-backdrop").classList.add("hidden");
}

el("drawer-close").addEventListener("click", closeDrawer);
el("drawer-backdrop").addEventListener("click", closeDrawer);
el("edit-cancel").addEventListener("click", closeDrawer);
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeDrawer();
});

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

// Bulk plan / apply (unchanged API behaviour)
el("bulk-plan-btn").addEventListener("click", async () => {
  setStatus("video-status", "Building plan...");
  const vs = currentVideoState();
  const body = {
    q: vs.q,
    visibility: vs.visibility,
    pattern: {
      template: el("bulk-template").value,
      regex: el("bulk-regex").value,
      replacement: el("bulk-replacement").value,
      descriptionTemplate: el("bulk-desc-template").value,
      startIndex: Number(el("bulk-start").value || 1),
    },
  };
  if (vs.channel) body.channelId = Number(vs.channel);
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

// ----------------------------------------------------------- saved searches
async function savedSearchCount(saved) {
  const params = new URLSearchParams();
  if (saved.q) params.set("q", saved.q);
  if (saved.channelId) params.set("channel", saved.channelId);
  if (saved.visibility) params.set("visibility", saved.visibility);
  params.set("limit", "1");
  try {
    const data = await api("GET", "/api/videos?" + params.toString());
    return data.count || 0;
  } catch (err) {
    return null;
  }
}

async function renderSavedSearches() {
  const list = el("saved-search-list");
  list.innerHTML = state.savedSearches.map((saved) =>
    "<div class=\"saved-item\"><button class=\"saved-apply\" data-saved=\"" + saved.id + "\">" +
    "<span class=\"saved-name\">" + escapeHTML(saved.name) + "</span>" +
    "<span class=\"saved-count\" id=\"saved-count-" + saved.id + "\">…</span></button>" +
    "<button class=\"saved-del\" data-saved-del=\"" + saved.id + "\" title=\"Delete saved search\">✕</button></div>").join("");
  for (const saved of state.savedSearches) {
    const count = await savedSearchCount(saved);
    const node = el("saved-count-" + saved.id);
    if (node) node.textContent = count === null ? "" : String(count);
  }
}

async function loadSavedSearches() {
  try {
    state.savedSearches = await api("GET", "/api/saved-searches");
    await renderSavedSearches();
  } catch (err) {
    state.savedSearches = [];
  }
}

el("video-save-search-btn").addEventListener("click", async () => {
  const name = window.prompt("Name this search");
  if (!name) return;
  const vs = currentVideoState();
  try {
    state.savedSearches = await api("POST", "/api/saved-searches", {
      name,
      q: vs.q,
      channelId: vs.channel ? Number(vs.channel) : 0,
      visibility: vs.visibility,
      sort: vs.sort,
    });
    await renderSavedSearches();
    setStatus("video-status", "Saved search “" + name + "”", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

el("saved-search-list").addEventListener("click", async (event) => {
  const del = event.target.closest("[data-saved-del]");
  const apply = event.target.closest("[data-saved]");
  if (del) {
    try {
      state.savedSearches = await api("DELETE", "/api/saved-searches/" + del.dataset.savedDel);
      await renderSavedSearches();
    } catch (err) { /* list stays as-is */ }
    return;
  }
  if (apply) {
    const saved = state.savedSearches.find((s) => String(s.id) === String(apply.dataset.saved));
    if (!saved) return;
    el("video-q").value = saved.q || "";
    el("video-channel").value = saved.channelId ? String(saved.channelId) : "";
    el("video-sort").value = saved.sort || "newest";
    LS.set("videoSort", el("video-sort").value);
    setVisibility(saved.visibility || "", false);
    setToggleChips(false, false, false);
    navigate("videos");
    state.videosLoaded = false;
    updateHash();
    loadVideos();
  }
});

// --------------------------------------------------------------- episodes
function episodeEmptyHTML() {
  return "<div class=\"empty-state\"><h3>No episodes yet</h3>" +
    "<p>Episodes are grouped automatically from titles like “Series EP3 Part 2” — the grouper reads episode and part markers even when they are inconsistent.</p>" +
    "<p>Sync a channel first, then run grouping from the CLI: <code>yt-manage episode group</code></p></div>";
}

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
    }).join("") || episodeEmptyHTML();
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
    loadStatus();
  } catch (err) {
    setStatus("schedule-status", err.message, "error");
  }
});

// --------------------------------------------------------------- schedules
async function loadSchedules() {
  setStatus("schedules-status", "Loading...");
  try {
    const rules = await api("GET", "/api/schedules");
    el("schedule-rows").innerHTML = rules.map((rule) =>
      "<tr><td>" + rule.id + "</td><td>" + escapeHTML(channelTitle(rule.channelId) || ("#" + rule.channelId)) + "</td>" +
      "<td>" + rule.cadenceDays + "</td><td>" + rule.videosPerEpisode + "</td>" +
      "<td>" + escapeHTML(rule.startsAt || "") + "</td><td>" + (rule.isActive ? "active" : "inactive") + "</td></tr>").join("");
    el("schedules-empty").innerHTML = rules.length === 0
      ? "<div class=\"empty-state\"><h3>No schedule rules yet</h3>" +
        "<p>A rule is created when you schedule an episode: it fixes the cadence (every day, week, 2 or 3 weeks) and the start date for a whole series.</p>" +
        "<p>Go to Episodes, load a channel, and use “Schedule this series”.</p></div>"
      : "";
    setStatus("schedules-status", rules.length + " rule(s)", "ok");
  } catch (err) {
    setStatus("schedules-status", err.message, "error");
  }
}

el("schedules-load-btn").addEventListener("click", loadSchedules);

// --------------------------------------------------------------- playlists
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
      "<div class=\"empty-state\"><h3>No playlists for this channel</h3>" +
      "<p>Playlists you create in YouTube Studio appear here after a sync — or create one with the form below and it is made on YouTube right away.</p></div>";
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

// --------------------------------------------------------------- comments
async function loadComments() {
  setStatus("comments-status", "Loading...");
  try {
    const comments = await api("GET", "/api/comments");
    el("comment-list").innerHTML = comments.map((comment) =>
      "<div class=\"card\"><h3>" + escapeHTML(comment.videoTitle || ("Video " + comment.videoId)) +
      " <span class=\"pill\">" + (comment.isPinnedKnown ? "pin marked in Studio" : "pin not marked") + "</span></h3>" +
      "<p>" + escapeHTML(comment.text) + "</p>" +
      "<p class=\"muted-text\">" + escapeHTML(comment.publishedAt || "") +
      (comment.ytVideoId ? " · <a href=\"https://www.youtube.com/watch?v=" + encodeURIComponent(comment.ytVideoId) + "\" target=\"_blank\" rel=\"noopener\">Watch on YouTube</a>" : "") +
      "</p></div>").join("") ||
      "<div class=\"empty-state\"><h3>No first comments yet</h3>" +
      "<p>yt-manage posts the first comment on a video from the CLI: <code>yt-manage comment post</code>. Posted comments are tracked here; pinning stays a manual YouTube Studio step.</p></div>";
    setStatus("comments-status", comments.length + " comment(s)", "ok");
  } catch (err) {
    setStatus("comments-status", err.message, "error");
  }
}

el("comments-load-btn").addEventListener("click", loadComments);

// --------------------------------------------------------------- A/B tests
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
    }).join("") ||
      "<div class=\"empty-state\"><h3>No A/B tests yet</h3>" +
      "<p>An A/B test tries two titles on one video. Create one with the form below, take snapshots as views come in, then pick the winner and yt-manage applies it.</p></div>";
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

// --------------------------------------------------------------- accounts
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
    el("accounts-empty").innerHTML = accounts.length === 0
      ? "<div class=\"empty-state\"><h3>No accounts connected</h3>" +
        "<p>Connect your first YouTube account from a terminal: <code>yt-manage account add</code> — it opens the Google consent flow, and the token is stored encrypted on this machine.</p></div>"
      : "";
    setStatus("account-status", accounts.length + " account(s)", "ok");
  } catch (err) {
    setStatus("account-status", err.message, "error");
  }
}

el("accounts-load-btn").addEventListener("click", loadAccounts);

// ------------------------------------------------------------------- init
applyDensity(LS.get("density", "comfortable"));
applySidebar(LS.get("sidebarCollapsed", false));
el("video-sort").value = LS.get("videoSort", "newest");
setVisibility("", false);
setToggleChips(false, false, false);

(async function init() {
  await loadStatus();
  loadSavedSearches();
  const parsed = parseHash();
  if (parsed) {
    if (parsed.section === "videos") applyVideoParams(parsed.params);
    navigate(parsed.section, true);
  } else {
    navigate(LS.get("lastSection", "home"), true);
  }
})();
