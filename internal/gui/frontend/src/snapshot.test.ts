import { describe, expect, it } from "vitest";
import { Item, Snapshot } from "./bindings/odm/internal/gui/models";
import { applyChange, type Change } from "./snapshot";

const item = (id: string, done = 0) => new Item({ id, fileName: `${id}.zip`, done });
const base = () => new Snapshot({ items: [item("a"), item("b"), item("c")], queues: ["Main"], speeds: [1] });
const change = (c: Partial<Change>): Change => ({
  items: [], ids: null, categories: [], queues: ["Main"], running: [], limit: 0, savedLimit: 0, speeds: [2], ...c,
});

describe("applyChange", () => {
  it("replaces the downloads that changed and keeps the rest", () => {
    const s = base();
    const next = applyChange(s, change({ items: [item("b", 5)] }));
    expect(next.items.map((d) => [d.id, d.done])).toEqual([["a", 0], ["b", 5], ["c", 0]]);
    expect(next.items[0]).toBe(s.items[0]);
    expect(next.speeds).toEqual([2]);
  });

  it("keeps the list itself when no download changed", () => {
    const s = base();
    expect(applyChange(s, change({ limit: 1024 })).items).toBe(s.items);
  });

  it("follows the order sent: added, removed and moved downloads", () => {
    const next = applyChange(base(), change({ items: [item("d")], ids: ["c", "a", "d"] }));
    expect(next.items.map((d) => d.id)).toEqual(["c", "a", "d"]);
    expect(applyChange(base(), change({ ids: [] })).items).toEqual([]);
  });

  it("drops a changed download the order does not list", () => {
    const next = applyChange(base(), change({ items: [item("x")] }));
    expect(next.items.map((d) => d.id)).toEqual(["a", "b", "c"]);
  });
});
