const test = require("node:test");
const assert = require("node:assert/strict");
const ml = require("../medialist.js");

test("orderMedia puts streams first, oldest first, then files by size", () => {
  const got = ml.orderMedia([
    { url: "f1", kind: "file", size: 10 },
    { url: "s2", kind: "stream", at: 20 },
    { url: "f2", kind: "file", size: 99 },
    { url: "s1", kind: "stream", at: 10 },
  ]).map((x) => x.url);
  assert.deepEqual(got, ["s1", "s2", "f2", "f1"]);
});

test("mediaLabel", () => {
  assert.equal(ml.mediaLabel({ kind: "stream", format: "HLS" }), "HLS stream");
  assert.equal(ml.mediaLabel({ kind: "stream" }), "Media stream");
  assert.equal(ml.mediaLabel({ kind: "file", mime: "video/mp4", size: 1536 }), "mp4 file, 1.50 KB");
  assert.equal(ml.mediaLabel({ kind: "file", size: 0 }), "video file");
});

test("ago", () => {
  assert.equal(ml.ago(0, 5000), "");
  assert.equal(ml.ago(1000, 13000), "12s ago");
  assert.equal(ml.ago(0 + 1, 1 + 180000), "3 min ago");
  assert.equal(ml.ago(1, 1 + 7200000), "2 h ago");
});

test("menuEntries lists a stream's qualities once ODM has read it", () => {
  const master = { url: "https://cdn/master.m3u8", kind: "stream", format: "HLS", at: 1 };
  const variant = { url: "https://cdn/720.m3u8", kind: "stream", format: "HLS", at: 2 };
  const file = { url: "https://cdn/a.mp4", kind: "file", mime: "video/mp4", size: 2048, at: 3 };
  const items = [file, variant, master];

  // Nothing read yet: each stream waits, files can be sent at once.
  let got = ml.menuEntries(items, new Map());
  assert.deepEqual(got.map((e) => [e.text, !!e.item]), [
    ["HLS stream · reading qualities…", false],
    ["HLS stream · reading qualities…", false],
    ["mp4 file, 2.00 KB", true],
  ]);

  const infos = new Map([
    [master.url, { ok: true, stream: { kind: "hls", note: "Live stream: ODM records it.", qualities: [
      { label: "1080p · 4.5 Mbps · H.264", estimate: 3 * 1024 * 1024 },
      { label: "720p · 2.0 Mbps · H.264" },
    ] } }],
    [variant.url, { ok: true, stream: { kind: "hls", single: true, qualities: [{ label: "As found (one quality)" }] } }],
  ]);
  got = ml.menuEntries(items, infos);
  assert.deepEqual(got.map((e) => e.text), [
    "HLS stream",
    "1080p · 4.5 Mbps · H.264 · ~3.00 MB",
    "720p · 2.0 Mbps · H.264",
    "Live stream: ODM records it.",
    "mp4 file, 2.00 KB", // the one-quality playlist is left out
  ]);
  assert.ok(got[0].heading && !got[0].item);
  assert.equal(got[1].item.quality, "1080p · 4.5 Mbps · H.264");
  assert.equal(got[1].item.url, master.url);
  assert.ok(!got[3].item);
});

test("menuEntries: one quality alone, refused, or not read", () => {
  const s = { url: "https://cdn/v.m3u8", kind: "stream", format: "HLS" };
  const one = new Map([[s.url, { ok: true, stream: { kind: "hls", single: true, qualities: [{ label: "As found (one quality)", estimate: 1024 }] } }]]);
  assert.deepEqual(ml.menuEntries([s], one).map((e) => [e.text, !!e.item]), [["HLS stream · one quality · ~1.00 KB", true]]);
  const drm = new Map([[s.url, { ok: false, refused: true, error: "this stream is DRM-protected and cannot be downloaded" }]]);
  assert.deepEqual(ml.menuEntries([s], drm).map((e) => [e.text, !!e.item]),
    [["HLS stream · this stream is DRM-protected and cannot be downloaded", false]]);
  const down = new Map([[s.url, { ok: false, error: "ODM is not running" }]]);
  const e = ml.menuEntries([s], down);
  assert.equal(e[0].text, "HLS stream · best quality");
  assert.equal(e[0].item.quality, undefined);
});
