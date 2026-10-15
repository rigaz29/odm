// Run with: node --test extension/test/*.test.js
const test = require("node:test");
const assert = require("node:assert/strict");
const bg = require("../background.js");

test("mediaKind", () => {
  assert.equal(bg.mediaKind("https://cdn/media/play?expire=1", "video/mp4"), "file");
  assert.equal(bg.mediaKind("https://cdn/x/master.m3u8", ""), "stream");
  assert.equal(bg.mediaKind("https://cdn/x/manifest", "application/dash+xml"), "stream");
  assert.equal(bg.mediaKind("https://cdn/x/seg-001.ts", "video/mp2t"), null); // HLS segment
  assert.equal(bg.mediaKind("https://cdn/x/chunk-5.m4s", "video/mp4"), null); // fMP4 segment
  assert.equal(bg.mediaKind("https://cdn/x/seg", "audio/iso.segment"), null);
  assert.equal(bg.mediaKind("https://cdn/clip.mp4", "application/octet-stream"), "file");
  assert.equal(bg.mediaKind("https://cdn/page.html", "text/html"), null);
  assert.equal(bg.mediaKind("https://cdn/song", "audio/mpeg; charset=x"), "file");
});

test("normalizeMediaURL drops the fragment and keeps the query", () => {
  assert.equal(bg.normalizeMediaURL("https://cdn.example/v.mp4?range=5&rn=3#t=1"), "https://cdn.example/v.mp4?range=5&rn=3");
  assert.equal(bg.normalizeMediaURL("not a url"), "not a url");
});

test("totalSize", () => {
  assert.equal(bg.totalSize([{ name: "Content-Range", value: "bytes 0-1048575/69652480" }]), 69652480);
  assert.equal(bg.totalSize([{ name: "content-length", value: "1234" }]), 1234);
  assert.equal(bg.totalSize([]), 0);
  assert.equal(bg.totalSize([{ name: "Content-Range", value: "bytes 0-9/*" }, { name: "Content-Length", value: "10" }]), 10);
});

test("exclusions", () => {
  assert.ok(bg.hostMatches("download.windowsupdate.com", "*.windowsupdate.com"));
  assert.ok(bg.hostMatches("windowsupdate.com", "*.windowsupdate.com"));
  assert.ok(!bg.hostMatches("evilwindowsupdate.com", "*.windowsupdate.com"));
  assert.ok(bg.isExcluded("https://Example.org/a.zip", ["example.org"]));
  assert.ok(!bg.isExcluded("https://sub.example.org/a.zip", ["example.org"]));
});

test("hasType checks the URL and the browser's file name", () => {
  assert.ok(bg.hasType("https://h/a.ZIP?x=1", ["zip"]));
  assert.ok(bg.hasType("https://h/download?id=3", [".exe"], "C:\\Users\\u\\Downloads\\setup.exe"));
  assert.ok(!bg.hasType("https://h/page", ["zip"], ""));
});

// fakeDownloads stands in for chrome.downloads with one item.
function fakeDownloads(item) {
  const listeners = new Set();
  return {
    listeners,
    onChanged: { addListener: (f) => listeners.add(f), removeListener: (f) => listeners.delete(f) },
    search: async ({ id }) => (id === item.id ? [item] : []),
    emit: (d) => { for (const f of [...listeners]) f(d); },
  };
}
const tick = () => new Promise((r) => setTimeout(r, 5));

test("settledFilename waits for the name Chromium gives after onCreated", async () => {
  const dl = fakeDownloads({ id: 4, filename: "", state: "in_progress" });
  const name = bg.settledFilename(dl, 4, 1000);
  await tick();
  dl.emit({ id: 3, filename: { current: "C:\\other.zip", previous: "" } });
  dl.emit({ id: 4, filename: { current: "C:\\Users\\u\\Downloads\\book.epub", previous: "" } });
  assert.equal(await name, "C:\\Users\\u\\Downloads\\book.epub");
  assert.equal(dl.listeners.size, 0);
  assert.ok(bg.hasType("https://h/get?id=7", ["epub"], await name));
});

test("settledFilename: named already, ended first, or too slow", async () => {
  assert.equal(await bg.settledFilename(fakeDownloads({ id: 1, filename: "C:\\a.zip", state: "in_progress" }), 1, 1000), "C:\\a.zip");

  const dl = fakeDownloads({ id: 2, filename: "", state: "in_progress" });
  const name = bg.settledFilename(dl, 2, 1000);
  await tick();
  dl.emit({ id: 2, state: { current: "interrupted", previous: "in_progress" } });
  assert.equal(await name, "");

  const slow = fakeDownloads({ id: 3, filename: "", state: "in_progress" });
  assert.equal(await bg.settledFilename(slow, 3, 20), "");
  assert.equal(slow.listeners.size, 0);
});

test("mergeMedia dedupes, keeps the larger size, caps the list", () => {
  let list = bg.mergeMedia([], { url: "a", size: 100 });
  list = bg.mergeMedia(list, { url: "a", size: 0 });
  assert.equal(list.length, 1);
  assert.equal(list[0].size, 100);
  for (let i = 0; i < 40; i++) list = bg.mergeMedia(list, { url: "u" + i, size: i });
  assert.equal(list.length, 15);
  assert.equal(list[0].url, "u39");
});

test("mergeMedia keeps when an item was first seen", () => {
  let list = bg.mergeMedia([], { url: "m", size: 0, at: 5 });
  list = bg.mergeMedia(list, { url: "m", size: 0, at: 9 });
  assert.equal(list[0].at, 5);
  assert.equal(bg.mergeMedia([], { url: "x", size: 0 })[0].at, undefined);
});

test("mergeMedia drops files before playlists when full", () => {
  let list = bg.mergeMedia([], { url: "master", kind: "stream", size: 0 });
  for (let i = 0; i < 40; i++) list = bg.mergeMedia(list, { url: "f" + i, kind: "file", size: i });
  assert.equal(list.length, 15);
  assert.ok(list.some((x) => x.url === "master"));
  assert.equal(list[0].url, "f39");
});

test("streamFormat", () => {
  assert.equal(bg.streamFormat("https://cdn/x/master.m3u8", ""), "HLS");
  assert.equal(bg.streamFormat("https://cdn/x/play", "application/x-mpegURL; charset=utf-8"), "HLS");
  assert.equal(bg.streamFormat("https://cdn/x/manifest", "application/dash+xml"), "DASH");
  assert.equal(bg.streamFormat("https://cdn/x/m.mpd?t=1", "text/plain"), "DASH");
  assert.equal(bg.streamFormat("https://cdn/x/video", "video/mp4"), "");
});

test("popupReferrer", () => {
  const tab = { url: "https://site.example/watch?v=1" };
  assert.equal(bg.popupReferrer({ frameId: 0, origin: "https://site.example" }, tab), tab.url);
  assert.equal(bg.popupReferrer({ frameId: 5, page: "https://player.example/embed/9" }, tab), "https://player.example/embed/9");
  assert.equal(bg.popupReferrer({ frameId: 5, origin: "https://player.example" }, tab), "https://player.example/");
  assert.equal(bg.popupReferrer({ frameId: 5 }, tab), tab.url);
  assert.equal(bg.popupReferrer({ frameId: 5 }, null), "");
});

test("pruneBypass keeps only recent Alt-click marks", () => {
  const now = 1_000_000;
  const got = bg.pruneBypass({
    "https://h/new.zip": now - 1000,
    "https://h/edge.zip": now - bg.BYPASS_MS,
    "https://h/old.zip": now - 60 * 60 * 1000,
  }, now);
  assert.deepEqual(got, { "https://h/new.zip": now - 1000 });
  assert.deepEqual(bg.pruneBypass(undefined, now), {});
});
