// @vitest-environment jsdom
//
// Dragging a column's right edge moves that edge, as in Explorer: the
// column widens or narrows, the columns to its right move with the edge,
// and Name keeps its place. Name took the space the others left, so a
// wider Size only made Name narrower: the edge between Size and Progress
// stayed under the pointer's start and the one between Name and Size
// moved, and Name's own edge could not be dragged at all.

import { act } from "react";
import { type Root, createRoot } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { Item } from "../bindings/odm/internal/gui/models";
import { type ColumnState, defaultColumns, nameFits, resized } from "../columns";
import { DownloadTable } from "./DownloadTable";

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, get: () => 800 });
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, get: () => 1200 });
  // jsdom has no pointer capture.
  Element.prototype.setPointerCapture = () => {};
});

let root: Root | null = null;
afterEach(() => {
  if (root) act(() => root!.unmount());
  root = null;
  document.body.innerHTML = "";
});

// render shows the list with columns; Name's header shows nameWidth px
// (jsdom lays nothing out).
function render(columns: ColumnState, nameWidth = 400) {
  const onColumns = vi.fn<(s: ColumnState) => void>();
  const el = document.createElement("div");
  document.body.append(el);
  root = createRoot(el);
  const items = [new Item({ id: "a", url: "https://h.example/a.zip", fileName: "a.zip", status: "paused", total: 1000, done: 100 })];
  act(() =>
    root!.render(
      <DownloadTable
        items={items} selected={new Set()} sort={null} onSort={() => {}} columns={columns} onColumns={onColumns}
        onPress={() => {}} onToggle={() => {}} onToggleAll={() => {}} onContext={() => {}} onOpen={() => {}}
        onKeyDown={() => {}} rowMenu={null} empty={null}
      />,
    ),
  );
  const name = document.querySelector<HTMLElement>('[data-col="name"]')!;
  name.getBoundingClientRect = () => ({ width: nameWidth }) as DOMRect;
  return onColumns;
}

const edgeOf = (id: string) => document.querySelector<HTMLElement>(`[data-col="${id}"] .cursor-col-resize`)!;
const header = () => document.querySelector<HTMLElement>('[role="row"][aria-rowindex="1"]')!;

function pointer(type: string, el: HTMLElement, clientX: number) {
  const Ctor = (window as unknown as { PointerEvent?: typeof MouseEvent }).PointerEvent ?? MouseEvent;
  act(() => {
    el.dispatchEvent(new Ctor(type, { bubbles: true, cancelable: true, clientX, button: 0 }));
  });
}

function drag(id: string, dx: number) {
  const edge = edgeOf(id);
  pointer("pointerdown", edge, 500);
  pointer("pointermove", edge, 500 + dx);
  return edge;
}

describe("column edges", () => {
  it("between Size and Progress: Size widens, Name keeps its place", () => {
    const onColumns = render(defaultColumns());
    const edge = drag("size", 60);
    // While dragging: Name holds the 400 px it showed, Size is 84 + 60, and
    // the space left goes to the empty column at the end.
    expect(header().style.gridTemplateColumns).toBe("40px 400px 144px 152px 96px 100px 112px 120px minmax(0,1fr) 40px");
    pointer("pointerup", edge, 560);
    expect(onColumns).toHaveBeenCalledTimes(1);
    expect(onColumns.mock.calls[0][0].widths).toEqual({ name: 400, size: 144 });
  });

  it("between Name and Size: Name has an edge of its own", () => {
    const onColumns = render(defaultColumns());
    const edge = drag("name", 80);
    expect(header().style.gridTemplateColumns.startsWith("40px 480px 84px ")).toBe(true);
    pointer("pointerup", edge, 580);
    expect(onColumns.mock.calls[0][0].widths).toEqual({ name: 480 });
  });

  it("every shown column's edge can be dragged", () => {
    render(defaultColumns());
    for (const id of defaultColumns().shown) expect(edgeOf(id), id).not.toBeNull();
  });

  it("a press without a move changes nothing, and Name still takes the space left", () => {
    const onColumns = render(defaultColumns());
    const edge = edgeOf("size");
    pointer("pointerdown", edge, 500);
    pointer("pointerup", edge, 500);
    expect(onColumns).not.toHaveBeenCalled();
    expect(header().style.gridTemplateColumns).toContain("minmax(160px,1fr)");
  });

  it("a double click on Name's edge gives Name the space left again", () => {
    const onColumns = render(resized(resized(defaultColumns(), "name", 480), "size", 120));
    act(() => {
      edgeOf("name").dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    });
    const s = onColumns.mock.calls.at(-1)![0];
    expect(nameFits(s)).toBe(true);
    expect(s.widths).toEqual({ size: 120 });
  });
});
