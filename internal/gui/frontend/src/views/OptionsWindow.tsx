// The Options window (?view=options&tab=…), as the Fyne UI's (internal/ui/
// dialogs_desktop.go, openOptionsTab), with its tabs as a side menu. OK
// saves (options.go) and closes; Cancel and closing the window discard.
// Registering with the browsers and updating the companion programs act at
// once, as in the Fyne UI.

import {
  AppWindow,
  CircleAlert,
  CircleCheck,
  CircleX,
  Download,
  FileType2,
  FolderOpen,
  FolderTree,
  Globe,
  KeyRound,
  Plus,
  RefreshCw,
  Settings2,
  Trash2,
  Wifi,
} from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { api } from "../api";
import { type BrowserStatus, type CategoryOption, OptionsForm, type ToolStatus } from "../bindings/odm/internal/gui/models";
import { type Confirm, ConfirmDialog } from "../components/ConfirmDialog";
import { Button } from "../components/ui/button";
import { Checkbox } from "../components/ui/checkbox";
import { Input, Select } from "../components/ui/input";
import { categoryLabel } from "../format";
import { markT, t, tf } from "../i18n";
import { cn } from "../lib/utils";

type Tab = "general" | "types" | "save" | "downloads" | "programs" | "connection" | "proxy" | "logins";

const TABS: [Tab, string, typeof Settings2][] = [
  ["general", markT("General"), Settings2],
  ["types", markT("File types"), FileType2],
  ["save", markT("Save to"), FolderTree],
  ["downloads", markT("Downloads"), Download],
  ["programs", markT("Programs"), AppWindow],
  ["connection", markT("Connection"), Wifi],
  ["proxy", markT("Proxy / Socks"), Globe],
  ["logins", markT("Site logins"), KeyRound],
];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="border-b border-border pb-1.5 text-sm font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function Hint({ children }: { children: ReactNode }) {
  return <p className="text-xs text-muted-foreground">{children}</p>;
}

function Check({ label, checked, onChange, disabled }: { label: string; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <label className={cn("flex items-start gap-2.5 text-sm", disabled && "opacity-50")}>
      <Checkbox className="mt-0.5" checked={checked} disabled={disabled} onCheckedChange={(v) => onChange(v === true)} />
      <span>{label}</span>
    </label>
  );
}

function Field({ label, children, htmlFor }: { label: string; children: ReactNode; htmlFor?: string }) {
  return (
    <div className="grid grid-cols-[11rem_1fr] items-center gap-3">
      <label htmlFor={htmlFor} className="text-sm text-muted-foreground">
        {label}
      </label>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

const num = (s: string) => Number.parseInt(s.replace(/\D/g, ""), 10) || 0;

export function OptionsWindow({ tab: first }: { tab: string }) {
  const [o, setO] = useState<OptionsForm | null>(null);
  const [tab, setTab] = useState<Tab>((TABS.find(([k]) => k === first)?.[0] ?? "general") as Tab);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [userAgent, setUserAgent] = useState("");

  useEffect(() => {
    api.setTitle(t("ODM Options"));
    void api.options().then(setO);
    void api.defaultUserAgent().then(setUserAgent);
  }, []);

  if (!o) return <div className="h-full bg-background" />;
  const set = (patch: Partial<OptionsForm>) => setO((p) => (p ? new OptionsForm({ ...p, ...patch }) : p));

  const save = async () => {
    setBusy(true);
    setError("");
    try {
      const r = await api.saveOptions(o);
      if (r.refused) {
        setTab(r.refused.tab as Tab);
        setError(r.refused.tab === "proxy" ? t("Enter the proxy server address, or choose another option.") : t("Each category needs a name of its own, which is also a folder name."));
        return;
      }
      if (r.startupError) {
        setError(tf("Could not change start-up launch: %s", r.startupError));
        return;
      }
      api.closeWindow();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex h-full bg-background text-foreground">
      <nav className="flex w-52 shrink-0 flex-col gap-0.5 border-r border-border p-3" aria-label={t("ODM Options")}>
        {TABS.map(([k, label, I]) => (
          <button
            key={k}
            onClick={() => setTab(k)}
            aria-current={tab === k}
            className={cn(
              "relative flex h-8 items-center gap-2.5 rounded-md px-2.5 text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
              tab === k ? "bg-accent font-medium" : "text-muted-foreground hover:bg-accent hover:text-foreground",
            )}
          >
            {tab === k && <span className="accent-gradient absolute -left-3 top-1.5 h-5 w-[3px] rounded-r-full" />}
            <I className="size-4 shrink-0" />
            <span className="truncate">{t(label)}</span>
          </button>
        ))}
      </nav>
      <main className="flex min-w-0 flex-1 flex-col">
        <h1 className="sr-only">{t(TABS.find(([k]) => k === tab)![1])}</h1>
        <div className="min-h-0 flex-1 space-y-6 overflow-y-auto p-5">
          {tab === "general" && <General o={o} set={set} />}
          {tab === "types" && <Types o={o} set={set} />}
          {tab === "save" && <SaveTo o={o} set={set} />}
          {tab === "downloads" && <Downloads o={o} set={set} userAgent={userAgent} />}
          {tab === "programs" && <Programs o={o} set={set} />}
          {tab === "connection" && <Connection o={o} set={set} />}
          {tab === "proxy" && <Proxy o={o} set={set} />}
          {tab === "logins" && <Logins o={o} set={set} />}
        </div>
        <div className="flex items-center gap-2 border-t border-border px-5 py-3">
          <p className="mr-auto min-w-0 truncate text-xs text-red-600 dark:text-red-400" title={error}>
            {error}
          </p>
          <Button onClick={api.closeWindow}>{t("Cancel")}</Button>
          <Button variant="brand" disabled={busy} onClick={() => void save()}>
            {t("OK")}
          </Button>
        </div>
      </main>
    </div>
  );
}

type TabProps = { o: OptionsForm; set: (patch: Partial<OptionsForm>) => void };

function General({ o, set }: TabProps) {
  const [status, setStatus] = useState<BrowserStatus | null>(null);
  const [problem, setProblem] = useState("");
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  useEffect(() => {
    void api.browserStatus().then(setStatus);
  }, []);
  const act = (f: () => Promise<BrowserStatus>) => {
    setProblem("");
    f().then(setStatus, (e) => {
      setProblem(String((e as Error)?.message ?? e));
      void api.browserStatus().then(setStatus);
    });
  };
  const state = status?.state ?? "none";
  const Icon = state === "ok" ? CircleCheck : state === "none" ? CircleX : CircleAlert;
  return (
    <>
      <Section title={t("Browser / system integration")}>
        <p className="text-sm font-medium">{t("Browser extension (Chrome, Edge, Chromium, Firefox)")}</p>
        <div
          className={cn(
            "flex items-start gap-2.5 rounded-lg border p-3 text-sm",
            state === "ok" ? "border-emerald-500/30 bg-emerald-500/[0.06]" : state === "none" ? "border-red-500/30 bg-red-500/[0.06]" : "border-amber-500/30 bg-amber-500/[0.06]",
          )}
        >
          <Icon className={cn("mt-0.5 size-4 shrink-0", state === "ok" ? "text-emerald-600" : state === "none" ? "text-red-600" : "text-amber-600")} />
          <div className="space-y-1">
            <p>{status?.summary ?? "…"}</p>
            <Hint>
              {state === "ok"
                ? t("Install the ODM extension in each browser (see the README), then restart the browser once.")
                : state === "none"
                  ? t("Register so browsers can hand downloads to ODM.")
                  : t("Register to point every browser at this ODM.")}
            </Hint>
          </div>
        </div>
        {problem && <p className="whitespace-pre-line text-xs text-red-600 dark:text-red-400">{problem}</p>}
        <div className="flex gap-2">
          <Button variant="brand" onClick={() => act(api.registerBrowsers)}>
            {state === "ok" ? t("Register again") : t("Register")}
          </Button>
          <Button
            disabled={!status?.registered}
            onClick={() =>
              setConfirm({
                title: t("Unregister browser integration"),
                message: t("Browsers will stop handing downloads to ODM until you register again. Continue?"),
                yes: t("Unregister"),
                destructive: true,
                run: () => act(api.unregisterBrowsers),
              })
            }
          >
            {t("Unregister")}
          </Button>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm" htmlFor="opt-exclude">
            {t("The browser extension leaves downloads from these sites alone:")}
          </label>
          <textarea
            id="opt-exclude"
            rows={2}
            value={o.excludeSites}
            onChange={(e) => set({ excludeSites: e.target.value })}
            placeholder="*.update.microsoft.com example.org"
            className="w-full rounded-md border border-input bg-background px-2.5 py-1.5 text-sm outline-none focus:border-ring/70 focus:ring-2 focus:ring-ring/25"
          />
          <Hint>{t("Separate names by spaces. A leading *. matches subdomains.")}</Hint>
        </div>
        <div className="space-y-2.5">
          <Check label={t("Launch ODM on startup (in the system tray)")} checked={o.startup} onChange={(v) => set({ startup: v })} />
          <Check label={t("Keep ODM running in the system tray when the window is closed")} checked={o.closeToTray} onChange={(v) => set({ closeToTray: v })} />
          <Check label={t("Automatically start downloading of URLs placed to the clipboard")} checked={o.clipboard} onChange={(v) => set({ clipboard: v })} />
          <Check label={t("Check for updates at start-up")} checked={o.updateCheck} onChange={(v) => set({ updateCheck: v })} />
        </div>
      </Section>
      <Section title={t("Appearance")}>
        <Field label={t("Theme")} htmlFor="opt-theme">
          <Select id="opt-theme" className="w-56" value={o.theme} onChange={(e) => set({ theme: e.target.value })}>
            <option value="system">{t("Same as Windows")}</option>
            <option value="light">{t("Light")}</option>
            <option value="dark">{t("Dark")}</option>
          </Select>
        </Field>
        <Field label={t("Language")} htmlFor="opt-lang">
          <Select id="opt-lang" className="w-56" value={o.language || "system"} onChange={(e) => set({ language: e.target.value })}>
            <option value="system">{t("Same as the system")}</option>
            <option value="en">English</option>
            <option value="id">Bahasa Indonesia</option>
          </Select>
        </Field>
        <Hint>{t("Every window takes a new theme or language as soon as you press OK.")}</Hint>
      </Section>
      <ConfirmDialog confirm={confirm} onClose={() => setConfirm(null)} />
    </>
  );
}

function Types({ o, set }: TabProps) {
  return (
    <Section title={t("Downloaded file types")}>
      <label className="text-sm" htmlFor="opt-types">
        {t("Automatically start downloading the following file types:")}
      </label>
      <textarea
        id="opt-types"
        rows={7}
        value={o.fileTypes}
        onChange={(e) => set({ fileTypes: e.target.value })}
        className="w-full rounded-md border border-input bg-background px-2.5 py-1.5 font-mono text-xs leading-relaxed outline-none focus:border-ring/70 focus:ring-2 focus:ring-ring/25"
      />
      <div className="flex items-start gap-3">
        <Hint>{t("Used by the clipboard monitor and the site grabber. Separate extensions with spaces.")}</Hint>
        <Button size="sm" className="ml-auto" onClick={() => void api.defaultFileTypes().then((v) => set({ fileTypes: v }))}>
          {t("Default")}
        </Button>
      </div>
    </Section>
  );
}

function FolderField({ id, value, onChange, placeholder }: { id: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <div className="flex gap-2">
      <Input id={id} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} spellCheck={false} />
      <Button onClick={() => void api.pickFolder(value || placeholder || "").then((d) => d && onChange(d))}>
        <FolderOpen /> {t("Browse")}
      </Button>
    </div>
  );
}

function SaveTo({ o, set }: TabProps) {
  const [at, setAt] = useState(0);
  const [newName, setNewName] = useState<string | null>(null);
  const [problem, setProblem] = useState("");
  const cats = o.categories;
  const cur = cats[Math.min(at, cats.length - 1)];
  const isOther = cur?.name === "Other";
  const put = (i: number, patch: Partial<CategoryOption>) => set({ categories: cats.map((c, k) => (k === i ? { ...c, ...patch } : c)) as CategoryOption[] });
  const add = () => {
    const n = (newName ?? "").trim();
    if (!n) return setProblem(t("Enter a name."));
    if (cats.some((c) => c.name.toLowerCase() === n.toLowerCase())) return setProblem(tf("There is already a category named %s.", n));
    if (/[\\/:*?"<>|]/.test(n)) return setProblem(t("A category name is also a folder name, so it cannot contain \\ / : * ? \" < > |."));
    const next = [...cats];
    next.splice(Math.max(0, next.length - 1), 0, { name: n, exts: "", folder: "" } as CategoryOption); // Other stays last
    set({ categories: next });
    setAt(next.length - 2);
    setNewName(null);
    setProblem("");
  };
  return (
    <>
      <Section title={t("Categories and folders")}>
        <Field label={t("Default download folder")} htmlFor="opt-dir">
          <FolderField id="opt-dir" value={o.dir} onChange={(v) => set({ dir: v })} />
        </Field>
        <div className="space-y-2.5">
          <Check label={t("Put files in category subfolders")} checked={o.categoryFolders} onChange={(v) => set({ categoryFolders: v })} />
          <Check label={t("Remember the folder last chosen in the New Download window")} checked={o.rememberDir} onChange={(v) => set({ rememberDir: v })} />
        </div>
        <Hint>{t("Downloads added from the clipboard, the site grabber or the API use the default folder.")}</Hint>
      </Section>
      <Section title={t("Category")}>
        <div className="flex gap-4">
          <div className="w-44 shrink-0 space-y-2">
            <div role="listbox" aria-label={t("Category")} className="max-h-60 overflow-y-auto rounded-lg border border-border p-1">
              {cats.map((c, i) => (
                <button
                  key={c.name}
                  role="option"
                  aria-selected={i === at}
                  onClick={() => setAt(i)}
                  className={cn("flex h-7 w-full items-center rounded-md px-2 text-left text-sm", i === at ? "bg-accent font-medium" : "hover:bg-accent/60")}
                >
                  <span className="truncate">{categoryLabel(c.name)}</span>
                </button>
              ))}
            </div>
            {newName === null ? (
              <div className="flex flex-wrap gap-1.5">
                <Button size="sm" onClick={() => setNewName("")}>
                  <Plus /> {t("New…")}
                </Button>
                <Button
                  size="sm"
                  disabled={isOther || !cur}
                  onClick={() => {
                    set({ categories: cats.filter((_, k) => k !== at) as CategoryOption[] });
                    setAt(0);
                  }}
                >
                  <Trash2 /> {t("Delete")}
                </Button>
                <Button
                  size="sm"
                  onClick={() =>
                    void api.defaultCategories().then((d) => {
                      // The folders stay with the categories that are back.
                      const folders = new Map(cats.map((c) => [c.name, c.folder]));
                      set({ categories: d.map((c) => ({ ...c, folder: folders.get(c.name) ?? "" })) as CategoryOption[] });
                      setAt(0);
                    })
                  }
                >
                  {t("Defaults")}
                </Button>
              </div>
            ) : (
              <form
                className="space-y-1.5"
                onSubmit={(e) => {
                  e.preventDefault();
                  add();
                }}
              >
                <Input autoFocus value={newName} placeholder={t("Comics")} onChange={(e) => setNewName(e.target.value)} aria-label={t("New category")} />
                <div className="flex gap-1.5">
                  <Button size="sm" type="submit" variant="brand">
                    {t("Add")}
                  </Button>
                  <Button size="sm" type="button" onClick={() => (setNewName(null), setProblem(""))}>
                    {t("Cancel")}
                  </Button>
                </div>
              </form>
            )}
            {problem && <p className="text-xs text-red-600 dark:text-red-400">{problem}</p>}
          </div>
          {cur && (
            <div className="min-w-0 flex-1 space-y-3">
              <div className="space-y-1.5">
                <label className="text-sm text-muted-foreground" htmlFor="opt-exts">
                  {t("File types")}
                </label>
                <Input id="opt-exts" value={cur.exts} disabled={isOther} placeholder="mp4 mkv avi" onChange={(e) => put(at, { exts: e.target.value })} spellCheck={false} />
                <Hint>
                  {isOther
                    ? t("Other takes every file the categories above do not claim.")
                    : t("Separate extensions with spaces. An extension listed in two categories belongs to the first.")}
                </Hint>
              </div>
              <div className="space-y-1.5">
                <label className="text-sm text-muted-foreground" htmlFor="opt-catdir">
                  {t("Folder for this category")}
                </label>
                <FolderField id="opt-catdir" value={cur.folder} placeholder={`${o.dir.replace(/[\\/]+$/, "")}\\${cur.name}`} onChange={(v) => put(at, { folder: v })} />
                <Hint>{t("Empty keeps <default folder>/<category>. Category folders apply when subfolders are on.")}</Hint>
              </div>
            </div>
          )}
        </div>
      </Section>
    </>
  );
}

function Downloads({ o, set, userAgent }: TabProps & { userAgent: string }) {
  return (
    <Section title={t("Default download settings")}>
      <div className="space-y-2.5">
        <Check label={t("Show the New Download window for downloads from the browser")} checked={o.startDialog} onChange={(v) => set({ startDialog: v })} />
        <Check label={t("Show the progress window when a download starts")} checked={o.progressDialog} onChange={(v) => set({ progressDialog: v })} />
        <Check label={t("Show the download complete dialog")} checked={o.completeDialog} onChange={(v) => set({ completeDialog: v })} />
      </div>
      <Hint>{t("These settings do not apply to queue processing.")}</Hint>
      <div className="space-y-2.5 border-t border-border pt-3">
        <Check label={t("Show a notification when a download finishes or fails")} checked={o.notifications} onChange={(v) => set({ notifications: v })} />
        {o.canSound && <Check label={t("Play a sound when a download finishes or fails")} checked={o.sounds} onChange={(v) => set({ sounds: v })} />}
        <Hint>{t("A finished download is not notified when its complete dialog is shown while ODM is in front.")}</Hint>
      </div>
      <div className="space-y-1.5 border-t border-border pt-3">
        <label className="text-sm" htmlFor="opt-dup">
          {t("If a link that is already in the list is added by the browser, clipboard or grabber:")}
        </label>
        <Select id="opt-dup" className="w-64" value={o.duplicateAgain ? "again" : "skip"} onChange={(e) => set({ duplicateAgain: e.target.value === "again" })}>
          <option value="skip">{t("Skip it")}</option>
          <option value="again">{t("Download it again")}</option>
        </Select>
        <Hint>{t("Adding a duplicate by hand always asks.")}</Hint>
      </div>
      <div className="space-y-1.5 border-t border-border pt-3">
        <label className="text-sm" htmlFor="opt-ua">
          {t("User-Agent for downloads:")}
        </label>
        <Input id="opt-ua" value={o.userAgent} placeholder={userAgent} onChange={(e) => set({ userAgent: e.target.value })} spellCheck={false} />
      </div>
    </Section>
  );
}

function toolText(tool: "ffmpeg" | "yt-dlp", s: ToolStatus | null): string {
  if (!s) return t("Checking…");
  switch (s.state) {
    case "found":
      return tf("Using %s (%s)", s.path, s.version || t("unknown version"));
    case "broken":
      return tf("Found %s, but it did not run: %s", s.path, s.error);
    case "nothere":
      return tool === "ffmpeg"
        ? t("No ffmpeg at this path. Leave it empty to search next to ODM and on PATH.")
        : t("No yt-dlp at this path. Leave it empty to search next to ODM and on PATH.");
  }
  return tool === "ffmpeg"
    ? t("Not found. Streams that keep video and audio apart need it: put ffmpeg next to ODM, add it to PATH, or choose it here.")
    : t("Not found. YouTube and similar sites need it: put yt-dlp next to ODM, add it to PATH, or choose it here.");
}

// useToolStatus checks the typed path once typing pauses.
function useToolStatus(tool: string, typed: string, again: number): ToolStatus | null {
  const [s, setS] = useState<ToolStatus | null>(null);
  const asked = useRef("");
  useEffect(() => {
    const key = `${typed}#${again}`;
    asked.current = key;
    setS(null);
    const timer = setTimeout(() => void api.toolStatus(tool, typed).then((r) => asked.current === key && setS(r)), 600);
    return () => clearTimeout(timer);
  }, [tool, typed, again]);
  return s;
}

// The browsers yt-dlp can take cookies from itself (ytdlp.CookieBrowsers);
// "extension" (engine.CookiesFromExtension) comes before them.
const COOKIE_BROWSERS: [string, string][] = [
  ["firefox", "Firefox"],
  ["chrome", "Chrome"],
  ["edge", "Microsoft Edge"],
  ["brave", "Brave"],
  ["opera", "Opera"],
  ["vivaldi", "Vivaldi"],
  ["chromium", "Chromium"],
];

function Programs({ o, set }: TabProps) {
  const [again, setAgain] = useState(0);
  const [updating, setUpdating] = useState("");
  const ff = useToolStatus("ffmpeg", o.ffmpegPath, again);
  const yt = useToolStatus("yt-dlp", o.ytdlpPath, again);
  const tool = (id: string, label: string, value: string, onChange: (v: string) => void, status: string) => (
    <div className="space-y-1.5">
      <label className="text-sm" htmlFor={id}>
        {label}
      </label>
      <div className="flex gap-2">
        <Input id={id} value={value} placeholder={t("Found automatically")} onChange={(e) => onChange(e.target.value)} spellCheck={false} />
        <Button onClick={() => void api.pickFile(value).then((f) => f && onChange(f))}>
          <FolderOpen /> {t("Browse")}
        </Button>
      </div>
      <p className="break-all text-xs text-muted-foreground">{status}</p>
    </div>
  );
  return (
    <Section title={t("Companion programs")}>
      {tool("opt-ffmpeg", t("ffmpeg (joins the video and audio of some streams):"), o.ffmpegPath, (v) => set({ ffmpegPath: v }), toolText("ffmpeg", ff))}
      {tool("opt-ytdlp", t("yt-dlp (downloads videos from YouTube and similar sites):"), o.ytdlpPath, (v) => set({ ytdlpPath: v }), toolText("yt-dlp", yt))}
      <div className="space-y-1.5">
        <label className="text-sm" htmlFor="opt-cookies">
          {t("Sign in to video sites with the cookies of:")}
        </label>
        <Select id="opt-cookies" className="w-56" value={o.ytdlpCookies} onChange={(e) => set({ ytdlpCookies: e.target.value })}>
          <option value="extension">{t("The browser you download from (ODM's extension)")}</option>
          <option value="none">{t("No browser")}</option>
          {COOKIE_BROWSERS.map(([id, name]) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </Select>
        <p className="text-xs text-muted-foreground">
          {t("The site then sees you signed in: YouTube asks some visitors to sign in to confirm they are not a bot. “The browser you download from” works in every browser with ODM's extension, Chrome included: the extension sends the site's cookies along with the video. For a browser named here, yt-dlp reads its cookies itself, which works with Firefox; Chrome, Edge and other Chromium browsers lock their cookies while open, and recent Chrome encrypts them.")}
        </p>
      </div>
      <div className="space-y-2 border-t border-border pt-3">
        <Check label={t("Download and update yt-dlp and ffmpeg automatically")} checked={o.toolUpdates} onChange={(v) => set({ toolUpdates: v })} />
        <div className="flex items-center gap-3">
          <p className="min-w-0 flex-1 text-xs text-muted-foreground">{updating}</p>
          <Button
            size="sm"
            disabled={!o.canTools || updating === t("Downloading…")}
            onClick={() => {
              setUpdating(t("Downloading…"));
              api.updateTools().then(
                () => setUpdating(t("Up to date.")),
                (e) => setUpdating(String((e as Error)?.message ?? e)),
              ).finally(() => setAgain((n) => n + 1));
            }}
          >
            <RefreshCw /> {t("Update now")}
          </Button>
        </div>
      </div>
      <Hint>{t("ODM keeps its own copies in its folder, checked against the checksums of their releases, and uses them unless a path is chosen above.")}</Hint>
    </Section>
  );
}

function Connection({ o, set }: TabProps) {
  const hosts = o.hosts;
  return (
    <>
      <Section title={t("Connections and limits")}>
        <Field label={t("Default max. connections")} htmlFor="opt-conns">
          <Select id="opt-conns" className="w-24" value={o.connections} onChange={(e) => set({ connections: Number(e.target.value) })}>
            {Array.from({ length: 32 }, (_, i) => (
              <option key={i} value={i + 1}>
                {i + 1}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("Simultaneous downloads")} htmlFor="opt-conc">
          <Input id="opt-conc" className="w-24" inputMode="numeric" value={o.concurrent || ""} onChange={(e) => set({ concurrent: num(e.target.value) })} />
        </Field>
        <Field label={t("Global speed limit")} htmlFor="opt-limit">
          <div className="flex items-center gap-2.5">
            <Checkbox checked={o.limitOn} onCheckedChange={(v) => set({ limitOn: v === true })} aria-label={t("On")} />
            <Input id="opt-limit" className="w-24" inputMode="numeric" disabled={!o.limitOn} value={o.limitKB || ""} onChange={(e) => set({ limitKB: num(e.target.value) })} />
            <span className="text-sm text-muted-foreground">KB/s</span>
          </div>
        </Field>
      </Section>
      <Section title={t("Exceptions:")}>
        <div className="rounded-lg border border-border">
          <div className="grid grid-cols-[1fr_7rem_2.5rem] gap-2 border-b border-border bg-muted px-3 py-1.5 text-xs font-medium text-muted-foreground">
            <span>{t("Server")}</span>
            <span>{t("Number")}</span>
            <span />
          </div>
          {hosts.length === 0 && <p className="px-3 py-2 text-xs text-muted-foreground">—</p>}
          {hosts.map((h, i) => (
            <div key={i} className="grid grid-cols-[1fr_7rem_2.5rem] items-center gap-2 border-b border-border px-3 py-1.5 last:border-b-0">
              <Input
                value={h.host}
                placeholder="cdn.example.com"
                aria-label={t("Server")}
                onChange={(e) => set({ hosts: hosts.map((x, k) => (k === i ? { ...x, host: e.target.value } : x)) as typeof hosts })}
              />
              <Select value={h.connections} aria-label={t("Connections")} onChange={(e) => set({ hosts: hosts.map((x, k) => (k === i ? { ...x, connections: Number(e.target.value) } : x)) as typeof hosts })}>
                {Array.from({ length: 32 }, (_, n) => (
                  <option key={n} value={n + 1}>
                    {n + 1}
                  </option>
                ))}
              </Select>
              <Button size="icon" variant="ghost" aria-label={t("Delete")} onClick={() => set({ hosts: hosts.filter((_, k) => k !== i) })}>
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
        <Button size="sm" onClick={() => set({ hosts: [...hosts, { host: "", connections: 4 }] as typeof hosts })}>
          <Plus /> {t("New")}
        </Button>
      </Section>
      <Section title={t("Download limits")}>
        <Check label={t("Download limits")} checked={o.capOn} onChange={(v) => set({ capOn: v })} />
        <div className={cn("flex flex-wrap items-center gap-2 text-sm", !o.capOn && "opacity-50")}>
          {t("Download no more than")}
          <Input className="w-20" inputMode="numeric" disabled={!o.capOn} value={o.capMB || ""} onChange={(e) => set({ capMB: num(e.target.value) })} aria-label="MB" />
          {t("MBytes every")}
          <Input className="w-16" inputMode="numeric" disabled={!o.capOn} value={o.capHours || ""} onChange={(e) => set({ capHours: num(e.target.value) })} aria-label={t("hours")} />
          {t("hours")}
        </div>
        <Check label={t("Show a warning when downloads are stopped")} checked={o.capWarning} disabled={!o.capOn} onChange={(v) => set({ capWarning: v })} />
      </Section>
    </>
  );
}

function Proxy({ o, set }: TabProps) {
  const manual = o.proxyMode === "http" || o.proxyMode === "socks5";
  const mode = (m: string, label: string) => (
    <label className="flex items-center gap-2.5 text-sm">
      <input
        type="radio"
        name="proxy"
        className="size-4 accent-violet-600"
        checked={m === "manual" ? manual : o.proxyMode === m}
        onChange={() => set({ proxyMode: m === "manual" ? "http" : m })}
      />
      {label}
    </label>
  );
  return (
    <Section title={t("Proxy / socks configuration")}>
      <div className="space-y-2">
        {mode("none", t("No proxy"))}
        {mode("system", t("Use system settings"))}
        {mode("manual", t("Manual proxy configuration"))}
      </div>
      <div className={cn("space-y-2.5", !manual && "opacity-50")}>
        <Field label={t("Type")} htmlFor="opt-ptype">
          <Select
            id="opt-ptype"
            className="w-36"
            disabled={!manual}
            value={o.proxyMode === "socks5" ? "socks5" : "http"}
            onChange={(e) => {
              const type = e.target.value;
              // The usual port of the type, unless one was typed.
              const port = [0, 8080, 1080].includes(o.proxyPort) ? (type === "socks5" ? 1080 : 8080) : o.proxyPort;
              set({ proxyMode: type, proxyPort: port });
            }}
          >
            <option value="http">HTTP</option>
            <option value="socks5">SOCKS5</option>
          </Select>
        </Field>
        <Field label={t("Proxy server")} htmlFor="opt-phost">
          <Input id="opt-phost" disabled={!manual} value={o.proxyHost} onChange={(e) => set({ proxyHost: e.target.value })} spellCheck={false} />
        </Field>
        <Field label={t("Port")} htmlFor="opt-pport">
          <Input id="opt-pport" className="w-24" inputMode="numeric" disabled={!manual} value={o.proxyPort || ""} onChange={(e) => set({ proxyPort: num(e.target.value) })} />
        </Field>
        <Field label={t("User name")} htmlFor="opt-puser">
          <Input id="opt-puser" disabled={!manual} value={o.proxyUser} onChange={(e) => set({ proxyUser: e.target.value })} spellCheck={false} />
        </Field>
        <Field label={t("Password")} htmlFor="opt-ppass">
          <Input id="opt-ppass" type="password" disabled={!manual} value={o.proxyPass} onChange={(e) => set({ proxyPass: e.target.value })} />
        </Field>
      </div>
      <Hint>
        {t("System settings use HTTP(S)_PROXY, and on Windows the proxy entered in Internet Options. Automatic configuration scripts (PAC) are not evaluated.")}
      </Hint>
    </Section>
  );
}

function Logins({ o, set }: TabProps) {
  const logins = o.logins;
  const put = (i: number, patch: Partial<(typeof logins)[number]>) => set({ logins: logins.map((l, k) => (k === i ? { ...l, ...patch } : l)) as typeof logins });
  return (
    <Section title={t("User names and passwords for sites")}>
      <div className="rounded-lg border border-border">
        <div className="grid grid-cols-[1fr_9rem_8rem_2.5rem] gap-2 border-b border-border bg-muted px-3 py-1.5 text-xs font-medium text-muted-foreground">
          <span>{t("Site")}</span>
          <span>{t("User")}</span>
          <span>{t("Password")}</span>
          <span />
        </div>
        {logins.length === 0 && <p className="px-3 py-2 text-xs text-muted-foreground">—</p>}
        {logins.map((l, i) => (
          <div key={i} className="grid grid-cols-[1fr_9rem_8rem_2.5rem] items-center gap-2 border-b border-border px-3 py-1.5 last:border-b-0">
            <Input value={l.host} placeholder="files.example.com" aria-label={t("Site")} onChange={(e) => put(i, { host: e.target.value })} spellCheck={false} />
            <Input value={l.user} aria-label={t("User")} onChange={(e) => put(i, { user: e.target.value })} spellCheck={false} />
            <Input type="password" value={l.pass} aria-label={t("Password")} onChange={(e) => put(i, { pass: e.target.value })} />
            <Button size="icon" variant="ghost" aria-label={t("Remove")} onClick={() => set({ logins: logins.filter((_, k) => k !== i) })}>
              <Trash2 />
            </Button>
          </div>
        ))}
      </div>
      <Button size="sm" onClick={() => set({ logins: [...logins, { host: "", user: "", pass: "" }] as typeof logins })}>
        <Plus /> {t("New")}
      </Button>
    </Section>
  );
}
