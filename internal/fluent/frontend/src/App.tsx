// The main window: command bar, navigation pane, download list and status
// bar, on the window's Mica.

import { Badge, FluentProvider, makeStyles, tokens } from "@fluentui/react-components";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, inODM } from "./api";
import type { Appearance, Item, Snapshot } from "./bindings/odm/internal/fluent/models";
import { AddUrlDialog } from "./components/AddUrlDialog";
import { CategoryPane } from "./components/CategoryPane";
import { CommandBar, type Command } from "./components/CommandBar";
import { DownloadList, type SortState, compareBy } from "./components/DownloadList";
import { type Filter, humanBytes, matchesFilter, matchesSearch, select } from "./format";
import { setLanguage, t, tf } from "./i18n";
import { makeTheme, surfaceVars } from "./theme";

const useStyles = makeStyles({
  provider: { height: "100%", backgroundColor: "transparent" },
  root: {
    display: "grid",
    gridTemplateRows: "auto minmax(0, 1fr) auto",
    gridTemplateColumns: "232px minmax(0, 1fr)",
    height: "100%",
  },
  bar: { gridColumn: "1 / span 2" },
  nav: {
    gridRow: 2,
    gridColumn: 1,
    minHeight: 0,
    overflow: "hidden",
  },
  content: {
    gridRow: 2,
    gridColumn: 2,
    minWidth: 0,
    minHeight: 0,
    overflow: "hidden",
    backgroundColor: "var(--odm-layer)",
    borderTop: "1px solid var(--odm-stroke)",
    borderLeft: "1px solid var(--odm-stroke)",
    borderTopLeftRadius: tokens.borderRadiusXLarge,
    padding: `${tokens.spacingVerticalXS} ${tokens.spacingHorizontalXS} 0`,
  },
  status: {
    gridColumn: "1 / span 2",
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: tokens.spacingHorizontalL,
    padding: `${tokens.spacingVerticalXS} ${tokens.spacingHorizontalL}`,
    fontSize: tokens.fontSizeBase200,
    color: tokens.colorNeutralForeground2,
    minHeight: "28px",
  },
  statusLeft: { display: "flex", alignItems: "center", gap: tokens.spacingHorizontalS },
});

export function App() {
  const s = useStyles();
  const [appearance, setAppearance] = useState<Appearance | null>(null);
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [systemDark, setSystemDark] = useState(() => matchMedia("(prefers-color-scheme: dark)").matches);
  const [filter, setFilter] = useState<Filter>({ kind: "all" });
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortState>({ sortColumn: undefined, sortDirection: "ascending" });
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [anchor, setAnchor] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
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
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "f") {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
    };
    window.addEventListener("keydown", on);
    return () => window.removeEventListener("keydown", on);
  }, []);

  const dark = appearance?.theme === "dark" || (appearance?.theme !== "light" && systemDark);
  const theme = useMemo(() => makeTheme(appearance?.accent || "#0067c0", dark), [appearance, dark]);
  useEffect(() => {
    document.body.classList.toggle("no-mica", !inODM);
    document.body.classList.toggle("dark", dark);
  }, [dark]);

  const items = snap?.items ?? [];
  const { counts, unfinished, finished } = useMemo(() => {
    const counts = new Map<string, number>();
    let unfinished = 0;
    for (const d of items) {
      counts.set(d.category, (counts.get(d.category) ?? 0) + 1);
      if (d.status !== "complete") unfinished++;
    }
    return { counts, unfinished, finished: items.length - unfinished };
  }, [items]);

  const visible = useMemo(() => {
    let v = items.filter((d) => matchesFilter(d, filter) && matchesSearch(d, query));
    const col = sort.sortColumn;
    if (col && compareBy[col]) {
      const c = compareBy[col];
      v = [...v].sort((a, b) => (sort.sortDirection === "ascending" ? c(a, b) : c(b, a)));
    }
    return v;
  }, [items, filter, query, sort]);
  const order = useMemo(() => visible.map((d) => d.id), [visible]);

  const chosen = items.filter((d) => selected.has(d.id));
  const current: Item | undefined = items.find((d) => d.id === anchor && selected.has(d.id)) ?? chosen[0];
  const resumable = chosen.filter((d) => d.status === "paused" || d.status === "error" || d.status === "queued");
  const pausable = chosen.filter((d) => d.status === "downloading" || d.status === "queued");
  const queue = filter.kind === "queue" ? filter.value : (snap?.queues[0] ?? "Main");
  const queueRunning = (snap?.running ?? []).includes(queue);

  const run = useCallback(
    (c: Command | "open") => {
      const ids = chosen.map((d) => d.id);
      switch (c) {
        case "add":
          setAdding(true);
          break;
        case "resume":
          void api.resume(resumable.map((d) => d.id));
          break;
        case "pause":
          void api.pause(pausable.map((d) => d.id));
          break;
        case "pauseAll":
          void api.pauseAll();
          break;
        case "remove":
          void api.remove(ids);
          setSelected(new Set());
          break;
        case "clean":
          void api.clearCompleted();
          break;
        case "startQueue":
          void api.startQueue(queue);
          break;
        case "stopQueue":
          void api.stopQueue(queue);
          break;
        case "folder":
          if (current) void api.showInFolder(current.id);
          break;
        case "open":
          if (current?.status === "complete") void api.openFile(current.id);
          break;
      }
    },
    [chosen, resumable, pausable, queue, current],
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
      run("remove");
    } else if (e.key === "Enter") {
      run("open");
    } else if (e.key === " ") {
      e.preventDefault();
      run(pausable.length > 0 ? "pause" : "resume");
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (order.length === 0) return;
      const at = current ? order.indexOf(current.id) : -1;
      const to = Math.min(order.length - 1, Math.max(0, at + (e.key === "ArrowDown" ? 1 : -1)));
      if (e.shiftKey) {
        const next = select(order, selected, anchor ?? order[at] ?? null, order[to], { ctrl: false, shift: true });
        setSelected(next.selected);
      } else {
        setSelected(new Set([order[to]]));
        setAnchor(order[to]);
      }
    }
  };

  if (!appearance) return null;

  const all = items.length === 1 ? t("1 item") : tf("%d items", items.length);
  let left = query ? tf("%d of %s match the search", visible.length, all) : all;
  if (chosen.length > 0) left += tf(" · %d selected", chosen.length);
  const active = items.filter((d) => d.status === "downloading");
  const speed = active.reduce((n, d) => n + d.speed, 0);
  const right = [
    active.length > 0 ? tf("%d active · %s", active.length, `${humanBytes(speed)}/s`) : "",
    snap && snap.limit > 0 ? tf("Speed limit %s", `${humanBytes(snap.limit)}/s`) : "",
  ]
    .filter(Boolean)
    .join("   ");

  return (
    <FluentProvider theme={theme} className={s.provider} style={surfaceVars(dark) as React.CSSProperties}>
      <div className={s.root}>
        <div className={s.bar}>
          <CommandBar
            canResume={resumable.length > 0}
            canPause={pausable.length > 0}
            anyRunning={active.length > 0}
            hasSelection={chosen.length > 0}
            hasFinished={finished > 0}
            queueRunning={queueRunning}
            canShowFolder={!!current}
            query={query}
            searchRef={searchRef}
            onQuery={setQuery}
            onCommand={run}
          />
        </div>
        <nav className={s.nav}>
          <CategoryPane
            filter={filter}
            onFilter={setFilter}
            categories={snap?.categories ?? []}
            queues={snap?.queues ?? []}
            counts={counts}
            total={items.length}
            unfinished={unfinished}
            finished={finished}
          />
        </nav>
        <main className={s.content}>
          <DownloadList
            items={visible}
            selected={selected}
            sort={sort}
            onSort={setSort}
            onPress={press}
            onContext={(id) => {
              if (!selected.has(id)) {
                setSelected(new Set([id]));
                setAnchor(id);
              } else {
                setAnchor(id);
              }
            }}
            onOpen={(d) => d.status === "complete" && void api.openFile(d.id)}
            onAction={(a) => run(a === "open" ? "open" : a)}
            onKeyDown={onKeyDown}
            canOpen={current?.status === "complete"}
            canResume={resumable.length > 0}
            canPause={pausable.length > 0}
            emptyText={t("Nothing here")}
          />
        </main>
        <footer className={s.status}>
          <span className={s.statusLeft}>
            {left}
            <Badge appearance="tint" size="small" color="informative">
              {t("Prototype")}
            </Badge>
          </span>
          <span>{right}</span>
        </footer>
      </div>
      <AddUrlDialog
        open={adding}
        onClose={() => setAdding(false)}
        onAdd={async (url) => {
          await api.addURL(url);
        }}
      />
    </FluentProvider>
  );
}
