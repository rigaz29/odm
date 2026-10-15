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
