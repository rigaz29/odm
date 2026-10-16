import { afterEach, describe, expect, it } from "vitest";
import { Item } from "./bindings/odm/internal/gui/models";
import {
  added,
  categoryLabel,
  clock,
  displayName,
  duration,
  etaText,
  humanBytes,
  looksLikeURL,
  matchesFilter,
  matchesSearch,
  matchesTab,
  select,
  since,
  statusKind,
  statusLabel,
} from "./format";
import { setLanguage } from "./i18n";

const item = (over: Partial<Item>) => new Item({ status: "complete", total: 0, done: 0, progress: 0, speed: 0, ...over });

afterEach(() => setLanguage("en"));

describe("humanBytes", () => {
  it("steps up at 1000 with a binary divisor, as the Fyne UI does", () => {
    expect(humanBytes(0)).toBe("");
    expect(humanBytes(999)).toBe("999 B");
    expect(humanBytes(1000)).toBe("0.98 KB");
    expect(humanBytes(6.3e6)).toBe("6.01 MB");
    expect(humanBytes(1000 * 1024 * 1024)).toBe("0.98 GB");
  });
  it("uses the decimal comma in Indonesian", () => {
    setLanguage("id");
    expect(humanBytes(5.8 * 1024 ** 3)).toBe("5,80 GB");
    expect(humanBytes(412 * 1024 ** 2)).toBe("412,0 MB");
  });
});

describe("times", () => {
  it("writes durations and recording clocks", () => {
    expect(duration(42)).toBe("42 s");
    expect(duration(190)).toBe("3 min 10 s");
    expect(duration(3840)).toBe("1 h 4 min");
    setLanguage("id");
    expect(duration(190)).toBe("3 mnt 10 dtk");
    expect(clock(2538)).toBe("00:42:18");
    expect(etaText(item({ status: "downloading", total: 1000, done: 100, speed: 10 }))).toBe("1 mnt 30 dtk");
    expect(etaText(item({ status: "downloading", live: true, total: 5, done: 5, speed: 10 }))).toBe("");
  });
  it("reads start times", () => {
    expect(since("0001-01-01T00:00:00Z")).toBe(0);
    expect(since("2026-10-16T12:00:00Z", Date.parse("2026-10-16T12:01:30Z"))).toBe(90);
    const now = new Date(2026, 9, 16, 15, 0);
    expect(added(new Date(2026, 9, 16, 14, 20).toISOString(), now)).toBe("Today, 14:20");
    expect(added(new Date(2026, 9, 15, 22, 10).toISOString(), now)).toBe("Yesterday, 22:10");
    expect(added("0001-01-01T00:00:00Z", now)).toBe("");
  });
});

describe("statuses and names", () => {
  it("tells a recording from a download", () => {
    expect(statusKind(item({ status: "downloading" }))).toBe("downloading");
    expect(statusKind(item({ status: "downloading", live: true }))).toBe("recording");
    expect(statusKind(item({ status: "paused", verifying: true }))).toBe("verifying");
    setLanguage("id");
    expect(statusLabel("paused")).toBe("Dijeda");
    expect(categoryLabel("Documents")).toBe("Dokumen");
    expect(categoryLabel("My stuff")).toBe("My stuff");
  });
  it("names a download without a file name after its address", () => {
    expect(displayName(item({ url: "https://h/a/b%20c.zip" }))).toBe("b c.zip");
    expect(displayName(item({ url: "https://host.example/" }))).toBe("host.example");
  });
});

describe("filters", () => {
  it("matches the sidebar, the tabs and every search word", () => {
    const d = item({ fileName: "Holiday Video.mp4", url: "https://h/x", category: "Video", queue: "Main", description: "", status: "error" });
    expect(matchesFilter(d, { kind: "category", value: "Video" })).toBe(true);
    expect(matchesFilter(d, { kind: "unfinished" })).toBe(true);
    expect(matchesFilter(d, { kind: "running" })).toBe(false);
    expect(matchesTab(d, "failed")).toBe(true);
    expect(matchesTab(d, "finished")).toBe(false);
    expect(matchesSearch(d, "holiday mp4")).toBe(true);
    expect(matchesSearch(d, "holiday zip")).toBe(false);
    expect(looksLikeURL(" https://x.example/a.iso ")).toBe(true);
    expect(looksLikeURL("ubuntu iso")).toBe(false);
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
