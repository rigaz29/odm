// The download list: a virtualized Fluent DataGrid (only the rows in view
// exist, so a thousand downloads scroll as smoothly as ten), sortable and
// with resizable columns. Selection works as in Explorer and is the
// page's: a press selects, Ctrl toggles, Shift selects a range; a right
// click on a row outside the selection selects it before the menu opens.

import {
  DataGrid,
  DataGridBody,
  DataGridCell,
  DataGridHeader,
  DataGridHeaderCell,
  DataGridRow,
} from "@fluentui-contrib/react-data-grid-react-window";
import {
  Menu,
  MenuDivider,
  MenuItem,
  MenuList,
  MenuPopover,
  ProgressBar,
  TableCellLayout,
  type TableColumnDefinition,
  type TableColumnSizingOptions,
  type SortDirection,
  createTableColumn,
  makeStyles,
  mergeClasses,
  tokens,
} from "@fluentui/react-components";
import { DeleteRegular, FolderOpenRegular, OpenRegular, PauseRegular, PlayRegular } from "@fluentui/react-icons";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Item } from "../bindings/odm/internal/fluent/models";
import { displayName, etaSeconds, etaText, humanBytes, lastTry, rateText, statusText } from "../format";
import { CategoryIcon } from "../icons";
import { t } from "../i18n";

export const ROW_HEIGHT = 40;
const HEADER_HEIGHT = 36;

export type SortState = { sortColumn: string | undefined; sortDirection: SortDirection };

const statusRank: Record<string, number> = { downloading: 0, queued: 1, paused: 2, error: 3, complete: 4 };

// compareBy orders the list by a column; App sorts with it, so it knows
// the order a Shift-click selects across.
export const compareBy: Record<string, (a: Item, b: Item) => number> = {
  name: (a, b) => displayName(a).localeCompare(displayName(b), undefined, { numeric: true, sensitivity: "base" }),
  size: (a, b) => a.total - b.total,
  status: (a, b) => (statusRank[a.status] ?? 9) - (statusRank[b.status] ?? 9) || a.progress - b.progress,
  eta: (a, b) => etaSeconds(a) - etaSeconds(b),
  rate: (a, b) => a.speed - b.speed,
  lasttry: (a, b) => Date.parse(a.updatedAt) - Date.parse(b.updatedAt),
  queue: (a, b) => a.queue.localeCompare(b.queue),
  description: (a, b) => a.description.localeCompare(b.description),
};

const useStyles = makeStyles({
  wrap: { height: "100%", outline: "none", position: "relative" },
  grid: { minWidth: "100%" },
  header: { backgroundColor: "transparent" },
  headerCell: {
    fontWeight: tokens.fontWeightSemibold,
    color: tokens.colorNeutralForeground2,
    whiteSpace: "nowrap",
    overflow: "hidden",
  },
  row: {
    cursor: "default",
    borderBottom: "none",
    borderRadius: tokens.borderRadiusMedium,
    // Windows 11 lists have a little room around each row.
    boxSizing: "border-box",
  },
  selected: {
    backgroundColor: tokens.colorSubtleBackgroundSelected,
    ":hover": { backgroundColor: tokens.colorSubtleBackgroundHover },
    // The accent pill Windows 11 lists put at the start of a selected row.
    "::before": {
      content: '""',
      position: "absolute",
      left: "2px",
      top: "50%",
      transform: "translateY(-50%)",
      width: "3px",
      height: "16px",
      borderRadius: "2px",
      backgroundColor: tokens.colorBrandBackground,
    },
  },
  status: { display: "flex", flexDirection: "column", gap: "3px", minWidth: 0, width: "100%" },
  ellipsis: { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  dim: { color: tokens.colorNeutralForeground3 },
  empty: {
    position: "absolute",
    inset: `${HEADER_HEIGHT}px 0 0 0`,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    color: tokens.colorNeutralForeground3,
  },
});

function StatusCell({ d }: { d: Item }) {
  const s = useStyles();
  const bar = d.status !== "complete" && d.total > 0 && !d.live;
  const color = d.status === "error" ? "error" : d.status === "paused" ? "warning" : "brand";
  const text = statusText(d);
  return (
    <div className={s.status} title={text}>
      <span className={s.ellipsis}>{text}</span>
      {bar ? <ProgressBar value={Math.min(1, d.progress / 100)} thickness="medium" shape="rounded" color={color} /> : null}
    </div>
  );
}

function columns(): TableColumnDefinition<Item>[] {
  const text = (id: string, header: string, cell: (d: Item) => string, dim = false) =>
    createTableColumn<Item>({
      columnId: id,
      compare: compareBy[id],
      renderHeaderCell: () => header,
      renderCell: (d) => (
        <span className={dim ? "odm-dim" : undefined} style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {cell(d)}
        </span>
      ),
    });
  return [
    createTableColumn<Item>({
      columnId: "name",
      compare: compareBy.name,
      renderHeaderCell: () => t("File Name"),
      renderCell: (d) => (
        <TableCellLayout truncate media={<CategoryIcon category={d.category} />} title={displayName(d)}>
          {displayName(d)}
        </TableCellLayout>
      ),
    }),
    text("size", t("Size"), (d) => humanBytes(d.total)),
    createTableColumn<Item>({
      columnId: "status",
      compare: compareBy.status,
      renderHeaderCell: () => t("Status"),
      renderCell: (d) => <StatusCell d={d} />,
    }),
    text("eta", t("Time left"), etaText),
    text("rate", t("Transfer rate"), rateText),
    text("lasttry", t("Last attempt"), (d) => lastTry(d.updatedAt), true),
    text("queue", t("Queue"), (d) => d.queue, true),
    text("description", t("Description"), (d) => d.description, true),
  ];
}

const others: TableColumnSizingOptions = {
  size: { minWidth: 70, idealWidth: 85, defaultWidth: 85 },
  status: { minWidth: 120, idealWidth: 150, defaultWidth: 150 },
  eta: { minWidth: 70, idealWidth: 85, defaultWidth: 85 },
  rate: { minWidth: 80, idealWidth: 100, defaultWidth: 100 },
  lasttry: { minWidth: 90, idealWidth: 110, defaultWidth: 110 },
  queue: { minWidth: 56, idealWidth: 64, defaultWidth: 64 },
  description: { minWidth: 80, idealWidth: 100, defaultWidth: 100 },
};

// sizing gives the name column what the others leave of width, as the
// Fyne list does.
function sizing(width: number): TableColumnSizingOptions {
  const rest = Object.values(others).reduce((n, o) => n + (o.idealWidth ?? 0), 0);
  // Room for the vertical scrollbar and the cells' borders.
  const name = Math.max(200, Math.floor(width - rest - 32));
  return { name: { minWidth: 200, idealWidth: name, defaultWidth: name }, ...others };
}

export function DownloadList(props: {
  items: Item[]; // filtered and sorted
  selected: Set<string>;
  sort: SortState;
  onSort: (s: SortState) => void;
  onPress: (id: string, mods: { ctrl: boolean; shift: boolean }) => void;
  onContext: (id: string) => void;
  onOpen: (d: Item) => void;
  onAction: (a: "open" | "folder" | "resume" | "pause" | "remove") => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  canOpen: boolean;
  canResume: boolean;
  canPause: boolean;
  emptyText: string;
}) {
  const s = useStyles();
  const wrap = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(400);
  const [width, setWidth] = useState(900);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);

  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      setHeight(Math.max(0, el.clientHeight - HEADER_HEIGHT));
      setWidth(el.clientWidth);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const columnSizing = useMemo(() => sizing(width), [width]);
  const cols = useMemo(() => columns(), []);

  const target = menu
    ? { getBoundingClientRect: () => ({ x: menu.x, y: menu.y, left: menu.x, top: menu.y, right: menu.x, bottom: menu.y, width: 0, height: 0 }) as DOMRect }
    : undefined;

  return (
    <div ref={wrap} className={s.wrap} tabIndex={0} onKeyDown={props.onKeyDown} data-odm-list>
      <DataGrid
        className={s.grid}
        items={props.items}
        columns={cols}
        getRowId={(d: Item) => d.id}
        sortable
        sortState={props.sort}
        onSortChange={(_, st) => props.onSort(st as SortState)}
        resizableColumns
        columnSizingOptions={columnSizing}
        size="medium"
      >
        <DataGridHeader className={s.header}>
          <DataGridRow>
            {({ renderHeaderCell }) => <DataGridHeaderCell className={s.headerCell}>{renderHeaderCell()}</DataGridHeaderCell>}
          </DataGridRow>
        </DataGridHeader>
        {/* Rows fill the width, the vertical scrollbar included: nothing to
            scroll sideways. */}
        <DataGridBody<Item> itemSize={ROW_HEIGHT} height={height} listProps={{ style: { overflowX: "hidden" } }}>
          {({ item, rowId }, style) => (
            <DataGridRow<Item>
              key={rowId}
              style={style}
              className={mergeClasses(s.row, props.selected.has(item.id) && s.selected)}
              aria-selected={props.selected.has(item.id)}
              onMouseDown={(e: React.MouseEvent) => {
                if (e.button !== 0) return;
                wrap.current?.focus();
                props.onPress(item.id, { ctrl: e.ctrlKey || e.metaKey, shift: e.shiftKey });
              }}
              onDoubleClick={() => props.onOpen(item)}
              onContextMenu={(e: React.MouseEvent) => {
                e.preventDefault();
                wrap.current?.focus();
                props.onContext(item.id);
                setMenu({ x: e.clientX, y: e.clientY });
              }}
            >
              {({ renderCell }) => <DataGridCell>{renderCell(item)}</DataGridCell>}
            </DataGridRow>
          )}
        </DataGridBody>
      </DataGrid>
      {props.items.length === 0 ? <div className={s.empty}>{props.emptyText}</div> : null}
      <Menu
        open={menu !== null}
        onOpenChange={(_, d) => {
          if (!d.open) setMenu(null);
        }}
        positioning={{ target, position: "below", align: "start" }}
      >
        <MenuPopover>
          <MenuList>
            <MenuItem icon={<OpenRegular />} disabled={!props.canOpen} onClick={() => props.onAction("open")}>
              {t("Open file")}
            </MenuItem>
            <MenuItem icon={<FolderOpenRegular />} onClick={() => props.onAction("folder")}>
              {t("Open folder")}
            </MenuItem>
            <MenuDivider />
            <MenuItem icon={<PlayRegular />} disabled={!props.canResume} onClick={() => props.onAction("resume")}>
              {t("Resume")}
            </MenuItem>
            <MenuItem icon={<PauseRegular />} disabled={!props.canPause} onClick={() => props.onAction("pause")}>
              {t("Stop")}
            </MenuItem>
            <MenuDivider />
            <MenuItem icon={<DeleteRegular />} onClick={() => props.onAction("remove")}>
              {t("Remove from list")}
            </MenuItem>
          </MenuList>
        </MenuPopover>
      </Menu>
    </div>
  );
}
