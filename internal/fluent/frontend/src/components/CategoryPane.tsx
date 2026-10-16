// The navigation pane: all downloads, each category, unfinished and
// finished ones, and the queues, with how many each holds. It sits on the
// window's Mica, as Windows 11's NavigationView does.

import {
  CounterBadge,
  NavCategory,
  NavCategoryItem,
  NavDrawer,
  NavDrawerBody,
  NavItem,
  NavSectionHeader,
  NavSubItem,
  NavSubItemGroup,
  makeStyles,
  tokens,
} from "@fluentui/react-components";
import {
  ArrowDownloadRegular,
  CheckmarkCircleRegular,
  ClockRegular,
  TextBulletListLtrRegular,
} from "@fluentui/react-icons";
import type { Filter } from "../format";
import { CategoryIcon } from "../icons";
import { t } from "../i18n";

const useStyles = makeStyles({
  drawer: {
    backgroundColor: "transparent",
    width: "100%",
    height: "100%",
    borderRight: "none",
    // Fluent's Nav fills its items with neutral background 4; on Mica they
    // are see-through, with Windows' subtle fills on hover and press.
    "--colorNeutralBackground4": "transparent",
    "--colorNeutralBackground4Hover": tokens.colorSubtleBackgroundHover,
    "--colorNeutralBackground4Pressed": tokens.colorSubtleBackgroundPressed,
  },
  body: { paddingTop: tokens.spacingVerticalXS },
  // Windows 11's navigation rows are about 36px high.
  item: { minHeight: "36px", paddingTop: "6px", paddingBottom: "6px" },
  label: { display: "flex", alignItems: "center", gap: tokens.spacingHorizontalS, width: "100%" },
  name: { flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
});

export function filterKey(f: Filter): string {
  return "value" in f ? `${f.kind}:${f.value}` : f.kind;
}

function parseKey(key: string): Filter {
  const [kind, ...rest] = key.split(":");
  const value = rest.join(":");
  switch (kind) {
    case "category":
      return { kind, value };
    case "queue":
      return { kind, value };
    case "unfinished":
    case "finished":
      return { kind };
  }
  return { kind: "all" };
}

export function CategoryPane(props: {
  filter: Filter;
  onFilter: (f: Filter) => void;
  categories: string[];
  queues: string[];
  counts: Map<string, number>;
  total: number;
  unfinished: number;
  finished: number;
}) {
  const s = useStyles();
  const label = (text: string, n?: number) => (
    <span className={s.label}>
      <span className={s.name}>{text}</span>
      {n ? <CounterBadge count={n} overflowCount={9999} appearance="ghost" color="informative" size="small" /> : null}
    </span>
  );
  return (
    <NavDrawer
      className={s.drawer}
      type="inline"
      size="small"
      open
      selectedValue={filterKey(props.filter)}
      defaultOpenCategories={["queues"]}
      onNavItemSelect={(_, d) => props.onFilter(parseKey(String(d.value)))}
    >
      <NavDrawerBody className={s.body}>
        <NavItem className={s.item} value="all" icon={<ArrowDownloadRegular />}>
          {label(t("All Downloads"), props.total)}
        </NavItem>
        <NavItem className={s.item} value="unfinished" icon={<ClockRegular />}>
          {label(t("Unfinished"), props.unfinished)}
        </NavItem>
        <NavItem className={s.item} value="finished" icon={<CheckmarkCircleRegular />}>
          {label(t("Finished"), props.finished)}
        </NavItem>
        <NavSectionHeader>{t("Categories")}</NavSectionHeader>
        {props.categories.map((c) => (
          <NavItem className={s.item} key={c} value={`category:${c}`} icon={<CategoryIcon category={c} />}>
            {label(c, props.counts.get(c))}
          </NavItem>
        ))}
        <NavCategory value="queues">
          <NavCategoryItem className={s.item} icon={<TextBulletListLtrRegular />}>
            {t("Queues")}
          </NavCategoryItem>
          <NavSubItemGroup>
            {props.queues.map((q) => (
              <NavSubItem className={s.item} key={q} value={`queue:${q}`}>
                {q}
              </NavSubItem>
            ))}
          </NavSubItemGroup>
        </NavCategory>
      </NavDrawerBody>
    </NavDrawer>
  );
}
