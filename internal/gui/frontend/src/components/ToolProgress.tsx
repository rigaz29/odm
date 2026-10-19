// The download of a companion program (yt-dlp, ffmpeg, odm-libtorrent), as
// the Setup window, ODM's questions and the status bar show it: a bar that
// fills with a light running over it while bytes come in, or with a
// segment sliding back and forth while the size is unknown, and how far it
// is in words.

import { Download } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import { humanBytes } from "../format";
import { tf } from "../i18n";
import { cn } from "../lib/utils";

// ToolDone is how far one program's download is: bytes so far and the
// total, 0 while unknown.
export type ToolDone = { done: number; total: number };

// useToolProgress follows the downloads of the programs, by name, as ODM
// reports them.
export function useToolProgress(): [Record<string, ToolDone>, (name: string, p: ToolDone) => void] {
  const [progress, setProgress] = useState<Record<string, ToolDone>>({});
  useEffect(() => api.onSetupProgress((p) => setProgress((m) => ({ ...m, [p.tool]: { done: p.done, total: p.total } }))), []);
  return [progress, (name, p) => setProgress((m) => ({ ...m, [name]: p }))];
}

// ToolBar is the bar for p; with no p yet (the download is about to start)
// it slides.
export function ToolBar({ p, label, className }: { p?: ToolDone; label: string; className?: string }) {
  const known = !!p && p.total > 0;
  const value = known ? Math.min(1, p.done / p.total) : 0;
  return (
    <span
      className={cn("relative block h-1.5 overflow-hidden rounded-full bg-muted", className)}
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={known ? Math.round(value * 100) : undefined}
    >
      {known ? (
        <span
          className={cn("accent-gradient block h-full rounded-full transition-[width] duration-300 ease-out", value < 1 && "odm-progress-run")}
          style={{ width: `${Math.max(value, 0.03) * 100}%` }}
        />
      ) : (
        <span className="odm-progress-slide accent-gradient absolute inset-y-0 w-1/3 rounded-full" />
      )}
    </span>
  );
}

// toolDoneText is p in words: "45% · 1.70 MB / 3.75 MB", or the bytes so
// far while the total is unknown.
export function toolDoneText(p?: ToolDone): string {
  if (!p || p.done <= 0) return "";
  if (p.total <= 0) return humanBytes(p.done);
  return `${Math.floor(Math.min(1, p.done / p.total) * 100)}% · ${humanBytes(p.done)} / ${humanBytes(p.total)}`;
}

// ToolActivity is the status bar's word on a program ODM is downloading,
// most of all one it fetches by itself (for a download, or the daily
// update): its name and a small bar. It goes a moment after the download
// is done, or once reports stop coming.
export function ToolActivity() {
  const [cur, setCur] = useState<{ tool: string; p: ToolDone } | null>(null);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const off = api.onSetupProgress((e) => {
      setCur({ tool: e.tool, p: { done: e.done, total: e.total } });
      clearTimeout(timer);
      timer = setTimeout(() => setCur(null), e.total > 0 && e.done >= e.total ? 1500 : 5000);
    });
    return () => {
      off();
      clearTimeout(timer);
    };
  }, []);
  if (!cur) return null;
  return (
    <span className="flex min-w-0 shrink-0 items-center gap-1.5 text-foreground" title={toolDoneText(cur.p)}>
      <Download className="size-3.5 text-violet-500" />
      <span className="truncate">{tf("Downloading %s", cur.tool)}</span>
      <ToolBar p={cur.p} label={cur.tool} className="w-16" />
    </span>
  );
}
