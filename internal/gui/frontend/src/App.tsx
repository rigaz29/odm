// The main window: title bar, sidebar, the running downloads' cards, the
// list and the status bar, in the look of docs/GUI.md (shadcn/ui with
// touches of Aurora).

import type { SortingState } from "@tanstack/react-table";
import { CalendarClock, Inbox, ListOrdered, Pause, Play, Plus, Settings, Square, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, inODM } from "./api";
import type { Appearance, Item, Snapshot } from "./bindings/odm/internal/gui/models";
import { AddUrlDialog } from "./components/AddUrlDialog";
import { type Action, DownloadTable } from "./components/DownloadTable";
import { RunningCards } from "./components/RunningCards";
import { Sidebar } from "./components/Sidebar";
import { TitleBar } from "./components/TitleBar";
import { Button } from "./components/ui/button";
import { Tooltip } from "./components/ui/tooltip";
import {
  type Filter,
  type Tab,
  categoryLabel,
  displayName,
  etaSeconds,
  humanBytes,
  isRunning,
  looksLikeURL,
  matchesFilter,
  matchesSearch,
  matchesTab,
  select,
  speedText,
  statusKind,
} from "./format";
import { setLanguage, t, tf } from "./i18n";
import { cn } from "./lib/utils";

const rank: Record<string, number> = { downloading: 0, recording: 0, verifying: 1, queued: 2, paused: 3, error: 4, complete: 5 };

// compareBy orders the list by a column.
const compareBy: Record<string, (a: Item, b: Item) => number> = {
  name: (a, b) => displayName(a).localeCompare(displayName(b), undefined, { numeric: true, sensitivity: "base" }),
  size: (a, b) => a.total - b.total,
  progress: (a, b) => a.progress - b.progress,
  speed: (a, b) => a.speed - b.speed,
  eta: (a, b) => etaSeconds(a) - etaSeconds(b),
  status: (a, b) => rank[statusKind(a)] - rank[statusKind(b)] || a.progress - b.progress,
  added: (a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt),
};

function load<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(key);
    return v === null ? fallback : (JSON.parse(v) as T);
  } catch {
    return fallback;
  }
}

function save(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* storage unavailable: the choice lasts this session */
  }
}

export function App() {
  const [appearance, setAppearance] = useState<Appearance | null>(null);
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [systemDark, setSystemDark] = useState(() => matchMedia("(prefers-color-scheme: dark)").matches);
  const [filter, setFilter] = useState<Filter>({ kind: "all" });
  const [tab, setTab] = useState<Tab>("all");
  const [query, setQuery] = useState("");
  const [sorting, setSorting] = useState<SortingState>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [anchor, setAnchor] = useState<string | null>(null);
  const [cardsOpen, setCardsOpen] = useState(() => load("odm.cards.open", true));
  const [adding, setAdding] = useState<string | null>(null); // the dialog's address, null when closed
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void api.appearance().then((a) => {
      setLanguage(a.language);
      setAppearance(a);
    });
    void api.snapshot().then(setSnap);
    return api.onSnapshot(setSnap);
  }, []);

  useEffect(() => {
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const on = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      const ctrl = e.ctrlKey || e.metaKey;
      if (ctrl && (e.key.toLowerCase() === "k" || e.key.toLowerCase() === "f")) {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      } else if (ctrl && e.key.toLowerCase() === "n") {
        e.preventDefault();
        setAdding("");
      }
    };
    window.addEventListener("keydown", on);
    return () => window.removeEventListener("keydown", on);
  }, []);

  const dark = appearance?.theme === "dark" || (appearance?.theme !== "light" && systemDark);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
  }, [dark]);

  const items = snap?.items ?? [];
  const running = useMemo(() => items.filter(isRunning), [items]);
  const visible = useMemo(() => {
    const v = items.filter((d) => matchesFilter(d, filter) && matchesTab(d, tab) && matchesSearch(d, query));
    const s = sorting[0];
    if (s && compareBy[s.id]) {
      const c = compareBy[s.id];
      v.sort((a, b) => (s.desc ? c(b, a) : c(a, b)));
    }
    return v;
  }, [items, filter, tab, query, sorting]);
  const order = useMemo(() => visible.map((d) => d.id), [visible]);

  const chosen = items.filter((d) => selected.has(d.id));
  const current: Item | undefined = items.find((d) => d.id === anchor && selected.has(d.id)) ?? chosen[0];
  const resumable = chosen.filter((d) => d.status === "paused" || d.status === "error" || d.status === "queued");
  const pausable = chosen.filter((d) => d.status === "downloading" || d.status === "queued");
  const queue = filter.kind === "queue" ? filter.value : (snap?.queues[0] ?? "Main");
  const queueRunning = (snap?.running ?? []).includes(queue);

  const act = useCallback(
    (a: Action | "pauseAll" | "clean") => {
      switch (a) {
        case "open":
          if (current?.status === "complete") void api.openFile(current.id);
          break;
        case "folder":
          if (current) void api.showInFolder(current.id);
          break;
        case "resume":
          void api.resume(resumable.map((d) => d.id));
          break;
        case "pause":
          void api.pause(pausable.map((d) => d.id));
          break;
        case "remove":
          void api.remove(chosen.map((d) => d.id));
          setSelected(new Set());
          break;
        case "pauseAll":
          void api.pauseAll();
          break;
        case "clean":
          void api.clearCompleted();
          break;
      }
    },
    [current, resumable, pausable, chosen],
  );

  const press = (id: string, mods: { ctrl: boolean; shift: boolean }) => {
    const next = select(order, selected, anchor, id, mods);
    setSelected(next.selected);
    setAnchor(next.anchor);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const ctrl = e.ctrlKey || e.metaKey;
    if (ctrl && e.key.toLowerCase() === "a") {
      e.preventDefault();
      setSelected(new Set(order));
    } else if (e.key === "Delete") {
      act("remove");
    } else if (e.key === "Enter") {
      act("open");
    } else if (e.key === " ") {
      e.preventDefault();
      act(pausable.length > 0 ? "pause" : "resume");
    } else if (e.key === "Escape") {
      setSelected(new Set());
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (order.length === 0) return;
      const at = current ? order.indexOf(current.id) : -1;
      const to = Math.min(order.length - 1, Math.max(0, at + (e.key === "ArrowDown" ? 1 : -1)));
      if (e.shiftKey) {
        setSelected(select(order, selected, anchor ?? order[at] ?? null, order[to], { ctrl: false, shift: true }).selected);
      } else {
        setSelected(new Set([order[to]]));
        setAnchor(order[to]);
      }
      document.querySelector(`[data-odm-list] [data-index="${to}"]`)?.scrollIntoView({ block: "nearest" });
    }
  };

  if (!appearance) return null;

  const unfinished = items.filter((d) => d.status !== "complete").length;
  const title =
    filter.kind === "all"
      ? t("All downloads")
      : filter.kind === "running"
        ? t("Running")
        : filter.kind === "unfinished"
          ? t("Unfinished")
          : filter.kind === "finished"
            ? t("Finished")
            : filter.kind === "category"
              ? categoryLabel(filter.value)
              : filter.value;
  const all = items.length === 1 ? t("1 item") : tf("%d items", items.length);
  let left = query ? tf("%d of %s match the search", visible.length, all) : all;
  if (chosen.length > 0) left += tf(" · %d selected", chosen.length);
  const totalSpeed = running.reduce((n, d) => n + d.speed, 0);
  const tabs: [Tab, string][] = [
    ["all", t("All")],
    ["running", t("Running")],
    ["finished", t("Finished")],
    ["failed", t("Failed")],
  ];
  const later = t("Comes with the new Options and Scheduler windows");

  return (
    <div className="aurora flex h-full flex-col bg-background font-sans text-foreground">
      <TitleBar
        query={query}
        onQuery={setQuery}
        searchRef={searchRef}
        onSubmit={() => {
          if (looksLikeURL(query)) {
            setAdding(query.trim());
            setQuery("");
          }
        }}
      />
      <div className="flex min-h-0 flex-1">
        <Sidebar
          items={items}
          categories={snap?.categories ?? []}
          queues={snap?.queues ?? []}
          filter={filter}
          onFilter={(f) => {
            setFilter(f);
            setTab("all");
          }}
          speed={totalSpeed}
          speeds={snap?.speeds ?? []}
        />
        <main className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-3 px-6 pb-3 pt-5">
            <div className="min-w-0 flex-1">
              <h1 className="truncate text-lg font-semibold tracking-tight">{title}</h1>
              <p className="text-sm text-muted-foreground">
                {tf("%d running · %d unfinished · %s downloaded", running.length, unfinished, humanBytes(items.reduce((n, d) => n + d.done, 0)) || "0 B")}
              </p>
            </div>
            <Button disabled={resumable.length === 0} onClick={() => act("resume")}>
              <Play /> {t("Resume")}
            </Button>
            <Button disabled={pausable.length === 0} onClick={() => act("pause")}>
              <Pause /> {t("Pause")}
            </Button>
            <Tooltip label={t("Remove from list")}>
              <Button size="icon" disabled={chosen.length === 0} onClick={() => act("remove")} aria-label={t("Remove from list")}>
                <Trash2 />
              </Button>
            </Tooltip>
            <Button variant="brand" onClick={() => setAdding("")}>
              <Plus /> {t("Add URL")}
            </Button>
          </div>

          <RunningCards
            running={running}
            open={cardsOpen}
            onToggle={() => {
              setCardsOpen(!cardsOpen);
              save("odm.cards.open", !cardsOpen);
            }}
            onShowAll={() => {
              setFilter({ kind: "running" });
              setTab("all");
            }}
            onOpen={() => {}}
          />

          <div className="flex items-center gap-2 px-6 pb-3">
            <div className="inline-flex h-8 items-center rounded-lg bg-muted p-0.5 text-sm">
              {tabs.map(([id, label]) => (
                <button
                  key={id}
                  onClick={() => setTab(id)}
                  className={cn("rounded-md px-3 py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring/50", tab === id ? "bg-background font-medium shadow-sm" : "text-muted-foreground hover:text-foreground")}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="ml-auto flex items-center gap-1">
              {running.length > 0 && (
                <Button variant="ghost" onClick={() => act("pauseAll")}>
                  <Pause /> {t("Pause all")}
                </Button>
              )}
              <Button variant="ghost" onClick={() => (queueRunning ? api.stopQueue(queue) : api.startQueue(queue))}>
                {queueRunning ? <Square /> : <ListOrdered />} {queueRunning ? t("Stop queue") : t("Start queue")}
              </Button>
              <Tooltip label={`${t("Scheduler")} · ${later}`}>
                <span>
                  <Button variant="ghost" disabled>
                    <CalendarClock /> {t("Scheduler")}
                  </Button>
                </span>
              </Tooltip>
              <Tooltip label={`${t("Options")} · ${later}`}>
                <span>
                  <Button variant="ghost" size="icon" disabled aria-label={t("Options")}>
                    <Settings />
                  </Button>
                </span>
              </Tooltip>
            </div>
          </div>

          <DownloadTable
            items={visible}
            selected={selected}
            sorting={sorting}
            onSorting={setSorting}
            onPress={press}
            onToggle={(id) => press(id, { ctrl: true, shift: false })}
            onToggleAll={() => setSelected(visible.every((d) => selected.has(d.id)) ? new Set() : new Set(order))}
            onContext={(id) => {
              if (!selected.has(id)) setSelected(new Set([id]));
              setAnchor(id);
            }}
            onOpen={(d) => d.status === "complete" && void api.openFile(d.id)}
            onAction={act}
            onKeyDown={onKeyDown}
            can={{ open: current?.status === "complete", resume: resumable.length > 0, pause: pausable.length > 0 }}
            empty={
              <div className="flex flex-col items-center gap-3 text-center">
                <span className="flex size-12 items-center justify-center rounded-xl border border-border bg-muted">
                  <Inbox className="size-5 text-muted-foreground" />
                </span>
                <div>
                  <p className="text-sm font-medium">{query ? t("Nothing matches the search") : t("No downloads here")}</p>
                  <p className="text-xs text-muted-foreground">{t("Downloads you add or send from the browser appear here.")}</p>
                </div>
                {!query && (
                  <Button variant="brand" size="sm" onClick={() => setAdding("")}>
                    <Plus /> {t("Add URL")}
                  </Button>
                )}
              </div>
            }
          />
        </main>
      </div>

      <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-border px-4 text-xs text-muted-foreground">
        <span>{left}</span>
        {!inODM && <span className="rounded bg-muted px-1.5">{t("Sample data")}</span>}
        <span className="ml-auto flex items-center gap-1.5">
          <span className={cn("size-1.5 rounded-full", running.length > 0 ? "accent-gradient" : "bg-zinc-400")} /> {tf("%d active", running.length)}
        </span>
        <span className="tabular-nums">↓ {speedText(totalSpeed) || "0 B/s"}</span>
        <span>{snap && snap.limit > 0 ? tf("Speed limit %s", speedText(snap.limit)) : t("No speed limit")}</span>
      </footer>

      <AddUrlDialog
        open={adding !== null}
        initial={adding ?? ""}
        onClose={() => setAdding(null)}
        onAdd={async (url) => {
          await api.addURL(url);
        }}
      />
    </div>
  );
}
