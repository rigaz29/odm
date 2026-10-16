// The toolbar's "more" menu: what the Fyne UI's Downloads and View menus
// do that needs no window of its own.

import { Ban, CirclePause, CirclePlay, Columns3, Eraser, Gauge, MoreHorizontal, PanelLeft } from "lucide-react";
import type { ColumnState } from "../columns";
import { t } from "../i18n";
import { ColumnItems } from "./ColumnItems";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";

export function MoreMenu(props: {
  onResumeAll: () => void;
  onPauseAll: () => void;
  onStopAll: () => void;
  onClearCompleted: () => void;
  limiter: { on: boolean; label: string; onToggle: () => void };
  columns: ColumnState;
  onColumns: (s: ColumnState) => void;
  sidebar: boolean;
  onSidebar: (shown: boolean) => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("More actions")} title={t("More actions")}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-60">
        <DropdownMenuItem onSelect={props.onResumeAll}>
          <CirclePlay /> {t("Resume all")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={props.onPauseAll}>
          <CirclePause /> {t("Pause all")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={props.onStopAll}>
          <Ban /> {t("Stop all (and stop queues)")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={props.onClearCompleted}>
          <Eraser /> {t("Clear completed")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem checked={props.limiter.on} onCheckedChange={props.limiter.onToggle}>
          <Gauge /> {props.limiter.label}
        </DropdownMenuCheckboxItem>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Columns3 /> {t("Columns")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <ColumnItems menu="dropdown" columns={props.columns} onColumns={props.onColumns} />
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuCheckboxItem checked={props.sidebar} onCheckedChange={(v) => props.onSidebar(v === true)}>
          <PanelLeft /> {t("Show sidebar")}
        </DropdownMenuCheckboxItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
