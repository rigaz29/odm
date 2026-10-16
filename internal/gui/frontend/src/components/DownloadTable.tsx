// The download list: TanStack Table for the columns and the header's
// sorting, TanStack Virtual so only the rows in view exist (a thousand
// downloads scroll as ten). The page owns the order and the selection: a
// press selects as Explorer does (Ctrl toggles, Shift picks a range), the
// checkbox toggles one row, a right click on a row outside the selection
// selects it before the menu opens.

import { type ColumnDef, type SortingState, flexRender, getCoreRowModel, useReactTable } from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ArrowDown, ArrowUp, FolderOpen, MoreHorizontal, Pause, Play, SquareArrowOutUpRight, Trash2 } from "lucide-react";
import { useMemo, useRef } from "react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { added, displayName, etaText, host, humanBytes, percent, speedText, statusKind } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { FileTile, ProgressBar, StatusBadge } from "./bits";
import { Checkbox } from "./ui/checkbox";
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuShortcut, ContextMenuTrigger } from "./ui/context-menu";

export const ROW_HEIGHT = 44;
const GRID = "40px minmax(200px,1fr) 84px 152px 96px 100px 112px 120px 40px";

export type Action = "open" | "folder" | "resume" | "pause" | "remove";

export function DownloadTable(props: {
  items: Item[]; // filtered and sorted
  selected: Set<string>;
  sorting: SortingState;
  onSorting: (s: SortingState) => void;
  onPress: (id: string, mods: { ctrl: boolean; shift: boolean }) => void;
  onToggle: (id: string) => void;
  onToggleAll: () => void;
  onContext: (id: string) => void;
  onOpen: (d: Item) => void;
  onAction: (a: Action) => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  can: { open: boolean; resume: boolean; pause: boolean };
  empty: React.ReactNode;
}) {
  const { items, selected } = props;
  const scroll = useRef<HTMLDivElement>(null);

  const allState = selected.size === 0 ? false : items.every((d) => selected.has(d.id)) ? true : "indeterminate";
  const columns = useMemo<ColumnDef<Item>[]>(
    () => [
      { id: "select", enableSorting: false, header: () => null, cell: () => null },
      { id: "name", header: () => t("Name") },
      { id: "size", header: () => t("Size") },
      { id: "progress", header: () => t("Progress") },
      { id: "speed", header: () => t("Speed") },
      { id: "eta", header: () => t("Time left") },
      { id: "status", header: () => t("Status") },
      { id: "added", header: () => t("Added") },
      { id: "more", enableSorting: false, header: () => null },
    ],
    [],
  );
  const table = useReactTable({
    data: items,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (d) => d.id,
    manualSorting: true,
    state: { sorting: props.sorting },
    onSortingChange: (u) => props.onSorting(typeof u === "function" ? u(props.sorting) : u),
  });
  const rows = table.getRowModel().rows;
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scroll.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  const openMenuAt = (el: HTMLElement) => {
    const r = el.getBoundingClientRect();
    el.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: r.left, clientY: r.bottom, button: 2 }));
  };

  return (
    <div className="mx-6 mb-3 flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-border bg-card">
      <div
        ref={scroll}
        tabIndex={0}
        onKeyDown={props.onKeyDown}
        data-odm-list
        className="relative min-h-0 flex-1 overflow-auto outline-none"
      >
        {/* Header */}
        <div
          className="sticky top-0 z-10 grid h-9 items-center border-b border-border bg-muted text-xs text-muted-foreground"
          style={{ gridTemplateColumns: GRID }}
        >
          {table.getHeaderGroups()[0].headers.map((h) => {
            if (h.id === "select") {
              return (
                <div key={h.id} className="pl-3">
                  <Checkbox checked={allState} onCheckedChange={props.onToggleAll} aria-label={t("Select all")} disabled={items.length === 0} />
                </div>
              );
            }
            const sorted = h.column.getIsSorted();
            return (
              <button
                key={h.id}
                disabled={!h.column.getCanSort()}
                onClick={h.column.getToggleSortingHandler()}
                className="flex h-full items-center gap-1 text-left font-medium hover:text-foreground disabled:hover:text-muted-foreground"
              >
                {flexRender(h.column.columnDef.header, h.getContext())}
                {sorted === "asc" && <ArrowUp className="size-3" />}
                {sorted === "desc" && <ArrowDown className="size-3" />}
              </button>
            );
          })}
        </div>

        {rows.length === 0 ? (
          <div className="flex h-[calc(100%-2.25rem)] items-center justify-center">{props.empty}</div>
        ) : (
          <ContextMenu>
            <ContextMenuTrigger asChild>
              <div style={{ height: virtual.getTotalSize(), position: "relative" }}>
                {virtual.getVirtualItems().map((v) => {
                  const d = rows[v.index].original;
                  const sel = selected.has(d.id);
                  const k = statusKind(d);
                  return (
                    <div
                      key={d.id}
                      data-index={v.index}
                      aria-selected={sel}
                      onMouseDown={(e) => {
                        if (e.button !== 0) return;
                        if ((e.target as HTMLElement).closest("[data-no-select]")) return;
                        scroll.current?.focus();
                        props.onPress(d.id, { ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey });
                      }}
                      onDoubleClick={() => props.onOpen(d)}
                      onContextMenu={() => {
                        scroll.current?.focus();
                        props.onContext(d.id);
                      }}
                      className={cn(
                        "absolute left-0 grid w-full items-center border-b border-border text-sm",
                        sel ? "bg-violet-500/[0.06] dark:bg-violet-500/[0.09]" : "hover:bg-muted/40",
                      )}
                      style={{ gridTemplateColumns: GRID, height: v.size, transform: `translateY(${v.start}px)` }}
                    >
                      <div className="relative flex h-full items-center pl-3" data-no-select>
                        {sel && <span className="accent-gradient absolute left-0 top-2 bottom-2 w-[3px] rounded-r-full" />}
                        <Checkbox checked={sel} onCheckedChange={() => props.onToggle(d.id)} aria-label={displayName(d)} />
                      </div>
                      <div className="flex min-w-0 items-center gap-2.5 pr-4">
                        <FileTile category={d.category} />
                        <div className="min-w-0">
                          <p className="truncate font-medium" title={displayName(d)}>
                            {displayName(d)}
                          </p>
                          <p className={cn("truncate text-xs", k === "error" ? "text-red-600 dark:text-red-400" : "text-muted-foreground")} title={d.error || undefined}>
                            {k === "error" && d.error ? d.error : host(d)}
                          </p>
                        </div>
                      </div>
                      <div className="tabular-nums text-muted-foreground">{humanBytes(d.total) || "—"}</div>
                      <div className="pr-4">
                        {k === "complete" ? (
                          <span className="text-xs text-muted-foreground">100%</span>
                        ) : k === "recording" || d.total <= 0 ? (
                          <span className="text-xs text-muted-foreground">{tf("%s so far", humanBytes(d.done) || "0 B")}</span>
                        ) : (
                          <div className="flex items-center gap-2.5">
                            <ProgressBar d={d} className="w-24" />
                            <span className="w-9 text-xs tabular-nums text-muted-foreground">{percent(d)}%</span>
                          </div>
                        )}
                      </div>
                      <div className="tabular-nums">{speedText(d.speed) || <span className="text-muted-foreground">—</span>}</div>
                      <div className="tabular-nums text-muted-foreground">{etaText(d) || "—"}</div>
                      <div>
                        <StatusBadge d={d} />
                      </div>
                      <div className="truncate pr-2 text-muted-foreground">{added(d.createdAt)}</div>
                      <div data-no-select>
                        <button
                          aria-label={t("More")}
                          onClick={(e) => {
                            props.onContext(d.id);
                            openMenuAt(e.currentTarget);
                          }}
                          className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                        >
                          <MoreHorizontal className="size-4" />
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem disabled={!props.can.open} onSelect={() => props.onAction("open")}>
                <SquareArrowOutUpRight /> {t("Open file")}
                <ContextMenuShortcut>Enter</ContextMenuShortcut>
              </ContextMenuItem>
              <ContextMenuItem onSelect={() => props.onAction("folder")}>
                <FolderOpen /> {t("Show in folder")}
              </ContextMenuItem>
              <ContextMenuSeparator />
              <ContextMenuItem disabled={!props.can.resume} onSelect={() => props.onAction("resume")}>
                <Play /> {t("Resume")}
              </ContextMenuItem>
              <ContextMenuItem disabled={!props.can.pause} onSelect={() => props.onAction("pause")}>
                <Pause /> {t("Pause")}
                <ContextMenuShortcut>Space</ContextMenuShortcut>
              </ContextMenuItem>
              <ContextMenuSeparator />
              <ContextMenuItem onSelect={() => props.onAction("remove")} className="text-red-600 dark:text-red-400 [&_svg]:text-current">
                <Trash2 /> {t("Remove from list")}
                <ContextMenuShortcut>Del</ContextMenuShortcut>
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        )}
      </div>
    </div>
  );
}
