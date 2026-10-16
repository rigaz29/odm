// Text of the page in English (the keys) and Indonesian. The words match
// the Fyne UI's (internal/ui/translations/id.json); stage 1 moves both UIs
// to one catalog.

const id: Record<string, string> = {
  "Add URL": "Tambah URL",
  "Resume": "Lanjutkan",
  "Stop": "Hentikan",
  "Stop All": "Hentikan Semua",
  "Delete": "Hapus",
  "Clean up": "Bersihkan",
  "Options": "Opsi",
  "Scheduler": "Penjadwal",
  "Start Queue": "Mulai Antrean",
  "Stop Queue": "Hentikan Antrean",
  "Open folder": "Buka folder",
  "Open file": "Buka file",
  "Remove from list": "Hapus dari daftar",
  "Search downloads (Ctrl+F)": "Cari unduhan (Ctrl+F)",
  "All Downloads": "Semua Unduhan",
  "Unfinished": "Belum selesai",
  "Finished": "Selesai",
  "Queues": "Antrean",
  "Categories": "Kategori",
  "File Name": "Nama File",
  "Queue": "Antrean",
  "Size": "Ukuran",
  "Status": "Status",
  "Time left": "Sisa waktu",
  "Transfer rate": "Kecepatan",
  "Last attempt": "Terakhir dicoba",
  "Description": "Keterangan",
  "Complete": "Selesai",
  "Queued": "Antre",
  "Paused": "Dijeda",
  "Downloading": "Mengunduh",
  "Recording": "Merekam",
  "Error": "Gagal",
  "Error: ": "Gagal: ",
  "Verifying checksum": "Memverifikasi checksum",
  "%d items": "%d item",
  "1 item": "1 item",
  "%d of %s match the search": "%d dari %s cocok dengan pencarian",
  " · %d selected": " · %d dipilih",
  "%d active · %s": "%d aktif · %s",
  "Speed limit %s": "batas kecepatan %s",
  "%dm %ds": "%dm %dd",
  "%ds": "%dd",
  "New Download": "Unduhan Baru",
  "Address": "Alamat",
  "Start": "Mulai",
  "Cancel": "Batal",
  "Comes in stage 3 of the Fluent UI": "Hadir di tahap 3 UI Fluent",
  "Nothing here": "Tidak ada unduhan di sini",
  "Prototype": "Prototipe",
};

let lang = "en";

export function setLanguage(l: string) {
  lang = l === "id" ? "id" : "en";
  if (typeof document !== "undefined") document.documentElement.lang = lang;
}

// t translates s; tf also fills %d and %s in order.
export function t(s: string): string {
  return (lang === "id" && id[s]) || s;
}

export function tf(s: string, ...args: (string | number)[]): string {
  let i = 0;
  return t(s).replace(/%[ds]/g, () => String(args[i++] ?? ""));
}
