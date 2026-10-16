// What a ZIP archive holds, read from its end before it is downloaded, as
// the Fyne UI's preview (internal/ui/zip_desktop.go).

import { Folder, Lock } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../api";
import type { ZipListing } from "../bindings/odm/internal/gui/models";
import { humanBytes } from "../format";
import { t, tf } from "../i18n";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";

export function ZipPreview(props: { open: boolean; url: string; token: string; name: string; onClose: () => void }) {
  const [list, setList] = useState<ZipListing | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!props.open) return;
    let live = true;
    setList(null);
    setError("");
    api.zipPreview(props.url, props.token).then(
      (l) => live && setList(l),
      (e) => live && setError(String((e as Error)?.message ?? e)),
    );
    return () => {
      live = false;
    };
  }, [props.open, props.url, props.token]);

  const status = error
    ? error
    : !list
      ? t("Reading the list of contents…")
      : tf(list.files === 1 ? "%d file, %s when extracted · %s read to list them" : "%d files, %s when extracted · %s read to list them", list.files, humanBytes(list.size) || "-", humanBytes(list.fetched) || "-");

  return (
    <Dialog open={props.open} onOpenChange={(o) => !o && props.onClose()}>
      <DialogContent className="max-w-2xl">
        <div className="space-y-1 pr-6">
          <DialogTitle className="truncate">{tf("Contents of %s", props.name)}</DialogTitle>
          <DialogDescription>{status}</DialogDescription>
        </div>
        {list && (
          <div className="max-h-80 overflow-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-muted text-xs text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">{t("Name")}</th>
                  <th className="w-24 px-3 py-2 text-left font-medium">{t("Size")}</th>
                  <th className="w-40 px-3 py-2 text-left font-medium">{t("Modified")}</th>
                </tr>
              </thead>
              <tbody>
                {list.entries.map((e) => (
                  <tr key={e.name} className="border-t border-border">
                    <td className="max-w-0 truncate px-3 py-1.5" title={e.name}>
                      <span className="inline-flex items-center gap-1.5">
                        {e.dir && <Folder className="size-3.5 text-muted-foreground" />}
                        {e.name}
                        {e.encrypted && <Lock className="size-3 text-amber-600" aria-label={t("encrypted")} />}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-3 py-1.5 tabular-nums text-muted-foreground">{e.dir ? t("folder") : humanBytes(e.size)}</td>
                    <td className="whitespace-nowrap px-3 py-1.5 tabular-nums text-muted-foreground">{modified(e.modified)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function modified(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 1981) return "";
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
