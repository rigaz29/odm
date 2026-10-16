// The toolbar's "more" menu: the rest of the Fyne UI's Downloads, View and
// Help menus.

import {
  Ban,
  CircleHelp,
  CirclePause,
  CirclePlay,
  Columns3,
  Eraser,
  FileDown,
  FileUp,
  Gauge,
  Globe,
  Info,
  Keyboard,
  ListPlus,
  MoreHorizontal,
  PanelLeft,
  RefreshCw,
} from "lucide-react";
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
  on: {
    newQueue(): void;
    grab(): void;
    exportList(): void;
    importList(): void;
    shortcuts(): void;
    checkUpdate(): void;
    about(): void;
  };
}) {
  const { on } = props;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("More actions")} title={t("More actions")}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-64">
        <DropdownMenuItem onSelect={on.newQueue}>
          <ListPlus /> {t("New queue…")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={on.grab}>
          <Globe /> {t("Grab site…")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
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
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <PanelLeft /> {t("View")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuCheckboxItem checked={props.sidebar} onCheckedChange={(v) => props.onSidebar(v === true)}>
              <PanelLeft /> {t("Show sidebar")}
            </DropdownMenuCheckboxItem>
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <Columns3 /> {t("Columns")}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <ColumnItems menu="dropdown" columns={props.columns} onColumns={props.onColumns} />
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={on.exportList}>
          <FileUp /> {t("Export download list…")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={on.importList}>
          <FileDown /> {t("Import download list…")}
        </DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <CircleHelp /> {t("Help")}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem onSelect={on.shortcuts}>
              <Keyboard /> {t("Keyboard shortcuts")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={on.checkUpdate}>
              <RefreshCw /> {t("Check for updates…")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={on.about}>
              <Info /> {t("About ODM")}
            </DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
