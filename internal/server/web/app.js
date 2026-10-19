"use strict";

const $ = (id) => document.getElementById(id);

/* ---------- icons ---------- */

// The Material icons the desktop UI uses (from Fyne's theme), so both look
// alike. Each is [viewBox, inner SVG].
const ICONS = {
  add: ["0 0 24 24", '<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/>'],
  download: ["0 0 24 24", '<path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z"/>'],
  stop: ["0 0 24 24", '<path d="M6 6h12v12H6V6z"/>'],
  pause: ["0 0 24 24", '<path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>'],
  play: ["0 0 24 24", '<path d="M8 5v14l11-7z"/>'],
  delete: ["0 0 24 24", '<path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>'],
  clear: ["0 0 24 24", '<path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>'],
  cancel: ["0 0 24 24", '<path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>'],
  settings: ["0 0 20 20", '<path d="M15.95 10.78c.03-.25.05-.51.05-.78s-.02-.53-.06-.78l1.69-1.32c.15-.12.19-.34.1-.51l-1.6-2.77c-.1-.18-.31-.24-.49-.18l-1.99.8c-.42-.32-.86-.58-1.35-.78L12 2.34c-.03-.2-.2-.34-.4-.34H8.4c-.2 0-.36.14-.39.34l-.3 2.12c-.49.2-.94.47-1.35.78l-1.99-.8c-.18-.07-.39 0-.49.18l-1.6 2.77c-.1.18-.06.39.1.51l1.69 1.32c-.04.25-.07.52-.07.78s.02.53.06.78L2.37 12.1c-.15.12-.19.34-.1.51l1.6 2.77c.1.18.31.24.49.18l1.99-.8c.42.32.86.58 1.35.78l.3 2.12c.04.2.2.34.4.34h3.2c.2 0 .37-.14.39-.34l.3-2.12c.49-.2.94-.47 1.35-.78l1.99.8c.18.07.39 0 .49-.18l1.6-2.77c.1-.18.06-.39-.1-.51l-1.67-1.32zM10 13c-1.65 0-3-1.35-3-3s1.35-3 3-3 3 1.35 3 3-1.35 3-3 3z"/>'],
  info: ["0 0 24 24", '<rect x="11" y="7" width="2" height="2"/><rect x="11" y="11" width="2" height="6"/><path d="M12,2C6.48,2,2,6.48,2,12c0,5.52,4.48,10,10,10s10-4.48,10-10C22,6.48,17.52,2,12,2z M12,20c-4.41,0-8-3.59-8-8c0-4.41,3.59-8,8-8s8,3.59,8,8C20,16.41,16.41,20,12,20z"/>'],
  folderOpen: ["0 0 24 24", '<path d="M19,20H4C2.89,20 2,19.1 2,18V6C2,4.89 2.89,4 4,4H10L12,6H19A2,2 0 0,1 21,8H21L4,8V18L6.14,10H23.21L20.93,18.5C20.7,19.37 19.92,20 19,20Z"/>'],
  folder: ["0 0 24 24", '<path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>'],
  search: ["0 0 24 24", '<path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>'],
  menu: ["0 0 24 24", '<path d="M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z"/>'],
  palette: ["0 0 24 24", '<path d="M12 3c-4.97 0-9 4.03-9 9s4.03 9 9 9c.83 0 1.5-.67 1.5-1.5 0-.39-.15-.74-.39-1.01-.23-.26-.38-.61-.38-.99 0-.83.67-1.5 1.5-1.5H16c2.76 0 5-2.24 5-5 0-4.42-4.03-8-9-8zm-5.5 9c-.83 0-1.5-.67-1.5-1.5S5.67 9 6.5 9 8 9.67 8 10.5 7.33 12 6.5 12zm3-4C8.67 8 8 7.33 8 6.5S8.67 5 9.5 5s1.5.67 1.5 1.5S10.33 8 9.5 8zm5 0c-.83 0-1.5-.67-1.5-1.5S13.67 5 14.5 5s1.5.67 1.5 1.5S15.33 8 14.5 8zm3 4c-.83 0-1.5-.67-1.5-1.5S16.67 9 17.5 9s1.5.67 1.5 1.5-.67 1.5-1.5 1.5z"/>'],
  check: ["0 0 24 24", '<path d="M9,16.17L4.83,12l-1.42,1.41L9,19L21,7l-1.41-1.41L9,16.17z"/>'],
  list: ["0 0 24 24", '<path d="M9,5V9H21V5M9,19H21V15H9M9,14H21V10H9M4,9H8V5H4M4,19H8V15H4M4,14H8V10H4V14Z"/>'],
  expand: ["0 0 24 24", '<path d="M10 17l5-5-5-5v10z"/>'],
  collapse: ["0 0 24 24", '<path d="M7 10l5 5 5-5z"/>'],
  video: ["0 0 24 24", '<path d="M17 10.5V7c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1v-3.5l4 4v-11l-4 4z"/>'],
  music: ["0 0 24 24", '<path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6z"/>'],
  document: ["0 0 24 24", '<path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/>'],
  computer: ["0 0 24 24", '<path d="M20 18c1.1 0 1.99-.9 1.99-2L22 6c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 6h16v10H4V6z"/>'],
  copy: ["0 0 24 24", '<path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/>'],
  storage: ["0 0 24 24", '<path d="M2 20h20v-4H2v4zm2-3h2v2H4v-2zM2 4v4h20V4H2zm4 3H4V5h2v2zm-4 7h20v-4H2v4zm2-3h2v2H4v-2z"/>'],
  photo: ["0 0 24 24", '<circle cx="12" cy="12" r="3.2"/><path d="M9 2L7.17 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2h-3.17L15 2H9zm3 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z"/>'],
  file: ["0 0 24 24", '<path d="M13,9V3.5L18.5,9M6,2C4.89,2 4,2.89 4,4V20A2,2 0 0,0 6,22H18A2,2 0 0,0 20,20V8L14,2H6Z"/>'],
  warning: ["0 0 24 24", '<path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"/>'],
  error: ["0 0 24 24", '<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>'],
};
function icon(name) {
  const [box, body] = ICONS[name] || ICONS.file;
  return `<svg class="ic" viewBox="${box}" aria-hidden="true">${body}</svg>`;
}
function fillIcons(root) {
  root.querySelectorAll("[data-icon]").forEach((el) => el.insertAdjacentHTML("afterbegin", icon(el.dataset.icon)));
}

const CATEGORY_ICON = {
  Video: "video", Audio: "music", Documents: "document", Programs: "computer",
  Compressed: "copy", "Disk Images": "storage", Images: "photo",
};
const categoryIcon = (c) => CATEGORY_ICON[c] || "file";

/* ---------- state ---------- */

const load = (key, fallback) => {
  try { const v = localStorage.getItem(key); return v == null ? fallback : JSON.parse(v); } catch { return fallback; }
};
const save = (key, value) => { try { localStorage.setItem(key, JSON.stringify(value)); } catch {} };

const state = {
  downloads: [],
  settings: { category_list: [], queues: [] },
  filter: { kind: "all", value: "" },
  sort: load("odm-sort", { col: "", asc: true }),
  collapsed: load("odm-collapsed", {}),
  query: [],
  selected: new Set(),
  focus: null,
  items: [],
  progId: null,
  details: true,
  graph: { id: null, samples: [], last: 0 },
};

/* ---------- formatting (as in the desktop list) ---------- */

function humanBytes(n) {
  if (!(n > 0)) return "";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let v = n, i = 0;
  // Step up at 1000 so 1000 MiB reads 0.98 GB; the divisor stays binary.
  while (v >= 1000 && i < u.length - 1) { v /= 1024; i++; }
  if (i === 0) return `${v.toFixed(0)} ${u[i]}`;
  return `${v.toFixed(v < 10 ? 2 : 1)} ${u[i]}`;
}
const rateText = (d) => (d.status === "downloading" && d.speed > 0 ? humanBytes(d.speed) + "/s" : "");
function etaSeconds(d) {
  if (d.status !== "downloading" || d.speed <= 0 || d.total <= 0 || d.live) return -1;
  return (d.total - d.done) / d.speed;
}
function etaText(d) {
  const s = etaSeconds(d);
  if (s < 0) return "";
  const m = Math.floor(s / 60);
  return m > 0 ? `${m}m ${Math.floor(s % 60)}s` : `${Math.floor(s)}s`;
}
function statusText(d) {
  if (d.verifying) return "Verifying checksum";
  const pct = d.total > 0 && d.progress > 0 && !d.live ? ` · ${d.progress.toFixed(0)}%` : "";
  switch (d.status) {
    case "downloading":
      if (d.live) return "Recording";
      return d.total > 0 ? `${d.progress.toFixed(2)}%` : "Downloading";
    case "queued": return "Queued" + pct;
    case "paused": return "Paused" + pct;
    case "complete": return "Complete";
    case "error": return d.error ? "Error: " + d.error : "Error" + pct;
  }
  return d.status;
}
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
function lastAttempt(d) {
  const t = new Date(d.updated_at);
  if (isNaN(t) || t.getFullYear() < 2000) return "";
  const p = (n) => String(n).padStart(2, "0");
  return `${MONTHS[t.getMonth()]} ${t.getDate()} ${p(t.getHours())}:${p(t.getMinutes())}`;
}
function displayName(d) {
  if (d.file_name) return d.file_name;
  try {
    const u = new URL(d.url);
    const base = decodeURIComponent(u.pathname.split("/").filter(Boolean).pop() || "");
    return base || u.host || d.url;
  } catch { return d.url; }
}
const queueOf = (d) => d.queue || "Main";
const active = (d) => d.status === "downloading" || d.status === "queued";

/* ---------- api ---------- */

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch {}
    throw new Error(msg);
  }
  return (res.headers.get("content-type") || "").includes("json") ? res.json() : res;
}
const send = (method, path, body) => api(path, body === undefined ? { method } :
  { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
const post = (path, body) => send("POST", path, body);

let toastTimer;
function toast(msg, isError) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.toggle("err", !!isError);
  el.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add("hidden"), 4000);
}
const fail = (e) => toast(e.message || String(e), true);

// ask shows a confirmation and resolves to whether OK was chosen.
function ask(title, text, okLabel = "OK") {
  const dlg = $("askDialog");
  $("askTitle").textContent = title;
  $("askText").textContent = text;
  $("askOk").textContent = okLabel;
  dlg.returnValue = "";
  dlg.showModal();
  return new Promise((resolve) => dlg.addEventListener("close", () => resolve(dlg.returnValue === "ok"), { once: true }));
}

/* ---------- list ---------- */

function matchesFilter(d) {
  const { kind, value } = state.filter;
  switch (kind) {
    case "unfinished": return d.status !== "complete";
    case "finished": return d.status === "complete";
    case "category": return d.category === value;
    case "queue": return queueOf(d).toLowerCase() === value.toLowerCase();
  }
  return true;
}
function matchesSearch(d) {
  if (!state.query.length) return true;
  const hay = `${d.file_name || ""} ${d.url} ${d.description || ""}`.toLowerCase();
  return state.query.every((w) => hay.includes(w));
}
const SORT_KEYS = {
  name: (d) => (d.file_name || "").toLowerCase(),
  queue: (d) => queueOf(d).toLowerCase(),
  size: (d) => d.total,
  status: (d) => d.status,
  eta: etaSeconds,
  rate: (d) => d.speed,
  last: (d) => d.updated_at,
  desc: (d) => (d.description || "").toLowerCase(),
};
function visibleItems() {
  const list = state.downloads.filter((d) => matchesFilter(d) && matchesSearch(d));
  const key = SORT_KEYS[state.sort.col];
  if (key) {
    const dir = state.sort.asc ? 1 : -1;
    list.sort((a, b) => {
      const x = key(a), y = key(b);
      return (x < y ? -1 : x > y ? 1 : 0) * dir;
    });
  } else if (state.filter.kind === "queue") {
    list.sort((a, b) => a.order - b.order); // the run order
  }
  return list;
}

function td(cls, text) {
  const el = document.createElement("td");
  el.className = cls;
  if (text != null) el.textContent = text;
  return el;
}
function renderRow(d) {
  const tr = document.createElement("tr");
  tr.className = `s-${d.status}` + (state.selected.has(d.id) ? " selected" : "");
  tr.dataset.id = d.id;

  const name = td("c-name");
  name.innerHTML = `<div class="name">${icon(categoryIcon(d.category))}<span></span></div>`;
  name.querySelector("span").textContent = displayName(d);
  name.title = d.url;

  const status = td("c-status");
  const st = document.createElement("span");
  st.className = "st";
  st.textContent = statusText(d);
  if (d.total > 0 && !d.live && d.status !== "complete" && d.progress > 0) {
    const mini = document.createElement("span");
    mini.className = "mini";
    mini.innerHTML = `<i style="width:${Math.min(100, d.progress)}%"></i>`;
    st.appendChild(mini);
  }
  status.appendChild(st);
  status.title = statusText(d);

  tr.append(
    name,
    td("c-q", d.queue || ""),
    td("c-size", humanBytes(d.total)),
    status,
    td("c-eta", d.live ? "" : etaText(d)),
    td("c-rate", rateText(d)),
    td("c-last", lastAttempt(d)),
    td("c-desc", d.description || ""),
  );
  return tr;
}

function renderTable() {
  state.items = visibleItems();
  const present = new Set(state.items.map((d) => d.id));
  for (const id of state.selected) if (!present.has(id)) state.selected.delete(id);
  if (state.focus && !present.has(state.focus)) state.focus = null;
  $("rows").replaceChildren(...state.items.map(renderRow));

  const empty = $("empty");
  empty.classList.toggle("hidden", state.items.length > 0);
  empty.textContent = state.downloads.length === 0
    ? "No downloads yet. Use Add URL (Ctrl+N) to start one."
    : "Nothing here matches.";

  document.querySelectorAll(".grid th").forEach((th) => {
    const base = th.dataset.title || (th.dataset.title = th.textContent);
    th.textContent = th.dataset.col === state.sort.col ? `${base} ${state.sort.asc ? "▲" : "▼"}` : base;
  });
}

/* ---------- sidebar ---------- */

function renderTree() {
  const all = state.downloads;
  const label = (name, n) => (n > 0 ? `${name} (${n})` : name);
  const counts = {}, perQueue = {};
  let unfinished = 0, finished = 0;
  const queues = [...(state.settings.queues || [])];
  const known = new Set(queues.map((q) => q.toLowerCase()));
  for (const d of all) {
    counts[d.category] = (counts[d.category] || 0) + 1;
    if (d.status === "complete") finished++; else unfinished++;
    const q = queueOf(d);
    perQueue[q.toLowerCase()] = (perQueue[q.toLowerCase()] || 0) + 1;
    if (!known.has(q.toLowerCase())) { known.add(q.toLowerCase()); queues.push(q); }
  }
  const nodes = [];
  const node = (opts) => {
    const el = document.createElement("div");
    const on = !opts.header && state.filter.kind === opts.kind && (state.filter.value || "") === (opts.value || "");
    el.className = "node" + (on ? " active" : "") + (opts.header ? " header" : "");
    el.style.paddingLeft = `${4 + (opts.indent || 0) * 16}px`;
    const arrow = document.createElement("span");
    arrow.className = "arrow";
    if (opts.node) {
      arrow.innerHTML = icon(state.collapsed[opts.node] ? "expand" : "collapse");
      arrow.onclick = (e) => {
        e.stopPropagation();
        state.collapsed[opts.node] = !state.collapsed[opts.node];
        save("odm-collapsed", state.collapsed);
        renderTree();
      };
    }
    el.appendChild(arrow);
    el.insertAdjacentHTML("beforeend", icon(opts.icon));
    const text = document.createElement("span");
    text.className = "label";
    text.textContent = opts.text;
    el.appendChild(text);
    if (!opts.header) {
      el.onclick = () => {
        state.filter = { kind: opts.kind, value: opts.value || "" };
        document.body.classList.remove("nav-open");
        render();
      };
    } else if (opts.node) {
      el.onclick = () => arrow.onclick(new Event("click"));
    }
    nodes.push(el);
  };
  node({ text: label("All Downloads", all.length), kind: "all", icon: "folderOpen", node: "all" });
  if (!state.collapsed.all) {
    for (const c of state.settings.category_list || []) {
      node({ text: label(c, counts[c]), kind: "category", value: c, icon: categoryIcon(c), indent: 1 });
    }
  }
  node({ text: label("Unfinished", unfinished), kind: "unfinished", icon: "download" });
  node({ text: label("Finished", finished), kind: "finished", icon: "check" });
  node({ text: "Queues", icon: "list", header: true, node: "queues" });
  if (!state.collapsed.queues) {
    for (const q of queues) {
      node({ text: label(q, perQueue[q.toLowerCase()]), kind: "queue", value: q, icon: "folder", indent: 1 });
    }
  }
  $("tree").replaceChildren(...nodes);
}

/* ---------- status bar and toolbar ---------- */

function renderStatus() {
  const all = state.downloads;
  let items = all.length === 1 ? "1 item" : `${all.length} items`;
  if (state.query.length) items = `${state.items.length} of ${items} match the search`;
  if (state.selected.size > 1) items += ` · ${state.selected.size} selected`;
  $("stLeft").textContent = items;

  const running = all.filter((d) => d.status === "downloading");
  const queued = all.filter((d) => d.status === "queued").length;
  const speed = running.reduce((n, d) => n + (d.speed || 0), 0);
  const parts = [];
  if (running.length && speed > 0) parts.push(`${running.length} active at ${humanBytes(speed)}/s`);
  else if (running.length) parts.push(`${running.length} active`);
  if (queued) parts.push(`${queued} queued`);
  const s = state.settings;
  if (!s.limiter_off && s.speed_limit > 0) parts.push(`speed limit ${humanBytes(s.speed_limit)}/s`);
  $("stRight").textContent = parts.join(" · ");
}

const tool = (act) => document.querySelector(`.tool[data-act="${act}"]`);
function updateToolbar() {
  const sel = selectedItems();
  const enable = (act, on) => { tool(act).disabled = !on; };
  enable("resume", sel.some((d) => !active(d) && d.status !== "complete"));
  enable("stop", sel.some(active));
  enable("stop-all", state.downloads.some(active));
  enable("delete", sel.length > 0);
  enable("clear-done", state.downloads.some((d) => d.status === "complete"));
  enable("progress", !!current());
  enable("folder", !!current());
}

function render() {
  renderTable();
  renderTree();
  renderStatus();
  updateToolbar();
  if ($("progDialog").open) renderProgress();
}

/* ---------- selection ---------- */

const selectedItems = () => state.items.filter((d) => state.selected.has(d.id));
const current = () => state.items.find((d) => d.id === state.focus) || null;

function clickRow(id, e) {
  const ids = state.items.map((d) => d.id);
  if (e.shiftKey && state.focus) {
    let a = ids.indexOf(state.focus), b = ids.indexOf(id);
    if (a > b) [a, b] = [b, a];
    state.selected = new Set(ids.slice(a, b + 1)); // the anchor stays
  } else if (e.ctrlKey || e.metaKey) {
    if (state.selected.has(id)) state.selected.delete(id); else state.selected.add(id);
    state.focus = id;
  } else {
    state.selected = new Set([id]);
    state.focus = id;
  }
  render();
}

$("rows").addEventListener("click", (e) => {
  const tr = e.target.closest("tr");
  if (tr) clickRow(tr.dataset.id, e);
});
$("rows").addEventListener("dblclick", (e) => {
  const tr = e.target.closest("tr");
  if (tr) openProgress(tr.dataset.id);
});
$("rows").addEventListener("contextmenu", (e) => {
  const tr = e.target.closest("tr");
  if (!tr) return;
  e.preventDefault();
  if (!state.selected.has(tr.dataset.id)) clickRow(tr.dataset.id, {});
  else state.focus = tr.dataset.id;
  showRowMenu(e.clientX, e.clientY);
});
document.querySelectorAll(".grid th").forEach((th) => {
  th.onclick = () => {
    const col = th.dataset.col;
    state.sort = state.sort.col === col ? { col, asc: !state.sort.asc } : { col, asc: true };
    save("odm-sort", state.sort);
    render();
  };
});

/* ---------- actions ---------- */

async function each(list, fn) {
  try { await Promise.all(list.map(fn)); } catch (e) { fail(e); }
}
function resumeSelected() { return each(selectedItems().filter((d) => d.status !== "complete"), (d) => post(`/api/downloads/${d.id}/resume`)); }
function stopSelected() { return each(selectedItems().filter(active), (d) => post(`/api/downloads/${d.id}/pause`)); }
// The focused row decides the direction for the whole selection.
function toggleSelected() {
  const d = current();
  if (!d || d.status === "complete") return;
  return active(d) ? stopSelected() : resumeSelected();
}
async function removeSelected(withFile) {
  const sel = selectedItems();
  if (!sel.length) return;
  const what = sel.length > 1 ? `${sel.length} downloads` : `"${displayName(sel[0])}"`;
  const ok = await ask("Delete", withFile
    ? `Remove ${what} from the list and delete the files from disk?`
    : `Remove ${what} from the list? (files are kept)`, withFile ? "Delete files" : "Remove");
  if (ok) each(sel, (d) => send("DELETE", `/api/downloads/${d.id}${withFile ? "?file=true" : ""}`));
}
function queueTarget() { return state.filter.kind === "queue" ? state.filter.value : "*"; }

const ACTIONS = {
  "add": () => openAdd(),
  "resume": resumeSelected,
  "stop": stopSelected,
  "stop-all": () => post("/api/actions/stop-all").catch(fail),
  "delete": () => removeSelected(false),
  "clear-done": async () => {
    if (await ask("Clear Done", "Remove every finished download from the list? Files are kept.", "Clear")) {
      post("/api/actions/clear-completed").catch(fail);
    }
  },
  "options": () => openOptions(),
  "start-queue": () => post(`/api/queues/${encodeURIComponent(queueTarget())}/start`).catch(fail),
  "stop-queue": () => post(`/api/queues/${encodeURIComponent(queueTarget())}/stop`).catch(fail),
  "progress": () => { const d = current(); if (d) openProgress(d.id); },
  "folder": () => { const d = current(); if (d) post(`/api/downloads/${d.id}/reveal`).catch(fail); },
};
document.querySelectorAll(".tool[data-act]").forEach((b) => { b.onclick = () => ACTIONS[b.dataset.act](); });

/* ---------- row menu ---------- */

function hideMenu() { $("ctxMenu").classList.add("hidden"); }
function showRowMenu(x, y) {
  const d = current();
  if (!d) return;
  const menu = $("ctxMenu");
  menu.replaceChildren();
  const item = (label, key, fn) => {
    const b = document.createElement("button");
    b.innerHTML = `<span></span><kbd>${key || ""}</kbd>`;
    b.firstChild.textContent = label;
    b.onclick = () => { hideMenu(); fn(); };
    menu.appendChild(b);
  };
  const line = () => menu.appendChild(document.createElement("hr"));
  item("Show progress", "Enter", () => openProgress(d.id));
  if (d.status === "complete") item("Open file", "", () => post(`/api/downloads/${d.id}/open`).catch(fail));
  item("Open folder", "", () => post(`/api/downloads/${d.id}/reveal`).catch(fail));
  item("Copy address", "", () => navigator.clipboard.writeText(d.url).then(() => toast("Address copied"), () => toast("The browser did not allow copying", true)));
  line();
  item("Pause / Resume", "Space", toggleSelected);
  line();
  item("Remove from list", "Del", () => removeSelected(false));
  item("Remove + delete file", "Shift+Del", () => removeSelected(true));
  menu.classList.remove("hidden");
  const r = menu.getBoundingClientRect();
  menu.style.left = `${Math.max(4, Math.min(x, innerWidth - r.width - 4))}px`;
  menu.style.top = `${Math.max(4, Math.min(y, innerHeight - r.height - 4))}px`;
}
document.addEventListener("click", (e) => { if (!e.target.closest("#ctxMenu")) hideMenu(); });
window.addEventListener("blur", hideMenu);
window.addEventListener("resize", hideMenu);

/* ---------- search, sidebar drawer, theme ---------- */

$("search").addEventListener("input", (e) => {
  state.query = e.target.value.toLowerCase().split(/\s+/).filter(Boolean);
  render();
});
$("search").addEventListener("keydown", (e) => {
  if (e.key === "Escape") { e.target.value = ""; state.query = []; render(); e.target.blur(); }
});
$("navToggle").onclick = () => document.body.classList.toggle("nav-open");
$("scrim").onclick = () => document.body.classList.remove("nav-open");

const THEME_NAMES = { system: "Same as the system", light: "Light", dark: "Dark" };
function setTheme(mode) {
  document.documentElement.dataset.theme = mode;
  try { localStorage.setItem("odm-theme", mode); } catch {}
  $("themeBtn").title = `Theme: ${THEME_NAMES[mode]} (click to change)`;
}
$("themeBtn").onclick = () => {
  const order = ["system", "light", "dark"];
  const now = document.documentElement.dataset.theme || "system";
  setTheme(order[(order.indexOf(now) + 1) % order.length]);
  toast(`Theme: ${THEME_NAMES[document.documentElement.dataset.theme]}`);
};

/* ---------- keyboard ---------- */

document.addEventListener("keydown", (e) => {
  const ctrl = e.ctrlKey || e.metaKey;
  if (ctrl && e.key.toLowerCase() === "n" && !document.querySelector("dialog[open]")) {
    e.preventDefault(); openAdd(); return;
  }
  if (ctrl && e.key.toLowerCase() === "f" && !document.querySelector("dialog[open]")) {
    e.preventDefault(); $("search").focus(); $("search").select(); return;
  }
  if (document.querySelector("dialog[open]") || e.target.closest("input, select, textarea")) return;
  const ids = state.items.map((d) => d.id);
  switch (e.key) {
    case "Delete": e.preventDefault(); removeSelected(e.shiftKey); break;
    case " ": e.preventDefault(); toggleSelected(); break;
    case "Enter": { const d = current(); if (d) { e.preventDefault(); openProgress(d.id); } break; }
    case "a": case "A":
      if (ctrl) { e.preventDefault(); state.selected = new Set(ids); if (!state.focus && ids.length) state.focus = ids[0]; render(); }
      break;
    case "ArrowUp": case "ArrowDown": {
      e.preventDefault();
      const i = ids.indexOf(state.focus) + (e.key === "ArrowUp" ? -1 : 1);
      if (i >= 0 && i < ids.length) {
        clickRow(ids[i], { shiftKey: e.shiftKey });
        document.querySelector(`#rows tr[data-id="${ids[i]}"]`)?.scrollIntoView({ block: "nearest" });
      }
      break;
    }
    case "Escape": hideMenu(); document.body.classList.remove("nav-open"); break;
  }
});

/* ---------- New Download ---------- */

let probeTimer, probeSeq = 0;
// torrentUpload is a .torrent file picked or dropped on this computer, which
// is sent instead of the address. catTouched: the category was chosen by
// hand; otherwise a torrent's largest file chooses it.
let torrentUpload = null, catTouched = false;
function fillSelect(sel, list, selected) {
  sel.replaceChildren(...list.map((name) => {
    const o = document.createElement("option");
    o.value = o.textContent = name;
    o.selected = name === selected;
    return o;
  }));
}
// card fills the probe card: an icon ("spinner" while checking), tinted
// "warn" or "err"; a title and detail line; and a badge, "ok" or "warn".
function card(iconName, title, detail = "", { iconLook = "", badge = "", badgeLook = "" } = {}) {
  const ic = $("pcIcon");
  ic.className = "card-icon" + (iconLook ? " " + iconLook : "");
  ic.innerHTML = iconName === "spinner" ? '<span class="spinner"></span>' : icon(iconName);
  $("pcTitle").textContent = title;
  $("pcDetail").textContent = detail;
  const b = $("pcBadge");
  b.textContent = badge;
  b.className = "badge" + (badgeLook ? " " + badgeLook : "");
}
function idleCard() {
  card("search", "No address yet", "Paste or type one: ODM checks the file's size and type before adding it.");
}
function openAdd() {
  const s = state.settings;
  torrentUpload = null;
  catTouched = false;
  $("url").value = "";
  $("fname").value = "";
  $("desc").value = "";
  $("checksum").value = "";
  $("dir").value = s.dir || "";
  $("conns").value = s.connections || 8;
  fillSelect($("category"), s.category_list || [], "Other");
  fillSelect($("queue"), s.queues && s.queues.length ? s.queues : ["Main"], "Main");
  idleCard();
  $("addDialog").showModal();
  $("url").focus();
}
async function probe(url, seq) {
  try {
    const p = await api(`/api/probe?url=${encodeURIComponent(url)}`);
    if (seq !== probeSeq) return;
    if (!$("fname").value) $("fname").value = p.file_name || "";
    if (p.category) $("category").value = p.category;
    const name = $("fname").value || "Unnamed file";
    const parts = [p.total > 0 ? humanBytes(p.total) : "Size unknown"];
    const ct = (p.content_type || "").split(";")[0].trim();
    if (ct) parts.push(ct);
    try { parts.push(new URL(p.final_url || url).hostname); } catch {}
    card(categoryIcon(p.category), name, parts.join(" · "),
      { badge: p.range_ok ? "Resumable" : "Cannot resume", badgeLook: p.range_ok ? "ok" : "warn" });
  } catch (e) {
    if (seq === probeSeq) card("error", "The file could not be checked", e.message, { iconLook: "err" });
  }
}
// pickTorrent makes the New Download add the .torrent file f.
function pickTorrent(f) {
  torrentUpload = f;
  clearTimeout(probeTimer);
  ++probeSeq;
  $("url").value = f.name;
  card("download", f.name, `${humanBytes(f.size) || "0 B"} · a .torrent file from this computer`, { badge: "Torrent", badgeLook: "ok" });
}
$("torrentPick").addEventListener("click", () => $("torrentFile").click());
$("torrentFile").addEventListener("change", () => {
  const f = $("torrentFile").files[0];
  $("torrentFile").value = "";
  if (f) pickTorrent(f);
});
$("category").addEventListener("change", () => { catTouched = true; });
// A .torrent file dropped on the page opens the New Download with it.
const isTorrent = (f) => /\.torrent$/i.test(f.name) || f.type === "application/x-bittorrent";
document.addEventListener("dragover", (e) => {
  if ([...e.dataTransfer.items].some((i) => i.kind === "file")) e.preventDefault();
});
document.addEventListener("drop", (e) => {
  const f = [...e.dataTransfer.files].find(isTorrent);
  if (!f) return;
  e.preventDefault();
  if (!$("addDialog").open) openAdd();
  pickTorrent(f);
});
$("url").addEventListener("input", () => {
  clearTimeout(probeTimer);
  torrentUpload = null;
  const url = $("url").value.trim();
  const seq = ++probeSeq;
  if (!url) { idleCard(); return; }
  if (/^magnet:\?/i.test(url)) {
    card("download", "Magnet link", "ODM gets the torrent's files from its peers once it starts.", { badge: "Torrent", badgeLook: "ok" });
    return;
  }
  if (!/^https?:\/\//i.test(url)) {
    card("warning", "Not checked", "Only http and https addresses are checked before they are added.", { iconLook: "warn" });
    return;
  }
  card("spinner", "Checking the address…");
  probeTimer = setTimeout(() => probe(url, seq), 600);
});
$("addForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const kind = e.submitter ? e.submitter.value : "ok";
  if (kind === "cancel") { $("addDialog").close(); return; }
  const body = {
    url: $("url").value.trim(),
    dir: $("dir").value.trim(),
    file_name: $("fname").value.trim(),
    description: $("desc").value.trim(),
    connections: parseInt($("conns").value, 10) || 0,
    category: $("category").value,
    queue: $("queue").value,
    checksum: $("checksum").value.trim(),
    start: kind === "ok",
  };
  if (!body.url) { $("url").focus(); return; }
  // A torrent's largest file chooses its category, unless one was picked.
  if ((torrentUpload || /^magnet:\?/i.test(body.url)) && !catTouched) body.category = "";
  let req = { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
  if (torrentUpload) {
    const form = new FormData();
    form.append("torrent", torrentUpload);
    for (const k of ["dir", "file_name", "description", "category", "queue"]) form.append(k, body[k]);
    form.append("start", String(body.start));
    req = { method: "POST", body: form };
  }
  try {
    const res = await fetch(torrentUpload ? "/api/torrents" : "/api/downloads", req);
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
    const info = await res.json();
    $("addDialog").close();
    if (res.status === 200) toast(`"${displayName(info)}" is already in the list`);
    else if (body.start) openProgress(info.id);
  } catch (err) { fail(err); }
});

/* ---------- Options ---------- */

function openOptions() {
  const s = state.settings;
  $("setDir").value = s.dir || "";
  $("setConns").value = s.connections || 8;
  $("setConcurrent").value = s.concurrent || 3;
  $("setLimit").value = s.speed_limit ? Math.round(s.speed_limit / 1024) : "";
  $("setLimitOn").checked = !s.limiter_off && s.speed_limit > 0;
  $("setCategories").checked = !!s.categories;
  $("setClipboard").checked = !!s.clipboard;
  $("setTheme").value = document.documentElement.dataset.theme || "system";
  $("optionsDialog").showModal();
}
$("optionsForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  if (e.submitter && e.submitter.value === "cancel") { $("optionsDialog").close(); return; }
  setTheme($("setTheme").value);
  const kb = Math.max(0, parseInt($("setLimit").value, 10) || 0);
  // Only these fields: the server keeps everything else as it is.
  const body = {
    dir: $("setDir").value.trim(),
    connections: parseInt($("setConns").value, 10) || 8,
    concurrent: parseInt($("setConcurrent").value, 10) || 3,
    speed_limit: kb * 1024,
    limiter_off: !$("setLimitOn").checked,
    categories: $("setCategories").checked,
    clipboard: $("setClipboard").checked,
  };
  try {
    const applied = await send("PATCH", "/api/settings", body);
    state.settings = { ...applied, category_list: state.settings.category_list };
    $("optionsDialog").close();
    render();
  } catch (err) { fail(err); }
});

/* ---------- progress ---------- */

const progressDownload = () => state.downloads.find((d) => d.id === state.progId) || null;

function openProgress(id) {
  state.progId = id;
  if (state.graph.id !== id) state.graph = { id, samples: [], last: 0 };
  const dlg = $("progDialog");
  if (!dlg.open) dlg.showModal();
  render();
}

function renderProgress() {
  const d = progressDownload();
  if (!d) return;
  const rec = d.live && d.status === "downloading";
  $("pgTitle").textContent = rec ? `Recording ${displayName(d)}` : `${d.progress.toFixed(0)}% ${displayName(d)}`;
  $("pgUrl").textContent = d.url;
  let status = { queued: "Queued", downloading: "Receiving data…", paused: "Paused", complete: "Complete", error: "Error" }[d.status] || d.status;
  if (rec) status = "Recording live stream…";
  if (d.verifying) status = "Verifying checksum…";
  if (d.error) status += " — " + d.error;
  $("pgStatus").textContent = status;
  $("pgSize").textContent = humanBytes(d.total) || "-";
  $("pgDone").textContent = d.live ? (humanBytes(d.done) || "-") : `${humanBytes(d.done) || "-"} (${d.progress.toFixed(2)} %)`;
  $("pgRate").textContent = rateText(d) || "-";
  $("pgEta").textContent = (!d.live && etaText(d)) || "-";
  $("pgResume").textContent = d.range_ok ? "Yes" : "No";
  $("pgPath").textContent = d.file_path || d.dir || "";
  const pct = rec ? 0 : d.total > 0 ? Math.min(100, d.progress) : 0;
  $("pgBar").style.width = `${pct}%`;
  $("pgPct").textContent = rec ? "Recording" : `${pct.toFixed(1)}%`;
  $("pgPause").textContent = rec ? "Stop" : active(d) ? "Pause" : "Start";
  $("pgOpenWhenDone").checked = !!d.open_when_done;
  if (document.activeElement !== $("pgLimit")) {
    $("pgLimitOn").checked = d.speed_limit > 0;
    if (d.speed_limit > 0) $("pgLimit").value = Math.round(d.speed_limit / 1024);
    else if (!$("pgLimit").value) $("pgLimit").value = 500;
    $("pgLimit").disabled = !(d.speed_limit > 0);
  }

  // The rate of the last minute, one sample a second at most.
  const g = state.graph, now = Date.now();
  if (d.status === "downloading" && now - g.last >= 900) {
    g.samples.push(d.speed || 0);
    if (g.samples.length > 60) g.samples.shift();
    g.last = now;
  }
  drawGraph();

  // A stream has segments instead of connections.
  const strip = $("pgStrip");
  if (d.stream) {
    let segs = d.segments || 0, have = d.segments_done || 0;
    let text = segs > 0 ? `${have} of ${segs}` : "";
    if (!segs && d.status === "complete") { segs = 1; have = 1; text = "complete"; }
    if (d.live && d.status !== "complete") { text = have ? `${have} recorded` : ""; segs = 0; }
    $("pgMapLabel").textContent = `Segments: ${text || "-"}`;
    strip.replaceChildren(...(segs > 0 ? [chunkEl(0, segs, segs, have, segs)] : []));
    document.querySelector(".conns").classList.add("hidden");
  } else {
    $("pgMapLabel").textContent = "Connection map";
    const total = d.total > 0 ? d.total : 1;
    strip.replaceChildren(...(d.chunks || []).map((c) => chunkEl(c.start, c.end - c.start + 1, total, c.done, c.end - c.start + 1)));
    document.querySelector(".conns").classList.remove("hidden");
    $("pgConns").replaceChildren(...(d.chunks || []).map((c, i) => {
      const tr = document.createElement("tr");
      tr.append(td("", String(i + 1)), td("", c.done > 0 ? humanBytes(c.done) : "0 B"), td("", c.state || ""));
      return tr;
    }));
  }
}
function chunkEl(start, size, total, done, span) {
  const el = document.createElement("div");
  el.className = "chunk" + (done >= span ? " done" : "");
  el.style.left = `${(start / total) * 100}%`;
  el.style.width = `calc(${Math.max(0.3, (size / total) * 100)}% - 2px)`;
  el.innerHTML = `<i style="width:${span > 0 ? Math.min(100, (done / span) * 100) : 0}%"></i>`;
  return el;
}

// drawGraph paints the rate samples as a filled line, newest on the right,
// scaled to the peak, like the desktop progress window.
function drawGraph() {
  const canvas = $("pgGraph");
  const w = canvas.clientWidth, h = canvas.clientHeight;
  if (!w || !h) return;
  const dpr = window.devicePixelRatio || 1;
  canvas.width = Math.round(w * dpr);
  canvas.height = Math.round(h * dpr);
  const ctx = canvas.getContext("2d");
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);
  const s = state.graph.samples;
  const peak = Math.max(0, ...s);
  $("pgPeak").textContent = peak > 0 ? `Peak ${humanBytes(peak)}/s` : "";
  if (s.length < 2 || peak <= 0) return;
  const accent = getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#4f7cff";
  const top = h / 4, step = w / 59, first = 60 - s.length;
  const pt = (i) => [(first + i) * step, h - 1 - (s[i] / peak) * (h - 1 - top)];
  ctx.beginPath();
  ctx.moveTo(pt(0)[0], h);
  s.forEach((_, i) => ctx.lineTo(...pt(i)));
  ctx.lineTo(w, h);
  ctx.closePath();
  const fill = ctx.createLinearGradient(0, top, 0, h);
  fill.addColorStop(0, accent + "70");
  fill.addColorStop(1, accent + "18");
  ctx.fillStyle = fill;
  ctx.fill();
  ctx.beginPath();
  s.forEach((_, i) => (i ? ctx.lineTo(...pt(i)) : ctx.moveTo(...pt(i))));
  ctx.strokeStyle = accent;
  ctx.lineWidth = 2;
  ctx.stroke();
}

$("pgPause").onclick = () => {
  const d = progressDownload();
  if (d) post(`/api/downloads/${d.id}/${active(d) ? "pause" : "resume"}`).catch(fail);
};
// Cancel stops the transfer and closes the window; × leaves it running.
$("pgCancel").onclick = () => {
  const d = progressDownload();
  if (d && active(d)) post(`/api/downloads/${d.id}/pause`).catch(fail);
  $("progDialog").close();
};
$("pgX").onclick = () => $("progDialog").close();
$("progDialog").addEventListener("close", () => { state.progId = null; });
$("pgDetails").onclick = () => {
  state.details = !state.details;
  $("pgDetailBox").classList.toggle("hidden", !state.details);
  $("pgDetails").textContent = state.details ? "Hide details" : "Show details";
};
let limitTimer;
function applyLimit() {
  const d = progressDownload();
  if (!d) return;
  const kb = Math.max(0, parseInt($("pgLimit").value, 10) || 0);
  const on = $("pgLimitOn").checked && kb > 0;
  $("pgLimit").disabled = !$("pgLimitOn").checked;
  post(`/api/downloads/${d.id}/limit`, { speed_limit: on ? kb * 1024 : 0 }).catch(fail);
}
$("pgLimitOn").onchange = applyLimit;
$("pgLimit").oninput = () => { clearTimeout(limitTimer); limitTimer = setTimeout(applyLimit, 400); };
$("pgOpenWhenDone").onchange = (e) => {
  const d = progressDownload();
  if (d) post(`/api/downloads/${d.id}/options`, { open_when_done: e.target.checked }).catch(fail);
};
document.querySelectorAll(".tab").forEach((tab) => {
  tab.onclick = () => {
    document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t === tab));
    document.querySelectorAll(".tab-panel").forEach((p) => p.classList.toggle("hidden", p.dataset.panel !== tab.dataset.tab));
    if (tab.dataset.tab === "status") drawGraph();
  };
});

/* ---------- dialogs ---------- */

// Enter in a field submits with the dialog's main button. Left to the
// browser, it would use the first one: Download Later, or Cancel.
document.querySelectorAll("form.dlg").forEach((form) => {
  form.addEventListener("keydown", (e) => {
    const main = form.querySelector("button.primary");
    if (e.key === "Enter" && main && e.target.matches("input:not([type=checkbox])")) {
      e.preventDefault();
      form.requestSubmit(main);
    }
  });
});

/* ---------- start ---------- */

async function loadSettings() {
  try { state.settings = await api("/api/config"); } catch {}
}
async function boot() {
  fillIcons(document);
  // Labels are hidden on narrow screens; the tooltip keeps the name.
  document.querySelectorAll(".tool").forEach((b) => { if (!b.title) b.title = b.textContent.trim(); });
  setTheme(document.documentElement.dataset.theme || "system");
  await loadSettings();
  try { state.downloads = await api("/api/downloads"); } catch {}
  render();
  const es = new EventSource("/api/events");
  es.onmessage = (e) => {
    try { state.downloads = JSON.parse(e.data) || []; render(); } catch {}
  };
  // Queues and the speed limit can change in the desktop window too.
  setInterval(async () => { await loadSettings(); renderTree(); renderStatus(); }, 15000);
}
boot();
