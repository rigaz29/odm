const $ = (id) => document.getElementById(id);
const rowsEl = $("rows");
const sidebarEl = $("sidebar");
const emptyEl = $("empty");
const ctxEl = $("ctxMenu");
const addDialog = $("addDialog");
const optionsDialog = $("optionsDialog");
const progDialog = $("progDialog");

const CAT_ICON = {
  Video: "▶", Audio: "♪", Documents: "▤", Programs: "⚙",
  Compressed: "▣", Images: "▦", "Disk Images": "◎", Other: "•",
};

const state = {
  downloads: [],
  settings: { category_list: [] },
  filter: { kind: "all", value: "" },
  sort: { key: "created_at", dir: -1 },
  selected: null,
  progId: null,
  details: true,
};

/* ---------- formatting ---------- */

function fmtBytes(n) {
  if (n == null || n < 0) return "-";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0, v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}
const fmtRate = (n) => (n > 0 ? fmtBytes(n) + "/s" : "-");
// A live recording's total follows what is done: no percentage, no ETA.
const sized = (d) => d.total > 0 && !d.live;
const recording = (d) => d.live && d.status === "downloading";
function etaSeconds(d) {
  if (d.status !== "downloading" || d.speed <= 0 || !sized(d)) return -1;
  return Math.max(0, (d.total - d.done) / d.speed);
}
function fmtEta(d) {
  const s = etaSeconds(d);
  if (s < 0) return "-";
  const m = Math.floor(s / 60);
  return m > 0 ? `${m} min ${Math.floor(s % 60)} sec` : `${Math.floor(s)} sec`;
}

/* ---------- api ---------- */

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch {}
    throw new Error(msg);
  }
  return res.headers.get("content-type")?.includes("json") ? res.json() : res;
}
const post = (p, body) => api(p, body
  ? { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }
  : { method: "POST" });

let toastTimer;
function toast(msg) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add("hidden"), 4000);
}

const selectedDownload = () => state.downloads.find((d) => d.id === state.selected) || null;
const progressDownload = () => state.downloads.find((d) => d.id === state.progId) || null;
const catIcon = (c) => CAT_ICON[c] || "•";

/* ---------- toolbar ---------- */

function actionOnSelection(single, all) {
  const d = selectedDownload();
  if (d) single(d); else all();
}

async function toolbar(action) {
  try {
    switch (action) {
      case "add": openAddDialog(); return;
      case "options": openOptionsDialog(); return;
      case "resume":
        actionOnSelection((d) => post(`/api/downloads/${d.id}/resume`), () => post("/api/actions/resume-all"));
        break;
      case "stop":
        actionOnSelection((d) => post(`/api/downloads/${d.id}/pause`), () => post("/api/actions/pause-all"));
        break;
      case "stop-all": await post("/api/actions/stop-all"); break;
      case "pause-all": await post("/api/actions/pause-all"); break;
      case "resume-all": await post("/api/actions/resume-all"); break;
      case "delete": {
        const d = selectedDownload();
        if (!d) { toast("Select a download first"); return; }
        if (confirm(`Remove "${d.file_name || d.url}" from the list? (file kept)`)) {
          await api(`/api/downloads/${d.id}`, { method: "DELETE" });
        }
        break;
      }
      case "delete-completed":
        if (confirm("Remove all completed entries? Files are kept.")) await post("/api/actions/clear-completed");
        break;
    }
  } catch (e) { toast(e.message); }
}
document.querySelectorAll(".toolbar [data-act]").forEach((b) => { b.onclick = () => toolbar(b.dataset.act); });

/* ---------- context menu ---------- */

function hideCtx() { ctxEl.classList.add("hidden"); }
function showCtx(d, x, y) {
  ctxEl.replaceChildren();
  const item = (label, fn, danger) => {
    const b = document.createElement("button");
    b.textContent = label;
    if (danger) b.className = "danger";
    b.onclick = () => { hideCtx(); fn(); };
    ctxEl.appendChild(b);
  };
  item("Show progress", () => openProgress(d.id));
  if (d.status === "complete") item("Open file", () => post(`/api/downloads/${d.id}/open`));
  item("Open folder", () => post(`/api/downloads/${d.id}/reveal`));
  item("Copy URL", () => copy(d.url));
  item("Copy path", () => copy(d.file_path));
  item("Remove from list", () => {
    if (confirm(`Remove "${d.file_name || d.url}"? (file kept)`)) api(`/api/downloads/${d.id}`, { method: "DELETE" });
  }, true);
  item("Remove + delete file", () => {
    if (confirm(`Delete "${d.file_name}" from disk too?`)) api(`/api/downloads/${d.id}?file=true`, { method: "DELETE" });
  }, true);
  ctxEl.style.left = `${x}px`;
  ctxEl.style.top = `${y}px`;
  ctxEl.classList.remove("hidden");
}
async function copy(text) {
  try { await navigator.clipboard.writeText(text); toast("Copied"); }
  catch { toast("Clipboard blocked"); }
}
document.addEventListener("click", hideCtx);
window.addEventListener("blur", hideCtx);

/* ---------- sidebar + table ---------- */

function inFilter(d) {
  const f = state.filter;
  if (f.kind === "all") return true;
  if (f.kind === "unfinished") return d.status !== "complete";
  if (f.kind === "finished") return d.status === "complete";
  if (f.kind === "category") return d.category === f.value;
  return true;
}
function sortValue(d, key) {
  if (key === "eta") return etaSeconds(d);
  if (key === "progress") return d.total > 0 ? d.progress : -1;
  return d[key] ?? "";
}
function visible() {
  const list = state.downloads.filter(inFilter);
  const { key, dir } = state.sort;
  return [...list].sort((a, b) => {
    const x = sortValue(a, key), y = sortValue(b, key);
    if (typeof x === "number" && typeof y === "number") return (x - y) * dir;
    return String(x).localeCompare(String(y)) * dir;
  });
}

function renderSidebar() {
  const counts = {};
  for (const d of state.downloads) counts[d.category] = (counts[d.category] || 0) + 1;
  const unfinished = state.downloads.filter((d) => d.status !== "complete").length;
  const finished = state.downloads.filter((d) => d.status === "complete").length;

  const mk = (kind, value, label, ico, count) => {
    const el = document.createElement("div");
    el.className = "cat" + (state.filter.kind === kind && state.filter.value === value ? " active" : "");
    el.innerHTML = `<span class="ico">${ico}</span>${label}<span class="count">${count || ""}</span>`;
    el.onclick = () => { state.filter = { kind, value }; render(); };
    return el;
  };
  const group = (t) => Object.assign(document.createElement("div"), { className: "group", textContent: t });

  sidebarEl.replaceChildren(
    mk("all", "", "All Downloads", "▣", state.downloads.length),
    group("Categories"),
    ...(state.settings.category_list || []).map((name) => mk("category", name, name, catIcon(name), counts[name])),
    group("Status"),
    mk("unfinished", "", "Unfinished", "◌", unfinished),
    mk("finished", "", "Finished", "✓", finished),
  );
}

function renderRow(d) {
  const tr = document.createElement("tr");
  tr.className = `s-${d.status}` + (state.selected === d.id ? " selected" : "");

  const name = document.createElement("td");
  name.textContent = `${catIcon(d.category)}  ${d.file_name || d.url}`;
  name.title = d.url;

  const size = document.createElement("td");
  size.className = "num";
  size.textContent = d.total > 0 ? fmtBytes(d.total) : "";

  const prog = document.createElement("td");
  const pcell = document.createElement("div");
  pcell.className = "pcell";
  const bar = document.createElement("div");
  bar.className = "pbar";
  const fill = document.createElement("i");
  fill.style.width = `${sized(d) ? Math.min(100, d.progress) : 0}%`;
  bar.appendChild(fill);
  const pct = document.createElement("span");
  pct.className = "pct";
  pct.textContent = sized(d) ? `${d.progress.toFixed(1)}%` : "";
  pcell.append(bar, pct);
  prog.appendChild(pcell);

  const status = document.createElement("td");
  const badge = document.createElement("span");
  badge.className = "status";
  badge.textContent = recording(d) ? "recording" : d.status;
  status.appendChild(badge);

  const eta = document.createElement("td");
  eta.className = "num";
  eta.textContent = d.status === "downloading" ? fmtEta(d) : "";

  const rate = document.createElement("td");
  rate.className = "num";
  rate.textContent = d.status === "downloading" ? fmtRate(d.speed) : "";

  const path = document.createElement("td");
  path.textContent = d.error ? d.error : d.dir;
  if (d.error) path.className = "err";
  path.title = d.file_path || d.dir;

  tr.append(name, size, prog, status, eta, rate, path);
  tr.onclick = () => { state.selected = d.id; render(); };
  tr.ondblclick = () => openProgress(d.id);
  tr.oncontextmenu = (e) => { e.preventDefault(); state.selected = d.id; render(); showCtx(d, e.clientX, e.clientY); };
  return tr;
}

function render() {
  rowsEl.replaceChildren(...visible().map(renderRow));
  emptyEl.classList.toggle("hidden", state.downloads.length > 0);
  renderSidebar();

  const active = state.downloads.filter((d) => d.status === "downloading");
  const speed = active.reduce((a, d) => a + d.speed, 0);
  $("globalStat").textContent = active.length
    ? `${active.length} active · ${fmtRate(speed)}`
    : `${state.downloads.length} items`;

  if (progDialog.open) renderProgress();
}

document.querySelectorAll(".grid thead th").forEach((th) => {
  th.onclick = () => {
    const key = th.dataset.sort;
    state.sort = state.sort.key === key ? { key, dir: -state.sort.dir } : { key, dir: 1 };
    render();
  };
});

/* ---------- Add: "New Download" ---------- */

let probeTimer, probed = null;

function fillCategoryOptions(selected) {
  const sel = $("category");
  const list = state.settings.category_list || [];
  sel.replaceChildren(...list.map((name) => {
    const o = document.createElement("option");
    o.value = name; o.textContent = name;
    if (name === selected) o.selected = true;
    return o;
  }));
}

function openAddDialog() {
  probed = null;
  $("url").value = "";
  $("dir").value = state.settings.dir || "";
  $("fname").value = "";
  $("desc").value = "";
  $("conns").value = state.settings.connections || 8;
  $("fiSize").textContent = "—";
  $("fiType").textContent = "";
  $("fiIcon").textContent = "▤";
  $("dir").placeholder = state.settings.dir || "(default folder)";
  fillCategoryOptions("");
  addDialog.showModal();
  $("url").focus();
}

async function probeURL() {
  const url = $("url").value.trim();
  if (!/^https?:\/\//i.test(url)) return;
  try {
    const p = await api(`/api/probe?url=${encodeURIComponent(url)}`);
    probed = p;
    $("fname").value = p.file_name || "";
    $("fiSize").textContent = p.total > 0 ? fmtBytes(p.total) : "unknown size";
    $("fiType").textContent = p.content_type || "";
    $("fiIcon").textContent = catIcon(p.category);
    if (!/^https?:\/\//i.test($("dir").value)) $("dir").value = state.settings.dir || "";
    fillCategoryOptions($("category").value || p.category);
    $("category").value = p.category;
  } catch (e) {
    $("fiSize").textContent = "—";
    $("fiType").textContent = String(e.message || e);
  }
}

$("url").addEventListener("input", () => {
  clearTimeout(probeTimer);
  probeTimer = setTimeout(probeURL, 600);
});

$("addForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const kind = e.submitter ? e.submitter.value : "ok";
  if (kind === "cancel") { addDialog.close(); return; }
  const body = {
    url: $("url").value.trim(),
    dir: $("dir").value.trim(),
    file_name: $("fname").value.trim(),
    connections: parseInt($("conns").value, 10) || 0,
    category: $("category").value,
    start: kind === "ok",
  };
  if (!body.url) { toast("Enter a URL"); return; }
  try {
    await post("/api/downloads", body);
    addDialog.close();
  } catch (err) { toast(err.message); }
});

/* ---------- Options ---------- */

function fillSettings() {
  const s = state.settings;
  $("setDir").value = s.dir || "";
  $("setConns").value = s.connections || 8;
  $("setConcurrent").value = s.concurrent || 3;
  $("setLimit").value = s.speed_limit ? Math.round(s.speed_limit / 1024) : 0;
  $("setCategories").checked = !!s.categories;
  $("setClipboard").checked = !!s.clipboard;
}
function openOptionsDialog() { fillSettings(); optionsDialog.showModal(); }

$("optionsForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  if (e.submitter && e.submitter.value === "cancel") { optionsDialog.close(); return; }
  const body = {
    dir: $("setDir").value.trim(),
    connections: parseInt($("setConns").value, 10) || 8,
    concurrent: parseInt($("setConcurrent").value, 10) || 3,
    speed_limit: Math.max(0, parseInt($("setLimit").value, 10) || 0) * 1024,
    categories: $("setCategories").checked,
    clipboard: $("setClipboard").checked,
  };
  try { state.settings = { ...state.settings, ...(await api("/api/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) })) }; }
  catch (err) { toast(err.message); }
  optionsDialog.close();
});

/* ---------- Progress window ---------- */

function openProgress(id) {
  state.progId = id;
  state.selected = id;
  if (!progDialog.open) progDialog.showModal();
  renderProgress();
  render();
}

function renderProgress() {
  const d = progressDownload();
  if (!d) return;
  $("pgTitle").textContent = `${recording(d) ? "Recording " : sized(d) ? d.progress.toFixed(0) + "% " : ""}${d.file_name || d.url}`;
  $("pgUrl").textContent = d.url;
  $("pgStatus").textContent = d.error ? `${d.status} — ${d.error}` :
    recording(d) ? "Recording live stream…" : (d.status === "downloading" ? "Receiving data…" : d.status);
  $("pgSize").textContent = d.total > 0 ? fmtBytes(d.total) : "unknown";
  $("pgDone").textContent = `${fmtBytes(d.done)}${sized(d) ? ` (${d.progress.toFixed(2)} %)` : ""}`;
  $("pgRate").textContent = d.status === "downloading" ? fmtRate(d.speed) : "-";
  $("pgEta").textContent = d.status === "downloading" ? fmtEta(d) : "-";
  $("pgResume").textContent = d.range_ok ? "Yes" : "No";
  $("pgBar").style.width = `${sized(d) ? Math.min(100, d.progress) : 0}%`;

  // connection strip
  const total = d.total > 0 ? d.total : 1;
  $("pgStrip").replaceChildren(...d.chunks.map((c) => {
    const size = c.end - c.start + 1;
    const seg = document.createElement("div");
    seg.className = "segchunk" + (c.complete ? " done" : "");
    seg.style.left = `${(c.start / total) * 100}%`;
    seg.style.width = `${Math.max(0.2, (size / total) * 100)}%`;
    const inner = document.createElement("i");
    inner.style.width = `${size > 0 ? Math.min(100, (c.done / size) * 100) : 0}%`;
    seg.appendChild(inner);
    return seg;
  }));

  // per-connection table
  $("pgConns").replaceChildren(...d.chunks.map((c, i) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${i + 1}</td><td>${fmtBytes(c.done)}</td><td>${c.state}</td>`;
    return tr;
  }));

  // Pausing a recording ends it and saves the file.
  $("pgPause").textContent = recording(d) ? "Stop" : (d.status === "downloading" || d.status === "queued") ? "Pause" : "Resume";
  $("pgLimit").value = d.speed_limit ? Math.round(d.speed_limit / 1024) : 0;
  $("pgOpenWhenDone").checked = !!d.open_when_done;
}

$("pgPause").onclick = async () => {
  const d = progressDownload();
  if (!d) return;
  try {
    if (d.status === "downloading" || d.status === "queued") await post(`/api/downloads/${d.id}/pause`);
    else await post(`/api/downloads/${d.id}/resume`);
  } catch (e) { toast(e.message); }
};
$("pgClose").onclick = () => progDialog.close();
$("pgDetails").onclick = () => {
  state.details = !state.details;
  $("pgConnWrap").classList.toggle("hidden", !state.details);
  $("pgStrip").classList.toggle("hidden", !state.details);
  document.querySelector(".strip-legend").classList.toggle("hidden", !state.details);
  $("pgDetails").textContent = state.details ? "Hide details" : "Show details";
};
$("pgApplyLimit").onclick = async () => {
  const d = progressDownload();
  if (!d) return;
  const kbps = Math.max(0, parseInt($("pgLimit").value, 10) || 0);
  try { await post(`/api/downloads/${d.id}/limit`, { speed_limit: kbps * 1024 }); toast(kbps ? `Limit set to ${kbps} KB/s` : "Limit removed"); }
  catch (e) { toast(e.message); }
};
$("pgOpenWhenDone").onchange = async (e) => {
  const d = progressDownload();
  if (!d) return;
  try { await post(`/api/downloads/${d.id}/options`, { open_when_done: e.target.checked }); }
  catch (err) { toast(err.message); }
};

document.querySelectorAll(".tab").forEach((tab) => {
  tab.onclick = () => {
    document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t === tab));
    document.querySelectorAll(".tab-panel").forEach((p) => p.classList.toggle("hidden", p.dataset.panel !== tab.dataset.tab));
  };
});

/* ---------- boot ---------- */

async function boot() {
  try { state.settings = await api("/api/config"); } catch {}
  try { state.downloads = await api("/api/downloads"); } catch {}
  render();

  const es = new EventSource("/api/events");
  es.onmessage = (e) => {
    try { state.downloads = JSON.parse(e.data) || []; render(); } catch {}
  };
}
boot();
