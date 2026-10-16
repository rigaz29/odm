// The command bar under the title bar, as in Windows 11's File Explorer:
// the main actions with their names, the others as icons with tooltips,
// and the search box at the end.

import {
  SearchBox,
  Toolbar,
  ToolbarButton,
  ToolbarDivider,
  Tooltip,
  makeStyles,
  tokens,
} from "@fluentui/react-components";
import {
  AddRegular,
  BroomRegular,
  CalendarClockRegular,
  DeleteRegular,
  FolderOpenRegular,
  PauseRegular,
  PlayCircleRegular,
  PlayRegular,
  RecordStopRegular,
  SettingsRegular,
  StopRegular,
} from "@fluentui/react-icons";
import type { RefObject } from "react";
import { t } from "../i18n";

const useStyles = makeStyles({
  bar: {
    display: "flex",
    alignItems: "center",
    gap: tokens.spacingHorizontalS,
    padding: `${tokens.spacingVerticalXS} ${tokens.spacingHorizontalM}`,
    minWidth: 0,
  },
  toolbar: { flex: 1, minWidth: 0, overflow: "hidden", padding: 0 },
  search: { width: "260px", flexShrink: 0 },
});

export type Command =
  | "add"
  | "resume"
  | "pause"
  | "pauseAll"
  | "remove"
  | "clean"
  | "startQueue"
  | "stopQueue"
  | "folder";

export function CommandBar(props: {
  canResume: boolean;
  canPause: boolean;
  anyRunning: boolean;
  hasSelection: boolean;
  hasFinished: boolean;
  queueRunning: boolean;
  canShowFolder: boolean;
  query: string;
  searchRef: RefObject<HTMLInputElement | null>;
  onQuery: (q: string) => void;
  onCommand: (c: Command) => void;
}) {
  const s = useStyles();
  const later = t("Comes in stage 3 of the Fluent UI");
  const icon = (label: string, el: React.JSX.Element) => (
    <Tooltip content={label} relationship="label" withArrow>
      {el}
    </Tooltip>
  );
  return (
    <div className={s.bar}>
      <Toolbar className={s.toolbar} aria-label="ODM">
        <ToolbarButton appearance="primary" icon={<AddRegular />} onClick={() => props.onCommand("add")}>
          {t("Add URL")}
        </ToolbarButton>
        <ToolbarButton icon={<PlayRegular />} disabled={!props.canResume} onClick={() => props.onCommand("resume")}>
          {t("Resume")}
        </ToolbarButton>
        <ToolbarButton icon={<PauseRegular />} disabled={!props.canPause} onClick={() => props.onCommand("pause")}>
          {t("Stop")}
        </ToolbarButton>
        <ToolbarButton icon={<RecordStopRegular />} disabled={!props.anyRunning} onClick={() => props.onCommand("pauseAll")}>
          {t("Stop All")}
        </ToolbarButton>
        {icon(t("Delete"), (
          <ToolbarButton aria-label={t("Delete")} icon={<DeleteRegular />} disabled={!props.hasSelection} onClick={() => props.onCommand("remove")} />
        ))}
        {icon(t("Clean up"), (
          <ToolbarButton aria-label={t("Clean up")} icon={<BroomRegular />} disabled={!props.hasFinished} onClick={() => props.onCommand("clean")} />
        ))}
        <ToolbarDivider />
        {props.queueRunning ? (
          <ToolbarButton icon={<StopRegular />} onClick={() => props.onCommand("stopQueue")}>
            {t("Stop Queue")}
          </ToolbarButton>
        ) : (
          <ToolbarButton icon={<PlayCircleRegular />} onClick={() => props.onCommand("startQueue")}>
            {t("Start Queue")}
          </ToolbarButton>
        )}
        <ToolbarDivider />
        {icon(t("Open folder"), (
          <ToolbarButton aria-label={t("Open folder")} icon={<FolderOpenRegular />} disabled={!props.canShowFolder} onClick={() => props.onCommand("folder")} />
        ))}
        {icon(`${t("Options")} · ${later}`, <ToolbarButton aria-label={t("Options")} icon={<SettingsRegular />} disabledFocusable />)}
        {icon(`${t("Scheduler")} · ${later}`, <ToolbarButton aria-label={t("Scheduler")} icon={<CalendarClockRegular />} disabledFocusable />)}
      </Toolbar>
      <SearchBox
        className={s.search}
        input={{ ref: props.searchRef }}
        placeholder={t("Search downloads (Ctrl+F)")}
        value={props.query}
        onChange={(_, d) => props.onQuery(d.value)}
      />
    </div>
  );
}
