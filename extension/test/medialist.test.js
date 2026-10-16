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

test("sitePage gives YouTube videos one watch address", () => {
  const watch = "https://www.youtube.com/watch?v=jNQXAC9IVRw";
  for (const href of [
    "https://www.youtube.com/watch?v=jNQXAC9IVRw&list=PL1&t=4s",
    "https://m.youtube.com/watch?v=jNQXAC9IVRw",
    "https://youtu.be/jNQXAC9IVRw?si=x",
    "https://www.youtube.com/shorts/jNQXAC9IVRw",
    "https://www.youtube.com/embed/jNQXAC9IVRw?autoplay=1",
    "https://www.youtube-nocookie.com/embed/jNQXAC9IVRw",
    "https://www.youtube.com/live/jNQXAC9IVRw",
  ]) {
    assert.equal(ml.sitePage(href), watch, href);
  }
  assert.equal(ml.sitePage("https://music.youtube.com/watch?v=jNQXAC9IVRw"), "https://music.youtube.com/watch?v=jNQXAC9IVRw");
  for (const href of ["https://www.youtube.com/", "https://www.youtube.com/@channel", "https://www.youtube.com/watch?v=short",
    "https://example.com/watch?v=jNQXAC9IVRw", "not a url"]) {
    assert.equal(ml.sitePage(href), "", href);
  }
});

test("siteTitle drops the site's name", () => {
  assert.equal(ml.siteTitle("(3) Me at the zoo - YouTube"), "Me at the zoo");
  assert.equal(ml.siteTitle("A song - YouTube Music"), "A song");
  assert.equal(ml.siteTitle("Plain"), "Plain");
});

test("menuEntries lists a site video's qualities from yt-dlp", () => {
  const page = { kind: "site", site: "YouTube", url: "https://www.youtube.com/watch?v=jNQXAC9IVRw", title: "Me at the zoo" };
  const file = { url: "https://cdn/a.mp4", kind: "file", mime: "video/mp4", size: 2048 };
  assert.deepEqual(ml.menuEntries([file, page], new Map()).map((e) => [e.text, !!e.item]), [
    ["YouTube video · reading qualities…", false],
    ["mp4 file, 2.00 KB", true],
  ]);
  const infos = new Map([[page.url, { ok: true, site: { title: "Me at the zoo", qualities: [
    { label: "240p · 313 kbps · H.264, AAC · MP4", format: "133+140", ext: "mp4", estimate: 1024 },
    { label: "Audio only · 130 kbps · AAC · M4A", format: "140", ext: "m4a" },
  ] } }]]);
  const got = ml.menuEntries([page], infos);
  assert.deepEqual(got.map((e) => e.text), [
    "YouTube video",
    "240p · 313 kbps · H.264, AAC · MP4 · ~1.00 KB",
    "Audio only · 130 kbps · AAC · M4A",
  ]);
  assert.deepEqual([got[1].item.format, got[1].item.ext, got[1].item.estimate, got[1].item.title], ["133+140", "mp4", 1024, "Me at the zoo"]);

  const say = (r) => ml.menuEntries([page], new Map([[page.url, r]])).map((e) => [e.text, !!e.item]);
  assert.deepEqual(say({ ok: false, missing: true, error: "yt-dlp not found" }), [["YouTube video · needs yt-dlp (see ODM's Options)", false]]);
  assert.deepEqual(say({ ok: false, refused: true, error: "live streams on video sites are not supported" }),
    [["YouTube video · live streams on video sites are not supported", false]]);
  assert.deepEqual(say({ ok: false, error: "ODM is not running" }), [["YouTube video · best quality", true]]);
});

test("menuEntries offers to record a broadcast yt-dlp found on a page", () => {
  const page = { kind: "site", site: "YouTube", url: "https://www.youtube.com/watch?v=live1234567", title: "Launch" };
  const reply = { ok: true, site: {
    title: "Launch", live: true, manifest: "https://manifest.example/master.m3u8", headers: { "User-Agent": "yt" },
    stream: { kind: "hls", live: true, qualities: [{ label: "1080p · 4.5 Mbps · H.264, AAC" }, { label: "720p · 2.0 Mbps · H.264, AAC" }] },
  } };
  const got = ml.menuEntries([page], new Map([[page.url, reply]]));
  assert.deepEqual(got.map((e) => [e.text, !!e.item]), [
    ["● LIVE · YouTube video", false],
    ["Record · 1080p · 4.5 Mbps · H.264, AAC", true],
    ["Record · 720p · 2.0 Mbps · H.264, AAC", true],
    ["Records until you stop it in ODM", false],
  ]);
  const it = got[1].item;
  assert.deepEqual([it.kind, it.url, it.page, it.quality, it.headers["User-Agent"], it.mime],
    ["stream", "https://manifest.example/master.m3u8", page.url, "1080p · 4.5 Mbps · H.264, AAC", "yt", "application/vnd.apple.mpegurl"]);
  assert.ok(ml.anyLive([page], new Map([[page.url, reply]])));

  // Without the playlist's qualities (ODM could not read it), one row.
  const bare = { ok: true, site: { live: true, manifest: "https://m/x.m3u8" } };
  const rows = ml.menuEntries([page], new Map([[page.url, bare]]));
  assert.equal(rows[1].text, "Record · best quality");
  assert.equal(rows[1].item.quality, undefined);
});

test("menuEntries marks a live stream the page's player loaded", () => {
  const s = { url: "https://cdn/live.m3u8", kind: "stream", format: "HLS" };
  const listed = { ok: true, stream: { kind: "hls", live: true, note: "Live stream: ODM records it from now until you stop the download.",
    qualities: [{ label: "720p · 2.0 Mbps" }] } };
  assert.deepEqual(ml.menuEntries([s], new Map([[s.url, listed]])).map((e) => e.text), [
    "● LIVE · HLS stream",
    "Record · 720p · 2.0 Mbps",
    "Live stream: ODM records it from now until you stop the download.",
  ]);
  const one = { ok: true, stream: { kind: "hls", live: true, single: true, qualities: [{ label: "As found (one quality)" }] } };
  assert.deepEqual(ml.menuEntries([s], new Map([[s.url, one]])).map((e) => e.text), ["HLS stream · ● Record live"]);
  assert.ok(!ml.anyLive([s], new Map()));
});
