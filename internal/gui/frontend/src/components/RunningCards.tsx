// The running downloads, as cards over the list: at most three, then "+N
// more", and the section folds away. A card shows the progress, speed,
// time left and sizes, a bar per connection and the speed over the last
// minute; a live recording shows LIVE and how long it has run.

import { ArrowDown, ChevronDown, ChevronRight, Timer } from "lucide-react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { clock, displayName, etaText, host, humanBytes, percent, since, speedText } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { FileTile, Sparkline } from "./bits";

export const MAX_CARDS = 3;

function Card({ d, onOpen }: { d: Item; onOpen: () => void }) {
  const live = d.live;
  return (
    <div
      role="button"
      tabIndex={0}
      title={t("Show progress")}
      onClick={onOpen}
      onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && (e.preventDefault(), onOpen())}
      className="flex min-w-0 cursor-default flex-col gap-2.5 rounded-xl border border-border bg-card p-3.5 shadow-xs outline-none transition-colors hover:border-ring/40 focus-visible:ring-2 focus-visible:ring-ring/50"
    >
      <div className="flex items-center gap-2.5">
        <FileTile category={d.category} size="size-8" icon="size-4" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium" title={displayName(d)}>
            {displayName(d)}
          </p>
          <p className="truncate text-xs text-muted-foreground">{host(d)}</p>
        </div>
        {live ? (
          <span className="flex items-center gap-1.5 rounded-md bg-rose-500/10 px-2 py-0.5 text-[11px] font-semibold tracking-wide text-rose-600 ring-1 ring-inset ring-rose-500/25 dark:text-rose-300">
            <span className="size-1.5 animate-pulse rounded-full bg-rose-500" /> LIVE
          </span>
        ) : d.total > 0 ? (
          <span className="text-xl font-semibold tabular-nums tracking-tight">
            {percent(d)}
            <span className="text-xs font-medium text-muted-foreground">%</span>
          </span>
        ) : null}
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
        {live || d.total <= 0 ? (
          <div className="h-full w-full animate-pulse rounded-full bg-gradient-to-r from-violet-500/50 via-fuchsia-500/50 to-cyan-400/50" />
        ) : (
          <div className="accent-gradient h-full rounded-full transition-[width] duration-500" style={{ width: `${percent(d)}%` }} />
        )}
      </div>
      {/* The sizes stay whole; when the card is narrow the time left gives way. */}
      <div className="flex min-w-0 items-center gap-2.5 whitespace-nowrap text-xs text-muted-foreground">
        <span className="flex shrink-0 items-center gap-1 text-foreground">
          <ArrowDown className="size-3.5 text-cyan-500" />
          <b className="font-semibold tabular-nums">{speedText(d.speed) || "—"}</b>
        </span>
        {live ? (
          <span className="flex min-w-0 items-center gap-1 tabular-nums">
            <Timer className="size-3.5 shrink-0" /> <span className="truncate">{clock(since(d.activeSince))}</span>
          </span>
        ) : (
          etaText(d) && (
            <span className="flex min-w-0 items-center gap-1">
              <Timer className="size-3.5 shrink-0" /> <span className="truncate">{etaText(d)}</span>
            </span>
          )
        )}
        <span className="ml-auto shrink-0 tabular-nums">{!live && d.total > 0 ? `${humanBytes(d.done)} / ${humanBytes(d.total)}` : humanBytes(d.done)}</span>
      </div>
      <div className="flex items-end justify-between gap-3">
        {d.conns.length > 0 ? (
          <div className="flex min-w-0 items-center gap-[3px] whitespace-nowrap" title={t("Progress per connection")}>
            {d.conns.map((c, k) => (
              <span key={k} className="h-1 w-3 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
                <span className="accent-gradient block h-full rounded-full" style={{ width: `${Math.round(c * 100)}%` }} />
              </span>
            ))}
            <span className="ml-1.5 text-[11px] text-muted-foreground">{tf("%d connections", d.conns.length)}</span>
          </div>
        ) : (
          <span className="truncate text-[11px] uppercase text-muted-foreground">{d.stream || (d.site ? "yt-dlp" : "")}</span>
        )}
        <Sparkline values={d.speeds} className="h-7 w-20 shrink-0" />
      </div>
    </div>
  );
}

export function RunningCards(props: {
  running: Item[];
  open: boolean;
  onToggle: () => void;
  onShowAll: () => void;
  onOpen: (d: Item) => void;
}) {
  const { running } = props;
  if (running.length === 0) return null;
  const more = running.length - MAX_CARDS;
  return (
    <section className="px-6 pb-4">
      <div className="mb-2 flex items-center gap-2">
        <button onClick={props.onToggle} className="flex items-center gap-2 rounded-md text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring/50">
          {props.open ? <ChevronDown className="size-4 text-muted-foreground" /> : <ChevronRight className="size-4 text-muted-foreground" />}
          {t("Running")}
        </button>
        <span className="rounded-md bg-muted px-1.5 text-xs tabular-nums text-muted-foreground">{running.length}</span>
        {more > 0 && props.open && (
          <button onClick={props.onShowAll} className="ml-auto text-xs font-medium text-brand hover:underline">
            {tf("+%d more", more)}
          </button>
        )}
      </div>
      {props.open && (
        <div className={cn("grid gap-3", running.length === 1 ? "grid-cols-[minmax(0,26rem)]" : "grid-cols-3")}>
          {running.slice(0, MAX_CARDS).map((d) => (
            <Card key={d.id} d={d} onOpen={() => props.onOpen(d)} />
          ))}
        </div>
      )}
    </section>
  );
}
