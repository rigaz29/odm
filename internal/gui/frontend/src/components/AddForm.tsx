// The New Download form, as the Fyne UI's (internal/ui/dialogs_desktop.go,
// newAddForm): the address, where and under what name to save it, and for a
// stream or a video page its quality and audio, for a torrent its files.
// Typing the address checks it (after a pause) and the card says what it
// is; Start Download adds and starts it, Download Later only adds it. The
// main window shows the form in a dialog, a download from the browser in a
// window of its own.

import { Download, FolderOpen, ListPlus, LoaderCircle } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { type AddDefaults, type AddPrefill, type Item, ProbeResult } from "../bindings/odm/internal/gui/models";
import { categoryLabel, displayName, humanBytes, statusKind, statusLabel, withExt } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { ProbeCard } from "./ProbeCard";
import { TorrentFiles } from "./TorrentFiles";
import { ZipPreview } from "./ZipPreview";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Input, Select } from "./ui/input";

const CHECK_DELAY = 600; // ms after the last key before the address is checked
const TORRENT_POLL = 1000; // ms between looks at a magnet link waiting for its files

// checked says whether the form checks addr before it is added: a web
// address, a magnet link or a .torrent file.
const checked = (addr: string) => addr.startsWith("http") || /^magnet:\?/i.test(addr) || /\.torrent$/i.test(addr);

function Row({ label, htmlFor, children }: { label: string; htmlFor?: string; children: React.ReactNode }) {
  return (
    <>
      <label htmlFor={htmlFor} className="pt-1.5 text-sm text-muted-foreground">
        {label}
      </label>
      <div className="min-w-0">{children}</div>
    </>
  );
}

export function AddForm(props: {
  token: string; // the browser's hand-off, "" for none
  initialURL?: string;
  onDone: () => void; // closes the dialog or the window
  footerClassName?: string;
}) {
  const [defaults, setDefaults] = useState<AddDefaults | null>(null);
  const [pre, setPre] = useState<AddPrefill | null>(null);
  const [url, setUrl] = useState("");
  const [category, setCategory] = useState("Other");
  const [catTouched, setCatTouched] = useState(false);
  const [folder, setFolder] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [connections, setConnections] = useState("8");
  const [queue, setQueue] = useState("Main");
  const [checksum, setChecksum] = useState("");
  const [probe, setProbe] = useState<ProbeResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [quality, setQuality] = useState(0);
  const [audio, setAudio] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [duplicate, setDuplicate] = useState<{ item: Item; start: boolean } | null>(null);
  const [zipOpen, setZipOpen] = useState(false);
  // A torrent's files left out, by index, and odm-libtorrent installing.
  const [skip, setSkip] = useState<Set<number>>(new Set());
  const [prio, setPrio] = useState<Record<number, number>>({});
  const [installing, setInstalling] = useState(false);
  const [installed, setInstalled] = useState(-1); // percent of its download, -1 unknown
  const urlRef = useRef<HTMLInputElement>(null);
  // The category before a stream or video page set Video, to put back
  // when the address changes.
  const before = useRef<string | null>(null);
  // The fields as they are now, for an answer that comes later.
  const latest = useRef({ category, catTouched });
  latest.current = { category, catTouched };

  useEffect(() => {
    void Promise.all([api.addDefaults(), props.token ? api.prefill(props.token) : Promise.resolve(null)]).then(([d, p]) => {
      setDefaults(d);
      setFolder(d.folder);
      setConnections(String(d.connections));
      setQueue(d.queue);
      if (p) {
        setPre(p);
        setName(p.fileName);
        setDescription(p.description);
        if (p.category) {
          setCategory(p.category);
          setCatTouched(true); // the browser's choice is kept
        }
      }
      setUrl(p?.url || props.initialURL || "");
    });
  }, [props.token, props.initialURL]);

  // The address field takes the keys once the form is there.
  useEffect(() => {
    if (defaults) urlRef.current?.focus();
  }, [defaults]);

  // apply takes the check of addr in, unless the address changed since.
  const apply = (addr: string, r: ProbeResult) => {
    if (asked.current !== addr) return;
    setChecking(false);
    setProbe(r);
    setQuality(r.quality);
    setAudio(0);
    setName((n) => {
      const v = n || r.fileName;
      return r.kind === "site" && r.qualities[r.quality] ? withExt(v, r.qualities[r.quality].ext) : v;
    });
    if (!latest.current.catTouched) {
      if (r.video) {
        before.current = latest.current.category;
        setCategory("Video");
      } else if (r.category) setCategory(r.category);
    }
  };

  // Check the address once typing pauses; a result for an address since
  // changed is dropped.
  const asked = useRef("");
  useEffect(() => {
    if (!defaults) return;
    const addr = url.trim();
    asked.current = addr;
    setProbe(null);
    setError("");
    if (before.current !== null) {
      setCategory(before.current);
      before.current = null;
    }
    if (!addr) {
      setChecking(false);
      return;
    }
    setChecking(checked(addr));
    setSkip(new Set());
    setPrio({});
    const timer = setTimeout(() => {
      void api.probe(addr, props.token).then((r) => apply(addr, r));
    }, CHECK_DELAY);
    return () => clearTimeout(timer);
    // Only a new address (or the defaults arriving) checks again; the
    // other fields are read as they are when the answer comes.
  }, [url, defaults]);

  const torrent = probe?.kind === "torrent" ? probe.torrent : null;
  // ODM lets go of a torrent it read once the address changes or the form
  // closes (after Start, it is the download's already).
  useEffect(() => {
    if (probe?.kind !== "torrent") return;
    const addr = probe.url;
    return () => void api.dropTorrentProbe(addr);
  }, [probe?.kind, probe?.url]);
  // A magnet link's files come from its peers: look again until they are
  // there, then check the address once more for its name and category.
  useEffect(() => {
    if (!probe || !torrent?.waiting) return;
    const addr = probe.url;
    const timer = setInterval(() => {
      void api.torrentProbe(addr).then(
        (tp) => {
          if (asked.current !== addr) return;
          if (!tp.waiting) {
            clearInterval(timer);
            void api.probe(addr, props.token).then((r) => apply(addr, r));
            return;
          }
          setProbe((p) => (p && p.url === addr && p.torrent ? new ProbeResult({ ...p, torrent: tp }) : p));
        },
        () => clearInterval(timer),
      );
    }, TORRENT_POLL);
    return () => clearInterval(timer);
  }, [probe?.url, torrent?.waiting]);

  useEffect(() => {
    if (!installing) return;
    setInstalled(-1);
    return api.onSetupProgress((p) => {
      if (p.tool === "odm-libtorrent" && p.total > 0) setInstalled(Math.floor((p.done / p.total) * 100));
    });
  }, [installing]);

  const installTorrents = async () => {
    setInstalling(true);
    setError("");
    try {
      await api.installTool("odm-libtorrent");
      const addr = url.trim();
      apply(addr, await api.probe(addr, props.token));
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setInstalling(false);
    }
  };

  const qualities = probe?.qualities ?? [];
  const audios = qualities[quality]?.audios ?? [];
  // A playlist or page the browser sent shows its rows before it is read.
  const waiting = !probe && (pre?.site || pre?.stream) && url.trim() === pre?.url.trim();
  const showQuality = qualities.length > 0 || waiting || probe?.kind === "stream" || probe?.kind === "site";
  // A torrent with its files listed and every one left out has nothing to
  // download.
  const realFiles = (torrent?.files ?? []).filter((f) => !f.pad).length;
  const noFiles = realFiles > 0 && (torrent?.files ?? []).every((f, i) => f.pad || skip.has(i));

  const create = async (start: boolean, again: boolean) => {
    setBusy(true);
    setError("");
    try {
      const res = await api.create({
        token: props.token,
        url,
        folder,
        fileName: name,
        description,
        checksum,
        category,
        queue,
        connections: Number.parseInt(connections, 10) || 0,
        quality,
        audio,
        start,
        again,
        skip: torrent ? [...skip] : [],
        priorities: torrent ? prio : {},
      });
      if (res.duplicate) {
        setDuplicate({ item: res.duplicate, start });
        return;
      }
      props.onDone();
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  if (!defaults) return <div className="h-96" />;
  const canAdd = url.trim() !== "" && !busy && !torrent?.needsProgram && !noFiles;

  return (
    <form
      className="flex min-h-0 flex-1 flex-col"
      onSubmit={(e) => {
        e.preventDefault();
        if (canAdd) void create(true, false);
      }}
    >
      <div className="grid grid-cols-[8.5rem_1fr] items-start gap-x-3 gap-y-2.5">
        <Row label={t("URL")} htmlFor="add-url">
          <Input id="add-url" ref={urlRef} value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://" spellCheck={false} />
        </Row>
        <Row label={t("Category")} htmlFor="add-cat">
          <Select
            id="add-cat"
            value={category}
            onChange={(e) => {
              setCategory(e.target.value);
              setCatTouched(true);
              before.current = null;
            }}
          >
            {defaults.categories.map((c) => (
              <option key={c} value={c}>
                {categoryLabel(c)}
              </option>
            ))}
          </Select>
        </Row>
        <Row label={t("Save folder")} htmlFor="add-folder">
          <div className="flex gap-2">
            <Input id="add-folder" value={folder} onChange={(e) => setFolder(e.target.value)} spellCheck={false} />
            <Button type="button" onClick={() => void api.pickFolder(folder).then((d) => d && setFolder(d))}>
              <FolderOpen /> {t("Browse…")}
            </Button>
          </div>
        </Row>
        <Row label={t("File name")} htmlFor="add-name">
          <Input id="add-name" value={name} onChange={(e) => setName(e.target.value)} spellCheck={false} />
        </Row>
        {showQuality && (
          <>
            <Row label={t("Quality")} htmlFor="add-quality">
              <Select
                id="add-quality"
                value={qualities.length ? quality : ""}
                disabled={qualities.length === 0}
                onChange={(e) => {
                  const q = Number(e.target.value);
                  setQuality(q);
                  setAudio(0);
                  if (probe?.kind === "site") setName((n) => withExt(n, qualities[q]?.ext ?? ""));
                }}
              >
                {qualities.length === 0 && <option value="">{waiting ? (pre?.site ? t("Asking yt-dlp…") : t("Reading the playlist…")) : "—"}</option>}
                {qualities.map((q, i) => (
                  <option key={q.label} value={i}>
                    {q.estimate > 0 ? `${q.label} · ~${humanBytes(q.estimate)}` : q.label}
                  </option>
                ))}
              </Select>
            </Row>
            <Row label={t("Audio")} htmlFor="add-audio">
              <Select id="add-audio" value={audios.length ? audio : ""} disabled={audios.length < 2} onChange={(e) => setAudio(Number(e.target.value))}>
                {audios.length === 0 && <option value="">{t("In the video")}</option>}
                {audios.map((a, i) => (
                  <option key={a} value={i}>
                    {a}
                  </option>
                ))}
              </Select>
            </Row>
          </>
        )}
        <Row label={t("Description")} htmlFor="add-desc">
          <Input id="add-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        </Row>
        {probe?.kind === "torrent" ? (
          <Row label={t("Queue")} htmlFor="add-queue">
            <Select id="add-queue" value={queue} onChange={(e) => setQueue(e.target.value)}>
              {defaults.queues.map((q) => (
                <option key={q} value={q}>
                  {q}
                </option>
              ))}
            </Select>
          </Row>
        ) : (
          <>
            <Row label={t("Connections")} htmlFor="add-conns">
              <div className="flex items-center gap-3">
                <Input id="add-conns" inputMode="numeric" className="w-20" value={connections} onChange={(e) => setConnections(e.target.value.replace(/\D/g, ""))} />
                <label htmlFor="add-queue" className="text-sm text-muted-foreground">
                  {t("Queue")}
                </label>
                <Select id="add-queue" className="flex-1" value={queue} onChange={(e) => setQueue(e.target.value)}>
                  {defaults.queues.map((q) => (
                    <option key={q} value={q}>
                      {q}
                    </option>
                  ))}
                </Select>
              </div>
            </Row>
            <Row label={t("Checksum")} htmlFor="add-sum">
              <Input id="add-sum" value={checksum} onChange={(e) => setChecksum(e.target.value)} placeholder={t("SHA-256, SHA-1 or MD5 (optional)")} spellCheck={false} />
            </Row>
          </>
        )}
      </div>

      <div className="mt-4">
        <ProbeCard
          card={probe?.card ?? null}
          torrent={torrent}
          checking={checking}
          name={name}
          category={category}
          action={
            torrent?.needsProgram ? (
              <Button type="button" size="sm" disabled={installing} onClick={() => void installTorrents()}>
                {installing ? <LoaderCircle className="animate-spin" /> : <Download />} {installing && installed >= 0 ? `${installed}%` : t("Install")}
              </Button>
            ) : (
              probe?.zip && (
                <Button type="button" size="sm" onClick={() => setZipOpen(true)}>
                  {t("Preview…")}
                </Button>
              )
            )
          }
        />
        {torrent && realFiles > 0 && (
          <div className="mt-3">
            <TorrentFiles files={torrent.files} skip={skip} onSkip={setSkip} prio={prio} onPrio={setPrio} />
          </div>
        )}
        {torrent?.firstTorrent && (
          <p className="mt-2 text-xs text-muted-foreground">{t("Windows may ask whether odm-libtorrent may use the network: allow it, so that peers can reach ODM.")}</p>
        )}
      </div>

      <div className={cn("mt-auto flex items-center gap-2 pt-4", props.footerClassName)}>
        <p className="mr-auto min-w-0 truncate text-xs text-red-600 dark:text-red-400" title={error}>
          {error}
        </p>
        <Button type="button" disabled={!canAdd} onClick={() => void create(false, false)}>
          <ListPlus /> {t("Download Later")}
        </Button>
        <Button type="button" onClick={props.onDone}>
          {t("Cancel")}
        </Button>
        <Button type="submit" variant="brand" disabled={!canAdd}>
          <Download /> {t("Start Download")}
        </Button>
      </div>

      <ZipPreview open={zipOpen} url={url} token={props.token} name={probe?.fileName || name} onClose={() => setZipOpen(false)} />
      <Dialog open={duplicate !== null} onOpenChange={(o) => !o && setDuplicate(null)}>
        {duplicate && (
          <DialogContent className="max-w-md">
            <div className="space-y-1.5 pr-6">
              <DialogTitle>{t("Duplicate download")}</DialogTitle>
              <DialogDescription>
                {tf("This address is already in the list as “%s” (%s).", displayName(duplicate.item), statusLabel(statusKind(duplicate.item)))}{" "}
                {t("Downloading it again saves it under a numbered name.")}
              </DialogDescription>
            </div>
            <div className="flex justify-end gap-2">
              <Button type="button" onClick={() => setDuplicate(null)}>
                {t("Cancel")}
              </Button>
              <Button
                type="button"
                onClick={() => {
                  void api.openProgress(duplicate.item.id);
                  setDuplicate(null);
                  props.onDone();
                }}
              >
                {t("Show existing")}
              </Button>
              <Button
                type="button"
                variant="brand"
                autoFocus
                onClick={() => {
                  const start = duplicate.start;
                  setDuplicate(null);
                  void create(start, true);
                }}
              >
                {t("Download again")}
              </Button>
            </div>
          </DialogContent>
        )}
      </Dialog>
    </form>
  );
}
