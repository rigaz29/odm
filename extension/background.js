// ODM Integration: hands browser downloads and detected videos to Ochi
// Download Manager through native messaging. The pure helpers at the top
// carry no browser state, so extension/test/ runs them under Node.

const HOST = "com.ochi.odm";

// Used until ODM answers a "config" message (ODM not running yet, or an
// older ODM): the same built-in list ODM ships with.
const DEFAULT_TYPES = [
  "3gp", "7z", "aac", "aiff", "apk", "appx", "avi", "bat", "bmp", "bz2", "csv", "deb", "dmg",
  "doc", "docx", "epub", "exe", "flac", "flv", "gif", "gz", "heic", "ico", "img", "iso", "jpeg",
  "jpg", "m4a", "m4v", "mkv", "mov", "mp3", "mp4", "mpeg", "mpg", "msi", "mts", "odt", "ogg",
  "opus", "pdf", "pkg", "png", "ppt", "pptx", "rar", "rpm", "rtf", "sh", "svg", "tar", "tiff",
  "ts", "txt", "vhd", "vob", "wav", "webm", "webp", "wma", "wmv", "xls", "xlsx", "xz", "zip", "zst",
];

// Smaller media responses are ads, previews or interface sounds.
const MIN_MEDIA_BYTES = 512 * 1024;
const CONFIG_TTL_MS = 5 * 60 * 1000;
const MAX_MEDIA_PER_TAB = 15;
const BYPASS_MS = 10 * 1000;
const STREAM_INFO_MS = 2 * 60 * 1000;
// Chromium names a download within milliseconds, unless it asks where to
// save it: then the name waits for the Save As dialog.
const NAME_WAIT_MS = 2 * 60 * 1000;

// pruneBypass keeps the Alt-click marks younger than BYPASS_MS. Older ones
// no longer apply, and dropping them keeps the session map from growing
// with every link Alt-clicked since the browser started.
function pruneBypass(bypass, now) {
  const out = {};
  for (const [url, at] of Object.entries(bypass || {})) {
    if (now - at < BYPASS_MS) out[url] = at;
  }
  return out;
}

const STREAM_TYPES = [
  "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl",
  "application/dash+xml",
];
const MEDIA_EXT = /\.(mp4|m4v|webm|mkv|mov|flv|mp3|m4a|aac|ogg|opus|flac|wav)$/i;

/* ---------- pure helpers ---------- */

function urlExt(url) {
  try {
    const m = /\.([a-z0-9]{1,8})$/i.exec(new URL(url).pathname);
    return m ? m[1].toLowerCase() : "";
  } catch {
    return "";
  }
}

// hasType reports whether url (or the browser's file name) ends in one of
// types, given without dots.
function hasType(url, types, filename) {
  const set = new Set(types.map((t) => String(t).toLowerCase().replace(/^\./, "")));
  const fromName = /\.([a-z0-9]{1,8})$/i.exec(filename || "");
  return set.has(urlExt(url)) || (fromName !== null && set.has(fromName[1].toLowerCase()));
}

// settledFilename resolves to the file name the browser gives download id.
// Chromium creates the item before it has read Content-Disposition, so at
// onCreated the name is still "" and a link such as download.php?id=7 says
// nothing of the type. "" when the download ends first or timeoutMs
// passes. downloads is chrome.downloads (a stand-in in the tests).
function settledFilename(downloads, id, timeoutMs) {
  return new Promise((resolve) => {
    let timer;
    let settled = false;
    const finish = (name) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      downloads.onChanged.removeListener(onChanged);
      resolve(name || "");
    };
    const onChanged = (d) => {
      if (d.id !== id) return;
      if (d.filename && d.filename.current) finish(d.filename.current);
      else if (d.state && d.state.current !== "in_progress") finish("");
    };
    timer = setTimeout(() => finish(""), timeoutMs);
    downloads.onChanged.addListener(onChanged);
    // The name may have come before the listener did.
    Promise.resolve(downloads.search({ id })).then(([it] = []) => {
      if (it && it.filename) finish(it.filename);
      else if (it && it.state !== "in_progress") finish("");
    }, () => {});
  });
}

// hostMatches: "example.org" matches that host; "*.example.org" matches it
// and every subdomain.
function hostMatches(host, pattern) {
  host = String(host).toLowerCase();
  pattern = String(pattern).toLowerCase().trim();
  if (!pattern) return false;
  if (pattern.startsWith("*.")) {
    const base = pattern.slice(2);
    return host === base || host.endsWith("." + base);
  }
  return host === pattern;
}

function isExcluded(url, patterns) {
  try {
    const host = new URL(url).hostname;
    return (patterns || []).some((p) => hostMatches(host, p));
  } catch {
    return false;
  }
}

// mediaKind classifies a response: "stream" for HLS/DASH playlists (ODM
// fetches their segments), "file" for a single audio/video file, null for
// anything else.
function mediaKind(url, mime) {
  mime = String(mime || "").split(";")[0].trim().toLowerCase();
  let path = "";
  try {
    path = new URL(url).pathname.toLowerCase();
  } catch {
    return null;
  }
  // YouTube's player fetches signed pieces of separate video and audio;
  // none is the video. Its page goes to yt-dlp instead (sitePage).
  try {
    if (/(^|\.)googlevideo\.com$/i.test(new URL(url).hostname)) return null;
  } catch {
    return null;
  }
  if (STREAM_TYPES.includes(mime) || path.endsWith(".m3u8") || path.endsWith(".mpd")) return "stream";
  // Segments of a stream, not whole files: MPEG-TS (HLS) and fragmented
  // MP4 (DASH, newer HLS). Listing them would also push the playlists out.
  if (mime === "video/mp2t" || path.endsWith(".ts")) return null;
  if (mime === "video/iso.segment" || mime === "audio/iso.segment" || path.endsWith(".m4s")) return null;
  if (mime.startsWith("video/") || mime.startsWith("audio/")) return "file";
  if (MEDIA_EXT.test(path) && (mime === "" || mime === "application/octet-stream")) return "file";
  return null;
}

// streamFormat names a stream's format for the menu: "HLS", "DASH" or ""
// when the type and URL don't say. ODM decides again on its side.
function streamFormat(url, mime) {
  mime = String(mime || "").split(";")[0].trim().toLowerCase();
  let path = "";
  try {
    path = new URL(url).pathname.toLowerCase();
  } catch {
    return "";
  }
  if (mime === "application/dash+xml" || path.endsWith(".mpd")) return "DASH";
  if (STREAM_TYPES.includes(mime) || path.endsWith(".m3u8")) return "HLS";
  return "";
}

// normalizeMediaURL drops the fragment, which the server never sees, so
// the same file reached as v.mp4 and v.mp4#t=10 is listed once. The query
// is kept as is: no site's parameters are special.
function normalizeMediaURL(url) {
  try {
    const u = new URL(url);
    u.hash = "";
    return u.toString();
  } catch {
    return url;
  }
}

// totalSize reads the full size from Content-Range ("bytes 0-99/1234") or,
// for a whole response, Content-Length. 0 when unknown.
function totalSize(headers) {
  const get = (name) => {
    const h = (headers || []).find((x) => x.name.toLowerCase() === name);
    return h ? h.value : "";
  };
  const range = /\/(\d+)\s*$/.exec(get("content-range"));
  if (range) return Number(range[1]);
  const len = Number(get("content-length"));
  return Number.isFinite(len) && len > 0 ? len : 0;
}

// mergeMedia adds item to list (deduplicated by URL, keeping the larger
// known size and the first time it was seen), newest first, capped. Over
// the cap the oldest file goes first: a player keeps loading files, while
// the playlist with every quality is loaded once, at the start.
function mergeMedia(list, item) {
  const rest = (list || []).filter((x) => x.url !== item.url);
  const old = (list || []).find((x) => x.url === item.url);
  if (old) {
    item = { ...item, size: Math.max(old.size, item.size) };
    if (old.at !== undefined && !(item.at <= old.at)) item.at = old.at;
  }
  const next = [item, ...rest];
  while (next.length > MAX_MEDIA_PER_TAB) {
    const f = next.findLastIndex((x) => x.kind !== "stream");
    next.splice(f >= 0 ? f : next.length - 1, 1);
  }
  return next;
}

/* ---------- browser side ---------- */

function askOdm(message) {
  return new Promise((resolve) => {
    try {
      chrome.runtime.sendNativeMessage(HOST, message, (reply) => {
        if (chrome.runtime.lastError) {
          resolve({ ok: false, error: chrome.runtime.lastError.message });
        } else {
          resolve(reply || { ok: false, error: "no reply from ODM" });
        }
      });
    } catch (e) {
      resolve({ ok: false, error: String(e) });
    }
  });
}

async function isEnabled() {
  const { enabled = true } = await chrome.storage.local.get("enabled");
  return enabled;
}

async function getConfig() {
  const { odmConfig } = await chrome.storage.session.get("odmConfig");
  if (odmConfig && Date.now() - odmConfig.at < CONFIG_TTL_MS) return odmConfig;
  const reply = await askOdm({ type: "config" });
  const cfg = reply.ok && reply.config
    ? { fileTypes: reply.config.fileTypes || DEFAULT_TYPES, excludeSites: reply.config.excludeSites || [], at: Date.now() }
    : { fileTypes: DEFAULT_TYPES, excludeSites: [], at: 0 }; // retry next time
  await chrome.storage.session.set({ odmConfig: cfg });
  return cfg;
}

async function cookieHeader(url) {
  try {
    const cookies = await chrome.cookies.getAll({ url });
    return cookies.map((c) => `${c.name}=${c.value}`).join("; ");
  } catch {
    return "";
  }
}

async function handOff(fields) {
  return askOdm({
    type: "download",
    cookies: await cookieHeader(fields.url),
    userAgent: navigator.userAgent,
    ...fields,
  });
}

const tabKey = (tabId) => `media:${tabId}`;

// What ODM said about each playlist or site page, kept for STREAM_INFO_MS:
// a menu asks whenever it opens, and the page need not be read each time.
// A failure that may pass (ODM busy, yt-dlp missing, a network error) is
// not kept.
const mediaInfos = new Map();
function mediaInfo(item, referrer) {
  const now = Date.now();
  for (const [u, e] of mediaInfos) {
    if (now - e.at >= STREAM_INFO_MS) mediaInfos.delete(u);
  }
  const url = item.url;
  const hit = mediaInfos.get(url);
  if (hit) return hit.reply;
  // yt-dlp reads a site's page itself, without the browser's cookies.
  const reply = item.kind === "site"
    ? askOdm({ type: "site-formats", url })
    : cookieHeader(url).then((cookies) =>
      askOdm({ type: "streams", url, referrer, cookies, userAgent: navigator.userAgent }));
  mediaInfos.set(url, { at: now, reply });
  reply.then((r) => {
    if (!r.ok && !r.refused) mediaInfos.delete(url);
  });
  return reply;
}

// grab sends one detected media item to ODM. The referrer is the page (or
// frame) it was found on; streams get the page title as their name, and
// the quality picked in a menu, if any.
function grab(item, referrer, title) {
  if (item.kind === "site") {
    // yt-dlp reads the page itself: no cookies or user agent go with it.
    return askOdm({
      type: "download", kind: "site", url: item.url, title: item.title || title,
      format: item.format || "", ext: item.ext || "", estimate: item.estimate || 0,
    });
  }
  return handOff({
    url: item.url, referrer, title,
    mime: item.mime || "", size: item.size || 0,
    kind: item.kind === "stream" ? "stream" : "file",
    quality: item.quality || "",
  });
}

// popupReferrer picks the referrer for an item sent from the popup, as
// close as possible to the frame URL the in-page button would send: the
// tab's URL for the top frame, else the frame's URL (Firefox) or origin
// (Chromium).
function popupReferrer(item, tab) {
  const tabURL = (tab && tab.url) || "";
  if (item.frameId === 0) return tabURL;
  return item.page || (item.origin ? item.origin + "/" : "") || tabURL;
}

// setBadge shows how many media items the tab has on the toolbar icon.
function setBadge(tabId, n) {
  try {
    const p = chrome.action.setBadgeText({ tabId, text: n ? String(n) : "" });
    if (p && p.catch) p.catch(() => { /* tab gone */ });
  } catch { /* tab gone */ }
}

async function tabMedia(tabId) {
  const key = tabKey(tabId);
  return (await chrome.storage.session.get(key))[key] || [];
}

// Players load several playlists at once; each update reads, merges and
// writes the tab's list, so updates to one tab run one after another or
// the later write drops what the earlier one added.
const tabQueues = new Map();
function updateTab(tabId, fn) {
  const run = (tabQueues.get(tabId) || Promise.resolve()).then(fn).catch(() => {});
  tabQueues.set(tabId, run);
  run.then(() => {
    if (tabQueues.get(tabId) === run) tabQueues.delete(tabId);
  });
  return run;
}

if (typeof chrome !== "undefined" && chrome.runtime && chrome.runtime.id) {
  // Listeners are registered synchronously at load, as both a Chromium
  // service worker and a Firefox event page require.

  chrome.downloads.onCreated.addListener(async (item) => {
    if (!(await isEnabled())) return;
    const url = item.finalUrl || item.url;
    if (!url || !/^https?:/.test(url)) return;
    const { bypass = {} } = await chrome.storage.session.get("bypass");
    if (bypass[url] && Date.now() - bypass[url] < BYPASS_MS) return; // Alt-click
    const cfg = await getConfig();
    if (isExcluded(url, cfg.excludeSites)) return;
    let filename = item.filename || "";
    if (!filename && !hasType(url, cfg.fileTypes, "")) {
      filename = await settledFilename(chrome.downloads, item.id, NAME_WAIT_MS);
    }
    if (!hasType(url, cfg.fileTypes, filename)) return;

    // Hold the browser's copy until ODM confirms; it used to be cancelled
    // first, so with ODM closed or unregistered the download was lost.
    try { await chrome.downloads.pause(item.id); } catch { /* may already be done */ }
    // Not the name awaited above: Chromium adds " (1)" when its own folder
    // has the file already. ODM reads the name from the server.
    const reply = await handOff({
      url, referrer: item.referrer || "", filename: item.filename || "",
      mime: item.mime || "", size: item.totalBytes > 0 ? item.totalBytes : 0,
    });
    if (reply.ok) {
      chrome.downloads.cancel(item.id, () => chrome.downloads.erase({ id: item.id }));
    } else {
      try { await chrome.downloads.resume(item.id); } catch { /* finished meanwhile */ }
    }
  });

  chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
      id: "odm-download",
      title: "Download with ODM",
      contexts: ["link", "video", "audio", "image"],
    });
  });

  chrome.contextMenus.onClicked.addListener((info, tab) => {
    const url = info.linkUrl || info.srcUrl;
    if (url && /^https?:/.test(url)) {
      handOff({ url, referrer: info.pageUrl || "", title: (tab && tab.title) || "" });
    }
  });

  chrome.webRequest.onHeadersReceived.addListener(
    (d) => {
      if (d.tabId < 0 || (d.statusCode !== 200 && d.statusCode !== 206)) return;
      const ct = (d.responseHeaders || []).find((h) => h.name.toLowerCase() === "content-type");
      const mime = ct ? ct.value.split(";")[0].trim().toLowerCase() : "";
      const kind = mediaKind(d.url, mime);
      if (!kind) return;
      const size = totalSize(d.responseHeaders);
      if (kind === "file" && size > 0 && size < MIN_MEDIA_BYTES) return;
      const item = { url: normalizeMediaURL(d.url), mime, size, kind, frameId: d.frameId, at: Date.now() };
      // The document that asked for it, for the referrer when the popup
      // sends it: Firefox gives the frame's URL, Chromium only its origin.
      if (d.documentUrl) item.page = d.documentUrl;
      else if (d.initiator && d.initiator !== "null") item.origin = d.initiator;
      if (kind === "stream") item.format = streamFormat(d.url, mime);
      updateTab(d.tabId, async () => {
        const next = mergeMedia(await tabMedia(d.tabId), item);
        await chrome.storage.session.set({ [tabKey(d.tabId)]: next });
        setBadge(d.tabId, next.length);
        const forFrame = next.filter((x) => x.frameId === d.frameId);
        chrome.tabs.sendMessage(d.tabId, { type: "odm-media", items: forFrame }, { frameId: d.frameId })
          .catch(() => { /* no content script in that frame (yet) */ });
      });
    },
    { urls: ["<all_urls>"], types: ["media", "xmlhttprequest", "object", "other"] },
    ["responseHeaders"],
  );

  // A new page in the tab starts a fresh media list.
  chrome.tabs.onUpdated.addListener((tabId, change) => {
    // Queued like the updates, or one still running for the old page
    // could write its list (and count) back after the reset.
    if (change.url) {
      updateTab(tabId, async () => {
        await chrome.storage.session.remove(tabKey(tabId));
        setBadge(tabId, 0);
      });
    }
  });
  chrome.tabs.onRemoved.addListener((tabId) => chrome.storage.session.remove(tabKey(tabId)));

  chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    const tab = sender.tab;
    if (!tab) {
      // The toolbar popup: not in a tab, so it names the tab it shows.
      // Content scripts in frames of tab-less extension pages also come
      // without a tab, so the sender must be our popup page itself.
      const popup = chrome.runtime.getURL("popup.html");
      if (sender.id !== chrome.runtime.id || !sender.url || !sender.url.startsWith(popup) ||
          typeof msg.tabId !== "number") return false;
      if (msg.type === "odm-tab-media") {
        tabMedia(msg.tabId).then(sendResponse);
        return true;
      }
      if (msg.type === "odm-grab" && msg.item) {
        chrome.tabs.get(msg.tabId).catch(() => null)
          .then((t) => grab(msg.item, popupReferrer(msg.item, t), (t && t.title) || ""))
          .then(sendResponse);
        return true;
      }
      if (msg.type === "odm-media-info" && msg.item) {
        chrome.tabs.get(msg.tabId).catch(() => null)
          .then((t) => mediaInfo(msg.item, popupReferrer(msg.item, t)))
          .then(sendResponse);
        return true;
      }
      return false;
    }
    if (msg.type === "odm-media-request") {
      tabMedia(tab.id).then((list) => sendResponse(list.filter((x) => x.frameId === sender.frameId)));
      return true; // async response
    }
    if (msg.type === "odm-grab" && msg.item) {
      grab(msg.item, sender.url || tab.url || "", tab.title || "").then(sendResponse);
      return true;
    }
    if (msg.type === "odm-media-info" && msg.item) {
      mediaInfo(msg.item, sender.url || tab.url || "").then(sendResponse);
      return true;
    }
    if (msg.type === "odm-bypass" && msg.url) {
      chrome.storage.session.get("bypass").then(({ bypass = {} }) => {
        const now = Date.now();
        const next = pruneBypass(bypass, now);
        next[msg.url] = now;
        chrome.storage.session.set({ bypass: next });
      });
    }
    return false;
  });
}

if (typeof module !== "undefined") {
  module.exports = { hasType, settledFilename, hostMatches, isExcluded, mediaKind, streamFormat, normalizeMediaURL, totalSize, mergeMedia, popupReferrer, pruneBypass, BYPASS_MS, DEFAULT_TYPES };
}
