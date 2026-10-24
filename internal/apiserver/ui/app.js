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
  // Wave 3 (items 51-75)
  settingsData: null,
  accounts: null,
  historyKind: "",
  // Wave 4 (items 71-100)
  selectionAnchor: -1,
  paletteIndex: 0,
  paletteItems: [],
  playlistEdits: new Map(),
  // Wave 5 (items 101-125)
  inbox: [],
  rules: [],
  proposals: [],
  series: [],
  wizard: null,
  commentCentre: [],
  ccFilter: "all",
  ccRendered: {},
  channelDefaults: new Map(),
  // Wave 6 (items 126-150)
  playlists: [],
  playlistChannel: "",
  plMove: { sourceId: "", targetId: "", items: [] },
  ccCoverage: [],
  ccGapFilter: null,
  ccBulkRows: [],
  longForm: false,
  insightsBy: "views",
  orphanPlaylists: [],
};

const SECTIONS = {
  home: "Home",
  suggestions: "Suggestions",
  videos: "Videos",
  episodes: "Episodes",
  automation: "Automation",
  commentcentre: "Comment Centre",
  schedules: "Schedules",
  playlists: "Playlists",
  insights: "Insights",
  comments: "Comments",
  abtests: "A/B Tests",
  accounts: "Accounts",
  history: "History",
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
  let response;
  try {
    response = await fetch(path, options);
  } catch (netErr) {
    // Network-level failure: the local server is unreachable (item 67).
    showOfflineBanner();
    const err = new Error(netErr && netErr.message ? netErr.message : "Network error");
    err.isNetworkError = true;
    throw err;
  }
  if (currentBannerId === "offline") clearBanner();
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

// ------------------------------------------------- feedback (wave 3)
// Plain-language errors (item 65): the friendly text is the headline;
// the raw message stays available under "Details" in error toasts.
function friendlyError(err) {
  const raw = (err && err.message) || String(err || "Unknown error");
  const lower = raw.toLowerCase();
  if ((err && err.isNetworkError) || lower.indexOf("failed to fetch") !== -1 ||
      lower.indexOf("networkerror") !== -1 || lower.indexOf("load failed") !== -1) {
    return "Cannot reach the local yt-manage server — is `yt-manage serve` still running?";
  }
  if (lower.indexOf("plan") !== -1 && (lower.indexOf("not found") !== -1 || lower.indexOf("expired") !== -1)) {
    return "That edit plan expired — build the plan again.";
  }
  if (lower.indexOf("token") !== -1 || lower.indexOf("auth") !== -1 || lower.indexOf("401") !== -1 ||
      lower.indexOf("unauthori") !== -1 || lower.indexOf("credential") !== -1 || lower.indexOf("oauth") !== -1) {
    return "The account's sign-in has expired — reconnect it from Accounts.";
  }
  return raw;
}

// Toasts (item 61): successes state the result and leave on their own;
// errors persist until dismissed and carry Details + a copyable
// report (item 69).
function showToast(message, kind) {
  const stack = el("toast-stack");
  if (!stack) return null;
  const toast = document.createElement("div");
  toast.className = "toast " + (kind === "error" ? "toast-error" : "toast-ok");
  toast.innerHTML = "<span class=\"toast-text\">" + escapeHTML(message) + "</span>" +
    "<button class=\"toast-close\" title=\"Dismiss\">✕</button>";
  toast.querySelector(".toast-close").addEventListener("click", () => toast.remove());
  stack.appendChild(toast);
  if (kind !== "error") {
    setTimeout(() => {
      toast.classList.add("toast-out");
      setTimeout(() => toast.remove(), 350);
    }, 4000);
  }
  return toast;
}

function showErrorToast(err, where) {
  const raw = (err && err.message) || String(err || "Unknown error");
  const toast = showToast(friendlyError(err), "error");
  if (!toast) return;
  const extra = document.createElement("div");
  extra.className = "toast-extra";
  extra.innerHTML = "<div class=\"toast-actions\">" +
    "<button class=\"toast-details-btn\">Details</button>" +
    "<button class=\"toast-copy-btn\">Copy report</button></div>" +
    "<pre class=\"toast-details hidden\"></pre>";
  const details = extra.querySelector(".toast-details");
  details.textContent = raw;
  extra.querySelector(".toast-details-btn").addEventListener("click", () => {
    details.classList.toggle("hidden");
  });
  const copyBtn = extra.querySelector(".toast-copy-btn");
  copyBtn.addEventListener("click", async () => {
    const report = JSON.stringify({
      when: new Date().toISOString(),
      where: where || state.section,
      message: raw,
    }, null, 2);
    try {
      await navigator.clipboard.writeText(report);
      copyBtn.textContent = "Copied";
    } catch (copyErr) {
      details.textContent = report;
      details.classList.remove("hidden");
      copyBtn.textContent = "Report shown below";
    }
    setTimeout(() => { copyBtn.textContent = "Copy report"; }, 2000);
  });
  toast.appendChild(extra);
}

// Banners (item 67): one clear banner at a time, dismissible, with a
// single action. Used for offline, quota-exhausted and sync errors.
const dismissedBanners = new Set();
let currentBannerId = null;

function showBanner(id, text, actionLabel, actionFn) {
  if (dismissedBanners.has(id) || currentBannerId === id) return;
  const area = el("banner-area");
  if (!area) return;
  currentBannerId = id;
  area.innerHTML = "<div class=\"banner\" role=\"alert\">" +
    "<span class=\"banner-text\">" + escapeHTML(text) + "</span>" +
    (actionLabel ? "<button class=\"banner-action\">" + escapeHTML(actionLabel) + "</button>" : "") +
    "<button class=\"banner-close\" title=\"Dismiss\">✕</button></div>";
  const actionBtn = area.querySelector(".banner-action");
  if (actionBtn && actionFn) {
    actionBtn.addEventListener("click", () => { clearBanner(); actionFn(); });
  }
  area.querySelector(".banner-close").addEventListener("click", () => {
    dismissedBanners.add(id);
    clearBanner();
  });
}

function clearBanner() {
  currentBannerId = null;
  const area = el("banner-area");
  if (area) area.innerHTML = "";
}

function showOfflineBanner() {
  showBanner("offline",
    "Cannot reach the local yt-manage server — is `yt-manage serve` still running?",
    "Retry", () => { loadStatus(); });
}

// Long-task progress centre (item 64): a fixed corner panel listing
// the tasks still running; the rest of the UI stays usable.
const activeTasks = new Map();
let taskTimer = null;

function startTask(id, label) {
  activeTasks.set(id, { label, started: Date.now() });
  if (!taskTimer) taskTimer = setInterval(renderTasks, 500);
  renderTasks();
}

function endTask(id) {
  activeTasks.delete(id);
  if (activeTasks.size === 0 && taskTimer) {
    clearInterval(taskTimer);
    taskTimer = null;
  }
  renderTasks();
}

function renderTasks() {
  const panel = el("progress-centre");
  if (!panel) return;
  if (activeTasks.size === 0) {
    panel.classList.add("hidden");
    panel.innerHTML = "";
    return;
  }
  panel.classList.remove("hidden");
  panel.innerHTML = Array.from(activeTasks.values()).map((task) =>
    "<div class=\"task-row\"><span class=\"spinner\"></span>" +
    "<span class=\"task-label\">" + escapeHTML(task.label) + "</span>" +
    "<span class=\"task-elapsed\">" + formatElapsed(Date.now() - task.started) + "</span></div>").join("");
}

// Quota meter (item 66): header bar fed by /api/status and
// /api/settings, amber over 80%, red over 100%, banner when exhausted.
function renderQuota(source) {
  const meter = el("quota-meter");
  if (!meter || !source) return;
  const used = source.quotaUsed;
  const budget = source.quotaBudget;
  if (used === undefined || used === null || !budget) {
    meter.classList.add("hidden");
    return;
  }
  meter.classList.remove("hidden");
  const pct = Math.round((used / budget) * 100);
  el("quota-label").textContent = "Quota today: " + used + " / " + budget;
  const fill = el("quota-fill");
  fill.style.width = Math.min(100, pct) + "%";
  fill.className = "quota-fill" + (pct > 100 ? " is-over" : pct > 80 ? " is-warn" : "");
  if (used >= budget) {
    showBanner("quota",
      "Today's YouTube API quota is used up (" + used + " / " + budget + "). Syncs and edits will fail until the quota resets.",
      "Open Settings", () => navigate("settings"));
  }
}

// ------------------------------------------------- wave 4 helpers
// Read-only mode (item 95): /api/status is the source of truth.
function isReadOnly() {
  return !!(state.status && state.status.readOnly);
}

function renderReadOnly() {
  const on = isReadOnly();
  document.body.classList.toggle("read-only", on);
  el("readonly-banner").classList.toggle("hidden", !on);
}

function relativeTime(iso) {
  const then = new Date(iso).getTime();
  if (isNaN(then)) return "";
  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return minutes + (minutes === 1 ? " minute ago" : " minutes ago");
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return hours + (hours === 1 ? " hour ago" : " hours ago");
  const days = Math.floor(hours / 24);
  return days + (days === 1 ? " day ago" : " days ago");
}

// Visibility as a coloured dot + text label (item 83): never colour
// alone, and the existing text stays for screen readers.
const VISIBILITY_DOTS = {
  public: "vis-public",
  private: "vis-private",
  unlisted: "vis-unlisted",
  scheduled: "vis-scheduled",
  draft: "vis-draft",
};

function visibilityHTML(visibility) {
  const value = String(visibility || "");
  const cls = VISIBILITY_DOTS[value.toLowerCase()] || "vis-private";
  return "<span class=\"vis\"><span class=\"vis-dot " + cls + "\" aria-hidden=\"true\"></span>" + escapeHTML(value) + "</span>";
}

// Thumbnails & avatars (item 82): YouTube CDN thumbnails degrade to
// a letter placeholder when the image fails — never a broken icon.
function thumbnailHTML(video) {
  const letter = escapeHTML(String(video.title || "?").trim().charAt(0).toUpperCase() || "?");
  if (!video.ytVideoId) return "<span class=\"thumb\"><span class=\"thumb-fallback\">" + letter + "</span></span>";
  return "<span class=\"thumb\"><span class=\"thumb-fallback\">" + letter + "</span>" +
    "<img src=\"https://i.ytimg.com/vi/" + encodeURIComponent(video.ytVideoId) + "/mqdefault.jpg\" alt=\"\" loading=\"lazy\" onerror=\"this.remove()\"></span>";
}

function avatarHTML(label) {
  const letter = escapeHTML(String(label || "?").trim().charAt(0).toUpperCase() || "?");
  return "<span class=\"avatar\" aria-hidden=\"true\">" + letter + "</span>";
}

// Recent items quick-jump (item 75): last 8 opened videos and last
// 5 viewed episodes, in this browser only.
function recentVideos() { return LS.get("recentVideos", []); }
function recentEpisodes() { return LS.get("recentEpisodes", []); }

function recordRecentVideo(video) {
  if (!video || !video.id) return;
  const entry = { id: video.id, title: video.title || "", ytVideoId: video.ytVideoId || "" };
  const next = [entry].concat(recentVideos().filter((item) => item.id !== entry.id)).slice(0, 8);
  LS.set("recentVideos", next);
  renderRecents();
}

function recordRecentEpisode(episode) {
  if (!episode || !episode.id) return;
  const entry = {
    id: episode.id,
    title: episode.title || episode.episodeKey || ("Episode " + episode.id),
    channelId: episode.channelId || "",
  };
  const next = [entry].concat(recentEpisodes().filter((item) => item.id !== entry.id)).slice(0, 5);
  LS.set("recentEpisodes", next);
  renderRecents();
}

function renderRecents() {
  const card = el("recent-card");
  if (!card) return;
  const videos = recentVideos();
  const episodes = recentEpisodes();
  if (videos.length === 0 && episodes.length === 0) { card.classList.add("hidden"); return; }
  card.classList.remove("hidden");
  let html = "";
  if (videos.length > 0) {
    html += "<div class=\"recent-group\"><span class=\"recent-heading\">Videos</span>" +
      videos.map((video) =>
        "<button class=\"recent-item\" data-recent-video=\"" + video.id + "\">" +
        thumbnailHTML(video) + "<span>" + escapeHTML(video.title || ("Video " + video.id)) + "</span></button>").join("") + "</div>";
  }
  if (episodes.length > 0) {
    html += "<div class=\"recent-group\"><span class=\"recent-heading\">Episodes</span>" +
      episodes.map((episode) =>
        "<button class=\"recent-item\" data-recent-episode=\"" + episode.id + "\" data-channel=\"" + escapeHTML(episode.channelId) + "\">" +
        "<span class=\"avatar\" aria-hidden=\"true\">▦</span><span>" + escapeHTML(episode.title) + "</span></button>").join("") + "</div>";
  }
  el("recent-list").innerHTML = html;
}

el("recent-list").addEventListener("click", async (event) => {
  const videoBtn = event.target.closest("[data-recent-video]");
  const episodeBtn = event.target.closest("[data-recent-episode]");
  if (videoBtn) {
    const id = videoBtn.dataset.recentVideo;
    const stub = recentVideos().find((item) => String(item.id) === String(id));
    navigate("videos");
    try {
      openDrawer(await api("GET", "/api/videos/" + id));
    } catch (err) {
      if (stub) openDrawer(stub);
    }
    return;
  }
  if (episodeBtn) {
    if (episodeBtn.dataset.channel) el("episode-channel").value = episodeBtn.dataset.channel;
    navigate("episodes");
    loadEpisodes();
  }
});

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

// Themes (item 81): System / Light / Dark, applied instantly via a
// data-theme attribute on <html> and CSS variables. System follows
// prefers-color-scheme and reacts when the OS theme changes.
const systemThemeQuery = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

function themePreference() {
  const value = LS.get("theme", "system");
  return value === "light" || value === "dark" ? value : "system";
}

function effectiveTheme() {
  const pref = themePreference();
  if (pref !== "system") return pref;
  return systemThemeQuery && systemThemeQuery.matches ? "dark" : "light";
}

function applyTheme() {
  document.documentElement.dataset.theme = effectiveTheme();
  const select = el("set-theme");
  if (select) select.value = themePreference();
}

function cycleTheme() {
  const order = ["system", "light", "dark"];
  const next = order[(order.indexOf(themePreference()) + 1) % order.length];
  LS.set("theme", next);
  applyTheme();
  showToast("Theme: " + next, "ok");
}

el("set-theme").addEventListener("change", () => {
  LS.set("theme", el("set-theme").value);
  applyTheme();
});
if (systemThemeQuery) {
  const onSystemThemeChange = () => { if (themePreference() === "system") applyTheme(); };
  if (systemThemeQuery.addEventListener) systemThemeQuery.addEventListener("change", onSystemThemeChange);
  else if (systemThemeQuery.addListener) systemThemeQuery.addListener(onSystemThemeChange);
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
  if (section === "home") loadHomeSuggestions();
  if (section === "suggestions") loadInbox();
  if (section === "videos" && !state.videosLoaded) loadVideos();
  if (section === "episodes") {
    loadSeries();
    if (el("episode-channel").value) loadEpisodes();
  }
  if (section === "automation") loadAutomation();
  if (section === "commentcentre") loadCommentCentre();
  if (section === "playlists" && el("playlist-channel").value) loadPlaylists();
  if (section === "insights") loadInsights();
  if (section === "schedules") loadSchedules();
  if (section === "comments") loadComments();
  if (section === "accounts") loadAccounts();
  if (section === "history") loadHistory();
  if (section === "settings") loadSettings();
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
  renderQuota(status);
  renderReadOnly();
  // Footer shows the real version from the binary + data dir (item 99).
  el("footer-version").textContent = "yt-manage" + (status.appVersion ? " v" + status.appVersion : "");
  el("footer-datadir").textContent = status.dataDir || "";
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
  fillChannelSelects();
  if (state.status && state.status.autoSync) renderAutoSyncCard();
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
  // Sync mirrors comments too; say when it last happened (item 97).
  const commentsLine = el("checklist-comments-line");
  if (status.commentsLastSyncAt) {
    commentsLine.textContent = "Comments last synced " + relativeTime(status.commentsLastSyncAt) + ".";
    commentsLine.classList.remove("hidden");
  } else {
    commentsLine.classList.add("hidden");
  }
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
    showToast("Demo channel loaded: " + data.videos + " videos, " + data.episodes + " episodes, " + data.playlists + " playlists", "ok");
    state.videosLoaded = false;
    await loadStatus();
  } catch (err) {
    setStatus("demo-status", friendlyError(err), "error");
    showErrorToast(err, "home");
  }
});

el("demo-clear-btn").addEventListener("click", async () => {
  if (!window.confirm("Remove all demo data? This deletes only the demo channel and its sample content — your real accounts and channels are not touched.")) return;
  setStatus("demo-status", "Removing demo data...");
  try {
    await api("POST", "/api/demo/clear", {});
    setStatus("demo-status", "Demo data removed.", "ok");
    showToast("Demo data removed", "ok");
    state.videosLoaded = false;
    await loadStatus();
    if (state.section === "videos") loadVideos();
  } catch (err) {
    setStatus("demo-status", friendlyError(err), "error");
    showErrorToast(err, "home");
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
    html += "<table class=\"grid sync-table\"><thead><tr><th>Account</th><th>Channels</th><th>Videos</th><th>Playlists</th><th>Comments</th><th>API units</th></tr></thead><tbody>" +
      rows.map((row) =>
        "<tr><td>" + escapeHTML(pickField(row, ["label", "account", "accountLabel", "accountId", "id"]) || "Account") + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["channels", "channelCount", "channel"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["videos", "videoCount"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["playlists", "playlistCount"])) + "</td>" +
        "<td>" + escapeHTML(pickField(row, ["comments", "commentCount"])) + "</td>" +
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
  startTask("sync", "Syncing accounts");
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
    if (hasErrors) {
      showBanner("sync-errors",
        "Sync finished with errors for some accounts — nothing was changed for the accounts that failed.",
        "Open Accounts", () => navigate("accounts"));
    } else {
      showToast("Sync complete in " + formatElapsed(Date.now() - started), "ok");
    }
    state.videosLoaded = false;
    await loadStatus();
    if (state.section === "accounts") loadAccounts();
  } catch (err) {
    if (progress) { progress.textContent = friendlyError(err); progress.className = "status-line error"; }
    if (resultId === "sync-result") setStatus("account-status", friendlyError(err), "error");
    showErrorToast(err, "accounts");
  } finally {
    clearInterval(timer);
    state.syncing = false;
    endTask("sync");
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
  if (state.shorts) state.longForm = false;
  el("chip-shorts").setAttribute("aria-pressed", String(state.shorts));
  el("chip-has-comments").setAttribute("aria-pressed", String(state.hasComments));
  renderLengthChips();
  if (state.section === "videos") updateHash();
  if (reload) loadVideos();
}

// Length views (item 139): All / Shorts (≤60s) / Long-form (≥61s).
// Shorts uses the server filter (Wave 1); long-form is its complement,
// filtered here from durationSeconds, which videoDTO carries.
function lengthViewName() {
  if (state.shorts) return "shorts";
  if (state.longForm) return "long";
  return "all";
}

function isShortVideo(video) {
  if (video.isShort) return true;
  return (video.durationSeconds || 0) > 0 && video.durationSeconds <= 60;
}

function displayVideos() {
  if (state.longForm && !state.shorts) {
    return state.videos.filter((video) => (video.durationSeconds || 0) >= 61);
  }
  return state.videos;
}

function renderLengthChips() {
  const view = lengthViewName();
  document.querySelectorAll("#length-chips .chip[data-length]").forEach((chip) =>
    chip.classList.toggle("is-active", chip.dataset.length === view));
  const all = state.videos.length;
  const shorts = state.videos.filter(isShortVideo).length;
  const long = state.videos.filter((video) => (video.durationSeconds || 0) >= 61).length;
  el("length-count-all").textContent = "(" + all + ")";
  el("length-count-shorts").textContent = "(" + shorts + ")";
  el("length-count-long").textContent = "(" + long + ")";
}

el("length-chips").addEventListener("click", (event) => {
  const chip = event.target.closest("[data-length]");
  if (!chip) return;
  const view = chip.dataset.length;
  if (view === "shorts") {
    state.longForm = false;
    setToggleChips(true, state.hasComments, true);
  } else if (view === "long") {
    state.longForm = true;
    if (state.shorts) setToggleChips(false, state.hasComments, true);
    else { renderLengthChips(); renderVideos(); renderLongFormEmpty(); }
  } else {
    state.longForm = false;
    if (state.shorts) setToggleChips(false, state.hasComments, true);
    else { renderLengthChips(); renderVideos(); }
  }
});

function renderLongFormEmpty() {
  const box = el("video-empty");
  if (!state.longForm || displayVideos().length > 0 || state.videos.length === 0) return;
  box.innerHTML = "<span class=\"state-art\" aria-hidden=\"true\">▷</span><h3>No long-form videos in these results</h3>" +
    "<p>None of the " + state.videos.length + " loaded video(s) runs 61 seconds or longer.</p>" +
    "<div class=\"row\"><button id=\"empty-length-all\" class=\"primary\">Show all lengths</button></div>";
  box.classList.remove("hidden");
  el("empty-length-all").addEventListener("click", () => {
    state.longForm = false;
    renderLengthChips();
    renderVideos();
    box.classList.add("hidden");
  });
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
    // Selection survives a reload of the same list: keep only the
    // ids still present (item 73).
    const present = new Set(state.videos.map((video) => video.id));
    state.selected.forEach((id) => { if (!present.has(id)) state.selected.delete(id); });
    state.activeVideoIndex = -1;
    state.videosLoaded = true;
    if (vs.q) recordRecentSearch(vs.q);
    renderVideos();
    updateBulkPreview();
    renderVideoEmpty(data.count || 0, vs);
    renderLongFormEmpty();
    setStatus("video-status", data.count + " video(s)", "ok");
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
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
    box.innerHTML = "<span class=\"state-art\" aria-hidden=\"true\">▷</span><h3>No videos yet</h3>" +
      "<p>Your local mirror is empty. Sync a connected account to mirror its videos here — after that, search is instant and costs no YouTube API quota.</p>" +
      "<div class=\"row\"><button id=\"empty-sync-btn\" class=\"primary\">Sync now</button>" +
      "<button id=\"empty-demo-btn\">Load demo channel</button></div>";
  } else {
    const filters = activeFilters(vs);
    box.innerHTML = "<span class=\"state-art\" aria-hidden=\"true\">⌕</span><h3>No videos match</h3>" +
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
  const display = displayVideos();
  const activeVideo = state.videos[state.activeVideoIndex];
  tbody.innerHTML = display.map((video) => {
    const checked = state.selected.has(video.id) ? " checked" : "";
    const rowCls = [];
    if (activeVideo && video.id === activeVideo.id) rowCls.push("keyboard-active");
    if (state.selected.has(video.id)) rowCls.push("selected");
    const snippet = descriptionSnippet(video.description, q);
    return "<tr data-id=\"" + video.id + "\"" + (rowCls.length ? " class=\"" + rowCls.join(" ") + "\"" : "") + ">" +
      "<td><input type=\"checkbox\" class=\"video-check\" data-id=\"" + video.id + "\" aria-label=\"Select video " + video.id + "\"" + checked + "></td>" +
      "<td>" + video.id + "</td>" +
      "<td><span class=\"title-cell\">" + thumbnailHTML(video) + "<span>" + highlightTitle(video.title) + "</span></span>" +
      (snippet ? "<div class=\"snippet\">" + snippet + "</div>" : "") + "</td>" +
      "<td>" + visibilityHTML(video.visibility) + "</td>" +
      "<td>" + video.viewCount + "</td>" +
      "<td>" + (video.commentCount || 0) + "</td>" +
      "<td>" + escapeHTML(video.publishedAt || "") + "</td></tr>";
  }).join("");
  tbody.classList.add("refreshing");
  // Tables announce their row counts to screen readers (item 86).
  el("video-table").setAttribute("aria-label", "Videos, " + display.length + " row" + (display.length === 1 ? "" : "s"));
  el("video-select-all").checked = state.videos.length > 0 && state.selected.size === state.videos.length;
  renderSelectionBar();
  renderLengthChips();
}

// File-manager style multi-select (item 73): shift-click selects a
// range, ctrl/cmd-click toggles one row, and the bulk action bar
// follows the selection (item 74).
function setVideoSelectedAt(index, on) {
  const video = state.videos[index];
  if (!video) return;
  if (on) state.selected.add(video.id); else state.selected.delete(video.id);
  const row = el("video-rows").children[index];
  if (row) {
    row.classList.toggle("selected", on);
    const box = row.querySelector(".video-check");
    if (box) box.checked = on;
  }
}

function selectVideoRange(from, to, on) {
  const lo = Math.min(from, to);
  const hi = Math.max(from, to);
  for (let i = lo; i <= hi; i++) setVideoSelectedAt(i, on);
}

function syncSelectionUI() {
  el("video-select-all").checked = state.videos.length > 0 && state.selected.size === state.videos.length;
  renderSelectionBar();
}

function renderSelectionBar() {
  const bar = el("selection-bar");
  if (!bar) return;
  const count = state.selected.size;
  bar.classList.toggle("hidden", count === 0);
  el("selection-count").textContent = count + (count === 1 ? " video selected" : " videos selected");
}

el("sel-clear-btn").addEventListener("click", () => {
  state.selected.clear();
  state.selectionAnchor = -1;
  renderVideos();
});

// "Rename selected": build the plan from the current filter (the
// only narrowing the plan API takes), then keep only the selected
// ids included — applies already honour the include set via
// onlyIds, so nothing outside the selection can be written.
el("sel-rename-btn").addEventListener("click", async () => {
  const wanted = new Set(state.selected);
  const built = await buildBulkPlan();
  if (!built) return;
  let kept = 0;
  state.planIncluded.forEach((id) => { if (!wanted.has(id)) state.planIncluded.delete(id); else kept++; });
  renderBulkPlan();
  el("bulk-plan").scrollIntoView({ block: "nearest" });
  setStatus("video-status", kept > 0
    ? "Plan ready: " + kept + " of your selected videos have changes — review, then apply."
    : "None of the selected videos would change with the current template/regex — adjust the pattern above.", kept > 0 ? "ok" : "error");
});

el("video-rows").addEventListener("change", (event) => {
  const box = event.target.closest(".video-check");
  if (!box) return;
  const row = box.closest("tr[data-id]");
  const index = state.videos.findIndex((v) => v.id === Number(box.dataset.id));
  if (box.checked) state.selected.add(Number(box.dataset.id)); else state.selected.delete(Number(box.dataset.id));
  if (row) row.classList.toggle("selected", box.checked);
  // Shift+click on a checkbox extends the anchor's state over the
  // whole range; the click's modifier was captured before change.
  if (state.pendingCheckShift && state.selectionAnchor >= 0 && index >= 0 && state.selectionAnchor !== index) {
    selectVideoRange(state.selectionAnchor, index, box.checked);
  } else if (index >= 0) {
    state.selectionAnchor = index;
  }
  state.pendingCheckShift = false;
  syncSelectionUI();
});

el("video-rows").addEventListener("click", (event) => {
  const box = event.target.closest(".video-check");
  if (box) {
    state.pendingCheckShift = !!event.shiftKey;
    return;
  }
  const row = event.target.closest("tr[data-id]");
  if (!row) return;
  const index = state.videos.findIndex((v) => v.id === Number(row.dataset.id));
  if (index === -1) return;
  if (event.ctrlKey || event.metaKey) {
    setVideoSelectedAt(index, !state.selected.has(state.videos[index].id));
    state.selectionAnchor = index;
    syncSelectionUI();
    return;
  }
  if (event.shiftKey && state.selectionAnchor >= 0) {
    selectVideoRange(state.selectionAnchor, index, true);
    syncSelectionUI();
    return;
  }
  const video = state.videos[index];
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
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
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
      setStatus("video-status", friendlyError(err), "error");
    }
  }
});

// Detail drawer (item 18)
function openDrawer(video) {
  recordRecentVideo(video);
  el("edit-video-id").textContent = "#" + video.id + " (" + video.ytVideoId + ")";
  el("drawer-visibility").innerHTML = visibilityHTML(video.visibility);
  const thumbImg = el("drawer-thumb-img");
  el("drawer-thumb-fallback").textContent = String(video.title || "?").trim().charAt(0).toUpperCase() || "?";
  if (video.ytVideoId) {
    thumbImg.src = "https://i.ytimg.com/vi/" + encodeURIComponent(video.ytVideoId) + "/mqdefault.jpg";
    thumbImg.classList.remove("hidden");
    thumbImg.onerror = () => thumbImg.classList.add("hidden");
  } else {
    thumbImg.classList.add("hidden");
  }
  el("drawer-views").textContent = String(video.viewCount || 0);
  el("drawer-comments").textContent = String(video.commentCount || 0);
  el("drawer-published").textContent = video.publishedAt || "";
  el("drawer-yt-link").href = "https://www.youtube.com/watch?v=" + encodeURIComponent(video.ytVideoId || "");
  el("edit-title").value = video.title || "";
  el("edit-description").value = video.description || "";
  el("video-edit-form").dataset.videoId = String(video.id);
  setStatus("edit-status", "");
  loadVideoHistory(video.id);
  loadTopComments(video);
  el("video-drawer").classList.add("open");
  el("video-drawer").setAttribute("aria-hidden", "false");
  el("drawer-backdrop").classList.remove("hidden");
}

// Rename history in the drawer (item 38).
async function loadVideoHistory(videoId) {
  const box = el("drawer-history");
  el("drawer-undo-btn").classList.add("hidden");
  box.innerHTML = "<p class=\"muted-text\">Loading history…</p>";
  try {
    const entries = await api("GET", "/api/videos/" + videoId + "/history");
    if (el("video-edit-form").dataset.videoId !== String(videoId)) return;
    if (!entries || entries.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No recorded renames yet</p>";
      return;
    }
    el("drawer-undo-btn").classList.remove("hidden");
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

// Top comments in the drawer (item 128): the mirrored comments,
// most liked first. Read-only — replies happen in YouTube Studio.
async function loadTopComments(video) {
  const box = el("drawer-top-comments");
  if (!box || !video || !video.id) return;
  box.innerHTML = "<p class=\"muted-text\">Loading comments…</p>";
  try {
    const comments = await api("GET", "/api/videos/" + video.id + "/comments");
    if (el("video-edit-form").dataset.videoId !== String(video.id)) return;
    const watchURL = "https://www.youtube.com/watch?v=" + encodeURIComponent(video.ytVideoId || "");
    const note = "<p class=\"muted-text\">Replies happen in YouTube Studio — this list is the read-only mirror. " +
      "<a href=\"" + watchURL + "\" target=\"_blank\" rel=\"noopener\">Open the video on YouTube</a></p>";
    if (!comments || comments.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No comments mirrored for this video yet — they arrive with a sync.</p>" + note;
      return;
    }
    box.innerHTML = comments.map((comment) =>
      "<div class=\"history-entry\"><div class=\"history-change\">" + escapeHTML(comment.text || "") + "</div>" +
      "<div class=\"muted-text\">" + escapeHTML(comment.author || "Unknown author") + " · ♥ " + (comment.likeCount || 0) +
      (comment.isFirst ? " · <span class=\"pill\">first comment</span>" : "") + "</div></div>").join("") + note;
  } catch (err) {
    if (el("video-edit-form").dataset.videoId === String(video.id)) {
      box.innerHTML = "<p class=\"muted-text\">Comments could not be loaded.</p>";
    }
  }
}

// Undo the most recent rename for the drawer's video (item 62).
el("drawer-undo-btn").addEventListener("click", async () => {
  const id = el("video-edit-form").dataset.videoId;
  if (!id) return;
  const button = el("drawer-undo-btn");
  button.disabled = true;
  try {
    const updated = await api("POST", "/api/videos/" + id + "/undo-title");
    el("edit-title").value = updated.title || "";
    showToast("Title restored to “" + (updated.title || "") + "”", "ok");
    setStatus("edit-status", "Rename undone: " + (updated.title || ""), "ok");
    await loadVideos();
    loadVideoHistory(Number(id));
  } catch (err) {
    setStatus("edit-status", friendlyError(err), "error");
    showErrorToast(err, "videos");
  } finally {
    button.disabled = false;
  }
});

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
    setStatus("edit-status", friendlyError(err), "error");
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
    showToast("Recipe “" + name + "” saved", "ok");
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
  }
});

el("recipe-delete-btn").addEventListener("click", async () => {
  const id = el("recipe-select").value;
  if (!id) { setStatus("video-status", "Select a recipe to delete.", "error"); return; }
  try {
    state.recipes = await api("DELETE", "/api/rename-recipes/" + id);
    renderRecipes("");
    setStatus("video-status", "Recipe deleted", "ok");
    showToast("Recipe deleted", "ok");
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
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
  el("bulk-scope").textContent = bulkScopeText();
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

async function buildBulkPlan() {
  setStatus("video-status", "Building plan...");
  const vs = currentVideoState();
  // Smart defaults (item 108): an empty template falls back to the
  // scoped channel's remembered rename template.
  if (vs.channel && !el("bulk-template").value) {
    const defaults = await fetchChannelDefaults(vs.channel);
    if (defaults && defaults.renameTemplate) {
      el("bulk-template").value = defaults.renameTemplate;
      updateBulkPreview();
    }
  }
  const body = {
    q: vs.q,
    visibility: vs.visibility,
    pattern: currentPatternFields(),
  };
  if (vs.channel) body.channelId = Number(vs.channel);
  try {
    const data = await api("POST", "/api/videos/bulk-plan", body);
    await ensureAccountsLoaded();
    state.bulkPlan = data;
    state.planIncluded = new Set((data.changes || []).map((change) => change.videoId));
    state.planApplied = new Set();
    state.planResults = new Map();
    state.planFailedIds = [];
    state.planKept = true;
    el("bulk-plan").classList.remove("hidden");
    hidePreflight("bulk-preflight");
    renderBulkPlan();
    setStatus("video-status", (data.changes || []).length + " change(s) planned", "ok");
    return true;
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
    showErrorToast(err, "videos");
    return false;
  }
}

el("bulk-plan-btn").addEventListener("click", buildBulkPlan);

// ---------------------------------------------------- trust helpers
// Scope statements (item 92) and pre-flight checks (item 93).
async function ensureAccountsLoaded() {
  if (state.accounts) return;
  try { state.accounts = await api("GET", "/api/accounts"); } catch (err) { state.accounts = []; }
}

function accountLabelForChannel(channelId) {
  const accounts = state.accounts || [];
  for (const account of accounts) {
    if ((account.channels || []).some((channel) => String(channel.id) === String(channelId))) {
      return account.label || ("Account " + account.id);
    }
  }
  return "";
}

function scopeSuffix(channelId) {
  if (!channelId) return "";
  const title = channelTitle(channelId) || ("channel " + channelId);
  const account = accountLabelForChannel(channelId);
  return " on " + title + (account ? " (" + account + ")" : "");
}

function bulkScopeText() {
  if (!state.bulkPlan) return "";
  const byChannel = new Map();
  (state.bulkPlan.changes || []).forEach((change) => {
    const video = state.videos.find((item) => item.id === change.videoId);
    const channelId = video ? video.channelId : (change.channelId || 0);
    byChannel.set(channelId, (byChannel.get(channelId) || 0) + 1);
  });
  const parts = [];
  byChannel.forEach((count, channelId) => {
    parts.push(count + (count === 1 ? " video" : " videos") + (channelId ? scopeSuffix(channelId) : " on other channels"));
  });
  return parts.length ? "This will change " + parts.join("; ") + "." : "";
}

function renderPreflight(boxId, result, onAnyway) {
  const box = el(boxId);
  if (!box) return;
  box.classList.remove("hidden");
  box.innerHTML = "<div class=\"preflight-title\">Pre-flight checks</div>" +
    (result.checks || []).map((check) =>
      "<div class=\"preflight-row " + (check.ok ? "is-ok" : "is-fail") + "\">" +
      "<span class=\"preflight-mark\">" + (check.ok ? "✓" : "✗") + "</span>" +
      "<span class=\"preflight-name\">" + escapeHTML(check.name) + "</span>" +
      "<span class=\"preflight-msg\">" + escapeHTML(check.message || "") + "</span></div>").join("") +
    (onAnyway ? "<div class=\"row\"><button class=\"preflight-anyway\" data-write>Apply anyway</button></div>" : "");
  const button = box.querySelector(".preflight-anyway");
  if (button && onAnyway) {
    button.addEventListener("click", () => { hidePreflight(boxId); onAnyway(); });
  }
}

function hidePreflight(boxId) {
  const box = el(boxId);
  if (box) { box.classList.add("hidden"); box.innerHTML = ""; }
}

// Bulk applies run the checks first; a failure needs an explicit
// "Apply anyway" — and a missing plan never gets one (item 93).
async function applyWithPreflight(ids, label) {
  if (!state.bulkPlan || ids.length === 0) return;
  let result;
  try {
    result = await api("POST", "/api/preflight", { kind: "bulk", planId: state.bulkPlan.planId });
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
    showErrorToast(err, "videos");
    return;
  }
  if (result.ok) {
    renderPreflight("bulk-preflight", result, null);
    await applyPlanIds(ids, label);
    return;
  }
  renderPreflight("bulk-preflight", result, () => applyPlanIds(ids, label));
  setStatus("video-status", "Pre-flight checks failed — review them below, or use Apply anyway.", "error");
}

async function applyPlanIds(ids, label) {
  if (!state.bulkPlan || ids.length === 0) return;
  setStatus("video-status", label + "...");
  startTask("bulk", label);
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
    if (appliedNow > 0 || failedNow > 0) {
      showToast("Renamed " + appliedNow + " video(s)" + (failedNow ? ", " + failedNow + " failed" : ""), failedNow ? "error" : "ok");
    }
    await loadVideos();
    loadStatus();
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
    showErrorToast(err, "videos");
    updatePlanLabels();
  } finally {
    endTask("bulk");
  }
}

el("bulk-apply-btn").addEventListener("click", () => applyWithPreflight(planIncludedIds(), "Applying included rows"));
el("bulk-apply-first-btn").addEventListener("click", () => applyWithPreflight(planIncludedIds().slice(0, 5), "Applying first 5"));
el("bulk-apply-all-btn").addEventListener("click", () =>
  applyWithPreflight(planPendingChanges().map((change) => change.videoId), "Applying all changes"));
el("bulk-retry-btn").addEventListener("click", () =>
  applyWithPreflight(state.planFailedIds.slice(), "Retrying failed rows"));

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
    "<button class=\"saved-del\" data-saved-del=\"" + saved.id + "\" title=\"Delete saved search\" aria-label=\"Delete saved search\" data-write>✕</button></div>").join("");
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
    showToast("Saved search “" + name + "”", "ok");
  } catch (err) {
    setStatus("video-status", friendlyError(err), "error");
  }
});

el("saved-search-list").addEventListener("click", async (event) => {
  const del = event.target.closest("[data-saved-del]");
  const apply = event.target.closest("[data-saved]");
  if (del) {
    try {
      state.savedSearches = await api("DELETE", "/api/saved-searches/" + del.dataset.savedDel);
      await renderSavedSearches();
      showToast("Saved search deleted", "ok");
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
  return "<div class=\"empty-state\"><span class=\"state-art\" aria-hidden=\"true\">▦</span><h3>No episodes yet</h3>" +
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
          (isFirst ? "" : " <button data-split-episode=\"" + episode.id + "\" data-split-video=\"" + part.videoId + "\" data-write>Split here</button>") +
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
    setStatus("episode-status", friendlyError(err), "error");
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
      setStatus("episode-status", friendlyError(err), "error");
    }
    return;
  }
  const button = event.target.closest("[data-schedule]");
  if (!button) return;
  state.currentEpisodeId = button.dataset.schedule;
  const episode = state.episodes.find((item) => String(item.id) === String(state.currentEpisodeId));
  if (episode) recordRecentEpisode(episode);
  // Smart defaults (item 108): pre-fill the cadence from the
  // channel's remembered default when it has one.
  if (episode) {
    fetchChannelDefaults(episode.channelId).then((defaults) => {
      if (!defaults || !(defaults.cadenceDays > 0)) return;
      if (String(state.currentEpisodeId) !== String(episode.id)) return;
      el("schedule-cadence").value = defaults.cadenceDays;
      el("schedule-cadence-choice").value =
        ["1", "7", "14", "21"].indexOf(String(defaults.cadenceDays)) !== -1 ? String(defaults.cadenceDays) : "custom";
      queueSchedulePlan();
    });
  }
  el("schedule-form").classList.remove("hidden");
  el("schedule-episode-id").textContent = "#" + state.currentEpisodeId;
  setStatus("schedule-status", "");
  el("schedule-result").innerHTML = "";
  el("schedule-scope").textContent = "";
  hidePreflight("schedule-preflight");
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
    setStatus("episode-status", friendlyError(err), "error");
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
    setStatus("episode-status", friendlyError(err), "error");
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
  renderScheduleScope();
}

// Scope statement for the schedule apply (item 92).
function renderScheduleScope() {
  const node = el("schedule-scope");
  if (!node) return;
  const entries = state.schedulePlan && state.schedulePlanEpisodeId === state.currentEpisodeId
    ? (state.schedulePlan.entries || []) : [];
  if (entries.length === 0) { node.textContent = ""; return; }
  const episode = state.episodes.find((item) => String(item.id) === String(state.currentEpisodeId));
  const suffix = episode ? scopeSuffix(episode.channelId) : "";
  node.textContent = "This will change " + entries.length + (entries.length === 1 ? " video" : " videos") + suffix + ".";
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
    await ensureAccountsLoaded();
    renderSchedulePlan();
  } catch (err) {
    setStatus("schedule-status", friendlyError(err), "error");
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

async function doApplySchedule() {
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
    showToast("Schedule applied: " + data.applied + " video(s) scheduled" +
      (data.skippedPublic ? ", " + data.skippedPublic + " public skipped" : ""), "ok");
    el("schedule-result").innerHTML = (data.entries || []).map((entry) =>
      "<li>" + escapeHTML(entry.ytVideoId) + " → " + escapeHTML(entry.publishAt) + "</li>").join("");
    state.schedulePlan = null;
    el("schedule-plan-preview").classList.add("hidden");
    el("schedule-scope").textContent = "";
    hidePreflight("schedule-preflight");
    loadStatus();
  } catch (err) {
    setStatus("schedule-status", friendlyError(err), "error");
  }
}

// Schedule applies run pre-flight first (item 93): token, quota and
// publish-time conflicts from the previewed plan entries.
el("schedule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const local = el("schedule-starts").value;
  if (!local) { setStatus("schedule-status", "Starts-at is required", "error"); return; }
  const episode = state.episodes.find((item) => String(item.id) === String(state.currentEpisodeId));
  if (!episode) { doApplySchedule(); return; }
  const entries = state.schedulePlan && state.schedulePlanEpisodeId === state.currentEpisodeId
    ? (state.schedulePlan.entries || []).map((entry) => ({ videoId: entry.videoId, publishAt: entry.publishAt }))
    : [];
  let result;
  try {
    result = await api("POST", "/api/preflight", { kind: "schedule", channelId: episode.channelId, entries });
  } catch (err) {
    setStatus("schedule-status", friendlyError(err), "error");
    showErrorToast(err, "episodes");
    return;
  }
  if (result.ok) {
    renderPreflight("schedule-preflight", result, null);
    doApplySchedule();
    return;
  }
  renderPreflight("schedule-preflight", result, () => doApplySchedule());
  setStatus("schedule-status", "Pre-flight checks failed — review them above, or use Apply anyway.", "error");
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
    setStatus("schedules-status", friendlyError(err), "error");
  }
}

el("schedules-load-btn").addEventListener("click", loadSchedules);

// --------------------------------------------------------------- playlists
// Playlist board (item 131): cards with counts, privacy and
// last-updated, staged title/description edits (item 91, kept), a
// detail area per playlist (checklist 138, gaps 133, reorder and
// privacy 136), and the two-pane move board below.
function playlistById(id) {
  return (state.playlists || []).find((playlist) => String(playlist.id) === String(id));
}

function playlistPrivacyPill(playlist) {
  // playlistDTO currently carries no privacy field (verified in
  // dto.go: the model has Privacy, the DTO does not expose it), so
  // show the pill only when a value is actually present.
  if (playlist.privacy) {
    return " <span class=\"pill\" data-pl-privacy-pill=\"" + playlist.id + "\">" + escapeHTML(playlist.privacy) + "</span>";
  }
  return " <span class=\"pill\" data-pl-privacy-pill=\"" + playlist.id + "\">privacy not recorded</span>";
}

function playlistSeriesKey(title) {
  // The series key passed to the gaps endpoint: the playlist
  // title's first two words, lowercased (the show key shape).
  return String(title || "").toLowerCase().split(/\s+/).filter(Boolean).slice(0, 2).join(" ");
}

async function loadPlaylists() {
  const channel = el("playlist-channel").value;
  if (!channel) { setStatus("playlist-status", "Channel ID is required", "error"); return; }
  setStatus("playlist-status", "Loading...");
  try {
    const playlists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(channel));
    state.playlists = playlists;
    state.playlistChannel = channel;
    state.playlistEdits = new Map(playlists.map((playlist) => [String(playlist.id), playlist]));
    el("playlist-list").innerHTML = playlists.map((playlist) =>
      "<div class=\"card playlist\"><h3>" + escapeHTML(playlist.title) +
      " <span class=\"pill\">" + playlist.itemCount + " items</span>" + playlistPrivacyPill(playlist) +
      (playlist.updatedAt ? " <span class=\"pill\">updated " + escapeHTML(relativeTime(playlist.updatedAt) || playlist.updatedAt) + "</span>" : "") + "</h3>" +
      "<p>" + escapeHTML(playlist.description || "") + "</p>" +
      "<div class=\"inline-form\"><input data-pl-title=\"" + playlist.id + "\" value=\"" + escapeHTML(playlist.title) + "\" aria-label=\"Playlist title\">" +
      "<input data-pl-desc=\"" + playlist.id + "\" value=\"" + escapeHTML(playlist.description || "") + "\" aria-label=\"Playlist description\">" +
      "<button data-pl-review=\"" + playlist.id + "\" data-write>Review changes</button></div>" +
      "<div class=\"pl-review hidden\" id=\"pl-review-" + playlist.id + "\"></div>" +
      "<div class=\"row pl-actions\">" +
      "<button data-pl-checklist=\"" + playlist.id + "\">Publish checklist</button>" +
      "<button data-pl-gaps=\"" + playlist.id + "\">Find gaps</button>" +
      "<button data-pl-reorder-open=\"" + playlist.id + "\">Reorder</button>" +
      "<span class=\"pl-privacy-edit\"><select data-pl-privacy-select=\"" + playlist.id + "\" aria-label=\"Playlist privacy\">" +
      "<option value=\"\">Set privacy…</option><option value=\"private\">private</option>" +
      "<option value=\"unlisted\">unlisted</option><option value=\"public\">public</option></select>" +
      "<button data-pl-privacy-apply=\"" + playlist.id + "\" data-write>Apply privacy</button></span></div>" +
      "<div class=\"pl-detail hidden\" id=\"pl-checklist-" + playlist.id + "\"></div>" +
      "<div class=\"pl-detail hidden\" id=\"pl-gaps-" + playlist.id + "\"></div>" +
      "<div class=\"pl-detail hidden\" id=\"pl-reorder-" + playlist.id + "\">" +
      "<div class=\"preview-label\">Reorder — preview first</div>" +
      "<p class=\"muted-text\">Reordering rebuilds the playlist through the YouTube API: every item is removed and re-inserted in the new order, at roughly 50 quota units per item touched.</p>" +
      "<div class=\"row\"><label>Order by <select data-pl-reorder-mode=\"" + playlist.id + "\">" +
      "<option value=\"episode\">Episode number</option><option value=\"date\">Publish date</option></select></label>" +
      "<button data-pl-reorder-preview=\"" + playlist.id + "\">Preview order</button>" +
      "<button class=\"primary hidden\" data-pl-reorder-apply=\"" + playlist.id + "\" data-write>Apply reorder</button></div>" +
      "<div data-pl-reorder-table=\"" + playlist.id + "\"></div></div>" +
      "</div>").join("") ||
      "<div class=\"empty-state\"><h3>No playlists for this channel</h3>" +
      "<p>Playlists you create in YouTube Studio appear here after a sync — or create one with the form below and it is made on YouTube right away.</p></div>";
    renderBoardSelects();
    setStatus("playlist-status", playlists.length + " playlist(s)", "ok");
  } catch (err) {
    setStatus("playlist-status", friendlyError(err), "error");
  }
}

el("playlist-load-btn").addEventListener("click", loadPlaylists);

// ---------------------------------------------------- playlist board
// Two-pane move view (item 131). The source pane lists the source
// playlist's mirrored items — enumerated via the reorder-preview
// endpoint, the one API that returns a playlist's items with
// titles. Click-to-move is the reliable path; drag-and-drop onto
// the target pane is the bonus path.
function renderBoardSelects() {
  const playlists = state.playlists || [];
  const options = playlists.map((playlist) =>
    "<option value=\"" + playlist.id + "\">" + escapeHTML(playlist.title) + " (" + playlist.itemCount + ")</option>").join("");
  const source = el("pl-move-source");
  const target = el("pl-move-target");
  source.innerHTML = "<option value=\"\">Choose a source playlist…</option>" + options;
  target.innerHTML = "<option value=\"\">Choose a target playlist…</option>" + options;
  if (state.plMove.sourceId && playlistById(state.plMove.sourceId)) source.value = state.plMove.sourceId;
  if (state.plMove.targetId && playlistById(state.plMove.targetId)) target.value = state.plMove.targetId;
  if (!state.plMove.targetId && playlists.length > 1) {
    state.plMove.targetId = String(playlists[1].id);
    target.value = state.plMove.targetId;
  }
  updateDropLabel();
  if (state.plMove.sourceId) loadBoardItems();
  else el("pl-move-items").innerHTML = "<p class=\"muted-text\">Choose a source playlist to list its videos here.</p>";
}

function updateDropLabel() {
  const target = playlistById(state.plMove.targetId);
  el("pl-move-drop-label").textContent = target ? "Target: " + target.title : "Choose a target playlist first.";
}

el("pl-move-source").addEventListener("change", () => {
  state.plMove.sourceId = el("pl-move-source").value;
  loadBoardItems();
});
el("pl-move-target").addEventListener("change", () => {
  state.plMove.targetId = el("pl-move-target").value;
  updateDropLabel();
  renderBoardItems();
});

async function loadBoardItems() {
  const box = el("pl-move-items");
  const sourceId = state.plMove.sourceId;
  if (!sourceId) {
    box.innerHTML = "<p class=\"muted-text\">Choose a source playlist to list its videos here.</p>";
    return;
  }
  box.innerHTML = "<p class=\"muted-text\">Loading the playlist's videos…</p>";
  try {
    const rows = await api("POST", "/api/playlists/" + sourceId + "/reorder-preview", { mode: "date" });
    state.plMove.items = (Array.isArray(rows) ? rows : []).slice()
      .sort((a, b) => (a.currentPos || 0) - (b.currentPos || 0));
    renderBoardItems();
  } catch (err) {
    box.innerHTML = "<p class=\"muted-text\">This playlist's items could not be listed (" + escapeHTML(friendlyError(err)) + ").</p>";
  }
}

function renderBoardItems() {
  const box = el("pl-move-items");
  if (!box || !state.plMove.sourceId) return;
  const items = state.plMove.items || [];
  const others = (state.playlists || []).filter((playlist) => String(playlist.id) !== String(state.plMove.sourceId));
  if (items.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No videos in this playlist (in the local mirror).</p>";
    return;
  }
  box.innerHTML = items.map((item) =>
    "<div class=\"board-item\" draggable=\"true\" data-drag-video=\"" + item.videoId + "\">" +
    "<span class=\"drag-handle\" title=\"Drag to the target pane\">⋮⋮</span>" +
    "<span class=\"board-item-title\">" + escapeHTML(item.title || ("Video " + item.videoId)) + "</span>" +
    "<select data-move-target-for=\"" + item.videoId + "\" aria-label=\"Move target\">" +
    others.map((playlist) =>
      "<option value=\"" + playlist.id + "\"" + (String(playlist.id) === String(state.plMove.targetId) ? " selected" : "") + ">" +
      escapeHTML(playlist.title) + "</option>").join("") + "</select>" +
    "<button data-pl-move=\"" + item.videoId + "\" data-write>Move</button></div>").join("");
}

async function movePlaylistVideo(videoId) {
  const sourceId = state.plMove.sourceId;
  const select = document.querySelector("[data-move-target-for=\"" + videoId + "\"]");
  const targetId = select ? select.value : state.plMove.targetId;
  if (!sourceId || !targetId) { showToast("Pick a source and a target playlist first.", "error"); return; }
  if (String(targetId) === String(sourceId)) { showToast("Source and target are the same playlist.", "error"); return; }
  const item = (state.plMove.items || []).find((entry) => String(entry.videoId) === String(videoId));
  const label = item ? item.title : ("Video " + videoId);
  setStatus("pl-move-status", "Moving “" + label + "”…");
  try {
    await api("DELETE", "/api/playlists/" + sourceId + "/items/" + videoId);
  } catch (err) {
    setStatus("pl-move-status", "Could not remove it from the source playlist — nothing was moved.", "error");
    showErrorToast(err, "playlists");
    return;
  }
  try {
    await api("POST", "/api/playlists/" + targetId + "/items", { videoId: Number(videoId) });
  } catch (err) {
    setStatus("pl-move-status", "Removed from the source, but adding to the target failed — the video is now in neither playlist.", "error");
    showErrorToast(err, "playlists");
    await loadBoardItems();
    return;
  }
  setStatus("pl-move-status", "Moved “" + label + "”.", "ok");
  showToast("Moved “" + label + "” to " + (playlistById(targetId) || {}).title, "ok");
  state.plMove.items = (state.plMove.items || []).filter((entry) => String(entry.videoId) !== String(videoId));
  renderBoardItems();
  loadPlaylists();
}

el("pl-move-items").addEventListener("click", (event) => {
  const button = event.target.closest("[data-pl-move]");
  if (button) movePlaylistVideo(button.dataset.plMove);
});
el("pl-move-items").addEventListener("dragstart", (event) => {
  const item = event.target.closest("[data-drag-video]");
  if (!item || !event.dataTransfer) return;
  event.dataTransfer.effectAllowed = "move";
  try { event.dataTransfer.setData("text/plain", item.dataset.dragVideo); } catch (err) { /* optional */ }
  state.dragVideoId = item.dataset.dragVideo;
});
el("pl-move-drop").addEventListener("dragover", (event) => {
  event.preventDefault();
  el("pl-move-drop").classList.add("is-over");
});
el("pl-move-drop").addEventListener("dragleave", () => el("pl-move-drop").classList.remove("is-over"));
el("pl-move-drop").addEventListener("drop", (event) => {
  event.preventDefault();
  el("pl-move-drop").classList.remove("is-over");
  let videoId = state.dragVideoId;
  if (event.dataTransfer) {
    try { videoId = event.dataTransfer.getData("text/plain") || videoId; } catch (err) { /* keep state copy */ }
  }
  state.dragVideoId = null;
  if (videoId) {
    // A drop always targets the board's target playlist.
    const select = document.querySelector("[data-move-target-for=\"" + videoId + "\"]");
    if (select && state.plMove.targetId) select.value = state.plMove.targetId;
    movePlaylistVideo(videoId);
  }
});

// Duplicate detector (item 134): findings only — removal happens
// through the board above, never automatically.
el("playlist-dup-btn").addEventListener("click", async () => {
  const box = el("playlist-dup-results");
  box.classList.remove("hidden");
  box.innerHTML = "<h2>Duplicate check</h2><p class=\"muted-text\">Checking every playlist in the mirror…</p>";
  try {
    const findings = await api("GET", "/api/playlists/duplicates");
    const list = Array.isArray(findings) ? findings : [];
    box.innerHTML = "<div class=\"card-head\"><h2>Duplicate check</h2><button id=\"playlist-dup-close\">Close</button></div>" +
      (list.length === 0
        ? "<p class=\"result-ok\">No duplicates found — no video appears twice in a playlist, and no two playlists overlap by 80% or more ✓</p>"
        : list.map((finding) =>
          "<div class=\"dup-row\"><span class=\"pill\">" + escapeHTML(finding.kind || "duplicate") + "</span> " +
          escapeHTML(finding.detail || "") +
          (finding.playlistId ? " <span class=\"muted-text\">(playlist #" + finding.playlistId + ")</span>" : "") + "</div>").join("")) +
      (list.length ? "<p class=\"muted-text\">Nothing was changed. Remove a copy through the playlist board above.</p>" : "");
    el("playlist-dup-close").addEventListener("click", () => box.classList.add("hidden"));
  } catch (err) {
    box.innerHTML = "<h2>Duplicate check</h2><p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
});

// Orphan videos (item 137): public videos in no playlist, grouped
// by likely series, with bulk add per group.
el("orphans-load-btn").addEventListener("click", async () => {
  const channelId = el("orphan-channel").value;
  const box = el("orphans-list");
  if (!channelId) { setStatus("orphans-status", "Choose a channel first.", "error"); return; }
  setStatus("orphans-status", "Finding orphans…");
  try {
    try {
      state.orphanPlaylists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(channelId));
    } catch (err) { state.orphanPlaylists = []; }
    const groups = await api("GET", "/api/orphans?channelId=" + encodeURIComponent(channelId));
    const list = Array.isArray(groups) ? groups : [];
    setStatus("orphans-status", list.length + " orphan group(s)", "ok");
    if (list.length === 0) {
      box.innerHTML = "<p class=\"result-ok\">No orphans — every public video on this channel is in a playlist ✓</p>";
      return;
    }
    box.innerHTML = list.map((group, index) =>
      "<div class=\"orphan-group\"><div class=\"orphan-head\"><strong>" +
      escapeHTML(group.seriesKey || "(no series detected)") + "</strong>" +
      " <span class=\"pill\">" + group.count + " video(s)</span></div>" +
      "<div class=\"row\"><select data-orphan-target=\"" + index + "\">" +
      (state.orphanPlaylists || []).map((playlist) =>
        "<option value=\"" + playlist.id + "\">" + escapeHTML(playlist.title) + "</option>").join("") + "</select>" +
      "<button data-orphan-add=\"" + index + "\" data-write>Add all to playlist</button>" +
      (group.seriesKey ? "<button data-orphan-series=\"" + index + "\" data-write>Create/update series playlist</button>" : "") +
      "</div><div class=\"status-line\" id=\"orphan-status-" + index + "\"></div></div>").join("");
    box.dataset.channel = channelId;
    box.dataset.groups = JSON.stringify(list);
  } catch (err) {
    setStatus("orphans-status", friendlyError(err), "error");
    showErrorToast(err, "playlists");
  }
});

el("orphans-list").addEventListener("click", async (event) => {
  const box = el("orphans-list");
  const channelId = box.dataset.channel;
  let groups = [];
  try { groups = JSON.parse(box.dataset.groups || "[]"); } catch (err) { groups = []; }
  const seriesBtn = event.target.closest("[data-orphan-series]");
  if (seriesBtn) {
    const group = groups[Number(seriesBtn.dataset.orphanSeries)];
    if (!group) return;
    try {
      const data = await api("POST", "/api/series/playlist", { channelId: Number(channelId), seriesKey: group.seriesKey });
      showToast("Playlist “" + (data.playlistTitle || group.seriesKey) + "”: added " + data.added + " of " + data.total, "ok");
      el("orphans-load-btn").click();
    } catch (err) { showErrorToast(err, "playlists"); }
    return;
  }
  const addBtn = event.target.closest("[data-orphan-add]");
  if (!addBtn) return;
  const index = Number(addBtn.dataset.orphanAdd);
  const group = groups[index];
  const select = document.querySelector("[data-orphan-target=\"" + index + "\"]");
  if (!group || !select || !select.value) { setStatus("orphan-status-" + index, "Choose a target playlist first.", "error"); return; }
  addBtn.disabled = true;
  let added = 0;
  const failures = [];
  for (const videoId of group.videoIds || []) {
    try {
      await api("POST", "/api/playlists/" + select.value + "/items", { videoId: Number(videoId) });
      added++;
    } catch (err) { failures.push(videoId); }
    setStatus("orphan-status-" + index, "Added " + added + " of " + (group.videoIds || []).length + "…");
  }
  setStatus("orphan-status-" + index,
    "Added " + added + " video(s)" + (failures.length ? " — failed: " + failures.join(", ") : ""), failures.length ? "error" : "ok");
  showToast("Orphans: added " + added + " video(s) to the playlist", failures.length ? "error" : "ok");
  addBtn.disabled = false;
});

// Playlist edits are staged (item 91): edit → review the diff →
// apply or discard. Nothing reaches YouTube before Apply.
function playlistFieldValues(id) {
  return {
    title: document.querySelector("[data-pl-title=\"" + id + "\"]").value,
    description: document.querySelector("[data-pl-desc=\"" + id + "\"]").value,
  };
}

function reviewPlaylist(id) {
  const original = state.playlistEdits.get(String(id));
  const box = el("pl-review-" + id);
  if (!original || !box) return;
  const next = playlistFieldValues(id);
  const rows = [];
  if (next.title !== (original.title || "")) {
    rows.push("<div class=\"diff-row\"><span class=\"diff-label\">Title</span><span class=\"diff-old\">" +
      escapeHTML(original.title || "") + "</span><span class=\"move-arrow\">→</span><span class=\"diff-new\">" +
      escapeHTML(next.title) + "</span></div>");
  }
  if (next.description !== (original.description || "")) {
    rows.push("<div class=\"diff-row\"><span class=\"diff-label\">Description</span><span class=\"diff-old\">" +
      escapeHTML(original.description || "") + "</span><span class=\"move-arrow\">→</span><span class=\"diff-new\">" +
      escapeHTML(next.description) + "</span></div>");
  }
  if (rows.length === 0) {
    box.classList.add("hidden");
    box.innerHTML = "";
    setStatus("playlist-status", "No changes to review — the fields match what is saved.", "ok");
    return;
  }
  box.innerHTML = "<div class=\"preview-label\">Review — not applied yet</div>" + rows.join("") +
    "<div class=\"row\"><button class=\"primary\" data-pl-apply=\"" + id + "\" data-write>Apply changes</button>" +
    "<button data-pl-discard=\"" + id + "\">Discard</button></div>";
  box.classList.remove("hidden");
}

el("playlist-list").addEventListener("click", async (event) => {
  // Publish checklist (item 138).
  const checklistBtn = event.target.closest("[data-pl-checklist]");
  if (checklistBtn) {
    const id = checklistBtn.dataset.plChecklist;
    const box = el("pl-checklist-" + id);
    box.classList.remove("hidden");
    box.innerHTML = "<p class=\"muted-text\">Checking…</p>";
    try {
      const checks = await api("GET", "/api/playlists/" + id + "/checklist");
      box.innerHTML = "<div class=\"preview-label\">Publish checklist</div>" + (checks || []).map((check) =>
        "<div class=\"checklist-line " + (check.ok ? "is-ok" : "is-fail") + "\">" +
        "<span class=\"preflight-mark\">" + (check.ok ? "✓" : "✗") + "</span> " +
        "<strong>" + escapeHTML(check.name || "") + "</strong> <span class=\"muted-text\">" +
        escapeHTML(check.message || "") + "</span></div>").join("");
    } catch (err) {
      box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
    }
    return;
  }
  // Gaps finder (item 133).
  const gapsBtn = event.target.closest("[data-pl-gaps]");
  if (gapsBtn) {
    const id = gapsBtn.dataset.plGaps;
    const playlist = playlistById(id);
    const box = el("pl-gaps-" + id);
    box.classList.remove("hidden");
    box.innerHTML = "<p class=\"muted-text\">Looking for gaps…</p>";
    try {
      const seriesKey = playlistSeriesKey(playlist ? playlist.title : "");
      const data = await api("GET", "/api/playlists/" + id + "/gaps?series=" + encodeURIComponent(seriesKey));
      const titleOf = (videoId) => {
        const known = (state.videos || []).find((video) => String(video.id) === String(videoId));
        return known ? known.title : ("Video #" + videoId);
      };
      box.innerHTML = "<div class=\"preview-label\">Gaps for series “" + escapeHTML(data.seriesKey || seriesKey) + "”</div>" +
        (data.missingCount === 0 && data.extraCount === 0
          ? "<p class=\"result-ok\">No gaps — the playlist matches the series ✓</p>"
          : "<p>Missing from the playlist (" + data.missingCount + "):</p>" +
            ((data.missingVideoIds || []).length
              ? "<ul class=\"parts\">" + data.missingVideoIds.map((videoId) => "<li>" + escapeHTML(titleOf(videoId)) + "</li>").join("") + "</ul>"
              : "<p class=\"muted-text\">None.</p>") +
            (data.missingCount > 0
              ? "<button class=\"primary\" data-pl-gaps-add=\"" + id + "\" data-series=\"" + escapeHTML(data.seriesKey || seriesKey) + "\" data-write>Add all missing</button>" : "") +
            "<p>Extra in the playlist, not in the series (" + data.extraCount + "):</p>" +
            ((data.extraVideoIds || []).length
              ? "<ul class=\"parts\">" + data.extraVideoIds.map((videoId) => "<li>" + escapeHTML(titleOf(videoId)) + "</li>").join("") + "</ul>"
              : "<p class=\"muted-text\">None.</p>")) +
        "<div class=\"status-line\" id=\"pl-gaps-status-" + id + "\"></div>";
    } catch (err) {
      box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
    }
    return;
  }
  const gapsAdd = event.target.closest("[data-pl-gaps-add]");
  if (gapsAdd) {
    const id = gapsAdd.dataset.plGapsAdd;
    const playlist = playlistById(id);
    if (!playlist) return;
    gapsAdd.disabled = true;
    try {
      const data = await api("POST", "/api/series/playlist", {
        channelId: playlist.channelId,
        seriesKey: gapsAdd.dataset.series,
        playlistId: Number(id),
      });
      setStatus("pl-gaps-status-" + id, "Added " + data.added + " of " + data.total + " video(s) to “" + (data.playlistTitle || playlist.title) + "”.", "ok");
      showToast("Gaps repaired: added " + data.added + " video(s)", "ok");
    } catch (err) {
      setStatus("pl-gaps-status-" + id, friendlyError(err), "error");
      showErrorToast(err, "playlists");
      gapsAdd.disabled = false;
    }
    return;
  }
  // Reorder (item 136): preview, then apply.
  const reorderOpen = event.target.closest("[data-pl-reorder-open]");
  if (reorderOpen) {
    const box = el("pl-reorder-" + reorderOpen.dataset.plReorderOpen);
    if (box) box.classList.toggle("hidden");
    return;
  }
  const reorderPreview = event.target.closest("[data-pl-reorder-preview]");
  if (reorderPreview) {
    const id = reorderPreview.dataset.plReorderPreview;
    const mode = document.querySelector("[data-pl-reorder-mode=\"" + id + "\"]").value;
    const tableBox = document.querySelector("[data-pl-reorder-table=\"" + id + "\"]");
    tableBox.innerHTML = "<p class=\"muted-text\">Building the preview…</p>";
    try {
      const rows = await api("POST", "/api/playlists/" + id + "/reorder-preview", { mode });
      const list = Array.isArray(rows) ? rows : [];
      tableBox.innerHTML = list.length === 0
        ? "<p class=\"muted-text\">No items in this playlist (in the local mirror).</p>"
        : "<table class=\"grid\"><thead><tr><th>Video</th><th>Current position</th><th>Proposed position</th></tr></thead><tbody>" +
          list.map((row) =>
            "<tr" + (row.currentPos !== row.proposedPos ? " class=\"plan-excluded\"" : "") + "><td>" +
            escapeHTML(row.title || ("Video " + row.videoId)) + "</td><td>" + (row.currentPos + 1) +
            "</td><td>" + (row.proposedPos + 1) + "</td></tr>").join("") + "</tbody></table>";
      const applyBtn = document.querySelector("[data-pl-reorder-apply=\"" + id + "\"]");
      applyBtn.classList.toggle("hidden", list.length === 0);
      applyBtn.dataset.mode = mode;
    } catch (err) {
      tableBox.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
    }
    return;
  }
  const reorderApply = event.target.closest("[data-pl-reorder-apply]");
  if (reorderApply) {
    const id = reorderApply.dataset.plReorderApply;
    const mode = reorderApply.dataset.mode ||
      document.querySelector("[data-pl-reorder-mode=\"" + id + "\"]").value;
    reorderApply.disabled = true;
    try {
      const data = await api("POST", "/api/playlists/" + id + "/reorder", { mode });
      showToast("Playlist reordered: " + (data.reordered || 0) + " item(s) now in " + mode + " order", "ok");
      setStatus("playlist-status", "Reorder applied (" + (data.reordered || 0) + " item(s))", "ok");
    } catch (err) {
      showErrorToast(err, "playlists");
      reorderApply.disabled = false;
    }
    return;
  }
  // Privacy (item 136).
  const privacyApply = event.target.closest("[data-pl-privacy-apply]");
  if (privacyApply) {
    const id = privacyApply.dataset.plPrivacyApply;
    const select = document.querySelector("[data-pl-privacy-select=\"" + id + "\"]");
    if (!select || !select.value) { setStatus("playlist-status", "Choose a privacy level first.", "error"); return; }
    try {
      const data = await api("POST", "/api/playlists/" + id + "/privacy", { privacy: select.value });
      const playlist = playlistById(id);
      if (playlist) playlist.privacy = data.privacy || select.value;
      const pill = document.querySelector("[data-pl-privacy-pill=\"" + id + "\"]");
      if (pill) pill.textContent = data.privacy || select.value;
      showToast("Playlist privacy set to " + (data.privacy || select.value), "ok");
      setStatus("playlist-status", "Privacy updated", "ok");
    } catch (err) {
      setStatus("playlist-status", friendlyError(err), "error");
      showErrorToast(err, "playlists");
    }
    return;
  }
  const review = event.target.closest("[data-pl-review]");
  if (review) { reviewPlaylist(review.dataset.plReview); return; }
  const discard = event.target.closest("[data-pl-discard]");
  if (discard) {
    const id = discard.dataset.plDiscard;
    const original = state.playlistEdits.get(String(id));
    if (original) {
      document.querySelector("[data-pl-title=\"" + id + "\"]").value = original.title || "";
      document.querySelector("[data-pl-desc=\"" + id + "\"]").value = original.description || "";
    }
    const box = el("pl-review-" + id);
    if (box) { box.classList.add("hidden"); box.innerHTML = ""; }
    setStatus("playlist-status", "Changes discarded — nothing was sent to YouTube.", "ok");
    return;
  }
  const apply = event.target.closest("[data-pl-apply]");
  if (!apply) return;
  const id = apply.dataset.plApply;
  setStatus("playlist-status", "Saving...");
  try {
    await api("PATCH", "/api/playlists/" + id, playlistFieldValues(id));
    setStatus("playlist-status", "Saved", "ok");
    showToast("Playlist saved", "ok");
    await loadPlaylists();
  } catch (err) {
    setStatus("playlist-status", friendlyError(err), "error");
    showErrorToast(err, "playlists");
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
    setStatus("playlist-create-status", friendlyError(err), "error");
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
    setStatus("comments-status", friendlyError(err), "error");
  }
}

el("comments-load-btn").addEventListener("click", loadComments);

// ============================================================ wave 5
// Smart suggestions, automation and the comment centre
// (items 101-125).

// ------------------------------------------------- channel defaults
// Per-channel smart defaults (item 108), also carrying the
// pipeline chain (item 114) and watched series (item 115).
function normalizeDefaults(data) {
  const d = data || {};
  return {
    renameTemplate: d.renameTemplate || "",
    cadenceDays: d.cadenceDays || 0,
    commentTemplate: d.commentTemplate || "",
    pipelineComment: !!d.pipelineComment,
    pipelineEmail: !!d.pipelineEmail,
    pipelinePlaylistId: d.pipelinePlaylistId || 0,
    watchSeries: Array.isArray(d.watchSeries) ? d.watchSeries : [],
    playlistTitleTemplate: d.playlistTitleTemplate || "",
    playlistDescriptionTemplate: d.playlistDescriptionTemplate || "",
  };
}

async function fetchChannelDefaults(channelId, force) {
  if (!channelId) return null;
  const key = String(channelId);
  if (!force && state.channelDefaults.has(key)) return state.channelDefaults.get(key);
  try {
    const data = normalizeDefaults(await api("GET", "/api/channels/" + channelId + "/defaults"));
    state.channelDefaults.set(key, data);
    return data;
  } catch (err) {
    return null;
  }
}

function fillChannelSelects() {
  const channels = (state.status && state.status.channels) || [];
  const options = channels.map((channel) =>
    "<option value=\"" + channel.id + "\">" + escapeHTML(channel.title) + "</option>").join("");
  const cd = el("cd-channel");
  if (cd) {
    const current = cd.value;
    cd.innerHTML = "<option value=\"\">Choose a channel…</option>" + options;
    if (current) cd.value = current;
  }
  const ruleChannel = el("rule-match-channel");
  if (ruleChannel) {
    const current = ruleChannel.value;
    ruleChannel.innerHTML = "<option value=\"\">Any channel</option>" + options;
    if (current) ruleChannel.value = current;
  }
  // Wave 6 selects fed by the same channel list.
  const plain = [
    ["cc-bulk-channel", "Choose a channel…"],
    ["orphan-channel", "Choose a channel…"],
    ["tags-channel", "Choose a channel…"],
    ["insights-channel", "Choose a channel…"],
    ["goal-channel", "Choose a channel…"],
  ];
  plain.forEach(([id, placeholder]) => {
    const select = el(id);
    if (!select) return;
    const current = select.value;
    select.innerHTML = "<option value=\"\">" + placeholder + "</option>" + options;
    if (current) select.value = current;
  });
}

// ----------------------------------------------------- suggestions
const SUGGESTION_TYPES = {
  patterns: "Pattern",
  grouping: "Grouping",
  "title-health": "Title health",
  cadence: "Cadence",
  "dead-slots": "Dead slot",
  thumbnails: "Thumbnail check",
  anomalies: "Anomaly",
};

async function fetchSuggestions() {
  const data = await api("GET", "/api/suggestions");
  state.inbox = Array.isArray(data) ? data : (data.suggestions || []);
  return state.inbox;
}

function activeSuggestions() {
  return (state.inbox || []).filter((item) => (item.state || "active") === "active");
}

function suggestionChannelName(suggestion) {
  if (!suggestion.channelId) return "";
  return channelTitle(suggestion.channelId) || ("channel " + suggestion.channelId);
}

function suggestionPrimaryAction(suggestion) {
  switch (suggestion.type) {
    case "patterns": return { label: "Open in Videos", act: "videos" };
    case "title-health": return { label: "Fix in Bulk Edit", act: "videos" };
    case "grouping": return { label: "Group episodes", act: "group" };
    case "cadence": return { label: "Use this cadence", act: "cadence" };
    case "thumbnails": return { label: "Review thumbnails", act: "thumbs" };
    case "dead-slots": return { label: "Open calendar", act: "calendar" };
    case "anomalies": return { label: "Open History", act: "history" };
    default: return null;
  }
}

// Home preview (item 101): top 3 active suggestions with a Review
// jump into the full inbox below.
async function loadHomeSuggestions() {
  const card = el("home-suggestions-card");
  if (!card) return;
  try {
    await fetchSuggestions();
  } catch (err) {
    card.classList.add("hidden");
    return;
  }
  const active = activeSuggestions();
  setBadge("badge-suggestions", active.length);
  if (active.length === 0) { card.classList.add("hidden"); return; }
  card.classList.remove("hidden");
  const sorted = active.slice().sort((a, b) =>
    (a.type === "patterns" ? 0 : 1) - (b.type === "patterns" ? 0 : 1));
  el("home-suggestions-list").innerHTML = sorted.slice(0, 3).map((suggestion) =>
    "<div class=\"suggestion-row\">" +
    "<span class=\"type-badge\">" + escapeHTML(SUGGESTION_TYPES[suggestion.type] || suggestion.type || "Suggestion") + "</span>" +
    "<span class=\"suggestion-text\"><strong>" + escapeHTML(suggestion.title || "") + "</strong>" +
    (suggestion.detail ? " <span class=\"muted-text\">" + escapeHTML(suggestion.detail) + "</span>" : "") + "</span>" +
    "<button data-home-review=\"1\">Review</button></div>").join("");
}

el("home-suggestions-open").addEventListener("click", () => navigate("suggestions"));
el("home-suggestions-list").addEventListener("click", (event) => {
  if (event.target.closest("[data-home-review]")) navigate("suggestions");
});

// The inbox itself (items 102, 110): every suggestion with its
// type-specific action plus dismiss / snooze / reset.
async function loadInbox() {
  setStatus("inbox-status", "Loading suggestions...");
  try {
    await fetchSuggestions();
    renderInbox();
    const count = activeSuggestions().length;
    setBadge("badge-suggestions", count);
    setStatus("inbox-status", count + (count === 1 ? " active suggestion" : " active suggestions"), "ok");
  } catch (err) {
    setStatus("inbox-status", friendlyError(err), "error");
    showErrorToast(err, "suggestions");
  }
}

function renderInbox() {
  const box = el("inbox-list");
  const showHidden = el("inbox-show-hidden").checked;
  const items = (state.inbox || [])
    .map((suggestion, index) => ({ suggestion, index }))
    .filter(({ suggestion }) => showHidden || (suggestion.state || "active") === "active");
  if (items.length === 0) {
    box.innerHTML = "<div class=\"empty-state\"><span class=\"state-art\" aria-hidden=\"true\">✦</span><h3>Nothing to suggest right now</h3>" +
      "<p>When patterns, title problems or scheduling hints show up in your synced channels, they collect here.</p></div>";
    return;
  }
  box.innerHTML = items.map(({ suggestion, index }) => {
    const stateName = suggestion.state || "active";
    const primary = suggestionPrimaryAction(suggestion);
    const videoCount = (suggestion.videoIds || []).length;
    let buttons = "";
    if (primary) {
      buttons += "<button class=\"primary\" data-sugg-act=\"" + primary.act + "\" data-sugg-index=\"" + index + "\">" +
        escapeHTML(primary.label) + "</button>";
    }
    if (stateName === "active") {
      buttons += "<button data-sugg-dismiss=\"" + escapeHTML(suggestion.type) + "\">Dismiss</button>" +
        "<button data-sugg-snooze=\"" + escapeHTML(suggestion.type) + "\">Snooze 7 days</button>";
    } else {
      buttons += "<span class=\"pill\">" + escapeHTML(stateName) + "</span>" +
        "<button data-sugg-reset=\"" + escapeHTML(suggestion.type) + "\">Reset</button>";
    }
    return "<div class=\"card suggestion-card" + (stateName !== "active" ? " is-hidden-state" : "") + "\">" +
      "<div class=\"suggestion-head\">" +
      "<span class=\"type-badge\">" + escapeHTML(SUGGESTION_TYPES[suggestion.type] || suggestion.type || "Suggestion") + "</span>" +
      (suggestionChannelName(suggestion) ? "<span class=\"muted-text\">" + escapeHTML(suggestionChannelName(suggestion)) + "</span>" : "") +
      (videoCount ? "<span class=\"pill\">" + videoCount + (videoCount === 1 ? " video" : " videos") + "</span>" : "") +
      "</div>" +
      "<h3>" + escapeHTML(suggestion.title || "") + "</h3>" +
      (suggestion.detail ? "<p>" + escapeHTML(suggestion.detail) + "</p>" : "") +
      "<div class=\"row\">" + buttons + "</div></div>";
  }).join("");
}

el("inbox-load-btn").addEventListener("click", loadInbox);
el("inbox-show-hidden").addEventListener("change", renderInbox);

async function setSuggestionState(type, action, days) {
  try {
    const body = { type, action };
    if (days) body.days = days;
    await api("POST", "/api/suggestions/state", body);
    showToast(action === "dismiss" ? "Suggestion type dismissed"
      : action === "snooze" ? "Snoozed for " + days + " days" : "Suggestion type reset", "ok");
    await loadInbox();
    loadHomeSuggestions();
  } catch (err) {
    showErrorToast(err, "suggestions");
  }
}

function cadenceDaysFromSuggestion(suggestion) {
  const data = suggestion.data || {};
  const raw = data.cadenceDays !== undefined ? data.cadenceDays
    : data.days !== undefined ? data.days
    : data.cadence !== undefined ? data.cadence : null;
  if (raw !== null && !isNaN(Number(raw))) return Number(raw);
  const text = (suggestion.title || "") + " " + (suggestion.detail || "");
  const match = text.match(/(\d+)\s*-?\s*day/i);
  return match ? Number(match[1]) : 0;
}

async function runSuggestionAction(act, suggestion) {
  if (act === "videos") {
    if (suggestion.channelId) el("video-channel").value = String(suggestion.channelId);
    setVisibility("", false);
    setToggleChips(false, false, false);
    state.videosLoaded = false;
    navigate("videos");
    loadVideos();
    return;
  }
  if (act === "group") {
    if (suggestion.channelId) el("episode-channel").value = String(suggestion.channelId);
    navigate("episodes");
    loadEpisodes();
    return;
  }
  if (act === "calendar") { navigate("schedules"); return; }
  if (act === "history") { navigate("history"); return; }
  if (act === "cadence") {
    const days = cadenceDaysFromSuggestion(suggestion);
    if (!days || !suggestion.channelId) {
      showToast("Could not read a cadence from this suggestion — set it under Settings → Channel defaults.", "error");
      return;
    }
    try {
      const defaults = await fetchChannelDefaults(suggestion.channelId, true);
      await api("PUT", "/api/channels/" + suggestion.channelId + "/defaults",
        Object.assign({}, defaults || {}, { cadenceDays: days }));
      state.channelDefaults.delete(String(suggestion.channelId));
      showToast("Default cadence for " + (suggestionChannelName(suggestion) || "channel") + " set to " + days + " day(s)", "ok");
    } catch (err) {
      showErrorToast(err, "suggestions");
    }
    return;
  }
  if (act === "thumbs") {
    openThumbnailReview(suggestion);
  }
}

// Thumbnail/title pairing check (item 106): titles that promise a
// part number, paired with the actual thumbnails for a visual check.
async function openThumbnailReview(suggestion) {
  const ids = (suggestion.videoIds || []).slice(0, 20);
  if (ids.length === 0) return;
  const videos = [];
  for (const id of ids) {
    try { videos.push(await api("GET", "/api/videos/" + id)); } catch (err) { /* skip missing */ }
  }
  const overlay = document.createElement("div");
  overlay.className = "overlay";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-label", "Thumbnail pairing check");
  overlay.innerHTML = "<div class=\"overlay-card thumb-review-card\">" +
    "<div class=\"card-head\"><h2>Check the thumbnails</h2>" +
    "<button class=\"thumb-review-close\" aria-label=\"Close thumbnail check\">✕</button></div>" +
    "<p class=\"muted-text\">These titles promise a part number. Check each thumbnail actually shows the same number — a mismatch confuses viewers picking the next part.</p>" +
    (videos.map((video) =>
      "<div class=\"thumb-review-row\">" + thumbnailHTML(video) + "<span>" + escapeHTML(video.title || "") + "</span></div>").join("") ||
      "<p class=\"muted-text\">Those videos could not be loaded.</p>") +
    "</div>";
  document.body.appendChild(overlay);
  const close = () => overlay.remove();
  overlay.querySelector(".thumb-review-close").addEventListener("click", close);
  overlay.addEventListener("click", (event) => { if (event.target === overlay) close(); });
}

el("inbox-list").addEventListener("click", (event) => {
  const dismiss = event.target.closest("[data-sugg-dismiss]");
  if (dismiss) { setSuggestionState(dismiss.dataset.suggDismiss, "dismiss"); return; }
  const snooze = event.target.closest("[data-sugg-snooze]");
  if (snooze) { setSuggestionState(snooze.dataset.suggSnooze, "snooze", 7); return; }
  const reset = event.target.closest("[data-sugg-reset]");
  if (reset) { setSuggestionState(reset.dataset.suggReset, "reset"); return; }
  const action = event.target.closest("[data-sugg-act]");
  if (action) {
    const suggestion = (state.inbox || [])[Number(action.dataset.suggIndex)];
    if (suggestion) runSuggestionAction(action.dataset.suggAct, suggestion);
  }
});

// ---------------------------------------------------------- series
// Series list + rename wizard (item 107).
async function loadSeriesData() {
  const data = await api("GET", "/api/series");
  state.series = Array.isArray(data) ? data : (data.series || []);
  return state.series;
}

async function loadSeries() {
  const box = el("series-list");
  if (!box) return;
  try {
    await loadSeriesData();
    renderSeries();
  } catch (err) {
    box.innerHTML = "<p class=\"muted-text\">Series could not be loaded.</p>";
  }
}

function renderSeries() {
  const box = el("series-list");
  if (!state.series || state.series.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No series detected yet — a series appears once two or more videos share a title pattern (for example “Show EP1”, “Show EP2”).</p>";
    return;
  }
  box.innerHTML = state.series.map((series, index) =>
    "<div class=\"series-row\"><div class=\"series-main\"><strong>" + escapeHTML(series.seriesKey) + "</strong>" +
    " <span class=\"pill\">" + series.count + " videos</span>" +
    (series.grouped ? " <span class=\"pill\">grouped</span>" : "") +
    " <span class=\"muted-text\">· " + escapeHTML(channelTitle(series.channelId) || ("channel " + series.channelId)) + "</span>" +
    (series.sample && series.sample.length
      ? "<div class=\"muted-text\">e.g. " + series.sample.map((title) => escapeHTML(title)).join(" · ") + "</div>" : "") +
    "</div><span class=\"row\"><button data-series-playlist=\"" + index + "\" data-write>Create/update playlist</button>" +
    "<button data-series-wizard=\"" + index + "\" data-write>Rename series</button></span></div>").join("");
}

el("series-list").addEventListener("click", (event) => {
  const playlistBtn = event.target.closest("[data-series-playlist]");
  if (playlistBtn) { createSeriesPlaylist(Number(playlistBtn.dataset.seriesPlaylist)); return; }
  const button = event.target.closest("[data-series-wizard]");
  if (button) openSeriesWizard(Number(button.dataset.seriesWizard));
});

// Series → playlist in one click (item 132): create the playlist
// when none matches the series title, otherwise update the match
// (title compared case-insensitively against the channel's mirror).
async function createSeriesPlaylist(index) {
  const series = (state.series || [])[index];
  if (!series) return;
  try {
    let playlistId = 0;
    try {
      const playlists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(series.channelId));
      const wanted = String(series.seriesKey || "").trim().toLowerCase();
      const match = (playlists || []).find((playlist) =>
        String(playlist.title || "").trim().toLowerCase() === wanted);
      if (match) playlistId = match.id;
    } catch (err) { /* no mirror list: the server creates a fresh playlist */ }
    const body = { channelId: series.channelId, seriesKey: series.seriesKey };
    if (playlistId) body.playlistId = playlistId;
    const data = await api("POST", "/api/series/playlist", body);
    showToast("Playlist “" + (data.playlistTitle || series.seriesKey) + "”: added " +
      data.added + " of " + data.total + " video(s)", "ok");
    loadStatus();
  } catch (err) {
    showErrorToast(err, "episodes");
  }
}

function openSeriesWizard(index) {
  const series = (state.series || [])[index];
  if (!series) return;
  state.wizard = { series, changeIds: null };
  const panel = el("series-wizard");
  panel.classList.remove("hidden");
  panel.innerHTML =
    "<h3>Rename series: " + escapeHTML(series.seriesKey) + "</h3>" +
    "<p class=\"muted-text\">" + series.count + " videos on " +
    escapeHTML(channelTitle(series.channelId) || ("channel " + series.channelId)) +
    ". The plan builder substitutes {title} and {n} only — any other token (like {episode} or {part}) stays literal in the new titles, so check the preview before applying.</p>" +
    "<label>New title template <input id=\"wizard-template\" type=\"text\"></label>" +
    "<div class=\"row\">" +
    "<button id=\"wizard-build-btn\" class=\"primary\" data-write>1. Build preview</button>" +
    "<button id=\"wizard-apply-btn\" data-write disabled>2. Apply rename</button>" +
    "<button id=\"wizard-close-btn\">Close</button></div>" +
    "<div class=\"status-line\" id=\"wizard-status\"></div>" +
    "<div id=\"wizard-preview\"></div>";
  const input = el("wizard-template");
  input.value = series.seriesKey + " EP{episode} Part {part}";
  fetchChannelDefaults(series.channelId).then((defaults) => {
    if (defaults && defaults.renameTemplate && state.wizard && state.wizard.series === series) {
      input.value = defaults.renameTemplate;
    }
  });
  el("wizard-close-btn").addEventListener("click", () => {
    panel.classList.add("hidden");
    panel.innerHTML = "";
    state.wizard = null;
  });
  el("wizard-build-btn").addEventListener("click", buildSeriesPreview);
  el("wizard-apply-btn").addEventListener("click", applySeriesRename);
  panel.scrollIntoView({ block: "nearest" });
}

async function buildSeriesPreview() {
  const wiz = state.wizard;
  if (!wiz) return;
  const series = wiz.series;
  const template = el("wizard-template").value;
  setStatus("wizard-status", "Building the plan for this series...");
  // Reuse the bulk-edit machinery: scope Videos to the series'
  // channel, put the template in the bulk bar, build the plan,
  // then narrow to exactly this series' videos.
  el("bulk-template").value = template;
  el("bulk-regex").value = "";
  el("bulk-replacement").value = "";
  el("bulk-desc-template").value = "";
  el("bulk-start").value = "1";
  updateBulkPreview();
  el("video-q").value = "";
  el("video-channel").value = String(series.channelId);
  setVisibility("", false);
  setToggleChips(false, false, false);
  state.videosLoaded = false;
  await loadVideos();
  const built = await buildBulkPlan();
  if (!built || !state.bulkPlan) {
    setStatus("wizard-status", "The plan could not be built — see Videos for the error.", "error");
    return;
  }
  const seriesIds = new Set((series.videoIds || []).map(Number));
  const changes = (state.bulkPlan.changes || []).filter((change) => seriesIds.has(change.videoId));
  wiz.changeIds = changes.map((change) => change.videoId);
  el("wizard-preview").innerHTML = changes.length
    ? "<p><strong>" + changes.length + " of " + series.count + "</strong> videos in this series would change:</p>" +
      changes.slice(0, 10).map((change) =>
        "<div class=\"preview-line\">" + escapeHTML(change.oldTitle) +
        " <span class=\"move-arrow\">→</span> <strong>" + escapeHTML(change.newTitle) + "</strong></div>").join("") +
      (changes.length > 10 ? "<p class=\"muted-text\">…and " + (changes.length - 10) + " more — the full list is in the Videos plan.</p>" : "")
    : "<p class=\"muted-text\">No videos in this series would change with that template.</p>";
  el("wizard-apply-btn").disabled = changes.length === 0;
  setStatus("wizard-status", changes.length ? "Preview ready — review it, then apply." : "", changes.length ? "ok" : "");
}

async function applySeriesRename() {
  const wiz = state.wizard;
  if (!wiz || !wiz.changeIds || wiz.changeIds.length === 0) return;
  const series = wiz.series;
  const template = el("wizard-template").value;
  const ids = wiz.changeIds.slice();
  state.planIncluded = new Set(ids);
  renderBulkPlan();
  // Remember the scheme (item 107): it becomes the channel's
  // default rename template and a reusable recipe.
  try {
    const defaults = await fetchChannelDefaults(series.channelId, true);
    await api("PUT", "/api/channels/" + series.channelId + "/defaults",
      Object.assign({}, defaults || {}, { renameTemplate: template }));
    state.channelDefaults.delete(String(series.channelId));
  } catch (err) { /* saving the default must not block the rename */ }
  try {
    state.recipes = await api("POST", "/api/rename-recipes", {
      name: series.seriesKey, template, regex: "", replacement: "",
      descriptionTemplate: "", startIndex: 1,
    });
    renderRecipes("");
  } catch (err) { /* the recipe is a convenience, not a blocker */ }
  showToast("Series scheme saved as the channel default and as a recipe", "ok");
  navigate("videos");
  el("bulk-plan").scrollIntoView({ block: "nearest" });
  await applyWithPreflight(ids, "Applying series rename");
}

// ------------------------------------------------------- automation
function loadAutomation() {
  loadProposals();
  renderAutoSyncCard();
  loadRules();
}

// Approval queue (item 116).
async function loadProposals() {
  try {
    const data = await api("GET", "/api/proposals");
    state.proposals = Array.isArray(data) ? data : (data.proposals || []);
    renderProposals();
  } catch (err) {
    setStatus("proposals-status", friendlyError(err), "error");
  }
}

function renderProposals() {
  const box = el("proposals-list");
  const items = (state.proposals || []).slice().sort((a, b) =>
    (a.status === "pending" ? 0 : 1) - (b.status === "pending" ? 0 : 1));
  const pending = items.filter((item) => item.status === "pending").length;
  setBadge("badge-proposals", pending);
  el("proposals-approve-all").disabled = pending === 0;
  if (items.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No proposals yet. Rules in propose mode will queue their actions here.</p>";
    return;
  }
  box.innerHTML = items.map((proposal) =>
    "<div class=\"proposal" + (proposal.status !== "pending" ? " is-done" : "") + "\">" +
    "<div class=\"proposal-head\"><strong>" + escapeHTML(proposal.ruleName || "Rule") + "</strong>" +
    " <span class=\"pill\">" + escapeHTML(proposal.status || "") + "</span>" +
    "<span class=\"muted-text\">" + (proposal.createdAt ? escapeHTML(relativeTime(proposal.createdAt)) : "") +
    " · " + (proposal.actions || []).length + " action(s)</span></div>" +
    "<ul class=\"proposal-actions\">" + (proposal.actions || []).map((action) =>
      "<li><span class=\"pill\">" + escapeHTML(action.type || "") + "</span> " +
      escapeHTML(action.videoTitle || ("Video " + action.videoId)) +
      (action.detail ? " — " + escapeHTML(action.detail) : "") + "</li>").join("") + "</ul>" +
    (proposal.results && proposal.results.length
      ? "<div class=\"muted-text\">Result: " + proposal.results.map((line) => escapeHTML(line)).join("; ") + "</div>" : "") +
    (proposal.status === "pending"
      ? "<div class=\"row\"><button class=\"primary\" data-proposal-approve=\"" + escapeHTML(proposal.id) + "\" data-write>Approve &amp; run</button>" +
        "<button data-proposal-reject=\"" + escapeHTML(proposal.id) + "\" data-write>Reject</button></div>" : "") +
    "</div>").join("");
}

async function decideProposal(id, decision) {
  try {
    await api("POST", "/api/proposals/" + encodeURIComponent(id) + "/" + decision);
    showToast(decision === "approve" ? "Proposal approved and run" : "Proposal rejected", "ok");
    await loadProposals();
  } catch (err) {
    showErrorToast(err, "automation");
  }
}

el("proposals-list").addEventListener("click", (event) => {
  const approve = event.target.closest("[data-proposal-approve]");
  if (approve) { decideProposal(approve.dataset.proposalApprove, "approve"); return; }
  const reject = event.target.closest("[data-proposal-reject]");
  if (reject) decideProposal(reject.dataset.proposalReject, "reject");
});

el("proposals-approve-all").addEventListener("click", async () => {
  try {
    await api("POST", "/api/proposals/approve-all");
    showToast("All pending proposals approved and run", "ok");
    await loadProposals();
  } catch (err) {
    showErrorToast(err, "automation");
  }
});

// Auto-sync (item 111).
function futureRelative(iso) {
  const then = new Date(iso).getTime();
  if (isNaN(then)) return "";
  const diff = then - Date.now();
  if (diff <= 60000) return "within a minute";
  const minutes = Math.round(diff / 60000);
  if (minutes < 60) return "in " + minutes + " min";
  const hours = Math.round(minutes / 60);
  if (hours < 24) return "in " + hours + (hours === 1 ? " hour" : " hours");
  const days = Math.round(hours / 24);
  return "in " + days + (days === 1 ? " day" : " days");
}

function renderAutoSyncCard() {
  const body = el("autosync-body");
  const toggle = el("autosync-toggle");
  if (!body || !toggle) return;
  const auto = state.status && state.status.autoSync;
  if (!auto) {
    body.innerHTML = "<p class=\"muted-text\">Auto-sync status is unavailable — reload the page.</p>";
    toggle.disabled = true;
    return;
  }
  toggle.disabled = false;
  toggle.textContent = auto.paused ? "Resume auto-sync" : "Pause auto-sync";
  if (auto.enabled === false && !auto.nextRunAt) {
    body.innerHTML = "<p>No account has an auto-sync interval yet, so nothing is scheduled. Set one per account under Settings → Per-account overrides.</p>";
    return;
  }
  body.innerHTML = "<p>" + (auto.paused ? "<strong>Paused.</strong> " : "<strong>Running.</strong> ") +
    (auto.nextRunAt
      ? "Next run " + futureRelative(auto.nextRunAt) + " (" + escapeHTML(auto.nextRunAt) + ")."
      : "The next run time is not known yet.") + "</p>";
}

el("autosync-toggle").addEventListener("click", async () => {
  const auto = (state.status && state.status.autoSync) || {};
  try {
    const data = await api("POST", "/api/autosync/pause", { paused: !auto.paused });
    if (!state.status) state.status = {};
    state.status.autoSync = data;
    renderAutoSyncCard();
    showToast(data.paused ? "Auto-sync paused" : "Auto-sync resumed", "ok");
  } catch (err) {
    showErrorToast(err, "automation");
  }
});

// Rules engine (items 112, 113).
function ruleFromForm() {
  const type = el("rule-action-type").value;
  const match = {
    query: el("rule-match-query").value.trim(),
    visibility: el("rule-match-visibility").value,
  };
  if (el("rule-match-channel").value) match.channelId = Number(el("rule-match-channel").value);
  const action = { type };
  if (type === "rename") action.template = el("rule-action-template").value;
  if (type === "comment") action.text = el("rule-action-text").value;
  if (type === "playlist") action.playlistId = Number(el("rule-action-playlist").value || 0);
  return {
    id: el("rule-id").value || "",
    name: el("rule-name").value.trim(),
    enabled: el("rule-enabled").checked,
    mode: el("rule-mode").value,
    match,
    action,
  };
}

function ruleActionSummary(rule) {
  const action = rule.action || {};
  if (action.type === "rename") return "rename → “" + (action.template || "") + "”";
  if (action.type === "comment") return "post first comment";
  if (action.type === "playlist") return "add to playlist #" + (action.playlistId || "");
  return action.type || "";
}

function ruleMatchSummary(rule) {
  const match = rule.match || {};
  const parts = [];
  parts.push(match.channelId ? (channelTitle(match.channelId) || ("channel " + match.channelId)) : "any channel");
  if (match.query) parts.push("title contains “" + match.query + "”");
  if (match.visibility) parts.push(match.visibility);
  return parts.join(" · ");
}

async function loadRules() {
  try {
    const data = await api("GET", "/api/rules");
    state.rules = Array.isArray(data) ? data : (data.rules || []);
    renderRules();
  } catch (err) {
    el("rules-list").innerHTML = "<p class=\"muted-text\">Rules could not be loaded.</p>";
  }
  loadRulePlaylists();
}

function renderRules() {
  const box = el("rules-list");
  if (!state.rules || state.rules.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No rules yet — create the first one below.</p>";
    return;
  }
  box.innerHTML = state.rules.map((rule, index) =>
    "<div class=\"rule-row\"><div class=\"rule-main\"><strong>" + escapeHTML(rule.name || "Untitled rule") + "</strong>" +
    " <span class=\"pill\">" + (rule.enabled ? "enabled" : "disabled") + "</span>" +
    " <span class=\"pill\">" + escapeHTML(rule.mode || "propose") + "</span>" +
    "<div class=\"muted-text\">When: " + escapeHTML(ruleMatchSummary(rule)) + "</div>" +
    "<div class=\"muted-text\">Then: " + escapeHTML(ruleActionSummary(rule)) + "</div></div>" +
    "<div class=\"row\"><button data-rule-edit=\"" + index + "\">Edit</button>" +
    "<button data-rule-delete=\"" + escapeHTML(rule.id) + "\" data-write>Delete</button></div></div>").join("");
}

async function loadRulePlaylists() {
  const select = el("rule-action-playlist");
  const channels = (state.status && state.status.channels) || [];
  const seen = new Map();
  for (const channel of channels) {
    try {
      const playlists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(channel.id));
      (playlists || []).forEach((playlist) => {
        if (!seen.has(playlist.id)) seen.set(playlist.id, playlist.title + " (" + channel.title + ")");
      });
    } catch (err) { /* a channel without playlists is fine */ }
  }
  const current = select.value;
  select.innerHTML = "<option value=\"\">Choose a playlist…</option>" + Array.from(seen.entries()).map(([id, label]) =>
    "<option value=\"" + id + "\">" + escapeHTML(label) + "</option>").join("");
  if (current) select.value = current;
}

function updateRuleActionRows() {
  const type = el("rule-action-type").value;
  el("rule-action-template-row").classList.toggle("hidden", type !== "rename");
  el("rule-action-text-row").classList.toggle("hidden", type !== "comment");
  el("rule-action-playlist-row").classList.toggle("hidden", type !== "playlist");
}

el("rule-action-type").addEventListener("change", updateRuleActionRows);
el("rule-match-channel").addEventListener("change", loadRulePlaylists);

function resetRuleForm() {
  el("rule-id").value = "";
  el("rule-name").value = "";
  el("rule-enabled").checked = true;
  el("rule-mode").value = "propose";
  el("rule-match-channel").value = "";
  el("rule-match-query").value = "";
  el("rule-match-visibility").value = "";
  el("rule-action-type").value = "rename";
  el("rule-action-template").value = "";
  el("rule-action-text").value = "";
  el("rule-action-playlist").value = "";
  el("rule-form-title").textContent = "New rule";
  el("rule-dry-run-result").classList.add("hidden");
  el("rule-dry-run-result").innerHTML = "";
  setStatus("rule-status", "");
  updateRuleActionRows();
}

el("rule-reset-btn").addEventListener("click", resetRuleForm);

function fillRuleForm(rule) {
  el("rule-id").value = rule.id || "";
  el("rule-name").value = rule.name || "";
  el("rule-enabled").checked = !!rule.enabled;
  el("rule-mode").value = rule.mode || "propose";
  const match = rule.match || {};
  el("rule-match-channel").value = match.channelId ? String(match.channelId) : "";
  el("rule-match-query").value = match.query || "";
  el("rule-match-visibility").value = match.visibility || "";
  const action = rule.action || {};
  el("rule-action-type").value = action.type || "rename";
  el("rule-action-template").value = action.template || "";
  el("rule-action-text").value = action.text || "";
  updateRuleActionRows();
  if (action.playlistId) {
    loadRulePlaylists().then(() => { el("rule-action-playlist").value = String(action.playlistId); });
  }
  el("rule-form-title").textContent = "Edit rule: " + (rule.name || "");
  el("rule-form").scrollIntoView({ block: "nearest" });
}

el("rules-list").addEventListener("click", async (event) => {
  const edit = event.target.closest("[data-rule-edit]");
  if (edit) { fillRuleForm(state.rules[Number(edit.dataset.ruleEdit)]); return; }
  const del = event.target.closest("[data-rule-delete]");
  if (del) {
    if (!window.confirm("Delete this rule? Its past activity stays in History.")) return;
    try {
      await api("DELETE", "/api/rules/" + encodeURIComponent(del.dataset.ruleDelete));
      showToast("Rule deleted", "ok");
      await loadRules();
    } catch (err) {
      showErrorToast(err, "automation");
    }
  }
});

el("rule-dry-run-btn").addEventListener("click", async () => {
  const rule = ruleFromForm();
  setStatus("rule-status", "Dry-running against your library...");
  try {
    const data = await api("POST", "/api/rules/dry-run", { rule });
    const box = el("rule-dry-run-result");
    box.classList.remove("hidden");
    const actions = data.actions || [];
    box.innerHTML = "<div class=\"preview-label\">Dry run — nothing was changed</div>" +
      "<p>" + (data.matched || 0) + " video(s) match, " + (data.alreadyProcessed || 0) +
      " already processed by this rule, " + actions.length + " action(s) would run:</p>" +
      (actions.length
        ? "<ul class=\"proposal-actions\">" + actions.map((action) =>
          "<li><span class=\"pill\">" + escapeHTML(action.type || "") + "</span> " +
          escapeHTML(action.videoTitle || ("Video " + action.videoId)) +
          (action.detail ? " — " + escapeHTML(action.detail) : "") + "</li>").join("") + "</ul>"
        : "<p class=\"muted-text\">No actions would run right now.</p>");
    setStatus("rule-status", "Dry run complete", "ok");
  } catch (err) {
    setStatus("rule-status", friendlyError(err), "error");
    showErrorToast(err, "automation");
  }
});

el("rule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const rule = ruleFromForm();
  if (!rule.name) { setStatus("rule-status", "Give the rule a name first.", "error"); return; }
  setStatus("rule-status", "Saving rule...");
  try {
    await api("POST", "/api/rules", rule);
    showToast("Rule “" + rule.name + "” saved", "ok");
    resetRuleForm();
    await loadRules();
  } catch (err) {
    setStatus("rule-status", friendlyError(err), "error");
    showErrorToast(err, "automation");
  }
});

// ---------------------------------------------------- comment centre
// One screen for first comments across channels (items 121-125).
async function loadCommentCentre() {
  setStatus("cc-status", "Loading...");
  try {
    const data = await api("GET", "/api/comment-centre");
    state.commentCentre = Array.isArray(data) ? data : (data.rows || data.items || []);
    renderCommentCentre();
    loadCommentCoverage();
    const pending = state.commentCentre.filter((row) => row.status === "pending").length;
    const needsPin = state.commentCentre.filter((row) => row.needsPin).length;
    setStatus("cc-status", state.commentCentre.length + " video(s) · " + pending + " pending · " + needsPin + " need pinning", "ok");
  } catch (err) {
    setStatus("cc-status", friendlyError(err), "error");
    showErrorToast(err, "commentcentre");
  }
}

el("cc-load-btn").addEventListener("click", loadCommentCentre);

el("cc-filter-chips").addEventListener("click", (event) => {
  const chip = event.target.closest("[data-cc-filter]");
  if (!chip) return;
  state.ccFilter = chip.dataset.ccFilter;
  document.querySelectorAll("#cc-filter-chips .chip").forEach((item) =>
    item.classList.toggle("is-active", item === chip));
  renderCommentCentre();
});

function ccRowVisible(row) {
  if (state.ccGapFilter && !state.ccGapFilter.ids.has(Number(row.videoId))) return false;
  if (state.ccFilter === "posted") return row.status === "posted";
  if (state.ccFilter === "pending") return row.status === "pending";
  if (state.ccFilter === "needspin") return !!row.needsPin;
  return true;
}

function renderCommentCentre() {
  const box = el("cc-list");
  const rows = (state.commentCentre || []).filter(ccRowVisible);
  if (rows.length === 0) {
    box.innerHTML = "<div class=\"empty-state\"><span class=\"state-art\" aria-hidden=\"true\">❝</span><h3>Nothing here</h3>" +
      "<p>No videos match this filter. Sync a channel to mirror its videos, then post and track first comments here.</p></div>";
    return;
  }
  box.innerHTML = rows.map((row) => {
    const comment = row.comment || null;
    const statusPill = row.status === "posted" ? "<span class=\"pill\">posted</span>"
      : row.status === "pending" ? "<span class=\"pill cc-pending\">pending</span>"
      : "<span class=\"pill\">draft</span>";
    const pinState = comment
      ? (row.needsPin ? "<span class=\"warn-badge\">needs pin</span>" : "<span class=\"result-ok\">pinned ✓</span>")
      : "<span class=\"muted-text\">—</span>";
    let actions = "";
    if (comment) {
      actions += "<a class=\"btn-link\" href=\"https://www.youtube.com/watch?v=" + encodeURIComponent(row.ytVideoId || "") +
        "\" target=\"_blank\" rel=\"noopener\">Open in YouTube</a>";
      if (row.needsPin) {
        actions += "<button data-cc-pin=\"" + comment.id + "\" data-cc-pinned=\"true\" data-write>Mark pinned</button>";
      } else {
        actions += "<button data-cc-pin=\"" + comment.id + "\" data-cc-pinned=\"false\" data-write>Unmark pin</button>";
      }
      actions += "<button data-cc-history-toggle=\"" + comment.id + "\">History &amp; edit</button>";
    } else if (row.status === "pending") {
      actions += "<button class=\"primary\" data-cc-post-open=\"" + row.videoId + "\" data-write>Post first comment</button>";
    }
    const commentText = comment
      ? escapeHTML(String(comment.text || "").slice(0, 140)) + (String(comment.text || "").length > 140 ? "…" : "")
      : "<span class=\"muted-text\">No first comment yet</span>";
    // Pinned-text drift (item 129): only when the DTO actually
    // carries pinnedText — the current commentDTO does not, so this
    // hint stays dormant rather than guessing.
    const pinHint = comment && typeof comment.pinnedText === "string" && comment.pinnedText !== "" &&
      comment.pinnedText !== (comment.text || "")
      ? " <span class=\"warn-badge\">text changed since pin</span>" : "";
    let html = "<div class=\"cc-row\" id=\"cc-row-" + row.videoId + "\">" +
      "<div class=\"cc-video\">" + thumbnailHTML(row) + "<span><strong>" + escapeHTML(row.title || "") + "</strong>" +
      "<span class=\"muted-text\"> · " + escapeHTML(channelTitle(row.channelId) || ("channel " + row.channelId)) + "</span></span></div>" +
      "<div class=\"cc-status\">" + statusPill + "</div>" +
      "<div class=\"cc-comment\">" + commentText + pinHint + "</div>" +
      "<div class=\"cc-pin\">" + pinState + "</div>" +
      "<div class=\"cc-actions row\">" + actions + "</div>";
    if (!comment && row.status === "pending") {
      html += "<div class=\"cc-editor hidden\" id=\"cc-editor-" + row.videoId + "\">" +
        "<label>First comment for this video <textarea id=\"cc-text-" + row.videoId + "\" rows=\"3\"></textarea></label>" +
        "<p class=\"muted-text\">Template tokens: {title} {episode} {series} {channel}. Preview renders the template for this video; posting sends the rendered text.</p>" +
        "<div class=\"row\"><button data-cc-preview=\"" + row.videoId + "\">Preview</button>" +
        "<button class=\"primary\" data-cc-send=\"" + row.videoId + "\" data-write>Post comment</button></div>" +
        "<div class=\"cc-preview hidden\" id=\"cc-preview-" + row.videoId + "\"></div></div>";
    }
    if (comment) {
      html += "<div class=\"cc-history hidden\" id=\"cc-history-" + comment.id + "\"><p class=\"muted-text\">Loading history…</p></div>";
    }
    return html + "</div>";
  }).join("");
}

async function openCommentEditor(videoId) {
  const editor = el("cc-editor-" + videoId);
  if (!editor) return;
  editor.classList.remove("hidden");
  const area = el("cc-text-" + videoId);
  const row = (state.commentCentre || []).find((item) => String(item.videoId) === String(videoId));
  if (area && !area.value && row) {
    const defaults = await fetchChannelDefaults(row.channelId);
    if (defaults && defaults.commentTemplate && !area.value) {
      area.value = defaults.commentTemplate;
    }
  }
  if (area) area.focus();
}

async function toggleCommentHistory(commentId) {
  const box = el("cc-history-" + commentId);
  if (!box) return;
  if (!box.classList.contains("hidden")) { box.classList.add("hidden"); return; }
  box.classList.remove("hidden");
  box.innerHTML = "<p class=\"muted-text\">Loading history…</p>";
  const row = (state.commentCentre || []).find((item) => item.comment && String(item.comment.id) === String(commentId));
  const current = row && row.comment ? row.comment.text || "" : "";
  try {
    const history = await api("GET", "/api/comments/" + commentId + "/history");
    const entries = Array.isArray(history) ? history : (history.entries || []);
    box.innerHTML =
      "<div class=\"preview-label\">Edit the current comment</div>" +
      "<label>Current text <textarea id=\"cc-edit-text-" + commentId + "\" rows=\"3\">" + escapeHTML(current) + "</textarea></label>" +
      "<div class=\"row\"><button class=\"primary\" data-cc-edit-save=\"" + commentId + "\" data-write>Save edit</button></div>" +
      "<div class=\"preview-label\">Prior versions</div>" +
      (entries.length
        ? entries.map((entry) =>
          "<div class=\"cc-version\"><span class=\"cc-version-text\">" + escapeHTML(entry.oldText || "") + "</span>" +
          "<span class=\"muted-text\">" + (entry.createdAt ? escapeHTML(relativeTime(entry.createdAt)) : "") + "</span>" +
          "<button data-cc-restore=\"" + commentId + "\" data-cc-history-id=\"" + entry.id + "\" data-write>Restore</button></div>").join("")
        : "<p class=\"muted-text\">No prior versions — edits you make are recorded here.</p>");
  } catch (err) {
    box.innerHTML = "<p class=\"muted-text\">History could not be loaded.</p>";
  }
}

el("cc-list").addEventListener("click", async (event) => {
  const pin = event.target.closest("[data-cc-pin]");
  if (pin) {
    try {
      await api("POST", "/api/comments/" + pin.dataset.ccPin + "/pin", { pinned: pin.dataset.ccPinned === "true" });
      showToast(pin.dataset.ccPinned === "true" ? "Marked as pinned" : "Pin mark removed", "ok");
      await loadCommentCentre();
    } catch (err) { showErrorToast(err, "commentcentre"); }
    return;
  }
  const open = event.target.closest("[data-cc-post-open]");
  if (open) { openCommentEditor(open.dataset.ccPostOpen); return; }
  const preview = event.target.closest("[data-cc-preview]");
  if (preview) {
    const videoId = preview.dataset.ccPreview;
    const row = (state.commentCentre || []).find((item) => String(item.videoId) === String(videoId));
    const area = el("cc-text-" + videoId);
    const box = el("cc-preview-" + videoId);
    if (!row || !area || !box) return;
    try {
      const results = await api("POST", "/api/comments/preview", { channelId: row.channelId, template: area.value });
      const mine = (Array.isArray(results) ? results : []).find((item) => String(item.videoId) === String(videoId));
      if (mine) {
        state.ccRendered[videoId] = { template: area.value, text: mine.text };
        box.innerHTML = "<div class=\"preview-label\">Preview — this exact text will be posted</div><p>" + escapeHTML(mine.text) + "</p>";
      } else {
        box.innerHTML = "<p class=\"muted-text\">The preview did not include this video — the raw text will be posted as-is.</p>";
        delete state.ccRendered[videoId];
      }
      box.classList.remove("hidden");
    } catch (err) { showErrorToast(err, "commentcentre"); }
    return;
  }
  const send = event.target.closest("[data-cc-send]");
  if (send) {
    const videoId = send.dataset.ccSend;
    const area = el("cc-text-" + videoId);
    if (!area || !area.value.trim()) { showToast("Write the comment first (or load the channel template).", "error"); return; }
    const rendered = state.ccRendered[videoId];
    const text = rendered && rendered.template === area.value ? rendered.text : area.value;
    send.disabled = true;
    try {
      await api("POST", "/api/videos/" + videoId + "/first-comment", { text });
      showToast("First comment posted", "ok");
      delete state.ccRendered[videoId];
      await loadCommentCentre();
      loadStatus();
    } catch (err) {
      showErrorToast(err, "commentcentre");
      send.disabled = false;
    }
    return;
  }
  const historyToggle = event.target.closest("[data-cc-history-toggle]");
  if (historyToggle) { toggleCommentHistory(historyToggle.dataset.ccHistoryToggle); return; }
  const editSave = event.target.closest("[data-cc-edit-save]");
  if (editSave) {
    const commentId = editSave.dataset.ccEditSave;
    const area = el("cc-edit-text-" + commentId);
    if (!area) return;
    try {
      await api("PATCH", "/api/comments/" + commentId, { text: area.value });
      showToast("Comment updated — the old text is in its history", "ok");
      await loadCommentCentre();
    } catch (err) { showErrorToast(err, "commentcentre"); }
    return;
  }
  const restore = event.target.closest("[data-cc-restore]");
  if (restore) {
    try {
      await api("POST", "/api/comments/" + restore.dataset.ccRestore + "/restore",
        { historyId: Number(restore.dataset.ccHistoryId) });
      showToast("Comment restored to that version", "ok");
      await loadCommentCentre();
    } catch (err) { showErrorToast(err, "commentcentre"); }
  }
});

// -------------------------------------- comment centre, wave 6
// Coverage report (item 127): per channel, how many public videos
// already have a first comment, with the gaps one click away.
async function loadCommentCoverage() {
  const box = el("cc-coverage");
  if (!box) return;
  try {
    const reports = await api("GET", "/api/comment-coverage");
    state.ccCoverage = Array.isArray(reports) ? reports : [];
    renderCommentCoverage();
  } catch (err) {
    box.innerHTML = "";
  }
}

function renderCommentCoverage() {
  const box = el("cc-coverage");
  if (!box) return;
  const reports = state.ccCoverage || [];
  if (reports.length === 0) { box.innerHTML = ""; return; }
  let html = reports.map((report) => {
    const pct = Math.round(report.percent || 0);
    const active = state.ccGapFilter && String(state.ccGapFilter.channelId) === String(report.channelId);
    return "<div class=\"cc-coverage-row\"><span><strong>" + escapeHTML(channelTitle(report.channelId) || ("Channel " + report.channelId)) +
      "</strong> — " + pct + "% of public videos have a first comment (" + report.withFirst + "/" + report.public + ")</span>" +
      (report.gapCount > 0
        ? "<button data-cc-gaps=\"" + report.channelId + "\">" + (active ? "Showing gaps" : "View gaps (" + report.gapCount + ")") + "</button>"
        : "<span class=\"result-ok\">No gaps ✓</span>") + "</div>";
  }).join("");
  if (state.ccGapFilter) {
    html += "<div class=\"cc-coverage-row\"><span>Filtered to the gap videos of " +
      escapeHTML(channelTitle(state.ccGapFilter.channelId) || ("channel " + state.ccGapFilter.channelId)) + ".</span>" +
      "<button data-cc-gaps-clear=\"1\">Clear gap filter</button></div>";
  }
  box.innerHTML = html;
}

el("cc-coverage").addEventListener("click", (event) => {
  if (event.target.closest("[data-cc-gaps-clear]")) {
    state.ccGapFilter = null;
    renderCommentCoverage();
    renderCommentCentre();
    return;
  }
  const button = event.target.closest("[data-cc-gaps]");
  if (!button) return;
  const report = (state.ccCoverage || []).find((item) => String(item.channelId) === button.dataset.ccGaps);
  if (!report) return;
  state.ccGapFilter = { channelId: report.channelId, ids: new Set((report.gapVideoIds || []).map(Number)) };
  renderCommentCoverage();
  renderCommentCentre();
});

// Link checker (item 126): distinct URLs in first comments, checked
// once each by the server (capped at 20 unique URLs).
el("cc-check-links-btn").addEventListener("click", async () => {
  const box = el("cc-link-results");
  box.classList.remove("hidden");
  box.innerHTML = "<h2>Comment links</h2><p class=\"muted-text\">Checking the links in your first comments…</p>";
  try {
    const results = await api("POST", "/api/comments/check-links", {});
    const list = Array.isArray(results) ? results : [];
    const dead = list.filter((item) => !item.ok);
    let html = "<div class=\"card-head\"><h2>Comment links</h2><button id=\"cc-link-close\">Close</button></div>" +
      "<p>" + list.length + " link(s) checked, " + dead.length + " dead" +
      (list.length >= 20 ? " — the checker stops at 20 unique URLs per run, so more links may exist." : "") + "</p>";
    if (list.length === 0) {
      html += "<p class=\"muted-text\">No URLs found in any first comment.</p>";
    } else if (dead.length === 0) {
      html += "<p class=\"result-ok\">Every checked link is alive ✓</p>";
    } else {
      html += dead.map((item) =>
        "<div class=\"cc-link-row\"><a href=\"" + escapeHTML(item.url) + "\" target=\"_blank\" rel=\"noopener\">" + escapeHTML(item.url) + "</a>" +
        "<span class=\"result-fail\">" + (item.error ? escapeHTML(item.error) : "HTTP " + item.status) + "</span>" +
        "<button data-cc-goto-video=\"" + item.videoId + "\">Show video in centre</button></div>").join("");
    }
    box.innerHTML = html;
    el("cc-link-close").addEventListener("click", () => box.classList.add("hidden"));
  } catch (err) {
    box.innerHTML = "<h2>Comment links</h2><p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
    showErrorToast(err, "commentcentre");
  }
});

el("cc-link-results").addEventListener("click", (event) => {
  const button = event.target.closest("[data-cc-goto-video]");
  if (!button) return;
  state.ccGapFilter = null;
  state.ccFilter = "all";
  document.querySelectorAll("#cc-filter-chips .chip").forEach((item) =>
    item.classList.toggle("is-active", item.dataset.ccFilter === "all"));
  renderCommentCoverage();
  renderCommentCentre();
  const row = el("cc-row-" + button.dataset.ccGotoVideo);
  if (row) {
    row.scrollIntoView({ block: "center" });
    row.classList.add("cc-flash");
    setTimeout(() => row.classList.remove("cc-flash"), 1600);
  }
});

// Bulk post first comments (item 130): preview the change-set,
// untick rows to exclude them, then post with per-item results.
el("cc-bulk-channel").addEventListener("change", async () => {
  const channelId = el("cc-bulk-channel").value;
  el("cc-bulk-results").innerHTML = "";
  el("cc-bulk-post-btn").disabled = true;
  state.ccBulkRows = [];
  if (!channelId) return;
  const defaults = await fetchChannelDefaults(channelId);
  if (defaults && defaults.commentTemplate && !el("cc-bulk-template").value) {
    el("cc-bulk-template").value = defaults.commentTemplate;
  }
});

el("cc-bulk-preview-btn").addEventListener("click", async () => {
  const channelId = el("cc-bulk-channel").value;
  if (!channelId) { setStatus("cc-bulk-status", "Choose a channel first.", "error"); return; }
  setStatus("cc-bulk-status", "Rendering the change-set…");
  try {
    const rows = await api("POST", "/api/comments/bulk-preview", {
      channelId: Number(channelId),
      template: el("cc-bulk-template").value,
    });
    state.ccBulkRows = Array.isArray(rows) ? rows : [];
    renderBulkCommentPreview();
    setStatus("cc-bulk-status", state.ccBulkRows.length + " pending video(s) on this channel", "ok");
  } catch (err) {
    setStatus("cc-bulk-status", friendlyError(err), "error");
    showErrorToast(err, "commentcentre");
  }
});

function renderBulkCommentPreview() {
  const box = el("cc-bulk-results");
  const rows = state.ccBulkRows || [];
  el("cc-bulk-post-btn").disabled = rows.length === 0;
  if (rows.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No pending videos — every public video on this channel already has a first comment.</p>";
    return;
  }
  box.innerHTML = "<table class=\"grid\"><thead><tr><th>Post</th><th>Video</th><th>Comment that will be posted</th></tr></thead><tbody>" +
    rows.map((row, index) =>
      "<tr><td><input type=\"checkbox\" class=\"cc-bulk-include\" data-bulk-index=\"" + index + "\" checked></td>" +
      "<td>" + escapeHTML(row.title || ("Video " + row.videoId)) + "</td>" +
      "<td>" + escapeHTML(row.text || "") + "</td></tr>").join("") + "</tbody></table>";
}

el("cc-bulk-post-btn").addEventListener("click", async () => {
  const picked = Array.from(document.querySelectorAll(".cc-bulk-include:checked"))
    .map((box) => state.ccBulkRows[Number(box.dataset.bulkIndex)])
    .filter(Boolean)
    .map((row) => ({ videoId: row.videoId, title: row.title, text: row.text }));
  if (picked.length === 0) { setStatus("cc-bulk-status", "Tick at least one row to post.", "error"); return; }
  const button = el("cc-bulk-post-btn");
  button.disabled = true;
  setStatus("cc-bulk-status", "Posting " + picked.length + " first comment(s)…");
  startTask("cc-bulk", "Posting first comments");
  try {
    const data = await api("POST", "/api/comments/bulk-post", { items: picked });
    const results = data.results || [];
    el("cc-bulk-results").innerHTML = "<p><strong>" + (data.posted || 0) + " posted</strong> of " + picked.length + ":</p>" +
      results.map((row) => row.error
        ? "<div class=\"cc-link-row\"><span>" + escapeHTML(row.title || ("Video " + row.videoId)) + "</span><span class=\"result-fail\">✗ " + escapeHTML(row.error) + "</span></div>"
        : "<div class=\"cc-link-row\"><span>" + escapeHTML(row.title || ("Video " + row.videoId)) + "</span><span class=\"result-ok\">✓ Posted</span></div>").join("");
    setStatus("cc-bulk-status", "Bulk post finished: " + (data.posted || 0) + " posted", "ok");
    showToast("Bulk post: " + (data.posted || 0) + " first comment(s) posted", "ok");
    state.ccBulkRows = [];
    await loadCommentCentre();
    loadStatus();
  } catch (err) {
    setStatus("cc-bulk-status", friendlyError(err), "error");
    showErrorToast(err, "commentcentre");
    button.disabled = false;
  } finally {
    endTask("cc-bulk");
  }
});

// ------------------------------------------- channel defaults card
el("cd-channel").addEventListener("change", loadChannelDefaultsCard);

async function loadChannelDefaultsCard() {
  const channelId = el("cd-channel").value;
  const fields = el("cd-fields");
  if (!channelId) { fields.classList.add("hidden"); return; }
  fields.classList.remove("hidden");
  setStatus("cd-status", "Loading…");
  const defaults = (await fetchChannelDefaults(channelId, true)) || normalizeDefaults(null);
  el("cd-rename-template").value = defaults.renameTemplate;
  el("cd-cadence").value = defaults.cadenceDays || "";
  el("cd-comment-template").value = defaults.commentTemplate;
  el("cd-playlist-title-template").value = defaults.playlistTitleTemplate;
  el("cd-playlist-desc-template").value = defaults.playlistDescriptionTemplate;
  el("cd-pipeline-comment").checked = defaults.pipelineComment;
  el("cd-pipeline-email").checked = defaults.pipelineEmail;
  try {
    const playlists = await api("GET", "/api/playlists?channel=" + encodeURIComponent(channelId));
    el("cd-pipeline-playlist").innerHTML = "<option value=\"0\">None</option>" + (playlists || []).map((playlist) =>
      "<option value=\"" + playlist.id + "\">" + escapeHTML(playlist.title) + "</option>").join("");
    el("cd-pipeline-playlist").value = String(defaults.pipelinePlaylistId || 0);
  } catch (err) {
    el("cd-pipeline-playlist").innerHTML = "<option value=\"0\">None</option>";
  }
  let series = (state.series || []).filter((item) => String(item.channelId) === String(channelId));
  if (series.length === 0) {
    try { series = (await loadSeriesData()).filter((item) => String(item.channelId) === String(channelId)); }
    catch (err) { series = []; }
  }
  const watched = new Set((defaults.watchSeries || []).map(String));
  el("cd-watch-series").innerHTML = series.length
    ? series.map((item) =>
      "<label class=\"check\"><input type=\"checkbox\" class=\"cd-watch\" value=\"" + escapeHTML(item.seriesKey) + "\"" +
      (watched.has(String(item.seriesKey)) ? " checked" : "") + "> " + escapeHTML(item.seriesKey) +
      " (" + item.count + " videos)</label>").join("")
    : "<p class=\"muted-text\">No series detected on this channel yet.</p>";
  setStatus("cd-status", "");
}

el("cd-save-btn").addEventListener("click", async () => {
  const channelId = el("cd-channel").value;
  if (!channelId) return;
  const body = {
    renameTemplate: el("cd-rename-template").value,
    cadenceDays: Number(el("cd-cadence").value || 0),
    commentTemplate: el("cd-comment-template").value,
    playlistTitleTemplate: el("cd-playlist-title-template").value,
    playlistDescriptionTemplate: el("cd-playlist-desc-template").value,
    pipelineComment: el("cd-pipeline-comment").checked,
    pipelineEmail: el("cd-pipeline-email").checked,
    pipelinePlaylistId: Number(el("cd-pipeline-playlist").value || 0),
    watchSeries: Array.from(document.querySelectorAll(".cd-watch:checked")).map((box) => box.value),
  };
  setStatus("cd-status", "Saving…");
  try {
    await api("PUT", "/api/channels/" + channelId + "/defaults", body);
    state.channelDefaults.set(String(channelId), normalizeDefaults(body));
    setStatus("cd-status", "Channel defaults saved", "ok");
    showToast("Channel defaults saved", "ok");
  } catch (err) {
    setStatus("cd-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
});

// Workflow bundles (item 118): rules + recipes + saved searches +
// channel defaults as one shareable file.
el("workflow-export-btn").addEventListener("click", async () => {
  try {
    const bundle = await api("GET", "/api/workflows/export");
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "yt-manage-workflow.json";
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 5000);
    setStatus("settings-status", "Workflow exported (rules, recipes, saved searches, channel defaults).", "ok");
    showToast("Workflow exported", "ok");
  } catch (err) {
    setStatus("settings-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
});

el("workflow-import-btn").addEventListener("click", () => el("workflow-import-file").click());

el("workflow-import-file").addEventListener("change", async () => {
  const file = el("workflow-import-file").files[0];
  el("workflow-import-file").value = "";
  if (!file) return;
  let bundle;
  try {
    bundle = JSON.parse(await file.text());
  } catch (err) {
    setStatus("settings-status", "That file is not valid JSON — nothing was imported.", "error");
    return;
  }
  if (!window.confirm("Import this workflow? It replaces your current rules, rename recipes and saved searches, and merges its channel defaults.")) return;
  try {
    await api("POST", "/api/workflows/import", bundle);
    setStatus("settings-status", "Workflow imported.", "ok");
    showToast("Workflow imported", "ok");
    loadRecipes();
    loadSavedSearches();
    state.channelDefaults.clear();
    if (el("cd-channel").value) loadChannelDefaultsCard();
  } catch (err) {
    setStatus("settings-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
});

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
        "<button data-ab-win=\"b\" data-ab-id=\"" + test.id + "\">Pick B</button>" +
        "<button data-ab-results=\"" + test.id + "\">Results</button></div>" +
        "<div class=\"ab-results hidden\" id=\"ab-results-" + test.id + "\"></div></div>";
    }).join("") ||
      "<div class=\"empty-state\"><h3>No A/B tests yet</h3>" +
      "<p>An A/B test tries two titles on one video. Create one with the form below, take snapshots as views come in, then pick the winner and yt-manage applies it.</p></div>";
    setStatus("ab-status", tests.length + " test(s)", "ok");
  } catch (err) {
    setStatus("ab-status", friendlyError(err), "error");
  }
}

el("ab-load-btn").addEventListener("click", loadABTests);

// A/B results view (item 143) + auto-schedule (item 144).
function toLocalInputValue(iso) {
  if (!iso) return "";
  const date = new Date(iso);
  if (isNaN(date.getTime())) return "";
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

function toRFC3339(localValue) {
  if (!localValue) return "";
  const date = new Date(localValue);
  return isNaN(date.getTime()) ? "" : date.toISOString();
}

function testDurationText(test) {
  const start = new Date(test.createdAt).getTime();
  if (isNaN(start)) return "";
  const end = test.isActive ? Date.now() : new Date(test.updatedAt).getTime();
  if (isNaN(end) || end < start) return "";
  const hours = Math.floor((end - start) / 3600000);
  if (hours < 24) return hours + (hours === 1 ? " hour" : " hours");
  const days = Math.floor(hours / 24);
  return days + (days === 1 ? " day" : " days") + (test.isActive ? " so far" : "");
}

async function toggleABResults(id) {
  const box = el("ab-results-" + id);
  if (!box) return;
  if (!box.classList.contains("hidden")) { box.classList.add("hidden"); return; }
  box.classList.remove("hidden");
  box.innerHTML = "<p class=\"muted-text\">Loading results…</p>";
  try {
    const data = await api("GET", "/api/ab-tests/" + id + "/results");
    renderABResults(id, data);
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

function renderABResults(id, data) {
  const box = el("ab-results-" + id);
  if (!box) return;
  const test = data.test || {};
  const video = data.video || {};
  const snapshots = data.snapshots || test.snapshots || [];
  const schedule = data.schedule || null;
  const delta = data.deltaViews || 0;
  let html = "<div class=\"preview-label\">Results</div>" +
    "<p><strong>A:</strong> " + escapeHTML(test.variantA || "") + "<br><strong>B:</strong> " + escapeHTML(test.variantB || "") + "</p>" +
    "<p class=\"muted-text\">Running for " + escapeHTML(testDurationText(test) || "—") +
    " · Video now: " + escapeHTML(video.title || ("Video " + test.videoId)) + " — " +
    (video.viewCount || 0) + " views, " + (video.commentCount || 0) + " comments</p>" +
    "<p>Snapshot views: first " + (data.firstSnapshotViews || 0) + " → last " + (data.lastSnapshotViews || 0) +
    " (<strong>" + (delta >= 0 ? "+" : "") + delta + "</strong> views across snapshots)</p>";
  if (snapshots.length) {
    html += "<ul class=\"parts\">" + snapshots.map((snap) =>
      "<li>" + escapeHTML(snap.capturedAt) + ": views " + escapeHTML(snap.viewsAContext) +
      (snap.ctrNote ? " — " + escapeHTML(snap.ctrNote) : "") + "</li>").join("") + "</ul>";
  } else {
    html += "<p class=\"muted-text\">No snapshots yet — take one from the test card so the results are comparable.</p>";
  }
  html += "<div class=\"row\"><button data-ab-win=\"a\" data-ab-id=\"" + id + "\" data-write>Pick winner A</button>" +
    "<button data-ab-win=\"b\" data-ab-id=\"" + id + "\" data-write>Pick winner B</button></div>";
  // Auto-schedule form (item 144): switch / mid snapshot / end.
  html += "<h4>Auto-schedule</h4>" +
    "<p class=\"muted-text\">The automation loop applies each step when its time comes, while the app is serving. Leave a step empty to clear it.</p>" +
    "<label>Switch to variant B <input type=\"datetime-local\" id=\"ab-sched-switch-" + id + "\" value=\"" +
    escapeHTML(toLocalInputValue(schedule ? schedule.switchAt : "")) + "\"></label>" +
    "<label>Mid-way snapshot <input type=\"datetime-local\" id=\"ab-sched-snapshot-" + id + "\" value=\"" +
    escapeHTML(toLocalInputValue(schedule ? schedule.snapshotAt : "")) + "\"></label>" +
    "<label>End test <input type=\"datetime-local\" id=\"ab-sched-end-" + id + "\" value=\"" +
    escapeHTML(toLocalInputValue(schedule ? schedule.endAt : "")) + "\"></label>";
  if (schedule) {
    const flags = [];
    if (schedule.switched) flags.push("switched ✓");
    if (schedule.snapshotted) flags.push("snapshot taken ✓");
    if (schedule.ended) flags.push("ended ✓");
    html += "<p class=\"muted-text\">Saved schedule: switch " + escapeHTML(schedule.switchAt || "—") +
      " · snapshot " + escapeHTML(schedule.snapshotAt || "—") + " · end " + escapeHTML(schedule.endAt || "—") +
      (flags.length ? " · " + flags.join(" · ") : "") + "</p>";
  }
  html += "<div class=\"row\"><button class=\"primary\" data-ab-sched-save=\"" + id + "\" data-write>Save schedule</button></div>" +
    "<div class=\"status-line\" id=\"ab-sched-status-" + id + "\"></div>";
  box.innerHTML = html;
}

el("ab-list").addEventListener("click", async (event) => {
  const resultsBtn = event.target.closest("[data-ab-results]");
  if (resultsBtn) { toggleABResults(resultsBtn.dataset.abResults); return; }
  const schedSave = event.target.closest("[data-ab-sched-save]");
  if (schedSave) {
    const id = schedSave.dataset.abSchedSave;
    const body = {
      switchAt: toRFC3339(el("ab-sched-switch-" + id).value),
      snapshotAt: toRFC3339(el("ab-sched-snapshot-" + id).value),
      endAt: toRFC3339(el("ab-sched-end-" + id).value),
    };
    try {
      await api("POST", "/api/ab-tests/" + id + "/schedule", body);
      setStatus("ab-sched-status-" + id, "Schedule saved", "ok");
      showToast("A/B auto-schedule saved", "ok");
      const data = await api("GET", "/api/ab-tests/" + id + "/results");
      renderABResults(id, data);
    } catch (err) {
      setStatus("ab-sched-status-" + id, friendlyError(err), "error");
      showErrorToast(err, "abtests");
    }
    return;
  }
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
    setStatus("ab-status", friendlyError(err), "error");
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
    setStatus("ab-create-status", friendlyError(err), "error");
  }
});

// ============================================================ wave 6
// Insights: dashboard, scorecards, performers, heatmap, growth and
// goals (items 141-150). Every panel renders its meta line (item
// 150): source, generated-at and window, straight from the API.

function insightMetaLine(meta) {
  if (!meta) return "";
  return "<p class=\"muted-text insight-meta\">Source: " + escapeHTML(meta.source || "local mirror") +
    (meta.generatedAt ? " · Generated " + escapeHTML(meta.generatedAt) : "") +
    (meta.windowDays ? " · Window: last " + meta.windowDays + " days" : "") + "</p>";
}

function insightsChannelId() {
  return el("insights-channel").value || LS.get("scopeChannel", "") || "";
}

async function loadInsights() {
  fillChannelSelects();
  await ensureAccountsLoaded();
  const accountSelect = el("insights-account");
  const accounts = state.accounts || [];
  const currentAccount = accountSelect.value;
  accountSelect.innerHTML = "<option value=\"\">Choose an account…</option>" + accounts.map((account) =>
    "<option value=\"" + account.id + "\">" + escapeHTML(account.label || ("Account " + account.id)) + "</option>").join("");
  if (currentAccount) accountSelect.value = currentAccount;
  if (!accountSelect.value && accounts.length) accountSelect.value = String(accounts[0].id);

  const channelId = insightsChannelId();
  if (channelId) el("insights-channel").value = channelId;
  updateInsightExportLinks();
  if (!channelId) {
    setStatus("insights-status", "Choose a channel to load its insights.", "error");
    return;
  }
  setStatus("insights-status", "Loading insights from the local mirror…");
  await Promise.all([
    loadInsightsDashboard(channelId),
    loadInsightsScorecards(channelId),
    loadInsightsPerformers(channelId),
    loadInsightsHeatmap(channelId),
  ]);
  loadInsightsGrowth();
  loadGoals();
  setStatus("insights-status", "Insights for " + (channelTitle(channelId) || ("channel " + channelId)), "ok");
}

function updateInsightExportLinks() {
  const channelId = insightsChannelId();
  const suffix = channelId ? "&channelId=" + encodeURIComponent(channelId) : "";
  el("insights-export-performers").href = "/api/insights/export?kind=performers" + suffix;
  el("insights-export-scorecards").href = "/api/insights/export?kind=scorecards" + suffix;
}

el("insights-load-btn").addEventListener("click", loadInsights);
el("insights-channel").addEventListener("change", loadInsights);

// Dashboard (item 141).
async function loadInsightsDashboard(channelId) {
  const box = el("insights-dashboard");
  try {
    const data = await api("GET", "/api/insights/dashboard?channelId=" + encodeURIComponent(channelId));
    const months = data.uploadsByMonth || [];
    const maxMonth = Math.max(1, ...months.map((row) => row.count || 0));
    box.innerHTML =
      "<div class=\"stat-grid\">" +
      [["Videos", data.totalVideos || 0], ["Views", data.totalViews || 0], ["Comments", data.totalComments || 0],
       ["Comment coverage", Math.round(data.commentCoveragePct || 0) + "%"],
       ["Schedule adherence", Math.round(data.scheduleAdherencePct || 0) + "% (" + (data.scheduledPublished || 0) + "/" + (data.scheduledTotal || 0) + ")"]]
        .map((row) => "<div class=\"stat\"><span class=\"stat-num\">" + row[1] + "</span><span class=\"stat-label\">" + row[0] + "</span></div>").join("") +
      "</div>" +
      "<h3>Uploads by month</h3>" +
      (months.length ? months.map((row) =>
        "<div class=\"bar-row\"><span class=\"bar-label\">" + escapeHTML(row.month) + "</span>" +
        "<span class=\"bar-track\"><span class=\"bar-fill\" style=\"width:" + Math.round(((row.count || 0) / maxMonth) * 100) + "%\"></span></span>" +
        "<span class=\"bar-value\">" + row.count + "</span></div>").join("")
        : "<p class=\"muted-text\">No uploads in the mirror yet.</p>") +
      "<h3>Views by series</h3>" +
      ((data.viewsBySeries || []).length
        ? "<table class=\"grid\"><thead><tr><th>Series</th><th>Videos</th><th>Views</th><th>Comments</th></tr></thead><tbody>" +
          data.viewsBySeries.map((row) =>
            "<tr><td>" + escapeHTML(row.seriesKey) + "</td><td>" + row.videos + "</td><td>" + row.views + "</td><td>" + row.comments + "</td></tr>").join("") +
          "</tbody></table>"
        : "<p class=\"muted-text\">No series detected on this channel yet.</p>") +
      insightMetaLine(data.meta);
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

// Series scorecards (item 142) with an inline sparkline of views.
function sparklineSVG(values) {
  if (!values || values.length === 0) return "";
  const max = Math.max(1, ...values);
  const width = 120;
  const height = 28;
  const step = values.length > 1 ? width / (values.length - 1) : 0;
  const points = values.map((value, index) =>
    (index * step).toFixed(1) + "," + (height - 3 - (value / max) * (height - 6)).toFixed(1)).join(" ");
  return "<svg class=\"sparkline\" width=\"" + width + "\" height=\"" + height + "\" aria-hidden=\"true\">" +
    "<polyline points=\"" + points + "\" fill=\"none\" stroke=\"var(--accent)\" stroke-width=\"2\"></polyline></svg>";
}

async function loadInsightsScorecards(channelId) {
  const box = el("insights-scorecards");
  try {
    const data = await api("GET", "/api/insights/scorecards?channelId=" + encodeURIComponent(channelId));
    const cards = data.scorecards || [];
    box.innerHTML = (cards.length ? cards.map((card) =>
      "<h3>" + escapeHTML(card.seriesKey) + " " + sparklineSVG((card.episodes || []).map((ep) => ep.views || 0)) + "</h3>" +
      "<table class=\"grid\"><thead><tr><th>Episode</th><th>Title</th><th>Views</th><th>Comments</th></tr></thead><tbody>" +
      (card.episodes || []).map((ep) =>
        "<tr" + (ep.dropoff ? " class=\"is-dropoff\"" : "") + "><td>" + ep.episodeNumber + "</td><td>" +
        escapeHTML(ep.title || "") + (ep.dropoff ? " <span class=\"warn-badge\">drop-off</span>" : "") +
        "</td><td>" + ep.views + "</td><td>" + ep.comments + "</td></tr>").join("") +
      "</tbody></table>").join("")
      : "<p class=\"muted-text\">No series scorecards yet — a series needs at least two episodes in the mirror.</p>") +
      insightMetaLine(data.meta);
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

// Best/worst performers (item 145).
async function loadInsightsPerformers(channelId) {
  const box = el("insights-performers");
  const by = state.insightsBy || "views";
  el("performers-by-views").classList.toggle("is-active", by === "views");
  el("performers-by-comments").classList.toggle("is-active", by === "comments");
  try {
    const data = await api("GET", "/api/insights/performers?channelId=" + encodeURIComponent(channelId) + "&by=" + by);
    const renderList = (title, rows) =>
      "<h3>" + title + "</h3>" + ((rows || []).length
        ? "<ol class=\"performer-list\">" + rows.map((row) =>
          "<li><span class=\"performer-title\">" + escapeHTML(row.title || ("Video " + row.videoId)) + "</span>" +
          "<span class=\"muted-text\">" + (row.views || 0) + " views · " + (row.comments || 0) + " comments</span></li>").join("") + "</ol>"
        : "<p class=\"muted-text\">Nothing here yet.</p>");
    box.innerHTML = renderList("Top 10 by " + by, data.top) + renderList("Bottom 10 by " + by, data.bottom) +
      insightMetaLine(data.meta);
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

el("performers-by-views").addEventListener("click", () => {
  state.insightsBy = "views";
  const channelId = insightsChannelId();
  if (channelId) loadInsightsPerformers(channelId);
});
el("performers-by-comments").addEventListener("click", () => {
  state.insightsBy = "comments";
  const channelId = insightsChannelId();
  if (channelId) loadInsightsPerformers(channelId);
});

// Publish-time heatmap (item 146): weekday rows (Monday first) ×
// hour columns, intensity from the publish count in each cell.
async function loadInsightsHeatmap(channelId) {
  const box = el("insights-heatmap");
  const windowDays = el("heatmap-window").value || "365";
  try {
    const data = await api("GET", "/api/insights/heatmap?channelId=" + encodeURIComponent(channelId) +
      "&windowDays=" + encodeURIComponent(windowDays));
    const cells = new Map();
    let max = 0;
    (data.cells || []).forEach((cell) => {
      cells.set(cell.weekday + ":" + cell.hour, cell);
      if (cell.count > max) max = cell.count;
    });
    const dayNames = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
    const order = [1, 2, 3, 4, 5, 6, 0];
    let html = "<div class=\"heat-wrap\"><div class=\"heat-grid\">";
    html += "<div class=\"heat-corner\"></div>" + Array.from({ length: 24 }, (_, hour) =>
      "<div class=\"heat-hour\">" + hour + "</div>").join("");
    order.forEach((weekday, rowIndex) => {
      html += "<div class=\"heat-day\">" + dayNames[rowIndex] + "</div>";
      for (let hour = 0; hour < 24; hour++) {
        const cell = cells.get(weekday + ":" + hour);
        const count = cell ? cell.count : 0;
        const alpha = count && max ? (0.12 + 0.88 * (count / max)).toFixed(2) : "0";
        html += "<div class=\"heat-cell\" style=\"--heat:" + alpha + "\" title=\"" +
          escapeHTML(dayNames[rowIndex] + " " + String(hour).padStart(2, "0") + ":00 — " + count +
            " publish(es)" + (cell ? ", avg " + Math.round(cell.avgViews || 0) + " views" : "")) + "\">" +
          (count || "") + "</div>";
      }
    });
    html += "</div></div>" + insightMetaLine(data.meta);
    box.innerHTML = html;
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

el("heatmap-window").addEventListener("change", () => {
  const channelId = insightsChannelId();
  if (channelId) loadInsightsHeatmap(channelId);
});

// Growth snapshots (item 147): the diff line is the server's own
// words, shown verbatim above the snapshot table.
async function loadInsightsGrowth() {
  const box = el("insights-growth");
  const accountId = el("insights-account").value;
  if (!accountId) {
    box.innerHTML = "<p class=\"muted-text\">Pick an account to see its snapshots — snapshots are recorded at each sync.</p>";
    return;
  }
  try {
    const data = await api("GET", "/api/insights/growth?accountId=" + encodeURIComponent(accountId));
    const snapshots = data.snapshots || [];
    box.innerHTML = "<p><strong>" + escapeHTML(data.diff || "") + "</strong></p>" +
      (snapshots.length
        ? "<table class=\"grid\"><thead><tr><th>Snapshot</th><th>Videos</th><th>Scheduled</th><th>Views</th><th>Comments</th></tr></thead><tbody>" +
          snapshots.map((point) =>
            "<tr><td>" + escapeHTML(point.at || "") + "</td><td>" + point.videos + "</td><td>" + point.scheduled +
            "</td><td>" + point.views + "</td><td>" + point.comments + "</td></tr>").join("") + "</tbody></table>"
        : "<p class=\"muted-text\">No snapshots yet for this account.</p>") +
      "<p class=\"muted-text insight-meta\">Source: growth snapshots recorded at each sync for this account.</p>";
  } catch (err) {
    box.innerHTML = "<p class=\"status-line error\">" + escapeHTML(friendlyError(err)) + "</p>";
  }
}

el("insights-account").addEventListener("change", loadInsightsGrowth);

// Goal tracking (item 149).
async function loadGoals() {
  const box = el("goals-list");
  try {
    const goals = await api("GET", "/api/goals");
    const list = Array.isArray(goals) ? goals : [];
    if (list.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No goals yet — add one below.</p>";
      return;
    }
    box.innerHTML = list.map((goal) =>
      "<div class=\"goal-row\"><div class=\"goal-head\"><strong>" + escapeHTML(goal.text || "") + "</strong>" +
      " <span class=\"pill\">" + escapeHTML(goal.type || "") + "</span>" +
      " <span class=\"muted-text\">" + escapeHTML(channelTitle(goal.channelId) || ("channel " + goal.channelId)) + "</span>" +
      "<button class=\"goal-del\" data-goal-del=\"" + escapeHTML(goal.id) + "\" data-write>Delete</button></div>" +
      "<div class=\"goal-bar\"><div class=\"goal-fill\" style=\"width:" + Math.min(100, Math.round(goal.progress || 0)) + "%\"></div></div>" +
      "<span class=\"muted-text\">" + Math.round(goal.progress || 0) + "% — " + escapeHTML(goal.detail || "") + "</span></div>").join("");
  } catch (err) {
    box.innerHTML = "<p class=\"muted-text\">Goals could not be loaded.</p>";
  }
}

el("goals-list").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-goal-del]");
  if (!button) return;
  try {
    await api("DELETE", "/api/goals/" + encodeURIComponent(button.dataset.goalDel));
    showToast("Goal deleted", "ok");
    loadGoals();
  } catch (err) {
    showErrorToast(err, "insights");
  }
});

el("goal-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const text = el("goal-text").value.trim();
  const channelId = el("goal-channel").value;
  if (!text || !channelId) { setStatus("goal-status", "A goal needs text and a channel.", "error"); return; }
  const type = el("goal-type").value;
  const body = { text, type, channelId: Number(channelId), target: Number(el("goal-target").value || 0), targetDate: "" };
  if (type === "scheduled-through") {
    body.targetDate = el("goal-date").value || "";
    if (!body.targetDate) { setStatus("goal-status", "Pick a target date for a scheduled-through goal.", "error"); return; }
  }
  try {
    await api("POST", "/api/goals", body);
    setStatus("goal-status", "Goal added", "ok");
    showToast("Goal added", "ok");
    el("goal-text").value = "";
    loadGoals();
  } catch (err) {
    setStatus("goal-status", friendlyError(err), "error");
    showErrorToast(err, "insights");
  }
});

// --------------------------------------------------------------- history
// Global activity timeline (items 58, 63): grouped by day, filterable
// by kind, with undo for schedule entries.
async function loadHistory() {
  setStatus("history-status", "Loading...");
  loadBackups();
  try {
    const path = "/api/activity?limit=100" + (state.historyKind ? "&kind=" + encodeURIComponent(state.historyKind) : "");
    const entries = await api("GET", path);
    renderHistory(entries || []);
    const count = (entries || []).length;
    setStatus("history-status", count + (count === 1 ? " entry" : " entries"), "ok");
  } catch (err) {
    setStatus("history-status", friendlyError(err), "error");
    showErrorToast(err, "history");
  }
}

// Backups (item 98): snapshots taken automatically before big
// writes; restoring one restores the local mirror only.
function backupKeyDate(key) {
  const raw = String(key || "");
  const parsed = new Date(raw);
  if (!isNaN(parsed.getTime()) && /\d{4}/.test(raw)) {
    return parsed.toLocaleString(undefined, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
  }
  return raw;
}

async function loadBackups() {
  const box = el("backups-list");
  if (!box) return;
  try {
    const backups = await api("GET", "/api/backups");
    if (!backups || backups.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No backups yet — one is taken automatically before any apply that touches more than 50 videos.</p>";
      return;
    }
    box.innerHTML = backups.map((backup) =>
      "<div class=\"backup-row\"><span class=\"backup-date\">" + escapeHTML(backupKeyDate(backup.key)) + "</span>" +
      "<span class=\"pill\">" + (backup.videoCount || 0) + " videos</span>" +
      "<button data-restore-backup=\"" + escapeHTML(backup.key) + "\" data-write>Restore local mirror</button></div>").join("");
  } catch (err) {
    box.innerHTML = "<p class=\"muted-text\">Backups could not be loaded.</p>";
  }
}

el("backups-list").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-restore-backup]");
  if (!button) return;
  const key = button.dataset.restoreBackup;
  if (!window.confirm("Restore this backup? It restores yt-manage's local records to the snapshot. " +
    "Titles already changed on YouTube itself are NOT changed back by this — revert those per video with “Undo last rename” in the video drawer.")) return;
  button.disabled = true;
  try {
    const data = await api("POST", "/api/backups/restore", { key });
    showToast(data.note || ("Restored " + (data.restored || 0) + " local record(s)"), "ok");
    setStatus("history-status", "Backup restored: " + (data.restored || 0) + " local record(s)", "ok");
    state.videosLoaded = false;
    loadHistory();
  } catch (err) {
    showErrorToast(err, "history");
    button.disabled = false;
  }
});

function historyDayLabel(iso) {
  const date = new Date(iso);
  if (isNaN(date.getTime())) return "Unknown date";
  if (date.toDateString() === new Date().toDateString()) return "Today";
  const yesterday = new Date(Date.now() - 86400000);
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";
  return date.toLocaleDateString(undefined, { weekday: "short", year: "numeric", month: "short", day: "numeric" });
}

function renderHistory(entries) {
  const box = el("history-list");
  if (entries.length === 0) {
    box.innerHTML = "<div class=\"empty-state\"><span class=\"state-art\" aria-hidden=\"true\">↩</span><h3>No activity yet</h3>" +
      "<p>Syncs, renames, schedules and setting changes are recorded here as they happen — across all accounts.</p></div>";
    return;
  }
  let html = "";
  let lastDay = null;
  entries.forEach((entry) => {
    const day = historyDayLabel(entry.createdAt);
    if (day !== lastDay) {
      html += "<h3 class=\"history-day\">" + escapeHTML(day) + "</h3>";
      lastDay = day;
    }
    const time = entry.createdAt
      ? new Date(entry.createdAt).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
      : "";
    html += "<div class=\"history-item" + (entry.isUndone ? " is-undone" : "") + "\">" +
      "<span class=\"history-time\">" + escapeHTML(time) + "</span>" +
      "<span class=\"pill history-kind\">" + escapeHTML(entry.kind || "") + "</span>" +
      "<span class=\"history-summary\">" + escapeHTML(entry.summary || "") +
      (entry.detail ? "<span class=\"history-detail\">" + escapeHTML(entry.detail) + "</span>" : "") +
      (entry.kind === "rename" && !entry.isUndone ? "<span class=\"history-note\">Undo from the video drawer</span>" : "") +
      (entry.isUndone ? "<span class=\"history-note\">Undone</span>" : "") +
      "</span>" +
      (entry.canUndo && entry.kind === "schedule" && !entry.isUndone
        ? "<button data-undo-activity=\"" + entry.id + "\" data-write>Undo schedule</button>" : "") +
      "</div>";
  });
  box.innerHTML = html;
}

el("history-load-btn").addEventListener("click", loadHistory);

el("history-kind-chips").addEventListener("click", (event) => {
  const chip = event.target.closest("[data-kind]");
  if (!chip) return;
  state.historyKind = chip.dataset.kind || "";
  document.querySelectorAll("#history-kind-chips .chip").forEach((item) =>
    item.classList.toggle("is-active", item === chip));
  loadHistory();
});

el("history-list").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-undo-activity]");
  if (!button) return;
  button.disabled = true;
  try {
    await api("POST", "/api/activity/" + button.dataset.undoActivity + "/undo");
    showToast("Schedule undone", "ok");
    await loadHistory();
    loadStatus();
  } catch (err) {
    showErrorToast(err, "history");
    button.disabled = false;
  }
});

// --------------------------------------------------------------- settings
// A real settings screen (items 51-60): grouped rows rendered from
// /api/settings, applied instantly, with defaults shown, per-account
// overrides, fenced secrets, and export/import.
function isSecretSetting(setting) {
  return setting.type === "secret" || setting.type === "password";
}

function settingValueString(setting) {
  return setting.value === undefined || setting.value === null ? "" : String(setting.value);
}

function settingDefaultString(setting) {
  return setting.default === undefined || setting.default === null ? "" : String(setting.default);
}

function settingChanged(setting) {
  if (isSecretSetting(setting)) return !!setting.isSet;
  return settingValueString(setting) !== settingDefaultString(setting);
}

async function loadSettings() {
  setStatus("settings-status", "Loading settings...");
  try {
    if (!state.accounts) {
      try { state.accounts = await api("GET", "/api/accounts"); } catch (err) { state.accounts = []; }
    }
    state.settingsData = await api("GET", "/api/settings");
    renderSettings();
    fillChannelSelects();
    if (el("cd-channel").value) loadChannelDefaultsCard();
    setStatus("settings-status", "");
  } catch (err) {
    setStatus("settings-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
}

function renderSettings() {
  const data = state.settingsData;
  if (!data) return;
  const groupsBox = el("settings-groups");
  const advBox = el("settings-advanced-rows");
  groupsBox.innerHTML = "";
  advBox.innerHTML = "";
  const groups = new Map();
  (data.settings || []).forEach((setting) => {
    if (isSecretSetting(setting)) {
      advBox.appendChild(settingRow(setting));
      return;
    }
    const group = setting.group || "General";
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(setting);
  });
  groups.forEach((rows, name) => {
    const card = document.createElement("div");
    card.className = "card setting-group";
    card.innerHTML = "<h2>" + escapeHTML(name) + "</h2>";
    rows.forEach((setting) => card.appendChild(settingRow(setting)));
    groupsBox.appendChild(card);
  });
  if (advBox.children.length === 0) {
    advBox.innerHTML = "<p class=\"muted-text\">No stored secrets.</p>";
  }
  renderPerAccountSettings();
  applySettingsSearch();
  renderQuota(data);
}

function settingRow(setting) {
  const row = document.createElement("div");
  row.className = "setting-row";
  row.dataset.key = setting.key;
  row.dataset.serverValue = isSecretSetting(setting) ? "" : settingValueString(setting);
  row.dataset.search = ((setting.label || "") + " " + setting.key + " " + (setting.group || "")).toLowerCase();
  const changed = settingChanged(setting);
  let control = "";
  if (isSecretSetting(setting)) {
    control = "<input type=\"password\" class=\"setting-input\" autocomplete=\"new-password\" placeholder=\"" +
      (setting.isSet ? "•••••••• (stored)" : "Not set") + "\">";
  } else if (setting.type === "number" || setting.type === "int" || setting.type === "port") {
    control = "<input type=\"number\" class=\"setting-input\" value=\"" + escapeHTML(settingValueString(setting)) + "\">";
  } else if (setting.type === "bool" || setting.type === "boolean") {
    control = "<input type=\"checkbox\" class=\"setting-input\"" + (settingValueString(setting) === "true" ? " checked" : "") + ">";
  } else {
    control = "<input type=\"text\" class=\"setting-input\" value=\"" + escapeHTML(settingValueString(setting)) + "\">";
  }
  row.innerHTML =
    "<div class=\"setting-head\"><span class=\"setting-label\">" + escapeHTML(setting.label || setting.key) + "</span>" +
    "<span class=\"setting-changed pill" + (changed ? "" : " hidden") + "\">changed</span>" +
    "<span class=\"setting-saved hidden\">Saved ✓</span></div>" +
    "<div class=\"setting-controls\">" + control +
    "<button class=\"setting-reset" + (changed ? "" : " hidden") + "\">Reset to default</button></div>" +
    "<div class=\"setting-meta\">Default: " + escapeHTML(settingDefaultString(setting) || "(empty)") +
    (isSecretSetting(setting)
      ? " · " + (setting.isSet ? "A value is stored — it is never shown" : "No value stored")
      : "") +
    " · <code>" + escapeHTML(setting.key) + "</code></div>" +
    "<div class=\"setting-error hidden\"></div>";
  wireSettingRow(row, setting);
  return row;
}

function wireSettingRow(row, setting) {
  const input = row.querySelector(".setting-input");
  let timer = null;
  const save = () => saveSetting(row, setting);
  if (input.type === "checkbox") {
    input.addEventListener("change", save);
    return;
  }
  input.addEventListener("input", () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(save, 600);
  });
  input.addEventListener("change", () => {
    if (timer) clearTimeout(timer);
    save();
  });
  row.querySelector(".setting-reset").addEventListener("click", () => resetSetting(row, setting));
}

function flashSaved(row) {
  const tick = row.querySelector(".setting-saved");
  if (!tick) return;
  tick.classList.remove("hidden");
  setTimeout(() => tick.classList.add("hidden"), 1600);
}

function showSettingError(row, err) {
  const box = row.querySelector(".setting-error");
  if (!box) return;
  box.textContent = friendlyError(err);
  box.classList.remove("hidden");
}

function clearSettingError(row) {
  const box = row.querySelector(".setting-error");
  if (!box) return;
  box.textContent = "";
  box.classList.add("hidden");
}

function refreshRowFromData(row, key) {
  const updated = (state.settingsData.settings || []).find((item) => item.key === key);
  if (!updated) return;
  row.dataset.serverValue = isSecretSetting(updated) ? "" : settingValueString(updated);
  const changed = settingChanged(updated);
  row.querySelector(".setting-changed").classList.toggle("hidden", !changed);
  row.querySelector(".setting-reset").classList.toggle("hidden", !changed);
}

async function saveSetting(row, setting) {
  if (isReadOnly()) {
    showErrorToast(new Error(READ_ONLY_HINT), "settings");
    renderSettings();
    return;
  }
  const input = row.querySelector(".setting-input");
  let value;
  if (input.type === "checkbox") value = input.checked ? "true" : "false";
  else value = input.value;
  if (isSecretSetting(setting)) {
    if (!value) return;
    if (!window.confirm("Change the stored value for “" + (setting.label || setting.key) + "”? The old value cannot be recovered.")) {
      input.value = "";
      return;
    }
  } else if (row.dataset.lastSent === value) {
    return;
  }
  row.dataset.lastSent = value;
  clearSettingError(row);
  try {
    const data = await api("PUT", "/api/settings", { key: setting.key, value: String(value) });
    state.settingsData = data;
    if (isSecretSetting(setting)) {
      input.value = "";
      input.placeholder = "•••••••• (stored)";
    }
    refreshRowFromData(row, setting.key);
    flashSaved(row);
    renderQuota(data);
  } catch (err) {
    delete row.dataset.lastSent;
    if (input.type === "checkbox") input.checked = row.dataset.serverValue === "true";
    else if (!isSecretSetting(setting)) input.value = row.dataset.serverValue || "";
    else input.value = "";
    showSettingError(row, err);
    showErrorToast(err, "settings");
  }
}

async function resetSetting(row, setting) {
  clearSettingError(row);
  try {
    const data = await api("POST", "/api/settings/reset", { key: setting.key });
    state.settingsData = data;
    const updated = (data.settings || []).find((item) => item.key === setting.key);
    if (updated) {
      const fresh = settingRow(updated);
      row.replaceWith(fresh);
      flashSaved(fresh);
    }
    renderQuota(data);
    showToast("“" + (setting.label || setting.key) + "” reset to default", "ok");
  } catch (err) {
    showSettingError(row, err);
    showErrorToast(err, "settings");
  }
}

// Per-account overrides (item 54): notify-on-publish + sync interval,
// "inherited" while both are at their inherit values.
function renderPerAccountSettings() {
  const box = el("settings-per-account");
  const data = state.settingsData;
  if (!data) { box.innerHTML = ""; return; }
  const accounts = state.accounts || [];
  if (accounts.length === 0) {
    box.innerHTML = "<p class=\"muted-text\">No accounts connected yet — connect one and its overrides appear here.</p>";
    return;
  }
  const per = data.perAccount || {};
  box.innerHTML = "";
  accounts.forEach((account) => {
    const cfg = per[String(account.id)] || per[account.id] || {};
    const notify = !!cfg.notifyOnPublish;
    const interval = Number(cfg.syncIntervalMin || 0);
    const row = document.createElement("div");
    row.className = "setting-row per-account-row";
    row.innerHTML =
      "<div class=\"setting-head\"><span class=\"setting-label\">" + escapeHTML(account.label || ("Account " + account.id)) + "</span>" +
      "<span class=\"setting-changed pill inherited-tag" + (!notify && interval === 0 ? "" : " hidden") + "\">inherited</span>" +
      "<span class=\"setting-saved hidden\">Saved ✓</span></div>" +
      "<div class=\"setting-controls per-account-controls\">" +
      "<label class=\"check\"><input type=\"checkbox\" class=\"pa-notify\"" + (notify ? " checked" : "") + "> Notify on publish</label>" +
      "<label class=\"pa-interval-label\">Sync interval (minutes; 0 = inherited) " +
      "<input type=\"number\" class=\"pa-interval\" min=\"0\" step=\"1\" value=\"" + interval + "\"></label></div>" +
      "<div class=\"setting-error hidden\"></div>";
    const notifyBox = row.querySelector(".pa-notify");
    const intervalInput = row.querySelector(".pa-interval");
    notifyBox.addEventListener("change", () =>
      savePerAccount(row, account.id, "notify_on_publish", notifyBox.checked ? "true" : "false"));
    let timer = null;
    intervalInput.addEventListener("input", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        const minutes = Math.max(0, Number(intervalInput.value || 0));
        savePerAccount(row, account.id, "sync_interval_min", String(minutes));
      }, 600);
    });
    box.appendChild(row);
  });
}

async function savePerAccount(row, accountId, key, value) {
  if (isReadOnly()) {
    showErrorToast(new Error(READ_ONLY_HINT), "settings");
    renderPerAccountSettings();
    return;
  }
  clearSettingError(row);
  try {
    const data = await api("PUT", "/api/settings", { key, value: String(value), accountId });
    state.settingsData = data;
    const notify = row.querySelector(".pa-notify").checked;
    const interval = Number(row.querySelector(".pa-interval").value || 0);
    row.querySelector(".inherited-tag").classList.toggle("hidden", !(!notify && interval === 0));
    flashSaved(row);
  } catch (err) {
    showSettingError(row, err);
    showErrorToast(err, "settings");
    renderPerAccountSettings();
  }
}

// Settings search (item 53): filter rows by label, key or group.
function applySettingsSearch() {
  const input = el("settings-search");
  if (!input) return;
  const query = input.value.trim().toLowerCase();
  document.querySelectorAll("#settings-groups .setting-row, #settings-advanced-rows .setting-row").forEach((row) => {
    row.classList.toggle("hidden", !!query && (row.dataset.search || "").indexOf(query) === -1);
  });
  document.querySelectorAll("#settings-groups .setting-group").forEach((card) => {
    const anyVisible = Array.from(card.querySelectorAll(".setting-row")).some((row) => !row.classList.contains("hidden"));
    card.classList.toggle("hidden", !!query && !anyVisible);
  });
  if (query) {
    const advMatch = Array.from(el("settings-advanced-rows").querySelectorAll(".setting-row"))
      .some((row) => !row.classList.contains("hidden"));
    if (advMatch) el("settings-advanced").open = true;
  }
}

el("settings-search").addEventListener("input", applySettingsSearch);

// Export / import (item 57): secrets are excluded by the server and
// the exclusion is surfaced, never silent.
el("settings-export-btn").addEventListener("click", async () => {
  try {
    const data = await api("POST", "/api/settings/export", {});
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "yt-manage-settings.json";
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 5000);
    const excluded = data.excluded || [];
    const note = excluded.length
      ? "Settings exported. Excluded secrets (never exported): " + excluded.join(", ")
      : "Settings exported.";
    setStatus("settings-status", note, "ok");
    showToast(note, "ok");
  } catch (err) {
    setStatus("settings-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
});

el("settings-import-btn").addEventListener("click", () => el("settings-import-file").click());

el("settings-import-file").addEventListener("change", async () => {
  const file = el("settings-import-file").files[0];
  el("settings-import-file").value = "";
  if (!file) return;
  let parsed;
  try {
    parsed = JSON.parse(await file.text());
  } catch (err) {
    setStatus("settings-status", "That file is not valid JSON — nothing was imported.", "error");
    return;
  }
  const settings = parsed.settings || parsed;
  const excluded = parsed.excluded || [];
  if (!window.confirm("Import these settings? This overwrites your current settings. Secrets are never imported.")) return;
  try {
    state.settingsData = await api("POST", "/api/settings/import", { settings });
    renderSettings();
    const note = "Settings imported." + (excluded.length
      ? " Excluded from the file and not imported: " + excluded.join(", ") + "."
      : " Note: secrets such as mail.password are never imported.");
    setStatus("settings-status", note, "ok");
    showToast(note, "ok");
  } catch (err) {
    setStatus("settings-status", friendlyError(err), "error");
    showErrorToast(err, "settings");
  }
});

// --------------------------------------------------------------- accounts
async function loadAccounts() {
  setStatus("account-status", "Loading...");
  try {
    const accounts = await api("GET", "/api/accounts");
    state.accounts = accounts;
    const rows = [];
    accounts.forEach((account) => {
      if (!account.channels || account.channels.length === 0) {
        rows.push("<tr><td>" + account.id + "</td><td>" + avatarHTML(account.label) + escapeHTML(account.label) +
          "</td><td colspan=\"4\">No channels</td></tr>");
        return;
      }
      account.channels.forEach((channel) => {
        rows.push("<tr><td>" + account.id + "</td><td>" + avatarHTML(account.label) + escapeHTML(account.label) + "</td><td>" +
          avatarHTML(channel.title) + escapeHTML(channel.title) + "</td><td>" + escapeHTML(channel.ytChannelId) + "</td><td>" +
          channel.subscriberCount + "</td><td>" + channel.videoCount + "</td></tr>");
      });
    });
    el("account-rows").innerHTML = rows.join("");
    renderAccountHealth(accounts);
    el("accounts-empty").innerHTML = accounts.length === 0
      ? "<div class=\"empty-state\"><span class=\"state-art\" aria-hidden=\"true\">◉</span><h3>No accounts connected</h3>" +
        "<p>Connect your first YouTube account from a terminal: <code>yt-manage account add</code> — it opens the Google consent flow, and the token is stored encrypted on this machine.</p></div>"
      : "";
    setStatus("account-status", accounts.length + " account(s)", "ok");
  } catch (err) {
    setStatus("account-status", friendlyError(err), "error");
  }
}

// Token health (item 96): how old each account's saved sign-in is
// and whether it is nearing the ~7-day Testing-mode expiry.
function renderAccountHealth(accounts) {
  const box = el("accounts-health");
  if (!box) return;
  if (!accounts || accounts.length === 0) { box.innerHTML = ""; return; }
  box.innerHTML = "<div class=\"card\"><h2>Token health</h2>" + accounts.map((account) => {
    const updated = account.updatedAt ? new Date(account.updatedAt) : null;
    let line;
    if (!updated || isNaN(updated.getTime())) {
      line = "<span class=\"muted-text\">Token age unknown — reconnect to refresh it.</span>";
    } else {
      const days = Math.floor((Date.now() - updated.getTime()) / 86400000);
      const when = days <= 0 ? "today" : days === 1 ? "1 day ago" : days + " days ago";
      line = days > 6
        ? "<span class=\"health-warn\">⚠ Token saved " + when + " — Google Testing-mode tokens expire after about 7 days, so reconnect soon.</span>"
        : "<span class=\"muted-text\">Token saved " + when + ".</span>";
    }
    return "<div class=\"health-row\">" + avatarHTML(account.label) +
      "<span class=\"health-label\">" + escapeHTML(account.label || ("Account " + account.id)) + "</span>" +
      line +
      "<span class=\"health-action\">Reconnect: <code>yt-manage account add</code></span></div>";
  }).join("") + "</div>";
}

el("accounts-load-btn").addEventListener("click", loadAccounts);

// ------------------------------------------- keyboard map & palette
// Full keyboard map (item 71): "g" then a letter navigates, "?"
// opens the shortcut overlay, j/k walk the video list, shift+arrows
// extend the selection. Ctrl/Cmd+K opens the command palette
// (item 72). Typing targets are ignored (except Esc, handled by the
// drawer/palette closers).
const G_NAV = { h: "home", v: "videos", e: "episodes", p: "playlists", a: "accounts", c: "history", s: "settings" };
let pendingGAt = 0;

function shortcutsOpen() { return !el("shortcuts-overlay").classList.contains("hidden"); }
function paletteOpen() { return !el("palette-overlay").classList.contains("hidden"); }

function openShortcuts() { el("shortcuts-overlay").classList.remove("hidden"); el("shortcuts-close").focus(); }
function closeShortcuts() { el("shortcuts-overlay").classList.add("hidden"); }
el("shortcuts-close").addEventListener("click", closeShortcuts);
el("shortcuts-overlay").addEventListener("click", (event) => {
  if (event.target === el("shortcuts-overlay")) closeShortcuts();
});

const PALETTE_COMMANDS = [
  { label: "Go to Home", run: () => navigate("home") },
  { label: "Go to Suggestions", run: () => navigate("suggestions") },
  { label: "Go to Videos", run: () => navigate("videos") },
  { label: "Go to Episodes", run: () => navigate("episodes") },
  { label: "Go to Automation", run: () => navigate("automation") },
  { label: "Go to Comment Centre", run: () => navigate("commentcentre") },
  { label: "Go to Playlists", run: () => navigate("playlists") },
  { label: "Go to Insights", run: () => navigate("insights") },
  { label: "Go to Accounts", run: () => navigate("accounts") },
  { label: "Go to History", run: () => navigate("history") },
  { label: "Go to Settings", run: () => navigate("settings") },
  { label: "Sync now", run: () => runSync(el("sync-all-btn"), "sync-progress", "sync-result") },
  { label: "Seed demo channel", run: () => el("demo-seed-btn").click() },
  { label: "Clear demo channel", run: () => el("demo-clear-btn").click() },
  { label: "Toggle density", run: () => el("density-toggle").click() },
  { label: "Toggle theme (system / light / dark)", run: () => cycleTheme() },
  { label: "Export settings", run: () => el("settings-export-btn").click() },
  {
    label: "Export audit CSV", run: () => {
      const link = document.createElement("a");
      link.href = "/api/activity/export";
      link.setAttribute("download", "");
      document.body.appendChild(link);
      link.click();
      link.remove();
    },
  },
  { label: "Open keyboard shortcuts", run: () => openShortcuts() },
];

function fuzzyMatch(text, query) {
  const hay = text.toLowerCase();
  const needle = query.toLowerCase();
  let hi = 0;
  for (let ni = 0; ni < needle.length; ni++) {
    hi = hay.indexOf(needle[ni], hi);
    if (hi === -1) return false;
    hi++;
  }
  return true;
}

function renderPalette() {
  const query = el("palette-input").value.trim();
  state.paletteItems = PALETTE_COMMANDS.filter((command) => !query || fuzzyMatch(command.label, query));
  state.paletteIndex = Math.max(0, Math.min(state.paletteIndex, state.paletteItems.length - 1));
  el("palette-list").innerHTML = state.paletteItems.map((command, index) =>
    "<button class=\"palette-item" + (index === state.paletteIndex ? " is-active" : "") +
    "\" role=\"option\" data-palette-index=\"" + index + "\">" + escapeHTML(command.label) + "</button>").join("") ||
    "<p class=\"muted-text palette-empty\">No matching command</p>";
}

function openPalette() {
  el("palette-overlay").classList.remove("hidden");
  el("palette-input").value = "";
  state.paletteIndex = 0;
  renderPalette();
  el("palette-input").focus();
}

function closePalette() { el("palette-overlay").classList.add("hidden"); }

function runPaletteItem(index) {
  const command = state.paletteItems[index];
  closePalette();
  if (command) command.run();
}

el("palette-input").addEventListener("input", () => { state.paletteIndex = 0; renderPalette(); });
el("palette-input").addEventListener("keydown", (event) => {
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    state.paletteIndex = Math.max(0, Math.min(state.paletteIndex + (event.key === "ArrowDown" ? 1 : -1), state.paletteItems.length - 1));
    renderPalette();
  } else if (event.key === "Enter") {
    event.preventDefault();
    runPaletteItem(state.paletteIndex);
  }
});
el("palette-list").addEventListener("click", (event) => {
  const item = event.target.closest("[data-palette-index]");
  if (item) runPaletteItem(Number(item.dataset.paletteIndex));
});
el("palette-overlay").addEventListener("click", (event) => {
  if (event.target === el("palette-overlay")) closePalette();
});

document.addEventListener("keydown", (event) => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    if (paletteOpen()) closePalette(); else openPalette();
    return;
  }
  if (event.key === "Escape") {
    if (paletteOpen()) { closePalette(); return; }
    if (shortcutsOpen()) { closeShortcuts(); return; }
  }
  if (isTypingTarget(event.target)) return;
  if (event.key === "?") {
    event.preventDefault();
    if (shortcutsOpen()) closeShortcuts(); else openShortcuts();
    return;
  }
  if (paletteOpen() || shortcutsOpen()) return;
  const now = Date.now();
  if (pendingGAt && now - pendingGAt <= 700) {
    const target = G_NAV[event.key.toLowerCase()];
    pendingGAt = 0;
    if (target) {
      event.preventDefault();
      navigate(target);
    }
    return;
  }
  pendingGAt = 0;
  if (event.key.toLowerCase() === "g" && !event.ctrlKey && !event.metaKey && !event.altKey) {
    pendingGAt = now;
    return;
  }
  if (state.section === "videos") {
    if (event.key === "j" || event.key === "k") {
      setActiveVideo(state.activeVideoIndex + (event.key === "j" ? 1 : -1));
      return;
    }
    // Shift+arrows extend the selection; the plain-arrow handler has
    // already moved the active row by the time this listener runs.
    if (event.shiftKey && (event.key === "ArrowDown" || event.key === "ArrowUp") && state.activeVideoIndex >= 0) {
      const anchor = state.selectionAnchor >= 0 ? state.selectionAnchor : state.activeVideoIndex;
      selectVideoRange(anchor, state.activeVideoIndex, true);
      if (state.selectionAnchor < 0) state.selectionAnchor = anchor;
      renderVideos();
    }
  }
});

// ------------------------------------------------ read-only guards
// While read-only mode is on (item 95), write controls look
// disabled and any activation is stopped with an explanation.
const READ_ONLY_HINT = "Read-only mode is on — writes are blocked. Turn it off with: yt-manage settings set read_only false";

document.addEventListener("click", (event) => {
  if (!isReadOnly()) return;
  if (event.target.closest("[data-write]")) {
    event.preventDefault();
    event.stopPropagation();
    showToast(READ_ONLY_HINT, "error");
  }
}, true);

document.addEventListener("submit", (event) => {
  if (!isReadOnly()) return;
  if (["schedule-form", "playlist-create-form", "ab-create-form", "video-edit-form"].indexOf(event.target.id) !== -1) {
    event.preventDefault();
    event.stopPropagation();
    showToast(READ_ONLY_HINT, "error");
  }
}, true);

// Cold start & prefetch (items 79, 80): once the first status has
// painted, warm the section endpoints during idle time so the
// first navigation to each screen opens instantly.
function prefetchSections() {
  const warm = () => {
    api("GET", "/api/schedules").catch(() => {});
    api("GET", "/api/accounts").then((accounts) => {
      if (!state.accounts) state.accounts = accounts;
    }).catch(() => {});
    api("GET", "/api/settings").catch(() => {});
    const scope = LS.get("scopeChannel", "");
    if (scope) {
      api("GET", "/api/episodes?channel=" + encodeURIComponent(scope)).catch(() => {});
      api("GET", "/api/playlists?channel=" + encodeURIComponent(scope)).catch(() => {});
    }
  };
  if (window.requestIdleCallback) window.requestIdleCallback(warm, { timeout: 2500 });
  else setTimeout(warm, 800);
}

// ------------------------------------------------------------------- init
applyTheme();
applyDensity(LS.get("density", "comfortable"));
applySidebar(LS.get("sidebarCollapsed", false));
el("video-sort").value = LS.get("videoSort", "newest");
setVisibility("", false);
setToggleChips(false, false, false);

(async function init() {
  await loadStatus();
  if (state.status && !state.status.appVersion) {
    try {
      const version = await api("GET", "/api/version");
      if (version && version.version) el("footer-version").textContent = "yt-manage v" + version.version;
    } catch (err) { /* footer keeps the plain name */ }
  }
  renderRecents();
  loadSavedSearches();
  loadRecipes();
  updateBulkPreview();
  prefetchSections();
  const parsed = parseHash();
  if (parsed) {
    if (parsed.section === "videos") applyVideoParams(parsed.params);
    navigate(parsed.section, true);
  } else {
    navigate(LS.get("lastSection", "home"), true);
  }
})();

// ----------------------------------------------------- tags manager
// Item 140: all tags on a channel with counts, from the mirror
// (tags are mirrored on sync). Rename/merge/remove previews the
// affected video count before anything is sent to YouTube.
async function loadTags() {
  const channelId = el("tags-channel").value;
  const box = el("tags-list");
  if (!channelId) { setStatus("tags-status", "Choose a channel first.", "error"); return; }
  setStatus("tags-status", "Loading tags…");
  try {
    const tags = await api("GET", "/api/tags?channelId=" + encodeURIComponent(channelId));
    const list = Array.isArray(tags) ? tags : [];
    box.dataset.channel = channelId;
    box.dataset.tags = JSON.stringify(list);
    setStatus("tags-status", list.length + " tag(s) on this channel", "ok");
    if (list.length === 0) {
      box.innerHTML = "<p class=\"muted-text\">No tags mirrored yet — run a sync first; tags arrive with it.</p>";
      return;
    }
    box.innerHTML = list.map((entry, index) =>
      "<div class=\"tag-row\"><strong>" + escapeHTML(entry.tag) + "</strong>" +
      " <span class=\"pill\">" + entry.count + " video(s)</span>" +
      " <button data-tag-rename=\"" + index + "\" data-write>Rename / merge / remove…</button>" +
      "<div class=\"tag-rename-box hidden\" id=\"tag-rename-" + index + "\">" +
      "<input type=\"text\" id=\"tag-new-name-" + index + "\" placeholder=\"New tag name (empty = remove)\">" +
      "<p class=\"muted-text\" id=\"tag-preview-" + index + "\"></p>" +
      "<button data-tag-apply=\"" + index + "\" class=\"primary\" data-write>Apply</button>" +
      " <button data-tag-cancel=\"" + index + "\">Cancel</button>" +
      "<div class=\"status-line\" id=\"tag-status-" + index + "\"></div></div></div>").join("");
  } catch (err) {
    setStatus("tags-status", friendlyError(err), "error");
    showErrorToast(err, "videos");
  }
}

el("tags-load-btn").addEventListener("click", loadTags);

el("tags-list").addEventListener("click", async (event) => {
  const box = el("tags-list");
  const channelId = box.dataset.channel;
  let tags = [];
  try { tags = JSON.parse(box.dataset.tags || "[]"); } catch (err) { tags = []; }
  const openBtn = event.target.closest("[data-tag-rename]");
  if (openBtn) {
    const index = Number(openBtn.dataset.tagRename);
    const entry = tags[index];
    const panel = el("tag-rename-" + index);
    if (!entry || !panel) return;
    panel.classList.remove("hidden");
    el("tag-preview-" + index).textContent =
      "Preview: “" + entry.tag + "” is on " + entry.count + " video(s). Applying updates each video on YouTube; an existing target name merges the tags.";
    return;
  }
  const cancelBtn = event.target.closest("[data-tag-cancel]");
  if (cancelBtn) {
    el("tag-rename-" + Number(cancelBtn.dataset.tagCancel)).classList.add("hidden");
    return;
  }
  const applyBtn = event.target.closest("[data-tag-apply]");
  if (applyBtn) {
    const index = Number(applyBtn.dataset.tagApply);
    const entry = tags[index];
    if (!entry || !channelId) return;
    const to = el("tag-new-name-" + index).value.trim();
    setStatus("tag-status-" + index, "Applying on YouTube…");
    try {
      const result = await api("POST", "/api/tags/rename", {
        channelId: Number(channelId), from: entry.tag, to: to,
      });
      const failures = (result && result.failures) || [];
      setStatus("tag-status-" + index,
        "Updated " + (result ? result.changed : 0) + " video(s)" +
        (failures.length ? "; failed: " + failures.join("; ") : "") + ".",
        failures.length ? "error" : "ok");
      showToast("Tag “" + entry.tag + "” updated on " + (result ? result.changed : 0) + " video(s)", "ok");
      loadTags();
    } catch (err) {
      setStatus("tag-status-" + index, friendlyError(err), "error");
      showErrorToast(err, "videos");
    }
  }
});
