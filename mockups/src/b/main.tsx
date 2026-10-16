// Mockup B: Material Design 3 with MUI: Google's tonal colour scheme
// (as Gmail and Drive use it), an extended FAB, a navigation drawer with
// pill indicators, a search bar, filter chips and two-line list items.

import "@fontsource-variable/roboto-flex";
import { createRoot } from "react-dom/client";
import {
  Box,
  Checkbox,
  Chip,
  CssBaseline,
  Fab,
  IconButton,
  InputBase,
  LinearProgress,
  ThemeProvider,
  Tooltip,
  Typography,
  createTheme,
} from "@mui/material";
import AddRounded from "@mui/icons-material/AddRounded";
import AlbumRounded from "@mui/icons-material/AlbumRounded";
import AppsRounded from "@mui/icons-material/AppsRounded";
import CheckCircleRounded from "@mui/icons-material/CheckCircleRounded";
import CheckRounded from "@mui/icons-material/CheckRounded";
import CloseRounded from "@mui/icons-material/CloseRounded";
import CropSquareRounded from "@mui/icons-material/CropSquareRounded";
import DeleteOutlineRounded from "@mui/icons-material/DeleteOutlineRounded";
import DescriptionRounded from "@mui/icons-material/DescriptionRounded";
import DownloadRounded from "@mui/icons-material/DownloadRounded";
import EventRepeatRounded from "@mui/icons-material/EventRepeatRounded";
import FolderZipRounded from "@mui/icons-material/FolderZipRounded";
import ImageRounded from "@mui/icons-material/ImageRounded";
import InboxRounded from "@mui/icons-material/InboxRounded";
import MoreVertRounded from "@mui/icons-material/MoreVertRounded";
import MovieRounded from "@mui/icons-material/MovieRounded";
import MusicNoteRounded from "@mui/icons-material/MusicNoteRounded";
import PauseRounded from "@mui/icons-material/PauseRounded";
import PlayArrowRounded from "@mui/icons-material/PlayArrowRounded";
import QueueRounded from "@mui/icons-material/QueueRounded";
import RemoveRounded from "@mui/icons-material/RemoveRounded";
import ScheduleRounded from "@mui/icons-material/ScheduleRounded";
import SearchRounded from "@mui/icons-material/SearchRounded";
import SettingsRounded from "@mui/icons-material/SettingsRounded";
import type { SvgIconComponent } from "@mui/icons-material";
import {
  type Category,
  type Row,
  type Status,
  active,
  bytes,
  categories,
  countBy,
  dark,
  eta,
  finished,
  pct,
  rate,
  rows,
  selected,
  statusLabel,
  totalSpeed,
  unfinished,
} from "../data";

// Material 3 tokens, Google's blue scheme.
const m3 = dark
  ? {
      primary: "#A8C7FA", onPrimary: "#062E6F", primaryContainer: "#0842A0", onPrimaryContainer: "#D3E3FD",
      secondaryContainer: "#004A77", onSecondaryContainer: "#C2E7FF",
      tertiaryContainer: "#0F5223", onTertiaryContainer: "#6DD58C",
      errorContainer: "#8C1D18", onErrorContainer: "#F9DEDC", error: "#F2B8B5",
      warnContainer: "#5C4200", onWarnContainer: "#FFDF99",
      background: "#131314", surface: "#1E1F20", surfaceLow: "#1B1B1B", surfaceHigh: "#282A2C", surfaceHighest: "#333537",
      onSurface: "#E3E3E3", onSurfaceVariant: "#C4C7C5", outline: "#8E918F", outlineVariant: "#444746",
    }
  : {
      primary: "#0B57D0", onPrimary: "#FFFFFF", primaryContainer: "#D3E3FD", onPrimaryContainer: "#041E49",
      secondaryContainer: "#C2E7FF", onSecondaryContainer: "#001D35",
      tertiaryContainer: "#C4EED0", onTertiaryContainer: "#0F5223",
      errorContainer: "#F9DEDC", onErrorContainer: "#8C1D18", error: "#B3261E",
      warnContainer: "#FFDF99", onWarnContainer: "#5C4200",
      background: "#F8FAFD", surface: "#FFFFFF", surfaceLow: "#F0F4F9", surfaceHigh: "#E9EEF6", surfaceHighest: "#DDE3EA",
      onSurface: "#1F1F1F", onSurfaceVariant: "#444746", outline: "#747775", outlineVariant: "#C4C7C5",
    };

const theme = createTheme({
  palette: {
    mode: dark ? "dark" : "light",
    primary: { main: m3.primary, contrastText: m3.onPrimary },
    background: { default: m3.background, paper: m3.surface },
    text: { primary: m3.onSurface, secondary: m3.onSurfaceVariant },
    divider: m3.outlineVariant,
  },
  shape: { borderRadius: 12 },
  typography: { fontFamily: '"Roboto Flex Variable", Roboto, "Segoe UI", sans-serif' },
});

const catIcon: Record<Category, SvgIconComponent> = {
  video: MovieRounded,
  audio: MusicNoteRounded,
  docs: DescriptionRounded,
  programs: AppsRounded,
  archives: FolderZipRounded,
  images: ImageRounded,
  disk: AlbumRounded,
};

const catTone: Record<Category, [string, string]> = dark
  ? { video: ["#4A2C6E", "#E9DDFF"], audio: ["#6A1D45", "#FFD8E8"], docs: ["#0842A0", "#D3E3FD"], programs: ["#3A3F45", "#DDE3EA"], archives: ["#5C4200", "#FFDF99"], images: ["#0F5223", "#C4EED0"], disk: ["#004A77", "#C2E7FF"] }
  : { video: ["#EADDFF", "#4A2C6E"], audio: ["#FFD8E8", "#6A1D45"], docs: ["#D3E3FD", "#041E49"], programs: ["#E1E3E6", "#2D3135"], archives: ["#FFDF99", "#5C4200"], images: ["#C4EED0", "#0F5223"], disk: ["#C2E7FF", "#001D35"] };

const chipTone: Record<Status, [string, string]> = {
  downloading: [m3.primaryContainer, m3.onPrimaryContainer],
  recording: [m3.errorContainer, m3.onErrorContainer],
  paused: [m3.warnContainer, m3.onWarnContainer],
  queued: [m3.surfaceHighest, m3.onSurfaceVariant],
  error: [m3.errorContainer, m3.onErrorContainer],
  complete: [m3.tertiaryContainer, m3.onTertiaryContainer],
};

function NavItem({ icon: Icon, label, count, on }: { icon: SvgIconComponent; label: string; count?: number; on?: boolean }) {
  return (
    <Box
      sx={{
        display: "flex", alignItems: "center", gap: 1.5, height: 38, px: 2, borderRadius: 999,
        bgcolor: on ? m3.secondaryContainer : "transparent", color: on ? m3.onSecondaryContainer : m3.onSurfaceVariant,
        fontWeight: on ? 700 : 500, fontSize: 14, cursor: "pointer", "&:hover": { bgcolor: on ? m3.secondaryContainer : m3.surfaceHigh },
      }}
    >
      <Icon sx={{ fontSize: 20 }} />
      <Box sx={{ flex: 1 }}>{label}</Box>
      {count !== undefined && <Box sx={{ fontSize: 12, fontWeight: 600 }}>{count}</Box>}
    </Box>
  );
}

function RowItem({ r }: { r: Row }) {
  const Icon = catIcon[r.category];
  const [bg, fg] = catTone[r.category];
  const [cbg, cfg] = chipTone[r.status];
  const sel = selected.has(r.id);
  const running = r.status === "downloading";
  const detail =
    r.status === "recording"
      ? `${bytes(r.done)} direkam · ${rate(r.speed)}`
      : r.status === "complete"
        ? `${bytes(r.size)} · ${r.added}`
        : running
          ? `${pct(r)}% · ${rate(r.speed)} · sisa ${eta(r.eta)}`
          : r.status === "error"
            ? r.error!
            : `${pct(r)}% dari ${bytes(r.size)}`;
  return (
    <Box
      sx={{
        display: "flex", alignItems: "center", gap: 2, px: 2, height: 64, borderRadius: 3, mx: 1,
        bgcolor: sel ? (dark ? "rgba(0,74,119,0.55)" : "rgba(194,231,255,0.65)") : "transparent",
        "&:hover": { bgcolor: sel ? undefined : m3.surfaceLow },
      }}
    >
      {sel ? (
        <Checkbox checked size="small" sx={{ p: 0.75, color: m3.primary, "&.Mui-checked": { color: m3.primary } }} />
      ) : (
        <Box sx={{ width: 40, height: 40, borderRadius: "50%", bgcolor: bg, color: fg, display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}>
          <Icon sx={{ fontSize: 20 }} />
        </Box>
      )}
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography noWrap sx={{ fontSize: 15, fontWeight: 500, color: m3.onSurface }}>
          {r.name}
        </Typography>
        <Typography noWrap sx={{ fontSize: 13, color: r.status === "error" ? m3.error : m3.onSurfaceVariant }}>
          {detail} {r.status !== "error" && <span style={{ opacity: 0.7 }}>· {r.host}</span>}
        </Typography>
      </Box>
      <Box sx={{ width: 180, display: r.status === "complete" || r.status === "recording" ? "none" : "block" }}>
        <LinearProgress
          variant="determinate"
          value={pct(r)}
          sx={{
            height: 6, borderRadius: 3, bgcolor: m3.surfaceHighest,
            "& .MuiLinearProgress-bar": { borderRadius: 3, bgcolor: r.status === "error" ? m3.error : r.status === "paused" ? (dark ? "#F9BC4D" : "#B57F00") : m3.primary },
          }}
        />
      </Box>
      <Chip
        size="small"
        label={statusLabel[r.status]}
        icon={r.status === "complete" ? <CheckRounded /> : r.status === "recording" ? <Box component="span" sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: "#E3342F", ml: "10px !important" }} /> : undefined}
        sx={{ bgcolor: cbg, color: cfg, fontWeight: 600, borderRadius: 2, width: 104, "& .MuiChip-icon": { color: cfg, fontSize: 16 } }}
      />
      <IconButton size="small" sx={{ color: m3.onSurfaceVariant }}>
        <MoreVertRounded fontSize="small" />
      </IconButton>
    </Box>
  );
}

function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ height: "100vh", display: "flex", flexDirection: "column", bgcolor: m3.background, color: m3.onSurface }}>
        {/* Title bar */}
        <Box sx={{ height: 40, display: "flex", alignItems: "center", pl: 2, flexShrink: 0 }}>
          <Box sx={{ width: 22, height: 22, borderRadius: "7px", bgcolor: m3.primary, color: m3.onPrimary, display: "flex", alignItems: "center", justifyContent: "center", mr: 1.25 }}>
            <DownloadRounded sx={{ fontSize: 16 }} />
          </Box>
          <Typography sx={{ fontSize: 13, fontWeight: 600 }}>Ochi Download Manager</Typography>
          <Box sx={{ ml: "auto", display: "flex", height: "100%" }}>
            {[RemoveRounded, CropSquareRounded, CloseRounded].map((I, i) => (
              <Box key={i} sx={{ width: 46, display: "flex", alignItems: "center", justifyContent: "center", color: m3.onSurfaceVariant, "&:hover": { bgcolor: i === 2 ? "#C42B1C" : m3.surfaceHigh, color: i === 2 ? "#fff" : undefined } }}>
                <I sx={{ fontSize: i === 1 ? 14 : 18 }} />
              </Box>
            ))}
          </Box>
        </Box>

        <Box sx={{ flex: 1, display: "flex", minHeight: 0 }}>
          {/* Navigation drawer */}
          <Box sx={{ width: 256, flexShrink: 0, px: 1.5, pt: 1, display: "flex", flexDirection: "column", gap: 0.25, overflow: "hidden" }}>
            <Fab variant="extended" sx={{ alignSelf: "flex-start", mb: 1.5, ml: 0.5, height: 52, borderRadius: 4, px: 2.5, bgcolor: m3.primaryContainer, color: m3.onPrimaryContainer, boxShadow: dark ? "none" : "0 1px 3px rgba(0,0,0,.15)", textTransform: "none", fontSize: 15, fontWeight: 600, "&:hover": { bgcolor: m3.primaryContainer } }}>
              <AddRounded sx={{ mr: 1.25 }} /> Tambah URL
            </Fab>
            <NavItem icon={InboxRounded} label="Semua unduhan" count={rows.length} on />
            <NavItem icon={DownloadRounded} label="Sedang berjalan" count={active.length} />
            <NavItem icon={ScheduleRounded} label="Belum selesai" count={unfinished} />
            <NavItem icon={CheckCircleRounded} label="Selesai" count={finished} />
            <Typography sx={{ fontSize: 13, fontWeight: 600, color: m3.onSurfaceVariant, px: 2, pt: 1.5, pb: 0.5 }}>Kategori</Typography>
            {categories.map((c) => (
              <NavItem key={c.id} icon={catIcon[c.id]} label={c.label} count={countBy(c.id)} />
            ))}
            <Typography sx={{ fontSize: 13, fontWeight: 600, color: m3.onSurfaceVariant, px: 2, pt: 1.5, pb: 0.5 }}>Antrean</Typography>
            <NavItem icon={QueueRounded} label="Utama" count={2} />
          </Box>

          {/* Content */}
          <Box sx={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", pr: 2, pb: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, py: 1 }}>
              <Box sx={{ flex: 1, maxWidth: 720, height: 48, borderRadius: 999, bgcolor: m3.surfaceHigh, display: "flex", alignItems: "center", px: 2, gap: 1.5 }}>
                <SearchRounded sx={{ color: m3.onSurfaceVariant }} />
                <InputBase placeholder="Cari unduhan atau tempel URL" sx={{ flex: 1, fontSize: 15 }} />
              </Box>
              <Box sx={{ ml: "auto", display: "flex", gap: 0.5 }}>
                <Tooltip title="Penjadwal"><IconButton sx={{ color: m3.onSurfaceVariant }}><EventRepeatRounded /></IconButton></Tooltip>
                <Tooltip title="Opsi"><IconButton sx={{ color: m3.onSurfaceVariant }}><SettingsRounded /></IconButton></Tooltip>
              </Box>
            </Box>

            <Box sx={{ flex: 1, minHeight: 0, bgcolor: m3.surface, borderRadius: 4, display: "flex", flexDirection: "column", overflow: "hidden" }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 3, pt: 2.5, pb: 1.5 }}>
                <Typography sx={{ fontSize: 22, fontWeight: 400, mr: 2 }}>Semua unduhan</Typography>
                {["Semua", "Berjalan", "Selesai", "Gagal"].map((t, i) => (
                  <Chip
                    key={t}
                    label={t}
                    icon={i === 0 ? <CheckRounded /> : undefined}
                    variant={i === 0 ? "filled" : "outlined"}
                    sx={{ borderRadius: 2, fontWeight: 500, height: 32, bgcolor: i === 0 ? m3.secondaryContainer : "transparent", color: i === 0 ? m3.onSecondaryContainer : m3.onSurfaceVariant, borderColor: m3.outline, "& .MuiChip-icon": { color: m3.onSecondaryContainer, fontSize: 18 } }}
                  />
                ))}
                <Box sx={{ ml: "auto", display: "flex", gap: 1 }}>
                  {[[PlayArrowRounded, "Lanjutkan"], [PauseRounded, "Jeda"], [DeleteOutlineRounded, "Hapus"]].map(([I, l]) => {
                    const Ico = I as SvgIconComponent;
                    return (
                      <Box key={l as string} sx={{ display: "flex", alignItems: "center", gap: 0.75, height: 40, px: 2, borderRadius: 999, bgcolor: m3.secondaryContainer, color: m3.onSecondaryContainer, fontSize: 14, fontWeight: 600 }}>
                        <Ico sx={{ fontSize: 18 }} /> {l as string}
                      </Box>
                    );
                  })}
                </Box>
              </Box>
              <Box sx={{ flex: 1, overflow: "hidden", pb: 1 }}>
                {rows.map((r) => (
                  <RowItem key={r.id} r={r} />
                ))}
              </Box>
            </Box>
            <Box sx={{ display: "flex", gap: 2, fontSize: 12, color: m3.onSurfaceVariant, px: 1, pt: 0.75 }}>
              <span>{rows.length} item · 2 dipilih</span>
              <span style={{ marginLeft: "auto" }}>{active.length} aktif · ↓ {rate(totalSpeed)}</span>
              <span>Batas kecepatan: tidak ada</span>
            </Box>
          </Box>
        </Box>
      </Box>
    </ThemeProvider>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
