// Sizes, speeds, times, statuses and filters as the window shows them.
// Sizes step up as the Fyne UI's do (internal/ui/desktop.go: humanBytes);
// numbers use the language's decimal mark ("5,8 GB" in Indonesian).

import type { Item } from "./bindings/odm/internal/gui/models";
import { language, t, tf } from "./i18n";

// decimalText writes v with digits decimals in the language's way.
export function decimalText(v: number, digits: number): string {
  return new Intl.NumberFormat(language() === "id" ? "id-ID" : "en-US", {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
    useGrouping: false,
  }).format(v);
}

// humanBytes steps up at 1000 so 1000 MiB reads 0.98 GB, not "1000.0 MB";
// the divisor stays binary. "" for nothing or an unknown size.
export function humanBytes(n: number): string {
  if (!n || n <= 0) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  if (i === 0) return `${decimalText(v, 0)} ${units[i]}`;
  return `${decimalText(v, v < 10 ? 2 : 1)} ${units[i]}`;
}

export function speedText(bps: number): string {
  return bps > 0 ? `${humanBytes(bps)}/s` : "";
}

export const isRunning = (d: Item) => d.status === "downloading";

export function etaSeconds(d: Item): number {
  if (!isRunning(d) || d.speed <= 0 || d.total <= 0 || d.live) return -1;
  return (d.total - d.done) / d.speed;
}

// duration is "3 min 10 s", "1 h 4 min", "42 s".
export function duration(secs: number): string {
  const s = Math.max(0, Math.floor(secs));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0) return tf("%d h %d min", h, m);
  if (m > 0) return tf("%d min %d s", m, s % 60);
  return tf("%d s", s % 60);
}

export function etaText(d: Item): string {
  const secs = etaSeconds(d);
  return secs < 0 ? "" : duration(secs);
}

// clock is a recording's length as "00:42:18".
export function clock(secs: number): string {
  const s = Math.max(0, Math.floor(secs));
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(Math.floor(s / 3600))}:${p(Math.floor((s % 3600) / 60))}:${p(s % 60)}`;
}

// since is how long ago iso was, in seconds; 0 for the zero time.
export function since(iso: string, now = Date.now()): number {
  const at = Date.parse(iso);
  return isNaN(at) || at < Date.parse("2000-01-01") ? 0 : (now - at) / 1000;
}

export type StatusKind = "downloading" | "recording" | "paused" | "queued" | "error" | "complete" | "verifying" | "seeding";

export function statusKind(d: Item): StatusKind {
  if (d.verifying) return "verifying";
  if (d.seeding) return "seeding"; // a finished torrent uploading
  if (d.status === "downloading") return d.live ? "recording" : "downloading";
  return (["paused", "queued", "error", "complete"].includes(d.status) ? d.status : "queued") as StatusKind;
}

export function statusLabel(k: StatusKind): string {
  switch (k) {
    case "downloading":
      return t("Downloading");
    case "recording":
      return t("Recording");
    case "paused":
      return t("Paused");
    case "queued":
      return t("Queued");
    case "error":
      return t("Error");
    case "complete":
      return t("Complete");
    case "verifying":
      return t("Verifying checksum");
    case "seeding":
      return t("Seeding");
  }
}

export const percent = (d: Item) => (d.total > 0 ? Math.min(100, Math.floor(d.progress)) : 0);

// displayName is the name shown for a download: its file name, or until a
// probe names it (and when that failed), the last part of its address.
export function displayName(d: Item): string {
  if (d.fileName) return d.fileName;
  try {
    const u = new URL(d.url);
    const base = decodeURIComponent(u.pathname.split("/").filter(Boolean).pop() ?? "");
    return base || u.host || d.url;
  } catch {
    return d.url;
  }
}

export function host(d: Item): string {
  try {
    return new URL(d.url).hostname.replace(/^www\./, "");
  } catch {
    return "";
  }
}

// added is when a download was added: "Today, 14.20", "Yesterday, 22.10",
// or "14 Oct, 20.31".
export function added(iso: string, now = new Date()): string {
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
  const id = language() === "id";
  const time = d.toLocaleTimeString(id ? "id-ID" : "en-GB", { hour: "2-digit", minute: "2-digit" });
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86400000);
  if (diff === 0) return tf("Today, %s", time);
  if (diff === 1) return tf("Yesterday, %s", time);
  return `${d.toLocaleDateString(id ? "id-ID" : "en-GB", { day: "numeric", month: "short" })}, ${time}`;
}

// categoryLabel names a built-in category in the page's language; a
// category a person made is shown as named.
export function categoryLabel(name: string): string {
  switch (name) {
    case "Video":
      return t("Video");
    case "Audio":
      return t("Audio");
    case "Documents":
      return t("Documents");
    case "Programs":
      return t("Programs");
    case "Compressed":
      return t("Compressed");
    case "Images":
      return t("Images");
    case "Disk Images":
      return t("Disk Images");
    case "Other":
      return t("Other");
  }
  return name;
}

// The sidebar's filters, and the tabs over the list.
export type Filter =
  | { kind: "all" }
  | { kind: "running" }
  | { kind: "unfinished" }
  | { kind: "finished" }
  | { kind: "category"; value: string }
  | { kind: "queue"; value: string };

export type Tab = "all" | "running" | "finished" | "failed";

export function matchesFilter(d: Item, f: Filter): boolean {
  switch (f.kind) {
    case "all":
      return true;
    case "running":
      return isRunning(d);
    case "unfinished":
      return d.status !== "complete";
    case "finished":
      return d.status === "complete";
    case "category":
      return d.category === f.value;
    case "queue":
      return d.queue === f.value;
  }
}

export function matchesTab(d: Item, tab: Tab): boolean {
  switch (tab) {
    case "all":
      return true;
    case "running":
      return isRunning(d);
    case "finished":
      return d.status === "complete";
    case "failed":
      return d.status === "error";
  }
}

// matchesSearch wants every word of query in the name, address or
// description.
export function matchesSearch(d: Item, query: string): boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const hay = `${displayName(d)} ${d.url} ${d.description}`.toLowerCase();
  return words.every((w) => hay.includes(w));
}

// withExt gives a video's name the container of the quality picked,
// replacing one an earlier pick gave it (internal/ui: withExt).
export function withExt(name: string, ext: string): string {
  const m = /\.(mp4|webm|mkv|m4a|mp3|opus|ogg|mov|flv|3gp)$/i.exec(name);
  const base = m ? name.slice(0, -m[0].length) : name;
  return base && ext ? `${base}.${ext}` : base;
}

// looksLikeURL is a pasted address the search box offers to download.
export const looksLikeURL = (s: string) => /^https?:\/\/\S+$/i.test(s.trim());

// step is where a key moves the keyboard's row in a list of n rows: by
// delta rows (Up -1, Page Down +page...), or to the first or last. From no
// row (at -1), moving down goes to the first and up to the last.
export function step(n: number, at: number, move: number | "first" | "last"): number {
  if (n === 0) return -1;
  if (move === "first") return 0;
  if (move === "last") return n - 1;
  if (at < 0) return move > 0 ? 0 : n - 1;
  return Math.min(n - 1, Math.max(0, at + move));
}

// select applies a click on id within order (the ids as listed): plain
// picks only it, Ctrl toggles it, Shift picks the range from the anchor
// (Ctrl+Shift adds that range). It returns the new selection and anchor.
export function select(
  order: string[],
  selected: Set<string>,
  anchor: string | null,
  id: string,
  mods: { ctrl: boolean; shift: boolean },
): { selected: Set<string>; anchor: string | null } {
  if (mods.shift && anchor !== null && order.includes(anchor)) {
    const a = order.indexOf(anchor);
    const b = order.indexOf(id);
    const [lo, hi] = a < b ? [a, b] : [b, a];
    const next = mods.ctrl ? new Set(selected) : new Set<string>();
    for (let i = lo; i <= hi; i++) next.add(order[i]);
    return { selected: next, anchor };
  }
  if (mods.ctrl) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return { selected: next, anchor: id };
  }
  return { selected: new Set([id]), anchor: id };
}
