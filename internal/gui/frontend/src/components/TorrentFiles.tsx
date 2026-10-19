// A torrent's files in the New Download window, each with a box to leave it
// out and its priority, and the pieces map of its progress window.

import type { TorrentFileRow } from "../bindings/odm/internal/gui/models";
import { humanBytes } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { Checkbox } from "./ui/checkbox";
import { Select } from "./ui/input";

// A file's priorities, as the engine counts them (engine.PriorityLow…); 0
// leaves a file out.
export const PRIORITY_LOW = 1;
export const PRIORITY_NORMAL = 4;
export const PRIORITY_HIGH = 7;

// PrioritySelect picks how soon a file downloads.
export function PrioritySelect({ value, onChange, disabled }: { value: number; onChange: (p: number) => void; disabled?: boolean }) {
  return (
    <Select
      className="w-24 shrink-0 [&>select]:h-6 [&>select]:pl-2 [&>select]:text-xs [&>svg]:right-1.5 [&>svg]:size-3.5"
      value={value || PRIORITY_NORMAL}
      disabled={disabled}
      aria-label={t("Priority")}
      onChange={(e) => onChange(Number(e.target.value))}
    >
      <option value={PRIORITY_HIGH}>{t("High")}</option>
      <option value={PRIORITY_NORMAL}>{t("Normal")}</option>
      <option value={PRIORITY_LOW}>{t("Low")}</option>
    </Select>
  );
}

// inFolder is a file's path without the torrent's own folder, as it is
// saved.
const inFolder = (path: string) => (path.includes("/") ? path.slice(path.indexOf("/") + 1) : path);

export function TorrentFiles({
  files,
  skip,
  onSkip,
  prio,
  onPrio,
}: {
  files: TorrentFileRow[];
  skip: Set<number>;
  onSkip: (s: Set<number>) => void;
  prio: Record<number, number>;
  onPrio: (p: Record<number, number>) => void;
}) {
  const real = files.map((f, i) => ({ ...f, i })).filter((f) => !f.pad);
  const chosen = real.filter((f) => !skip.has(f.i));
  const size = chosen.reduce((n, f) => n + f.size, 0);
  const all = chosen.length === real.length;
  return (
    <div className="flex min-h-0 flex-col rounded-lg border border-border">
      <label className="flex items-center gap-2.5 border-b border-border bg-muted px-3 py-1.5 text-xs font-medium text-muted-foreground">
        <Checkbox
          checked={all ? true : chosen.length === 0 ? false : "indeterminate"}
          onCheckedChange={() => onSkip(all ? new Set(real.map((f) => f.i)) : new Set())}
          aria-label={t("All files")}
        />
        <span className="flex-1">{tf("%d of %d files", chosen.length, real.length)}</span>
        <span className="tabular-nums">{humanBytes(size) || "0 B"}</span>
        <span className="w-24 shrink-0 pl-2">{t("Priority")}</span>
      </label>
      <ul className="max-h-36 overflow-auto">
        {real.map((f) => (
          <li key={f.i} className="border-b border-border last:border-b-0">
            <label className={cn("flex items-center gap-2.5 px-3 py-1 text-sm", skip.has(f.i) && "text-muted-foreground")}>
              <Checkbox
                checked={!skip.has(f.i)}
                onCheckedChange={(v) => {
                  const next = new Set(skip);
                  if (v === true) next.delete(f.i);
                  else next.add(f.i);
                  onSkip(next);
                }}
              />
              <span className="min-w-0 flex-1 truncate" title={inFolder(f.path)}>
                {inFolder(f.path)}
              </span>
              <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{humanBytes(f.size) || "0 B"}</span>
              <PrioritySelect value={prio[f.i] ?? PRIORITY_NORMAL} disabled={skip.has(f.i)} onChange={(p) => onPrio({ ...prio, [f.i]: p })} />
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}

// PieceMap draws which pieces a torrent has: base64 of a bitfield, the first
// piece in the high bit, grouped into at most 240 bars.
export function PieceMap({ pieces, count }: { pieces: string; count: number }) {
  let bytes: Uint8Array;
  try {
    bytes = Uint8Array.from(atob(pieces), (c) => c.charCodeAt(0));
  } catch {
    bytes = new Uint8Array();
  }
  const has = (i: number) => (bytes[i >> 3] ?? 0) & (0x80 >> (i & 7));
  const bars = Math.min(count, 240);
  const fill: number[] = [];
  for (let b = 0; b < bars; b++) {
    const from = Math.floor((b * count) / bars);
    const to = Math.max(from + 1, Math.floor(((b + 1) * count) / bars));
    let n = 0;
    for (let i = from; i < to; i++) if (has(i)) n++;
    fill.push(n / (to - from));
  }
  return (
    <div className="flex h-4 overflow-hidden rounded-md bg-muted ring-1 ring-inset ring-border" role="img" aria-label={t("Pieces")}>
      {fill.map((f, b) => (
        <div key={b} className="h-full flex-1">
          <div className="accent-gradient h-full" style={{ opacity: f }} />
        </div>
      ))}
    </div>
  );
}
