// A stand-in for ODM in a plain browser: ?n=1000 makes a thousand
// downloads, some of them running, whose progress moves every half second
// as ODM's snapshots do. ?lang=id and ?theme=dark|light set the look.

import type { Api } from "./api";
import { Appearance, Item, Snapshot } from "./bindings/odm/internal/fluent/models";

const categories = ["Video", "Audio", "Documents", "Programs", "Compressed", "Images", "Disk Images", "Other"];
const samples: [string, string, number][] = [
  ["ubuntu-24.04-desktop-amd64.iso", "Disk Images", 6.1e9],
  ["holiday.mp4", "Video", 6.3e6],
  ["report-2026.pdf", "Documents", 2.1e6],
  ["Live_Video_from_the_International_Space_Station.mp4", "Video", 10.8e6],
  ["setup-x64.exe", "Programs", 92e6],
  ["album.zip", "Compressed", 310e6],
  ["podcast-episode-12.mp3", "Audio", 48e6],
  ["wallpaper.png", "Images", 4.2e6],
  ["notes.txt", "Other", 12e3],
  ["Me_at_the_zoo.mp4", "Video", 744e3],
];
const statuses = ["complete", "complete", "complete", "paused", "queued", "error", "downloading"];

function makeItems(n: number): Item[] {
  const now = Date.now();
  const out: Item[] = [];
  for (let i = 0; i < n; i++) {
    const [base, category, size] = samples[i % samples.length];
    const name = i < samples.length ? base : base.replace(/(\.[^.]+)$/, ` (${Math.floor(i / samples.length) + 1})$1`);
    const status = i < 3 ? "downloading" : statuses[i % statuses.length];
    const progress = status === "complete" ? 100 : status === "queued" ? 0 : (i * 37) % 100;
    out.push(
      new Item({
        id: `d${i}`,
        url: `https://downloads.example.com/files/${encodeURIComponent(name)}`,
        fileName: name,
        filePath: `C:\\Users\\me\\Downloads\\${name}`,
        status,
        error: status === "error" ? "connection reset by peer" : "",
        total: size,
        done: Math.round((size * progress) / 100),
        progress,
        speed: status === "downloading" ? 2.5e6 + i * 4e5 : 0,
        category,
        queue: "Main",
        description: i % 9 === 0 ? "from the browser" : "",
        createdAt: new Date(now - i * 3.6e6).toISOString(),
        updatedAt: new Date(now - i * 3.1e6).toISOString(),
      }),
    );
  }
  return out;
}

export function mockApi(): Api {
  const q = new URLSearchParams(location.search);
  const n = Number(q.get("n") || 40);
  let items = makeItems(n);
  let running: string[] = [];
  const subs = new Set<(s: Snapshot) => void>();
  const snap = () => new Snapshot({ items: items.map((d) => new Item(d)), categories, queues: ["Main"], running, limit: 0 });
  const push = () => subs.forEach((cb) => cb(snap()));

  setInterval(() => {
    for (const d of items) {
      if (d.status !== "downloading") continue;
      d.done = Math.min(d.total, d.done + d.speed / 2);
      d.progress = (d.done / d.total) * 100;
      if (d.done >= d.total) {
        d.status = "complete";
        d.speed = 0;
      }
    }
    push();
  }, 500);

  const set = (ids: string[], status: string, speed: number) => {
    for (const d of items) {
      if (ids.includes(d.id) && d.status !== "complete") {
        d.status = status;
        d.speed = speed;
      }
    }
    push();
  };

  return {
    snapshot: async () => snap(),
    onSnapshot: (cb) => {
      subs.add(cb);
      return () => subs.delete(cb);
    },
    appearance: async () =>
      new Appearance({ language: q.get("lang") || "id", theme: q.get("theme") || "system", accent: q.get("accent") || "#0067c0" }),
    resume: async (ids) => set(ids, "downloading", 3e6),
    pause: async (ids) => set(ids, "paused", 0),
    pauseAll: async () => set(items.filter((d) => d.status === "downloading").map((d) => d.id), "paused", 0),
    remove: async (ids) => {
      items = items.filter((d) => !ids.includes(d.id));
      push();
    },
    clearCompleted: async () => {
      const before = items.length;
      items = items.filter((d) => d.status !== "complete");
      push();
      return before - items.length;
    },
    startQueue: async (name) => {
      running = [...running, name];
      push();
    },
    stopQueue: async (name) => {
      running = running.filter((r) => r !== name);
      push();
    },
    addURL: async (address) => {
      const name = decodeURIComponent(address.split("/").pop() || "download");
      const d = makeItems(1)[0];
      Object.assign(d, { id: `n${Date.now()}`, url: address, fileName: name, status: "downloading", done: 0, progress: 0 });
      items = [d, ...items];
      push();
      return d;
    },
    openFile: async () => {},
    showInFolder: async () => {},
  };
}
