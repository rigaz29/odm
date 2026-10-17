// The Scheduler window (?view=scheduler), as the Fyne UI's (internal/ui/
// scheduler_desktop.go): the queues on the left, then the download limits;
// for a queue its schedule and its files. Edits stay here until Apply,
// which checks every queue's times (scheduler.go); Start now applies first.

import { ArrowDown, ArrowUp, CalendarClock, Gauge, ListOrdered, Play, Plus, RefreshCw, Square, Trash2, X } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { api } from "../api";
import type { Item, Limits, Schedule, SchedulerData, Snapshot } from "../bindings/odm/internal/gui/models";
import { StatusBadge } from "../components/bits";
import { Button } from "../components/ui/button";
import { Checkbox } from "../components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "../components/ui/dialog";
import { Input } from "../components/ui/input";
import { displayName, etaText, humanBytes } from "../format";
import { markT, t, tf } from "../i18n";
import { cn } from "../lib/utils";

const LIMITS = "\u0000limits"; // the list's last entry, not a queue

const DAYS = [markT("Sunday"), markT("Monday"), markT("Tuesday"), markT("Wednesday"), markT("Thursday"), markT("Friday"), markT("Saturday")];

const FINISH: [string, string][] = [
  ["", markT("Do nothing")],
  ["exit", markT("Exit ODM")],
  ["sleep", markT("Sleep")],
  ["hibernate", markT("Hibernate")],
  ["shutdown", markT("Turn off computer")],
];

const num = (s: string) => Number.parseInt(s.replace(/\D/g, ""), 10) || 0;

function Check({ label, checked, onChange, disabled }: { label: string; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <label className={cn("flex items-start gap-2.5 text-sm", disabled && "opacity-50")}>
      <Checkbox className="mt-0.5" checked={checked} disabled={disabled} onCheckedChange={(v) => onChange(v === true)} />
      <span>{label}</span>
    </label>
  );
}

const Hint = ({ children }: { children: ReactNode }) => <p className="text-xs text-muted-foreground">{children}</p>;

export function SchedulerWindow({ newQueue }: { newQueue: boolean }) {
  const [data, setData] = useState<SchedulerData | null>(null);
  const [edits, setEdits] = useState<Record<string, Schedule>>({});
  const [limits, setLimits] = useState<Limits | null>(null);
  const [current, setCurrent] = useState("Main");
  const [tab, setTab] = useState<"schedule" | "files">("schedule");
  const [dirty, setDirty] = useState(false);
  const [problem, setProblem] = useState("");
  const [naming, setNaming] = useState<string | null>(null); // a new queue's name, while asked
  const [closing, setClosing] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [snap, setSnap] = useState<Snapshot | null>(null);

  const load = (keepEdits: boolean) =>
    api.scheduler().then((d) => {
      setData(d);
      setLimits((l) => (keepEdits && l ? l : d.limits));
      setEdits((e) => {
        const next: Record<string, Schedule> = {};
        for (const q of d.queues) {
          const sc = (keepEdits && e[q.name]) || d.schedules[q.name];
          if (sc) next[q.name] = sc;
        }
        return next;
      });
    });

  useEffect(() => {
    api.setTitle(t("Scheduler"));
    void load(false);
    void api.snapshot().then(setSnap);
    const off = api.onSnapshot((s) => {
      setSnap(s);
      void api.scheduler().then((d) => setData((was) => (was ? { ...d, schedules: was.schedules } : d)));
    });
    if (newQueue) setNaming(tf("Queue # %d", 2));
    return off;
  }, []);

  const files = useMemo(
    () => (snap?.items ?? []).filter((d) => (d.queue || "Main") === current).sort((a, b) => a.order - b.order),
    [snap, current],
  );
  const [picked, setPicked] = useState<string | null>(null);

  if (!data || !limits) return <div className="h-full bg-background" />;
  const sc = edits[current];
  const state = data.queues.find((q) => q.name === current);
  const isQueue = current !== LIMITS && !!sc;
  const edit = (patch: Partial<Schedule>) => {
    setEdits((e) => ({ ...e, [current]: { ...e[current], ...patch } as Schedule }));
    setDirty(true);
  };
  const editLimits = (patch: Partial<Limits>) => {
    setLimits((l) => ({ ...l!, ...patch }) as Limits);
    setDirty(true);
  };

  const apply = async (): Promise<boolean> => {
    setProblem("");
    const r = await api.saveScheduler(edits, limits);
    if (r) {
      setCurrent(r.queue);
      setTab("schedule");
      setProblem(
        r.field === "repeat"
          ? tf("Enter how many minutes “%s” waits before it starts again (1 or more).", r.queue)
          : tf("%s time “%s” in “%s” is not a time; use HH:MM, e.g. 23:00.", r.field === "start" ? t("Start") : t("Stop"), r.value, r.queue),
      );
      return false;
    }
    setDirty(false);
    await load(false);
    return true;
  };

  const addQueue = async () => {
    const name = (naming ?? "").trim();
    try {
      await api.addQueue(name);
    } catch (e) {
      setProblem(String((e as Error)?.message ?? e) === "that queue already exists" ? t("That queue already exists.") : t("Enter a name."));
      return;
    }
    setNaming(null);
    setProblem("");
    await load(true);
    setCurrent(name);
  };

  const move = (delta: number) => picked && void api.moveInQueue(picked, delta);

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      <div className="flex min-h-0 flex-1">
        <aside className="flex w-56 shrink-0 flex-col gap-2 border-r border-border p-3">
          <p className="px-1 text-xs font-medium text-muted-foreground">{t("Queues")}</p>
          <div role="listbox" aria-label={t("Queues")} className="min-h-0 flex-1 space-y-0.5 overflow-y-auto">
            {data.queues.map((q, i) => (
              <button
                key={q.name}
                role="option"
                aria-selected={current === q.name}
                onClick={() => setCurrent(q.name)}
                className={cn("relative flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm", current === q.name ? "bg-accent font-medium" : "text-muted-foreground hover:bg-accent hover:text-foreground")}
              >
                {current === q.name && <span className="accent-gradient absolute -left-3 top-1.5 h-5 w-[3px] rounded-r-full" />}
                {edits[q.name]?.sync ? <RefreshCw className="size-4 shrink-0" /> : i === 0 ? <ListOrdered className="size-4 shrink-0" /> : <CalendarClock className="size-4 shrink-0" />}
                <span className="flex-1 truncate">{q.name}</span>
                {(q.running || q.repeating) && <span className="accent-gradient size-1.5 rounded-full" title={t("Queue started")} />}
              </button>
            ))}
            <button
              role="option"
              aria-selected={current === LIMITS}
              onClick={() => setCurrent(LIMITS)}
              className={cn("relative flex h-8 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm", current === LIMITS ? "bg-accent font-medium" : "text-muted-foreground hover:bg-accent hover:text-foreground")}
            >
              {current === LIMITS && <span className="accent-gradient absolute -left-3 top-1.5 h-5 w-[3px] rounded-r-full" />}
              <Gauge className="size-4 shrink-0" />
              <span className="truncate">{t("Download limits")}</span>
            </button>
          </div>
          {naming === null ? (
            <div className="grid grid-cols-2 gap-1.5">
              <Button size="sm" onClick={() => setNaming(tf("Queue # %d", data.queues.length + 1))}>
                <Plus /> {t("New queue")}
              </Button>
              <Button size="sm" disabled={!isQueue || current === data.queues[0]?.name} onClick={() => setRemoving(current)}>
                <Trash2 /> {t("Delete")}
              </Button>
            </div>
          ) : (
            <form
              className="space-y-1.5"
              onSubmit={(e) => {
                e.preventDefault();
                void addQueue();
              }}
            >
              <p className="text-xs text-muted-foreground">{t("Enter a name for the new queue that will be displayed in the list of queues")}</p>
              <Input autoFocus value={naming} onChange={(e) => setNaming(e.target.value)} aria-label={t("Enter queue name")} onFocus={(e) => e.currentTarget.select()} />
              <div className="flex gap-1.5">
                <Button size="sm" type="submit" variant="brand">
                  {t("OK")}
                </Button>
                <Button size="sm" type="button" onClick={() => (setNaming(null), setProblem(""))}>
                  {t("Cancel")}
                </Button>
              </div>
            </form>
          )}
        </aside>

        <main className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-3 border-b border-border px-5 py-3">
            <h1 className="flex-1 truncate text-base font-semibold">{current === LIMITS ? t("Download limits") : current}</h1>
            {isQueue && (
              <div className="inline-flex h-8 items-center rounded-lg bg-muted p-0.5 text-sm" role="tablist">
                {(
                  [
                    ["schedule", t("Schedule")],
                    ["files", t("Files in the queue")],
                  ] as const
                ).map(([k, label]) => (
                  <button
                    key={k}
                    role="tab"
                    aria-selected={tab === k}
                    onClick={() => setTab(k)}
                    className={cn("rounded-md px-3 py-1", tab === k ? "bg-background font-medium shadow-sm" : "text-muted-foreground hover:text-foreground")}
                  >
                    {label}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-5">
            {current === LIMITS ? (
              <LimitsForm limits={limits} edit={editLimits} />
            ) : sc && tab === "schedule" ? (
              <ScheduleForm sc={sc} edit={edit} />
            ) : sc ? (
              <FilesTab
                sc={sc}
                edit={edit}
                files={files}
                picked={picked}
                onPick={setPicked}
                onMove={move}
                canRemove={current !== data.queues[0]?.name}
                onRemove={() => picked && void api.moveToQueue([picked], data.queues[0]?.name ?? "Main")}
              />
            ) : null}
          </div>
        </main>
      </div>

      <div className="flex items-center gap-2 border-t border-border px-4 py-3">
        {isQueue && (
          <>
            <Button
              disabled={state?.running}
              onClick={() =>
                void apply().then((ok) => {
                  if (ok) void api.startQueue(current);
                })
              }
            >
              <Play /> {t("Start now")}
            </Button>
            <Button disabled={!state?.running && !state?.repeating} onClick={() => void api.stopQueue(current)}>
              <Square /> {t("Stop")}
            </Button>
          </>
        )}
        <p className="mx-2 min-w-0 flex-1 truncate text-xs text-red-600 dark:text-red-400" title={problem}>
          {problem}
        </p>
        <Button variant="brand" disabled={!dirty} onClick={() => void apply()}>
          {t("Apply")}
        </Button>
        <Button onClick={() => (dirty ? setClosing(true) : api.closeWindow())}>{t("Close")}</Button>
      </div>

      <Dialog open={closing} onOpenChange={setClosing}>
        <DialogContent className="max-w-sm">
          <div className="space-y-1.5 pr-6">
            <DialogTitle>{t("Scheduler")}</DialogTitle>
            <DialogDescription>{t("Apply the changes before closing?")}</DialogDescription>
          </div>
          <div className="flex justify-end gap-2">
            <Button onClick={() => setClosing(false)}>{t("Cancel")}</Button>
            <Button onClick={api.closeWindow}>{t("No")}</Button>
            <Button
              variant="brand"
              autoFocus
              onClick={() =>
                void apply().then((ok) => {
                  setClosing(false);
                  if (ok) api.closeWindow();
                })
              }
            >
              {t("Yes")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={removing !== null} onOpenChange={(o) => !o && setRemoving(null)}>
        <DialogContent className="max-w-sm">
          <div className="space-y-1.5 pr-6">
            <DialogTitle>{t("Delete queue")}</DialogTitle>
            <DialogDescription>{tf("Delete “%s”? Its downloads move to %s.", removing ?? "", data.queues[0]?.name ?? "Main")}</DialogDescription>
          </div>
          <div className="flex justify-end gap-2">
            <Button onClick={() => setRemoving(null)}>{t("Cancel")}</Button>
            <Button
              variant="destructive"
              autoFocus
              onClick={() => {
                const q = removing!;
                setRemoving(null);
                void api.removeQueue(q).then(() => {
                  setCurrent(data.queues[0]?.name ?? "Main");
                  void load(true);
                });
              }}
            >
              {t("Delete")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ScheduleForm({ sc, edit }: { sc: Schedule; edit: (p: Partial<Schedule>) => void }) {
  const time = (value: string, on: boolean, onChange: (v: string) => void, label: string) => (
    <Input className="w-24 tabular-nums" value={value} disabled={!on} placeholder="HH:MM" onChange={(e) => onChange(e.target.value)} aria-label={label} />
  );
  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Check label={t("Synchronization queue: check finished files and download them again when they change on the server")} checked={sc.sync} onChange={(v) => edit({ sync: v })} />
        <Hint>{t('The new copy replaces the local file once it is complete. With "Start the queue again" below, the check runs every so many minutes.')}</Hint>
      </div>
      <div className="border-t border-border pt-4">
        <Check label={t("Start download on ODM startup")} checked={sc.onStartup} onChange={(v) => edit({ onStartup: v })} />
      </div>
      <div className="space-y-3 border-t border-border pt-4">
        <div className="flex items-center gap-3">
          <Check label={t("Start download at")} checked={sc.startEnabled} onChange={(v) => edit({ startEnabled: v })} />
          {time(sc.startAt, sc.startEnabled, (v) => edit({ startAt: v }), t("Start download at"))}
        </div>
        <div className={cn("space-y-2.5 pl-6", !sc.startEnabled && "opacity-50")}>
          <div className="flex items-center gap-4 text-sm">
            <label className="flex items-center gap-2">
              <input type="radio" name="mode" className="size-4 accent-violet-600" disabled={!sc.startEnabled} checked={sc.once} onChange={() => edit({ once: true })} />
              {t("Once at")}
            </label>
            <Input type="date" className="w-44" disabled={!sc.startEnabled || !sc.once} value={sc.onceDate} onChange={(e) => edit({ onceDate: e.target.value })} aria-label={t("Once at")} />
            <label className="flex items-center gap-2">
              <input type="radio" name="mode" className="size-4 accent-violet-600" disabled={!sc.startEnabled} checked={!sc.once} onChange={() => edit({ once: false })} />
              {t("Daily")}
            </label>
          </div>
          <div className="grid grid-cols-4 gap-2">
            {DAYS.map((d, i) => (
              <Check
                key={d}
                label={t(d)}
                disabled={!sc.startEnabled || sc.once}
                checked={sc.days[i]}
                onChange={(v) => edit({ days: sc.days.map((x, k) => (k === i ? v : x)) })}
              />
            ))}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Check label={t("Start the queue again every")} checked={sc.repeatEnabled} onChange={(v) => edit({ repeatEnabled: v })} />
          <Input className="w-20" inputMode="numeric" disabled={!sc.repeatEnabled} value={sc.repeatMinutes || ""} onChange={(e) => edit({ repeatMinutes: num(e.target.value) })} aria-label={t("Start the queue again every")} />
          <span className={cn("text-sm text-muted-foreground", !sc.repeatEnabled && "opacity-50")}>{t("minutes, until it is stopped")}</span>
        </div>
      </div>
      <div className="space-y-3 border-t border-border pt-4">
        <div className="flex items-center gap-3">
          <Check label={t("Stop download at")} checked={sc.stopEnabled} onChange={(v) => edit({ stopEnabled: v })} />
          {time(sc.stopAt, sc.stopEnabled, (v) => edit({ stopAt: v }), t("Stop download at"))}
        </div>
        <div className="flex items-center gap-3">
          <Check label={t("Number of retries for each file if downloading failed:")} checked={sc.retriesEnabled} onChange={(v) => edit({ retriesEnabled: v })} />
          <Input className="w-20" inputMode="numeric" disabled={!sc.retriesEnabled} value={sc.retries || ""} onChange={(e) => edit({ retries: num(e.target.value) })} aria-label={t("Number of retries for each file if downloading failed:")} />
        </div>
      </div>
      <div className={cn("space-y-3 border-t border-border pt-4", sc.repeatEnabled && "opacity-50")}>
        <Check label={t("Open the following file when done:")} checked={sc.openEnabled} disabled={sc.repeatEnabled} onChange={(v) => edit({ openEnabled: v })} />
        <div className="flex gap-2 pl-6">
          <Input value={sc.openPath} disabled={sc.repeatEnabled || !sc.openEnabled} placeholder={t("file to open")} onChange={(e) => edit({ openPath: e.target.value })} spellCheck={false} />
          <Button disabled={sc.repeatEnabled || !sc.openEnabled} onClick={() => void api.pickFile(sc.openPath).then((f) => f && edit({ openPath: f }))}>
            …
          </Button>
        </div>
        <p className="text-sm">{t("When the queue has finished:")}</p>
        <div className="flex flex-wrap gap-x-4 gap-y-2 pl-6 text-sm">
          {FINISH.map(([action, label]) => (
            <label key={action} className="flex items-center gap-2">
              <input type="radio" name="finish" className="size-4 accent-violet-600" disabled={sc.repeatEnabled} checked={sc.finish === action} onChange={() => edit({ finish: action })} />
              {t(label)}
            </label>
          ))}
        </div>
        <Hint>{t("A queue that starts again is never finished, so these do not run for it.")}</Hint>
      </div>
    </div>
  );
}

function FilesTab(props: {
  sc: Schedule;
  edit: (p: Partial<Schedule>) => void;
  files: Item[];
  picked: string | null;
  onPick: (id: string) => void;
  onMove: (delta: number) => void;
  canRemove: boolean;
  onRemove: () => void;
}) {
  const { sc, files } = props;
  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center gap-2 text-sm">
        {t("Download")}
        <Input className="w-16" inputMode="numeric" value={sc.parallel || ""} onChange={(e) => props.edit({ parallel: num(e.target.value) })} aria-label={t("files at the same time")} />
        {t("files at the same time")}
      </div>
      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-muted text-xs text-muted-foreground">
            <tr>
              <th className="px-3 py-1.5 text-left font-medium">{t("File Name")}</th>
              <th className="w-24 px-3 py-1.5 text-left font-medium">{t("Size")}</th>
              <th className="w-32 px-3 py-1.5 text-left font-medium">{t("Status")}</th>
              <th className="w-28 px-3 py-1.5 text-left font-medium">{t("Time left")}</th>
            </tr>
          </thead>
          <tbody>
            {files.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-4 text-center text-xs text-muted-foreground">
                  —
                </td>
              </tr>
            )}
            {files.map((d) => (
              <tr
                key={d.id}
                aria-selected={props.picked === d.id}
                onClick={() => props.onPick(d.id)}
                className={cn("cursor-default border-t border-border", props.picked === d.id ? "bg-violet-500/10" : "hover:bg-muted/50")}
              >
                <td className="max-w-0 truncate px-3 py-1.5" title={displayName(d)}>
                  {displayName(d)}
                </td>
                <td className="px-3 py-1.5 tabular-nums text-muted-foreground">{humanBytes(d.total) || "—"}</td>
                <td className="px-3 py-1.5">
                  <StatusBadge d={d} />
                </td>
                <td className="px-3 py-1.5 tabular-nums text-muted-foreground">{etaText(d) || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex gap-1.5">
        <Button size="sm" disabled={!props.picked} onClick={() => props.onMove(-1)} aria-label={t("Move up in queue")}>
          <ArrowUp /> {t("Move up in queue")}
        </Button>
        <Button size="sm" disabled={!props.picked} onClick={() => props.onMove(1)} aria-label={t("Move down in queue")}>
          <ArrowDown /> {t("Move down in queue")}
        </Button>
        <Button size="sm" disabled={!props.picked || !props.canRemove} onClick={props.onRemove}>
          <X /> {t("Take out of the queue")}
        </Button>
      </div>
    </div>
  );
}

function LimitsForm({ limits, edit }: { limits: Limits; edit: (p: Partial<Limits>) => void }) {
  return (
    <div className="space-y-3">
      <Check label={t("Download limits")} checked={limits.on} onChange={(v) => edit({ on: v })} />
      <div className={cn("flex flex-wrap items-center gap-2 pl-6 text-sm", !limits.on && "opacity-50")}>
        {t("Download no more than")}
        <Input className="w-20" inputMode="numeric" disabled={!limits.on} value={limits.mb || ""} onChange={(e) => edit({ mb: num(e.target.value) })} aria-label="MB" />
        {t("MBytes every")}
        <Input className="w-16" inputMode="numeric" disabled={!limits.on} value={limits.hours || ""} onChange={(e) => edit({ hours: num(e.target.value) })} aria-label={t("hours")} />
        {t("hours")}
      </div>
      <div className="pl-6">
        <Check label={t("Show a warning when downloads are stopped")} checked={limits.warning} disabled={!limits.on} onChange={(v) => edit({ warning: v })} />
      </div>
    </div>
  );
}
