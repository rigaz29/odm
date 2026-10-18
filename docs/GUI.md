# Rencana UI baru (shadcn/ui + sentuhan Aurora)

Status: **disetujui (2026-10-16)**, mulai Tahap 1. Daftar centangnya ada di
[ROADMAP.md](ROADMAP.md), Fase 7. Dokumen ini berisi riwayat, keputusan,
arsitektur, cara build dan uji, isi tiap tahap, serta risikonya.

## Riwayat singkat

1. **UI Fluent (Windows 11) dihentikan.** Prototipe Tahap 0 di cabang
   `ui-fluent` membuktikan kerangkanya layak, dengan pembanding UI Fyne:

   | | Fyne | Prototipe |
   |---|---|---|
   | Ukuran exe | 28,6 MB | 15,4 MB |
   | Build | butuh CGO + MinGW | tanpa CGO |
   | Scroll 1.023 unduhan | — | 60 fps |
   | Memori | 338 MB | ±365 MB |

   Tetapi tampilan Fluent dinilai kurang bagus.
2. **Tiga mockup** jendela utama dibandingkan di cabang `ui-mockups`
   (folder `mockups/`): A shadcn/ui, B Material Design 3, D desain sendiri
   "Aurora".
3. **Pilihan pengguna: A ditambah sentuhan D**: halaman `mockups/ad.html`,
   tangkapan `mockups/screenshots/ad-dark.jpg` dan `ad-light.jpg`.

## Tujuan

- Jendela yang modern, rapi, dan konsisten, sesuai mockup A+D, dalam mode
  gelap dan terang.
- Semua yang bisa dilakukan di UI Fyne tetap bisa (paritas) sebelum UI baru
  menjadi bawaan.
- Engine, ekstensi, host native, dan API lokal tidak berubah perilakunya.
- Tetap Go dan tetap satu file exe.

## Bahasa desain

**Dari A (shadcn/ui):**
- palet zinc netral, garis 1 px, radius 6–8 px;
- font Inter, angka `tabular-nums`;
- tabel dengan kotak centang dan badge status bercincin tipis, kepadatan
  baris sekitar 44 px.

**Dari D (Aurora):**
- **Aksen gradien ungu-sian** (`#8b5cf6 → #6366f1 → #22d3ee`), hanya di
  beberapa tempat: tombol utama, progres unduhan aktif, penanda menu aktif,
  kotak centang terpilih, angka dan grafik kecepatan.
- **Kartu "Sedang berjalan"** di atas tabel, berisi:
  - progres, kecepatan, sisa waktu, dan ukuran;
  - bilah per koneksi dan grafik kecepatan;
  - label LIVE dan lama rekaman untuk siaran.

  Kartu digambar bergaya A (berbingkai, tanpa efek kaca). Paling banyak 3
  kartu ditambah tautan "+N lainnya", dan bagiannya bisa diciutkan.
- **Ikon file berwarna** per kategori (gradien kecil).
- **Cahaya aurora sangat tipis** di bagian atas jendela.

**Title bar buatan sendiri** dengan kotak pencarian atau tempel URL (Ctrl K)
di tengah. Perilaku aslinya tetap dari Windows (lihat Arsitektur).

## Keputusan teknis

Versi dipatok saat Tahap 1, dan dinaikkan hanya dengan sengaja.

| Bagian | Pilihan | Alasan |
|---|---|---|
| Kerangka aplikasi | Wails v3 (`v3.0.0-beta.30`, Go ≥ 1.25) | Sudah teruji di Tahap 0. Punya banyak jendela, tray, jendela tanpa bingkai, dan wilayah non-client lewat CSS. |
| Mesin tampilan | WebView2 | Sudah ada di Windows 11 dan sebagian besar Windows 10. |
| Komponen | shadcn/ui (kode komponen disalin ke repo) di atas Radix | Menu, menu klik kanan, dialog, tooltip, tab, dan kotak centang yang aksesibel. Tampilannya bisa diatur penuh. |
| Gaya | Tailwind CSS 4 | Token tema sebagai variabel CSS, mode gelap lewat kelas. |
| Daftar | TanStack Virtual | Hanya baris yang terlihat yang ada di DOM. Sortir, lebar, dan visibilitas kolom ditangani halaman sendiri (`columns.ts`); TanStack Table dilepas di Tahap 2. |
| Ikon dan font | Lucide, Inter (dibundel) | Sama seperti mockup. |
| Build halaman | Vite + TypeScript, React 19 | Ditanam ke exe dengan `go:embed`. |

## Arsitektur

```
cmd/odm ──┬─ -tags fyne → internal/ui   (Fyne, UI sekarang)
          └─ -tags gui  → internal/gui  (Wails v3, UI baru)
                            ├─ service.go   metode Go yang dipanggil halaman
                            ├─ app.go       jendela (tanpa bingkai), event
                            └─ frontend/    Vite + React + Tailwind + shadcn/ui
internal/i18n   katalog terjemahan bersama (Fyne dan halaman)
internal/engine, server, native, tools …    tidak berubah
```

- **Asal kode**: Go dan kerangka halaman dipindah dari prototipe Fluent
  (`internal/fluent` → `internal/gui`, tag `fluent` → `gui`). Yang dipakai
  lagi: service dan tesnya, binding, lapisan `api` beserta data tiruan
  untuk uji visual, format, dan terjemahan. Komponen Fluent diganti.
- **Halaman memanggil Go** lewat binding Wails. Perubahan daftar dikirim
  sebagai event hanya saat berubah, paling cepat tiap 500 ms.
- **Data tambahan untuk sentuhan D**, dihitung di service:
  - progres per koneksi, diambil dari `engine.Info.Chunks` dan hanya untuk
    unduhan yang aktif;
  - riwayat kecepatan per unduhan dan total, sebagai cincin sampel tiap
    detik;
  - lama rekaman siaran live.
- **Title bar**: jendela `Frameless`. Area kosongnya `--wails-draggable`,
  sehingga jendela dipindah oleh Windows sendiri (diseret ke tepi layar
  tetap snap), dan klik ganda me-maximize lewat runtime. WebView2 di-host
  di jendela anak, bukan lewat composition (`WebView2CompositionHosting`).
  Composition sempat dipakai di Tahap 1 agar Windows menangani bagian
  title bar lewat `--wails-non-client-region`, termasuk snap layout di
  tombol Maximize. Namun di Tahap 5 ternyata dalam mode itu UI Automation
  halaman hanya sampai ke Windows lewat jendela induk, dan Wails beta.30
  tidak meneruskannya, sehingga Narrator tidak bisa membaca jendela utama.
  Pembaca layar diutamakan; snap layout di tombol Maximize (dan Win+Z)
  dilepas.
- **Terjemahan**: satu katalog `id.json` di `internal/i18n`, dipakai Fyne
  dan halaman. Tes memastikan setiap teks `t("…")` di halaman punya
  terjemahan.
- **Satu instance** lewat socket kontrol ODM. Unduhan dari ekstensi
  membuka jendela Unduhan Baru yang selalu di atas.
- **Tema**: mode gelap/terang ikut Windows atau pilihan di Opsi; aksen
  tetap gradien ODM.

## Build, pengembangan, CI

- Kebutuhan: Go ≥ 1.25, Node LTS + npm, dan CLI `wails3` (untuk membuat
  binding).
- Build: `bash scripts/build.sh windows`. Isinya `npm ci`, lalu `npm run
  build` di `internal/gui/frontend`, lalu `go build -tags gui,production`
  tanpa CGO, dan hasilnya `dist/ODM.exe` (sejak Tahap 6; sebelumnya
  pratinjau `ODM-Next.exe` lewat target `gui`). UI Fyne untuk Windows tetap
  bisa dibangun dengan `bash scripts/build.sh windows-fyne`.
- Linux dan macOS (sejak Tahap 8): `bash scripts/build.sh linux`, `darwin`,
  atau `darwin-intel`, dengan CGO (GTK 4 + WebKitGTK 6.0 di Linux, Xcode
  command line tools di macOS); hasilnya `dist/odm`. UI Fyne di sana:
  `linux-fyne` dan `darwin-fyne`.
- Tag `production` adalah mode rilis Wails: menu klik kanan bawaan WebView2
  (Back, Refresh, Print, Inspect) disembunyikan kecuali di kolom teks dan
  teks yang diseleksi, dan DevTools mati. Build uji yang perlu DevTools atau
  CDP menambahkan tag `devtools`.
- Binding hasil generate di-commit; `frontend/dist` tidak. Build dan tes
  tanpa tag tetap jalan tanpa Node.
- Selama transisi `ODM.exe` (Fyne) dan `ODM-Next.exe` memakai data yang
  sama, tetapi hanya satu yang bisa berjalan sekaligus. Pengaturan ikut
  pindah, termasuk tema dan bahasa dari preferensi Fyne.
- CI: job "Windows (desktop UI)" menjalankan `npm ci`, Vitest, `vite
  build`, lalu `go vet` dan `go test` dengan `-tags gui`, dan menghasilkan
  `ODM.exe`. Rilis membangun `ODM.exe` Windows dengan cara yang sama; Linux
  dan macOS tetap Fyne. Build Fyne Windows tetap diuji di CI.

## Pengujian

- **Go**: tes service (snapshot, event, data koneksi dan kecepatan) dengan
  `Manager` sungguhan.
- **Halaman**: Vitest untuk logika (format, pilihan, filter, sortir).
- **Visual**: halaman dengan data tiruan di Chrome (tangkapan gelap/terang),
  dan aplikasi sungguhan lewat DevTools WebView2
  (`ODM_WEBVIEW2_ARGS=--remote-debugging-port=…`).
- **Manual**: di ODM terisolasi seperti biasa; aksesibilitas dicek dengan
  Narrator dan keyboard.

## Tahapan

**Tahap 0: kerangka dan gaya.** Selesai. Prototipe Fluent (`ui-fluent`)
membuktikan kerangkanya, lalu mockup (`ui-mockups`) menentukan gaya A+D.

**Tahap 1: fondasi.**
- Cabang `ui-next` dari `ui-fluent`, dengan `internal/fluent` diganti nama
  menjadi `internal/gui` dan tag menjadi `gui`.
- Tailwind 4, komponen shadcn/ui yang dibutuhkan, dan token tema A+D.
- Title bar buatan sendiri (wilayah non-client, dilepas di Tahap 5).
- Katalog terjemahan bersama `internal/i18n`.
- Service ditambah data koneksi, riwayat kecepatan, dan lama rekaman.
- Jendela utama pertama sesuai mockup dengan data engine sungguhan.
- Job CI, target build, dan `ODM-Next.exe` pratinjau.

**Tahap 2: jendela utama setara.**
- Tabel:
  - sortir, lebar kolom, dan kolom tampil/sembunyi yang diingat;
  - pilih dengan klik, Ctrl, Shift, dan Ctrl+A;
  - menu klik kanan, klik ganda, dan pintasan keyboard.
- Kartu "Sedang berjalan" (+N lainnya, bisa diciutkan), sidebar dengan
  jumlah, tab filter, palet Ctrl K (cari atau tempel URL), status bar, dan
  pembatas kecepatan.

**Tahap 3: dialog dan jendela.**
- Unduhan Baru: kartu probe, kualitas stream dan audio, yt-dlp, siaran
  live.
- Jendela progres: peta koneksi dan grafik kecepatan.
- Opsi (jendela pengaturan dengan menu samping), Penjadwal, Kategori,
  Program pendamping, Unduhan selesai.
- Impor/Ekspor, pratinjau ZIP, perbarui alamat, Tentang, Pintasan.

**Tahap 4: integrasi Windows.**
- Tray, notifikasi, dan mulai tersembunyi (`-hidden`).
- Jendela konfirmasi dari ekstensi yang selalu di atas.
- Mulai bersama Windows, pemantau clipboard, cek pembaruan, suara.

**Tahap 5: perapian.**
- Narrator dan navigasi keyboard, DPI tinggi, animasi halus.
- Terjemahan lengkap, uji 1.000+ unduhan, dan memori.

**Tahap 6: alih.**
- UI baru menjadi `ODM.exe` bawaan di Windows: `scripts/build.sh windows`,
  CI, dan rilis memakai tag `gui`, tanpa MinGW.
- Fyne tetap dipakai untuk macOS/Linux sampai dievaluasi; untuk Windows
  masih bisa dibangun (`windows-fyne`).
- Pemeriksaan WebView2 Runtime sebelum UI dibuka.
- README (fitur, build, tangkapan layar) dan CHANGELOG diperbarui.

**Setelah Tahap 6: jendela Persiapan.**
- Terbuka sendiri sekali, saat ODM pertama kali dibuka (atau untuk
  pengguna lama kalau yt-dlp atau ffmpeg belum ada); bisa dibuka lagi dari
  ⋯ → Bantuan → Persiapan ODM… dan Opsi → Umum.
- Memasang yt-dlp dan ffmpeg sekaligus dengan progress, plus pilihan
  perbarui otomatis.
- Daftar browser terpasang (`internal/browsers`, dari registry
  `StartMenuInternet`) dengan status extension. Firefox dan turunannya
  membuka add-on bertanda tangan dari rilis; Chromium mendapat folder
  extension dari dalam `ODM.exe` (`extension/files.go`,
  `extpack.Unpack`) dan langkah *Load unpacked*. Browser tidak mau
  membuka `chrome://extensions` dari program lain, jadi alamatnya
  disalin untuk ditempel.
- Status "terhubung" dari native host: extension mengirim nama
  browsernya, host meneruskannya di header `X-ODM-Browser`, dan ODM
  menyimpannya di `extension_browsers`.
- Daftar browser baru ada untuk Windows; di Linux/macOS jendela Setup
  menunjuk ke halaman rilis (Tahap 7).

**Tahap 7: Linux dan macOS, penyesuaian minimal.** Selesai 2026-10-18;
lihat [rencananya](#rencana-linux-dan-macos-penyesuaian-minimal) dan hasil
ujinya.

**Tahap 8: alih di Linux dan macOS.** Selesai 2026-10-18, atas permintaan
pemilik setelah uji jalan Tahap 7.
- UI baru menjadi rilis bawaan Linux (amd64, arm64) dan macOS (Apple
  silicon, Intel): `scripts/build.sh linux`/`darwin`/`darwin-intel`, CI,
  dan rilis memakai tag `gui`.
- UI Fyne tetap dirilis untuk Linux tanpa WebKitGTK 6.0 (Ubuntu 22.04,
  Linux Mint 21, Debian 11) sebagai `odm-linux-amd64-fyne` dan
  `odm-linux-arm64-fyne`, dibangun di Ubuntu 22.04 supaya jalan di sana.
  macOS tidak butuh cadangan: WebKit selalu ada.
- Tema dan bahasa dari preferensi Fyne kini dibaca dari tempat Fyne
  menyimpannya di tiap sistem (`~/.config/fyne` di Linux,
  `~/Library/Preferences/fyne` di macOS), tidak hanya di Windows.
- README (instal, kebutuhan Linux, build) dan CHANGELOG diperbarui.

## Rencana Linux dan macOS (penyesuaian minimal)

Status: **selesai 2026-10-18** (rencana dicatat 2026-10-17): butir 1–5
di kode dan CI, dan lolos uji jalan di WSL Ubuntu. Tujuannya UI
baru *bisa jalan* di Linux dan macOS dengan penyesuaian sekecil mungkin,
ibarat mesin yang dipindah ke mobil lain: bautnya disesuaikan, bodinya
tidak dipoles. Kerapian per platform menyusul bila diperlukan.

**Temuan** (Wails v3.0.0-beta.30, sudah dicek di kodenya):
- Linux memakai GTK4 + WebKitGTK 6.0 secara bawaan; GTK3 + WebKitGTK 4.1
  hanya lewat tag build `gtk3`, yang oleh Wails disebut *legacy*. (Catatan
  awal rencana ini terbalik; dicek ulang di `linux_cgo.go` saat mulai
  dikerjakan.) Tray dan notifikasi lewat DBus.
- Di Linux dan macOS halaman disajikan dari `wails://localhost`, bukan
  `http://wails.localhost` seperti di Windows. Halaman yang hanya mengenali
  alamat Windows mengira dirinya di browser biasa dan memakai data tiruan.
- Jendela Setup juga terbuka di start pertama di Linux/macOS, tetapi daftar
  browser di sana kosong (`browsers.Installed`), sehingga tertulis "No
  browser was found.".
- macOS memakai WKWebView. Bila aplikasi tidak memasang menu, Wails
  memasang menu bawaannya (Edit dengan ⌘C/⌘V, Quit dengan ⌘Q).
- Service notifikasi gagal start di macOS bila ODM bukan `.app` (butuh
  bundle identifier), dan di Linux bila tidak ada session DBus. Bila
  sebuah service gagal start, Wails membatalkan start seluruh aplikasi.
- Pemantau clipboard di luar Windows (`internal/clipmon/clipmon_other.go`)
  selama ini dipoll lewat Fyne; tanpa Fyne ia diam.
- Di luar Windows, `systemDark()` selalu false, `workArea()` tidak tahu
  ukuran layar, dan suara selesai/gagal tidak ada (`*_other.go` di
  `internal/gui`). Semuanya aman, hanya kurang rapi.
- yt-dlp diunduh otomatis juga untuk macOS dan Linux; ffmpeg hanya untuk
  Windows. Ini urusan engine, sama untuk kedua UI.
- Tray Wails di Linux tidak punya tooltip: label tray dipakai sebagai nama
  ikon (Id, Title) dan tooltip-nya, dan tanpa label namanya "Wails".
  (Ditemukan saat uji jalan.)

**Wajib**, supaya bisa start:
1. Notifikasi "kalau bisa": service notifikasi dibungkus supaya gagal start
   tidak membatalkan ODM, dan `desk.notify` diam bila notifikasi tidak
   tersedia (`app.go`, `tray.go`). Binding halaman untuk service itu
   dibuat ulang bila namanya berubah.
2. Cara build: `scripts/build.sh` mendapat `linux-gui` dan `darwin-gui`
   (halaman dulu, lalu `go build -tags gui,production` dengan cgo).
   Kebutuhan build: Linux `libgtk-4-dev libwebkitgtk-6.0-dev`, macOS Xcode
   command line tools (sudah dipakai untuk Fyne). CI mendapat job yang
   menghasilkan artefak pratinjau. Pengguna Linux butuh WebKitGTK 6.0, yang
   ada di Ubuntu 24.04+, Debian 12+, dan Fedora 39+; Ubuntu 22.04 tidak.

**Disarankan**, juga kecil:
3. Bingkai jendela bawaan sistem di Linux/macOS (`Frameless` hanya di
   Windows); halaman menyembunyikan tombol jendelanya di sana, jadi perlu
   tahu platformnya (misalnya lewat `Appearance`). Hasilnya dua baris di
   atas, bingkai sistem dan bar pencarian: tidak dipoles, tetapi seret,
   ubah ukuran, dan tutup jendela pasti jalan.
4. Pemantau clipboard lewat polling clipboard Wails (sekitar 20 baris), atau
   dibiarkan mati di Linux/macOS dan dicatat di README.
5. Jendela Setup di Linux/macOS: bagian browser diganti petunjuk ke halaman
   rilis (berkas `.xpi` untuk Firefox, `.zip` untuk *Load unpacked*),
   supaya tidak tertulis "tidak ada browser".

**Dibiarkan** (tetap jalan, hanya kurang rapi):
- label "Ctrl" dan "Same as Windows", suara, ukuran jendela ke layar, dan
  kilatan putih saat jendela dibuka;
- macOS tanpa `.app`, ikon `.icns`, maupun codesign/notarisasi: dibuka
  dengan klik kanan → Open seperti binary Fyne sekarang, dan tanpa
  notifikasi;
- Linux tanpa berkas `.desktop` atau paket; tray di GNOME butuh ekstensi
  AppIndicator, sama seperti Fyne.

**Rilis:** awalnya Fyne tetap menjadi rilis Linux/macOS dan UI baru hanya
artefak pratinjau CI (`odm-next-linux-amd64`, `odm-next-darwin-arm64`).
Setelah uji jalan, rilisnya dialihkan di Tahap 8 (2026-10-18): UI baru
menjadi `odm`, dan Fyne tetap ada sebagai `odm-linux-*-fyne` untuk Linux
tanpa WebKitGTK 6.0.

**Pengujian:**
- Linux: WSL Ubuntu di komputer pengembang (26.04). Paket build
  (`golang-go build-essential pkg-config libgtk-4-dev
  libwebkitgtk-6.0-dev`) dipasang atas izin pemiliknya lewat
  `wsl -u root`. Yang diuji: start, jendela utama, menambah unduhan, tray,
  notifikasi, clipboard, Setup, dan Opsi.
- macOS: CI membangun dan menjalankan tes; pemiliknya tidak punya Mac, jadi
  belum ada uji jalan.

**Hasil uji Linux (2026-10-18),** binary `-tags gui,production` yang
dibangun di WSL. Sebagian besar di Xvfb (X11) dengan openbox sebagai
window manager, plus session DBus sendiri yang berisi host tray
(StatusNotifierWatcher) dan server notifikasi tiruan, supaya tray dan
notifikasi bisa diperiksa lewat DBus. Ditambah satu smoke test di WSLg
(Wayland).
- Start pertama membuka jendela Setup dengan petunjuk dan tombol halaman
  rilis; start kedua tidak membukanya lagi, dan daftar unduhan kembali.
- Jendela utama, New Download (probe ukuran dan tipe), Download complete,
  dan Opsi tampil benar. Bingkai sistem tampil, dan tombol jendela halaman
  tersembunyi.
- Unduhan dari server lokal selesai dan isinya identik dengan sumbernya.
- Tray terdaftar di host tray. Menunya lengkap; "Add URL…", "Show ODM",
  "Exit", dan klik kiri ikon berjalan. Tombol × menyembunyikan jendela ke
  tray. Menjalankan ODM lagi memunculkan jendelanya ("ODM is already
  running.").
- Notifikasi "ODM is still running" dan "Download failed" terkirim, dan
  klik pada notifikasi memunculkan jendela. "Download complete" sengaja
  tidak dikirim bila jendelanya terbuka.
- Clipboard: setelah dinyalakan di Opsi, URL yang disalin langsung
  terunduh.
- Tanpa session DBus sama sekali, ODM tetap start; notifier mencatat
  `notifications: failed to connect to session bus` di `odm.log`.
- WSLg (Wayland): jendela tampil dengan bingkai GTK 4. Di sana tidak ada
  host tray maupun server notifikasi, jadi jendela yang ditutup baru bisa
  dimunculkan dengan menjalankan ODM lagi (README sudah menyebutnya untuk
  GNOME tanpa AppIndicator).
- Diperbaiki: ikon tray bernama "Wails"; sekarang di Linux ODM memberi
  tray label "Ochi Download Manager".
- Catatan: WebKitGTK sendiri crash bila `DBUS_SESSION_BUS_ADDRESS` menunjuk
  ke socket yang tidak ada, karena sandbox-nya (bwrap) gagal memasang
  socket itu. Ini perilaku WebKit, bukan ODM, dan tidak terjadi bila
  variabelnya kosong.

**Keputusan (2026-10-18):**
- butir 3, 4, dan 5 dipakai;
- Linux memakai GTK4 + WebKitGTK 6.0 (bawaan Wails), bukan tag `gtk3`;
- rilis Linux/macOS tetap Fyne, UI baru sebagai artefak pratinjau CI;
- paket build di WSL dipasang pemiliknya; macOS cukup lewat CI.

**Perkiraan:** setengah tahap UI: sekitar 3–5 berkas Go, sedikit di halaman,
`build.sh`, dan `ci.yml`.

**Kelanjutan**, di luar rencana minimal: `.app` dengan ikon dan
codesign/notarisasi (syarat notifikasi macOS; Apple Developer ID
berbayar), menu aplikasi sendiri, title bar *inset* di macOS, label ⌘,
bingkai Linux yang rapi, `.desktop`/AppImage, suara, tema awal jendela,
dan ukuran jendela ke layar.

## Risiko dan penanganannya

- **Wails v3 masih beta.** Versinya dipatok dan lapisan `internal/gui`
  dibuat tipis.
- **Title bar buatan sendiri.** Seret jendela memakai `--wails-draggable`
  (gerak jendela bawaan Windows), klik ganda dan tombol memanggil jendela.
  Snap layout di tombol Maximize tidak tersedia tanpa composition hosting;
  bisa dikembalikan bila Wails meneruskan UI Automation dalam mode itu.
- **Konsistensi desain shadcn/ui.** Token warna, jarak, dan set komponen
  dikunci di Tahap 1; layar baru hanya memakai komponen itu.
- **WebView2 tidak ada** di sebagian Windows 10 lama. Wails beta.30 tidak
  menawarkan pemasangnya; jendelanya hanya kosong. Karena itu ODM memeriksa
  runtime di registry sebelum membuka UI, lalu menjelaskan apa yang kurang
  dan menawarkan halaman unduhan Microsoft (`webview2_windows.go`). Hal ini
  juga dicatat di README.
- **Dua UI dirawat selama transisi.** Logika bersama ada di service dan
  `internal/i18n`. Setelah Tahap 2, UI Fyne hanya menerima perbaikan bug.
