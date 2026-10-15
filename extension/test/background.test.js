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
