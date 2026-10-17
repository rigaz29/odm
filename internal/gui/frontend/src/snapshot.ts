// What ODM sends as the list changes: not the whole snapshot each time but
// a Change (internal/gui/service.go), the downloads that changed and, when
// it changed, the list's order. applyChange brings the snapshot the page
// holds up to date with one.

import type { Item, Snapshot } from "./bindings/odm/internal/gui/models";

export interface Change {
  items: Item[] | null;
  // Every download's id in order, when that changed; null when it did not.
  ids: string[] | null;
  categories: string[] | null;
  queues: string[] | null;
  running: string[] | null;
  limit: number;
  savedLimit: number;
  speeds: number[] | null;
}

// applyChange returns s with ch applied. Downloads that did not change keep
// their objects, and with none changed and the order the same, the list
// itself is s's, so what the page works out from it is not worked out again.
export function applyChange(s: Snapshot, ch: Change): Snapshot {
  const changed = ch.items ?? [];
  let items = s.items;
  if (changed.length > 0 || ch.ids) {
    const byID = new Map(s.items.map((d) => [d.id, d]));
    for (const d of changed) byID.set(d.id, d);
    const ids = ch.ids ?? s.items.map((d) => d.id);
    items = [];
    for (const id of ids) {
      const d = byID.get(id);
      if (d) items.push(d);
    }
  }
  return {
    items,
    categories: ch.categories ?? [],
    queues: ch.queues ?? [],
    running: ch.running ?? [],
    limit: ch.limit,
    savedLimit: ch.savedLimit,
    speeds: ch.speeds ?? [],
  };
}
