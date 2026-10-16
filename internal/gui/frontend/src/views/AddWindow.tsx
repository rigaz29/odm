// The New Download window for a download the browser handed over
// (?view=add&token=…): the form, prefilled. It closes once the download is
// added or Cancel is pressed.

import { ArrowDownToLine } from "lucide-react";
import { useEffect } from "react";
import { api } from "../api";
import { AddForm } from "../components/AddForm";
import { t } from "../i18n";

export function AddWindow({ token }: { token: string }) {
  useEffect(() => {
    api.setTitle(t("New Download"));
    const drop = () => void api.dropPrefill(token);
    window.addEventListener("pagehide", drop);
    return () => window.removeEventListener("pagehide", drop);
  }, [token]);

  return (
    <main className="aurora flex h-full flex-col bg-background p-5 text-foreground">
      <div className="mb-4 flex items-center gap-2.5">
        <span className="accent-gradient flex size-8 items-center justify-center rounded-lg text-white shadow-sm shadow-violet-500/30">
          <ArrowDownToLine className="size-4" strokeWidth={2.5} />
        </span>
        <div>
          <h1 className="text-base font-semibold tracking-tight">{t("New Download")}</h1>
          <p className="text-xs text-muted-foreground">{t("Check the address and where it is saved, then start it.")}</p>
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <AddForm token={token} onDone={api.closeWindow} />
      </div>
    </main>
  );
}
