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
- **Title bar**: jendela `Frameless`, ditambah `NonClientRegionSupport` dan
  properti CSS `--wails-non-client-region` (`caption`, `minimize`,
  `maximize`, `close`). Dengan begitu menyeret jendela, klik ganda untuk
  maximize, dan snap layout Windows 11 tetap dikerjakan Windows sendiri.
  Di Wails beta.30 wilayah itu baru dibaca jika WebView2 di-host lewat
  composition (`WebView2CompositionHosting`). Tanpa itu halaman berada di
  jendela anak, dan bingkai jendela tidak pernah menerima kursor.
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
- Build: `bash scripts/build.sh gui`. Isinya `npm ci`, lalu `npm run build`
  di `internal/gui/frontend`, lalu `go build -tags gui` tanpa CGO, dan
  hasilnya `dist/ODM-Next.exe`.
- Binding hasil generate di-commit; `frontend/dist` tidak. Build dan tes
  tanpa tag tetap jalan tanpa Node.
- Selama transisi `ODM.exe` (Fyne) dan `ODM-Next.exe` memakai data yang
  sama, tetapi hanya satu yang bisa berjalan sekaligus.
- CI: job baru menjalankan `npm ci`, Vitest, `vite build`, lalu `go vet` dan
  `go test` dengan `-tags gui`.

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
- Title bar buatan sendiri dengan wilayah non-client.
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
- UI baru menjadi `ODM.exe` bawaan di Windows.
- Fyne tetap dipakai untuk macOS/Linux sampai dievaluasi.

## Risiko dan penanganannya

- **Wails v3 masih beta.** Versinya dipatok dan lapisan `internal/gui`
  dibuat tipis.
- **Title bar buatan sendiri.** Seret jendela, klik ganda, dan snap layout
  diserahkan ke Windows lewat wilayah non-client. Sudah dicek di Tahap 1
  dan berfungsi dengan composition hosting. Untuk WebView2 tanpa wilayah
  non-client, halaman memakai `--wails-draggable` dan memanggil jendela
  sendiri.
- **Konsistensi desain shadcn/ui.** Token warna, jarak, dan set komponen
  dikunci di Tahap 1; layar baru hanya memakai komponen itu.
- **WebView2 tidak ada** di sebagian Windows 10 lama. Wails bisa menawarkan
  pemasangnya; hal ini dicatat di README.
- **Dua UI dirawat selama transisi.** Logika bersama ada di service dan
  `internal/i18n`. Setelah Tahap 2, UI Fyne hanya menerima perbaikan bug.
