// The title bar the page draws (the window has no frame, see app.go): its
// empty areas are --wails-draggable, so they move the window as Windows'
// own title bar does, and maximize it on a double click; the buttons call
// the window. The search box in the middle takes clicks. On Linux and macOS
// the window keeps the system's frame, and the bar has no window buttons.
//
// The search box (Ctrl K) filters the list as it is typed and offers
// commands under it (palette.ts): a pasted address first, then the
// commands its words match. Up and Down pick one, Enter runs it; with no
// command to run, Enter goes to the list.

import { Window } from "@wailsio/runtime";
import { ArrowDownToLine, CornerDownLeft, Download, Minus, Search, Square, X } from "lucide-react";
import { type CSSProperties, type RefObject, useEffect, useRef, useState } from "react";
import { framed, inODM } from "../api";
import { looksLikeURL } from "../format";
import { t } from "../i18n";
import { cn } from "../lib/utils";
import { type Command, cycle, matchCommands } from "../palette";

export function TitleBar(props: {
  query: string;
  onQuery: (q: string) => void;
  commands: Command[];
  onDownload: (url: string) => void; // a pasted address: the Add URL dialog
  onToList: () => void; // Enter with no command to run
  searchRef: RefObject<HTMLInputElement | null>;
}) {
  const [open, setOpen] = useState(false);
  const [at, setAt] = useState(0);
  const list = useRef<HTMLDivElement>(null);
  const call = (f: () => Promise<void>) => () => {
    if (inODM) void f();
  };
  const caption = {
    style: { "--wails-draggable": "drag" } as CSSProperties,
    onDoubleClick: call(() => Window.ToggleMaximise()),
  };

  const url = looksLikeURL(props.query) ? props.query.trim() : "";
  const items: Command[] = url
    ? [{ id: "download", label: t("Download this address"), english: "", group: "", icon: Download, run: () => props.onDownload(url) }]
    : matchCommands(props.commands, props.query);
  const showing = open && items.length > 0;

  useEffect(() => setAt(0), [props.query]);
  useEffect(() => {
    list.current?.querySelector(`[data-at="${at}"]`)?.scrollIntoView({ block: "nearest" });
  }, [at]);

  const run = (c: Command) => {
    setOpen(false);
    props.onQuery("");
    props.searchRef.current?.blur();
    c.run();
  };

  return (
    <header className="relative z-30 flex h-10 shrink-0 items-stretch border-b border-border">
      <div className="flex flex-1 items-center gap-2 pl-3" {...caption}>
        <span className="accent-gradient flex size-5 items-center justify-center rounded-[5px] text-white shadow-sm shadow-violet-500/30">
          <ArrowDownToLine className="size-3.5" strokeWidth={2.5} />
        </span>
        <span className="text-[13px] font-semibold">ODM</span>
        <span className="truncate text-[13px] text-muted-foreground">Ochi Download Manager</span>
      </div>
      <div className="relative flex items-center">
        <label className="flex h-7 w-[26rem] items-center gap-2 rounded-md border border-border bg-background/60 px-2.5 text-[13px] text-muted-foreground focus-within:border-ring/70 focus-within:ring-2 focus-within:ring-ring/25">
          <Search className="size-3.5 shrink-0" />
          <input
            ref={props.searchRef}
            role="combobox"
            aria-label={t("Search downloads or paste a URL…")}
            aria-keyshortcuts="Control+K"
            aria-expanded={showing}
            aria-controls="odm-palette"
            aria-activedescendant={showing ? `odm-palette-${at}` : undefined}
            className="min-w-0 flex-1 bg-transparent text-foreground outline-none placeholder:text-muted-foreground"
            placeholder={t("Search downloads or paste a URL…")}
            value={props.query}
            onChange={(e) => {
              props.onQuery(e.target.value);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            onBlur={() => setOpen(false)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                e.preventDefault();
                setOpen(true);
                setAt((a) => cycle(a, e.key === "ArrowDown" ? 1 : -1, items.length));
              } else if (e.key === "Enter") {
                e.preventDefault();
                if (showing && items[at]) run(items[at]);
                else {
                  setOpen(false);
                  props.onToList();
                }
              } else if (e.key === "Escape") {
                if (props.query) props.onQuery("");
                else if (showing) setOpen(false);
                else e.currentTarget.blur();
              }
            }}
          />
          <kbd aria-hidden className="rounded border border-border bg-background px-1.5 text-[10px] font-medium">
            Ctrl K
          </kbd>
        </label>
        {showing && (
          <div data-side="bottom" className="pop absolute left-1/2 top-full z-50 mt-1.5 w-[30rem] -translate-x-1/2 overflow-hidden rounded-lg border border-border bg-popover text-sm shadow-xl">
            <div ref={list} id="odm-palette" role="listbox" aria-label={t("Commands")} tabIndex={-1} className="max-h-80 overflow-y-auto p-1">
              {items.map((c, i) => (
                <div key={c.id} role="presentation">
                  {c.group && c.group !== items[i - 1]?.group && (
                    <p role="presentation" className="px-2 pb-1 pt-2 text-xs font-medium text-muted-foreground">
                      {c.group}
                    </p>
                  )}
                  <div
                    id={`odm-palette-${i}`}
                    data-at={i}
                    role="option"
                    aria-selected={i === at}
                    onMouseDown={(e) => e.preventDefault()} // the box keeps the focus
                    onMouseMove={() => setAt(i)}
                    onClick={() => run(c)}
                    className={cn("flex h-8 cursor-default items-center gap-2 rounded-md px-2", i === at && "bg-accent")}
                  >
                    {c.icon && <c.icon className="size-4 shrink-0 text-muted-foreground" />}
                    <span className="min-w-0 flex-1 truncate">
                      {c.label}
                      {c.id === "download" && <span className="ml-2 text-muted-foreground">{url}</span>}
                    </span>
                    {c.shortcut && <span className="text-xs text-muted-foreground">{c.shortcut}</span>}
                  </div>
                </div>
              ))}
            </div>
            <div className="flex items-center gap-3 whitespace-nowrap border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
              <span>↑↓ {t("to choose")}</span>
              <span className="flex items-center gap-1">
                <CornerDownLeft className="size-3" /> {t("to run")}
              </span>
              <span>Esc {t("to close")}</span>
            </div>
          </div>
        )}
      </div>
      <div className="flex-1" {...caption} />
      <div className={cn("flex", framed && "hidden")}>
        {(
          [
            ["minimize", t("Minimize"), Minus, () => Window.Minimise()],
            ["maximize", t("Maximize"), Square, () => Window.ToggleMaximise()],
            ["close", t("Close"), X, () => Window.Close()],
          ] as const
        ).map(([kind, name, I, f]) => (
          <button
            key={kind}
            aria-label={name}
            tabIndex={-1}
            onClick={call(f)}
            className={cn(
              "flex w-11 items-center justify-center text-muted-foreground",
              kind === "close" ? "hover:bg-red-600 hover:text-white" : "hover:bg-accent hover:text-foreground",
            )}
          >
            <I className={kind === "maximize" ? "size-3" : "size-4"} />
          </button>
        ))}
      </div>
    </header>
  );
}
