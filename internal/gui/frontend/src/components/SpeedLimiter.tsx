// The speed limiter: one limit for all downloads together, as the Fyne
// UI's (internal/ui/limiter_desktop.go). The status bar opens it; the
// switch turns the limit on or off, a preset or a typed speed sets it and
// turns it on. ODM keeps the last limit while it is off.

import { Gauge } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { speedText } from "../format";
import { t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";
import { Switch } from "./ui/switch";

const KB = 1024;
const MB = 1024 * KB;
export const PRESETS = [256 * KB, 512 * KB, 1 * MB, 2 * MB, 5 * MB, 10 * MB];

// preset names a preset as people say it: "512 KB/s", "2 MB/s".
export const preset = (bps: number) => (bps % MB === 0 ? `${bps / MB} MB/s` : `${bps / KB} KB/s`);

// The speed the switch turns on when no limit was ever set; the Fyne UI
// offers the same.
const FIRST_LIMIT = 500 * KB;

export function SpeedLimiter(props: {
  limit: number; // in force, 0 for none
  saved: number; // kept while off, 0 when never set
  onSet: (on: boolean, bytesPerSecond: number) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode; // the trigger
}) {
  const on = props.limit > 0;
  const current = on ? props.limit : props.saved;
  const [kb, setKb] = useState("");
  const [error, setError] = useState(false);

  useEffect(() => {
    if (props.open) {
      setKb(String(Math.round((current || FIRST_LIMIT) / KB)));
      setError(false);
    }
  }, [props.open, current]);

  const apply = () => {
    const v = Math.floor(Number(kb.trim().replace(",", ".")));
    if (!Number.isFinite(v) || v <= 0) {
      setError(true);
      return;
    }
    props.onSet(true, v * KB);
  };

  return (
    <Popover open={props.open} onOpenChange={props.onOpenChange}>
      <PopoverTrigger asChild>{props.children}</PopoverTrigger>
      <PopoverContent align="end" side="top" className="w-80 space-y-3">
        <div className="flex items-center gap-2">
          <Gauge className="size-4 text-muted-foreground" />
          <p className="flex-1 font-medium">{t("Speed limiter")}</p>
          <Switch
            checked={on}
            aria-label={t("Speed limiter")}
            onCheckedChange={(v) => props.onSet(v, v && props.saved <= 0 ? FIRST_LIMIT : 0)}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          {on ? tf("All downloads together run at %s at most.", speedText(props.limit)) : t("Downloads run as fast as they can.")}
        </p>
        <div className="grid grid-cols-3 gap-1.5">
          {PRESETS.map((p) => (
            <button
              key={p}
              onClick={() => props.onSet(true, p)}
              className={cn(
                "h-8 rounded-md border text-xs font-medium tabular-nums outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring/50",
                on && props.limit === p ? "border-violet-500/60 bg-violet-500/10 text-violet-700 dark:text-violet-300" : "border-border",
              )}
            >
              {preset(p)}
            </button>
          ))}
        </div>
        <form
          className="space-y-1"
          onSubmit={(e) => {
            e.preventDefault();
            apply();
          }}
        >
          <label className="text-xs text-muted-foreground" htmlFor="odm-limit">
            {t("Maximum speed (KB/s)")}
          </label>
          <div className="flex gap-2">
            <input
              id="odm-limit"
              inputMode="numeric"
              value={kb}
              onChange={(e) => {
                setKb(e.target.value);
                setError(false);
              }}
              className="h-8 min-w-0 flex-1 rounded-md border border-input bg-background px-2.5 text-sm tabular-nums outline-none focus:border-ring/70 focus:ring-2 focus:ring-ring/25"
            />
            <Button type="submit" variant="brand">
              {t("Turn on")}
            </Button>
          </div>
          {error && <p className="text-xs text-red-600 dark:text-red-400">{t("Enter a speed above 0 KB/s.")}</p>}
        </form>
      </PopoverContent>
    </Popover>
  );
}
