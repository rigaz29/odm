// What the page asks of ODM. Inside ODM (Wails serves the page from
// wails.localhost) the calls go to the Go Service; in a plain browser
// (npm run dev, or the visual tests) a stand-in answers with made-up
// downloads, so the page can be worked on and measured without ODM.

import { Events } from "@wailsio/runtime";
import * as Service from "./bindings/odm/internal/fluent/service";
import { Appearance, Item, Snapshot } from "./bindings/odm/internal/fluent/models";
import { mockApi } from "./mock";

export interface Api {
  snapshot(): Promise<Snapshot>;
  // onSnapshot calls cb with each new snapshot; it returns the unsubscribe.
  onSnapshot(cb: (s: Snapshot) => void): () => void;
  appearance(): Promise<Appearance>;
  resume(ids: string[]): Promise<void>;
  pause(ids: string[]): Promise<void>;
  pauseAll(): Promise<void>;
  remove(ids: string[]): Promise<void>;
  clearCompleted(): Promise<number>;
  startQueue(name: string): Promise<void>;
  stopQueue(name: string): Promise<void>;
  addURL(address: string): Promise<Item>;
  openFile(id: string): Promise<void>;
  showInFolder(id: string): Promise<void>;
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
  remove: (ids) => Service.Remove(ids),
  clearCompleted: () => Service.ClearCompleted(),
  startQueue: (name) => Service.StartQueue(name),
  stopQueue: (name) => Service.StopQueue(name),
  addURL: (address) => Service.AddURL(address),
  openFile: (id) => Service.OpenFile(id),
  showInFolder: (id) => Service.ShowInFolder(id),
};

export const api: Api = inODM ? wailsApi : mockApi();
