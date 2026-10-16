// A stand-in for ODM in a plain browser: made-up downloads in every state,
// some of them running, whose progress, connections and speeds move every
// half second as ODM's snapshots do. ?n=1000 makes a thousand; ?lang=en and
// ?theme=dark|light set the look.

import type { Api } from "./api";
import { Appearance, Item, Snapshot } from "./bindings/odm/internal/gui/models";

const categories = ["Video", "Audio", "Documents", "Programs", "Compressed", "Images", "Disk Images", "Other"];
const GB = 1024 ** 3;
const MB = 1024 ** 2;

type Seed = [name: string, category: string, size: number, status: string, progress: number, speed: number, host: string, extra?: Partial<Item>];
const seeds: Seed[] = [
  ["ubuntu-24.04.1-desktop-amd64.iso", "Disk Images", 5.8 * GB, "downloading", 63, 12.4 * MB, "releases.ubuntu.com"],
  ["Blender-4.5-windows-x64.msi", "Programs", 412 * MB, "downloading", 27, 6.1 * MB, "download.blender.org"],
  ["Siaran NASA — ISS Earth View (live)", "Video", 0, "downloading", 0, 0.9 * MB, "youtube.com", { live: true, stream: "hls", done: 248 * MB }],
  ["Interstellar.2014.2160p.UHD.mkv", "Video", 21.4 * GB, "paused", 41, 0, "cdn.example.net"],
  ["Figma-Desktop-Setup-x64.exe", "Programs", 98 * MB, "queued", 0, 0, "desktop.figma.com", { queue: "Malam hari" }],
  ["foto-liburan-bali-2026.zip", "Compressed", 1.3 * GB, "error", 32, 0, "drive.example.com", { error: "403 Forbidden" }],
  ["dataset-iklim-indonesia-2010-2026.csv", "Documents", 640 * MB, "queued", 0, 0, "data.bmkg.go.id", { queue: "Malam hari", description: "Data untuk skripsi" }],
  ["podcast-teknologi-eps-112.mp3", "Audio", 86 * MB, "complete", 100, 0, "anchor.fm"],
  ["laporan-keuangan-Q3-2026.pdf", "Documents", 3.2 * MB, "complete", 100, 0, "intranet.example.co.id"],
  ["Kuliah-Umum-AI-2026.mp4", "Video", 1.8 * GB, "complete", 100, 0, "vod.kampus.ac.id"],
  ["node-v24.14.1-x64.msi", "Programs", 31 * MB, "complete", 100, 0, "nodejs.org"],
  ["wallpaper-aurora-8k.png", "Images", 24 * MB, "complete", 100, 0, "unsplash.com"],
  ["android-studio-2026.1-windows.exe", "Programs", 1.4 * GB, "complete", 100, 0, "dl.google.com"],
  ["album-lofi-chill-vol3.zip", "Compressed", 512 * MB, "complete", 100, 0, "bandcamp.com"],
];

function makeItems(n: number): Item[] {
  const now = Date.now();
  const out: Item[] = [];
  for (let i = 0; i < n; i++) {
    const [base, category, size, status0, progress0, speed0, host, extra] = seeds[i % seeds.length];
    const copy = Math.floor(i / seeds.length);
    const name = copy === 0 ? base : base.replace(/(\.[^.]+)?$/, (ext) => ` (${copy + 1})${ext}`);
    const running = copy === 0 && status0 === "downloading";
    const status = running ? "downloading" : status0 === "downloading" ? "paused" : status0;
    const progress = progress0;
    out.push(
      new Item({
        id: `d${i}`,
        url: `https://${host}/files/${encodeURIComponent(name)}`,
        fileName: name,
        filePath: `C:\\Users\\me\\Downloads\\${name}`,
        status,
        total: size,
        done: Math.round((size * progress) / 100),
        progress,
        speed: running ? speed0 : 0,
        category,
        queue: "Main",
        order: i,
        createdAt: new Date(now - i * 3.3e6).toISOString(),
        updatedAt: new Date(now - i * 3.1e6).toISOString(),
        conns: running && !extra?.live ? Array.from({ length: i === 0 ? 8 : 4 }, (_, k) => Math.max(0, 1 - k * 0.13 - (i ? 0.3 : 0))) : [],
        speeds: running ? wave(i, speed0) : [],
        activeSince: running ? new Date(now - (i === 2 ? 2538e3 : 600e3)).toISOString() : "0001-01-01T00:00:00Z",
        ...(copy === 0 ? extra : { live: false }),
      }),
    );
  }
  return out;
}

function wave(seed: number, around: number, n = 60): number[] {
  return Array.from({ length: n }, (_, k) => Math.max(0, around * (0.75 + 0.25 * Math.sin(k * 0.45 + seed) + 0.1 * Math.cos(k * 1.7 + seed * 2))));
}

export function mockApi(): Api {
  const q = new URLSearchParams(location.search);
  const n = Number(q.get("n") || seeds.length);
  let items = makeItems(n);
  let running: string[] = [];
  const queues = ["Main", "Malam hari"];
  const limiter = { on: false, saved: 0 };
  let total = wave(9, 19 * MB);
  const base = new Map<string, number>(); // each download's speed before the jitter
  const subs = new Set<(s: Snapshot) => void>();
  const snap = () =>
    new Snapshot({
      items: items.map((d) => new Item(d)),
      categories,
      queues,
      running,
      limit: limiter.on ? limiter.saved : 0,
      savedLimit: limiter.saved,
      speeds: total.slice(),
    });
  const push = () => subs.forEach((cb) => cb(snap()));

  setInterval(() => {
    let sum = 0;
    for (const d of items) {
      if (d.status !== "downloading") continue;
      if (!base.has(d.id)) base.set(d.id, d.speed);
      const v = base.get(d.id)! * (0.85 + Math.random() * 0.3);
      d.speed = v;
      d.speeds = [...d.speeds.slice(-59), v];
      sum += v;
      if (d.live) {
        d.done += v / 2;
        continue;
      }
      d.done = Math.min(d.total, d.done + v / 2);
      d.progress = (d.done / d.total) * 100;
      d.conns = d.conns.map((c) => Math.min(1, c + 0.004));
      if (d.done >= d.total) {
        d.status = "complete";
        d.speed = 0;
      }
    }
    total = [...total.slice(-59), sum];
    push();
  }, 500);

  const set = (ids: string[], status: string, speed: number) => {
    for (const d of items) {
      if (ids.includes(d.id) && d.status !== "complete") {
        d.status = status;
        d.speed = speed;
        if (speed > 0) d.activeSince = new Date().toISOString();
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
    appearance: async () => new Appearance({ language: q.get("lang") || "id", theme: q.get("theme") || "system" }),
    resume: async (ids) => set(ids, "downloading", 3 * MB),
    pause: async (ids) => set(ids, "paused", 0),
    pauseAll: async () => set(items.filter((d) => d.status === "downloading").map((d) => d.id), "paused", 0),
    resumeAll: async () => set(items.filter((d) => d.status === "paused" || d.status === "error").map((d) => d.id), "downloading", 3 * MB),
    stopAll: async () => {
      running = [];
      set(items.filter((d) => d.status === "downloading" || d.status === "queued").map((d) => d.id), "paused", 0);
    },
    moveToQueue: async (ids, queue) => {
      for (const d of items) if (ids.includes(d.id)) d.queue = queue;
      push();
    },
    moveInQueue: async (id, delta) => {
      const d = items.find((x) => x.id === id);
      if (!d) return;
      const sib = items.filter((x) => x.queue === d.queue).sort((a, b) => a.order - b.order);
      const other = sib[sib.indexOf(d) + delta];
      if (other) [d.order, other.order] = [other.order, d.order];
      push();
    },
    setSpeedLimiter: async (on, bps) => {
      limiter.on = on;
      if (bps > 0) limiter.saved = bps;
      push();
    },
    copyText: (text) => navigator.clipboard.writeText(text),
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
      Object.assign(d, { id: `n${Date.now()}`, url: address, fileName: name, status: "downloading", done: 0, progress: 0, createdAt: new Date().toISOString() });
      items = [...items, d];
      push();
      return d;
    },
    openFile: async () => {},
    showInFolder: async () => {},
  };
}
