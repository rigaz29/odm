// The New Download form, as the Fyne UI's (internal/ui/dialogs_desktop.go,
// newAddForm): the address, where and under what name to save it, and for a
// stream or a video page its quality and audio. Typing the address checks
// it (after a pause) and the card says what it is; Start Download adds and
// starts it, Download Later only adds it. The main window shows the form in
// a dialog, a download from the browser in a window of its own.

import { Download, FolderOpen, ListPlus } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import type { AddDefaults, AddPrefill, Item, ProbeResult } from "../bindings/odm/internal/gui/models";
import { categoryLabel, displayName, humanBytes, statusKind, statusLabel, withExt } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { ProbeCard } from "./ProbeCard";
import { ZipPreview } from "./ZipPreview";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Input, Select } from "./ui/input";

const CHECK_DELAY = 600; // ms after the last key before the address is checked

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
    setChecking(addr.startsWith("http"));
    const timer = setTimeout(() => {
      void api.probe(addr, props.token).then((r) => {
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
      });
    }, CHECK_DELAY);
    return () => clearTimeout(timer);
    // Only a new address (or the defaults arriving) checks again; the
    // other fields are read as they are when the answer comes.
  }, [url, defaults]);

  const qualities = probe?.qualities ?? [];
  const audios = qualities[quality]?.audios ?? [];
  // A playlist or page the browser sent shows its rows before it is read.
  const waiting = !probe && (pre?.site || pre?.stream) && url.trim() === pre?.url.trim();
  const showQuality = qualities.length > 0 || waiting || probe?.kind === "stream" || probe?.kind === "site";

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
        skip: [],
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
  const canAdd = url.trim() !== "" && !busy;

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
      </div>

      <div className="mt-4">
        <ProbeCard
          card={probe?.card ?? null}
          checking={checking}
          name={name}
          category={category}
          action={
            probe?.zip && (
              <Button type="button" size="sm" onClick={() => setZipOpen(true)}>
                {t("Preview…")}
              </Button>
            )
          }
        />
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
