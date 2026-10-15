// How a tab's media list is ordered and labelled, shared by the in-page
// button (content.js) and the toolbar popup (popup.js) so both read alike.
// Loaded as a plain script in both places, and by Node for the tests.
(function (root) {
  function fmtSize(n) {
    if (!n) return "";
    const u = ["B", "KB", "MB", "GB"];
    let i = 0;
    while (n >= 1000 && i < u.length - 1) {
      n /= 1024;
      i++;
    }
    return `${n.toFixed(i ? 2 : 0)} ${u[i]}`;
  }

  // Streams first, in the order the player loaded them: a player reads the
  // playlist listing every quality before the one quality it plays, so the
  // first is usually the complete one. Files follow, largest first.
  function orderMedia(items) {
    const streams = items.filter((i) => i.kind === "stream").sort((a, b) => (a.at || 0) - (b.at || 0));
    const files = items.filter((i) => i.kind !== "stream").sort((a, b) => b.size - a.size);
    return [...streams, ...files];
  }

  function mediaLabel(it) {
    // Only headers were seen for a stream: whether its playlist lists every
    // quality or is one of them is known once ODM reads it.
    if (it.kind === "stream") return `${it.format || "Media"} stream`;
    const type = (it.mime && it.mime.split("/")[1]) || "video";
    return `${type} file${it.size ? ", " + fmtSize(it.size) : ""}`;
  }

  // qualityLabel is a stream quality as menus list it: ODM's label
  // (resolution, bitrate, codecs) and the expected size.
  function qualityLabel(q) {
    return q.estimate ? `${q.label} · ~${fmtSize(q.estimate)}` : q.label;
  }

  // menuEntries lists what a media menu shows. infos maps a playlist's URL
  // to ODM's reply to a "streams" message; a stream without one is still
  // being asked about. A stream's qualities come under a heading, each an
  // entry whose item carries the quality's label. Players load the
  // playlist listing every quality and then the one they play, so a
  // playlist that is one quality is left out once a list is known.
  // Entries without an item cannot be clicked.
  function menuEntries(items, infos) {
    const ordered = orderMedia(items);
    const info = (it) => (infos ? infos.get(it.url) : undefined);
    const listing = (r) => r && r.ok && r.stream && !r.stream.single;
    const anyListing = ordered.some((it) => it.kind === "stream" && listing(info(it)));
    const out = [];
    for (const it of ordered) {
      if (it.kind !== "stream") {
        out.push({ text: mediaLabel(it), item: it, at: it.at });
        continue;
      }
      const r = info(it);
      const name = r && r.ok && r.stream && r.stream.kind ? `${r.stream.kind.toUpperCase()} stream` : mediaLabel(it);
      if (!r) {
        out.push({ text: `${name} · reading qualities…`, at: it.at });
      } else if (listing(r)) {
        out.push({ text: name, heading: true, at: it.at });
        for (const q of r.stream.qualities) out.push({ text: qualityLabel(q), item: { ...it, quality: q.label }, sub: true });
        if (r.stream.note) out.push({ text: r.stream.note, sub: true });
      } else if (r.ok && r.stream) {
        if (anyListing) continue;
        const q = r.stream.qualities[0];
        out.push({ text: `${name} · one quality${q && q.estimate ? " · ~" + fmtSize(q.estimate) : ""}`, item: it, at: it.at });
      } else if (r.refused) {
        out.push({ text: `${name} · ${r.error}`, at: it.at });
      } else {
        // ODM could not read it just now; it picks the quality when it runs.
        out.push({ text: `${name} · best quality`, item: it, at: it.at });
      }
    }
    return out;
  }

  // "12s ago", "3 min ago"; "" without a time.
  function ago(at, now) {
    if (!at) return "";
    const s = Math.max(0, Math.round((now - at) / 1000));
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.round(s / 60)} min ago`;
    return `${Math.round(s / 3600)} h ago`;
  }

  const api = { fmtSize, orderMedia, mediaLabel, qualityLabel, menuEntries, ago };
  // Always set: in a content script, a page element with id="module"
  // makes the global "module" look defined.
  root.odmMediaList = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof self !== "undefined" ? self : this);
