// ODM Integration: shows a "Download this video" button over the page's
// video when the background script has seen a downloadable media response
// in this frame. Runs in every frame, because players are usually iframes.
(() => {
  if (window.__odmContent) return;
  window.__odmContent = true;

  const MIN_W = 200;
  const MIN_H = 120;
  let items = [];
  let dismissed = false;
  let host = null;
  let ui = null;
  // medialist.js runs first in the same content-script world.
  const { fmtSize, menuEntries } = self.odmMediaList;
  // ODM's replies about the streams (their qualities), by playlist URL;
  // asked once each, again when the reply was a failure that may pass.
  const infos = new Map();
  const asked = new Set();

  // The largest visible video is the player; small ones are previews.
  function mainVideo() {
    let best = null;
    let bestArea = 0;
    for (const v of document.querySelectorAll("video")) {
      const r = v.getBoundingClientRect();
      if (r.width < MIN_W || r.height < MIN_H) continue;
      const area = r.width * r.height;
      if (area > bestArea) {
        best = v;
        bestArea = area;
      }
    }
    return best;
  }

  // The button lives in a closed shadow root so the page's CSS cannot
  // restyle or hide it, and ours cannot leak into the page.
  function build() {
    host = document.createElement("div");
    host.style.cssText = "position:absolute;z-index:2147483647;top:0;left:0;";
    const root = host.attachShadow({ mode: "closed" });
    root.innerHTML = `
      <style>
        .bar { display:flex; gap:2px; font:12px/1.4 system-ui,sans-serif; }
        button { all:unset; cursor:pointer; background:#1f6feb; color:#fff; padding:4px 10px;
                 border-radius:4px; box-shadow:0 1px 4px rgba(0,0,0,.4); white-space:nowrap; }
        button:hover { background:#388bfd; }
        .x { background:#30363d; padding:4px 7px; }
        .menu, .status { font:13px/1.4 system-ui,sans-serif; }
        .menu { margin-top:4px; background:#161b22; color:#e6edf3; border-radius:4px;
                box-shadow:0 2px 8px rgba(0,0,0,.5); min-width:220px; max-height:60vh; overflow-y:auto; }
        .menu div { padding:6px 10px; cursor:pointer; white-space:nowrap; }
        .menu div:hover { background:#1f6feb; }
        .menu .off { color:#8b949e; cursor:default; }
        .menu .off:hover { background:none; }
        .menu .head { color:#8b949e; font-size:11px; padding-bottom:2px; }
        .menu .sub { padding-left:20px; }
        .status { margin-top:4px; background:#161b22; color:#e6edf3; padding:4px 10px; border-radius:4px; }
      </style>
      <div class="bar"><button class="dl"></button><button class="x" title="Hide">✕</button></div>
      <div class="menu" hidden></div>
      <div class="status" hidden></div>`;
    ui = {
      dl: root.querySelector(".dl"),
      x: root.querySelector(".x"),
      menu: root.querySelector(".menu"),
      status: root.querySelector(".status"),
    };
    ui.x.addEventListener("click", (e) => {
      e.stopPropagation();
      dismissed = true;
      host.remove();
    });
    ui.dl.addEventListener("click", (e) => {
      e.stopPropagation();
      // A stream opens the menu too, to pick its quality.
      if (items.length === 1 && items[0].kind !== "stream") send(items[0]);
      else toggleMenu();
    });
    document.documentElement.appendChild(host);
  }

  function toggleMenu() {
    if (!ui.menu.hidden) {
      ui.menu.hidden = true;
      return;
    }
    renderMenu();
    ui.menu.hidden = false;
  }

  // renderMenu lists the media, each stream with its qualities once ODM
  // has read its playlist, and asks ODM about streams not asked about yet.
  function renderMenu() {
    for (const it of items) {
      if (it.kind !== "stream" || asked.has(it.url)) continue;
      asked.add(it.url);
      chrome.runtime.sendMessage({ type: "odm-stream-info", item: it }, (reply) => {
        const r = chrome.runtime.lastError || !reply ? { ok: false, error: "ODM is not reachable" } : reply;
        infos.set(it.url, r);
        if (!r.ok && !r.refused) asked.delete(it.url); // ask again next time
        if (!ui.menu.hidden) renderMenu();
      });
    }
    ui.menu.replaceChildren();
    for (const e of menuEntries(items, infos)) {
      const row = document.createElement("div");
      row.textContent = e.text;
      if (e.heading) row.className = "head";
      else if (!e.item) row.className = "off";
      if (e.sub) row.classList.add("sub");
      if (e.item) {
        row.addEventListener("click", (ev) => {
          ev.stopPropagation();
          ui.menu.hidden = true;
          send(e.item);
        });
      }
      ui.menu.appendChild(row);
    }
  }

  function flash(text) {
    ui.status.textContent = text;
    ui.status.hidden = false;
    setTimeout(() => (ui.status.hidden = true), 3000);
  }

  function send(item) {
    chrome.runtime.sendMessage({ type: "odm-grab", item }, (reply) => {
      if (chrome.runtime.lastError || !reply) flash("ODM is not reachable");
      else if (!reply.ok) flash(`ODM: ${reply.error}`);
      else flash(reply.message === "already in the list" ? "Already in ODM" : "Sent to ODM");
    });
  }

  function label() {
    const files = items.filter((i) => i.kind === "file");
    const best = files.sort((a, b) => b.size - a.size)[0];
    if (!best) return "Download this video";
    return `Download this video${best.size ? " · " + fmtSize(best.size) : ""}`;
  }

  // Keeps the button at the video's top-right corner as the page scrolls,
  // resizes or swaps players.
  let raf = 0;
  function place() {
    raf = 0;
    if (dismissed || items.length === 0) return;
    const v = mainVideo();
    if (!v) {
      if (host) host.style.display = "none";
      return;
    }
    if (!host) build();
    host.style.display = "";
    ui.dl.textContent = label();
    const r = v.getBoundingClientRect();
    const w = host.getBoundingClientRect().width || 220;
    host.style.top = `${window.scrollY + r.top + 8}px`;
    host.style.left = `${window.scrollX + Math.max(r.left, r.right - w - 8)}px`;
  }
  const schedule = () => {
    if (!raf) raf = requestAnimationFrame(place);
  };

  function setItems(list) {
    items = Array.isArray(list) ? list : [];
    if (ui && !ui.menu.hidden) renderMenu();
    schedule();
  }

  chrome.runtime.onMessage.addListener((msg) => {
    if (msg && msg.type === "odm-media") setItems(msg.items);
  });
  chrome.runtime.sendMessage({ type: "odm-media-request" }, (list) => {
    if (!chrome.runtime.lastError) setItems(list);
  });

  window.addEventListener("scroll", schedule, { passive: true });
  window.addEventListener("resize", schedule, { passive: true });
  setInterval(schedule, 1000); // players resize or replace the <video> element

  // Alt-click on a link: let the browser download it this once.
  document.addEventListener("click", (e) => {
    if (!e.altKey) return;
    const a = e.target instanceof Element && e.target.closest("a[href]");
    if (a) chrome.runtime.sendMessage({ type: "odm-bypass", url: a.href });
  }, true);
})();
