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
  // Wave 2 (items 26-50)
  activeVideoIndex: -1,
  suggestions: [],
  suggestionIndex: -1,
  searchAll: null,
  searchAllOpen: false,
  planIncluded: new Set(),
  planApplied: new Set(),
  planResults: new Map(),
  planFailedIds: [],
  recipes: [],
  episodes: [],
  episodeMergeSelected: new Set(),
  schedulePlan: null,
  schedulePlanEpisodeId: null,
  schedulePlanWarnings: [],
  calendarCursor: null,
  calendarVideos: [],
  calendarRules: [],
  dragPart: null,
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
    state.activeVideoIndex = -1;
    state.videosLoaded = true;
    if (vs.q) recordRecentSearch(vs.q);
    renderVideos();
    updateBulkPreview();
    renderVideoEmpty(data.count || 0, vs);
    setStatus("video-status", data.count + " video(s)", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
}

function activeFilters(vs) {
  const filters = [];
  if (vs.q) filters.push({ key: "q", label: "search “" + vs.q + "”" });
  if (vs.channel) filters.push({ key: "channel", label: "channel #" + vs.channel });
  if (vs.visibility) filters.push({ key: "visibility", label: "visibility: " + vs.visibility });
  if (vs.shorts) filters.push({ key: "shorts", label: "shorts only" });
  if (vs.hasComments) filters.push({ key: "hasComments", label: "has comments" });
  return filters;
}

function activeFilterLabels(vs) {
  return activeFilters(vs).map((filter) => filter.label);
}

function removeVideoFilter(key) {
  if (key === "q") el("video-q").value = "";
  if (key === "channel") el("video-channel").value = "";
  if (key === "visibility") setVisibility("", false);
  if (key === "shorts") setToggleChips(false, state.hasComments, false);
  if (key === "hasComments") setToggleChips(state.shorts, false, false);
  updateHash();
  loadVideos();
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
    const filters = activeFilters(vs);
    box.innerHTML = "<h3>No videos match</h3>" +
      "<p>" + (filters.length ? "One of these filters may be excluding everything. Remove one at a time to find it:" : "Nothing matches the current view.") + "</p>" +
      (filters.length ? "<div class=\"filter-recovery\">" + filters.map((filter) =>
        "<span class=\"filter-token\">" + escapeHTML(filter.label) +
        "<button data-remove-filter=\"" + filter.key + "\" title=\"Remove this filter\">✕</button></span>").join("") + "</div>" : "") +
      "<div class=\"row\"><button id=\"empty-clear-btn\" class=\"primary\">Clear all filters</button></div>";
  }
  box.classList.remove("hidden");
  const syncBtn = el("empty-sync-btn");
  if (syncBtn) syncBtn.addEventListener("click", () => navigate("accounts"));
  const demoBtn = el("empty-demo-btn");
  if (demoBtn) demoBtn.addEventListener("click", () => el("demo-seed-btn").click());
  const clearBtn = el("empty-clear-btn");
  if (clearBtn) clearBtn.addEventListener("click", clearVideoFilters);
  box.querySelectorAll("[data-remove-filter]").forEach((button) =>
    button.addEventListener("click", () => removeVideoFilter(button.dataset.removeFilter)));
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
  tbody.innerHTML = state.videos.map((video, index) => {
    const checked = state.selected.has(video.id) ? " checked" : "";
    const active = index === state.activeVideoIndex ? " class=\"keyboard-active\"" : "";
    const snippet = descriptionSnippet(video.description, q);
    return "<tr data-id=\"" + video.id + "\"" + active + ">" +
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
let suggestionTimer = null;
el("video-q").addEventListener("input", () => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => { updateHash(); loadVideos(); }, 150);
  if (suggestionTimer) clearTimeout(suggestionTimer);
  suggestionTimer = setTimeout(loadSuggestions, 200);
});

// ------------------------------------------------- recent & suggested
// Recent searches, last 8, localStorage (item 26).
function recentSearches() { return LS.get("recentSearches", []); }

function recordRecentSearch(query) {
  const value = String(query || "").trim();
  if (!value) return;
  const next = [value].concat(recentSearches().filter((item) => item !== value)).slice(0, 8);
  LS.set("recentSearches", next);
}

function hideSearchDropdown() {
  el("search-dropdown").classList.add("hidden");
  state.suggestionIndex = -1;
}

function dropdownItems() {
  return Array.from(el("search-dropdown").querySelectorAll("[data-query]"));
}

function setSuggestionIndex(index) {
  const items = dropdownItems();
  if (items.length === 0) { state.suggestionIndex = -1; return; }
  state.suggestionIndex = Math.max(0, Math.min(index, items.length - 1));
  items.forEach((item, i) => item.classList.toggle("is-active", i === state.suggestionIndex));
}

function renderSearchDropdown() {
  const box = el("search-dropdown");
  const query = el("video-q").value.trim();
  const recents = recentSearches();
  let html = "";
  if (!query && recents.length > 0) {
    html += "<div class=\"dropdown-heading\">Recent searches</div>" +
      recents.map((item) => "<button class=\"dropdown-item\" data-query=\"" + escapeHTML(item) + "\">" + escapeHTML(item) + "</button>").join("") +
      "<button class=\"dropdown-clear\" data-clear-recents=\"1\">Clear recent searches</button>";
  }
  if (query && state.suggestions.length > 0) {
    html += "<div class=\"dropdown-heading\">Suggestions from your videos</div>" +
      state.suggestions.map((item) => "<button class=\"dropdown-item\" data-query=\"" + escapeHTML(item) + "\">" + escapeHTML(item) + "</button>").join("");
  }
  if (query && recents.length > 0) {
    const matching = recents.filter((item) => item !== query);
    if (matching.length > 0) {
      html += "<div class=\"dropdown-heading\">Recent searches</div>" +
        matching.map((item) => "<button class=\"dropdown-item\" data-query=\"" + escapeHTML(item) + "\">" + escapeHTML(item) + "</button>").join("") +
        "<button class=\"dropdown-clear\" data-clear-recents=\"1\">Clear recent searches</button>";
    }
  }
  box.innerHTML = html;
  box.classList.toggle("hidden", html === "");
  state.suggestionIndex = -1;
}

async function loadSuggestions() {
  const query = el("video-q").value.trim();
  if (!query) { state.suggestions = []; renderSearchDropdown(); return; }
  try {
    const data = await api("GET", "/api/search/suggestions?q=" + encodeURIComponent(query));
    if (el("video-q").value.trim() !== query) return;
    state.suggestions = data.suggestions || [];
    renderSearchDropdown();
  } catch (err) {
    state.suggestions = [];
  }
}

function applySearchQuery(query) {
  el("video-q").value = query;
  hideSearchDropdown();
  updateHash();
  loadVideos();
  el("video-q").focus();
}

el("video-q").addEventListener("focus", renderSearchDropdown);
el("search-dropdown").addEventListener("click", (event) => {
  const clear = event.target.closest("[data-clear-recents]");
  if (clear) {
    LS.set("recentSearches", []);
    renderSearchDropdown();
    return;
  }
  const item = event.target.closest("[data-query]");
  if (item) applySearchQuery(item.dataset.query);
});
document.addEventListener("click", (event) => {
  if (!event.target.closest(".search-box")) hideSearchDropdown();
});

// ---------------------------------------------- keyboard-first search
// "/" focuses search; arrows move through results; Enter opens the
// active (or first) result; Esc returns to search (item 29).
function setActiveVideo(index) {
  if (state.videos.length === 0) return;
  state.activeVideoIndex = Math.max(0, Math.min(index, state.videos.length - 1));
  renderVideos();
  const row = el("video-rows").children[state.activeVideoIndex];
  if (row) row.scrollIntoView({ block: "nearest" });
}

function openActiveVideo() {
  const index = state.activeVideoIndex >= 0 ? state.activeVideoIndex : 0;
  if (state.videos[index]) openDrawer(state.videos[index]);
}

function isTypingTarget(target) {
  return target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.tagName === "SELECT" || target.isContentEditable);
}

document.addEventListener("keydown", (event) => {
  if (event.key === "/" && !isTypingTarget(event.target)) {
    event.preventDefault();
    if (state.section !== "videos") navigate("videos");
    el("video-q").focus();
    el("video-q").select();
    return;
  }
  if (state.section !== "videos") return;
  if (event.key === "Escape" && event.target === el("video-q") &&
      !el("search-dropdown").classList.contains("hidden")) {
    hideSearchDropdown();
    return;
  }
  const dropdownOpen = !el("search-dropdown").classList.contains("hidden") && dropdownItems().length > 0;
  if (event.target === el("video-q") && dropdownOpen && (event.key === "ArrowDown" || event.key === "ArrowUp")) {
    // While the dropdown is open, arrows walk its suggestions/recent
    // items; once it closes, arrows walk the results (item 27 + 29).
    event.preventDefault();
    setSuggestionIndex(state.suggestionIndex + (event.key === "ArrowDown" ? 1 : -1));
    return;
  }
  if (event.target === el("video-q") && event.key === "Enter" && state.suggestionIndex >= 0) {
    const item = dropdownItems()[state.suggestionIndex];
    if (item) {
      event.preventDefault();
      applySearchQuery(item.dataset.query);
      return;
    }
  }
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    if (event.target !== el("video-q") && isTypingTarget(event.target)) return;
    event.preventDefault();
    hideSearchDropdown();
    setActiveVideo(state.activeVideoIndex + (event.key === "ArrowDown" ? 1 : -1));
    return;
  }
  if (event.key === "Enter" && event.target === el("video-q")) {
    if (state.videos.length > 0) {
      event.preventDefault();
      openActiveVideo();
    }
    return;
  }
  if (event.key === "Escape" && !el("video-drawer").classList.contains("open")) {
    el("video-q").focus();
  }
});

// Suggestions are real buttons: keyboard users can Tab to one and
// press Enter to apply it (item 27).
el("search-dropdown").addEventListener("keydown", (event) => {
  const item = event.target.closest("[data-query]");
  if (item && event.key === "Enter") {
    event.preventDefault();
    applySearchQuery(item.dataset.query);
  }
});

// --------------------------------------------------- search everywhere
// One query across videos, playlists, episodes and comments (item 30).
async function runSearchAll() {
  const query = el("video-q").value.trim();
  const box = el("search-all-results");
  if (!query) { setStatus("video-status", "Type a search first, then use Search everywhere.", "error"); return; }
  recordRecentSearch(query);
  box.classList.remove("hidden");
  box.innerHTML = "<p class=\"muted-text\">Searching everything for “" + escapeHTML(query) + "”…</p>";
  el("search-all-btn").setAttribute("aria-pressed", "true");
  state.searchAllOpen = true;
  try {
    const data = await api("GET", "/api/search/all?q=" + encodeURIComponent(query));
    state.searchAll = data;
    renderSearchAll(query, data);
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(err.message) + "</p>";
  }
}

function renderSearchAll(query, data) {
  const box = el("search-all-results");
  const group = (title, items) => items.length === 0 ? "" :
    "<h3>" + title + " (" + items.length + ")</h3><div class=\"search-all-group\">" + items.join("") + "</div>";
  const videos = (data.videos || []).map((video) =>
    "<button class=\"search-all-item\" data-all-video=\"" + video.id + "\"><strong>" + escapeHTML(video.title) +
    "</strong><span>" + escapeHTML(video.visibility || "") + " · " + (video.viewCount || 0) + " views</span></button>");
  const playlists = (data.playlists || []).map((playlist) =>
    "<button class=\"search-all-item\" data-all-playlist=\"" + playlist.id + "\" data-channel=\"" + playlist.channelId + "\"><strong>" +
    escapeHTML(playlist.title) + "</strong><span>" + (playlist.itemCount || 0) + " items</span></button>");
  const episodes = (data.episodes || []).map((episode) =>
    "<button class=\"search-all-item\" data-all-episode=\"" + episode.id + "\" data-channel=\"" + episode.channelId + "\"><strong>" +
    escapeHTML(episode.title || episode.episodeKey) + "</strong><span>Episode " + episode.episodeNumber + " · " + (episode.parts || []).length + " parts</span></button>");
  const comments = (data.comments || []).map((comment) =>
    "<button class=\"search-all-item\" data-all-comment=\"" + comment.videoId + "\"><strong>" +
    escapeHTML(comment.videoTitle || ("Video " + comment.videoId)) + "</strong><span>" + escapeHTML(String(comment.text || "").slice(0, 90)) + "</span></button>");
  const total = videos.length + playlists.length + episodes.length + comments.length;
  box.innerHTML = "<div class=\"card-head\"><h3>Everything matching “" + escapeHTML(query) + "”</h3>" +
    "<button id=\"search-all-close\">Close</button></div>" +
    (total === 0 ? "<p class=\"muted-text\">No matches in videos, playlists, episodes or comments.</p>" :
      group("Videos", videos) + group("Playlists", playlists) + group("Episodes", episodes) + group("Comments", comments));
  const close = el("search-all-close");
  if (close) close.addEventListener("click", () => {
    box.classList.add("hidden");
    state.searchAllOpen = false;
    el("search-all-btn").setAttribute("aria-pressed", "false");
  });
}

el("search-all-btn").addEventListener("click", () => {
  if (state.searchAllOpen) {
    el("search-all-results").classList.add("hidden");
    state.searchAllOpen = false;
    el("search-all-btn").setAttribute("aria-pressed", "false");
  } else {
    runSearchAll();
  }
});

el("search-all-results").addEventListener("click", async (event) => {
  const videoBtn = event.target.closest("[data-all-video]");
  const playlistBtn = event.target.closest("[data-all-playlist]");
  const episodeBtn = event.target.closest("[data-all-episode]");
  const commentBtn = event.target.closest("[data-all-comment]");
  if (videoBtn) {
    const video = (state.searchAll.videos || []).find((v) => String(v.id) === videoBtn.dataset.allVideo);
    if (video) { navigate("videos"); openDrawer(video); }
    return;
  }
  if (playlistBtn) {
    el("playlist-channel").value = playlistBtn.dataset.channel || "";
    navigate("playlists");
    loadPlaylists();
    return;
  }
  if (episodeBtn) {
    el("episode-channel").value = episodeBtn.dataset.channel || "";
    navigate("episodes");
    loadEpisodes();
    return;
  }
  if (commentBtn) {
    try {
      const video = await api("GET", "/api/videos/" + commentBtn.dataset.allComment);
      navigate("videos");
      openDrawer(video);
    } catch (err) {
      setStatus("video-status", err.message, "error");
    }
  }
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
  loadVideoHistory(video.id);
  el("video-drawer").classList.add("open");
  el("video-drawer").setAttribute("aria-hidden", "false");
  el("drawer-backdrop").classList.remove("hidden");
}

// Rename history in the drawer (item 38).
async function loadVideoHistory(videoId) {
  const box = el("drawer-history");
  box.innerHTML = "<p class=\"muted-text\">Loading history…</p>";
  try {
    const entries = await api("GET", "/api/videos/" + videoId + "/history");
    if (el("video-edit-form").dataset.videoId !== String(videoId)) return;
    if (!entries || entries.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No recorded renames yet</p>";
      return;
    }
    box.innerHTML = entries.map((entry) =>
      "<div class=\"history-entry\"><div class=\"history-change\">" + escapeHTML(entry.oldTitle) +
      " <span class=\"move-arrow\">→</span> " + escapeHTML(entry.newTitle) + "</div>" +
      "<div class=\"muted-text\">" + escapeHTML(entry.source || "") +
      (entry.changedAt ? " · " + escapeHTML(entry.changedAt) : "") + "</div></div>").join("");
  } catch (err) {
    if (el("video-edit-form").dataset.videoId === String(videoId)) {
      box.innerHTML = "<p class=\"muted-text\">No recorded renames yet</p>";
    }
  }
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

// ---------------------------------------------------------- bulk edit
// Template builder with live preview on real titles (items 32, 40).
function substituteBulkTemplate(template, video, n) {
  return String(template || "")
    .replace(/\{title\}/g, video.title || "")
    .replace(/\{description\}/g, video.description || "")
    .replace(/\{n\}/g, String(n));
}

function updateBulkPreview() {
  const box = el("bulk-preview");
  if (!box) return;
  updateBulkMatchCount();
  const template = el("bulk-template").value;
  const descTemplate = el("bulk-desc-template").value;
  if (!template && !descTemplate) {
    box.innerHTML = "<span class=\"muted-text\">Preview: type a template to see it applied to your first 3 loaded titles.</span>";
    return;
  }
  const start = Number(el("bulk-start").value || 1);
  const sample = state.videos.slice(0, 3);
  if (sample.length === 0) {
    box.innerHTML = "<span class=\"muted-text\">Preview: load some videos first — the preview uses real titles.</span>";
    return;
  }
  box.innerHTML = "<span class=\"bulk-label\">Preview:</span>" + sample.map((video, index) => {
    let line = "<div class=\"preview-line\">" + escapeHTML(video.title) + " <span class=\"move-arrow\">→</span> <strong>" +
      escapeHTML(template ? substituteBulkTemplate(template, video, start + index) : video.title) + "</strong>";
    if (descTemplate) {
      line += "<div class=\"snippet\">Description: " + escapeHTML(substituteBulkTemplate(descTemplate, video, start + index)) + "</div>";
    }
    return line + "</div>";
  }).join("");
}

// Find & replace live match count over the loaded titles (item 33).
function updateBulkMatchCount() {
  const node = el("bulk-match-count");
  const pattern = el("bulk-regex").value;
  if (!pattern) { node.textContent = ""; return; }
  let regex = null;
  try { regex = new RegExp(pattern); } catch (err) { node.textContent = "Invalid regex"; return; }
  const count = state.videos.filter((video) => regex.test(video.title || "")).length;
  node.textContent = count + " of " + state.videos.length + " loaded titles match";
}

function insertAtCursor(input, text) {
  const start = input.selectionStart == null ? input.value.length : input.selectionStart;
  const end = input.selectionEnd == null ? input.value.length : input.selectionEnd;
  input.value = input.value.slice(0, start) + text + input.value.slice(end);
  input.focus();
  const pos = start + text.length;
  try { input.setSelectionRange(pos, pos); } catch (err) { /* not all inputs support it */ }
  updateBulkPreview();
}

document.querySelectorAll(".token-chip").forEach((chip) => {
  chip.addEventListener("click", () => insertAtCursor(el("bulk-template"), chip.dataset.token));
});
["bulk-template", "bulk-desc-template", "bulk-start"].forEach((id) =>
  el(id).addEventListener("input", updateBulkPreview));
el("bulk-regex").addEventListener("input", updateBulkMatchCount);

// Rename recipes (item 39): saved pattern sets, reusable anywhere.
function currentPatternFields() {
  return {
    template: el("bulk-template").value,
    regex: el("bulk-regex").value,
    replacement: el("bulk-replacement").value,
    descriptionTemplate: el("bulk-desc-template").value,
    startIndex: Number(el("bulk-start").value || 1),
  };
}

function renderRecipes(selectedId) {
  const select = el("recipe-select");
  select.innerHTML = "<option value=\"\">No recipe selected</option>" + state.recipes.map((recipe) =>
    "<option value=\"" + recipe.id + "\">" + escapeHTML(recipe.name) + "</option>").join("");
  if (selectedId) select.value = selectedId;
}

async function loadRecipes() {
  try {
    state.recipes = await api("GET", "/api/rename-recipes");
    renderRecipes(el("recipe-select").value);
  } catch (err) {
    state.recipes = [];
  }
}

el("recipe-select").addEventListener("change", () => {
  const recipe = state.recipes.find((item) => String(item.id) === el("recipe-select").value);
  if (!recipe) return;
  el("bulk-template").value = recipe.template || "";
  el("bulk-regex").value = recipe.regex || "";
  el("bulk-replacement").value = recipe.replacement || "";
  el("bulk-desc-template").value = recipe.descriptionTemplate || "";
  el("bulk-start").value = recipe.startIndex || 1;
  updateBulkPreview();
  setStatus("video-status", "Recipe “" + recipe.name + "” loaded", "ok");
});

el("recipe-save-btn").addEventListener("click", async () => {
  const name = window.prompt("Name this recipe");
  if (!name) return;
  try {
    state.recipes = await api("POST", "/api/rename-recipes", Object.assign({ name }, currentPatternFields()));
    const saved = state.recipes.filter((item) => item.name === name).pop();
    renderRecipes(saved ? saved.id : "");
    setStatus("video-status", "Recipe “" + name + "” saved", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

el("recipe-delete-btn").addEventListener("click", async () => {
  const id = el("recipe-select").value;
  if (!id) { setStatus("video-status", "Select a recipe to delete.", "error"); return; }
  try {
    state.recipes = await api("DELETE", "/api/rename-recipes/" + id);
    renderRecipes("");
    setStatus("video-status", "Recipe deleted", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

// Change-set plan: include/exclude, warnings, staged apply, retry
// (items 31, 34-37).
function planPendingChanges() {
  if (!state.bulkPlan) return [];
  return (state.bulkPlan.changes || []).filter((change) => !state.planApplied.has(change.videoId));
}

function planIncludedIds() {
  return planPendingChanges()
    .filter((change) => state.planIncluded.has(change.videoId))
    .map((change) => change.videoId);
}

function updatePlanLabels() {
  if (!state.bulkPlan) return;
  const included = planIncludedIds();
  const pending = planPendingChanges();
  el("bulk-apply-btn").textContent = "Apply included (" + included.length + ")";
  el("bulk-apply-all-btn").textContent = "Apply all changes (" + pending.length + ")";
  el("bulk-apply-first-btn").textContent = "Apply first " + Math.min(5, included.length);
  const canApply = !!state.planKept && pending.length > 0;
  el("bulk-apply-btn").disabled = !canApply || included.length === 0;
  el("bulk-apply-all-btn").disabled = !canApply;
  el("bulk-apply-first-btn").disabled = !canApply || included.length === 0;
  const retry = el("bulk-retry-btn");
  const showRetry = !!state.planKept && state.planFailedIds.length > 0;
  retry.classList.toggle("hidden", !showRetry);
  retry.disabled = !showRetry;
  retry.textContent = "Retry failed only (" + state.planFailedIds.length + ")";
}

function renderBulkPlan() {
  if (!state.bulkPlan) return;
  const changes = state.bulkPlan.changes || [];
  const warned = changes.filter((change) => (change.warnings || []).length > 0).length;
  el("bulk-summary").textContent = (state.bulkPlan.summary || "") +
    "\n" + warned + " of " + changes.length + " change(s) have warnings" +
    ((state.bulkPlan.warnings || []).length ? "\n" + state.bulkPlan.warnings.join("\n") : "");
  el("bulk-changes").innerHTML = changes.map((change) => {
    const applied = state.planApplied.has(change.videoId);
    const included = state.planIncluded.has(change.videoId) && !applied;
    const result = state.planResults.get(change.videoId);
    const rowClass = applied ? "plan-applied" : (included ? "" : "plan-excluded");
    const warnings = (change.warnings || []).map((warning) =>
      "<span class=\"warn-badge\">⚠ " + escapeHTML(warning) + "</span>").join(" ");
    let resultCell = "";
    if (result) {
      resultCell = result.ok
        ? "<span class=\"result-ok\">✓ Done</span>"
        : "<span class=\"result-fail\">✗ " + escapeHTML(result.error || "Failed") + "</span>";
    }
    return "<tr class=\"" + rowClass + "\" data-plan-row=\"" + change.videoId + "\">" +
      "<td><input type=\"checkbox\" class=\"plan-include\" data-plan-video=\"" + change.videoId + "\"" +
      (included ? " checked" : "") + (applied ? " disabled" : "") + "></td>" +
      "<td>" + escapeHTML(change.ytVideoId) + "</td><td>" + escapeHTML(change.oldTitle) +
      "</td><td>" + escapeHTML(change.newTitle) + "</td><td>" +
      (change.hasDescriptionChange ? "changes" : "") + "</td><td>" + warnings + "</td><td>" + resultCell + "</td></tr>";
  }).join("");
  updatePlanLabels();
}

el("bulk-changes").addEventListener("change", (event) => {
  const box = event.target.closest(".plan-include");
  if (!box) return;
  const id = Number(box.dataset.planVideo);
  if (box.checked) state.planIncluded.add(id); else state.planIncluded.delete(id);
  const row = box.closest("tr");
  if (row) row.classList.toggle("plan-excluded", !box.checked);
  updatePlanLabels();
});

el("bulk-plan-btn").addEventListener("click", async () => {
  setStatus("video-status", "Building plan...");
  const vs = currentVideoState();
  const body = {
    q: vs.q,
    visibility: vs.visibility,
    pattern: currentPatternFields(),
  };
  if (vs.channel) body.channelId = Number(vs.channel);
  try {
    const data = await api("POST", "/api/videos/bulk-plan", body);
    state.bulkPlan = data;
    state.planIncluded = new Set((data.changes || []).map((change) => change.videoId));
    state.planApplied = new Set();
    state.planResults = new Map();
    state.planFailedIds = [];
    state.planKept = true;
    el("bulk-plan").classList.remove("hidden");
    renderBulkPlan();
    setStatus("video-status", (data.changes || []).length + " change(s) planned", "ok");
  } catch (err) {
    setStatus("video-status", err.message, "error");
  }
});

async function applyPlanIds(ids, label) {
  if (!state.bulkPlan || ids.length === 0) return;
  setStatus("video-status", label + "...");
  ["bulk-apply-btn", "bulk-apply-all-btn", "bulk-apply-first-btn", "bulk-retry-btn"].forEach((id) => { el(id).disabled = true; });
  try {
    const data = await api("POST", "/api/videos/bulk-edit", {
      planId: state.bulkPlan.planId,
      confirmToken: state.bulkPlan.confirmToken,
      onlyIds: ids,
    });
    let appliedNow = 0;
    (data.results || []).forEach((result) => {
      state.planResults.set(result.videoId, result);
      state.planFailedIds = state.planFailedIds.filter((id) => id !== result.videoId);
      if (result.ok) {
        appliedNow++;
        state.planApplied.add(result.videoId);
      } else if (!state.planFailedIds.includes(result.videoId)) {
        state.planFailedIds.push(result.videoId);
      }
    });
    state.planKept = !!data.planKept;
    renderBulkPlan();
    const failedNow = (data.results || []).filter((result) => !result.ok).length;
    setStatus("video-status",
      "Applied " + appliedNow + ", failed " + failedNow + ", " + (data.remaining || 0) + " remaining in plan",
      failedNow ? "error" : "ok");
    await loadVideos();
    loadStatus();
  } catch (err) {
    setStatus("video-status", err.message, "error");
    updatePlanLabels();
  }
}

el("bulk-apply-btn").addEventListener("click", () => applyPlanIds(planIncludedIds(), "Applying included rows"));
el("bulk-apply-first-btn").addEventListener("click", () => applyPlanIds(planIncludedIds().slice(0, 5), "Applying first 5"));
el("bulk-apply-all-btn").addEventListener("click", () =>
  applyPlanIds(planPendingChanges().map((change) => change.videoId), "Applying all changes"));
el("bulk-retry-btn").addEventListener("click", () =>
  applyPlanIds(state.planFailedIds.slice(), "Retrying failed rows"));

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

function episodeSeriesName(episode) {
  const key = String(episode.episodeKey || "");
  if (key.indexOf("|ep") !== -1) return key.split("|ep")[0];
  return episode.title || key || "Series";
}

function episodeDisplayRows(episode) {
  const rows = (episode.parts || []).map((part) => ({ kind: "part", partNumber: part.partNumber, sequence: part.sequence, part }));
  (episode.gapParts || []).forEach((number) =>
    rows.push({ kind: "gap", partNumber: number, sequence: 9999, part: null }));
  rows.sort((a, b) => (a.partNumber - b.partNumber) || (a.sequence - b.sequence));
  return rows;
}

function partStatusChip(part) {
  if (part.visibility === "scheduled") {
    return "<span class=\"status-chip is-scheduled\">● Scheduled" + (part.publishAt ? " " + escapeHTML(part.publishAt) : "") + "</span>";
  }
  return "<span class=\"pill\">" + escapeHTML(part.visibility || "") + "</span>";
}

function renderEpisodeBoard() {
  const list = el("episode-list");
  if (state.episodes.length === 0) { list.innerHTML = episodeEmptyHTML(); return; }
  const groups = new Map();
  state.episodes.forEach((episode) => {
    const series = episodeSeriesName(episode);
    if (!groups.has(series)) groups.set(series, []);
    groups.get(series).push(episode);
  });
  let html = "";
  groups.forEach((episodes, series) => {
    html += "<div class=\"series-group\"><h3 class=\"series-title\">" + escapeHTML(series) + "</h3>";
    html += episodes.map((episode) => {
      const warnings = episode.warnings || [];
      const rows = episodeDisplayRows(episode).map((row) => {
        if (row.kind === "gap") {
          return "<li class=\"part-row gap-slot\">Part " + row.partNumber + " — missing</li>";
        }
        const part = row.part;
        const isFirst = (episode.parts || [])[0] === part;
        return "<li class=\"part-row\" draggable=\"true\" data-part-video=\"" + part.videoId +
          "\" data-episode-id=\"" + episode.id + "\"><span class=\"drag-handle\" title=\"Drag to resequence\">⋮⋮</span> Part " +
          part.partNumber + " (seq " + part.sequence + "): " + escapeHTML(part.title) +
          " <span class=\"pill\">" + escapeHTML(part.ytVideoId) + "</span> " + partStatusChip(part) +
          (isFirst ? "" : " <button data-split-episode=\"" + episode.id + "\" data-split-video=\"" + part.videoId + "\">Split here</button>") +
          "</li>";
      }).join("");
      const mergeChecked = state.episodeMergeSelected.has(episode.id) ? " checked" : "";
      return "<div class=\"card episode" + (warnings.length ? " needs-review" : "") + "\"><h3>" +
        "<label class=\"merge-pick\"><input type=\"checkbox\" class=\"episode-merge-check\" data-merge-episode=\"" + episode.id +
        "\" data-merge-channel=\"" + episode.channelId + "\"" + mergeChecked + "> Merge</label> " +
        escapeHTML(episode.title || episode.episodeKey) +
        " <span class=\"pill\">episode " + episode.episodeNumber + "</span>" +
        (warnings.length ? " <span class=\"warn-badge\">Needs review</span>" : "") + "</h3>" +
        (warnings.length ? "<ul class=\"warning-list\">" + warnings.map((warning) =>
          "<li>⚠ " + escapeHTML(warning) + "</li>").join("") + "</ul>" : "") +
        "<ul class=\"parts\" data-parts-for=\"" + episode.id + "\">" + rows + "</ul>" +
        "<button data-schedule=\"" + episode.id + "\">Schedule this series</button></div>";
    }).join("");
    html += "</div>";
  });
  list.innerHTML = html;
  updateMergeButton();
}

function updateMergeButton() {
  const count = state.episodeMergeSelected.size;
  const button = el("episode-merge-btn");
  button.textContent = "Merge selected (" + count + ")";
  button.disabled = count < 2;
}

async function loadEpisodes() {
  const channel = el("episode-channel").value;
  if (!channel) { setStatus("episode-status", "Channel ID is required", "error"); return; }
  setStatus("episode-status", "Loading...");
  try {
    state.episodes = await api("GET", "/api/episodes?channel=" + encodeURIComponent(channel));
    state.episodeMergeSelected.clear();
    renderEpisodeBoard();
    setStatus("episode-status", state.episodes.length + " episode(s)", "ok");
  } catch (err) {
    setStatus("episode-status", err.message, "error");
  }
}

el("episode-load-btn").addEventListener("click", loadEpisodes);

el("episode-list").addEventListener("click", async (event) => {
  const split = event.target.closest("[data-split-episode]");
  if (split) {
    try {
      setStatus("episode-status", "Splitting episode...");
      await api("POST", "/api/episodes/" + split.dataset.splitEpisode + "/split", {
        videoId: Number(split.dataset.splitVideo),
      });
      setStatus("episode-status", "Episode split", "ok");
      await loadEpisodes();
    } catch (err) {
      setStatus("episode-status", err.message, "error");
    }
    return;
  }
  const button = event.target.closest("[data-schedule]");
  if (!button) return;
  state.currentEpisodeId = button.dataset.schedule;
  el("schedule-form").classList.remove("hidden");
  el("schedule-episode-id").textContent = "#" + state.currentEpisodeId;
  setStatus("schedule-status", "");
  el("schedule-result").innerHTML = "";
  state.schedulePlan = null;
  renderSchedulePlan();
});

el("episode-list").addEventListener("change", (event) => {
  const box = event.target.closest(".episode-merge-check");
  if (!box) return;
  const id = Number(box.dataset.mergeEpisode);
  const channel = Number(box.dataset.mergeChannel);
  if (box.checked) {
    const selectedChannels = state.episodes
      .filter((episode) => state.episodeMergeSelected.has(episode.id))
      .map((episode) => episode.channelId);
    if (selectedChannels.length > 0 && selectedChannels[0] !== channel) {
      box.checked = false;
      setStatus("episode-status", "Merge only works for episodes on the same channel.", "error");
      return;
    }
    state.episodeMergeSelected.add(id);
  } else {
    state.episodeMergeSelected.delete(id);
  }
  updateMergeButton();
});

el("episode-merge-btn").addEventListener("click", async () => {
  const ids = Array.from(state.episodeMergeSelected);
  if (ids.length < 2) return;
  try {
    setStatus("episode-status", "Merging episodes...");
    await api("POST", "/api/episodes/merge", { episodeIds: ids });
    state.episodeMergeSelected.clear();
    setStatus("episode-status", "Episodes merged", "ok");
    await loadEpisodes();
    loadStatus();
  } catch (err) {
    setStatus("episode-status", err.message, "error");
  }
});

// Drag to resequence parts within one episode (item 42).
el("episode-list").addEventListener("dragstart", (event) => {
  const row = event.target.closest(".part-row[data-part-video]");
  if (!row) return;
  state.dragPart = { episodeId: row.dataset.episodeId, videoId: row.dataset.partVideo, element: row };
  row.classList.add("dragging");
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    try { event.dataTransfer.setData("text/plain", row.dataset.partVideo); } catch (err) { /* optional */ }
  }
});

el("episode-list").addEventListener("dragover", (event) => {
  if (!state.dragPart) return;
  const row = event.target.closest(".part-row[data-part-video]");
  if (!row || row.dataset.episodeId !== state.dragPart.episodeId) return;
  event.preventDefault();
  const rect = row.getBoundingClientRect();
  const after = (event.clientY - rect.top) > rect.height / 2;
  const list = row.parentElement;
  if (after) list.insertBefore(state.dragPart.element, row.nextSibling);
  else list.insertBefore(state.dragPart.element, row);
});

el("episode-list").addEventListener("drop", async (event) => {
  if (!state.dragPart) return;
  event.preventDefault();
  const dragged = state.dragPart;
  state.dragPart = null;
  const list = document.querySelector("[data-parts-for=\"" + dragged.episodeId + "\"]");
  if (!list) { await loadEpisodes(); return; }
  const videoIds = Array.from(list.querySelectorAll(".part-row[data-part-video]"))
    .map((row) => Number(row.dataset.partVideo));
  try {
    setStatus("episode-status", "Resequencing...");
    const updated = await api("POST", "/api/episodes/" + dragged.episodeId + "/resequence", { videoIds });
    const index = state.episodes.findIndex((episode) => String(episode.id) === String(dragged.episodeId));
    if (index !== -1) state.episodes[index] = updated;
    renderEpisodeBoard();
    setStatus("episode-status", "Parts resequenced", "ok");
  } catch (err) {
    setStatus("episode-status", err.message, "error");
    await loadEpisodes();
  }
});

el("episode-list").addEventListener("dragend", () => {
  state.dragPart = null;
  document.querySelectorAll(".part-row.dragging").forEach((row) => row.classList.remove("dragging"));
});

// Schedule-plan preview: real dates, what moves, same-day links and
// health warnings, before anything is applied (items 46-49).
let schedulePlanTimer = null;

function formatPlanDate(value) {
  if (!value) return "";
  const date = new Date(value);
  if (isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { weekday: "short", year: "numeric", month: "short", day: "numeric" }) +
    " " + date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function planDateKey(value) {
  const date = new Date(value);
  if (isNaN(date.getTime())) return String(value || "");
  return date.getFullYear() + "-" + String(date.getMonth() + 1).padStart(2, "0") + "-" + String(date.getDate()).padStart(2, "0");
}

function computeSchedulePlanWarnings(entries, cadenceDays) {
  const warnings = [];
  const byDate = new Map();
  entries.forEach((entry) => {
    const key = planDateKey(entry.publishAt);
    if (!byDate.has(key)) byDate.set(key, new Set());
    byDate.get(key).add(entry.episodeNumber);
    if (new Date(entry.publishAt).getTime() < Date.now()) {
      warnings.push("A planned publish date is in the past: " + formatPlanDate(entry.publishAt));
    }
  });
  byDate.forEach((episodeNumbers, key) => {
    if (episodeNumbers.size > 1) {
      warnings.push(episodeNumbers.size + " episodes land on the same day (" + key + ") on this channel");
    }
  });
  const dates = Array.from(byDate.keys()).sort();
  for (let i = 1; i < dates.length; i++) {
    const gap = Math.round((new Date(dates[i]).getTime() - new Date(dates[i - 1]).getTime()) / 86400000);
    if (cadenceDays > 0 && gap > cadenceDays) {
      warnings.push("Gap of " + gap + " days between " + dates[i - 1] + " and " + dates[i] + " exceeds the " + cadenceDays + "-day cadence");
    }
  }
  return Array.from(new Set(warnings));
}

function renderSchedulePlanHealth() {
  const box = el("schedule-plan-health");
  if (!box) return;
  if (!state.schedulePlanWarnings || state.schedulePlanWarnings.length === 0) { box.innerHTML = ""; return; }
  box.innerHTML = "<div class=\"warn-box\">Schedule preview warnings:<ul>" +
    state.schedulePlanWarnings.map((warning) => "<li>" + escapeHTML(warning) + "</li>").join("") + "</ul></div>";
}

function renderSchedulePlan() {
  const box = el("schedule-plan-preview");
  const entriesBox = el("schedule-plan-entries");
  const warningsBox = el("schedule-plan-warnings");
  if (!state.schedulePlan) {
    box.classList.remove("hidden");
    warningsBox.innerHTML = "";
    entriesBox.innerHTML = "<p class=\"muted-text\">Pick a cadence and a start date to preview the first 10 real publish dates here — nothing is applied yet.</p>";
    return;
  }
  box.classList.remove("hidden");
  const entries = state.schedulePlan.entries || [];
  state.schedulePlanWarnings = computeSchedulePlanWarnings(entries, Number(el("schedule-cadence").value || 0));
  warningsBox.innerHTML = state.schedulePlanWarnings.length
    ? "<ul class=\"warning-list\">" + state.schedulePlanWarnings.map((warning) =>
      "<li>⚠ " + escapeHTML(warning) + "</li>").join("") + "</ul>"
    : "";
  renderSchedulePlanHealth();
  const byDate = new Map();
  entries.slice(0, 10).forEach((entry) => {
    const key = planDateKey(entry.publishAt);
    if (!byDate.has(key)) byDate.set(key, []);
    byDate.get(key).push(entry);
  });
  let html = "";
  byDate.forEach((dayEntries, key) => {
    html += "<div class=\"plan-day\"><span class=\"date-chip\">" + escapeHTML(formatPlanDate(dayEntries[0].publishAt)) + "</span>" +
      dayEntries.map((entry) => {
        const moves = entry.currentPublishAt && entry.currentPublishAt !== entry.publishAt;
        return "<div class=\"plan-entry\">Episode " + entry.episodeNumber + " · Part " + entry.partNumber + " — " +
          escapeHTML(entry.title || entry.ytVideoId) +
          (entry.isPublic ? " <span class=\"pill\">public</span>" : "") +
          (moves ? " <span class=\"move-highlight\">" + escapeHTML(formatPlanDate(entry.currentPublishAt)) +
            " <span class=\"move-arrow\">→</span> " + escapeHTML(formatPlanDate(entry.publishAt)) + "</span>" : "") +
          "</div>";
      }).join("") + "</div>";
  });
  if (entries.length > 10) html += "<p class=\"muted-text\">…and " + (entries.length - 10) + " more entries in this plan.</p>";
  if (state.schedulePlan.skippedPublic) {
    html += "<p class=\"muted-text\">" + state.schedulePlan.skippedPublic + " public video(s) skipped.</p>";
  }
  entriesBox.innerHTML = html || "<p class=\"muted-text\">No entries in this plan.</p>";
}

async function loadSchedulePlan() {
  if (!state.currentEpisodeId) return;
  const local = el("schedule-starts").value;
  if (!local) { state.schedulePlan = null; renderSchedulePlan(); return; }
  try {
    const data = await api("POST", "/api/episodes/" + state.currentEpisodeId + "/schedule-plan", {
      cadenceDays: Number(el("schedule-cadence").value || 7),
      startsAt: new Date(local).toISOString(),
      includePublic: el("schedule-public").checked,
    });
    state.schedulePlan = data;
    state.schedulePlanEpisodeId = state.currentEpisodeId;
    renderSchedulePlan();
  } catch (err) {
    setStatus("schedule-status", err.message, "error");
  }
}

function queueSchedulePlan() {
  if (schedulePlanTimer) clearTimeout(schedulePlanTimer);
  schedulePlanTimer = setTimeout(loadSchedulePlan, 200);
}

el("schedule-cadence-choice").addEventListener("change", () => {
  const choice = el("schedule-cadence-choice").value;
  if (choice !== "custom") el("schedule-cadence").value = choice;
  else el("schedule-cadence").focus();
  queueSchedulePlan();
});
el("schedule-cadence").addEventListener("input", () => {
  const value = el("schedule-cadence").value;
  const preset = ["1", "7", "14", "21"].includes(value) ? value : "custom";
  el("schedule-cadence-choice").value = preset;
  queueSchedulePlan();
});
el("schedule-starts").addEventListener("change", queueSchedulePlan);
el("schedule-public").addEventListener("change", queueSchedulePlan);

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
    state.schedulePlan = null;
    el("schedule-plan-preview").classList.add("hidden");
    loadStatus();
  } catch (err) {
    setStatus("schedule-status", err.message, "error");
  }
});

// --------------------------------------------------------------- schedules
const CALENDAR_PALETTE = ["#4f8cff", "#2ea043", "#d29922", "#f778ba", "#a371f7", "#39c5cf", "#f85149", "#e3b341"];

function calendarChannelColor(channelId) {
  return CALENDAR_PALETTE[Math.abs(Number(channelId) || 0) % CALENDAR_PALETTE.length];
}

function renderCalendar() {
  const box = el("schedule-calendar");
  if (!state.calendarCursor) {
    const now = new Date();
    state.calendarCursor = new Date(now.getFullYear(), now.getMonth(), 1);
  }
  const cursor = state.calendarCursor;
  el("calendar-title").textContent = cursor.toLocaleDateString(undefined, { month: "long", year: "numeric" });
  const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
  const gridStart = new Date(first);
  gridStart.setDate(first.getDate() - first.getDay());
  const videoEvents = new Map();
  state.calendarVideos.forEach((video) => {
    if (!video.publishAt) return;
    const key = planDateKey(video.publishAt);
    if (!videoEvents.has(key)) videoEvents.set(key, []);
    videoEvents.get(key).push(video);
  });
  const ruleEvents = new Map();
  state.calendarRules.forEach((rule) => {
    if (!rule.startsAt) return;
    const key = planDateKey(rule.startsAt);
    if (!ruleEvents.has(key)) ruleEvents.set(key, []);
    ruleEvents.get(key).push(rule);
  });
  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  let html = "<div class=\"calendar-head\">" + weekdays.map((day) => "<div>" + day + "</div>").join("") + "</div><div class=\"calendar-grid\">";
  for (let i = 0; i < 42; i++) {
    const day = new Date(gridStart);
    day.setDate(gridStart.getDate() + i);
    const key = planDateKey(day.toISOString());
    const videos = videoEvents.get(key) || [];
    const rules = ruleEvents.get(key) || [];
    const outside = day.getMonth() !== cursor.getMonth();
    const conflict = videos.length > 1;
    html += "<div class=\"calendar-cell" + (outside ? " is-outside" : "") + (conflict ? " has-conflict" : "") + "\">" +
      "<div class=\"calendar-day\">" + day.getDate() + (conflict ? " <span class=\"conflict-badge\">" + videos.length + " overlap</span>" : "") + "</div>" +
      videos.slice(0, 3).map((video) =>
        "<div class=\"calendar-event\" style=\"border-left-color:" + calendarChannelColor(video.channelId) +
        ";background:" + calendarChannelColor(video.channelId) + "22\" title=\"" + escapeHTML(video.title) + " (" +
        escapeHTML(channelTitle(video.channelId) || ("channel " + video.channelId)) + ")\">" + escapeHTML(video.title) + "</div>").join("") +
      (videos.length > 3 ? "<div class=\"calendar-more\">+" + (videos.length - 3) + " more</div>" : "") +
      rules.map((rule) =>
        "<div class=\"calendar-event rule-marker\" style=\"border-left-color:" + calendarChannelColor(rule.channelId) +
        "\" title=\"Schedule rule #" + rule.id + " starts\">Rule #" + rule.id + " starts</div>").join("") +
      "</div>";
  }
  box.innerHTML = html + "</div>";
  renderSchedulePlanHealth();
}

el("calendar-prev-btn").addEventListener("click", () => {
  if (!state.calendarCursor) state.calendarCursor = new Date();
  state.calendarCursor = new Date(state.calendarCursor.getFullYear(), state.calendarCursor.getMonth() - 1, 1);
  renderCalendar();
});
el("calendar-next-btn").addEventListener("click", () => {
  if (!state.calendarCursor) state.calendarCursor = new Date();
  state.calendarCursor = new Date(state.calendarCursor.getFullYear(), state.calendarCursor.getMonth() + 1, 1);
  renderCalendar();
});

async function loadSchedules() {
  setStatus("schedules-status", "Loading...");
  try {
    const rules = await api("GET", "/api/schedules");
    state.calendarRules = rules;
    el("schedule-rows").innerHTML = rules.map((rule) =>
      "<tr><td>" + rule.id + "</td><td>" + escapeHTML(channelTitle(rule.channelId) || ("#" + rule.channelId)) + "</td>" +
      "<td>" + rule.cadenceDays + "</td><td>" + rule.videosPerEpisode + "</td>" +
      "<td>" + escapeHTML(rule.startsAt || "") + "</td><td>" + (rule.isActive ? "active" : "inactive") + "</td></tr>").join("");
    el("schedules-empty").innerHTML = rules.length === 0
      ? "<div class=\"empty-state\"><h3>No schedule rules yet</h3>" +
        "<p>A rule is created when you schedule an episode: it fixes the cadence (every day, week, 2 or 3 weeks) and the start date for a whole series.</p>" +
        "<p>Go to Episodes, load a channel, and use “Schedule this series”.</p></div>"
      : "";
    try {
      const data = await api("GET", "/api/videos?visibility=scheduled&limit=500");
      state.calendarVideos = data.videos || [];
    } catch (err) {
      state.calendarVideos = [];
    }
    renderCalendar();
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
  loadRecipes();
  updateBulkPreview();
  const parsed = parseHash();
  if (parsed) {
    if (parsed.section === "videos") applyVideoParams(parsed.params);
    navigate(parsed.section, true);
  } else {
    navigate(LS.get("lastSection", "home"), true);
  }
})();
