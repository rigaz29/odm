// Add URL: the address only, for now. Stage 3 brings the full New Download
// window (probe card, file name, folder, queue, stream qualities).

import {
  Button,
  Dialog,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogSurface,
  DialogTitle,
  Field,
  Input,
} from "@fluentui/react-components";
import { LinkRegular } from "@fluentui/react-icons";
import { useState } from "react";
import { t } from "../i18n";

export function AddUrlDialog(props: { open: boolean; onClose: () => void; onAdd: (url: string) => Promise<void> }) {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      await props.onAdd(url.trim());
      setUrl("");
      props.onClose();
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={(_, d) => !d.open && props.onClose()}>
      <DialogSurface>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <DialogBody>
            <DialogTitle>{t("New Download")}</DialogTitle>
            <DialogContent>
              <Field label={t("Address")} validationMessage={error || undefined} validationState={error ? "error" : "none"}>
                <Input
                  autoFocus
                  contentBefore={<LinkRegular />}
                  placeholder="https://"
                  value={url}
                  onChange={(_, d) => setUrl(d.value)}
                />
              </Field>
            </DialogContent>
            <DialogActions>
              <Button appearance="secondary" onClick={props.onClose}>
                {t("Cancel")}
              </Button>
              <Button appearance="primary" type="submit" disabled={busy || !url.trim()}>
                {t("Start")}
              </Button>
            </DialogActions>
          </DialogBody>
        </form>
      </DialogSurface>
    </Dialog>
  );
}
