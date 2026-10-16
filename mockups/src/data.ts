// The same downloads in every mockup, so the styles compare like for like.
// Figures are made up; nothing here talks to ODM.

export type Status = "downloading" | "recording" | "paused" | "queued" | "error" | "complete";
export type Category = "video" | "audio" | "docs" | "programs" | "archives" | "images" | "disk";

export interface Row {
  id: string;
  name: string;
  category: Category;
  size: number; // bytes; 0 when unknown (a live recording)
  done: number;
  status: Status;
  speed: number; // bytes per second
  eta: number; // seconds, 0 when none
  added: string;
  error?: string;
  conns?: number[]; // per-connection progress 0..1, for the segment bars
  host: string;
}

const GB = 1024 ** 3;
const MB = 1024 ** 2;

export const rows: Row[] = [
  { id: "r1", name: "ubuntu-24.04.1-desktop-amd64.iso", category: "disk", size: 5.8 * GB, done: 3.66 * GB, status: "downloading", speed: 12.4 * MB, eta: 190, added: "Hari ini, 14.20", conns: [1, 0.92, 0.71, 0.64, 0.55, 0.48, 0.33, 0.21], host: "releases.ubuntu.com" },
  { id: "r2", name: "Blender-4.5-windows-x64.msi", category: "programs", size: 412 * MB, done: 111 * MB, status: "downloading", speed: 6.1 * MB, eta: 51, added: "Hari ini, 14.31", conns: [0.6, 0.41, 0.25, 0.12], host: "download.blender.org" },
  { id: "r3", name: "Siaran NASA — ISS Earth View (live)", category: "video", size: 0, done: 248 * MB, status: "recording", speed: 0.9 * MB, eta: 0, added: "Hari ini, 13.58", host: "youtube.com" },
  { id: "r4", name: "Interstellar.2014.2160p.UHD.mkv", category: "video", size: 21.4 * GB, done: 8.8 * GB, status: "paused", speed: 0, eta: 0, added: "Kemarin, 22.10", host: "cdn.example.net" },
  { id: "r5", name: "Figma-Desktop-Setup-x64.exe", category: "programs", size: 98 * MB, done: 0, status: "queued", speed: 0, eta: 0, added: "Hari ini, 14.33", host: "desktop.figma.com" },
  { id: "r6", name: "foto-liburan-bali-2026.zip", category: "archives", size: 1.3 * GB, done: 0.42 * GB, status: "error", speed: 0, eta: 0, added: "Kemarin, 19.45", error: "403 Forbidden — tautan kedaluwarsa", host: "drive.example.com" },
  { id: "r7", name: "dataset-iklim-indonesia-2010-2026.csv", category: "docs", size: 640 * MB, done: 0, status: "queued", speed: 0, eta: 0, added: "Hari ini, 14.34", host: "data.bmkg.go.id" },
  { id: "r8", name: "podcast-teknologi-eps-112.mp3", category: "audio", size: 86 * MB, done: 86 * MB, status: "complete", speed: 0, eta: 0, added: "Hari ini, 09.12", host: "anchor.fm" },
  { id: "r9", name: "laporan-keuangan-Q3-2026.pdf", category: "docs", size: 3.2 * MB, done: 3.2 * MB, status: "complete", speed: 0, eta: 0, added: "Hari ini, 08.40", host: "intranet.example.co.id" },
  { id: "r10", name: "Kuliah-Umum-AI-2026.mp4", category: "video", size: 1.8 * GB, done: 1.8 * GB, status: "complete", speed: 0, eta: 0, added: "Kemarin, 16.05", host: "vod.kampus.ac.id" },
  { id: "r11", name: "node-v24.14.1-x64.msi", category: "programs", size: 31 * MB, done: 31 * MB, status: "complete", speed: 0, eta: 0, added: "Kemarin, 11.22", host: "nodejs.org" },
  { id: "r12", name: "wallpaper-aurora-8k.png", category: "images", size: 24 * MB, done: 24 * MB, status: "complete", speed: 0, eta: 0, added: "14 Okt, 20.31", host: "unsplash.com" },
  { id: "r13", name: "android-studio-2026.1-windows.exe", category: "programs", size: 1.4 * GB, done: 1.4 * GB, status: "complete", speed: 0, eta: 0, added: "13 Okt, 10.02", host: "dl.google.com" },
  { id: "r14", name: "album-lofi-chill-vol3.zip", category: "archives", size: 512 * MB, done: 512 * MB, status: "complete", speed: 0, eta: 0, added: "12 Okt, 21.47", host: "bandcamp.com" },
];

export const selected = new Set(["r2", "r4"]);

export const categories: { id: Category; label: string }[] = [
  { id: "video", label: "Video" },
  { id: "audio", label: "Audio" },
  { id: "docs", label: "Dokumen" },
  { id: "programs", label: "Program" },
  { id: "archives", label: "Arsip" },
  { id: "images", label: "Gambar" },
  { id: "disk", label: "Disk image" },
];

export const countBy = (c: Category) => rows.filter((r) => r.category === c).length;
export const active = rows.filter((r) => r.status === "downloading" || r.status === "recording");
export const unfinished = rows.filter((r) => r.status !== "complete").length;
export const finished = rows.length - unfinished;
export const totalSpeed = active.reduce((n, r) => n + r.speed, 0);

const num = (v: number, digits = 1) => new Intl.NumberFormat("id-ID", { maximumFractionDigits: digits, minimumFractionDigits: v < 10 && v % 1 !== 0 ? 1 : 0 }).format(v);

export function bytes(n: number): string {
  if (n <= 0) return "—";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1000 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${num(v, v < 100 ? 1 : 0)} ${u[i]}`;
}

export const rate = (n: number) => (n > 0 ? `${bytes(n)}/d` : "—");

export function eta(s: number): string {
  if (!s) return "—";
  const m = Math.floor(s / 60);
  const d = s % 60;
  return m > 0 ? `${m} mnt ${d} dtk` : `${d} dtk`;
}

export const pct = (r: Row) => (r.size > 0 ? Math.round((r.done / r.size) * 100) : 0);

export const statusLabel: Record<Status, string> = {
  downloading: "Mengunduh",
  recording: "Merekam",
  paused: "Dijeda",
  queued: "Antre",
  error: "Gagal",
  complete: "Selesai",
};

// ?theme=dark|light picks the mode; dark by default.
export const dark = new URLSearchParams(location.search).get("theme") !== "light";

// A speed history for the sparklines: deterministic, gently varying.
export function history(seed: number, n = 32): number[] {
  const out: number[] = [];
  let v = 0.55;
  for (let i = 0; i < n; i++) {
    v += Math.sin(i * 0.7 + seed) * 0.08 + Math.cos(i * 1.3 + seed * 2) * 0.05;
    v = Math.min(0.95, Math.max(0.15, v));
    out.push(v);
  }
  return out;
}

export function sparkPath(values: number[], w: number, h: number): string {
  const step = w / (values.length - 1);
  return values.map((v, i) => `${i ? "L" : "M"}${(i * step).toFixed(1)},${(h - v * h).toFixed(1)}`).join(" ");
}
