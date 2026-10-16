import { describe, expect, it } from "vitest";
import { Item } from "./bindings/odm/internal/fluent/models";
import { etaText, humanBytes, lastTry, matchesFilter, matchesSearch, select, statusText } from "./format";
import { setLanguage } from "./i18n";

const item = (over: Partial<Item>) => new Item({ status: "complete", total: 0, done: 0, progress: 0, speed: 0, ...over });

describe("humanBytes", () => {
  it("steps up at 1000 with a binary divisor, as the Fyne UI does", () => {
    expect(humanBytes(0)).toBe("");
    expect(humanBytes(999)).toBe("999 B");
    expect(humanBytes(1000)).toBe("0.98 KB");
    expect(humanBytes(6.3e6)).toBe("6.01 MB");
    expect(humanBytes(1000 * 1024 * 1024)).toBe("0.98 GB");
  });
});

describe("status and time left", () => {
  it("reads like the Fyne list, in Indonesian", () => {
    setLanguage("id");
    expect(statusText(item({ status: "paused", total: 100, progress: 42.4 }))).toBe("Dijeda · 42%");
    expect(statusText(item({ status: "downloading", total: 100, progress: 42.4 }))).toBe("42.40%");
    expect(statusText(item({ status: "downloading", live: true, total: 5, progress: 100 }))).toBe("Merekam");
    expect(statusText(item({ status: "error", error: "403" }))).toBe("Gagal: 403");
    expect(etaText(item({ status: "downloading", total: 1000, done: 100, speed: 10 }))).toBe("1m 30d");
    setLanguage("en");
    expect(etaText(item({ status: "downloading", total: 1000, done: 900, speed: 10 }))).toBe("10s");
    expect(lastTry("0001-01-01T00:00:00Z")).toBe("");
  });
});

describe("filters", () => {
  it("matches categories, finished state, queues and every search word", () => {
    const d = item({ fileName: "Holiday Video.mp4", url: "https://h/x", category: "Video", queue: "Main", description: "" });
    expect(matchesFilter(d, { kind: "category", value: "Video" })).toBe(true);
    expect(matchesFilter(d, { kind: "unfinished" })).toBe(false);
    expect(matchesFilter(d, { kind: "queue", value: "Main" })).toBe(true);
    expect(matchesSearch(d, "holiday mp4")).toBe(true);
    expect(matchesSearch(d, "holiday zip")).toBe(false);
  });
});

describe("select", () => {
  const order = ["a", "b", "c", "d", "e"];
  const run = (steps: [string, string][]) => {
    let st = { selected: new Set<string>(), anchor: null as string | null };
    for (const [id, mods] of steps) {
      st = select(order, st.selected, st.anchor, id, { ctrl: mods.includes("c"), shift: mods.includes("s") });
    }
    return [...st.selected].sort().join("");
  };
  it("works as Explorer's", () => {
    expect(run([["b", ""]])).toBe("b");
    expect(run([["b", ""], ["d", "c"]])).toBe("bd");
    expect(run([["b", ""], ["d", "c"], ["b", "c"]])).toBe("d");
    expect(run([["a", ""], ["c", "s"]])).toBe("abc");
    expect(run([["a", ""], ["c", "s"], ["b", "s"]])).toBe("ab");
    expect(run([["a", ""], ["e", "c"], ["c", "cs"]])).toBe("acde");
  });
});
