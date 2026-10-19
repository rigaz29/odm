// The Setup window (?view=setup, internal/gui/setup.go): yt-dlp, ffmpeg
// and odm-libtorrent, installed in one go, and ODM's extension in each
// browser. Firefox adds the signed add-on after one question of its own; a
// Chromium browser loads the folder ODM writes ("Load unpacked"), which the
// window walks through. It opens by itself when ODM opens and a program is
// missing (unless turned off below), and from the menu.

import { CircleCheck, Copy, Download, FolderOpen, LoaderCircle, Puzzle, Wrench } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { api } from "../api";
import type { ExtensionStep, SetupBrowser, SetupStatus, SetupTool } from "../bindings/odm/internal/gui/models";
import { Button } from "../components/ui/button";
import { Checkbox } from "../components/ui/checkbox";
import { t, tf } from "../i18n";

const errorText = (e: unknown) => String((e as Error)?.message ?? e);

export function SetupWindow() {
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [browsers, setBrowsers] = useState<SetupBrowser[] | null>(null);
  const [atStart, setAtStart] = useState(true);
  useEffect(() => {
    api.setTitle(t("Set up ODM"));
    void api.setupStatus().then((s) => {
      setStatus(s);
      setAtStart(s.atStart);
    });
    // Again every few seconds: a browser turns connected once its
    // extension talks to ODM.
    const timer = setInterval(() => void api.setupBrowsers().then(setBrowsers), 2000);
    return () => clearInterval(timer);
  }, []);
  if (!status) return <div className="h-full bg-background" />;
  return (
    <main className="flex h-full flex-col bg-background text-foreground">
      <div className="flex-1 space-y-6 overflow-y-auto p-5">
        <div>
          <h1 className="text-base font-semibold tracking-tight">{t("Set up ODM")}</h1>
          <p className="text-sm text-muted-foreground">{t("ODM downloads files without these. They let it download from video sites and torrents, and take over the browser's downloads.")}</p>
        </div>
        <Tools status={status} />
        <Extension browsers={browsers ?? status.browsers} releasesPage={status.releasesPage} />
      </div>
      <div className="flex items-center justify-between gap-3 border-t border-border p-3">
        <label className="flex items-center gap-2 text-sm text-muted-foreground">
          <Checkbox
            checked={atStart}
            onCheckedChange={(v) => {
              setAtStart(v === true);
              void api.setSetupAtStart(v === true);
            }}
          />
          {t("Show this window when ODM opens and a program is missing")}
        </label>
        <Button variant="brand" onClick={api.closeWindow}>
          {t("Close")}
        </Button>
      </div>
    </main>
  );
}

function Heading({ n, icon, title, text }: { n: number; icon: ReactNode; title: string; text: string }) {
  return (
    <div className="mb-3 flex items-start gap-2.5">
      <span className="mt-0.5 [&_svg]:size-5">{icon}</span>
      <div>
        <h2 className="text-sm font-semibold">
          {n} · {title}
        </h2>
        <p className="text-xs text-muted-foreground">{text}</p>
      </div>
    </div>
  );
}

function Tools({ status }: { status: SetupStatus }) {
  const [tools, setTools] = useState(status.tools);
  const missing = tools.filter((x) => x.state !== "found" && x.canInstall);
  // On by default while ODM has not asked yet; what was chosen otherwise.
  const [auto, setAuto] = useState(status.toolMode === "auto" || (status.toolMode === "" && missing.length > 0));
  // busy is the button's label while installing: the list it was for.
  const [busy, setBusy] = useState("");
  const [progress, setProgress] = useState<Record<string, number>>({});
  const [error, setError] = useState("");
  useEffect(() => api.onSetupProgress((p) => setProgress((m) => ({ ...m, [p.tool]: p.total > 0 ? p.done / p.total : -1 }))), []);

  const label = missing.length > 1 ? t("Install all") : missing.length === 1 ? tf("Install %s", missing[0].name) : "";
  const install = async () => {
    setBusy(label);
    setError("");
    try {
      for (const tool of missing) {
        setProgress((m) => ({ ...m, [tool.name]: 0 }));
        const after = await api.installTool(tool.name);
        setTools((list) => list.map((x) => (x.name === after.name ? after : x)));
      }
      await api.setToolUpdates(auto);
    } catch (e) {
      setError(errorText(e));
    }
    setBusy("");
  };
  // With nothing to install the box takes effect at once; otherwise with
  // Install.
  const toggle = (on: boolean) => {
    setAuto(on);
    if (missing.length === 0) void api.setToolUpdates(on);
  };

  return (
    <section>
      <Heading n={1} icon={<Wrench className="text-violet-500" />} title={t("Companion programs")} text={t("Downloaded from their official releases and checked against their checksums.")} />
      <ul className="divide-y divide-border rounded-lg border border-border">
        {tools.map((tool) => (
          <ToolRow key={tool.name} tool={tool} progress={busy ? progress[tool.name] : undefined} />
        ))}
      </ul>
      <div className="mt-3 flex items-center justify-between gap-3">
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={auto} onCheckedChange={(v) => toggle(v === true)} disabled={busy !== ""} />
          {t("Keep them up to date automatically")}
        </label>
        {(busy || missing.length > 0) && (
          <Button variant="brand" disabled={busy !== ""} onClick={() => void install()}>
            {busy ? <LoaderCircle className="animate-spin" /> : <Download />}
            {busy || label}
          </Button>
        )}
      </div>
      {error && <p className="mt-2 text-xs text-red-600 dark:text-red-400">{error}</p>}
    </section>
  );
}

function ToolRow({ tool, progress }: { tool: SetupTool; progress?: number }) {
  const what =
    tool.name === "yt-dlp"
      ? t("Downloads videos from YouTube, TikTok and other sites.")
      : tool.name === "ffmpeg"
        ? t("Joins a video with its audio and saves live streams as MP4.")
        : t("Downloads torrents and magnet links.");
  const found = tool.state === "found";
  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">{tool.name}</p>
        <p className={found ? "truncate text-xs text-muted-foreground" : "text-xs text-muted-foreground"} title={found ? tool.path : undefined}>
          {found ? tool.path : what}
        </p>
      </div>
      {found ? (
        <span className="flex shrink-0 items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
          <CircleCheck className="size-4" /> {tool.version || t("Installed")}
        </span>
      ) : progress !== undefined ? (
        <span className="h-1.5 w-32 shrink-0 overflow-hidden rounded-full bg-muted" role="progressbar" aria-label={tool.name}>
          <span className="accent-gradient block h-full transition-[width]" style={{ width: `${Math.max(progress, 0.03) * 100}%` }} />
        </span>
      ) : (
        <span className="shrink-0 text-xs text-muted-foreground">{tool.canInstall ? t("Not installed") : t("Not installed: ODM cannot download it here")}</span>
      )}
    </li>
  );
}

// Extension lists the browsers; where ODM cannot find them (outside
// Windows), it points to the releases page instead.
function Extension({ browsers, releasesPage }: { browsers: SetupBrowser[]; releasesPage: string }) {
  const [steps, setSteps] = useState<Record<string, ExtensionStep>>({});
  const [busy, setBusy] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const install = (b: SetupBrowser) => {
    setBusy(b.key);
    setErrors((m) => ({ ...m, [b.key]: "" }));
    api
      .installExtension(b.key)
      .then(
        (step) => {
          // The browser does not open its extensions page from outside:
          // its address is ready to paste.
          if (step.page) void api.copyText(step.page);
          setSteps((m) => ({ ...m, [b.key]: step }));
        },
        (e) => setErrors((m) => ({ ...m, [b.key]: errorText(e) })),
      )
      .finally(() => setBusy(""));
  };
  return (
    <section>
      <Heading
        n={2}
        icon={<Puzzle className="text-sky-500" />}
        title={t("Browser extension")}
        text={t("It hands the browser's downloads to ODM, and the videos a page plays.")}
      />
      {releasesPage ? (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">
            {t(
              "ODM finds the browsers on Windows only for now. Download the extension from ODM's releases page: odm-extension-firefox.xpi for Firefox (open the file with Firefox), odm-extension-chromium.zip for Chrome and other browsers (unpack it, then on the browser's extensions page turn on Developer mode and click Load unpacked).",
            )}
          </p>
          <Button size="sm" variant="outline" onClick={() => void api.openURL(releasesPage)}>
            {t("Open the releases page")}
          </Button>
        </div>
      ) : browsers.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("No browser was found.")}</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {browsers.map((b) => (
            <li key={b.key} className="px-3 py-2.5">
              <div className="flex items-center gap-3">
                <p className="min-w-0 flex-1 truncate text-sm font-medium">{b.name}</p>
                {b.connected ? (
                  <span className="flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                    <CircleCheck className="size-4" /> {t("Connected")}
                  </span>
                ) : (
                  <>
                    <span className="text-xs text-muted-foreground">{steps[b.key] ? t("Waiting for the extension…") : t("Not installed yet")}</span>
                    <Button size="sm" variant={steps[b.key] ? "outline" : "brand"} disabled={busy !== ""} onClick={() => install(b)}>
                      {busy === b.key && <LoaderCircle className="animate-spin" />}
                      {steps[b.key] ? t("Again") : t("Install")}
                    </Button>
                  </>
                )}
              </div>
              {errors[b.key] && <p className="mt-1.5 text-xs text-red-600 dark:text-red-400">{errors[b.key]}</p>}
              {steps[b.key] && !b.connected && (b.family === "firefox" ? <FirefoxHint name={b.name} /> : <ChromiumSteps name={b.name} step={steps[b.key]} />)}
            </li>
          ))}
        </ul>
      )}
      {!releasesPage && (
        <p className="mt-2 text-xs text-muted-foreground">{t("A browser shows as connected once its extension first talks to ODM, such as when the browser starts.")}</p>
      )}
    </section>
  );
}

function FirefoxHint({ name }: { name: string }) {
  return <p className="mt-2 rounded-md bg-muted/60 p-2.5 text-xs">{tf("%s asks whether to add ODM Integration: click Add.", name)}</p>;
}

// ChromiumSteps walks through "Load unpacked": browsers take an extension
// from outside their store only so.
function ChromiumSteps({ name, step }: { name: string; step: ExtensionStep }) {
  const copy = (text: string) => () => void api.copyText(text);
  return (
    <div className="mt-2 space-y-2 rounded-md bg-muted/60 p-2.5 text-xs">
      <ol className="list-decimal space-y-2 pl-4">
        <li>
          {tf("In %s, paste this address in the address bar and press Enter (it is already copied):", name)}
          <Copyable text={step.page} onCopy={copy(step.page)} />
        </li>
        <li>{t("Turn on Developer mode.")}</li>
        <li>
          {t("Click Load unpacked, paste this folder in the dialog's address bar and choose Select Folder:")}
          <Copyable text={step.folder} onCopy={copy(step.folder)}>
            <Button size="sm" variant="ghost" onClick={() => void api.showExtensionFolder()} title={t("Show the folder")} aria-label={t("Show the folder")}>
              <FolderOpen />
            </Button>
          </Copyable>
        </li>
      </ol>
      <p className="text-muted-foreground">{t("Or drag the folder onto the extensions page. Keep the folder: ODM updates it with each release.")}</p>
    </div>
  );
}

function Copyable({ text, onCopy, children }: { text: string; onCopy: () => void; children?: ReactNode }) {
  return (
    <div className="mt-1 flex items-center gap-1">
      <code className="min-w-0 flex-1 truncate rounded border border-border bg-background px-2 py-1 font-mono" title={text}>
        {text}
      </code>
      <Button size="sm" variant="ghost" onClick={onCopy} title={t("Copy")} aria-label={t("Copy")}>
        <Copy />
      </Button>
      {children}
    </div>
  );
}
