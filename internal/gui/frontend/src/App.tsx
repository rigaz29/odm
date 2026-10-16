// The main window: title bar, sidebar, the running downloads' cards, the
// list and the status bar, in the look of docs/GUI.md (shadcn/ui with
// touches of Aurora).

import {
  Ban,
  CalendarClock,
  CheckCircle2,
  CirclePause,
  CirclePlay,
  Clock,
  Eraser,
  Gauge,
  Inbox,
  ListOrdered,
  PanelLeft,
  Pause,
  Play,
  Plus,
  Settings,
  Square,
  Trash2,
  Zap,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, inODM } from "./api";
import type { Item, Snapshot } from "./bindings/odm/internal/gui/models";
import { type ColumnId, type ColumnState, type Sort, nextSort, readColumns, readSort } from "./columns";
import { AddForm } from "./components/AddForm";
import { type Confirm, ConfirmDialog } from "./components/ConfirmDialog";
import { DownloadTable, type TableHandle } from "./components/DownloadTable";
import { MoreMenu } from "./components/MoreMenu";
import { RowMenu } from "./components/RowMenu";
import { RunningCards } from "./components/RunningCards";
import { Sidebar } from "./components/Sidebar";
import { SpeedLimiter } from "./components/SpeedLimiter";
import { TitleBar } from "./components/TitleBar";
import { categoryIcon } from "./components/bits";
import { Button } from "./components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./components/ui/dialog";
import { Input } from "./components/ui/input";
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
  step,
} from "./format";
import { t, tf } from "./i18n";
import { cn } from "./lib/utils";
import type { Command } from "./palette";

const rank: Record<string, number> = { downloading: 0, recording: 0, verifying: 1, queued: 2, paused: 3, error: 4, complete: 5 };
const text = (a: string, b: string) => a.localeCompare(b, undefined, { numeric: true, sensitivity: "base" });

// compareBy orders the list by a column.
const compareBy: Record<ColumnId, (a: Item, b: Item) => number> = {
  name: (a, b) => text(displayName(a), displayName(b)),
  queue: (a, b) => text(a.queue, b.queue) || a.order - b.order,
  size: (a, b) => a.total - b.total,
  progress: (a, b) => a.progress - b.progress,
  speed: (a, b) => a.speed - b.speed,
  eta: (a, b) => etaSeconds(a) - etaSeconds(b),
  status: (a, b) => rank[statusKind(a)] - rank[statusKind(b)] || a.progress - b.progress,
  added: (a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt),
  lastTry: (a, b) => Date.parse(a.updatedAt) - Date.parse(b.updatedAt),
  description: (a, b) => text(a.description, b.description),
};

// The window's own choices, kept in WebView2's profile.
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

const resumableStatus = (d: Item) => d.status === "paused" || d.status === "error" || d.status === "queued";
const pausableStatus = (d: Item) => d.status === "downloading" || d.status === "queued";

export function App() {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [filter, setFilter] = useState<Filter>({ kind: "all" });
  const [tab, setTab] = useState<Tab>("all");
  const [query, setQuery] = useState("");
  const [sort, setSortState] = useState<Sort>(() => readSort(load("odm.sort", null)));
  const [columns, setColumnsState] = useState<ColumnState>(() => readColumns(load("odm.columns", null)));
  const [sidebar, setSidebarState] = useState(() => load("odm.sidebar", true));
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [anchor, setAnchor] = useState<string | null>(null); // where Shift ranges start
  const [lead, setLead] = useState<string | null>(null); // the row the keyboard is on
  const [cardsOpen, setCardsOpen] = useState(() => load("odm.cards.open", true));
  const [adding, setAdding] = useState<string | null>(null); // the dialog's address, null when closed
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  const [readdress, setReaddress] = useState<{ item: Item; url: string; error: string; busy: boolean } | null>(null);
  const [limiterOpen, setLimiterOpen] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const table = useRef<TableHandle>(null);

  const setSort = (s: Sort) => {
    setSortState(s);
    save("odm.sort", s);
  };
  const setColumns = (s: ColumnState) => {
    setColumnsState(s);
    save("odm.columns", s);
  };
  const setSidebar = (v: boolean) => {
    setSidebarState(v);
    save("odm.sidebar", v);
  };

  useEffect(() => {
    void api.snapshot().then(setSnap);
    return api.onSnapshot(setSnap);
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

  const items = snap?.items ?? [];
  const queues = snap?.queues ?? [];
  const running = useMemo(() => items.filter(isRunning), [items]);
  // A pasted address is for downloading (the search box offers it), not a
  // search.
  const search = looksLikeURL(query) ? "" : query;
  const visible = useMemo(() => {
    const v = items.filter((d) => matchesFilter(d, filter) && matchesTab(d, tab) && matchesSearch(d, search));
    if (sort) {
      const c = compareBy[sort.id];
      v.sort((a, b) => (sort.desc ? c(b, a) : c(a, b)));
    } else if (filter.kind === "queue") {
      // A queue shows its run order, so Move up and Move down show.
      v.sort((a, b) => a.order - b.order);
    }
    return v;
  }, [items, filter, tab, search, sort]);
  const order = useMemo(() => visible.map((d) => d.id), [visible]);

  // The selection in list order; current is the row actions about one
  // download use (Open file): the keyboard's row if selected, else the first.
  const chosen = visible.filter((d) => selected.has(d.id));
  const current: Item | undefined = chosen.find((d) => d.id === lead) ?? chosen[0];
  const resumable = chosen.filter(resumableStatus);
  const pausable = chosen.filter(pausableStatus);
  const queue = filter.kind === "queue" ? filter.value : (queues[0] ?? "Main");
  const startedQueues = snap?.running ?? [];
  const queueRunning = startedQueues.includes(queue);
  const limit = snap?.limit ?? 0;
  const savedLimit = snap?.savedLimit ?? 0;

  // Rows that left the list (removed, filtered away) leave the selection.
  useEffect(() => {
    const present = new Set(order);
    if ([...selected].some((id) => !present.has(id))) setSelected(new Set([...selected].filter((id) => present.has(id))));
  }, [order, selected]);

  const focusRow = (to: number, shift: boolean) => {
    if (to < 0) return;
    const id = order[to];
    if (shift) {
      const from = anchor && order.includes(anchor) ? anchor : (lead ?? id);
      setSelected(select(order, selected, from, id, { ctrl: false, shift: true }).selected);
      setAnchor(from);
    } else {
      setSelected(new Set([id]));
      setAnchor(id);
    }
    setLead(id);
    table.current?.scrollTo(to);
  };

  const askRemove = useCallback((list: Item[], withFiles: boolean) => {
    if (list.length === 0) return;
    const what = list.length === 1 ? `“${displayName(list[0])}”` : tf("%d downloads", list.length);
    setConfirm({
      title: withFiles ? t("Remove + delete file") : t("Remove from list"),
      message: withFiles ? tf("Remove %s from the list and delete the files from disk?", what) : tf("Remove %s from the list? (files are kept)", what),
      yes: withFiles ? t("Delete") : t("Remove"),
      destructive: true,
      run: () => {
        void api.remove(
          list.map((d) => d.id),
          withFiles,
        );
        setSelected(new Set());
      },
    });
  }, []);

  // open is a double click or Enter: a finished download's file, else its
  // progress window.
  const open = (d: Item | undefined) => d && void (d.status === "complete" ? api.openFile(d.id) : api.openProgress(d.id));

  const rowActions = {
    open: () => open(current),
    progress: () => current && void api.openProgress(current.id),
    readdress: () => current && setReaddress({ item: current, url: current.url, error: "", busy: false }),
    folder: () => current && void api.showInFolder(current.id),
    copy: () => chosen.length > 0 && void api.copyText(chosen.map((d) => d.url).join("\n")),
    resume: () => void api.resume(resumable.map((d) => d.id)),
    pause: () => void api.pause(pausable.map((d) => d.id)),
    moveTo: (q: string) =>
      void api.moveToQueue(
        chosen.map((d) => d.id),
        q,
      ),
    move: (delta: number) => current && void api.moveInQueue(current.id, delta),
    remove: (withFiles: boolean) => askRemove(chosen, withFiles),
  };

  // toggle is Space: pause the selection when the keyboard's row runs or
  // waits, resume it when that row is stopped, as the Fyne UI does.
  const toggle = () => {
    if (!current || current.status === "complete") return;
    if (pausableStatus(current)) rowActions.pause();
    else rowActions.resume();
  };

  const toggleLimiter = () => {
    if (limit > 0) void api.setSpeedLimiter(false, 0);
    else if (savedLimit > 0) void api.setSpeedLimiter(true, 0);
    else setLimiterOpen(true);
  };

  const press = (id: string, mods: { ctrl: boolean; shift: boolean }) => {
    const next = select(order, selected, anchor, id, mods);
    setSelected(next.selected);
    setAnchor(next.anchor);
    setLead(id);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const ctrl = e.ctrlKey || e.metaKey;
    const at = lead ? order.indexOf(lead) : -1;
    const page = table.current?.pageSize() ?? 10;
    const moves: Record<string, number | "first" | "last"> = { ArrowDown: 1, ArrowUp: -1, PageDown: page, PageUp: -page, Home: "first", End: "last" };
    if (e.key in moves && !ctrl) {
      e.preventDefault();
      focusRow(step(order.length, at, moves[e.key]), e.shiftKey);
    } else if (ctrl && e.key.toLowerCase() === "a") {
      e.preventDefault();
      setSelected(new Set(order));
    } else if (ctrl && e.key.toLowerCase() === "c") {
      e.preventDefault();
      rowActions.copy();
    } else if (e.key === "Delete") {
      askRemove(chosen, e.shiftKey);
    } else if (e.key === "Enter") {
      rowActions.open();
    } else if (e.key === " ") {
      e.preventDefault();
      toggle();
    } else if (e.key === "Escape") {
      setSelected(new Set());
    }
  };

  const unfinished = items.filter((d) => d.status !== "complete").length;
  const filterTitle = (f: Filter) =>
    f.kind === "all"
      ? t("All downloads")
      : f.kind === "running"
        ? t("Running")
        : f.kind === "unfinished"
          ? t("Unfinished")
          : f.kind === "finished"
            ? t("Finished")
            : f.kind === "category"
              ? categoryLabel(f.value)
              : f.value;
  const all = items.length === 1 ? t("1 item") : tf("%d items", items.length);
  let left = search ? tf("%d of %s match the search", visible.length, all) : all;
  if (chosen.length > 0) left += tf(" · %d selected", chosen.length);
  const totalSpeed = running.reduce((n, d) => n + d.speed, 0);
  const tabs: [Tab, string][] = [
    ["all", t("All")],
    ["running", t("Running")],
    ["finished", t("Finished")],
    ["failed", t("Failed")],
  ];
  const later = t("Comes with the new Options and Scheduler windows");
  const limiterLabel = limit > 0 ? tf("Speed limiter (%s/s)", humanBytes(limit)) : t("Speed limiter");
  const goTo = (f: Filter) => () => {
    setFilter(f);
    setTab("all");
  };

  // The commands the search box offers (Ctrl K).
  const actions = t("Actions");
  const show = t("Show");
  const commands: Command[] = [
    { id: "add", group: actions, label: t("Add URL…"), english: "Add URL new download", icon: Plus, shortcut: "Ctrl+N", run: () => setAdding("") },
    { id: "resume-all", group: actions, label: t("Resume all"), english: "Resume all", icon: CirclePlay, run: () => void api.resumeAll() },
    { id: "pause-all", group: actions, label: t("Pause all"), english: "Pause all", icon: CirclePause, run: () => void api.pauseAll() },
    { id: "stop-all", group: actions, label: t("Stop all (and stop queues)"), english: "Stop all queues", icon: Ban, run: () => void api.stopAll() },
    { id: "clear", group: actions, label: t("Clear completed"), english: "Clear completed finished", icon: Eraser, run: () => void api.clearCompleted() },
    ...queues.map(
      (q): Command =>
        startedQueues.includes(q)
          ? { id: `stop-${q}`, group: actions, label: tf("Stop queue “%s”", q), english: `Stop queue ${q}`, icon: Square, run: () => void api.stopQueue(q) }
          : { id: `start-${q}`, group: actions, label: tf("Start queue “%s”", q), english: `Start queue ${q}`, icon: ListOrdered, run: () => void api.startQueue(q) },
    ),
    {
      id: "limiter",
      group: actions,
      label: limit > 0 ? t("Turn the speed limiter off") : t("Turn the speed limiter on"),
      english: limit > 0 ? "Turn the speed limiter off limit" : "Turn the speed limiter on limit",
      icon: Gauge,
      run: toggleLimiter,
    },
    { id: "sidebar", group: actions, label: sidebar ? t("Hide sidebar") : t("Show sidebar"), english: sidebar ? "Hide sidebar" : "Show sidebar", icon: PanelLeft, run: () => setSidebar(!sidebar) },
    { id: "go-all", group: show, label: t("All downloads"), english: "All downloads", icon: Inbox, run: goTo({ kind: "all" }) },
    { id: "go-running", group: show, label: t("Running"), english: "Running", icon: Zap, run: goTo({ kind: "running" }) },
    { id: "go-unfinished", group: show, label: t("Unfinished"), english: "Unfinished", icon: Clock, run: goTo({ kind: "unfinished" }) },
    { id: "go-finished", group: show, label: t("Finished"), english: "Finished", icon: CheckCircle2, run: goTo({ kind: "finished" }) },
    ...(snap?.categories ?? []).map(
      (c): Command => ({ id: `go-cat-${c}`, group: show, label: categoryLabel(c), english: c, icon: categoryIcon(c), run: goTo({ kind: "category", value: c }) }),
    ),
    ...queues.map((q): Command => ({ id: `go-queue-${q}`, group: show, label: tf("Queue “%s”", q), english: `Queue ${q}`, icon: ListOrdered, run: goTo({ kind: "queue", value: q }) })),
  ];

  return (
    <div className="aurora flex h-full flex-col bg-background font-sans text-foreground">
      <TitleBar
        query={query}
        onQuery={setQuery}
        searchRef={searchRef}
        commands={commands}
        onDownload={setAdding}
        onToList={() => {
          if (order.length > 0 && !order.includes(lead ?? "")) focusRow(0, false);
          table.current?.focus();
        }}
      />
      <div className="flex min-h-0 flex-1">
        {sidebar && (
          <Sidebar
            items={items}
            categories={snap?.categories ?? []}
            queues={queues}
            started={startedQueues}
            filter={filter}
            onFilter={(f) => goTo(f)()}
            speed={totalSpeed}
            speeds={snap?.speeds ?? []}
          />
        )}
        <main className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-3 px-6 pb-3 pt-5">
            <div className="min-w-0 flex-1">
              <h1 className="truncate text-lg font-semibold tracking-tight">{filterTitle(filter)}</h1>
              <p className="truncate text-sm text-muted-foreground">
                {tf("%d running · %d unfinished · %s downloaded", running.length, unfinished, humanBytes(items.reduce((n, d) => n + d.done, 0)) || "0 B")}
              </p>
            </div>
            <Button disabled={resumable.length === 0} onClick={rowActions.resume}>
              <Play /> {t("Resume")}
            </Button>
            <Button disabled={pausable.length === 0} onClick={rowActions.pause}>
              <Pause /> {t("Pause")}
            </Button>
            <Tooltip label={t("Remove from list")}>
              <Button size="icon" disabled={chosen.length === 0} onClick={() => rowActions.remove(false)} aria-label={t("Remove from list")}>
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
            onShowAll={goTo({ kind: "running" })}
            onOpen={(d) => void api.openProgress(d.id)}
          />

          <div className="flex items-center gap-2 px-6 pb-3">
            <div className="inline-flex h-8 items-center rounded-lg bg-muted p-0.5 text-sm">
              {tabs.map(([id, label]) => (
                <button
                  key={id}
                  onClick={() => setTab(id)}
                  className={cn(
                    "rounded-md px-3 py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                    tab === id ? "bg-background font-medium shadow-sm" : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="ml-auto flex items-center gap-1">
              {running.length > 0 && (
                <Button variant="ghost" onClick={() => void api.pauseAll()}>
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
              <MoreMenu
                onResumeAll={() => void api.resumeAll()}
                onPauseAll={() => void api.pauseAll()}
                onStopAll={() => void api.stopAll()}
                onClearCompleted={() => void api.clearCompleted()}
                limiter={{ on: limit > 0, label: limiterLabel, onToggle: toggleLimiter }}
                columns={columns}
                onColumns={setColumns}
                sidebar={sidebar}
                onSidebar={setSidebar}
              />
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
            handle={table}
            items={visible}
            selected={selected}
            sort={sort}
            onSort={(id) => setSort(nextSort(sort, id))}
            columns={columns}
            onColumns={setColumns}
            onPress={press}
            onToggle={(id) => press(id, { ctrl: true, shift: false })}
            onToggleAll={() => setSelected(visible.every((d) => selected.has(d.id)) ? new Set() : new Set(order))}
            onContext={(id) => {
              if (!selected.has(id)) setSelected(new Set([id]));
              setAnchor(id);
              setLead(id);
            }}
            onOpen={open}
            onKeyDown={onKeyDown}
            rowMenu={<RowMenu chosen={chosen} current={current} queues={queues} can={{ open: current?.status === "complete", resume: resumable.length > 0, pause: pausable.length > 0 }} on={rowActions} />}
            empty={
              <div className="flex flex-col items-center gap-3 text-center">
                <span className="flex size-12 items-center justify-center rounded-xl border border-border bg-muted">
                  <Inbox className="size-5 text-muted-foreground" />
                </span>
                <div>
                  <p className="text-sm font-medium">{search ? t("Nothing matches the search") : t("No downloads here")}</p>
                  {!search && <p className="text-xs text-muted-foreground">{t("Downloads you add or send from the browser appear here.")}</p>}
                </div>
                {!search && (
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
        <span className="truncate">{left}</span>
        {!inODM && <span className="rounded bg-muted px-1.5">{t("Sample data")}</span>}
        <span className="ml-auto flex shrink-0 items-center gap-1.5">
          <span className={cn("size-1.5 rounded-full", running.length > 0 ? "accent-gradient" : "bg-zinc-400")} /> {tf("%d active", running.length)}
        </span>
        <span className="shrink-0 tabular-nums">↓ {speedText(totalSpeed) || "0 B/s"}</span>
        <SpeedLimiter limit={limit} saved={savedLimit} onSet={(on, bps) => void api.setSpeedLimiter(on, bps)} open={limiterOpen} onOpenChange={setLimiterOpen}>
          <button
            className={cn(
              "-mx-1.5 flex h-5 shrink-0 items-center gap-1 rounded px-1.5 outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50",
              limit > 0 && "text-foreground",
            )}
          >
            <Gauge className="size-3.5" />
            {limit > 0 ? tf("Speed limit %s", speedText(limit)) : t("No speed limit")}
          </button>
        </SpeedLimiter>
      </footer>

      <Dialog open={adding !== null} onOpenChange={(o) => !o && setAdding(null)}>
        <DialogContent className="max-w-2xl" onOpenAutoFocus={(e) => e.preventDefault()}>
          <div className="space-y-1 pr-6">
            <DialogTitle>{t("New Download")}</DialogTitle>
            <DialogDescription>{t("Paste the address of a file, a stream or a video page.")}</DialogDescription>
          </div>
          {adding !== null && <AddForm token="" initialURL={adding} onDone={() => setAdding(null)} />}
        </DialogContent>
      </Dialog>
      <Dialog open={readdress !== null} onOpenChange={(o) => !o && setReaddress(null)}>
        {readdress && (
          <DialogContent className="max-w-xl">
            <div className="space-y-1 pr-6">
              <DialogTitle>{t("Refresh download address")}</DialogTitle>
              <DialogDescription className="truncate">{displayName(readdress.item)}</DialogDescription>
            </div>
            <form
              className="space-y-3"
              onSubmit={(e) => {
                e.preventDefault();
                const url = readdress.url.trim();
                if (!url || url === readdress.item.url) return setReaddress(null);
                setReaddress({ ...readdress, busy: true, error: "" });
                api.readdress(readdress.item.id, url).then(
                  () => setReaddress(null),
                  (err) => setReaddress((r) => r && { ...r, busy: false, error: String((err as Error)?.message ?? err) }),
                );
              }}
            >
              <Input autoFocus value={readdress.url} onChange={(e) => setReaddress({ ...readdress, url: e.target.value })} spellCheck={false} />
              <p className="text-xs text-muted-foreground">{t("Paste the new address of the same file. Progress is kept if the server supports resuming.")}</p>
              {readdress.error && <p className="text-xs text-red-600 dark:text-red-400">{readdress.error}</p>}
              <div className="flex justify-end gap-2">
                <Button type="button" onClick={() => setReaddress(null)}>
                  {t("Cancel")}
                </Button>
                <Button type="submit" variant="brand" disabled={readdress.busy}>
                  {t("Change")}
                </Button>
              </div>
            </form>
          </DialogContent>
        )}
      </Dialog>
      <ConfirmDialog confirm={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}
