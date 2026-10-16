# Rencana UI Fluent (Windows 11)

Status: **rencana**, belum ada kode. Opsi C (Wails + WebView2 + Fluent UI)
dipilih pengguna pada 2026-10-16, setelah dibandingkan dengan dua opsi lain:
memoles UI Fyne (A) dan UI WinUI 3 asli dalam C# (B). Daftar centang ada di
[ROADMAP.md](ROADMAP.md), Fase 7. Dokumen ini berisi alasan, arsitektur,
cara build dan uji, serta isi tiap tahap.

## Tujuan

- Tampilan dan rasa Windows 11: desain Fluent 2, latar Mica, font Segoe UI
  Variable, ikon Fluent, mode gelap/terang dan warna aksen dari Windows,
  sudut membulat, animasi halus.
- Semua yang bisa dilakukan di UI Fyne sekarang tetap bisa (paritas)
  sebelum UI baru menjadi bawaan.
- Engine, ekstensi, host native, dan API lokal tidak berubah perilakunya.
- Tetap Go dan tetap satu file exe.

Bukan tujuan: kontrol WinUI asli (itu opsi B), merombak alur kerja, atau
meniru tampilan program lain.

## Keputusan teknis

Versi di bawah adalah yang terbaru pada 2026-10-16. Versi akan dipatok saat
Tahap 0, dan dinaikkan hanya dengan sengaja.

| Bagian | Pilihan | Alasan |
|---|---|---|
| Kerangka aplikasi | **Wails v3** (`v3.0.0-beta.30`, butuh Go ≥ 1.25) | Wails v2 (stabil, v2.14) hanya punya satu jendela dan tidak punya ikon tray. ODM butuh keduanya: jendela Unduhan Baru, Progres, Opsi, Penjadwal, Program pendamping, dan ikon tray. Wails v3 punya banyak jendela, tray, latar Mica/Acrylic, dialog file bawaan Windows, dan binding TypeScript yang dibuat otomatis. Konsekuensinya: v3 masih beta. |
| Mesin tampilan | WebView2 (Evergreen) | Sudah terpasang di Windows 11 dan di sebagian besar Windows 10. |
| Pustaka UI | **React 19 + Fluent UI React v9** (`@fluentui/react-components` 9.74, `@fluentui/react-icons` 2.0) | v9 punya semua yang dibutuhkan download manager: DataGrid/Table (pilih banyak, sortir, lebar kolom), Tree, Toolbar, Menu, Dialog, TabList, Toast, Field. Fluent UI Web Components 3.x tidak punya tabel data. Ini mengoreksi review awal yang menyebut web components. |
| Build frontend | Vite 8 + TypeScript | Hasil build ditanam ke exe dengan `go:embed`. |

## Arsitektur

```
cmd/odm ──┬─ -tags fyne   → internal/ui      (Fyne, UI sekarang)
          └─ -tags fluent → internal/fluent  (Wails v3, UI baru)
                              ├─ service.go   metode Go yang dipanggil frontend
                              ├─ events.go    perubahan engine → event ke frontend
                              ├─ windows.go   jendela-jendela dan isinya
                              ├─ tray.go      ikon dan menu tray
                              └─ frontend/    Vite + React + Fluent UI

internal/engine, server, native, tools, ytdlp, ffmpeg …  tidak berubah
```

- Frontend memanggil Go lewat **binding Wails**, bukan HTTP. Engine tetap
  berjalan di proses yang sama, seperti dengan Fyne.
- Perubahan daftar unduhan dikirim ke frontend sebagai event, paling sering
  sekitar 4 kali per detik, dengan bentuk data yang sama seperti
  `/api/downloads` (`engine.Info`).
- Logika yang tidak terkait tampilan dipindah dari `internal/ui` ke paket
  bersama, supaya dua UI tidak menduplikasinya. Contohnya: format ukuran,
  kecepatan, dan sisa waktu; definisi kolom dan kategori; pencarian dan
  sortir; daftar pintasan. Nama paketnya sementara `internal/uicore`, dan
  katalog terjemahan pindah ke `internal/i18n`.
- **Terjemahan**: satu katalog `id.json` dipakai Fyne dan frontend. Tes Go
  memindai teks `t("…")` di kode frontend dan memastikan setiap teks punya
  terjemahan, seperti `TestIndonesianCoversTheUI` sekarang.
- **Satu instance** tetap lewat socket kontrol ODM, bukan fitur
  SingleInstance milik Wails.
- **Unduhan dari ekstensi** (prompt) membuka jendela Unduhan Baru yang
  selalu di atas, seperti sekarang.
- **Jendela**: utama, Unduhan Baru, Progres (satu per unduhan), Opsi,
  Penjadwal, Program pendamping, dan Unduhan selesai. Dialog kecil
  (konfirmasi, Tentang, Pintasan, kategori) tampil di dalam jendela sebagai
  Dialog Fluent.
- **Ciri Windows 11**:
  - latar Mica di Windows 11 22H2 ke atas, warna polos di Windows 10;
  - title bar bawaan sistem dulu (title bar buatan sendiri bisa menyusul);
  - tema gelap/terang ikut Windows atau sesuai pilihan di Opsi;
  - warna aksen dari Windows (`AccentColor` di registry DWM) atau pilihan
    pengguna;
  - font Segoe UI Variable.
- **Web UI** (`-serve`) tetap seperti sekarang. Lapisan `api` di frontend
  dibuat bisa diganti sejak awal, supaya nanti frontend yang sama bisa juga
  melayani `-serve` lewat HTTP + SSE.

## Build, pengembangan, CI

- **Kebutuhan untuk mengembangkan**:
  - Go ≥ 1.25 (Go lokal 1.27);
  - Node LTS + npm;
  - CLI `wails3` untuk membuat binding:
    `go install github.com/wailsapp/wails/v3/cmd/wails3@<versi yang dipatok>`.
- **`go.mod`**: directive `go` naik dari 1.23 ke 1.25, karena Wails v3
  memintanya. `go-version: "1.23"` di `.github/workflows/ci.yml` ikut naik.
- **Rencana perintah build** (nanti dibungkus `scripts/build.sh fluent`):
  1. `npm --prefix internal/fluent/frontend ci`
  2. `npm --prefix internal/fluent/frontend run build`
  3. `go build -tags fluent -trimpath -ldflags "-s -w -H=windowsgui -X …" -o dist/ODM-Fluent.exe ./cmd/odm`
- **CGO**: di Windows, build Wails seharusnya tidak butuh CGO atau MinGW,
  berbeda dengan Fyne. Ini dibuktikan di Tahap 0.
- **Yang di-commit**: binding hasil generate di-commit, supaya mengubah
  frontend tidak butuh CLI `wails3`. `frontend/dist` tidak di-commit.
  `go build`, `go vet ./...`, dan `go test ./...` tanpa tag tetap jalan
  tanpa Node.
- **Masa transisi**:
  - Dari kode yang sama dibangun `ODM.exe` (Fyne, bawaan) dan
    `ODM-Fluent.exe` (pratinjau).
  - Keduanya memakai data yang sama (`%APPDATA%\odm`). Hanya satu yang bisa
    berjalan sekaligus, karena socket kontrol menolak yang kedua.
- **CI**: job baru di `windows-latest`:
  1. setup-node lalu `npm ci`;
  2. Vitest;
  3. `vite build`;
  4. `go vet` dan `go test` dengan `-tags fluent`;
  5. build exe sebagai artefak.

  Di rilis, `ODM-Fluent` ikut sebagai aset pratinjau terpisah sampai Tahap 6.

## Pengujian

- **Go**: tes untuk service dan event dengan `Manager` sungguhan, seperti
  tes engine.
- **Frontend**: Vitest + Testing Library untuk komponen dan logika, misalnya
  pilih dengan Ctrl/Shift, sortir, filter, dan format.
- **Visual**: harness CDP yang sudah ada, dengan dua cara:
  - frontend dijalankan di Chrome (`vite dev` dengan binding tiruan) untuk
    tangkapan layar mode gelap, terang, dan layar sempit;
  - aplikasi sungguhan dikendalikan lewat WebView2 dengan
    `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=…`.
- **Manual**: di ODM terisolasi seperti biasa: `APPDATA` lewat junction,
  `ODM_TOOLS_DIR` diarahkan ke folder scratch, dihentikan per PID.
- **Aksesibilitas**: Narrator dan navigasi keyboard penuh dicek per layar.

## Tahapan

**Tahap 0: prototipe (gerbang keputusan).** Kerjanya di cabang `ui-fluent`.
- Kerangka Wails v3 + React + Fluent v9.
- Jendela utama berisi toolbar, pohon kategori, daftar unduhan dari engine
  sungguhan (dengan pembaruan langsung), dan status bar, dengan Mica serta
  mode gelap/terang.
- Yang diukur: ukuran exe, memori (pembanding: ODM uji dengan Fyne sekitar
  180–250 MB), waktu buka, dan kelancaran daftar dengan 1.000 item.
- Tangkapan layarnya dibandingkan berdampingan dengan UI Fyne. Pengguna
  memutuskan lanjut atau tidak.

**Tahap 1: fondasi.**
- Paket bersama (`uicore`, `i18n`).
- Service dan event untuk semua yang dipakai UI. UI Fyne sekarang memanggil
  36 fungsi engine.
- `go.mod` naik ke 1.25, job CI, dan script build.
- `ODM-Fluent.exe` pratinjau.

**Tahap 2: jendela utama setara.**
- DataGrid:
  - kolom bisa ditampilkan/disembunyikan, lebar dan sortirnya diingat;
  - pilih dengan klik, Ctrl, Shift, dan Ctrl+A;
  - menu klik kanan dan klik ganda;
  - ikon tipe file dan progres.
- Toolbar, pohon kategori dan antrean, pencarian, status bar, dan pembatas
  kecepatan.
- Pintasan keyboard, serta menu Tugas/Unduhan/Tampilan/Bantuan dalam bentuk
  menu bar Fluent.

**Tahap 3: dialog dan jendela.**
- Unduhan Baru: kartu probe, pilihan kualitas dan audio stream, yt-dlp, dan
  siaran live.
- Progres: peta koneksi, grafik kecepatan, tab pembatas kecepatan, dan opsi
  setelah selesai.
- Opsi (6 tab), Penjadwal, Kategori, Program pendamping, dan Unduhan
  selesai.
- Impor/Ekspor, pratinjau ZIP, perbarui alamat, Tentang, dan Pintasan.

**Tahap 4: integrasi Windows.**
- Tray, notifikasi, dan mulai tersembunyi (`-hidden`).
- Prompt dari ekstensi (jendela selalu di atas).
- Mulai bersama Windows, pemantau clipboard, dan cek pembaruan.
- Suara, serta buka file dan folder.

**Tahap 5: perapian.**
- Narrator dan navigasi keyboard.
- DPI tinggi dan tampilan di Windows 10.
- Terjemahan lengkap.
- Performa dan memori.

**Tahap 6: alih.**
- UI Fluent menjadi `ODM.exe` bawaan di Windows.
- Fyne tetap dipakai untuk macOS/Linux, sampai Wails di sana dievaluasi.
- README, CHANGELOG, dan tangkapan layar diperbarui.

**Nanti:**
- Frontend baru juga dipakai untuk web UI `-serve`.
- macOS/Linux dengan Wails.
- Title bar buatan sendiri dengan tab.

## Risiko dan penanganannya

- **Wails v3 masih beta**, sehingga API-nya bisa berubah.
  - Versinya dipatok.
  - Lapisan `internal/fluent` dibuat tipis.
  - Ada gerbang keputusan di Tahap 0. Kalau gagal, bisa mundur ke opsi A
    tanpa menyentuh engine.
- **WebView2 tidak ada** di sebagian Windows 10 lama atau LTSC. Wails bisa
  menawarkan pemasang WebView2; hal ini dicatat di README.
- **Mica** hanya ada di Windows 11 22H2 ke atas. Di bawahnya dipakai warna
  polos.
- **Dua UI dirawat selama transisi.**
  - Logika bersama dipindah ke paket bersama.
  - Setelah Tahap 2, UI Fyne hanya menerima perbaikan bug, tidak fitur baru.
- **Toolchain bertambah** (Node), tetapi hanya untuk tag `fluent`. Build dan
  tes bawaan tetap jalan tanpa Node.
- **Memori WebView2** diukur di Tahap 0, dibandingkan dengan Fyne.
