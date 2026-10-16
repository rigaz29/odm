// The download complete window (?view=complete&id=…), as the Fyne UI's
// (internal/ui/dialogs_desktop.go, showComplete): what was saved where,
// with Open, Open folder and Close.

import { CheckCircle2, FolderOpen, SquareArrowOutUpRight } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import type { Detail } from "../bindings/odm/internal/gui/models";
import { Button } from "../components/ui/button";
import { Checkbox } from "../components/ui/checkbox";
import { displayName, humanBytes } from "../format";
import { t, tf } from "../i18n";

export function CompleteWindow({ id }: { id: string }) {
  const [d, setD] = useState<Detail | null>(null);
  const [again, setAgain] = useState(false);

  useEffect(() => {
    api.setTitle(t("Download complete"));
    api.detail(id).then(setD, () => api.closeWindow());
  }, [id]);

  if (!d) return <div className="h-full bg-background" />;
  const rows: [string, string][] = [
    [t("Address"), d.item.url],
    [t("File size"), humanBytes(d.item.total) || "-"],
    [t("Saved to"), d.path],
    [t("Checksum"), d.checksumName ? tf("%s matches", d.checksumName) : t("Not checked")],
  ];
  const done = (f?: () => Promise<void>) => () => {
    if (f) void f();
    api.closeWindow();
  };

  return (
    <main className="aurora flex h-full flex-col gap-4 bg-background p-5 text-foreground">
      <div className="flex items-center gap-2.5">
        <CheckCircle2 className="size-6 text-emerald-500" />
        <div className="min-w-0">
          <h1 className="text-base font-semibold tracking-tight">{t("Download complete")}</h1>
          <p className="truncate text-sm text-muted-foreground" title={displayName(d.item)}>
            {displayName(d.item)}
          </p>
        </div>
      </div>
      <dl className="grid grid-cols-[6.5rem_1fr] gap-x-3 gap-y-1.5 rounded-lg border border-border bg-card p-3 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-muted-foreground">{k}</dt>
            <dd className="truncate" title={v}>
              {v || "-"}
            </dd>
          </div>
        ))}
      </dl>
      <label className="flex items-center gap-2 text-sm">
        <Checkbox
          checked={again}
          onCheckedChange={(v) => {
            setAgain(v === true);
            void api.setCompleteDialog(v !== true);
          }}
        />
        {t("Don't show this dialog again")}
      </label>
      <div className="mt-auto flex justify-end gap-2">
        <Button variant="brand" autoFocus onClick={done(() => api.openFile(id))}>
          <SquareArrowOutUpRight /> {t("Open")}
        </Button>
        <Button onClick={done(() => api.showInFolder(id))}>
          <FolderOpen /> {t("Open folder")}
        </Button>
        <Button onClick={done()}>{t("Close")}</Button>
      </div>
    </main>
  );
}
