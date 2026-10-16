// Add URL: the address only, for now. Stage 3 brings the full New Download
// window (probe card, file name, folder, queue, stream qualities).

import { Link } from "lucide-react";
import { useEffect, useState } from "react";
import { t } from "../i18n";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";

export function AddUrlDialog(props: { open: boolean; initial: string; onClose: () => void; onAdd: (url: string) => Promise<void> }) {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (props.open) {
      setUrl(props.initial);
      setError("");
    }
  }, [props.open, props.initial]);

  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      await props.onAdd(url.trim());
      props.onClose();
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={(o) => !o && props.onClose()}>
      <DialogContent>
        <div className="space-y-1">
          <DialogTitle>{t("New download")}</DialogTitle>
          <DialogDescription>{t("Paste the address of a file, a stream or a video page.")}</DialogDescription>
        </div>
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-1.5">
            <label className="flex h-9 items-center gap-2 rounded-md border border-input bg-background px-2.5 text-sm focus-within:border-ring/70 focus-within:ring-2 focus-within:ring-ring/25">
              <Link className="size-4 text-muted-foreground" />
              <input
                autoFocus
                className="min-w-0 flex-1 bg-transparent outline-none placeholder:text-muted-foreground"
                placeholder="https://"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
              />
            </label>
            {error && <p className="text-xs text-red-600 dark:text-red-400">{error}</p>}
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={props.onClose}>
              {t("Cancel")}
            </Button>
            <Button type="submit" variant="brand" disabled={busy || !url.trim()}>
              {t("Start")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
