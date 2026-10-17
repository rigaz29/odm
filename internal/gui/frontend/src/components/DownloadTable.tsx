// The download list: TanStack Virtual so only the rows in view exist (a
// thousand downloads scroll as ten). Which columns show, how wide they are
// and the order are the page's (columns.ts): a header sorts by its column,
// its right edge drags its width (a double click restores it), and
// the header's context menu, or the button over the row menus, shows and
// hides columns. When the columns are wider than the list it scrolls
// sideways, with the checkbox, Name and the row menu kept in view. The page
// owns the order and the selection: a press selects
// as Explorer does (Ctrl toggles, Shift picks a range), the checkbox toggles
// one row, a right click on a row outside the selection selects it before
// the menu opens. To Narrator it is a grid: the header's cells and each
// row's, the row count of the whole list though only those in view exist,
// and the row the keyboard is on. The list is one stop for Tab: the arrows
// move the selection through the rows (Shift extends it, Ctrl+A takes
// them all), and the Menu key or Shift+F10 opens the row menu, so the
// rows' own checkboxes and menu buttons are left out of the Tab order.

import { useVirtualizer } from "@tanstack/react-virtual";
import { ArrowDown, ArrowUp, Columns3, MoreHorizontal } from "lucide-react";
import { type ReactNode, type Ref, useEffect, useImperativeHandle, useRef, useState } from "react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { COLUMNS, type ColumnId, type ColumnState, type Sort, gridTemplate, minWidth, resetWidth, resized, widthOf } from "../columns";
import { added, displayName, etaText, host, humanBytes, percent, speedText, statusKind } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { FileTile, ProgressBar, StatusBadge } from "./bits";
import { Checkbox } from "./ui/checkbox";
import { ColumnItems } from "./ColumnItems";
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "./ui/context-menu";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "./ui/dropdown-menu";

export const ROW_HEIGHT = 44;
const HEADER_HEIGHT = 36;

// The cells kept in view while the list scrolls sideways; a line marks the
// edge with columns scrolled under it.
const stickLeft = "sticky left-0 z-[1] self-stretch bg-inherit";
const stickName = "sticky left-10 z-[1] self-stretch bg-inherit";
const stickRight = "sticky right-0 z-[1] self-stretch bg-inherit";
const edgeLeft = "shadow-[1px_0_0_var(--color-border)]";
const edgeRight = "shadow-[-1px_0_0_var(--color-border)]";

// Back to the list after a menu, without scrolling it.
const backToList = (el: HTMLElement | null) => (e: Event) => {
  e.preventDefault();
  el?.focus({ preventScroll: true });
};

const label = (id: ColumnId) => t(COLUMNS.find((c) => c.id === id)!.label);

// The element id of a download's row, for aria-activedescendant.
const rowID = (id: string) => `row-${id}`;

// TableHandle lets the page move the list: scroll a row into view, focus
// it, and learn how many rows a page (Page Up, Page Down) holds.
export interface TableHandle {
  scrollTo(index: number): void;
  focus(): void;
  pageSize(): number;
}

export function DownloadTable(props: {
  items: Item[]; // filtered and sorted
  selected: Set<string>;
  sort: Sort;
  onSort: (id: ColumnId) => void; // a click on a header
  columns: ColumnState;
  onColumns: (s: ColumnState) => void;
  onPress: (id: string, mods: { ctrl: boolean; shift: boolean }) => void;
  onToggle: (id: string) => void;
  onToggleAll: () => void;
  onContext: (id: string) => void;
  onOpen: (d: Item) => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  lead?: string | null; // the row the keyboard is on
  rowMenu: ReactNode; // the row context menu's items
  empty: ReactNode;
  handle?: Ref<TableHandle>;
}) {
  const { items, selected } = props;
  const scroll = useRef<HTMLDivElement>(null);
  // The width of the column being dragged, kept here until the drag ends.
  const [drag, setDrag] = useState<{ id: ColumnId; w: number } | null>(null);
  const cols = drag ? resized(props.columns, drag.id, drag.w) : props.columns;
  const grid = gridTemplate(cols);
  const shown = cols.shown;
  // Whether columns are scrolled out of view to the left or the right.
  const [edges, setEdges] = useState({ left: false, right: false });
  const measure = () => {
    const el = scroll.current;
    if (!el) return;
    const left = el.scrollLeft > 0;
    const right = el.scrollLeft + el.clientWidth < el.scrollWidth - 1;
    setEdges((e) => (e.left === left && e.right === right ? e : { left, right }));
  };
  useEffect(() => {
    measure();
    const ro = new ResizeObserver(measure);
    if (scroll.current) ro.observe(scroll.current);
    return () => ro.disconnect();
  }, [grid]);

  const allState = selected.size === 0 ? false : items.every((d) => selected.has(d.id)) ? true : "indeterminate";
  const virtual = useVirtualizer({
    count: items.length,
    getScrollElement: () => scroll.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
    // The rows start under the header, which stays over them as they scroll.
    scrollMargin: HEADER_HEIGHT,
    scrollPaddingStart: HEADER_HEIGHT,
  });

  useImperativeHandle(
    props.handle,
    () => ({
      scrollTo: (index) => virtual.scrollToIndex(index, { align: "auto" }),
      focus: () => scroll.current?.focus({ preventScroll: true }),
      pageSize: () => Math.max(1, Math.floor(((scroll.current?.clientHeight ?? 0) - HEADER_HEIGHT) / ROW_HEIGHT) - 1),
    }),
    [virtual],
  );

  const openMenuAt = (el: HTMLElement) => {
    const r = el.getBoundingClientRect();
    const x = el.getAttribute("role") === "row" ? r.left + 48 : r.left; // a row's: under its name
    el.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: x, clientY: r.bottom, button: 2 }));
  };

  const startDrag = (id: ColumnId, e: React.PointerEvent<HTMLElement>) => {
    e.preventDefault();
    e.stopPropagation();
    const el = e.currentTarget;
    const x0 = e.clientX;
    const w0 = widthOf(props.columns, id);
    el.setPointerCapture(e.pointerId);
    let w = w0;
    const move = (ev: PointerEvent) => {
      w = w0 + ev.clientX - x0;
      setDrag({ id, w });
    };
    const up = () => {
      el.removeEventListener("pointermove", move);
      el.removeEventListener("pointerup", up);
      el.removeEventListener("pointercancel", up);
      setDrag(null);
      if (w !== w0) props.onColumns(resized(props.columns, id, w));
    };
    el.addEventListener("pointermove", move);
    el.addEventListener("pointerup", up);
    el.addEventListener("pointercancel", up);
  };

  const rows = virtual.getVirtualItems();
  // The keyboard's row, while it exists (scrolled away it does not).
  const lead = props.lead && rows.some((v) => items[v.index]?.id === props.lead) ? rowID(props.lead) : undefined;

  return (
    <div className="mx-6 mb-3 flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-border bg-card has-[[data-odm-list]:focus-visible]:ring-2 has-[[data-odm-list]:focus-visible]:ring-ring/50">
      <div
        ref={scroll}
        tabIndex={0}
        onKeyDown={(e) => {
          const row = props.lead ? document.getElementById(rowID(props.lead)) : null;
          if (row && (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10"))) {
            e.preventDefault();
            props.onContext(props.lead!);
            openMenuAt(row);
            return;
          }
          props.onKeyDown(e);
        }}
        onScroll={measure}
        data-odm-list
        role="grid"
        aria-label={t("Download list")}
        aria-multiselectable
        aria-rowcount={items.length + 1}
        aria-colcount={shown.length + 2}
        aria-activedescendant={lead}
        className="group/list relative min-h-0 flex-1 overflow-auto outline-none"
      >
        <div style={{ minWidth: minWidth(cols) }} className="flex min-h-full flex-col">
          {/* Header */}
          <ContextMenu>
            <ContextMenuTrigger asChild>
              <div
                role="row"
                aria-rowindex={1}
                className="sticky top-0 z-10 grid shrink-0 items-center border-b border-border bg-muted text-xs text-muted-foreground"
                style={{ gridTemplateColumns: grid, height: HEADER_HEIGHT }}
              >
                <div role="columnheader" className={cn(stickLeft, "flex items-center pl-3")}>
                  <Checkbox checked={allState} onCheckedChange={props.onToggleAll} aria-label={t("Select all")} disabled={items.length === 0} />
                </div>
                {shown.map((id) => {
                  const sorted = props.sort?.id === id ? (props.sort.desc ? "desc" : "asc") : false;
                  return (
                    <div
                      key={id}
                      role="columnheader"
                      aria-sort={sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : undefined}
                      className={cn("relative flex h-full min-w-0 items-center pr-3", id === "name" && cn(stickName, edges.left && edgeLeft))}
                    >
                      <button
                        onClick={() => props.onSort(id)}
                        className="flex h-full min-w-0 items-center gap-1 rounded-sm text-left font-medium outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                      >
                        <span className="truncate">{label(id)}</span>
                        {sorted === "asc" && <ArrowUp className="size-3 shrink-0" />}
                        {sorted === "desc" && <ArrowDown className="size-3 shrink-0" />}
                      </button>
                      {id !== "name" && (
                        <span
                          aria-hidden // the mouse's: the Columns menu restores widths
                          title={t("Drag to resize, double-click to restore")}
                          onPointerDown={(e) => startDrag(id, e)}
                          onDoubleClick={() => props.onColumns(resetWidth(props.columns, id))}
                          className="group absolute -right-1 top-0 z-10 flex h-full w-2 cursor-col-resize justify-center"
                        >
                          <span className={cn("my-2 w-px bg-border group-hover:bg-ring", drag?.id === id && "bg-ring")} />
                        </span>
                      )}
                    </div>
                  );
                })}
                <div role="columnheader" className={cn(stickRight, "flex items-center justify-center", edges.right && edgeRight)}>
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      aria-label={t("Columns")}
                      title={t("Columns")}
                      className="flex size-7 items-center justify-center rounded-md outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                    >
                      <Columns3 className="size-4" />
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" onCloseAutoFocus={backToList(scroll.current)}>
                      <ColumnItems menu="dropdown" columns={props.columns} onColumns={props.onColumns} />
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            </ContextMenuTrigger>
            <ContextMenuContent onCloseAutoFocus={backToList(scroll.current)}>
              <ColumnItems menu="context" columns={props.columns} onColumns={props.onColumns} />
            </ContextMenuContent>
          </ContextMenu>

          {items.length === 0 ? (
            <div role="row" className="flex flex-1">
              <div role="gridcell" className="flex flex-1 items-center justify-center py-10">
                {props.empty}
              </div>
            </div>
          ) : (
            <ContextMenu>
              <ContextMenuTrigger asChild>
                <div style={{ height: virtual.getTotalSize(), position: "relative" }}>
                  {rows.map((v) => {
                    const d = items[v.index];
                    const sel = selected.has(d.id);
                    return (
                      <div
                        key={d.id}
                        id={rowID(d.id)}
                        data-index={v.index}
                        role="row"
                        aria-rowindex={v.index + 2}
                        aria-selected={sel}
                        onMouseDown={(e) => {
                          if (e.button !== 0) return;
                          if ((e.target as HTMLElement).closest("[data-no-select]")) return;
                          scroll.current?.focus({ preventScroll: true });
                          props.onPress(d.id, { ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey });
                        }}
                        onDoubleClick={() => props.onOpen(d)}
                        onContextMenu={() => {
                          scroll.current?.focus({ preventScroll: true });
                          props.onContext(d.id);
                        }}
                        className={cn(
                          "absolute left-0 grid w-full items-center border-b border-border text-sm",
                          // Opaque, so the cells kept in view can take the row's colour.
                          sel
                            ? "bg-[color-mix(in_oklab,var(--color-card)_94%,#8b5cf6)] dark:bg-[color-mix(in_oklab,var(--color-card)_91%,#8b5cf6)]"
                            : "bg-card hover:bg-[color-mix(in_oklab,var(--color-card)_60%,var(--color-muted))]",
                          // The keyboard's row, while the list has the keyboard.
                          d.id === props.lead && "group-focus-visible/list:shadow-[inset_0_0_0_2px_var(--color-ring)]",
                        )}
                        style={{ gridTemplateColumns: grid, height: v.size, transform: `translateY(${v.start - HEADER_HEIGHT}px)` }}
                      >
                        <div role="gridcell" className={cn(stickLeft, "relative flex items-center pl-3")} data-no-select>
                          {sel && <span className="accent-gradient absolute left-0 top-2 bottom-2 w-[3px] rounded-r-full" />}
                          {/* The row says whether it is selected; the box is the mouse's. */}
                          <Checkbox checked={sel} tabIndex={-1} aria-hidden onCheckedChange={() => props.onToggle(d.id)} />
                        </div>
                        {shown.map((id) => (
                          <Cell key={id} id={id} d={d} className={id === "name" ? cn(stickName, edges.left && edgeLeft) : undefined} />
                        ))}
                        <div role="gridcell" data-no-select className={cn(stickRight, "flex items-center justify-center", edges.right && edgeRight)}>
                          <button
                            aria-label={t("More")}
                            tabIndex={-1}
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
              <ContextMenuContent onCloseAutoFocus={backToList(scroll.current)}>{props.rowMenu}</ContextMenuContent>
            </ContextMenu>
          )}
        </div>
      </div>
    </div>
  );
}

const muted = "truncate pr-3 text-muted-foreground";

function Cell({ id, d, className }: { id: ColumnId; d: Item; className?: string }) {
  const k = statusKind(d);
  switch (id) {
    case "name":
      return (
        <div role="gridcell" className={cn("flex min-w-0 items-center gap-2.5 pr-4", className)}>
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
      );
    case "queue":
      return <div role="gridcell" className={muted}>{d.queue}</div>;
    case "size":
      return <div role="gridcell" className={cn(muted, "tabular-nums")}>{humanBytes(d.total) || "—"}</div>;
    case "progress":
      return (
        <div role="gridcell" className="min-w-0 pr-4">
          {k === "complete" ? (
            <span className="text-xs text-muted-foreground">100%</span>
          ) : k === "recording" || d.total <= 0 ? (
            <span className="block truncate text-xs text-muted-foreground">{tf("%s so far", humanBytes(d.done) || "0 B")}</span>
          ) : (
            <div className="flex items-center gap-2.5">
              <ProgressBar d={d} className="min-w-8 flex-1" />
              <span className="w-9 shrink-0 text-xs tabular-nums text-muted-foreground">{percent(d)}%</span>
            </div>
          )}
        </div>
      );
    case "speed":
      return <div role="gridcell" className="truncate pr-3 tabular-nums">{speedText(d.speed) || <span className="text-muted-foreground">—</span>}</div>;
    case "eta":
      return <div role="gridcell" className={cn(muted, "tabular-nums")}>{etaText(d) || "—"}</div>;
    case "status":
      return (
        <div role="gridcell" className="min-w-0 overflow-hidden pr-2">
          <StatusBadge d={d} />
        </div>
      );
    case "added":
      return <div role="gridcell" className={muted}>{added(d.createdAt)}</div>;
    case "lastTry":
      return <div role="gridcell" className={muted}>{added(d.updatedAt)}</div>;
    case "description":
      return (
        <div role="gridcell" className={muted} title={d.description || undefined}>
          {d.description}
        </div>
      );
  }
}
