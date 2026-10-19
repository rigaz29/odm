// A download's progress window (?view=progress&id=…), as the Fyne UI's
// (internal/ui/dialogs_desktop.go, openProgressWindow): its status, speed
// limiter and what happens when it is done, the progress bar, Pause and
// Cancel, and the details: the connection map and each connection's part,
// or a torrent's pieces and files.
// Closing the window leaves the download running; Cancel pauses it. ODM
// closes the window when the download finishes.

import { ChevronDown, ChevronUp, CircleStop, Pause, Play, Square } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import type { Chunk, Detail } from "../bindings/odm/internal/gui/models";
import { FileTile, Sparkline } from "../components/bits";
import { PieceMap, PRIORITY_NORMAL, PrioritySelect } from "../components/TorrentFiles";
import { Button } from "../components/ui/button";
import { Checkbox } from "../components/ui/checkbox";
import { Input } from "../components/ui/input";
import { decimalText, displayName, etaText, humanBytes, speedText } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";

const POLL = 500; // ms
// The window's height with the details, as ODM opens it: a torrent's files
// need more room.
const TALL = 620;
const TORRENT_TALL = 720;

type Tab = "status" | "limit" | "done";

const KB = 1024;

export function ProgressWindow({ id }: { id: string }) {
  const [d, setD] = useState<Detail | null>(null);
  const [tab, setTab] = useState<Tab>("status");
  const [details, setDetails] = useState(true);
  const [limitKB, setLimitKB] = useState("500");
  const [useLimit, setUseLimit] = useState(false);
  const seeded = useRef(false);
  const page = useRef<HTMLElement>(null);
  const shown = useRef(details);
  const torrent = useRef(false);

  useEffect(() => {
    let live = true;
    const load = () =>
      api.detail(id).then(
        (x) => {
          if (!live) return;
          setD(x);
          if (!seeded.current) {
            seeded.current = true;
            if (x.speedLimit > 0) {
              setUseLimit(true);
              setLimitKB(String(Math.round(x.speedLimit / KB)));
            }
          }
        },
        () => live && api.closeWindow(), // removed from the list
      );
    void load();
    const timer = setInterval(load, POLL);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [id]);

  // Without the details the window takes the height the rest needs, which
  // a torrent's extra rows make taller. The window's size counts Windows'
  // frame, so the frame is what the window has past the page.
  useEffect(() => {
    if (shown.current === details) return;
    shown.current = details;
    void api.windowSize().then(({ width, height }) => {
      const main = page.current;
      const last = main?.lastElementChild;
      if (details || !main || !last) {
        api.setSize(width, Math.min(torrent.current ? TORRENT_TALL : TALL, window.screen.availHeight));
        return;
      }
      const need = last.getBoundingClientRect().bottom + Number.parseFloat(getComputedStyle(main).paddingBottom);
      api.setSize(width, Math.ceil(need + height - window.innerHeight));
    });
  }, [details]);

  const it = d?.item;
  torrent.current = !!d?.torrent;
  const recording = !!it && it.live && it.status === "downloading";
  const seeding = !!it?.seeding;
  useEffect(() => {
    if (!it) return;
    api.setTitle(recording ? tf("Recording %s", displayName(it)) : seeding ? tf("Seeding %s", displayName(it)) : `${Math.floor(it.progress)}% ${displayName(it)}`);
  }, [it?.progress, it?.fileName, recording, seeding]);

  if (!d || !it) return <div className="h-full bg-background" />;

  const running = it.status === "downloading" || it.status === "queued";
  const applyLimit = (on: boolean, kb: string) => {
    const v = Number.parseInt(kb, 10);
    void api.setDownloadLimit(id, on && v > 0 ? v * KB : 0);
  };
  const status = d.item.verifying
    ? t("Verifying checksum…")
    : recording
      ? t("Recording live stream…")
      : seeding
        ? t("Seeding: uploading to other peers…")
        : ({ queued: t("Queued"), downloading: t("Receiving data…"), paused: t("Paused"), complete: t("Complete"), error: t("Error") } as Record<string, string>)[it.status] ?? it.status;
  const rate = speedText(it.speed) || "-";
  const pct = it.total > 0 ? Math.min(100, it.progress) : 0;

  const rows: [string, string][] = [
    [t("Status"), it.error ? `${status} — ${it.error}` : status],
    [t("File size"), humanBytes(it.total) || "-"],
    [t("Downloaded"), it.live ? humanBytes(it.done) || "-" : `${humanBytes(it.done) || "-"} (${decimalText(it.progress, 2)} %)`],
    [t("Transfer rate"), rate],
    [t("Time left"), it.live || seeding ? "-" : etaText(it) || "-"],
    [t("Resumable"), d.resumable ? t("Yes") : t("No")],
  ];
  const tor = d.torrent;
  if (tor) {
    // A torrent always resumes; its peers and what it uploaded say more.
    rows.splice(5, 1, [t("Peers"), tf("%d connected (%d seeds)", tor.peers, tor.seeds)]);
    rows.push([t("Uploaded"), `${humanBytes(tor.uploaded) || "0 B"} · ${tf("ratio %s", decimalText(tor.ratio, 2))}`]);
    rows.push([t("Upload rate"), speedText(tor.uploadSpeed) || "-"]);
  }

  const tabs: [Tab, string][] = [
    ["status", t("Download status")],
    ["limit", t("Speed limiter")],
    ["done", t("Options on completion")],
  ];
  // The tabs share one cell, so the box keeps the tallest one's height
  // whichever is open, and the window fitted to it stays right.
  const panel = (k: Tab, className: string) => cn("[grid-area:1/1]", className, tab !== k && "invisible");

  return (
    <main ref={page} className="flex h-full flex-col gap-3 bg-background p-4 text-foreground">
      <div className="flex items-center gap-2.5">
        <FileTile category={it.category} size="size-9" icon="size-4" />
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-sm font-semibold" title={displayName(it)}>
            {displayName(it)}
          </h1>
          <p className="truncate text-xs text-muted-foreground" title={it.url}>
            {it.url}
          </p>
        </div>
      </div>

      <div className="inline-flex h-8 w-fit items-center rounded-lg bg-muted p-0.5 text-sm" role="tablist">
        {tabs.map(([k, label]) => (
          <button
            key={k}
            role="tab"
            aria-selected={tab === k}
            onClick={() => setTab(k)}
            className={cn("rounded-md px-3 py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring/50", tab === k ? "bg-background font-medium shadow-sm" : "text-muted-foreground hover:text-foreground")}
          >
            {label}
          </button>
        ))}
      </div>

      <div className="grid min-h-[8.5rem] shrink-0 grid-cols-1 rounded-lg border border-border bg-card p-3">
        <div className={panel("status", "flex gap-4")}>
          <dl className="grid min-w-0 flex-1 grid-cols-[7.5rem_1fr] gap-x-3 gap-y-1 text-sm">
            {rows.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-muted-foreground">{k}</dt>
                <dd className="truncate tabular-nums" title={v}>
                  {v}
                </dd>
              </div>
            ))}
          </dl>
          <Sparkline values={it.speeds.length ? it.speeds : [0]} className="h-24 w-40 shrink-0 self-end" />
        </div>
        <div className={panel("limit", "space-y-3 text-sm")}>
          <p>
            <span className="text-muted-foreground">{t("Transfer rate")}</span> <span className="ml-2 tabular-nums">{rate}</span>
          </p>
          <label className="flex items-center gap-2">
            <Checkbox
              checked={useLimit}
              onCheckedChange={(v) => {
                setUseLimit(v === true);
                applyLimit(v === true, limitKB);
              }}
            />
            {t("Use speed limiter")}
          </label>
          <div className="flex items-center gap-2">
            <span className="w-32 text-muted-foreground">{t("Maximum speed")}</span>
            <Input
              className="w-24"
              inputMode="numeric"
              disabled={!useLimit}
              value={limitKB}
              onChange={(e) => {
                const v = e.target.value.replace(/\D/g, "");
                setLimitKB(v);
                applyLimit(useLimit, v);
              }}
            />
            <span className="text-muted-foreground">KB/s</span>
          </div>
        </div>
        <div className={panel("done", "space-y-3 text-sm")}>
          <p className="flex gap-2">
            <span className="shrink-0 text-muted-foreground">{t("Save to")}</span>
            <span className="truncate" title={d.path}>
              {d.path || "-"}
            </span>
          </p>
          <label className="flex items-center gap-2">
            <Checkbox checked={d.completeDialog} onCheckedChange={(v) => void api.setCompleteDialog(v === true)} />
            {t("Show download complete dialog")}
          </label>
          <label className="flex items-center gap-2">
            <Checkbox checked={d.openWhenDone} onCheckedChange={(v) => void api.setOpenWhenDone(id, v === true)} />
            {t("Open the file when done")}
          </label>
        </div>
      </div>

      <div className="space-y-1">
        <div className="h-2 overflow-hidden rounded-full bg-muted ring-1 ring-inset ring-border">
          {recording ? (
            <div className="h-full w-full animate-pulse rounded-full bg-gradient-to-r from-violet-500/50 via-fuchsia-500/50 to-cyan-400/50" />
          ) : (
            <div className="accent-gradient h-full rounded-full transition-[width] duration-500" style={{ width: `${pct}%` }} />
          )}
        </div>
        <p className="text-right text-xs tabular-nums text-muted-foreground">{recording ? t("Recording") : `${decimalText(pct, 1)}%`}</p>
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          onClick={() => setDetails(!details)}
        >
          {details ? <ChevronUp /> : <ChevronDown />} {details ? t("Hide details") : t("Show details")}
        </Button>
        <span className="flex-1" />
        {seeding ? (
          <Button variant="outline" onClick={() => void api.stopSeeding(id)}>
            <CircleStop /> {t("Stop seeding")}
          </Button>
        ) : (
          <Button
            onClick={() => void (running || recording ? api.pause([id]) : api.resume([id]))}
            disabled={it.status === "complete"}
            variant={running || recording ? "outline" : "brand"}
          >
            {recording ? <Square /> : running ? <Pause /> : <Play />} {recording ? t("Stop") : running ? t("Pause") : t("Start")}
          </Button>
        )}
        <Button
          onClick={() => {
            void api.pause([id]);
            api.closeWindow();
          }}
        >
          {t("Cancel")}
        </Button>
      </div>

      {details && (d.torrent ? <TorrentParts id={id} d={d} /> : <Connections d={d} />)}
    </main>
  );
}

// TorrentParts is a torrent's pieces map and its files, each with how much
// of it is in. Until it is complete each file can be left out, or given
// another priority; one file at least stays.
function TorrentParts({ id, d }: { id: string; d: Detail }) {
  const tor = d.torrent!;
  // Choices sent and not yet in what ODM reports, by file index.
  const [sent, setSent] = useState<Record<number, number>>({});
  const [error, setError] = useState("");
  useEffect(() => {
    setSent((s) => {
      const left = Object.fromEntries(Object.entries(s).filter(([i, p]) => tor.files.find((f) => f.index === Number(i))?.priority !== p));
      return Object.keys(left).length === Object.keys(s).length ? s : left;
    });
  }, [tor.files]);
  const editable = d.item.status !== "complete" && !tor.seeding;
  const prio = (f: { index: number; priority: number }) => sent[f.index] ?? f.priority;
  const wanted = tor.files.filter((f) => prio(f) > 0);
  const all = wanted.length === tor.files.length;
  const choose = (change: Record<number, number>) => {
    setError("");
    setSent((s) => ({ ...s, ...change }));
    api.setTorrentFiles(id, change).catch((e) => {
      setSent((s) => Object.fromEntries(Object.entries(s).filter(([i]) => !(Number(i) in change))));
      setError(String((e as Error)?.message ?? e));
    });
  };
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-1.5">
      <p className="text-center text-xs text-muted-foreground">{tor.pieceCount > 0 ? tf("Pieces: %d", tor.pieceCount) : t("Pieces")}</p>
      {tor.pieceCount > 0 ? <PieceMap pieces={tor.pieces} count={tor.pieceCount} /> : <div className="h-4 rounded-md bg-muted ring-1 ring-inset ring-border" />}
      {error && (
        <p className="truncate text-xs text-red-600 dark:text-red-400" title={error}>
          {error}
        </p>
      )}
      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border">
        <table className="w-full table-fixed text-sm">
          <thead className="sticky top-0 bg-muted text-xs text-muted-foreground">
            <tr>
              <th className="w-9 py-1.5 pl-3 text-left font-medium">
                <Checkbox
                  checked={all ? true : "indeterminate"}
                  disabled={!editable || all}
                  aria-label={t("All files")}
                  onCheckedChange={() => choose(Object.fromEntries(tor.files.filter((f) => prio(f) === 0).map((f) => [f.index, PRIORITY_NORMAL])))}
                />
              </th>
              <th className="px-2 py-1.5 text-left font-medium">{t("File")}</th>
              <th className="w-24 px-2 py-1.5 text-right font-medium">{t("Size")}</th>
              <th className="w-20 px-2 py-1.5 text-right font-medium">{t("Downloaded")}</th>
              <th className="w-28 py-1.5 pl-2 pr-3 text-left font-medium">{t("Priority")}</th>
            </tr>
          </thead>
          <tbody>
            {tor.files.map((f) => {
              const p = prio(f);
              const last = p > 0 && wanted.length === 1;
              return (
                <tr key={f.index} className={cn("border-t border-border", p === 0 && "text-muted-foreground")}>
                  <td className="py-1 pl-3">
                    <Checkbox
                      checked={p > 0}
                      disabled={!editable || last}
                      title={editable && last ? t("The last file cannot be left out") : undefined}
                      aria-label={f.name}
                      onCheckedChange={(v) => choose({ [f.index]: v === true ? PRIORITY_NORMAL : 0 })}
                    />
                  </td>
                  <td className="truncate px-2 py-1" title={f.name}>
                    {f.name}
                  </td>
                  <td className="whitespace-nowrap px-2 py-1 text-right tabular-nums">{humanBytes(f.size) || "0 B"}</td>
                  <td className="px-2 py-1 text-right tabular-nums">{p === 0 && f.done === 0 ? "-" : `${f.size > 0 ? Math.floor((f.done / f.size) * 100) : 100}%`}</td>
                  <td className="py-1 pl-2 pr-3">
                    {p === 0 ? (
                      <span className="text-xs">{t("Skipped")}</span>
                    ) : (
                      <PrioritySelect value={p} disabled={!editable} onChange={(v) => choose({ [f.index]: v })} />
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Connections is the connection map (each part of the file and how much of
// it is in) and the table of parts; a stream has segments instead.
function Connections({ d }: { d: Detail }) {
  const it = d.item;
  if (it.stream) {
    let segs = d.segments;
    let have = d.segmentsDone;
    let text = segs > 0 ? tf("%d of %d", have, segs) : "";
    // The counts are not saved: after a restart a finished stream has none.
    if (segs === 0 && it.status === "complete") [segs, have, text] = [1, 1, t("complete")];
    if (it.live && it.status !== "complete") {
      text = have > 0 ? tf("%d recorded", have) : "";
      segs = 0;
    }
    return (
      <div className="space-y-1.5">
        <p className="text-center text-xs text-muted-foreground">
          {t("Segments:")} {text || "-"}
        </p>
        <Strip chunks={segs > 0 ? [{ start: 0, end: segs - 1, done: have, state: "" }] : []} total={segs} />
      </div>
    );
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-1.5">
      <p className="text-center text-xs text-muted-foreground">{t("Connection map")}</p>
      <Strip chunks={d.chunks} total={it.total} />
      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-muted text-xs text-muted-foreground">
            <tr>
              <th className="w-12 px-3 py-1.5 text-left font-medium">N.</th>
              <th className="w-32 px-3 py-1.5 text-left font-medium">{t("Downloaded")}</th>
              <th className="px-3 py-1.5 text-left font-medium">{t("Info")}</th>
            </tr>
          </thead>
          <tbody>
            {d.chunks.map((c, i) => (
              <tr key={c.start} className="border-t border-border">
                <td className="px-3 py-1 tabular-nums text-muted-foreground">{i + 1}</td>
                <td className="px-3 py-1 tabular-nums">{c.done > 0 ? humanBytes(c.done) : "0 B"}</td>
                <td className="truncate px-3 py-1 text-muted-foreground">{c.state}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Strip draws each part where it lies in the file, filled as far as it got.
function Strip({ chunks, total }: { chunks: Chunk[]; total: number }) {
  const size = total > 0 ? total : chunks.reduce((n, c) => Math.max(n, c.end + 1), 0);
  return (
    <div className="relative h-4 overflow-hidden rounded-md bg-muted ring-1 ring-inset ring-border">
      {size > 0 &&
        chunks.map((c) => {
          const len = Math.max(1, c.end - c.start + 1);
          return (
            <div key={c.start} className="absolute inset-y-0 border-r border-background/60" style={{ left: `${(c.start / size) * 100}%`, width: `${(len / size) * 100}%` }}>
              <div className="accent-gradient h-full" style={{ width: `${Math.min(100, (c.done / len) * 100)}%` }} />
            </div>
          );
        })}
    </div>
  );
}
