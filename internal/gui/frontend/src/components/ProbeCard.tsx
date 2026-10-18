// The card under the New Download form: what checking the address found
// (internal/gui/add.go's Card), as the Fyne UI's probe card does.

import { AlertTriangle, CircleX, Loader2, Search } from "lucide-react";
import type { ReactNode } from "react";
import type { Card, TorrentProbe } from "../bindings/odm/internal/gui/models";
import { humanBytes } from "../format";
import { markT, t, tf } from "../i18n";
import { cn } from "../lib/utils";
import { FileTile } from "./bits";

// The English texts add.go sends as a card's Title and Detail.
export const CARD_TEXTS = [
  markT("No address yet"),
  markT("Paste or type one: ODM checks the file's size and type before adding it."),
  markT("Not checked"),
  markT("Only http and https addresses are checked before they are added."),
  markT("Not a stream ODM can read"),
  markT("It downloads as a file"),
  markT("The file could not be checked"),
  markT("This stream cannot be downloaded"),
  markT("The playlist could not be read just now"),
  markT("Start lets ODM pick the best quality when it runs"),
  markT("Videos from this site need yt-dlp"),
  markT("Install yt-dlp (put yt-dlp.exe next to ODM or on PATH), or choose it in Options → Downloads."),
  markT("This video cannot be downloaded"),
  markT("yt-dlp could not read the page just now"),
  markT("Start lets yt-dlp pick the best quality when it runs"),
  markT("This ODM cannot download torrents"),
  markT("Torrents need odm-libtorrent"),
  markT("ODM downloads it once from its releases."),
  markT("The torrent could not be read"),
];

function Shell({ icon, title, detail, badge, action, tone }: { icon: ReactNode; title: string; detail?: string; badge?: ReactNode; action?: ReactNode; tone?: "warn" | "problem" }) {
  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-lg border px-3 py-2.5",
        tone === "problem" ? "border-red-500/30 bg-red-500/[0.06]" : tone === "warn" ? "border-amber-500/30 bg-amber-500/[0.06]" : "border-border bg-muted/50",
      )}
      aria-live="polite"
    >
      {icon}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" title={title}>
          {title}
        </p>
        {detail && (
          <p className="line-clamp-2 text-xs text-muted-foreground" title={detail}>
            {detail}
          </p>
        )}
      </div>
      {badge}
      {action}
    </div>
  );
}

const roundIcon = (I: typeof Search, className?: string) => (
  <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg bg-background ring-1 ring-inset ring-border", className)}>
    <I className="size-4" />
  </span>
);

// withError joins a translated text and the engine's reason.
const withError = (detail: string, error: string) => [detail && t(detail), error].filter(Boolean).join(" · ");

export function ProbeCard({
  card,
  torrent,
  checking,
  name,
  category,
  action,
}: {
  card: Card | null;
  torrent?: TorrentProbe | null;
  checking: boolean;
  name: string;
  category: string;
  action?: ReactNode;
}) {
  if (checking) return <Shell icon={roundIcon(Loader2, "[&_svg]:animate-spin text-muted-foreground")} title={t("Checking the address…")} />;
  if (!card || card.state === "idle") return <Shell icon={roundIcon(Search, "text-muted-foreground")} title={t(card?.title || "No address yet")} detail={t(card?.detail || "Paste or type one: ODM checks the file's size and type before adding it.")} />;
  switch (card.state) {
    case "file": {
      const parts = [card.size > 0 ? humanBytes(card.size) : t("Size unknown"), card.contentType, card.host].filter(Boolean);
      return (
        <Shell
          icon={<FileTile category={category} size="size-8" icon="size-4" />}
          title={name || card.name || t("Unnamed file")}
          detail={parts.join(" · ")}
          badge={
            <span
              className={cn(
                "shrink-0 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
                card.resumable ? "bg-emerald-500/10 text-emerald-700 ring-emerald-500/25 dark:text-emerald-400" : "bg-amber-500/10 text-amber-700 ring-amber-500/25 dark:text-amber-300",
              )}
            >
              {card.resumable ? t("Resumable") : t("Cannot resume")}
            </span>
          }
          action={action}
        />
      );
    }
    case "stream": {
      const info = [tf("%s stream", card.format), card.note].filter(Boolean).join(" · ");
      return <Shell icon={<FileTile category="Video" size="size-8" icon="size-4" />} title={card.name || name || t("Video stream")} detail={info} />;
    }
    case "site":
      return (
        <Shell
          icon={<FileTile category="Video" size="size-8" icon="size-4" />}
          title={card.name || name}
          detail={tf("%s video · downloaded by yt-dlp", card.site || t("Video site"))}
        />
      );
    case "torrent": {
      if (torrent?.waiting) {
        return (
          <Shell
            icon={roundIcon(Loader2, "[&_svg]:animate-spin text-muted-foreground")}
            title={name || card.name || t("Torrent")}
            detail={withError(tf("Getting its files from its peers: %d connected. Start downloads every file.", torrent.peers), torrent.error)}
          />
        );
      }
      const files = (torrent?.files ?? []).filter((f) => !f.pad).length;
      const parts = [humanBytes(card.size) || "", files === 1 ? t("1 file") : tf("%d files", files), torrent?.private ? t("Private torrent") : ""].filter(Boolean);
      return <Shell icon={<FileTile category={category} size="size-8" icon="size-4" />} title={name || card.name || t("Torrent")} detail={parts.join(" · ")} />;
    }
    case "warn":
      return <Shell tone="warn" icon={roundIcon(AlertTriangle, "text-amber-600 dark:text-amber-400")} title={t(card.title)} detail={withError(card.detail, card.error)} action={action} />;
    default:
      return <Shell tone="problem" icon={roundIcon(CircleX, "text-red-600 dark:text-red-400")} title={t(card.title)} detail={withError(card.detail, card.error)} />;
  }
}

