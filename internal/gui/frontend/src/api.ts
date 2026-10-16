// What the page asks of ODM. Inside ODM (Wails serves the page from
// wails.localhost) the calls go to the Go Service; in a plain browser
// (npm run dev, or the visual tests) a stand-in answers with made-up
// downloads, so the page can be worked on and measured without ODM.

import { Clipboard, Events, Window } from "@wailsio/runtime";
import {
  type AddDefaults,
  type AddPrefill,
  Appearance,
  type CreateRequest,
  type CreateResult,
  type Detail,
  type ProbeResult,
  Snapshot,
  type ZipListing,
} from "./bindings/odm/internal/gui/models";
import * as Service from "./bindings/odm/internal/gui/service";
import { mockApi } from "./mock";

export interface Api {
  snapshot(): Promise<Snapshot>;
  // onSnapshot calls cb with each new snapshot; it returns the unsubscribe.
  onSnapshot(cb: (s: Snapshot) => void): () => void;
  appearance(): Promise<Appearance>;
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

  // The progress and download complete windows (detail.go).
  detail(id: string): Promise<Detail>;
  setDownloadLimit(id: string, bytesPerSecond: number): Promise<void>;
  setOpenWhenDone(id: string, on: boolean): Promise<void>;
  setCompleteDialog(on: boolean): Promise<void>;
  openProgress(id: string): Promise<void>;
  readdress(id: string, address: string): Promise<void>;

  // This window: its title and size, and closing it.
  setTitle(title: string): void;
  setSize(width: number, height: number): void;
  closeWindow(): void;
}

const SNAPSHOT_EVENT = "odm:snapshot";

export const inODM = location.hostname === "wails.localhost";

const wailsApi: Api = {
  snapshot: () => Service.Snapshot(),
  onSnapshot: (cb) => Events.On(SNAPSHOT_EVENT, (e) => cb(Snapshot.createFrom(e.data))),
  appearance: () => Service.Appearance(),
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
  detail: (id) => Service.Detail(id),
  setDownloadLimit: (id, bps) => Service.SetDownloadLimit(id, bps),
  setOpenWhenDone: (id, on) => Service.SetOpenWhenDone(id, on),
  setCompleteDialog: (on) => Service.SetCompleteDialog(on),
  openProgress: (id) => Service.OpenProgress(id),
  readdress: (id, address) => Service.Readdress(id, address),
  setTitle: (title) => {
    document.title = title;
    void Window.SetTitle(title);
  },
  setSize: (w, h) => void Window.SetSize(w, h),
  closeWindow: () => void Window.Close(),
};

export const api: Api = inODM ? wailsApi : mockApi();
