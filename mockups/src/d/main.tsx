// Mockup D: a look of ODM's own, "Aurora": frosted glass over a soft
// aurora, a violet-to-cyan accent, Plus Jakarta Sans. Running downloads get
// cards that show what a segmented download manager does (a bar per
// connection, the speed over time); the rest is a compact list.

import { createRoot } from "react-dom/client";
import {
  AppWindow,
  Archive,
  ArrowDown,
  CalendarClock,
  CheckCircle2,
  Clock,
  Disc3,
  FileText,
  Film,
  Gauge,
  Image,
  Inbox,
  ListOrdered,
  Minus,
  MoreHorizontal,
  Music,
  Pause,
  Play,
  Plus,
  Search,
  Settings,
  Square,
  Timer,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import type { ComponentType } from "react";
import { twMerge } from "tailwind-merge";
import "./d.css";
import {
  type Category,
  type Row,
  type Status,
  active,
  bytes,
  categories,
  countBy,
  dark,
  eta,
  finished,
  history,
  pct,
  rate,
  rows,
  selected,
  sparkPath,
  statusLabel,
  totalSpeed,
  unfinished,
} from "../data";

document.documentElement.classList.toggle("dark", dark);

type Icon = ComponentType<{ className?: string; strokeWidth?: number }>;
const catIcon: Record<Category, Icon> = { video: Film, audio: Music, docs: FileText, programs: AppWindow, archives: Archive, images: Image, disk: Disc3 };
const catGrad: Record<Category, string> = {
  video: "from-fuchsia-500 to-violet-600",
  audio: "from-pink-500 to-rose-500",
  docs: "from-sky-500 to-blue-600",
  programs: "from-slate-500 to-slate-700",
  archives: "from-amber-400 to-orange-500",
  images: "from-emerald-400 to-teal-500",
  disk: "from-indigo-500 to-cyan-500",
};
const dot: Record<Status, string> = {
  downloading: "bg-cyan-400",
  recording: "bg-rose-500",
  paused: "bg-amber-400",
  queued: "bg-slate-400",
  error: "bg-rose-500",
  complete: "bg-emerald-400",
};

const cn = (...c: (string | false | undefined)[]) => twMerge(c.filter(Boolean).join(" "));

function Tile({ c, size = "size-10" }: { c: Category; size?: string }) {
  const I = catIcon[c];
  return (
    <span className={cn("flex shrink-0 items-center justify-center rounded-xl bg-gradient-to-br text-white shadow-lg shadow-black/10", catGrad[c], size)}>
      <I className="size-[18px]" strokeWidth={2} />
    </span>
  );
}

function Nav({ icon: I, label, count, on }: { icon: Icon; label: string; count?: number; on?: boolean }) {
  return (
    <a
      className={cn(
        "relative flex h-9 items-center gap-3 rounded-xl px-3 text-[13.5px] transition",
        on
          ? "bg-gradient-to-r from-violet-500/20 via-indigo-500/10 to-transparent font-semibold text-slate-900 dark:text-white"
          : "text-slate-600 hover:bg-black/[0.04] dark:text-slate-400 dark:hover:bg-white/[0.05]",
      )}
    >
      {on && <span className="accent-gradient absolute left-0 top-2 h-5 w-[3px] rounded-full" />}
      <I className="size-4" />
      <span className="flex-1">{label}</span>
      {count !== undefined && (
        <span className={cn("rounded-full px-2 py-px text-[11px] font-semibold tabular-nums", on ? "bg-violet-500/20 text-violet-700 dark:text-violet-200" : "text-slate-500")}>{count}</span>
      )}
    </a>
  );
}

function ActiveCard({ r, seed }: { r: Row; seed: number }) {
  const live = r.status === "recording";
  const spark = sparkPath(history(seed, 36), 120, 34);
  return (
    <div className="glass flex min-w-0 flex-col gap-3 rounded-2xl p-4">
      <div className="flex items-start gap-3">
        <Tile c={r.category} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-[14px] font-semibold text-slate-900 dark:text-white">{r.name}</p>
          <p className="truncate text-xs text-slate-500 dark:text-slate-400">{r.host}</p>
        </div>
        {live ? (
          <span className="flex items-center gap-1.5 rounded-full bg-rose-500/15 px-2.5 py-1 text-[11px] font-bold tracking-wide text-rose-600 dark:text-rose-300">
            <span className="size-1.5 animate-pulse rounded-full bg-rose-500" /> LIVE
          </span>
        ) : (
          <span className="text-2xl font-bold tabular-nums tracking-tight text-slate-900 dark:text-white">
            {pct(r)}
            <span className="text-sm font-semibold text-slate-400">%</span>
          </span>
        )}
      </div>

      {live ? (
        <div className="flex items-center gap-3">
          <div className="h-2 flex-1 overflow-hidden rounded-full bg-black/[0.06] dark:bg-white/[0.07]">
            <div className="h-full w-full animate-pulse rounded-full bg-gradient-to-r from-rose-500/70 via-fuchsia-500/70 to-violet-500/70" />
          </div>
          <span className="text-xs font-semibold tabular-nums text-slate-600 dark:text-slate-300">00:42:18</span>
        </div>
      ) : (
        <div className="relative h-2 overflow-hidden rounded-full bg-black/[0.06] dark:bg-white/[0.07]">
          <div className="accent-gradient h-full rounded-full shadow-[0_0_14px_rgba(124,92,255,0.65)]" style={{ width: `${pct(r)}%` }} />
        </div>
      )}

      <div className="flex items-center gap-x-3 whitespace-nowrap text-xs text-slate-600 dark:text-slate-300">
        <span className="flex items-center gap-1"><ArrowDown className="size-3.5 text-cyan-500" /> <b className="font-semibold tabular-nums">{rate(r.speed)}</b></span>
        {live ? (
          <span className="tabular-nums">{bytes(r.done)} direkam</span>
        ) : (
          <>
            <span className="flex items-center gap-1"><Timer className="size-3.5 text-violet-500" /> {eta(r.eta)}</span>
            <span className="ml-auto tabular-nums text-slate-500">{bytes(r.done)} / {bytes(r.size)}</span>
          </>
        )}
      </div>

      <div className="flex items-end justify-between gap-3">
        {r.conns ? (
          <div className="flex min-w-0 items-center gap-1 whitespace-nowrap" title="Progres per koneksi">
            {r.conns.map((c, i) => (
              <span key={i} className="h-1.5 w-4 overflow-hidden rounded-full bg-black/[0.07] dark:bg-white/[0.08]">
                <span className="block h-full rounded-full bg-gradient-to-r from-violet-500 to-cyan-400" style={{ width: `${c * 100}%` }} />
              </span>
            ))}
            <span className="ml-1.5 text-[11px] text-slate-500">{r.conns.length} koneksi</span>
          </div>
        ) : (
          <span className="text-[11px] text-slate-500">HLS · 1080p · direkam sejak 13.58</span>
        )}
        <svg viewBox="0 0 120 34" className="h-8 w-24 shrink-0">
          <defs>
            <linearGradient id={`g${seed}`} x1="0" x2="1">
              <stop offset="0" stopColor="#8b5cf6" />
              <stop offset="1" stopColor="#22d3ee" />
            </linearGradient>
            <linearGradient id={`f${seed}`} x1="0" x2="0" y1="0" y2="1">
              <stop offset="0" stopColor="#8b5cf6" stopOpacity="0.25" />
              <stop offset="1" stopColor="#8b5cf6" stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={`${spark} L120,34 L0,34 Z`} fill={`url(#f${seed})`} />
          <path d={spark} fill="none" stroke={`url(#g${seed})`} strokeWidth="2" strokeLinecap="round" />
        </svg>
      </div>
    </div>
  );
}

function App() {
  const others = rows.filter((r) => r.status !== "downloading" && r.status !== "recording");
  const speed = sparkPath(history(4.2, 48), 196, 44);
  return (
    <div className="relative flex h-full flex-col overflow-hidden bg-[#f3f4fa] font-sans text-slate-900 dark:bg-[#06070e] dark:text-slate-100">
      {/* Aurora */}
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-40 -top-48 size-[640px] rounded-full bg-violet-400/40 blur-[120px] dark:bg-violet-700/35" />
        <div className="absolute -right-32 -top-40 size-[560px] rounded-full bg-sky-300/45 blur-[120px] dark:bg-cyan-600/25" />
        <div className="absolute -bottom-64 left-1/3 size-[620px] rounded-full bg-pink-300/35 blur-[130px] dark:bg-fuchsia-700/20" />
      </div>

      {/* Title bar */}
      <header className="relative flex h-11 shrink-0 items-center pl-4">
        <span className="accent-gradient flex size-6 items-center justify-center rounded-lg text-white shadow-md shadow-violet-500/30">
          <ArrowDown className="size-4" strokeWidth={2.75} />
        </span>
        <span className="ml-2.5 text-[13px] font-bold tracking-tight">ODM</span>
        <span className="ml-1.5 text-[13px] text-slate-500 dark:text-slate-400">Ochi Download Manager</span>
        <div className="ml-auto flex h-full">
          {[Minus, Square, X].map((I, i) => (
            <button key={i} className={cn("flex w-11 items-center justify-center text-slate-500 dark:text-slate-400", i === 2 ? "hover:bg-red-600 hover:text-white" : "hover:bg-black/5 dark:hover:bg-white/10")}>
              <I className={i === 1 ? "size-3" : "size-4"} />
            </button>
          ))}
        </div>
      </header>

      <div className="relative flex min-h-0 flex-1 gap-3 px-3 pb-3">
        {/* Sidebar */}
        <aside className="glass flex w-[232px] shrink-0 flex-col gap-4 rounded-2xl p-3">
          <nav className="space-y-0.5">
            <Nav icon={Inbox} label="Semua unduhan" count={rows.length} on />
            <Nav icon={Zap} label="Sedang berjalan" count={active.length} />
            <Nav icon={Clock} label="Belum selesai" count={unfinished} />
            <Nav icon={CheckCircle2} label="Selesai" count={finished} />
          </nav>
          <div>
            <p className="mb-1 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400">Kategori</p>
            <nav className="space-y-0.5">
              {categories.map((c) => (
                <Nav key={c.id} icon={catIcon[c.id]} label={c.label} count={countBy(c.id)} />
              ))}
            </nav>
          </div>
          <div>
            <p className="mb-1 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400">Antrean</p>
            <Nav icon={ListOrdered} label="Utama" count={2} />
          </div>
          <div className="mt-auto rounded-xl bg-black/[0.03] p-3 dark:bg-white/[0.04]">
            <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <Gauge className="size-3.5" /> Kecepatan total
            </div>
            <p className="accent-text mt-1 text-xl font-bold tabular-nums">{rate(totalSpeed)}</p>
            <svg viewBox="0 0 196 44" className="mt-1 h-11 w-full">
              <defs>
                <linearGradient id="sg" x1="0" x2="1">
                  <stop offset="0" stopColor="#8b5cf6" />
                  <stop offset="1" stopColor="#22d3ee" />
                </linearGradient>
                <linearGradient id="sf" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="0" stopColor="#22d3ee" stopOpacity="0.22" />
                  <stop offset="1" stopColor="#22d3ee" stopOpacity="0" />
                </linearGradient>
              </defs>
              <path d={`${speed} L196,44 L0,44 Z`} fill="url(#sf)" />
              <path d={speed} fill="none" stroke="url(#sg)" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </div>
        </aside>

        {/* Main */}
        <main className="flex min-w-0 flex-1 flex-col gap-3">
          <div className="flex items-center gap-3 pt-1">
            <div className="flex-1">
              <h1 className="text-[22px] font-bold tracking-tight">Unduhan</h1>
              <p className="text-[13px] text-slate-500 dark:text-slate-400">
                {active.length} berjalan · {unfinished} belum selesai · {finished} selesai
              </p>
            </div>
            <div className="glass flex h-10 w-80 items-center gap-2.5 rounded-xl px-3.5 text-[13px] text-slate-500 dark:text-slate-400">
              <Search className="size-4" />
              <span className="flex-1">Cari atau tempel URL…</span>
              <kbd className="rounded-md bg-black/5 px-1.5 py-0.5 text-[10px] font-semibold dark:bg-white/10">Ctrl K</kbd>
            </div>
            {[CalendarClock, Settings].map((I, i) => (
              <button key={i} className="glass flex size-10 items-center justify-center rounded-xl text-slate-600 dark:text-slate-300">
                <I className="size-[18px]" />
              </button>
            ))}
            <button className="accent-gradient flex h-10 items-center gap-2 rounded-xl px-4 text-[13.5px] font-semibold text-white shadow-lg shadow-violet-500/30">
              <Plus className="size-4" strokeWidth={2.5} /> Tambah URL
            </button>
          </div>

          <section>
            <div className="mb-2 flex items-center gap-2 px-1">
              <h2 className="text-[13px] font-semibold text-slate-700 dark:text-slate-200">Sedang berjalan</h2>
              <span className="rounded-full bg-cyan-500/15 px-2 text-[11px] font-semibold text-cyan-700 dark:text-cyan-300">{active.length}</span>
              <div className="ml-auto flex gap-1.5">
                {[[Pause, "Jeda semua"], [Play, "Lanjutkan"]].map(([I, l]) => {
                  const Ico = I as Icon;
                  return (
                    <button key={l as string} className="glass flex h-7 items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium text-slate-600 dark:text-slate-300">
                      <Ico className="size-3.5" /> {l as string}
                    </button>
                  );
                })}
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              {active.map((r, i) => (
                <ActiveCard key={r.id} r={r} seed={i * 1.7 + 0.4} />
              ))}
            </div>
          </section>

          <section className="glass flex min-h-0 flex-1 flex-col rounded-2xl">
            <div className="flex items-center gap-2 px-4 pb-2 pt-3">
              <h2 className="text-[13px] font-semibold text-slate-700 dark:text-slate-200">Antrean & riwayat</h2>
              <div className="ml-3 flex gap-1 text-xs">
                {["Semua", "Antre", "Dijeda", "Gagal", "Selesai"].map((t, i) => (
                  <span key={t} className={cn("rounded-lg px-2.5 py-1 font-medium", i === 0 ? "bg-violet-500/15 text-violet-700 dark:text-violet-200" : "text-slate-500")}>
                    {t}
                  </span>
                ))}
              </div>
              <button className="ml-auto flex h-7 items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium text-slate-500 hover:bg-black/5 dark:hover:bg-white/5">
                <Trash2 className="size-3.5" /> Bersihkan selesai
              </button>
            </div>
            <div className="min-h-0 flex-1 overflow-hidden px-2 pb-2">
              {others.map((r) => {
                const sel = selected.has(r.id);
                return (
                  <div key={r.id} className={cn("grid h-11 grid-cols-[28px_minmax(0,1fr)_92px_150px_110px_120px_28px] items-center gap-3 rounded-xl px-2.5 text-[13px]", sel ? "bg-violet-500/[0.12] ring-1 ring-inset ring-violet-500/25" : "hover:bg-black/[0.03] dark:hover:bg-white/[0.04]")}>
                    <Tile c={r.category} size="size-7 rounded-lg" />
                    <div className="min-w-0">
                      <p className="truncate font-medium text-slate-800 dark:text-slate-100">{r.name}</p>
                    </div>
                    <span className="tabular-nums text-slate-500 dark:text-slate-400">{bytes(r.size)}</span>
                    <span className="flex items-center gap-2">
                      {r.status === "complete" ? (
                        <span className="text-slate-400">—</span>
                      ) : (
                        <>
                          <span className="h-1.5 w-20 overflow-hidden rounded-full bg-black/[0.07] dark:bg-white/[0.08]">
                            <span className={cn("block h-full rounded-full", r.status === "error" ? "bg-rose-500" : r.status === "paused" ? "bg-amber-400" : "bg-slate-400")} style={{ width: `${pct(r)}%` }} />
                          </span>
                          <span className="text-xs tabular-nums text-slate-500">{pct(r)}%</span>
                        </>
                      )}
                    </span>
                    <span className={cn("flex items-center gap-2 text-xs font-medium", r.status === "error" ? "text-rose-600 dark:text-rose-300" : "text-slate-600 dark:text-slate-300")} title={r.error}>
                      <span className={cn("size-1.5 rounded-full", dot[r.status])} /> {statusLabel[r.status]}
                    </span>
                    <span className="text-xs text-slate-500">{r.added}</span>
                    <MoreHorizontal className="size-4 text-slate-400" />
                  </div>
                );
              })}
            </div>
          </section>
        </main>
      </div>

      <footer className="relative flex h-7 shrink-0 items-center gap-4 px-5 text-[11.5px] text-slate-500 dark:text-slate-400">
        <span>{rows.length} item · 2 dipilih</span>
        <span className="ml-auto">{active.length} aktif · ↓ {rate(totalSpeed)}</span>
        <span>Batas kecepatan: tidak ada</span>
      </footer>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
