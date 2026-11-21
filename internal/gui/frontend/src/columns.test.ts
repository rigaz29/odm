import { describe, expect, it } from "vitest";
import { defaultColumns, gridTemplate, minWidth, nameFits, nextSort, readColumns, readSort, resetWidth, resized, toggleColumn, widthOf } from "./columns";

describe("columns", () => {
  it("shows the usual ones until told otherwise", () => {
    const s = defaultColumns();
    expect(s.shown).toEqual(["name", "size", "progress", "speed", "eta", "status", "added"]);
    expect(gridTemplate(s)).toBe("40px minmax(160px,1fr) 84px 152px 96px 100px 112px 120px 40px");
    expect(minWidth(s)).toBe(40 + 160 + 84 + 152 + 96 + 100 + 112 + 120 + 40);
  });

  it("shows and hides columns in their place, never Name", () => {
    let s = toggleColumn(defaultColumns(), "queue");
    expect(s.shown.slice(0, 3)).toEqual(["name", "queue", "size"]);
    s = toggleColumn(s, "size");
    expect(s.shown).not.toContain("size");
    expect(toggleColumn(s, "name").shown).toContain("name");
  });

  it("keeps dragged widths at the column's minimum or above", () => {
    let s = resized(defaultColumns(), "speed", 140.4);
    expect(widthOf(s, "speed")).toBe(140);
    s = resized(s, "status", 10);
    expect(widthOf(s, "status")).toBe(88);
    s = resized(s, "eta", 5000);
    expect(widthOf(s, "eta")).toBe(1000);
    expect(widthOf(resetWidth(s, "speed"), "speed")).toBe(96);
  });

  it("gives the space left to an empty column at the end once Name has a width", () => {
    const s = resized(defaultColumns(), "name", 400);
    expect(nameFits(s)).toBe(false);
    expect(gridTemplate(s)).toBe("40px 400px 84px 152px 96px 100px 112px 120px minmax(0,1fr) 40px");
    expect(minWidth(s)).toBe(40 + 400 + 84 + 152 + 96 + 100 + 112 + 120 + 40);
    expect(widthOf(resized(s, "name", 50), "name")).toBe(160);
    const back = resetWidth(s, "name");
    expect(nameFits(back)).toBe(true);
    expect(gridTemplate(back)).toBe(gridTemplate(defaultColumns()));
  });

  it("reads back only what makes sense", () => {
    expect(readColumns(null)).toEqual(defaultColumns());
    const s = readColumns({ shown: ["size", "bogus", "description"], widths: { size: 70, eta: 5, bogus: 90, name: 400, speed: "x" } });
    expect(s.shown).toEqual(["name", "size", "description"]);
    expect(s.widths).toEqual({ size: 70, name: 400 });
    expect(readColumns({ widths: { name: 100, size: 2000 } }).widths).toEqual({});
    expect(readSort({ id: "size", desc: true })).toEqual({ id: "size", desc: true });
    expect(readSort({ id: "bogus" })).toBeNull();
    expect(readSort("x")).toBeNull();
    expect(readSort(null)).toBeNull();
  });

  it("sorts by a header, then reversed, then as added", () => {
    expect(nextSort(null, "size")).toEqual({ id: "size", desc: false });
    expect(nextSort({ id: "size", desc: false }, "size")).toEqual({ id: "size", desc: true });
    expect(nextSort({ id: "size", desc: true }, "size")).toBeNull();
    expect(nextSort({ id: "size", desc: true }, "name")).toEqual({ id: "name", desc: false });
  });
});
