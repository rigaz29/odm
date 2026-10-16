// The main window's small dialogs, for the "more" menu, as the Fyne UI's
// (internal/ui/desktop.go, dialogs_desktop.go): the site grabber, About,
// the keyboard shortcuts, an update found, and a plain message (import,
// export, no update).

import { Globe, Play, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import type { About, GrabProgress, UpdateInfo } from "../bindings/odm/internal/gui/models";
import { markT, t, tf } from "../i18n";
import { Button } from "./ui/button";
import { Checkbox } from "./ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Input } from "./ui/input";

export function MessageDialog({ message, onClose }: { message: { title: string; text: string } | null; onClose: () => void }) {
  return (
    <Dialog open={message !== null} onOpenChange={(o) => !o && onClose()}>
      {message && (
        <DialogContent className="max-w-sm">
          <div className="space-y-1.5 pr-6">
            <DialogTitle>{message.title}</DialogTitle>
            <DialogDescription className="whitespace-pre-line break-words">{message.text}</DialogDescription>
          </div>
          <div className="flex justify-end">
            <Button variant="brand" autoFocus onClick={onClose}>
              {t("OK")}
            </Button>
          </div>
        </DialogContent>
      )}
    </Dialog>
  );
}

export function GrabberDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [url, setUrl] = useState("");
  const [depth, setDepth] = useState("1");
  const [pages, setPages] = useState("50");
  const [files, setFiles] = useState("100");
  const [sameHost, setSameHost] = useState(true);
  const [start, setStart] = useState(true);
  const [info, setInfo] = useState("");
  const [running, setRunning] = useState(false);

  useEffect(() => api.onGrab((p: GrabProgress) => setInfo(tf("Exploring… %d pages, %d files", p.pages, p.files))), []);

  const go = () => {
    if (!url.trim()) return setInfo(t("Enter a start page."));
    setRunning(true);
    setInfo(t("Exploring…"));
    const n = (s: string, d: number) => Number.parseInt(s, 10) || d;
    api.grab({ url, depth: n(depth, 1), maxPages: n(pages, 50), maxFiles: n(files, 100), sameHost, start }).then(
      (r) => setInfo(tf("Done: %d pages explored, %d files queued.", r.pages, r.files)),
      (e) => setInfo(t("Failed: ") + String((e as Error)?.message ?? e)),
    ).finally(() => setRunning(false));
  };
  const field = (label: string, value: string, set: (v: string) => void) => (
    <label className="grid grid-cols-[8rem_1fr] items-center gap-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <Input className="w-24" inputMode="numeric" value={value} onChange={(e) => set(e.target.value.replace(/\D/g, ""))} />
    </label>
  );

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !running && onClose()}>
      <DialogContent className="max-w-lg">
        <div className="space-y-1 pr-6">
          <DialogTitle>{t("ODM Site Grabber")}</DialogTitle>
          <DialogDescription>{t("Explores a site from a start page and adds the files it links to (the types in Options → File types).")}</DialogDescription>
        </div>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            go();
          }}
        >
          <label className="grid grid-cols-[8rem_1fr] items-center gap-3 text-sm">
            <span className="text-muted-foreground">{t("Start page")}</span>
            <span className="flex items-center gap-2 rounded-md border border-input bg-background px-2.5 focus-within:border-ring/70 focus-within:ring-2 focus-within:ring-ring/25">
              <Globe className="size-4 shrink-0 text-muted-foreground" />
              <input autoFocus className="h-8 min-w-0 flex-1 bg-transparent outline-none" value={url} placeholder="https://site.example/downloads/" onChange={(e) => setUrl(e.target.value)} />
            </span>
          </label>
          {field(t("Depth"), depth, setDepth)}
          {field(t("Max pages"), pages, setPages)}
          {field(t("Max files"), files, setFiles)}
          <label className="flex items-center gap-2.5 text-sm">
            <Checkbox checked={sameHost} onCheckedChange={(v) => setSameHost(v === true)} /> {t("Stay on the same host")}
          </label>
          <label className="flex items-center gap-2.5 text-sm">
            <Checkbox checked={start} onCheckedChange={(v) => setStart(v === true)} /> {t("Start downloads immediately")}
          </label>
          <p className="min-h-5 text-sm text-muted-foreground" aria-live="polite">
            {info}
          </p>
          <div className="flex justify-end gap-2">
            {running ? (
              <Button type="button" onClick={() => void api.stopGrab()}>
                <Square /> {t("Stop")}
              </Button>
            ) : (
              <Button type="button" onClick={onClose}>
                {t("Close")}
              </Button>
            )}
            <Button type="submit" variant="brand" disabled={running}>
              <Play /> {t("Start")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function AboutDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [about, setAbout] = useState<About | null>(null);
  useEffect(() => {
    if (open) void api.about().then(setAbout);
  }, [open]);
  const repo = about ? `https://github.com/${about.repo}` : "";
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-sm">
        <div className="flex items-center gap-3 pr-6">
          <span className="accent-gradient flex size-11 items-center justify-center rounded-xl text-lg font-bold text-white shadow-md shadow-violet-500/30">O</span>
          <div>
            <DialogTitle>{about?.name ?? "Ochi Download Manager"}</DialogTitle>
            <DialogDescription>{about ? tf("Version %s", about.version) : ""}</DialogDescription>
          </div>
        </div>
        <p className="text-sm text-muted-foreground">{t("A download manager: many connections per file, queues and a scheduler, streams and video sites, and a browser extension.")}</p>
        {about && (
          <p className="text-xs text-muted-foreground">
            <button className="text-violet-600 hover:underline dark:text-violet-400" onClick={() => void api.openURL(repo)}>
              {repo}
            </button>
            <br />
            {about.runtime}
          </p>
        )}
        <div className="flex justify-end">
          <Button variant="brand" autoFocus onClick={onClose}>
            {t("Close")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// The keys the list and the window take.
const SHORTCUTS: [string, string][] = [
  ["Ctrl+N", markT("Add a new download")],
  ["Ctrl+K, Ctrl+F", markT("Search the list or run a command")],
  ["Ctrl+A", markT("Select every download in the list")],
  ["Ctrl+C", markT("Copy the addresses of the selected downloads")],
  ["Enter", markT("Open the file, or show the progress of an unfinished download")],
  ["Space", markT("Pause or resume the selected downloads")],
  ["Del", markT("Remove the selected downloads from the list")],
  ["Shift+Del", markT("Remove them and delete their files")],
  ["↑ ↓, PgUp PgDn, Home End", markT("Move the selection (with Shift: extend it)")],
  ["Ctrl+click", markT("Add a download to the selection, or take it out")],
  ["Shift+click", markT("Select every download from the last one clicked")],
  ["Esc", markT("Clear the selection")],
];

export function ShortcutsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogTitle>{t("Keyboard shortcuts")}</DialogTitle>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
          {SHORTCUTS.map(([keys, what]) => (
            <div key={keys} className="contents">
              <dt className="whitespace-nowrap text-right">
                <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 text-xs font-medium">{keys}</kbd>
              </dt>
              <dd className="text-muted-foreground">{t(what)}</dd>
            </div>
          ))}
        </dl>
        <div className="flex justify-end">
          <Button variant="brand" autoFocus onClick={onClose}>
            {t("Close")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function UpdateDialog({ update, onClose }: { update: UpdateInfo | null; onClose: () => void }) {
  return (
    <Dialog open={update !== null} onOpenChange={(o) => !o && onClose()}>
      {update && (
        <DialogContent className="max-w-lg">
          <div className="space-y-1 pr-6">
            <DialogTitle>{t("Update available")}</DialogTitle>
            <DialogDescription>{tf("ODM %s is available — you have %s.", update.latest, update.current)}</DialogDescription>
          </div>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-muted/50 p-3 font-sans text-sm">{update.notes}</pre>
          <div className="flex justify-end gap-2">
            <Button onClick={onClose}>{t("Later")}</Button>
            <Button
              variant="brand"
              autoFocus
              onClick={() => {
                void api.openURL(update.url);
                onClose();
              }}
            >
              {t("Open release page")}
            </Button>
          </div>
        </DialogContent>
      )}
    </Dialog>
  );
}
