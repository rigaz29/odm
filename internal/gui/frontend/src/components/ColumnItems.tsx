// The column menu's items: a check for each column but Name, and Reset
// columns. The list's header shows them as its context menu and behind
// its button, the toolbar's "more" menu as a submenu.

import { COLUMNS, type ColumnState, defaultColumns, toggleColumn } from "../columns";
import { t } from "../i18n";
import { ContextMenuCheckboxItem, ContextMenuItem, ContextMenuSeparator } from "./ui/context-menu";
import { DropdownMenuCheckboxItem, DropdownMenuItem, DropdownMenuSeparator } from "./ui/dropdown-menu";

const kinds = {
  context: [ContextMenuCheckboxItem, ContextMenuItem, ContextMenuSeparator],
  dropdown: [DropdownMenuCheckboxItem, DropdownMenuItem, DropdownMenuSeparator],
} as const;

export function ColumnItems({ menu, columns, onColumns }: { menu: keyof typeof kinds; columns: ColumnState; onColumns: (s: ColumnState) => void }) {
  const [Check, Plain, Sep] = kinds[menu];
  return (
    <>
      {COLUMNS.filter((c) => c.id !== "name").map((c) => (
        <Check
          key={c.id}
          checked={columns.shown.includes(c.id)}
          onSelect={(e: Event) => e.preventDefault()} // the menu stays open for the next
          onCheckedChange={() => onColumns(toggleColumn(columns, c.id))}
        >
          {t(c.label)}
        </Check>
      ))}
      <Sep />
      <Plain onSelect={() => onColumns(defaultColumns())}>{t("Reset columns")}</Plain>
    </>
  );
}
