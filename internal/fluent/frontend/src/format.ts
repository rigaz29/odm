// Sizes, rates, times and statuses as the list shows them, the same way as
// the Fyne UI (internal/ui/desktop.go: humanBytes, rateText, etaText,
// statusText).

import type { Item } from "./bindings/odm/internal/fluent/models";
import { t, tf } from "./i18n";

// humanBytes steps up at 1000 so 1000 MiB reads 0.98 GB, not "1000.0 MB";
// the divisor stays binary.
export function humanBytes(n: number): string {
  if (!n || n <= 0) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  if (i === 0) return `${v.toFixed(0)} ${units[i]}`;
  return `${v < 10 ? v.toFixed(2) : v.toFixed(1)} ${units[i]}`;
}

export function rateText(d: Item): string {
  return d.status === "downloading" && d.speed > 0 ? `${humanBytes(d.speed)}/s` : "";
}

export function etaSeconds(d: Item): number {
  if (d.status !== "downloading" || d.speed <= 0 || d.total <= 0) return -1;
  return (d.total - d.done) / d.speed;
}

export function etaText(d: Item): string {
  const secs = etaSeconds(d);
  if (secs < 0) return "";
  const m = Math.floor(secs / 60);
  const s = Math.floor(secs % 60);
  return m > 0 ? tf("%dm %ds", m, s) : tf("%ds", s);
}

export function statusText(d: Item): string {
  if (d.verifying) return t("Verifying checksum");
  const pct = d.total > 0 && d.progress > 0 && !d.live ? ` · ${d.progress.toFixed(0)}%` : "";
  switch (d.status) {
    case "downloading":
      if (d.live) return t("Recording");
      return d.total > 0 ? `${d.progress.toFixed(2)}%` : t("Downloading");
    case "queued":
      return t("Queued") + pct;
    case "paused":
      return t("Paused") + pct;
    case "complete":
      return t("Complete");
    case "error":
      return d.error ? t("Error: ") + d.error : t("Error") + pct;
  }
  return d.status;
}

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

// lastTry is "15/10 11:22", as the Fyne list writes the last attempt.
export function lastTry(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getDate())}/${p(d.getMonth() + 1)} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// The category pane's filters.
export type Filter =
  | { kind: "all" }
  | { kind: "category"; value: string }
  | { kind: "unfinished" }
  | { kind: "finished" }
  | { kind: "queue"; value: string };

export function matchesFilter(d: Item, f: Filter): boolean {
  switch (f.kind) {
    case "all":
      return true;
    case "category":
      return d.category === f.value;
    case "unfinished":
      return d.status !== "complete";
    case "finished":
      return d.status === "complete";
    case "queue":
      return d.queue === f.value;
  }
}

// matchesSearch wants every word of query in the name, address or
// description.
export function matchesSearch(d: Item, query: string): boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return true;
  const hay = `${d.fileName} ${d.url} ${d.description}`.toLowerCase();
  return words.every((w) => hay.includes(w));
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
