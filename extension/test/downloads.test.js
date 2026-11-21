// Run with: node --test extension/test/*.test.js
// How the extension hands the browser's downloads to ODM, with the
// browser's API stood in for.
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");

const bgFile = path.join(__dirname, "..", "background.js");

// fakeBrowser stands in for chrome.*: chromium gives it onDeterminingFilename,
// as Chromium browsers have and Firefox has not. odmReply answers a download.
function fakeBrowser({ chromium = true, odmReply = { ok: true, message: "opened in ODM" } } = {}) {
  const listeners = {};
  const on = (name) => ({ addListener: (fn) => { listeners[name] = fn; }, removeListener() {} });
  const session = {};
  const sent = [];
  const calls = [];
  const chrome = {
    runtime: {
      id: "test-extension",
      lastError: undefined,
      sendNativeMessage(host, msg, cb) {
        sent.push(msg);
        if (msg.type === "config") cb({ ok: true, config: { fileTypes: ["rar", "zip"], excludeSites: [], torrents: false } });
        else cb(odmReply);
      },
      onInstalled: on("installed"),
      onMessage: on("message"),
      getURL: (p) => "chrome-extension://test-extension/" + p,
    },
    storage: {
      local: { get: async () => ({ enabled: true }), set: async () => {} },
      session: {
        get: async (key) => (key in session ? { [key]: session[key] } : {}),
        set: async (obj) => { Object.assign(session, obj); },
        remove: async (key) => { delete session[key]; },
      },
    },
    downloads: {
      onCreated: on("created"),
      onChanged: on("changed"),
      pause: async (id) => { calls.push(["pause", id]); },
      resume: async (id) => { calls.push(["resume", id]); },
      cancel: (id, cb) => { calls.push(["cancel", id]); cb && cb(); },
      erase: (q) => { calls.push(["erase", q.id]); },
      search: async ({ id }) => [{ id, filename: "C:\\Users\\me\\Downloads\\app.zip", state: "in_progress" }],
    },
    contextMenus: { create() {}, onClicked: on("menu") },
    webRequest: { onHeadersReceived: on("headers") },
    tabs: { onUpdated: on("tabUpdated"), onRemoved: on("tabRemoved"), sendMessage: async () => {}, get: async () => null },
    cookies: { getAll: async () => [] },
    action: { setBadgeText: async () => {} },
  };
  if (chromium) chrome.downloads.onDeterminingFilename = on("determining");
  global.chrome = chrome;
  delete require.cache[require.resolve(bgFile)];
  require(bgFile);
  return { listeners, sent, calls };
}

// determine runs Chromium's naming of item and resolves once the extension
// is done with it: { returned, suggested } (whether it suggested a name).
async function determine(fake, item) {
  let suggested = 0;
  const returned = fake.listeners.determining(item, () => { suggested++; });
  for (let i = 0; i < 50 && suggested === 0 && !fake.calls.some(([c]) => c === "erase"); i++) {
    await new Promise((r) => setTimeout(r, 5));
  }
  return { returned, suggested };
}

const hostItem = (over = {}) => ({
  id: 43, url: "https://dl.filehost.example/dl/5HzKRR", finalUrl: "https://dl.filehost.example/dl/5HzKRR",
  filename: "Game_--_.part01.rar", mime: "application/x-rar-compressed", totalBytes: 524288000,
  startTime: new Date().toISOString(), referrer: "", ...over,
});

test.afterEach(() => { delete global.chrome; });

test("a file host's link with no file type goes to ODM before the browser's Save As dialog", async () => {
  const fake = fakeBrowser();
  assert.equal(fake.listeners.created, undefined, "onCreated would hand it over twice");
  const { returned, suggested } = await determine(fake, hostItem());
  assert.equal(returned, true);
  const downloads = fake.sent.filter((m) => m.type === "download");
  assert.equal(downloads.length, 1);
  assert.equal(downloads[0].url, "https://dl.filehost.example/dl/5HzKRR");
  assert.equal(downloads[0].filename, "Game_--_.part01.rar");
  assert.deepEqual(fake.calls, [["pause", 43], ["cancel", 43], ["erase", 43]]);
  assert.equal(suggested, 0, "the browser went on to name it (and ask where to save it)");
});

test("a download ODM does not take is named by the browser as usual", async () => {
  const fake = fakeBrowser();
  const { suggested } = await determine(fake, hostItem({ filename: "page.html", mime: "text/html" }));
  assert.equal(suggested, 1);
  assert.equal(fake.sent.filter((m) => m.type === "download").length, 0);
  assert.deepEqual(fake.calls, []);
});

test("a download ODM refuses goes on in the browser", async () => {
  const fake = fakeBrowser({ odmReply: { ok: false, error: "ODM is not running and could not be started" } });
  const { suggested } = await determine(fake, hostItem());
  assert.equal(suggested, 1);
  assert.deepEqual(fake.calls, [["pause", 43], ["resume", 43]]);
});

test("Alt-click keeps a redirected download in the browser", async () => {
  const fake = fakeBrowser();
  const href = "https://github.com/owner/app/releases/download/v1.0/app.zip";
  fake.listeners.message({ type: "odm-bypass", url: href }, { tab: { id: 1 }, frameId: 0 }, () => {});
  await new Promise((r) => setTimeout(r, 20));
  const { suggested } = await determine(fake, hostItem({
    url: href, finalUrl: "https://objects.githubusercontent.com/release-asset/1/2?X-Amz-Signature=abc",
    filename: "app.zip", mime: "application/zip",
  }));
  assert.equal(suggested, 1);
  assert.equal(fake.sent.filter((m) => m.type === "download").length, 0);
});

test("Firefox: onCreated hands a download over, recognised by its media type", async () => {
  const fake = fakeBrowser({ chromium: false });
  await fake.listeners.created(hostItem({ filename: "" }));
  const downloads = fake.sent.filter((m) => m.type === "download");
  assert.equal(downloads.length, 1);
  assert.deepEqual(fake.calls, [["pause", 43], ["cancel", 43], ["erase", 43]]);
});
