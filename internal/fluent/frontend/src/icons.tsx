// An icon and colour per download category, as Explorer tints file types.

import type { FluentIcon } from "@fluentui/react-icons";
import {
  AppsRegular,
  DocumentQuestionMarkRegular,
  DocumentRegular,
  FolderZipRegular,
  ImageRegular,
  MusicNote2Regular,
  StorageRegular,
  VideoRegular,
} from "@fluentui/react-icons";

const byCategory: Record<string, [FluentIcon, string]> = {
  Video: [VideoRegular, "#8764b8"],
  Audio: [MusicNote2Regular, "#e3008c"],
  Documents: [DocumentRegular, "#0f6cbd"],
  Programs: [AppsRegular, "#5c6970"],
  Compressed: [FolderZipRegular, "#c19c00"],
  Images: [ImageRegular, "#038387"],
  "Disk Images": [StorageRegular, "#4f6bed"],
};

export function categoryIcon(category: string): [FluentIcon, string] {
  return byCategory[category] ?? [DocumentQuestionMarkRegular, "#7a7574"];
}

export function CategoryIcon({ category, size = 20 }: { category: string; size?: number }) {
  const [Icon, color] = categoryIcon(category);
  return <Icon fontSize={size} style={{ color, flexShrink: 0 }} />;
}
