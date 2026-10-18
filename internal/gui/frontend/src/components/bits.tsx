// Small pieces the window repeats: the coloured file tile, the status
// badge, the progress bar and the speed sparkline.

import { AppWindow, Archive, Disc3, File, FileText, Film, Image, Music } from "lucide-react";
import type { ComponentType } from "react";
import { useId } from "react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { type StatusKind, percent, statusKind, statusLabel } from "../format";
import { cn } from "../lib/utils";

type Icon = ComponentType<{ className?: string; strokeWidth?: number }>;

const tiles: Record<string, [Icon, string]> = {
  Video: [Film, "from-fuchsia-500 to-violet-600"],
  Audio: [Music, "from-pink-500 to-rose-500"],
  Documents: [FileText, "from-sky-500 to-blue-600"],
  Programs: [AppWindow, "from-zinc-500 to-zinc-700"],
  Compressed: [Archive, "from-amber-400 to-orange-500"],
  Images: [Image, "from-emerald-400 to-teal-500"],
  "Disk Images": [Disc3, "from-indigo-500 to-cyan-500"],
};

export function categoryIcon(category: string): Icon {
  return (tiles[category] ?? [File])[0];
}

// FileTile is a download's file type: its category's icon on its colour.
export function FileTile({ category, size = "size-7", icon = "size-3.5" }: { category: string; size?: string; icon?: string }) {
  const [I, gradient] = tiles[category] ?? [File, "from-zinc-400 to-zinc-600"];
  return (
    <span className={cn("flex shrink-0 items-center justify-center rounded-lg bg-gradient-to-br text-white", gradient, size)}>
      <I className={icon} strokeWidth={2.25} />
    </span>
  );
}

const badgeStyle: Record<StatusKind, string> = {
  downloading: "bg-violet-500/10 text-violet-700 ring-violet-500/25 dark:text-violet-300",
  recording: "bg-rose-500/10 text-rose-700 ring-rose-500/25 dark:text-rose-300",
  paused: "bg-amber-500/10 text-amber-700 ring-amber-500/25 dark:text-amber-300",
  queued: "bg-zinc-500/10 text-zinc-600 ring-zinc-500/20 dark:text-zinc-400",
  error: "bg-red-500/10 text-red-700 ring-red-500/25 dark:text-red-400",
  complete: "bg-emerald-500/10 text-emerald-700 ring-emerald-500/25 dark:text-emerald-400",
  verifying: "bg-sky-500/10 text-sky-700 ring-sky-500/25 dark:text-sky-300",
  seeding: "bg-teal-500/10 text-teal-700 ring-teal-500/25 dark:text-teal-300",
};

export function StatusBadge({ d }: { d: Item }) {
  const k = statusKind(d);
  return (
    <span className={cn("inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset", badgeStyle[k])}>
      {k === "recording" && <span className="size-1.5 animate-pulse rounded-full bg-rose-500" />}
      {k === "downloading" && <span className="accent-gradient size-1.5 rounded-full" />}
      {k === "seeding" && <span className="size-1.5 rounded-full bg-teal-500" />}
      {statusLabel(k)}
    </span>
  );
}

// ProgressBar is a thin bar: the accent while running, amber when
// paused, red when failed, grey when waiting.
export function ProgressBar({ d, className }: { d: Item; className?: string }) {
  const k = statusKind(d);
  const fill =
    k === "downloading" ? "accent-gradient" : k === "error" ? "bg-red-500" : k === "paused" ? "bg-amber-500" : "bg-zinc-400 dark:bg-zinc-600";
  return (
    <div className={cn("h-1.5 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border", className)}>
      <div className={cn("h-full rounded-full transition-[width] duration-500", fill)} style={{ width: `${percent(d)}%` }} />
    </div>
  );
}

// Sparkline draws values (oldest first) as a line in the accent over a
// faint fill, scaled to the largest.
export function Sparkline({ values, className }: { values: number[]; className?: string }) {
  const id = useId().replace(/:/g, "");
  const w = 100;
  const h = 32;
  const max = Math.max(...values, 1);
  const pts = values.length > 1 ? values : [values[0] ?? 0, values[0] ?? 0];
  const step = w / (pts.length - 1);
  const line = pts.map((v, i) => `${i ? "L" : "M"}${(i * step).toFixed(1)},${(h - 2 - (v / max) * (h - 4)).toFixed(1)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className={className}>
      <defs>
        <linearGradient id={`l${id}`} x1="0" x2="1">
          <stop offset="0" stopColor="#8b5cf6" />
          <stop offset="1" stopColor="#22d3ee" />
        </linearGradient>
        <linearGradient id={`f${id}`} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="#8b5cf6" stopOpacity="0.2" />
          <stop offset="1" stopColor="#8b5cf6" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={`${line} L${w},${h} L0,${h} Z`} fill={`url(#f${id})`} />
      <path d={line} fill="none" stroke={`url(#l${id})`} strokeWidth="1.75" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
