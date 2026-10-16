// ODM's notices (?view=notice&kind=…), above other windows as ODM may be
// out of sight: the download limit reached (kind=limit), the countdown
// before the action set for when the queues finish (kind=finish&action=…),
// which Cancel stops, and the question whether ODM may download yt-dlp and
// ffmpeg itself (kind=tools), as in the Fyne UI.

import { Download, Gauge, Power, Wrench } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import { Button } from "../components/ui/button";
import { t, tf } from "../i18n";

const WAIT = 30; // seconds before the action runs

export function NoticeWindow({ q }: { q: URLSearchParams }) {
  const kind = q.get("kind");
  if (kind === "limit") return <LimitNotice mb={q.get("mb") ?? ""} hours={q.get("hours") ?? ""} />;
  if (kind === "tools") return <ToolsNotice />;
  return <FinishNotice action={q.get("action") ?? ""} />;
}

function ToolsNotice() {
  useEffect(() => api.setTitle(t("Companion programs")), []);
  const answer = (on: boolean) => void api.setToolUpdates(on).then(api.closeWindow);
  return (
    <div className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
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
    </div>
  );
}

function LimitNotice({ mb, hours }: { mb: string; hours: string }) {
  useEffect(() => api.setTitle(t("Download limit reached")), []);
  return (
    <div className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
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
    </div>
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
    <div className="flex h-full flex-col gap-3 bg-background p-5 text-foreground">
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
    </div>
  );
}
