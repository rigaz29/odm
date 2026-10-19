// What the page asks of ODM. Inside ODM (Wails serves the page from
// http://wails.localhost on Windows, wails://localhost on Linux and macOS)
// the calls go to the Go Service; in a plain browser
// (npm run dev, or the visual tests) a stand-in answers with made-up
// downloads, so the page can be worked on and measured without ODM.

import { Clipboard, Events, Window } from "@wailsio/runtime";
import {
  type About,
  type AddDefaults,
  type AddPrefill,
  Appearance,
  type BrowserStatus,
  type CategoryOption,
  type CreateRequest,
  type CreateResult,
  type Detail,
  type ExtensionStep,
  type GrabProgress,
  type GrabRequest,
  Item,
  type Limits,
  type Schedule,
  type SchedulerData,
  type SchedulerRefusal,
  type UpdateInfo,
  type OptionsForm,
  type ProbeResult,
  type SaveResult,
  type SetupBrowser,
  type SetupStatus,
  type SetupTool,
  type Snapshot,
  type ToolStatus,
  type TorrentProbe,
  type ZipListing,
} from "./bindings/odm/internal/gui/models";
import * as Service from "./bindings/odm/internal/gui/service";
import { applyChange, type Change } from "./snapshot";
import { mockApi } from "./mock";

export interface Api {
  snapshot(): Promise<Snapshot>;
  // onSnapshot calls cb with each new snapshot; it returns the unsubscribe.
  onSnapshot(cb: (s: Snapshot) => void): () => void;
  appearance(): Promise<Appearance>;
  // onAppearance calls cb when the Options change the look.
  onAppearance(cb: (a: Appearance) => void): () => void;
  resume(ids: string[]): Promise<void>;
  pause(ids: string[]): Promise<void>;
  pauseAll(): Promise<void>;
  resumeAll(): Promise<void>;
  stopAll(): Promise<void>;
  remove(ids: string[], withFiles: boolean): Promise<void>;
  moveToQueue(ids: string[], queue: string): Promise<void>;
  moveInQueue(id: string, delta: number): Promise<void>;
  setSpeedLimiter(on: boolean, bytesPerSecond: number): Promise<void>;
  clearCompleted(): Promise<number>;
  startQueue(name: string): Promise<void>;
  stopQueue(name: string): Promise<void>;
  openFile(id: string): Promise<void>;
  showInFolder(id: string): Promise<void>;
  copyText(text: string): Promise<void>;

  // The New Download form (add.go).
  addDefaults(): Promise<AddDefaults>;
  prefill(token: string): Promise<AddPrefill>;
  dropPrefill(token: string): Promise<void>;
  probe(address: string, token: string): Promise<ProbeResult>;
  create(req: CreateRequest): Promise<CreateResult>;
  zipPreview(address: string, token: string): Promise<ZipListing>;
  pickFolder(current: string): Promise<string>;
  // A torrent the form reads (torrent.go): what it holds now, and the end
  // of reading it.
  torrentProbe(address: string): Promise<TorrentProbe>;
  dropTorrentProbe(address: string): Promise<void>;
  openTorrentFile(): Promise<void>;
  stopSeeding(id: string): Promise<void>;
  torrentMagnet(id: string): Promise<string>;
  openAssociationSettings(): Promise<void>;

  // The progress and download complete windows (detail.go).
  detail(id: string): Promise<Detail>;
  setDownloadLimit(id: string, bytesPerSecond: number): Promise<void>;
  setOpenWhenDone(id: string, on: boolean): Promise<void>;
  setCompleteDialog(on: boolean): Promise<void>;
  openProgress(id: string): Promise<void>;
  readdress(id: string, address: string): Promise<void>;

  // The Options window (options.go).
  options(): Promise<OptionsForm>;
  saveOptions(o: OptionsForm): Promise<SaveResult>;
  defaultFileTypes(): Promise<string>;
  defaultCategories(): Promise<CategoryOption[]>;
  defaultUserAgent(): Promise<string>;
  browserStatus(): Promise<BrowserStatus>;
  registerBrowsers(): Promise<BrowserStatus>;
  unregisterBrowsers(): Promise<BrowserStatus>;
  toolStatus(tool: string, typed: string): Promise<ToolStatus>;
  updateTools(): Promise<void>;
  openOptions(tab: string): Promise<void>;
  pickFile(current: string): Promise<string>;

  // The Scheduler (scheduler.go) and the rest of the menus (extras.go).
  scheduler(): Promise<SchedulerData>;
  saveScheduler(schedules: Record<string, Schedule>, limits: Limits): Promise<SchedulerRefusal | null>;
  addQueue(name: string): Promise<void>;
  removeQueue(name: string): Promise<void>;
  openScheduler(newQueue: boolean): Promise<void>;
  exportList(): Promise<string>;
  importList(): Promise<number>;
  checkUpdate(): Promise<UpdateInfo>;
  // onUpdate calls cb with a newer release the start-up check found.
  onUpdate(cb: (u: UpdateInfo) => void): () => void;
  about(): Promise<About>;
  openURL(address: string): Promise<void>;
  grab(r: GrabRequest): Promise<GrabProgress>;
  onGrab(cb: (p: GrabProgress) => void): () => void;
  stopGrab(): Promise<void>;
  runFinish(action: string): Promise<void>;
  setToolUpdates(on: boolean): Promise<void>;

  // The Setup window (setup.go): yt-dlp, ffmpeg and the extension.
  setupStatus(): Promise<SetupStatus>;
  setSetupAtStart(on: boolean): Promise<void>;
  setupBrowsers(): Promise<SetupBrowser[]>;
  installTool(name: string): Promise<SetupTool>;
  // onSetupProgress calls cb as a tool downloads.
  onSetupProgress(cb: (p: SetupProgress) => void): () => void;
  installExtension(key: string): Promise<ExtensionStep>;
  showExtensionFolder(): Promise<void>;
  openSetup(): Promise<void>;

  // This window: its title and size, and closing it.
  setTitle(title: string): void;
  setSize(width: number, height: number): void;
  closeWindow(): void;
}

const SNAPSHOT_EVENT = "odm:snapshot";
const APPEARANCE_EVENT = "odm:appearance";
const UPDATE_EVENT = "odm:update";
const GRAB_EVENT = "odm:grab";
const SETUP_PROGRESS_EVENT = "odm:setup-progress";

// SetupProgress is how far a tool's download is (setup.go): bytes so far
// and the total, 0 when unknown.
export interface SetupProgress {
  tool: string;
  done: number;
  total: number;
}

// framed: the window has the system's frame (Linux, macOS), and the page
// draws no window buttons. Told by the address, known from the start:
// Wails' own word on the system (System.IsWindows) can come after the page
// has loaded. In a plain browser the page looks as on Windows.
export const framed = location.protocol === "wails:";

export const inODM = location.hostname === "wails.localhost" || framed;

// held is the snapshot this window has: the one it asked for, brought up to
// date with each Change ODM sends after it. Changes before it are dropped:
// it is newer than they are, and the next change mends what an older one
// arriving late undid.
let held: Snapshot | null = null;

const wailsApi: Api = {
  snapshot: () => Service.Snapshot().then((s) => (held = s)),
  onSnapshot: (cb) =>
    Events.On(SNAPSHOT_EVENT, (e) => {
      if (!held) return;
      const ch = e.data as Change;
      // As the bindings make them: Go's empty lists come as null.
      ch.items = (ch.items ?? []).map((d) => Item.createFrom(d));
      cb((held = applyChange(held, ch)));
    }),
  appearance: () => Service.Appearance(),
  onAppearance: (cb) => Events.On(APPEARANCE_EVENT, (e) => cb(Appearance.createFrom(e.data))),
  resume: (ids) => Service.Resume(ids),
  pause: (ids) => Service.Pause(ids),
  pauseAll: () => Service.PauseAll(),
  resumeAll: () => Service.ResumeAll(),
  stopAll: () => Service.StopAll(),
  remove: (ids, withFiles) => Service.Remove(ids, withFiles),
  moveToQueue: (ids, queue) => Service.MoveToQueue(ids, queue),
  moveInQueue: (id, delta) => Service.MoveInQueue(id, delta),
  setSpeedLimiter: (on, bps) => Service.SetSpeedLimiter(on, bps),
  clearCompleted: () => Service.ClearCompleted(),
  startQueue: (name) => Service.StartQueue(name),
  stopQueue: (name) => Service.StopQueue(name),
  openFile: (id) => Service.OpenFile(id),
  showInFolder: (id) => Service.ShowInFolder(id),
  copyText: (text) => Clipboard.SetText(text),
  addDefaults: () => Service.AddDefaults(),
  prefill: (token) => Service.Prefill(token),
  dropPrefill: (token) => Service.DropPrefill(token),
  probe: (address, token) => Service.Probe(address, token),
  create: (req) => Service.Create(req),
  zipPreview: (address, token) => Service.ZipPreview(address, token),
  pickFolder: (current) => Service.PickFolder(current),
  torrentProbe: (address) => Service.TorrentProbe(address),
  dropTorrentProbe: (address) => Service.DropTorrentProbe(address),
  openTorrentFile: () => Service.OpenTorrentFile(),
  stopSeeding: (id) => Service.StopSeeding(id),
  torrentMagnet: (id) => Service.TorrentMagnet(id),
  openAssociationSettings: () => Service.OpenAssociationSettings(),
  detail: (id) => Service.Detail(id),
  setDownloadLimit: (id, bps) => Service.SetDownloadLimit(id, bps),
  setOpenWhenDone: (id, on) => Service.SetOpenWhenDone(id, on),
  setCompleteDialog: (on) => Service.SetCompleteDialog(on),
  openProgress: (id) => Service.OpenProgress(id),
  readdress: (id, address) => Service.Readdress(id, address),
  options: () => Service.Options(),
  saveOptions: (o) => Service.SaveOptions(o),
  defaultFileTypes: () => Service.DefaultFileTypes(),
  defaultCategories: () => Service.DefaultCategories(),
  defaultUserAgent: () => Service.DefaultUserAgent(),
  browserStatus: () => Service.BrowserStatus(),
  registerBrowsers: () => Service.RegisterBrowsers(),
  unregisterBrowsers: () => Service.UnregisterBrowsers(),
  toolStatus: (tool, typed) => Service.ToolStatus(tool, typed),
  updateTools: () => Service.UpdateTools(),
  openOptions: (tab) => Service.OpenOptions(tab),
  pickFile: (current) => Service.PickFile(current),
  scheduler: () => Service.Scheduler(),
  saveScheduler: (schedules, limits) => Service.SaveScheduler(schedules, limits),
  addQueue: (name) => Service.AddQueue(name),
  removeQueue: (name) => Service.RemoveQueue(name),
  openScheduler: (newQueue) => Service.OpenScheduler(newQueue),
  exportList: () => Service.ExportList(),
  importList: () => Service.ImportList(),
  checkUpdate: () => Service.CheckUpdate(),
  onUpdate: (cb) => Events.On(UPDATE_EVENT, (e) => cb(e.data as UpdateInfo)),
  about: () => Service.About(),
  openURL: (address) => Service.OpenURL(address),
  grab: (r) => Service.Grab(r),
  onGrab: (cb) => Events.On(GRAB_EVENT, (e) => cb(e.data as GrabProgress)),
  stopGrab: () => Service.StopGrab(),
  runFinish: (action) => Service.RunFinish(action),
  setToolUpdates: (on) => Service.SetToolUpdates(on),
  setupStatus: () => Service.SetupStatus(),
  setSetupAtStart: (on) => Service.SetSetupAtStart(on),
  setupBrowsers: () => Service.SetupBrowsers(),
  installTool: (name) => Service.InstallTool(name),
  onSetupProgress: (cb) => Events.On(SETUP_PROGRESS_EVENT, (e) => cb(e.data as SetupProgress)),
  installExtension: (key) => Service.InstallExtension(key),
  showExtensionFolder: () => Service.ShowExtensionFolder(),
  openSetup: () => Service.OpenSetup(),
  setTitle: (title) => {
    document.title = title;
    void Window.SetTitle(title);
  },
  setSize: (w, h) => void Window.SetSize(w, h),
  closeWindow: () => void Window.Close(),
};

export const api: Api = inODM ? wailsApi : mockApi();
