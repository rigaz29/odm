// A stand-in for ODM in a plain browser: made-up downloads in every state,
// some of them running, whose progress, connections and speeds move every
// half second as ODM's snapshots do. ?n=1000 makes a thousand; ?lang=en and
// ?theme=dark|light set the look. The New Download form checks made-up
// addresses: a YouTube page, an .m3u8 playlist, a .zip, "missing", or any
// other file; ?view=add&token=demo opens it prefilled as the browser would.

import type { Api, SetupProgress } from "./api";
import {
  About,
  AddDefaults,
  AddPrefill,
  Appearance,
  BrowserStatus,
  Card,
  CategoryOption,
  Chunk,
  CreateResult,
  Detail,
  ExtensionStep,
  GrabProgress,
  Limits,
  QueueState,
  Schedule,
  SchedulerData,
  SchedulerRefusal,
  UpdateInfo,
  HostConnections,
  Item,
  OptionsError,
  OptionsForm,
  ProbeResult,
  Quality,
  SaveResult,
  SetupBrowser,
  SetupStatus,
  SetupTool,
  SiteLogin,
  Snapshot,
  ToolStatus,
  ZipEntry,
  ZipListing,
  TorrentDetail,
  TorrentFileDetail,
  TorrentFileRow,
  TorrentProbe,
} from "./bindings/odm/internal/gui/models";

const categories = ["Video", "Audio", "Documents", "Programs", "Compressed", "Images", "Disk Images", "Other"];
// With ?lang=en the made-up names are English too (the README's screenshot).
const english = new URLSearchParams(location.search).get("lang") === "en";
const say = (id: string, en: string) => (english ? en : id);
const night = say("Malam hari", "Night");
const GB = 1024 ** 3;
const MB = 1024 ** 2;

type Seed = [name: string, category: string, size: number, status: string, progress: number, speed: number, host: string, extra?: Partial<Item>];
const seeds: Seed[] = [
  ["ubuntu-24.04.1-desktop-amd64.iso", "Disk Images", 5.8 * GB, "downloading", 63, 12.4 * MB, "releases.ubuntu.com"],
  ["Blender-4.5-windows-x64.msi", "Programs", 412 * MB, "downloading", 27, 6.1 * MB, "download.blender.org"],
  [say("Siaran NASA — ISS Earth View (live)", "NASA — ISS Earth View (live)"), "Video", 0, "downloading", 0, 0.9 * MB, "youtube.com", { live: true, stream: "hls", done: 248 * MB }],
  ["Interstellar.2014.2160p.UHD.mkv", "Video", 21.4 * GB, "paused", 41, 0, "cdn.example.net"],
  ["Figma-Desktop-Setup-x64.exe", "Programs", 98 * MB, "queued", 0, 0, "desktop.figma.com", { queue: night }],
  [say("foto-liburan-bali-2026.zip", "bali-holiday-photos-2026.zip"), "Compressed", 1.3 * GB, "error", 32, 0, "drive.example.com", { error: "403 Forbidden" }],
  [say("dataset-iklim-indonesia-2010-2026.csv", "climate-dataset-2010-2026.csv"), "Documents", 640 * MB, "queued", 0, 0, "data.bmkg.go.id", { queue: night, description: say("Data untuk skripsi", "Data for the thesis") }],
  [say("podcast-teknologi-eps-112.mp3", "tech-podcast-ep-112.mp3"), "Audio", 86 * MB, "complete", 100, 0, "anchor.fm"],
  [say("laporan-keuangan-Q3-2026.pdf", "financial-report-Q3-2026.pdf"), "Documents", 3.2 * MB, "complete", 100, 0, "intranet.example.co.id"],
  [say("Kuliah-Umum-AI-2026.mp4", "AI-public-lecture-2026.mp4"), "Video", 1.8 * GB, "complete", 100, 0, "vod.kampus.ac.id"],
  ["node-v24.14.1-x64.msi", "Programs", 31 * MB, "complete", 100, 0, "nodejs.org"],
  ["wallpaper-aurora-8k.png", "Images", 24 * MB, "complete", 100, 0, "unsplash.com"],
  ["android-studio-2026.1-windows.exe", "Programs", 1.4 * GB, "complete", 100, 0, "dl.google.com"],
  ["album-lofi-chill-vol3.zip", "Compressed", 512 * MB, "complete", 100, 0, "bandcamp.com"],
  [
    "debian-13.7.0-amd64-netinst.iso", "Disk Images", 754 * MB, "complete", 100, 0, "cdimage.debian.org",
    { url: "magnet:?xt=urn:btih:6d4795dee70aeb88e03e5336ca7c9fcf0a1e206d&dn=debian-13.7.0-amd64-netinst.iso", torrent: true, seeding: true, uploaded: 412 * MB, uploadSpeed: 1.6 * MB, peers: 6, seeds: 1 },
  ],
  [
    "Nusantara.Documentary.S01.1080p", "Video", 4.2 * GB, "downloading", 38, 5.4 * MB, "torrent",
    { url: "magnet:?xt=urn:btih:0f3c6a1d9b2e4f5a6b7c8d9e0f1a2b3c4d5e6f70&dn=Nusantara.Documentary.S01.1080p", torrent: true, uploaded: 96 * MB, uploadSpeed: 210 * 1024, peers: 41, seeds: 33 },
  ],
];

// btih is a magnet link's v1 info hash in lowercase, "" for none.
const btih = (url: string) => /[?&]xt=urn:btih:([0-9a-f]{40})/i.exec(url)?.[1].toLowerCase() ?? "";

// The files of a torrent the progress window lists: a season of episodes for
// the documentary, else one file.
function torrentFilesOf(d: Item): TorrentFileDetail[] {
  if (!d.fileName.startsWith("Nusantara")) return [new TorrentFileDetail({ index: 0, name: d.fileName, size: d.total, done: d.total, priority: 4 })];
  const ep = (n: number, done: number) => ({ name: `${say("Episode", "Episode")} 0${n}.mkv`, size: 1.3 * GB, done: done * 1.3 * GB });
  return [ep(1, 1), ep(2, 0.42), ep(3, 0), { name: "Subs/indonesian.srt", size: 64 * 1024, done: 64 * 1024 }, { name: "Subs/english.srt", size: 61 * 1024, done: 0 }, { name: "sample.mkv", size: 280 * MB, done: 0 }].map(
    (f, i) => new TorrentFileDetail({ ...f, index: i + (i > 2 ? 1 : 0), priority: i === 5 ? 0 : i === 1 ? 7 : 4, skip: i === 5 }),
  );
}

// A torrent the New Download form reads: a magnet link shows its files once
// its made-up peers answered, a few seconds after it was first asked.
const torrentAsked = new Map<string, number>();
function torrentLook(url: string): TorrentProbe {
  const dn = /[?&]dn=([^&]+)/.exec(url)?.[1];
  const name = dn ? decodeURIComponent(dn.replace(/\+/g, " ")) : decodeURIComponent(url.split("/").pop() || "").replace(/\.torrent$/i, "");
  if (!torrentAsked.has(url)) torrentAsked.set(url, Date.now());
  if (/^magnet:/i.test(url) && Date.now() - (torrentAsked.get(url) ?? 0) < 2500) {
    return new TorrentProbe({ name, waiting: true, peers: 3, firstTorrent: true });
  }
  const files = [
    new TorrentFileRow({ path: `${name}/${say("Episode", "Episode")} 01.mkv`, size: 1.4 * GB }),
    new TorrentFileRow({ path: `${name}/${say("Episode", "Episode")} 02.mkv`, size: 1.3 * GB }),
    new TorrentFileRow({ path: `${name}/Subs/indonesian.srt`, size: 61 * 1024 }),
    new TorrentFileRow({ path: `${name}/Subs/english.srt`, size: 58 * 1024 }),
    new TorrentFileRow({ path: `${name}/README.txt`, size: 2 * 1024 }),
  ];
  return new TorrentProbe({ name, size: files.reduce((n, f) => n + f.size, 0), files, firstTorrent: true });
}

function makeItems(n: number): Item[] {
  const now = Date.now();
  const out: Item[] = [];
  for (let i = 0; i < n; i++) {
    const [base, category, size, status0, progress0, speed0, host, extra] = seeds[i % seeds.length];
    const copy = Math.floor(i / seeds.length);
    const name = copy === 0 ? base : base.replace(/(\.[^.]+)?$/, (ext) => ` (${copy + 1})${ext}`);
    const running = copy === 0 && status0 === "downloading";
    const status = running ? "downloading" : status0 === "downloading" ? "paused" : status0;
    const progress = progress0;
    out.push(
      new Item({
        id: `d${i}`,
        url: `https://${host}/files/${encodeURIComponent(name)}`,
        fileName: name,
        filePath: `C:\\Users\\me\\Downloads\\${name}`,
        status,
        total: size,
        done: Math.round((size * progress) / 100),
        progress,
        speed: running ? speed0 : 0,
        category,
        queue: "Main",
        order: i,
        createdAt: new Date(now - i * 3.3e6).toISOString(),
        updatedAt: new Date(now - i * 3.1e6).toISOString(),
        conns: running && !extra?.live ? Array.from({ length: i === 0 ? 8 : 4 }, (_, k) => Math.max(0, 1 - k * 0.13 - (i ? 0.3 : 0))) : [],
        speeds: running ? wave(i, speed0) : [],
        activeSince: running ? new Date(now - (i === 2 ? 2538e3 : 600e3)).toISOString() : "0001-01-01T00:00:00Z",
        ...(copy === 0 ? extra : { live: false }),
      }),
    );
  }
  return out;
}

function wave(seed: number, around: number, n = 60): number[] {
  return Array.from({ length: n }, (_, k) => Math.max(0, around * (0.75 + 0.25 * Math.sin(k * 0.45 + seed) + 0.1 * Math.cos(k * 1.7 + seed * 2))));
}

export function mockApi(): Api {
  const q = new URLSearchParams(location.search);
  const n = Number(q.get("n") || seeds.length);
  let items = makeItems(n);
  let running: string[] = [];
  const queues = ["Main", night];
  const limiter = { on: false, saved: 0 };
  let total = wave(9, 19 * MB);
  const limits = new Map<string, number>();
  const torrentFiles = new Map<string, TorrentFileDetail[]>(); // as setTorrentFiles left them
  const seedRules = new Map<string, { mode: string; ratio: number; minutes: number }>(); // as setTorrentSeed left them
  const newSchedule = () => ({
    sync: false, onStartup: false, startEnabled: false, startAt: "23:00", once: false, onceDate: "2026-10-17", days: [true, true, true, true, true, true, true],
    repeatEnabled: false, repeatMinutes: 60, stopEnabled: false, stopAt: "07:30", retriesEnabled: false, retries: 10,
    openEnabled: false, openPath: "", finish: "", parallel: 1,
  });
  const schedules: Record<string, Schedule> = {
    [night]: new Schedule({ ...newSchedule(), startEnabled: true, startAt: "23:30", stopEnabled: true, finish: "shutdown", parallel: 2 }),
  };
  let limitsState = new Limits({ on: false, mb: 200, hours: 5, warning: true });
  const grabSubs = new Set<(p: GrabProgress) => void>();
  // The Setup window: ffmpeg missing, Chrome's extension talking to ODM.
  const setupSubs = new Set<(p: SetupProgress) => void>();
  const setup = {
    toolMode: "",
    tools: [
      { name: "yt-dlp", state: "found", path: String.raw`C:\Users\me\AppData\Roaming\Python\Python310\Scripts\yt-dlp.exe`, version: "2026.08.19", canInstall: true },
      { name: "ffmpeg", state: "missing", path: "", version: "", canInstall: true },
      { name: "odm-libtorrent", state: "missing", path: "", version: "", canInstall: true },
    ],
    atStart: true,
    browsers: [
      { key: "chrome", name: "Google Chrome", family: "chromium", connected: true },
      { key: "edge", name: "Microsoft Edge", family: "chromium", connected: false },
      { key: "firefox", name: "Mozilla Firefox", family: "firefox", connected: false },
    ],
  };
  const defaultCategories = [
    { name: "Video", exts: "3gp avi flv m4v mkv mov mp4 mpeg mpg webm wmv", folder: "" },
    { name: "Audio", exts: "aac flac m4a mp3 ogg opus wav wma", folder: "" },
    { name: "Documents", exts: "csv doc docx epub odt pdf ppt pptx rtf txt xls xlsx", folder: "" },
    { name: "Programs", exts: "apk appx bat deb dmg exe msi pkg rpm sh", folder: "" },
    { name: "Compressed", exts: "7z bz2 gz rar tar xz zip zst", folder: "" },
    { name: "Images", exts: "bmp gif heic ico jpeg jpg png svg tiff webp", folder: "" },
    { name: "Disk Images", exts: "img iso vhd", folder: "" },
    { name: "Other", exts: "", folder: "" },
  ];
  let options = new OptionsForm({
    startup: true, closeToTray: true, clipboard: false, updateCheck: true, excludeSites: "*.update.microsoft.com",
    theme: q.get("theme") || "system", language: "system", fileTypes: "3GP 7Z AAC AVI EXE ISO MKV MP3 MP4 PDF RAR ZIP",
    dir: String.raw`C:\Users\me\Downloads`, categoryFolders: true, rememberDir: true, categories: defaultCategories.map((c) => new CategoryOption(c)),
    startDialog: true, progressDialog: true, completeDialog: true, notifications: true, sounds: false, duplicateAgain: false, userAgent: "",
    ffmpegPath: "", ytdlpPath: "", ytdlpCookies: "extension", toolUpdates: true, connections: 8, concurrent: 3, limitOn: false, limitKB: 512,
    hosts: [new HostConnections({ host: "cdn.example.com", connections: 2 })], capOn: false, capMB: 200, capHours: 5, capWarning: true,
    proxyMode: "system", proxyHost: "", proxyPort: 8080, proxyUser: "", proxyPass: "",
    seed: "", seedRatio: 100, seedMinutes: 30, torrentPort: 51413, incoming: true, portForwarding: true, uploadLimitKB: 0, torrentLinks: true,
    associate: true, canAssociate: true,
    logins: [new SiteLogin({ host: "files.example.com", user: "rigaz", pass: "rahasia" })], canSound: true, canTools: true,
  });
  const openWhenDone = new Set<string>();
  let completeDialog = true;
  const base = new Map<string, number>(); // each download's speed before the jitter
  const subs = new Set<(s: Snapshot) => void>();
  const snap = () =>
    new Snapshot({
      items: items.map((d) => new Item(d)),
      categories,
      queues,
      running,
      limit: limiter.on ? limiter.saved : 0,
      savedLimit: limiter.saved,
      speeds: total.slice(),
    });
  const push = () => subs.forEach((cb) => cb(snap()));

  setInterval(() => {
    let sum = 0;
    for (const d of items) {
      if (d.status !== "downloading") continue;
      if (!base.has(d.id)) base.set(d.id, d.speed);
      const v = base.get(d.id)! * (0.85 + Math.random() * 0.3);
      d.speed = v;
      d.speeds = [...d.speeds.slice(-59), v];
      sum += v;
      if (d.live) {
        d.done += v / 2;
        continue;
      }
      d.done = Math.min(d.total, d.done + v / 2);
      d.progress = (d.done / d.total) * 100;
      d.conns = d.conns.map((c) => Math.min(1, c + 0.004));
      if (d.done >= d.total) {
        d.status = "complete";
        d.speed = 0;
      }
    }
    total = [...total.slice(-59), sum];
    push();
  }, 500);

  const set = (ids: string[], status: string, speed: number) => {
    for (const d of items) {
      if (ids.includes(d.id) && d.status !== "complete") {
        d.status = status;
        d.speed = speed;
        if (speed > 0) d.activeSince = new Date().toISOString();
      }
    }
    push();
  };

  return {
    snapshot: async () => snap(),
    onSnapshot: (cb) => {
      subs.add(cb);
      return () => subs.delete(cb);
    },
    appearance: async () => new Appearance({ language: q.get("lang") || "id", theme: q.get("theme") || "system" }),
    resume: async (ids) => set(ids, "downloading", 3 * MB),
    pause: async (ids) => set(ids, "paused", 0),
    pauseAll: async () => set(items.filter((d) => d.status === "downloading").map((d) => d.id), "paused", 0),
    resumeAll: async () => set(items.filter((d) => d.status === "paused" || d.status === "error").map((d) => d.id), "downloading", 3 * MB),
    stopAll: async () => {
      running = [];
      set(items.filter((d) => d.status === "downloading" || d.status === "queued").map((d) => d.id), "paused", 0);
    },
    moveToQueue: async (ids, queue) => {
      for (const d of items) if (ids.includes(d.id)) d.queue = queue;
      push();
    },
    moveInQueue: async (id, delta) => {
      const d = items.find((x) => x.id === id);
      if (!d) return;
      const sib = items.filter((x) => x.queue === d.queue).sort((a, b) => a.order - b.order);
      const other = sib[sib.indexOf(d) + delta];
      if (other) [d.order, other.order] = [other.order, d.order];
      push();
    },
    setSpeedLimiter: async (on, bps) => {
      limiter.on = on;
      if (bps > 0) limiter.saved = bps;
      push();
    },
    copyText: (text) => navigator.clipboard.writeText(text),
    remove: async (ids) => {
      items = items.filter((d) => !ids.includes(d.id));
      push();
    },
    clearCompleted: async () => {
      const before = items.length;
      items = items.filter((d) => d.status !== "complete");
      push();
      return before - items.length;
    },
    startQueue: async (name) => {
      running = [...running, name];
      push();
    },
    stopQueue: async (name) => {
      running = running.filter((r) => r !== name);
      push();
    },
    openFile: async () => {},
    showInFolder: async () => {},

    addDefaults: async () => new AddDefaults({ folder: String.raw`C:\Users\me\Downloads`, connections: 8, categories, queues, queue: "Main" }),
    prefill: async (token) =>
      new AddPrefill(
        token === "demo"
          ? { url: "https://releases.ubuntu.com/24.04/ubuntu-24.04.1-desktop-amd64.iso", fileName: "", description: "Dari Chrome" }
          : token === "video"
            ? { url: "https://www.youtube.com/watch?v=ISS-live", site: true }
            : {},
      ),
    dropPrefill: async () => {},
    probe: async (address) => {
      await new Promise((r) => setTimeout(r, 350));
      const url = address.trim();
      const name = decodeURIComponent(url.split("?")[0].split("/").pop() || "");
      if (!url) return new ProbeResult({ url, card: new Card({ state: "idle", title: "No address yet", detail: "Paste or type one: ODM checks the file's size and type before adding it." }) });
      if (/^magnet:\?/i.test(url) || /\.torrent$/i.test(url)) {
        const twin = btih(url) && items.find((d) => d.torrent && btih(d.url) === btih(url));
        if (twin && (twin.status !== "complete" || twin.seeding)) {
          return new ProbeResult({ url, kind: "torrent", card: new Card({ state: "twin", name: twin.fileName }) });
        }
        if (/needs-libtorrent/.test(url)) {
          return new ProbeResult({ url, kind: "torrent", card: new Card({ state: "warn", title: "Torrents need odm-libtorrent", detail: "ODM downloads it once from its releases." }), torrent: new TorrentProbe({ needsProgram: true }) });
        }
        const tp = torrentLook(url);
        return new ProbeResult({ url, kind: "torrent", fileName: tp.name, category: tp.waiting ? "" : "Video", card: new Card({ state: "torrent", name: tp.name, size: tp.size }), torrent: tp });
      }
      if (/youtube\.com|youtu\.be/.test(url)) {
        return new ProbeResult({
          url, kind: "site", video: true, fileName: "Siaran NASA — ISS Earth View",
          card: new Card({ state: "site", name: "Siaran NASA — ISS Earth View", site: "YouTube" }),
          qualities: [
            new Quality({ label: "1080p60 · MP4", estimate: 412 * MB, ext: "mp4" }),
            new Quality({ label: "720p · MP4", estimate: 198 * MB, ext: "mp4" }),
            new Quality({ label: "Audio · M4A", estimate: 18 * MB, ext: "m4a" }),
          ],
        });
      }
      if (/\.m3u8/.test(url)) {
        return new ProbeResult({
          url, kind: "stream", video: true,
          card: new Card({ state: "stream", format: "HLS", note: "" }),
          qualities: [
            new Quality({ label: "1920×1080 · 5,0 Mbit/s", estimate: 1.2 * GB, audios: ["English", "Deutsch"] }),
            new Quality({ label: "1280×720 · 0,8 Mbit/s", estimate: 210 * MB, audios: ["English", "Deutsch"] }),
          ],
        });
      }
      if (/missing/.test(url)) return new ProbeResult({ url, card: new Card({ state: "problem", title: "The file could not be checked", error: "404 Not Found" }) });
      const zip = /\.zip$/i.test(name);
      return new ProbeResult({
        url, kind: "file", fileName: name, category: zip ? "Compressed" : /\.iso$/i.test(name) ? "Disk Images" : "Other", zip,
        card: new Card({ state: "file", name, size: zip ? 512 * MB : 5.8 * GB, contentType: zip ? "application/zip" : "application/x-iso9660-image", host: new URL(url).hostname, resumable: true }),
      });
    },
    create: async (req) => {
      const twin = btih(req.url) && items.find((d) => d.torrent && btih(d.url) === btih(req.url));
      if (twin) {
        const noAgain = twin.status !== "complete" || twin.seeding;
        if (noAgain || !req.again) return new CreateResult({ id: "", duplicate: new Item(twin), sameTorrent: true, noAgain });
      }
      const dup = items.find((d) => d.url === req.url.trim());
      if (dup && !req.again) return new CreateResult({ id: "", duplicate: new Item(dup) });
      const d = makeItems(1)[0];
      Object.assign(d, {
        id: `n${Date.now()}`, url: req.url.trim(), fileName: req.fileName, category: req.category, queue: req.queue, description: req.description,
        status: req.start ? "downloading" : "paused", done: 0, progress: 0, speed: req.start ? 4 * MB : 0, createdAt: new Date().toISOString(),
      });
      items = [...items, d];
      push();
      return new CreateResult({ id: d.id });
    },
    zipPreview: async () => {
      await new Promise((r) => setTimeout(r, 400));
      const at = new Date(2026, 8, 3, 14, 20).toISOString();
      return new ZipListing({
        files: 4, size: 1.4 * GB, fetched: 38 * 1024,
        entries: [
          new ZipEntry({ name: "lofi-vol3/", dir: true, modified: at }),
          new ZipEntry({ name: "lofi-vol3/01 - Hujan Sore.flac", size: 41 * MB, modified: at }),
          new ZipEntry({ name: "lofi-vol3/02 - Kopi Pagi.flac", size: 38 * MB, modified: at }),
          new ZipEntry({ name: "lofi-vol3/cover.jpg", size: 2.1 * MB, modified: at }),
          new ZipEntry({ name: "lofi-vol3/bonus.zip", size: 1.3 * GB, modified: at, encrypted: true }),
        ],
      });
    },
    pickFolder: async (current) => String.raw`${current}\Video`,
    torrentProbe: async (address) => torrentLook(address.trim()),
    dropTorrentProbe: async () => {},
    openTorrentFile: async () => {},
    stopSeeding: async (id) => {
      const d = items.find((x) => x.id === id);
      if (d) Object.assign(d, { seeding: false, uploadSpeed: 0, peers: 0, seeds: 0 });
      push();
    },
    torrentMagnet: async () => "magnet:?xt=urn:btih:0000",
    setTorrentSeed: async (id, mode, ratio, minutes) => void seedRules.set(id, { mode, ratio, minutes }),
    setTorrentFiles: async (id, change) => {
      const d = items.find((x) => x.id === id);
      if (!d) throw new Error("download not found");
      const files = torrentFiles.get(id) ?? torrentFilesOf(d);
      const next = files.map((f) => new TorrentFileDetail({ ...f, priority: change[f.index] ?? f.priority, skip: (change[f.index] ?? f.priority) === 0 }));
      if (!next.some((f) => f.priority > 0)) throw new Error("choose at least one file to download");
      torrentFiles.set(id, next);
    },
    openAssociationSettings: async () => {},

    detail: async (id) => {
      const d = items.find((x) => x.id === id) ?? items[0];
      const n = d.conns.length || (d.status === "complete" ? 8 : 4);
      const part = d.total / n;
      const chunks = d.live || d.stream ? [] : Array.from({ length: n }, (_, k) => {
        const p = d.status === "complete" ? 1 : (d.conns[k] ?? d.progress / 100);
        return new Chunk({ start: Math.round(k * part), end: Math.round((k + 1) * part) - 1, done: Math.round(p * part), state: d.status === "downloading" ? (p >= 1 ? "Complete" : "Receiving data…") : "" });
      });
      return new Detail({
        item: new Item(d), path: d.filePath, checksum: "", checksumName: "", resumable: true, speedLimit: limits.get(id) ?? 0,
        openWhenDone: openWhenDone.has(id), segments: d.live ? 0 : 0, segmentsDone: d.live ? 1270 : 0, chunks, completeDialog,
        torrent: d.torrent
          ? new TorrentDetail({
              seeding: d.seeding, peers: d.peers, seeds: d.seeds, uploadSpeed: d.uploadSpeed, uploaded: d.uploaded, ratio: d.uploaded / d.total,
              pieces: btoa(String.fromCharCode(...new Array(Math.ceil(377 / 8)).fill(255))), pieceCount: 377, magnet: d.url,
              files: torrentFiles.get(d.id) ?? torrentFilesOf(d),
              seed: seedRules.get(d.id)?.mode ?? "options",
              seedRatio: seedRules.get(d.id)?.ratio || 100,
              seedMinutes: seedRules.get(d.id)?.minutes || 30,
            })
          : null,
      });
    },
    setDownloadLimit: async (id, bps) => void limits.set(id, Math.max(0, bps)),
    setOpenWhenDone: async (id, on) => void (on ? openWhenDone.add(id) : openWhenDone.delete(id)),
    setCompleteDialog: async (on) => void (completeDialog = on),
    openProgress: async (id) => void window.open(`?view=progress&id=${id}`, "_blank", "width=580,height=620"),
    readdress: async (id, address) => {
      const d = items.find((x) => x.id === id);
      if (d) d.url = address;
      push();
    },
    onAppearance: () => () => {},
    options: async () => new OptionsForm({ ...options, categories: options.categories.map((c) => new CategoryOption(c)) }),
    saveOptions: async (o) => {
      if ((o.proxyMode === "http" || o.proxyMode === "socks5") && !o.proxyHost.trim()) return new SaveResult({ refused: new OptionsError({ tab: "proxy", field: "proxyHost" }) });
      options = o;
      return new SaveResult({});
    },
    defaultFileTypes: async () => "3GP 7Z AAC AVI EXE ISO MKV MP3 MP4 PDF RAR ZIP",
    defaultCategories: async () => defaultCategories.map((c) => new CategoryOption(c)),
    defaultUserAgent: async () => "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36",
    browserStatus: async () => new BrowserStatus({ state: "partial", summary: "Terdaftar untuk Chrome, Edge. Belum terdaftar untuk: Firefox.", registered: true }),
    registerBrowsers: async () => new BrowserStatus({ state: "ok", summary: "Terdaftar untuk Chrome, Edge, Firefox.", registered: true }),
    unregisterBrowsers: async () => new BrowserStatus({ state: "none", summary: "Belum terdaftar: Chrome, Edge, Firefox.", registered: false }),
    toolStatus: async (tool, typed) => {
      await new Promise((r) => setTimeout(r, 300));
      if (typed.trim()) return new ToolStatus({ state: "nothere" });
      return tool === "ffmpeg"
        ? new ToolStatus({ state: "found", path: String.raw`C:\Users\me\AppData\Roaming\odm\tools\ffmpeg.exe`, version: "ffmpeg version n8.1" })
        : new ToolStatus({ state: "found", path: String.raw`C:\Users\me\AppData\Roaming\odm\tools\yt-dlp.exe`, version: "2026.08.19" });
    },
    updateTools: async () => void (await new Promise((r) => setTimeout(r, 900))),
    openOptions: async (tab) => void window.open(`?view=options&tab=${tab}`, "_blank", "width=820,height=640"),
    pickFile: async () => String.raw`C:\Tools\ffmpeg.exe`,
    scheduler: async () =>
      new SchedulerData({
        queues: queues.map((n) => new QueueState({ name: n, running: running.includes(n), repeating: false })),
        schedules: Object.fromEntries(queues.map((n) => [n, new Schedule(schedules[n] ?? newSchedule())])),
        limits: new Limits(limitsState),
      }),
    saveScheduler: async (s, l) => {
      for (const [q, sc] of Object.entries(s)) {
        if (sc.startEnabled && !/^([01]\d|2[0-3]):[0-5]\d$/.test(sc.startAt)) return new SchedulerRefusal({ queue: q, field: "start", value: sc.startAt });
        if (sc.stopEnabled && !/^([01]\d|2[0-3]):[0-5]\d$/.test(sc.stopAt)) return new SchedulerRefusal({ queue: q, field: "stop", value: sc.stopAt });
        if (sc.repeatEnabled && sc.repeatMinutes < 1) return new SchedulerRefusal({ queue: q, field: "repeat" });
      }
      Object.assign(schedules, s);
      limitsState = l;
      return null;
    },
    addQueue: async (name) => {
      if (!name.trim()) throw new Error("enter a name");
      if (queues.some((q) => q.toLowerCase() === name.trim().toLowerCase())) throw new Error("that queue already exists");
      queues.push(name.trim());
      push();
    },
    removeQueue: async (name) => {
      const i = queues.indexOf(name);
      if (i <= 0) throw new Error("no such queue");
      queues.splice(i, 1);
      for (const d of items) if (d.queue === name) d.queue = "Main";
      push();
    },
    openScheduler: async (newQueue) => void window.open(`?view=scheduler${newQueue ? "&new=1" : ""}`, "_blank", "width=860,height=640"),
    exportList: async () => String.raw`C:\Users\me\Documents\odm-downloads.json`,
    importList: async () => 3,
    checkUpdate: async () => new UpdateInfo({ current: "0.4.2", latest: "v0.5.0", newer: true, url: "https://github.com/rigaz29/odm/releases/tag/v0.5.0", notes: "## ODM 0.5.0\n\n- UI baru untuk Windows (pratinjau)\n- Rekam siaran live YouTube dari ujung siaran\n- Perbaikan Unduh Nanti yang hilang setelah restart" }),
    onUpdate: () => () => {},
    about: async () => new About({ name: "Ochi Download Manager", version: "0.4.2", repo: "rigaz29/odm", runtime: "go1.27.2 · windows/amd64" }),
    openURL: async (address) => void window.open(address, "_blank"),
    grab: async () => {
      for (let i = 1; i <= 4; i++) {
        await new Promise((r) => setTimeout(r, 300));
        grabSubs.forEach((cb) => cb(new GrabProgress({ pages: i, files: i * 3 })));
      }
      return new GrabProgress({ pages: 4, files: 12 });
    },
    onGrab: (cb) => {
      grabSubs.add(cb);
      return () => grabSubs.delete(cb);
    },
    stopGrab: async () => {},
    runFinish: async () => {},
    setToolUpdates: async (on) => void (setup.toolMode = on ? "auto" : "off"),
    setupStatus: async () =>
      new SetupStatus({
        toolMode: setup.toolMode, atStart: setup.atStart,
        tools: setup.tools.map((x) => new SetupTool(x)), browsers: setup.browsers.map((b) => new SetupBrowser(b)),
      }),
    setSetupAtStart: async (on) => void (setup.atStart = on),
    setupBrowsers: async () => setup.browsers.map((b) => new SetupBrowser(b)),
    installTool: async (name) => {
      const total = 9_400_000;
      for (let done = 0; done <= total; done += 940_000) {
        await new Promise((r) => setTimeout(r, 120));
        setupSubs.forEach((cb) => cb({ tool: name, done, total }));
      }
      const tool = setup.tools.find((x) => x.name === name) ?? { name };
      Object.assign(tool, { state: "found", path: "C:\\Users\\me\\AppData\\Local\\odm\\tools\\" + name + ".exe", version: name === "ffmpeg" ? "ffmpeg version n8.1" : "2026.10.10" });
      return new SetupTool(tool);
    },
    onSetupProgress: (cb) => {
      setupSubs.add(cb);
      return () => setupSubs.delete(cb);
    },
    installExtension: async (key) => {
      const b = setup.browsers.find((x) => x.key === key)!;
      setTimeout(() => (b.connected = true), 4000); // as if loaded, then talking to ODM
      if (b.family === "firefox") return new ExtensionStep({});
      return new ExtensionStep({ page: key === "edge" ? "edge://extensions" : "chrome://extensions", folder: String.raw`C:\Users\me\AppData\Local\odm\extension\chromium` });
    },
    showExtensionFolder: async () => {},
    openSetup: async () => void window.open("?view=setup", "_blank", "width=640,height=740"),
    setTitle: (title) => void (document.title = title),
    windowSize: async () => ({ width: window.innerWidth, height: window.innerHeight }),
    setSize: () => {},
    closeWindow: () => window.close(),
  };
}
