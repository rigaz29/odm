// Mockup A+D: A's shadcn/ui structure and neutral palette, with D's
// touches: the violet-to-cyan accent, cards for running downloads (a bar
// per connection, the speed over time) drawn in A's bordered style,
// coloured file tiles and a faint aurora at the top.

import { createRoot } from "react-dom/client";
import {
  AppWindow,
  Archive,
  ArrowDown,
  ArrowDownToLine,
  CalendarClock,
  Check,
  CheckCircle2,
  ChevronDown,
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
import "./ad.css";
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
const cn = (...c: (string | false | undefined)[]) => twMerge(c.filter(Boolean).join(" "));

const catIcon: Record<Category, Icon> = { video: Film, audio: Music, docs: FileText, programs: AppWindow, archives: Archive, images: Image, disk: Disc3 };
const catGrad: Record<Category, string> = {
  video: "from-fuchsia-500 to-violet-600",
  audio: "from-pink-500 to-rose-500",
  docs: "from-sky-500 to-blue-600",
  programs: "from-zinc-500 to-zinc-700",
  archives: "from-amber-400 to-orange-500",
  images: "from-emerald-400 to-teal-500",
  disk: "from-indigo-500 to-cyan-500",
};

function Tile({ c, size = "size-7", icon = "size-3.5" }: { c: Category; size?: string; icon?: string }) {
  const I = catIcon[c];
  return (
    <span className={cn("flex shrink-0 items-center justify-center rounded-lg bg-gradient-to-br text-white", catGrad[c], size)}>
      <I className={icon} strokeWidth={2.25} />
    </span>
  );
}

function Badge({ status }: { status: Status }) {
  const styles: Record<Status, string> = {
    downloading: "bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-violet-500/25",
    recording: "bg-rose-500/10 text-rose-600 dark:text-rose-300 ring-rose-500/25",
    paused: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/25",
    queued: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 ring-zinc-500/20",
    error: "bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/25",
    complete: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/25",
  };
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset", styles[status])}>
      {status === "recording" && <span className="size-1.5 animate-pulse rounded-full bg-rose-500" />}
      {status === "downloading" && <span className="accent-gradient size-1.5 rounded-full" />}
      {statusLabel[status]}
    </span>
  );
}

function Bar({ row }: { row: Row }) {
  if (row.status === "complete") return <span className="text-xs text-muted-foreground">100%</span>;
  if (row.status === "recording") return <span className="text-xs text-muted-foreground">{bytes(row.done)} direkam</span>;
  const p = pct(row);
  const fill = row.status === "downloading" ? "accent-gradient" : row.status === "error" ? "bg-red-500" : row.status === "paused" ? "bg-amber-500" : "bg-zinc-400 dark:bg-zinc-600";
  return (
    <div className="flex items-center gap-2.5">
      <div className="h-1.5 w-24 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
        <div className={cn("h-full rounded-full", fill)} style={{ width: `${p}%` }} />
      </div>
      <span className="w-9 text-xs tabular-nums text-muted-foreground">{p}%</span>
    </div>
  );
}

function Checkbox({ checked }: { checked: boolean }) {
  return (
    <span className={cn("flex size-4 items-center justify-center rounded-[4px] border", checked ? "border-transparent accent-gradient text-white" : "border-zinc-300 dark:border-zinc-700")}>
      {checked && <Check className="size-3" strokeWidth={3} />}
    </span>
  );
}

function Button({ children, variant = "outline", icon: I }: { children?: React.ReactNode; variant?: "primary" | "outline" | "ghost"; icon?: Icon }) {
  const v = {
    primary: "accent-gradient text-white shadow-md shadow-violet-500/25 hover:brightness-110",
    outline: "border border-border bg-background hover:bg-accent",
    ghost: "hover:bg-accent",
  }[variant];
  return (
    <button className={cn("inline-flex h-8 items-center gap-1.5 rounded-md px-3 text-sm font-medium", v, !children && "w-8 justify-center px-0")}>
      {I && <I className="size-4" />}
      {children}
    </button>
  );
}

function NavItem({ icon: I, label, count, on }: { icon: Icon; label: string; count?: number; on?: boolean }) {
  return (
    <a className={cn("relative flex h-8 items-center gap-2.5 rounded-md px-2.5 text-sm", on ? "bg-accent font-medium text-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground")}>
      {on && <span className="accent-gradient absolute -left-3 top-1.5 h-5 w-[3px] rounded-r-full" />}
      <I className="size-4" />
      <span className="flex-1 truncate">{label}</span>
      {count !== undefined && <span className="text-xs tabular-nums text-muted-foreground">{count}</span>}
    </a>
  );
}

function Spark({ seed, w, h, id }: { seed: number; w: number; h: number; id: string }) {
  const d = sparkPath(history(seed, 40), w, h);
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-full w-full">
      <defs>
        <linearGradient id={`l${id}`} x1="0" x2="1">
          <stop offset="0" stopColor="#8b5cf6" />
          <stop offset="1" stopColor="#22d3ee" />
        </linearGradient>
        <linearGradient id={`f${id}`} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="#8b5cf6" stopOpacity="0.2" />
          <stop offset="1" stopColor="#8b5cf6" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={`${d} L${w},${h} L0,${h} Z`} fill={`url(#f${id})`} />
      <path d={d} fill="none" stroke={`url(#l${id})`} strokeWidth="1.75" strokeLinecap="round" />
    </svg>
  );
}

function RunningCard({ r, i }: { r: Row; i: number }) {
  const live = r.status === "recording";
  return (
    <div className="flex min-w-0 flex-col gap-2.5 rounded-xl border border-border bg-card p-3.5 shadow-xs">
      <div className="flex items-center gap-2.5">
        <Tile c={r.category} size="size-8" icon="size-4" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">{r.name}</p>
          <p className="truncate text-xs text-muted-foreground">{r.host}</p>
        </div>
        {live ? (
          <span className="flex items-center gap-1.5 rounded-md bg-rose-500/10 px-2 py-0.5 text-[11px] font-semibold tracking-wide text-rose-600 ring-1 ring-inset ring-rose-500/25 dark:text-rose-300">
            <span className="size-1.5 animate-pulse rounded-full bg-rose-500" /> LIVE
          </span>
        ) : (
          <span className="text-xl font-semibold tabular-nums tracking-tight">
            {pct(r)}
            <span className="text-xs font-medium text-muted-foreground">%</span>
          </span>
        )}
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
        {live ? (
          <div className="h-full w-full animate-pulse rounded-full bg-gradient-to-r from-rose-500/60 via-fuchsia-500/60 to-violet-500/60" />
        ) : (
          <div className="accent-gradient h-full rounded-full" style={{ width: `${pct(r)}%` }} />
        )}
      </div>
      <div className="flex items-center gap-3 whitespace-nowrap text-xs text-muted-foreground">
        <span className="flex items-center gap-1 text-foreground">
          <ArrowDown className="size-3.5 text-cyan-500" />
          <b className="font-semibold tabular-nums">{rate(r.speed)}</b>
        </span>
        {live ? (
          <>
            <span className="flex items-center gap-1"><Timer className="size-3.5" /> 00:42:18</span>
            <span className="ml-auto tabular-nums">{bytes(r.done)}</span>
          </>
        ) : (
          <>
            <span className="flex items-center gap-1"><Timer className="size-3.5" /> {eta(r.eta)}</span>
            <span className="ml-auto tabular-nums">{bytes(r.done)} / {bytes(r.size)}</span>
          </>
        )}
      </div>
      <div className="flex items-end justify-between gap-3">
        {r.conns ? (
          <div className="flex min-w-0 items-center gap-[3px] whitespace-nowrap" title="Progres per koneksi">
            {r.conns.map((c, k) => (
              <span key={k} className="h-1 w-3 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
                <span className="accent-gradient block h-full rounded-full" style={{ width: `${c * 100}%` }} />
              </span>
            ))}
            <span className="ml-1.5 text-[11px] text-muted-foreground">{r.conns.length} koneksi</span>
          </div>
        ) : (
          <span className="text-[11px] text-muted-foreground">HLS · 1080p</span>
        )}
        <div className="h-7 w-20 shrink-0">
          <Spark seed={i * 1.9 + 0.3} w={80} h={28} id={`c${i}`} />
        </div>
      </div>
    </div>
  );
}

function App() {
  return (
    <div className="aurora flex h-full flex-col bg-background font-sans text-foreground">
      {/* Title bar */}
      <header className="flex h-10 shrink-0 items-center border-b border-border pl-3">
        <div className="flex items-center gap-2">
          <div className="accent-gradient flex size-5 items-center justify-center rounded-[5px] text-white shadow-sm shadow-violet-500/30">
            <ArrowDownToLine className="size-3.5" strokeWidth={2.5} />
          </div>
          <span className="text-[13px] font-semibold">ODM</span>
          <span className="text-[13px] text-muted-foreground">Ochi Download Manager</span>
        </div>
        <div className="mx-auto flex h-7 w-96 items-center gap-2 rounded-md border border-border bg-background/60 px-2.5 text-[13px] text-muted-foreground">
          <Search className="size-3.5" />
          <span className="flex-1">Cari unduhan atau tempel URL…</span>
          <kbd className="rounded border border-border bg-background px-1.5 text-[10px] font-medium">Ctrl K</kbd>
        </div>
        <div className="flex h-full">
          {[Minus, Square, X].map((I, i) => (
            <button key={i} className={cn("flex w-11 items-center justify-center text-muted-foreground", i === 2 ? "hover:bg-red-600 hover:text-white" : "hover:bg-accent")}>
              <I className={i === 1 ? "size-3" : "size-4"} />
            </button>
          ))}
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        {/* Sidebar */}
        <aside className="flex w-60 shrink-0 flex-col gap-5 border-r border-border p-3">
          <nav className="space-y-0.5">
            <NavItem icon={Inbox} label="Semua unduhan" count={rows.length} on />
            <NavItem icon={Zap} label="Sedang berjalan" count={active.length} />
            <NavItem icon={Clock} label="Belum selesai" count={unfinished} />
            <NavItem icon={CheckCircle2} label="Selesai" count={finished} />
          </nav>
          <div>
            <p className="mb-1.5 px-2.5 text-xs font-medium text-muted-foreground">Kategori</p>
            <nav className="space-y-0.5">
              {categories.map((c) => (
                <NavItem key={c.id} icon={catIcon[c.id]} label={c.label} count={countBy(c.id)} />
              ))}
            </nav>
          </div>
          <div>
            <p className="mb-1.5 px-2.5 text-xs font-medium text-muted-foreground">Antrean</p>
            <nav className="space-y-0.5">
              <NavItem icon={ListOrdered} label="Utama" count={2} />
              <NavItem icon={CalendarClock} label="Malam hari" count={0} />
            </nav>
          </div>
          <div className="mt-auto rounded-lg border border-border bg-card p-3">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Gauge className="size-3.5" /> Kecepatan total
            </div>
            <p className="accent-text mt-0.5 text-lg font-semibold tabular-nums">{rate(totalSpeed)}</p>
            <div className="mt-1 h-9">
              <Spark seed={4.2} w={188} h={36} id="total" />
            </div>
          </div>
        </aside>

        {/* Main */}
        <main className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-3 px-6 pb-3 pt-5">
            <div className="flex-1">
              <h1 className="text-lg font-semibold tracking-tight">Semua unduhan</h1>
              <p className="text-sm text-muted-foreground">
                {active.length} berjalan · {unfinished} belum selesai · {bytes(rows.reduce((n, r) => n + r.done, 0))} diunduh
              </p>
            </div>
            <Button icon={Play}>Lanjutkan</Button>
            <Button icon={Pause}>Jeda</Button>
            <Button icon={Trash2} />
            <Button variant="primary" icon={Plus}>
              Tambah URL
            </Button>
          </div>

          {/* Running downloads */}
          <section className="px-6 pb-4">
            <div className="mb-2 flex items-center gap-2">
              <ChevronDown className="size-4 text-muted-foreground" />
              <h2 className="text-sm font-medium">Sedang berjalan</h2>
              <span className="rounded-md bg-muted px-1.5 text-xs tabular-nums text-muted-foreground">{active.length}</span>
            </div>
            <div className="grid grid-cols-3 gap-3">
              {active.map((r, i) => (
                <RunningCard key={r.id} r={r} i={i} />
              ))}
            </div>
          </section>

          <div className="flex items-center gap-2 px-6 pb-3">
            <div className="inline-flex h-8 items-center rounded-lg bg-muted p-0.5 text-sm">
              {["Semua", "Berjalan", "Selesai", "Gagal"].map((t, i) => (
                <span key={t} className={cn("rounded-md px-3 py-1", i === 0 ? "bg-background font-medium shadow-sm" : "text-muted-foreground")}>
                  {t}
                </span>
              ))}
            </div>
            <div className="ml-auto flex items-center gap-1">
              <Button variant="ghost" icon={ListOrdered}>
                Antrean
              </Button>
              <Button variant="ghost" icon={CalendarClock}>
                Penjadwal
              </Button>
              <Button variant="ghost" icon={Settings} />
            </div>
          </div>

          <div className="mx-6 mb-3 min-h-0 flex-1 overflow-hidden rounded-lg border border-border bg-card">
            <table className="w-full table-fixed text-sm">
              <thead className="border-b border-border bg-muted/40 text-left text-xs text-muted-foreground">
                <tr className="h-9">
                  <th className="w-10 pl-3"><Checkbox checked={false} /></th>
                  <th className="font-medium">Nama</th>
                  <th className="w-20 font-medium">Ukuran</th>
                  <th className="w-40 font-medium">Progres</th>
                  <th className="w-24 font-medium">Kecepatan</th>
                  <th className="w-24 font-medium">Sisa waktu</th>
                  <th className="w-28 font-medium">Status</th>
                  <th className="w-28 font-medium">Ditambahkan</th>
                  <th className="w-10" />
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => {
                  const sel = selected.has(r.id);
                  return (
                    <tr key={r.id} className={cn("relative h-11 border-b border-border last:border-0", sel ? "bg-violet-500/[0.06] dark:bg-violet-500/[0.09]" : "hover:bg-muted/40")}>
                      <td className="relative pl-3">
                        {sel && <span className="accent-gradient absolute left-0 top-2 h-7 w-[3px] rounded-r-full" />}
                        <Checkbox checked={sel} />
                      </td>
                      <td className="pr-4">
                        <div className="flex min-w-0 items-center gap-2.5">
                          <Tile c={r.category} />
                          <div className="min-w-0">
                            <p className="truncate font-medium">{r.name}</p>
                            <p className={cn("truncate text-xs", r.error ? "text-red-600 dark:text-red-400" : "text-muted-foreground")}>{r.error ?? r.host}</p>
                          </div>
                        </div>
                      </td>
                      <td className="tabular-nums text-muted-foreground">{bytes(r.size)}</td>
                      <td><Bar row={r} /></td>
                      <td className="tabular-nums">{rate(r.speed)}</td>
                      <td className="tabular-nums text-muted-foreground">{eta(r.eta)}</td>
                      <td><Badge status={r.status} /></td>
                      <td className="text-muted-foreground">{r.added}</td>
                      <td><MoreHorizontal className="size-4 text-muted-foreground" /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </main>
      </div>

      {/* Status bar */}
      <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-border px-4 text-xs text-muted-foreground">
        <span>{rows.length} item · 2 dipilih</span>
        <span className="ml-auto flex items-center gap-1.5">
          <span className="accent-gradient size-1.5 rounded-full" /> {active.length} aktif
        </span>
        <span className="tabular-nums">↓ {rate(totalSpeed)}</span>
        <span>Batas kecepatan: tidak ada</span>
      </footer>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
