// A question before an action that cannot be undone, such as removing
// downloads; Enter answers yes, Esc no.

import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { t } from "../i18n";

export interface Confirm {
  title: string;
  message: string;
  yes: string;
  destructive?: boolean;
  run: () => void;
}

export function ConfirmDialog({ confirm, onClose }: { confirm: Confirm | null; onClose: () => void }) {
  return (
    <Dialog open={confirm !== null} onOpenChange={(o) => !o && onClose()}>
      {confirm && (
        <DialogContent className="max-w-sm">
          <div className="space-y-1.5 pr-6">
            <DialogTitle>{confirm.title}</DialogTitle>
            {/* A file name has no spaces to wrap at: break it anywhere. */}
            <DialogDescription className="[overflow-wrap:anywhere]">{confirm.message}</DialogDescription>
          </div>
          <div className="flex justify-end gap-2">
            <Button onClick={onClose}>{t("Cancel")}</Button>
            <Button
              autoFocus
              variant={confirm.destructive ? "destructive" : "brand"}
              onClick={() => {
                onClose();
                confirm.run();
              }}
            >
              {confirm.yes}
            </Button>
          </div>
        </DialogContent>
      )}
    </Dialog>
  );
}
