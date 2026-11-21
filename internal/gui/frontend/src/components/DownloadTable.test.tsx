// @vitest-environment jsdom
//
// The list's keys act only while the list itself has the focus. A row menu
// or a column menu is a portal but a React child of the list, so its keys
// used to reach the list too: walking a menu moved the selection and the
// item chosen acted on another download. The header's boxes and buttons
// keep their own keys, and the list's shortcuts work as before.

import { act } from "react";
import { type Root, createRoot } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

// ODM, as the page asks it: every call is noted as "method(args)".
const odm = vi.hoisted(() => {
  const calls: string[] = [];
  const answers: Record<string, unknown> = {};
  const api = new Proxy(answers, {
    get(own, name: string) {
      if (name in own) return own[name];
      if (name.startsWith("on")) return () => () => {}; // a subscription
      return (...args: unknown[]) => {
        calls.push(`${name}(${JSON.stringify(args).slice(1, -1)})`);
        return Promise.resolve(undefined);
      };
    },
  });
  return { api, answers, calls };
});
vi.mock("../api", () => ({ api: odm.api, inODM: true, framed: false }));

import { App } from "../App";
import { Item, Snapshot } from "../bindings/odm/internal/gui/models";
import { TooltipProvider } from "./ui/tooltip";

const ids = ["a", "b", "c", "d", "e", "f", "g", "h"];
odm.answers.snapshot = () =>
  Promise.resolve(
    new Snapshot({
      items: ids.map(
        (id, i) =>
          new Item({ id, url: `https://h.example/${id}.zip`, fileName: `${id}.zip`, status: "paused", total: 1000, done: 100, progress: 10, queue: "Main", order: i }),
      ),
      categories: ["Other"],
      queues: ["Main"],
    }),
  );

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // What jsdom lacks: Radix measures with ResizeObserver, and the virtual
  // list its box with offsetHeight (at 0 no row would be in view).
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, get: () => 800 });
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, get: () => 1200 });
});

let root: Root;
beforeEach(async () => {
  odm.calls.length = 0;
  localStorage.clear();
  const el = document.createElement("div");
  document.body.append(el);
  root = createRoot(el);
  await act(async () => root.render(<TooltipProvider><App /></TooltipProvider>));
  await act(async () => {});
});
afterEach(() => {
  act(() => root.unmount());
  document.body.innerHTML = "";
});

const list = () => document.querySelector<HTMLElement>('[role="grid"]')!;
const row = (id: string) => document.getElementById(`row-${id}`)!;
const selected = () => [...document.querySelectorAll('[role="row"][aria-selected="true"]')].map((r) => r.id.slice("row-".length));
const focused = () => document.activeElement as HTMLElement;
const text = (el: Element) => el.textContent?.trim() ?? "";
const menu = () => document.querySelector('[role="menu"]');
const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]');
const button = (inside: Element, text: string) => [...inside.querySelectorAll("button")].find((b) => b.textContent === text)!;

async function key(el: Element, k: string, mods: KeyboardEventInit = {}) {
  await act(async () => {
    el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...mods }));
    // Radix moves the focus between a menu's items in a timeout.
    await new Promise((r) => setTimeout(r, 0));
  });
}

async function rightClick(el: Element) {
  await act(async () => {
    el.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, button: 2, clientX: 20, clientY: 60 }));
  });
}

async function press(el: Element) {
  const e = new MouseEvent("mousedown", { bubbles: true, cancelable: true, button: 0 });
  await act(async () => {
    el.dispatchEvent(e);
  });
  return e;
}

async function click(el: HTMLElement) {
  await act(async () => el.click());
}

describe("the row menu", () => {
  it("is walked without moving the selection, and Resume resumes the row it is for", async () => {
    await rightClick(row("a"));
    expect(selected()).toEqual(["a"]);
    for (let i = 0; i < 12 && !text(focused()).startsWith("Resume"); i++) {
      await key(focused(), "ArrowDown");
      expect(selected()).toEqual(["a"]);
    }
    expect(text(focused())).toMatch(/^Resume/);
    await key(focused(), "Enter");
    // Enter is the item's alone: the row's progress window does not open.
    expect(odm.calls).toEqual(['resume(["a"])']);
  });

  it("asks about the row it is for, from Up to Remove + delete file", async () => {
    await rightClick(row("d"));
    await key(focused(), "ArrowUp"); // the menu wraps to its last item
    expect(text(focused())).toMatch(/^Remove \+ delete file/);
    expect(selected()).toEqual(["d"]);
    await key(focused(), "Enter");
    expect(dialog()?.textContent).toContain("Remove “d.zip” from the list and delete the files from disk?");
    await click(button(dialog()!, "Delete"));
    expect(odm.calls).toEqual(['remove(["d"],true)']);
  });

  it("keeps the selection through Home, End, Page Up, Page Down and Escape", async () => {
    await rightClick(row("c"));
    for (const k of ["End", "Home", "PageDown", "PageUp"]) {
      await key(focused(), k);
      expect(selected()).toEqual(["c"]);
    }
    await key(focused(), "Escape");
    expect(menu()).toBeNull();
    expect(selected()).toEqual(["c"]);
  });
});

describe("the header", () => {
  it("keeps the keys of its box and buttons", async () => {
    await press(row("b"));
    expect(selected()).toEqual(["b"]);
    const all = document.querySelector<HTMLElement>('[aria-label="Select all"]')!;
    const name = button(document.querySelector('[role="columnheader"]:nth-child(2)')!, "Name");
    for (const el of [all, name]) {
      el.focus();
      for (const k of [" ", "Enter", "ArrowDown", "Delete", "Escape"]) await key(el, k);
    }
    expect(odm.calls).toEqual([]);
    expect(dialog()).toBeNull();
    expect(selected()).toEqual(["b"]);
    // Its own action still is: Select all takes every row.
    await click(all);
    expect(selected()).toEqual(ids);
  });

  it("has a column menu walked and ticked without moving, pausing or resuming the selection", async () => {
    await press(row("b"));
    await rightClick(document.querySelector('[role="row"][aria-rowindex="1"]')!);
    expect(menu()).not.toBeNull();
    for (let i = 0; i < 3; i++) {
      await key(focused(), "ArrowDown");
      expect(selected()).toEqual(["b"]);
    }
    await key(focused(), "ArrowUp");
    await key(focused(), "ArrowUp");
    expect(text(focused())).toBe("Queue");
    await key(focused(), " ");
    expect(odm.calls).toEqual([]);
    expect(selected()).toEqual(["b"]);
    expect([...document.querySelectorAll('[role="columnheader"] button')].map(text)).toContain("Queue");
  });

  it("leaves the keyboard on the list when its box is pressed, as a row's box does", async () => {
    for (const box of [row("c").querySelector<HTMLElement>('button[role="checkbox"]')!, document.querySelector<HTMLElement>('[aria-label="Select all"]')!]) {
      list().blur();
      const down = await press(box);
      expect(down.defaultPrevented).toBe(true);
      expect(focused()).toBe(list());
    }
    await click(row("c").querySelector<HTMLElement>('button[role="checkbox"]')!);
    expect(selected()).toEqual(["c"]);
    await key(list(), "ArrowDown");
    expect(selected()).toEqual(["d"]);
  });
});

describe("the list's own keys", () => {
  it("move, select, open, pause or resume, copy and remove while it has the focus", async () => {
    await press(row("b"));
    expect(focused()).toBe(list());
    await key(list(), "ArrowDown");
    expect(selected()).toEqual(["c"]);
    await key(list(), "ArrowDown", { shiftKey: true });
    expect(selected()).toEqual(["c", "d"]);
    await key(list(), "End");
    expect(selected()).toEqual(["h"]);
    await key(list(), "Home");
    expect(selected()).toEqual(["a"]);
    await key(list(), "a", { ctrlKey: true });
    expect(selected()).toEqual(ids);
    await key(list(), "Escape");
    expect(selected()).toEqual([]);
    await key(list(), "ArrowDown");
    expect(selected()).toEqual(["b"]);

    await key(list(), " ");
    await key(list(), "Enter");
    await key(list(), "c", { ctrlKey: true });
    expect(odm.calls).toEqual(['resume(["b"])', 'openProgress("b")', 'copyText("https://h.example/b.zip")']);

    await key(list(), "Delete");
    expect(dialog()?.textContent).toContain("Remove “b.zip” from the list? (files are kept)");
    await click(button(dialog()!, "Cancel"));
    list().focus();
    await key(list(), "Delete", { shiftKey: true });
    expect(dialog()?.textContent).toContain("Remove “b.zip” from the list and delete the files from disk?");
    await click(button(dialog()!, "Cancel"));
  });

  it("open the keyboard's row menu with the Menu key and Shift+F10", async () => {
    await press(row("e"));
    await key(list(), "ArrowDown");
    await key(list(), "ContextMenu");
    expect(menu()).not.toBeNull();
    expect(selected()).toEqual(["f"]);
    await key(focused(), "Escape");
    expect(menu()).toBeNull();
    list().focus();
    await key(list(), "F10", { shiftKey: true });
    expect(menu()).not.toBeNull();
    expect(selected()).toEqual(["f"]);
  });
});
