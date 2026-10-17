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

  // The video of a site's page first, then streams (live ones sent as one
  // response too) in the order the player loaded them: a player reads the
  // playlist listing every quality before the one quality it plays, so the
  // first is usually the complete one. Files follow, largest first.
  function orderMedia(items) {
    const isStream = (i) => i.kind === "stream" || i.kind === "live";
    const sites = items.filter((i) => i.kind === "site");
    const streams = items.filter(isStream).sort((a, b) => (a.at || 0) - (b.at || 0));
    const files = items.filter((i) => !isStream(i) && i.kind !== "site").sort((a, b) => b.size - a.size);
    return [...sites, ...streams, ...files];
  }

  const YOUTUBE_ID = /^[A-Za-z0-9_-]{11}$/;

  const TIKTOK_USER = /^@[A-Za-z0-9._-]+$/;

  // sitePage is the page yt-dlp is given for a video site's address, or ""
  // when the address shows no single video. YouTube: its player fetches
  // signed pieces no extension can hand over, so yt-dlp reads the page.
  // Embedded players (youtube-nocookie too) and short links become the
  // watch page, so one video has one address. TikTok: a video's page, and
  // a live page, whose stream address yt-dlp finds again when it expires.
  function sitePage(href) {
    let u;
    try {
      u = new URL(href);
    } catch {
      return "";
    }
    const host = u.hostname.toLowerCase().replace(/^www\./, "");
    const parts = u.pathname.split("/").filter(Boolean);
    if (host === "tiktok.com" || host === "m.tiktok.com") {
      if (!TIKTOK_USER.test(parts[0] || "")) return "";
      if (parts[1] === "live" && parts.length === 2) return `https://www.tiktok.com/${parts[0]}/live`;
      if (parts[1] === "video" && /^\d+$/.test(parts[2] || "")) return `https://www.tiktok.com/${parts[0]}/video/${parts[2]}`;
      return "";
    }
    let id = "";
    if (host === "youtu.be") {
      id = parts[0] || "";
    } else if (host === "youtube.com" || host === "m.youtube.com" || host === "music.youtube.com" || host === "youtube-nocookie.com") {
      if (parts[0] === "watch") id = u.searchParams.get("v") || "";
      else if (["shorts", "embed", "live", "v"].includes(parts[0])) id = parts[1] || "";
      if (host === "music.youtube.com" && YOUTUBE_ID.test(id)) return `https://music.youtube.com/watch?v=${id}`;
    }
    return YOUTUBE_ID.test(id) ? `https://www.youtube.com/watch?v=${id}` : "";
  }

  // siteName names the site of a page sitePage gave.
  function siteName(page) {
    return /^https:\/\/www\.tiktok\.com\//.test(page) ? "TikTok" : "YouTube";
  }

  // siteTitle drops the site's name a page title ends with. A title that is
  // only the site's own names nothing ("" lets ODM ask yt-dlp): TikTok
  // keeps "TikTok - Make Your Day" while its player moves between videos.
  function siteTitle(title) {
    const t = String(title || "").replace(/^\(\d+\)\s*/, "").replace(/\s*[-–|]\s*(YouTube( Music)?|TikTok)$/, "").trim();
    return /^(TikTok(\s*[-–|]\s*Make Your Day)?|YouTube( Music)?)$/i.test(t) ? "" : t;
  }

  function mediaLabel(it) {
    if (it.kind === "site") return `${it.site || "Site"} video`;
    // Only headers were seen for a stream: whether its playlist lists every
    // quality or is one of them is known once ODM reads it.
    if (it.kind === "stream") return `${it.format || "Media"} stream`;
    if (it.kind === "live") return `${it.format || "Live"} stream`;
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
      if (it.kind === "site") {
        out.push(...siteEntries(it, info(it)));
        continue;
      }
      if (it.kind === "live") {
        // One endless response, one quality: nothing for ODM to read first.
        out.push({ text: `● LIVE · ${mediaLabel(it)}`, heading: true, at: it.at });
        out.push({ text: "Record · as the player gets it", item: it, sub: true });
        continue;
      }
      if (it.kind !== "stream") {
        out.push({ text: mediaLabel(it), item: it, at: it.at });
        continue;
      }
      const r = info(it);
      const name = r && r.ok && r.stream && r.stream.kind ? `${r.stream.kind.toUpperCase()} stream` : mediaLabel(it);
      if (!r) {
        out.push({ text: `${name} · reading qualities…`, at: it.at });
      } else if (listing(r)) {
        const live = r.stream.live;
        out.push({ text: live ? `● LIVE · ${name}` : name, heading: true, at: it.at });
        for (const q of r.stream.qualities) {
          out.push({ text: (live ? "Record · " : "") + qualityLabel(q), item: { ...it, quality: q.label }, sub: true });
        }
        if (r.stream.note) out.push({ text: r.stream.note, sub: true });
      } else if (r.ok && r.stream) {
        if (anyListing) continue;
        const q = r.stream.qualities[0];
        const what = r.stream.live ? "● Record live" : `one quality${q && q.estimate ? " · ~" + fmtSize(q.estimate) : ""}`;
        out.push({ text: `${name} · ${what}`, item: it, at: it.at });
      } else if (r.refused) {
        out.push({ text: `${name} · ${r.error}`, at: it.at });
      } else {
        // ODM could not read it just now; it picks the quality when it runs.
        out.push({ text: `${name} · best quality`, item: it, at: it.at });
      }
    }
    return out;
  }

  // siteEntries are a site video's rows: its qualities as yt-dlp lists them
  // (r is ODM's reply to a "site-formats" message), each item carrying
  // yt-dlp's format, the container, the expected size and its title of the
  // video, which names the file better than the page's (on TikTok, the
  // video's caption).
  function siteEntries(it, r) {
    const name = mediaLabel(it);
    if (!r) return [{ text: `${name} · reading qualities…`, at: it.at }];
    if (r.ok && r.site && r.site.live) return liveEntries(it, r.site, name);
    if (r.ok && r.site) {
      const out = [{ text: name, heading: true, at: it.at }];
      const title = r.site.title || it.title;
      for (const q of r.site.qualities) {
        out.push({ text: qualityLabel(q), item: { ...it, title, format: q.format, ext: q.ext, estimate: q.estimate || 0 }, sub: true });
      }
      return out;
    }
    if (r.missing) return [{ text: `${name} · needs yt-dlp (see ODM's Options)`, at: it.at }];
    if (r.refused) return [{ text: `${name} · ${r.error}`, at: it.at }];
    // ODM could not ask yt-dlp just now; it picks the quality when it runs.
    return [{ text: `${name} · best quality`, item: it, at: it.at }];
  }

  // liveEntries are a broadcast's rows: ODM records the playlist or stream
  // yt-dlp found (site.manifest, fetched with site.headers; site.kind is
  // "hls", "dash" or "http-live") and keeps the page to ask yt-dlp again
  // when its address expires.
  function liveEntries(it, site, name) {
    const live = site.kind === "http-live"
      ? { kind: "live", mime: "video/x-flv", format: "FLV" }
      : site.kind === "dash"
        ? { kind: "stream", mime: "application/dash+xml", format: "DASH" }
        : { kind: "stream", mime: "application/vnd.apple.mpegurl", format: "HLS" };
    Object.assign(live, { url: site.manifest, headers: site.headers || {}, sitePage: it.url, title: site.title || it.title, at: it.at });
    const out = [{ text: `● LIVE · ${name}`, heading: true, at: it.at }];
    const s = site.stream;
    if (s && !s.single && s.qualities && s.qualities.length) {
      for (const q of s.qualities) out.push({ text: `Record · ${qualityLabel(q)}`, item: { ...live, quality: q.label }, sub: true });
    } else {
      out.push({ text: "Record · best quality", item: live, sub: true });
    }
    out.push({ text: "Records until you stop it in ODM", sub: true });
    return out;
  }

  // anyLive reports whether one of the items is a broadcast: ODM said so,
  // or it is one endless response.
  function anyLive(items, infos) {
    return items.some((it) => {
      if (it.kind === "live") return true;
      const r = infos ? infos.get(it.url) : undefined;
      return !!(r && r.ok && ((r.site && r.site.live) || (r.stream && r.stream.live)));
    });
  }

  // "12s ago", "3 min ago"; "" without a time.
  function ago(at, now) {
    if (!at) return "";
    const s = Math.max(0, Math.round((now - at) / 1000));
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.round(s / 60)} min ago`;
    return `${Math.round(s / 3600)} h ago`;
  }

  const api = { fmtSize, orderMedia, mediaLabel, qualityLabel, menuEntries, anyLive, sitePage, siteName, siteTitle, ago };
  // Always set: in a content script, a page element with id="module"
  // makes the global "module" look defined.
  root.odmMediaList = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof self !== "undefined" ? self : this);
