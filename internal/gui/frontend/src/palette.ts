// The commands the title bar's search box offers (Ctrl K): typed words pick
// commands by their label in the page's language or in English, so "pause"
// finds "Jeda semua" too. The list below filters by the same words, so the
// box searches the downloads and runs commands at once; a pasted address
// is offered for download first.

import type { ComponentType } from "react";

export interface Command {
  id: string;
  label: string; // in the page's language
  english: string; // the label's English, for typing either
  group: string;
  icon?: ComponentType<{ className?: string }>;
  shortcut?: string;
  run: () => void;
}

const fold = (s: string) =>
  s
    .toLowerCase()
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "");

// matchCommands keeps the commands whose label (either language) holds
// every word of query, in their order; all of them for an empty query.
export function matchCommands(commands: Command[], query: string): Command[] {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) return commands;
  return commands.filter((c) => {
    const hay = fold(`${c.label} ${c.english}`);
    return words.every((w) => hay.includes(w));
  });
}

// cycle moves a highlight through n entries, wrapping at either end.
export const cycle = (at: number, delta: number, n: number) => (n === 0 ? -1 : (((at + delta) % n) + n) % n);
