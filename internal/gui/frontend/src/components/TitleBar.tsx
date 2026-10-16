// The title bar the page draws (the window has no frame). Its parts carry
// --wails-non-client-region so Windows handles them as its own: the empty
// areas move the window and maximize it on a double click, the buttons
// minimize, maximize (with the snap layouts) and close. The search box in
// the middle is not one of them, so it takes clicks. For a WebView2
// without non-client regions the page does the same through the runtime:
// the caption is also --wails-draggable and maximizes on a double click,
// and the buttons call the window.

import { Window } from "@wailsio/runtime";
import { ArrowDownToLine, Minus, Search, Square, X } from "lucide-react";
import type { CSSProperties, RefObject } from "react";
import { inODM } from "../api";
import { t } from "../i18n";
import { cn } from "../lib/utils";

const region = (kind: "caption" | "minimize" | "maximize" | "close") => ({ "--wails-non-client-region": kind }) as CSSProperties;

export function TitleBar(props: {
  query: string;
  onQuery: (q: string) => void;
  onSubmit: () => void;
  searchRef: RefObject<HTMLInputElement | null>;
}) {
  const call = (f: () => Promise<void>) => () => {
    if (inODM) void f();
  };
  const caption = {
    style: { ...region("caption"), "--wails-draggable": "drag" } as CSSProperties,
    onDoubleClick: call(() => Window.ToggleMaximise()),
  };
  return (
    <header className="flex h-10 shrink-0 items-stretch border-b border-border">
      <div className="flex flex-1 items-center gap-2 pl-3" {...caption}>
        <span className="accent-gradient flex size-5 items-center justify-center rounded-[5px] text-white shadow-sm shadow-violet-500/30">
          <ArrowDownToLine className="size-3.5" strokeWidth={2.5} />
        </span>
        <span className="text-[13px] font-semibold">ODM</span>
        <span className="truncate text-[13px] text-muted-foreground">Ochi Download Manager</span>
      </div>
      <div className="flex items-center">
        <label className="flex h-7 w-[26rem] items-center gap-2 rounded-md border border-border bg-background/60 px-2.5 text-[13px] text-muted-foreground focus-within:border-ring/70 focus-within:ring-2 focus-within:ring-ring/25">
          <Search className="size-3.5 shrink-0" />
          <input
            ref={props.searchRef}
            className="min-w-0 flex-1 bg-transparent text-foreground outline-none placeholder:text-muted-foreground"
            placeholder={t("Search downloads or paste a URL…")}
            value={props.query}
            onChange={(e) => props.onQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") props.onSubmit();
              if (e.key === "Escape") props.onQuery("");
            }}
          />
          <kbd className="rounded border border-border bg-background px-1.5 text-[10px] font-medium">Ctrl K</kbd>
        </label>
      </div>
      <div className="flex-1" {...caption} />
      <div className="flex">
        {(
          [
            ["minimize", Minus, () => Window.Minimise()],
            ["maximize", Square, () => Window.ToggleMaximise()],
            ["close", X, () => Window.Close()],
          ] as const
        ).map(([kind, I, f]) => (
          <button
            key={kind}
            aria-label={kind}
            tabIndex={-1}
            style={region(kind)}
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
