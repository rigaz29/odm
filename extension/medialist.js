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

  // "12s ago", "3 min ago"; "" without a time.
  function ago(at, now) {
    if (!at) return "";
    const s = Math.max(0, Math.round((now - at) / 1000));
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.round(s / 60)} min ago`;
    return `${Math.round(s / 3600)} h ago`;
  }

  const api = { fmtSize, orderMedia, mediaLabel, ago };
  // Always set: in a content script, a page element with id="module"
  // makes the global "module" look defined.
  root.odmMediaList = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof self !== "undefined" ? self : this);
