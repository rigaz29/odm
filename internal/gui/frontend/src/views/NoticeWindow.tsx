// ODM's notices (?view=notice&kind=…), above other windows as ODM may be
// out of sight: the download limit reached (kind=limit), the countdown
// before the action set for when the queues finish (kind=finish&action=…),
// which Cancel stops, the question whether ODM may download yt-dlp and
// ffmpeg itself (kind=tools), as in the Fyne UI, and whether it may
// download odm-libtorrent for a torrent (kind=program&name=odm-libtorrent).

import { Download, Gauge, LoaderCircle, Magnet, Power, Wrench } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import { Button } from "../components/ui/button";
import { t, tf } from "../i18n";

const WAIT = 30; // seconds before the action runs

export function NoticeWindow({ q }: { q: URLSearchParams }) {
  const kind = q.get("kind");
  if (kind === "limit") return <LimitNotice mb={q.get("mb") ?? ""} hours={q.get("hours") ?? ""} />;
  if (kind === "tools") return <ToolsNotice />;
  if (kind === "program") return <ProgramNotice name={q.get("name") ?? ""} />;
  return <FinishNotice action={q.get("action") ?? ""} />;
}

function ToolsNotice() {
  useEffect(() => api.setTitle(t("Companion programs")), []);
  const answer = (on: boolean) => void api.setToolUpdates(on).then(api.closeWindow);
  return (
    <main className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
      <div className="flex items-center gap-2.5">
        <Wrench className="size-5 text-violet-500" />
        <h1 className="text-base font-semibold">{t("Companion programs")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">
        {t("YouTube videos need yt-dlp, and joining a video with its audio needs ffmpeg. ODM can download both from their official releases (about 20 MB), check their checksums, and keep them up to date by itself. Download them now?")}
      </p>
      <div className="mt-auto flex justify-end gap-2">
        <Button onClick={() => answer(false)}>{t("No")}</Button>
        <Button variant="brand" autoFocus onClick={() => answer(true)}>
          <Download /> {t("Download now")}
        </Button>
      </div>
    </main>
  );
}

// ProgramNotice offers to download a program fetched on demand, which
// only odm-libtorrent is: a torrent waits for it. It shows the download,
// and closes once the program is in (ODM then starts the torrents).
function ProgramNotice({ name }: { name: string }) {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(-1); // fraction of the download, -1 unknown
  const [error, setError] = useState("");
  useEffect(() => api.setTitle(t("Torrents need odm-libtorrent")), []);
  useEffect(() => api.onSetupProgress((p) => p.tool === name && setDone(p.total > 0 ? p.done / p.total : -1)), [name]);
  const install = () => {
    setBusy(true);
    setError("");
    setDone(0);
    api.installTool(name).then(api.closeWindow, (e) => {
      setBusy(false);
      setError(String((e as Error)?.message ?? e));
    });
  };
  return (
    <main className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
      <div className="flex items-center gap-2.5">
        <Magnet className="size-5 text-violet-500" />
        <h1 className="text-base font-semibold">{t("Torrents need odm-libtorrent")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">
        {t("ODM downloads torrents with odm-libtorrent, a small companion program (about 4 MB) from ODM's releases, checked against its checksum. Download it now? The torrents waiting for it start once it is in.")}
      </p>
      {busy && (
        <span className="h-1.5 overflow-hidden rounded-full bg-muted" role="progressbar" aria-label={name}>
          <span className="accent-gradient block h-full transition-[width]" style={{ width: `${Math.max(done, 0.03) * 100}%` }} />
        </span>
      )}
      {error && (
        <p className="truncate text-xs text-red-600 dark:text-red-400" title={error}>
          {error}
        </p>
      )}
      <div className="mt-auto flex justify-end gap-2">
        <Button onClick={api.closeWindow} disabled={busy}>
          {t("No")}
        </Button>
        <Button variant="brand" autoFocus disabled={busy} onClick={install}>
          {busy ? <LoaderCircle className="animate-spin" /> : <Download />} {t("Download now")}
        </Button>
      </div>
    </main>
  );
}

function LimitNotice({ mb, hours }: { mb: string; hours: string }) {
  useEffect(() => api.setTitle(t("Download limit reached")), []);
  return (
    <main className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
      <div className="flex items-center gap-2.5">
        <Gauge className="size-5 text-amber-500" />
        <h1 className="text-base font-semibold">{t("Download limit reached")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">
        {tf("More than %s MB were downloaded in the last %s hours, so downloads were stopped.", mb, hours)} {t("They can start again once the window has passed.")}
      </p>
      <div className="mt-auto flex justify-end">
        <Button variant="brand" autoFocus onClick={api.closeWindow}>
          {t("OK")}
        </Button>
      </div>
    </main>
  );
}

function FinishNotice({ action }: { action: string }) {
  const [left, setLeft] = useState(WAIT);
  const [error, setError] = useState("");
  const what =
    (
      {
        exit: t("ODM will exit"),
        sleep: t("The computer will go to sleep"),
        hibernate: t("The computer will hibernate"),
        shutdown: t("The computer will shut down"),
      } as Record<string, string>
    )[action] ?? "";
  const run = () =>
    api.runFinish(action).then(api.closeWindow, (e) => {
      setError(String((e as Error)?.message ?? e));
      setLeft(-1);
    });

  useEffect(() => api.setTitle(t("Queues finished")), []);
  useEffect(() => {
    if (left < 0) return;
    if (left === 0) {
      void run();
      return;
    }
    const timer = setTimeout(() => setLeft(left - 1), 1000);
    return () => clearTimeout(timer);
  }, [left]);

  return (
    <main className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
      <div className="flex items-center gap-2.5">
        <Power className="size-5 text-red-500" />
        <h1 className="text-base font-semibold">{t("Queues finished")}</h1>
      </div>
      <p className="text-sm">{t("All queues have finished.")}</p>
      {left >= 0 && (
        <p className="text-sm text-muted-foreground" aria-live="polite">
          {tf("%s in %d s.", what, left)}
        </p>
      )}
      {error && <p className="text-xs text-red-600 dark:text-red-400">{error}</p>}
      <div className="mt-auto flex justify-end gap-2">
        <Button autoFocus onClick={api.closeWindow}>
          {t("Cancel")}
        </Button>
        <Button variant="destructive" disabled={left < 0} onClick={() => void run()}>
          {t("Now")}
        </Button>
      </div>
    </main>
  );
}
