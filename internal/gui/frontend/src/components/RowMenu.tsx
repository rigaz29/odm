// The list's context menu, for the selected downloads, as the Fyne UI's
// row menu.

import { Activity, ArrowDown, ArrowUp, Copy, FolderOpen, Link2, ListOrdered, Pause, Play, SquareArrowOutUpRight, Trash2, X } from "lucide-react";
import type { Item } from "../bindings/odm/internal/gui/models";
import { t } from "../i18n";
import {
  ContextMenuCheckboxItem,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from "./ui/context-menu";

export interface RowActions {
  open(): void;
  progress(): void;
  readdress(): void;
  folder(): void;
  copy(): void;
  resume(): void;
  pause(): void;
  moveTo(queue: string): void;
  move(delta: number): void; // in the queue's order
  remove(withFiles: boolean): void;
}

export function RowMenu({ chosen, current, queues, can, on }: { chosen: Item[]; current?: Item; queues: string[]; can: { open: boolean; resume: boolean; pause: boolean }; on: RowActions }) {
  const one = chosen.length === 1 && chosen[0].status !== "complete";
  // Only an unfinished, stopped download takes a new address: the engine
  // refuses others.
  const readdress = !!current && !["complete", "downloading", "queued"].includes(current.status);
  return (
    <>
      <ContextMenuItem disabled={!current || current.status === "complete"} onSelect={on.progress}>
        <Activity /> {t("Show progress")}
        {current?.status !== "complete" && <ContextMenuShortcut>Enter</ContextMenuShortcut>}
      </ContextMenuItem>
      <ContextMenuItem disabled={!can.open} onSelect={on.open}>
        <SquareArrowOutUpRight /> {t("Open file")}
        {can.open && <ContextMenuShortcut>Enter</ContextMenuShortcut>}
      </ContextMenuItem>
      <ContextMenuItem disabled={chosen.length === 0} onSelect={on.folder}>
        <FolderOpen /> {t("Show in folder")}
      </ContextMenuItem>
      <ContextMenuItem disabled={chosen.length === 0} onSelect={on.copy}>
        <Copy /> {t("Copy address")}
        <ContextMenuShortcut>Ctrl+C</ContextMenuShortcut>
      </ContextMenuItem>
      <ContextMenuItem disabled={!readdress} onSelect={on.readdress}>
        <Link2 /> {t("Refresh download address…")}
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem disabled={!can.resume} onSelect={on.resume}>
        <Play /> {t("Resume")}
        {can.resume && !can.pause && <ContextMenuShortcut>Space</ContextMenuShortcut>}
      </ContextMenuItem>
      <ContextMenuItem disabled={!can.pause} onSelect={on.pause}>
        <Pause /> {t("Pause")}
        {can.pause && <ContextMenuShortcut>Space</ContextMenuShortcut>}
      </ContextMenuItem>
      <ContextMenuSub>
        <ContextMenuSubTrigger disabled={chosen.length === 0}>
          <ListOrdered /> {t("Move to queue")}
        </ContextMenuSubTrigger>
        <ContextMenuSubContent>
          {queues.map((q) => (
            <ContextMenuCheckboxItem key={q} checked={chosen.length > 0 && chosen.every((d) => d.queue === q)} onCheckedChange={() => on.moveTo(q)}>
              {q}
            </ContextMenuCheckboxItem>
          ))}
        </ContextMenuSubContent>
      </ContextMenuSub>
      <ContextMenuItem disabled={!one} onSelect={() => on.move(-1)}>
        <ArrowUp /> {t("Move up in queue")}
      </ContextMenuItem>
      <ContextMenuItem disabled={!one} onSelect={() => on.move(1)}>
        <ArrowDown /> {t("Move down in queue")}
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem disabled={chosen.length === 0} onSelect={() => on.remove(false)}>
        <X /> {t("Remove from list")}
        <ContextMenuShortcut>Del</ContextMenuShortcut>
      </ContextMenuItem>
      <ContextMenuItem disabled={chosen.length === 0} onSelect={() => on.remove(true)} className="text-red-600 dark:text-red-400 [&_svg]:text-current">
        <Trash2 /> {t("Remove + delete file")}
        <ContextMenuShortcut>Shift+Del</ContextMenuShortcut>
      </ContextMenuItem>
    </>
  );
}
