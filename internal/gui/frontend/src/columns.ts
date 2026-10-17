// The list's columns: which show, how wide each is, and the order the list
// is sorted in. The window keeps the choices (localStorage, in WebView2's
// profile next to ODM's state), as the Fyne UI keeps them in its
// preferences. Name always shows and takes the width the others leave.

import { markT } from "./i18n";

export type ColumnId = "name" | "queue" | "size" | "progress" | "speed" | "eta" | "status" | "added" | "lastTry" | "description";

export interface Column {
  id: ColumnId;
  label: string; // English, shown through t
  width: number; // the default, in px; Name's is its minimum
  min: number;
  hidden?: boolean; // hidden until shown from the column menu
}

export const COLUMNS: Column[] = [
  { id: "name", label: markT("Name"), width: 220, min: 160 },
  { id: "queue", label: markT("Queue"), width: 96, min: 56, hidden: true },
  { id: "size", label: markT("Size"), width: 84, min: 60 },
  { id: "progress", label: markT("Progress"), width: 152, min: 96 },
  { id: "speed", label: markT("Speed"), width: 96, min: 64 },
  { id: "eta", label: markT("Time left"), width: 100, min: 64 },
  { id: "status", label: markT("Status"), width: 112, min: 88 },
  { id: "added", label: markT("Added"), width: 120, min: 72 },
  { id: "lastTry", label: markT("Last attempt"), width: 120, min: 72, hidden: true },
  { id: "description", label: markT("Description"), width: 180, min: 80, hidden: true },
];

// The fixed columns at either end: the checkbox and the row's menu.
export const EDGE = 40;

export interface ColumnState {
  shown: ColumnId[]; // in COLUMNS order
  widths: Partial<Record<ColumnId, number>>; // only the ones dragged
}

export const defaultColumns = (): ColumnState => ({ shown: COLUMNS.filter((c) => !c.hidden).map((c) => c.id), widths: {} });

const byId = new Map(COLUMNS.map((c) => [c.id, c]));

// readColumns checks a saved state: unknown ids and widths below a
// column's minimum are dropped, and Name always shows.
export function readColumns(saved: unknown): ColumnState {
  const out = defaultColumns();
  if (!saved || typeof saved !== "object") return out;
  const s = saved as { shown?: unknown; widths?: unknown };
  if (Array.isArray(s.shown)) {
    const want = new Set(s.shown.filter((id): id is ColumnId => typeof id === "string" && byId.has(id as ColumnId)));
    want.add("name");
    out.shown = COLUMNS.filter((c) => want.has(c.id)).map((c) => c.id);
  }
  if (s.widths && typeof s.widths === "object") {
    for (const [id, w] of Object.entries(s.widths as Record<string, unknown>)) {
      const c = byId.get(id as ColumnId);
      if (c && c.id !== "name" && typeof w === "number" && w >= c.min && w <= 1000) out.widths[c.id] = Math.round(w);
    }
  }
  return out;
}

export function toggleColumn(s: ColumnState, id: ColumnId): ColumnState {
  if (id === "name") return s;
  const want = new Set(s.shown);
  if (want.has(id)) want.delete(id);
  else want.add(id);
  return { ...s, shown: COLUMNS.filter((c) => want.has(c.id)).map((c) => c.id) };
}

export const widthOf = (s: ColumnState, id: ColumnId) => s.widths[id] ?? byId.get(id)!.width;

// resized sets a column's width, kept at its minimum or above.
export function resized(s: ColumnState, id: ColumnId, w: number): ColumnState {
  const c = byId.get(id);
  if (!c || id === "name") return s;
  return { ...s, widths: { ...s.widths, [id]: Math.max(c.min, Math.round(w)) } };
}

export function resetWidth(s: ColumnState, id: ColumnId): ColumnState {
  const widths = { ...s.widths };
  delete widths[id];
  return { ...s, widths };
}

// gridTemplate is the CSS grid of a row: the checkbox, the shown columns
// with Name taking what is left, and the menu.
export function gridTemplate(s: ColumnState): string {
  const cols = s.shown.map((id) => (id === "name" ? `minmax(${byId.get("name")!.min}px,1fr)` : `${widthOf(s, id)}px`));
  return [`${EDGE}px`, ...cols, `${EDGE}px`].join(" ");
}

// minWidth is the narrowest the list can be before it scrolls sideways.
export const minWidth = (s: ColumnState) => 2 * EDGE + s.shown.reduce((n, id) => n + (id === "name" ? byId.get("name")!.min : widthOf(s, id)), 0);

// The list's order: by one column, either way, or as added (null; a
// queue's run order when the sidebar shows a queue).
export type Sort = { id: ColumnId; desc: boolean } | null;

// nextSort is a click on a column's header: by it, then reversed, then as
// added again.
export function nextSort(s: Sort, id: ColumnId): Sort {
  if (s?.id !== id) return { id, desc: false };
  return s.desc ? null : { id, desc: true };
}

// readSort checks a saved order.
export function readSort(saved: unknown): Sort {
  const s = saved as { id?: unknown; desc?: unknown } | null;
  return s && typeof s.id === "string" && byId.has(s.id as ColumnId) ? { id: s.id as ColumnId, desc: s.desc === true } : null;
}
