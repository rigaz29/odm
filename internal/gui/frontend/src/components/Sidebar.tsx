// The sidebar: all, running, unfinished and finished downloads, the
// categories and the queues, each with how many it holds; at the bottom
// the total speed over the last minute.

import { CalendarClock, CheckCircle2, Clock, Gauge, Inbox, ListOrdered, Zap } from "lucide-react";
import type { ComponentType } from "react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { type Filter, categoryLabel, isRunning, speedText } from "../format";
import { t } from "../i18n";
import { cn } from "../lib/utils";
import { Sparkline, categoryIcon } from "./bits";

type Icon = ComponentType<{ className?: string }>;

const same = (a: Filter, b: Filter) => a.kind === b.kind && ("value" in a ? a.value : "") === ("value" in b ? b.value : "");

function NavItem({ icon: I, label, count, on, live, onClick }: { icon: Icon; label: string; count: number; on: boolean; live?: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      aria-current={on ? "page" : undefined}
      className={cn(
        "relative flex h-8 w-full items-center gap-2.5 rounded-md px-2.5 text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
        on ? "bg-accent font-medium text-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground",
      )}
    >
      {on && <span className="accent-gradient absolute -left-3 top-1.5 h-5 w-[3px] rounded-r-full" />}
      <I className="size-4 shrink-0" />
      <span className="flex-1 truncate">{label}</span>
      {live && <span className="accent-gradient size-1.5 shrink-0 rounded-full" title={live} />}
      {count > 0 && <span className="text-xs tabular-nums text-muted-foreground">{count}</span>}
    </button>
  );
}

export function Sidebar(props: {
  items: Item[];
  categories: string[];
  queues: string[];
  started: string[]; // the queues that are started
  filter: Filter;
  onFilter: (f: Filter) => void;
  speed: number; // now, the status bar's figure
  speeds: number[]; // the last minute, a sample a second
}) {
  const { items, filter, onFilter } = props;
  const running = items.filter(isRunning).length;
  const unfinished = items.filter((d) => d.status !== "complete").length;
  const nav = (f: Filter, icon: Icon, label: string, count: number, live?: string) => (
    <NavItem key={label} icon={icon} label={label} count={count} on={same(filter, f)} live={live} onClick={() => onFilter(f)} />
  );
  const now = props.speed;
  return (
    <aside className="flex w-60 shrink-0 flex-col gap-5 overflow-y-auto border-r border-border p-3">
      <nav className="space-y-0.5" aria-label={t("Downloads")}>
        {nav({ kind: "all" }, Inbox, t("All downloads"), items.length)}
        {nav({ kind: "running" }, Zap, t("Running"), running)}
        {nav({ kind: "unfinished" }, Clock, t("Unfinished"), unfinished)}
        {nav({ kind: "finished" }, CheckCircle2, t("Finished"), items.length - unfinished)}
      </nav>
      <div>
        <p id="nav-categories" className="mb-1.5 px-2.5 text-xs font-medium text-muted-foreground">
          {t("Categories")}
        </p>
        <nav className="space-y-0.5" aria-labelledby="nav-categories">
          {props.categories.map((c) =>
            nav({ kind: "category", value: c }, categoryIcon(c), categoryLabel(c), items.filter((d) => d.category === c).length),
          )}
        </nav>
      </div>
      <div>
        <p id="nav-queues" className="mb-1.5 px-2.5 text-xs font-medium text-muted-foreground">
          {t("Queues")}
        </p>
        <nav className="space-y-0.5" aria-labelledby="nav-queues">
          {props.queues.map((q) =>
            nav(
              { kind: "queue", value: q },
              q === props.queues[0] ? ListOrdered : CalendarClock,
              q,
              items.filter((d) => d.queue === q && d.status !== "complete").length,
              props.started.includes(q) ? t("Queue started") : undefined,
            ),
          )}
        </nav>
      </div>
      <div className="mt-auto rounded-lg border border-border bg-card p-3">
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Gauge className="size-3.5" /> {t("Total speed")}
        </div>
        <p className={cn("mt-0.5 text-lg font-semibold tabular-nums", now > 0 ? "accent-text" : "text-muted-foreground")}>{speedText(now) || "—"}</p>
        <Sparkline values={props.speeds} className="mt-1 h-9 w-full" />
      </div>
    </aside>
  );
}
