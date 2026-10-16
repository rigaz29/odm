// Mockup A: shadcn/ui's look (Radix primitives + Tailwind, zinc palette,
// Inter, Lucide icons): neutral, sharp, dense, in the manner of Linear and
// Vercel.

import { createRoot } from "react-dom/client";
import {
  AppWindow,
  Archive,
  ArrowDownToLine,
  CalendarClock,
  Check,
  CheckCircle2,
  CircleDashed,
  Clock,
  Disc3,
  FileText,
  Film,
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
  Trash2,
  X,
} from "lucide-react";
import type { ComponentType } from "react";
import { twMerge } from "tailwind-merge";
import "./a.css";
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

const catIcon: Record<Category, ComponentType<{ className?: string }>> = {
  video: Film,
  audio: Music,
  docs: FileText,
  programs: AppWindow,
  archives: Archive,
  images: Image,
  disk: Disc3,
};

const cn = (...c: (string | false | undefined)[]) => twMerge(c.filter(Boolean).join(" "));

function Badge({ status }: { status: Status }) {
  const styles: Record<Status, string> = {
    downloading: "bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-blue-500/20",
    recording: "bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/20",
    paused: "bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-amber-500/20",
    queued: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 ring-zinc-500/20",
    error: "bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/20",
    complete: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/20",
  };
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset", styles[status])}>
      {status === "recording" && <span className="size-1.5 animate-pulse rounded-full bg-red-500" />}
      {status === "downloading" && <span className="size-1.5 rounded-full bg-blue-500" />}
      {statusLabel[status]}
    </span>
  );
}

function Progress({ row }: { row: Row }) {
  if (row.status === "complete") return <span className="text-xs text-muted-foreground">100%</span>;
  if (row.status === "recording") return <span className="text-xs text-muted-foreground">{bytes(row.done)} direkam</span>;
  const p = pct(row);
  const color = row.status === "error" ? "bg-red-500" : row.status === "paused" ? "bg-amber-500" : row.status === "queued" ? "bg-zinc-400" : "bg-blue-500";
  return (
    <div className="flex items-center gap-2.5">
      <div className="h-1.5 w-24 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
        <div className={cn("h-full rounded-full", color)} style={{ width: `${p}%` }} />
      </div>
      <span className="w-9 text-xs tabular-nums text-muted-foreground">{p}%</span>
    </div>
  );
}

function Checkbox({ checked }: { checked: boolean }) {
  return (
    <span className={cn("flex size-4 items-center justify-center rounded-[4px] border", checked ? "border-primary bg-primary text-primary-foreground" : "border-zinc-300 dark:border-zinc-700")}>
      {checked && <Check className="size-3" strokeWidth={3} />}
    </span>
  );
}

function Button({ children, variant = "outline", icon: Icon }: { children?: React.ReactNode; variant?: "primary" | "outline" | "ghost"; icon?: ComponentType<{ className?: string }> }) {
  const v = {
    primary: "bg-primary text-primary-foreground shadow-sm hover:opacity-90",
    outline: "border border-border bg-background shadow-xs hover:bg-accent",
    ghost: "hover:bg-accent",
  }[variant];
  return (
    <button className={cn("inline-flex h-8 items-center gap-1.5 rounded-md px-3 text-sm font-medium transition-colors", v, !children && "w-8 justify-center px-0")}>
      {Icon && <Icon className="size-4" />}
      {children}
    </button>
  );
}

function NavItem({ icon: Icon, label, count, activeItem }: { icon: ComponentType<{ className?: string }>; label: string; count?: number; activeItem?: boolean }) {
  return (
    <a className={cn("flex h-8 items-center gap-2.5 rounded-md px-2.5 text-sm", activeItem ? "bg-accent font-medium text-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground")}>
      <Icon className="size-4" />
      <span className="flex-1 truncate">{label}</span>
      {count !== undefined && <span className="text-xs tabular-nums text-muted-foreground">{count}</span>}
    </a>
  );
}

function App() {
  const spark = sparkPath(history(1.3, 40), 188, 36);
  return (
    <div className="flex h-full flex-col bg-background font-sans text-foreground">
      {/* Title bar */}
      <header className="flex h-10 shrink-0 items-center border-b border-border pl-3">
        <div className="flex items-center gap-2">
          <div className="flex size-5 items-center justify-center rounded-[5px] bg-primary text-primary-foreground">
            <ArrowDownToLine className="size-3.5" strokeWidth={2.5} />
          </div>
          <span className="text-[13px] font-semibold">ODM</span>
          <span className="text-[13px] text-muted-foreground">Ochi Download Manager</span>
        </div>
        <div className="mx-auto flex h-7 w-96 items-center gap-2 rounded-md border border-border bg-muted/50 px-2.5 text-[13px] text-muted-foreground">
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
            <NavItem icon={Inbox} label="Semua unduhan" count={rows.length} activeItem />
            <NavItem icon={ArrowDownToLine} label="Sedang berjalan" count={active.length} />
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
          <div className="mt-auto rounded-lg border border-border p-3">
            <div className="flex items-baseline justify-between">
              <span className="text-xs text-muted-foreground">Kecepatan total</span>
              <span className="text-sm font-semibold tabular-nums">{rate(totalSpeed)}</span>
            </div>
            <svg viewBox="0 0 188 36" className="mt-2 h-9 w-full text-blue-500">
              <path d={`${spark} L188,36 L0,36 Z`} className="fill-blue-500/10" />
              <path d={spark} fill="none" stroke="currentColor" strokeWidth="1.5" />
            </svg>
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

          <div className="flex items-center gap-2 px-6 pb-3">
            <div className="inline-flex h-8 items-center rounded-lg bg-muted p-0.5 text-sm">
              {["Semua", "Berjalan", "Selesai", "Gagal"].map((t, i) => (
                <span key={t} className={cn("rounded-md px-3 py-1", i === 0 ? "bg-background font-medium shadow-sm" : "text-muted-foreground")}>
                  {t}
                </span>
              ))}
            </div>
            <div className="ml-auto flex items-center gap-1 text-sm text-muted-foreground">
              <Button variant="ghost" icon={ListOrdered}>
                Antrean
              </Button>
              <Button variant="ghost" icon={CalendarClock}>
                Penjadwal
              </Button>
              <Button variant="ghost" icon={Settings} />
            </div>
          </div>

          <div className="mx-6 mb-3 min-h-0 flex-1 overflow-hidden rounded-lg border border-border">
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
                  const Icon = catIcon[r.category];
                  const sel = selected.has(r.id);
                  return (
                    <tr key={r.id} className={cn("h-11 border-b border-border last:border-0", sel ? "bg-muted/60" : "hover:bg-muted/40")}>
                      <td className="pl-3"><Checkbox checked={sel} /></td>
                      <td className="pr-4">
                        <div className="flex min-w-0 items-center gap-2.5">
                          <span className="flex size-7 shrink-0 items-center justify-center rounded-md border border-border bg-background">
                            <Icon className="size-3.5 text-muted-foreground" />
                          </span>
                          <div className="min-w-0">
                            <p className="truncate font-medium">{r.name}</p>
                            <p className="truncate text-xs text-muted-foreground">{r.error ?? r.host}</p>
                          </div>
                        </div>
                      </td>
                      <td className="tabular-nums text-muted-foreground">{bytes(r.size)}</td>
                      <td><Progress row={r} /></td>
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
          <CircleDashed className="size-3" /> {active.length} aktif
        </span>
        <span className="tabular-nums">↓ {rate(totalSpeed)}</span>
        <span>Batas kecepatan: tidak ada</span>
      </footer>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
