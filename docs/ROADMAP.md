# Roadmap ODM (fork rigaz29)

Daftar pekerjaan untuk proyek ini, urut dari yang dikerjakan lebih dulu.
Centang `[x]` berarti sudah selesai dan sudah ada di `main`. Ukuran:
**K** kecil, **S** sedang, **B** besar.

Aturan yang dipegang untuk semua fitur: ODM meniru *perilaku* yang umum di
download manager desktop, tidak menyalin kode, aset, atau teks program lain.
Lisensi/registrasi, stream ber-DRM, driver capture, dan dekompilasi tidak
dikerjakan.

## Sudah selesai

- [x] Sebelas perubahan engine (PR #1–#11 ke upstream, masih terbuka)
      digabung ke `main`.
- [x] Upstream v0.3.0-rc1 (HLS/DASH) digabung ke `main`, lalu v0.3.1
      (lihat 0.1).
- [x] File `.odmpart` ditutup saat download berhenti; test ffmpeg DASH di
      Windows (PR #12 dan #13, sudah di-merge upstream).
- [x] Review engine, 9 bug diperbaiki: limiter macet di bawah 64 KB/s, file
      rusak pada server tanpa Range, state download hilang atau hidup lagi,
      crash saat membaca pengaturan, `settings.json` tidak atomik, proxy baru
      tidak berlaku, tanpa timeout koneksi/header, nama file Windows, stall
      pada segmen stream.

## Langkah 0: persiapan

- [x] **0.1** Merge upstream v0.3.1 ke `main` (perekaman live HLS, popup
      media di extension, playlist publik tidak boleh ke jaringan lokal, nama
      device Windows). **K**
- [x] **0.2** Pasang compiler C (gcc) supaya UI desktop Fyne bisa di-build
      dan dicek di mesin ini. **K** WinLibs GCC 16.2 (UCRT) terpasang lewat
      winget. `scripts/build.sh` kini mencegah manifest bawaan toolchain ikut
      tertanam di exe.

## Fase 1: fitur cepat

- [x] Cari di daftar: Ctrl+F dan kotak filter di toolbar. **K**
- [x] "Stop all" menghentikan juga antrean yang berjalan (sekarang sama
      dengan "Pause all"). **K** Termasuk tombol Stop All di web UI
      (`/api/actions/stop-all`).
- [x] Speed limiter on/off cepat dari menu dan tray. **K** Nilai tetap
      tersimpan saat dimatikan; status bar menampilkan batas yang aktif.
- [x] Notifikasi Windows dan suara saat download selesai atau gagal. **K**
      Diatur di Options → Downloads.
- [x] Temuan saat aplikasi dijalankan, sudah diperbaiki: toolbar terpotong
      oleh kotak pencarian, baris tanpa nama saat probe gagal, urutan daftar
      teracak setelah restart, "Last attempt" berisi jam restart, posisi
      antrean bentrok untuk download baru setelah restart.
- [ ] Batch download dengan pola (`foto[001-120].jpg`, `[a-z]`) dan batch
      dari clipboard. **S**
- [x] Ingat folder simpan terakhir di jendela New Download. **K** Kembali ke
      folder default kalau folder itu hilang atau default diganti; bisa
      dimatikan di Options → Save to.
- [ ] Folder sementara terpisah. **K**
- [x] Verifikasi checksum SHA-256/MD5 setelah selesai. **K–S** Dikerjakan
      di Fase 5.
- [ ] Jalankan pemindai antivirus setelah selesai (program dan argumen bisa
      diatur). **K** Sebagian: Attachment Services (Fase 5) sudah memicu
      antivirus yang terdaftar di Windows; program pilihan sendiri belum.

## Fase 2: fitur besar

- [x] Antrean sinkronisasi: cek berkala apakah file di server berubah
      (ETag/Last-Modified), lalu unduh ulang. **S–B** File baru menimpa
      file lama setelah selesai; rename dicoba ulang kalau file lama
      sedang dibuka program lain.
- [x] Scheduler: ulang tiap N menit/jam; sleep/hibernate setelah selesai. **S**
- [x] Kategori bisa ditambah dan diedit (ekstensi dan folder). **S**
- [x] Preview isi ZIP sebelum diunduh (Range ke central directory). **S–B**
      Hanya membaca ujung arsip (sekitar 1 KB untuk arsip 2,5 MB).
- [ ] Drop target melayang untuk link dari browser (cek dukungan Fyne
      dulu). **S** **Dilewati** (keputusan 2026-10-15). GLFW 3.4 di Windows
      hanya menerima drop file (`WM_DROPFILES`), bukan link dari browser;
      butuh implementasi COM `IDropTarget` sendiri (**B**).
- [x] Engine: `429`/`Retry-After`, file tanpa `Content-Length`, Digest
      auth. **S** Sekaligus memperbaiki Site logins yang tidak pernah
      bekerja untuk file yang butuh login (probe dikirim tanpa login).

## Fase 3: modernisasi dan perapian UI

- [x] Tema terang dan gelap mengikuti setelan Windows, pilihan warna
      aksen. **S** Options → General → Appearance: Same as Windows / Light /
      Dark, 8 warna aksen. Disimpan di preferences Fyne
      (`%USERPROFILE%\AppData\Roamingyne\com.ochi.odm`, bukan
      `%APPDATA%`).
- [x] Kolom bisa disembunyikan/ditampilkan; lebar dan sortir diingat. **S**
      Klik kanan header atau View → Columns; drag tepi header untuk lebar.
      Disimpan di preferences Fyne. Catatan teknis: `widget.Table` menutup
      header dengan lapisan divider yang menangkap semua drag, jadi resize
      memakai lapisan grip transparan sendiri di atas baris header.
- [x] Status bar: total kecepatan, jumlah aktif, status limiter. **K**
      Kiri: jumlah item, hasil pencarian, jumlah yang dipilih. Kanan:
      jumlah aktif dan kecepatan total, jumlah antre, batas kecepatan.
- [x] Dialog Add URL dengan kartu hasil probe; jendela progress dengan
      grafik kecepatan. **S** Kartu menggantikan baris "File info" (ikon tipe,
      nama, ukuran, tipe, server, badge resumable, spinner, error). Grafik
      60 detik terakhir di tab Download status.
- [x] Shortcut keyboard: Del, Ctrl+N, Ctrl+F, Space. **K** Ditambah
      Shift+Del (hapus beserta file), label shortcut di menu, dan Help →
      Keyboard shortcuts. Sekalian menghapus item "Quit" ganda di menu Tasks.
- [x] Web UI diseragamkan dengan desktop, responsif, mode terang. **S**
      Palet, ikon, toolbar, pohon kategori, kolom, pencarian, status bar,
      shortcut, kartu probe, grafik, laci kategori dan kartu di layar
      sempit. Daftar dicek visual (gelap, terang, 390 px). Dialog web (New
      Download, Options, Progress) tidak dicek visual: dilewati atas
      keputusan pengguna (2026-10-15).
- [x] Bahasa Indonesia sebagai bahasa kedua. **B** `T()`/`Tf()` di semua
      teks desktop (392 teks, `translations/id.json`, termasuk status
      integrasi browser), teks bawaan Fyne (`fyne.id.json`), pilihan bahasa
      di Options → General (ikut sistem / English / Bahasa Indonesia,
      berlaku setelah restart). Test memastikan setiap teks punya terjemahan
      dan placeholder `%d`/`%s` cocok. Dicek visual: jendela utama, menu,
      Options, New Download, Penjadwal, menu klik kanan. Label toolbar yang
      panjang dipecah dua baris. Masih Inggris: web UI, pesan error dari
      engine, dan nama kategori/antrean (data milik pengguna).

## Fase 4: perapian teknis

- [x] Test engine memakai data acak, bukan pola `byte(i)` yang berulang
      tiap 256 byte. **K** 12 tempat di 9 file test kini memakai
      `randomData(n)`.
- [x] Batas stall untuk body manifest dan kunci AES. **K** Keduanya dibaca
      lewat `readAllIdle` dengan batas yang sama seperti segmen (30 detik).
- [x] Map `bypass` di extension dibersihkan dari entri lama. **K** Setiap
      Alt-click baru membuang tanda yang lebih tua dari 10 detik
      (`pruneBypass`, ada test-nya).
- [x] `/api/settings` bisa diubah sebagian (PATCH). **K** PUT dan PATCH
      sama-sama hanya mengubah field yang dikirim. Ditemukan saat merapikan
      web UI: Save di Options web menghapus queue, jadwal, login, dan proxy.
- [x] Perbandingan nama antrean konsisten (tidak peka huruf besar). **K**
      Nama antrean dikanonkan ke ejaan di settings saat unduhan dibuat,
      dipindah, diimpor, dan dipulihkan; settings membuang ejaan ganda
      (jadwalnya ikut pindah); `RemoveQueue`, `QueueRunning`, dan
      `QueueRepeating` tidak peka huruf besar; antrean utama selalu `Main`.

## Fase 5: hasil perbandingan dengan downloader open-source

Dari perbandingan dengan aria2, Gopeed, dan wget2 (2026-10-15): ODM sudah
setara untuk kecepatan dan stabilitas, tetapi ada celah berikut.

- [x] Tandai file hasil unduhan dengan Mark-of-the-Web (`Zone.Identifier`)
      lewat Attachment Services Windows (`IAttachmentExecute`), yang juga
      memicu pemindai antivirus terdaftar. **S** Paket `internal/quarantine`,
      dipasang lewat `engine.MarkFile` di `main`. Dicek end-to-end:
      `ZoneId=3` + `HostUrl`. Saat mengujinya ketemu bug besar: unduhan
      multi-koneksi dari server HTTPS ber-HTTP/2 selalu gagal (sudah
      diperbaiki).
- [x] Buang karakter kontrol arah teks (RTLO dan sejenisnya) dari nama file,
      agar `foto‮gpj.exe` tidak tampil sebagai `fotoexe.jpg`. **K** Di
      `SanitizeName`, yang dilalui nama dari server, API, dan form.
- [ ] `fsync` berkala sebelum progres disimpan, agar resume setelah listrik
      mati tidak meninggalkan lubang di file. **K** Ditunda.
- [x] Validasi akhir dan total ukuran di `Content-Range`, bukan hanya byte
      awal. **K** Total yang berubah = file di server berganti → unduh ulang
      dari awal. Akhir range yang melebihi permintaan tetap diterima.
- [x] Verifikasi checksum SHA-256/SHA-1/MD5 setelah selesai (= item Fase 1). **S**
      Juga SHA-512. Dicek sebelum rename; tidak cocok → tetap `.odmpart`,
      error menyebut kedua hash, Resume mengunduh ulang semua byte. Ada di
      form desktop, web UI, API, dan `-add -checksum`.
- [x] Koneksi yang jauh lebih lambat dari yang lain diambil alih seluruh
      sisanya, agar tidak ada "ekor" di akhir unduhan. **S** Rasio ≥4×, sisa
      ≥16 KB yang butuh >1 detik. Koneksi cepat yang menganggur menunggu
      (cek tiap 250 ms) selama yang lain masih bekerja. Uji lokal: 24 MB,
      satu dari 8 koneksi 16 KB/s, 9,1 s → 3,1 s.

## Fase 6: rekam siaran langsung

Tujuan: siaran langsung dari banyak platform (Instagram, TikTok, Facebook,
YouTube, Twitch, situs TV/berita) bisa direkam dari saat tombol Rekam
diklik sampai Stop, lalu otomatis digabung menjadi satu file MP4 dengan
ffmpeg, tanpa encode ulang.

Rancangan (disepakati 2026-10-16):

- Deteksi lewat dua jalur. Pertama, ekstensi melihat playlist atau aliran
  live yang dimuat pemutar; jalur ini paling tahan terhadap perubahan situs
  karena memakai apa yang dimuat pemutar yang sudah login. Kedua, yt-dlp
  memberi alamat siaran (dan header-nya) dari halaman platform yang
  dikenalnya.
- ODM sendiri yang merekam, bukan yt-dlp: di Windows yt-dlp hanya berhenti
  rapi dengan Ctrl+C, yang tidak bisa dikirim dengan andal oleh program
  GUI; kalau dibunuh, file live bisa tidak tertutup. Perekam ODM
  (`live.go`) sudah menangani Stop = selesai + gabung, berhenti sendiri
  saat siaran berakhir atau diam, menyambung setelah ODM ditutup, dan
  membaca ulang playlist master saat token kedaluwarsa.
- Hasil akhir MP4 (remux `-c copy`). Codec yang tidak muat di MP4 (jarang)
  disimpan sebagai MKV; encode ulang tidak dipakai karena ffmpeg ringan
  ODM tidak punya encoder.
- Ide dari referensi (N_m3u8DL-RE, nurrony/hlsdownloader,
  puemos/hls-downloader, Logosww/hls-downloader): pilih kualitas dan trek
  audio sebelum merekam, gabung ke MP4 setelah selesai
  (`--mux-after-done`), berhenti saat siaran diam (`--live-idle-timeout`,
  sudah ada), batas durasi rekaman (`--live-record-limit`), menulis fMP4
  bertahap selama merekam (Logosww).

Tahap 1: dasar (HLS)

- [x] Hasil rekaman (dan unduhan HLS) satu jalur MPEG-TS dikemas ulang ke
      MP4 setelah selesai; gagal → MKV; tanpa ffmpeg → tetap TS. **K**
      `remuxTS` sebelum `finish`, juga untuk rekaman yang dihentikan.
      Dicek dengan ffmpeg penuh dan ffmpeg ringan ODM (TS → MP4 dan MKV).
- [x] Siaran live dari halaman yang dibaca yt-dlp (YouTube Live, Twitch,
      TikTok bila ada HLS): yt-dlp memberi alamat playlist HLS dan
      header-nya, ODM merekam dengan perekamnya sendiri, dan meminta
      alamat baru ke yt-dlp saat alamat lama kedaluwarsa (juga saat Resume
      berjam-jam kemudian). **S** `StreamJob.Page`; `/api/probe/site`
      mengembalikan `live`, `manifest`, `headers` dan kualitas playlist.
      Dicek dengan siaran NASA di YouTube: menu ekstensi → 720p60 → rekam
      ~40 detik → Hentikan → MP4 H.264 720p + AAC.
- [x] Ekstensi menandai siaran langsung: tombol "● Record live", judul
      "LIVE", dan baris kualitas dengan keterangan bahwa perekaman berjalan
      sampai dihentikan. **K** Label tombol berubah setelah ODM menjawab
      (saat menu dibuka); ODM tidak ditanya hanya karena halaman dibuka.

Tahap 2: aliran kontinu (TikTok)

- [x] Perekam aliran HTTP tanpa akhir (FLV, kadang TS) sampai Stop. **S**
      `recordHTTPLive` (`livehttp.go`): respons disalin ke bagian p1,
      p2… di folder kerja; koneksi putus → bagian baru; alamat ditolak
      (403/404/410) → alamat baru dari yt-dlp; berhenti saat Stop atau
      saat lama tidak ada data; respons berukuran tetap yang tuntas
      dianggap file, bukan siaran. Bagian digabung dengan demuxer concat
      ffmpeg ke MP4 (gagal → MKV; tanpa ffmpeg bagian disimpan apa
      adanya). Ekstensi: FLV tanpa ukuran = "● LIVE · FLV stream";
      halaman TikTok (`/@user/live`, `/@user/video/ID`) lewat yt-dlp,
      dengan urutan HLS → DASH → FLV untuk siaran. Dicek dengan siaran FLV
      publik Bilibili lewat host native (`kind: "live"`): ~35 detik →
      Hentikan → MP4 H.264 720p + AAC. Lewat ekstensi di Chrome uji: FLV
      tanpa ukuran → "● LIVE · FLV stream" → Record → koneksi diputus
      tiap 10 detik → 3 bagian → MP4 30 detik. Saat diuji tidak ada akun
      TikTok publik yang sedang live; jalurnya sama, alamatnya dari
      yt-dlp.
- [x] Demuxer FLV di ffmpeg ringan (rilis `ffmpeg-min-n8.1-2`), lalu remux
      FLV → MP4. **K** Ditambah demuxer `flv`, `live_flv` dan `concat`;
      dicek: dua bagian FLV → satu MP4 6 detik. ODM yang memakai ffmpeg
      unduhan sendiri memperbaruinya otomatis.

Tahap 3: DASH live (Instagram, Facebook)

- [x] Perekam DASH live: MPD dinamis, `SegmentTemplate`/`SegmentTimeline`,
      `availabilityStartTime`, muat ulang manifest. **B** `dashlive.go`
      memakai loop, state `live.json`, aturan berhenti dan penggabungan
      yang sama dengan perekam HLS. Segmen dikenali dari waktu mulainya
      (URL bisa ditandatangani ulang tiap muat), init dari period +
      Representation. Mulai dari ujung siaran (3 segmen terakhir), muat
      ulang tiap `minimumUpdatePeriod`, selesai saat MPD jadi statis.
      `media`: template berbasis nomor dan `r="-1"` dijabarkan memakai jam
      sampai sekarang (minus satu segmen cadangan), paling jauh 10 menit
      ke belakang atau sepanjang `timeShiftBufferDepth`. Dicek dengan
      simulator DASH-IF (`livesim2`, template nomor dan SegmentTimeline):
      42 detik video + audio, tanpa celah; juga lewat menu ekstensi
      ("● LIVE · DASH stream" → Record · 360p). Siaran Facebook/Instagram
      sungguhan belum dicoba: butuh akun yang login.
- [x] ~~Cookie login dari browser untuk platform yang mewajibkan login.~~
      Tidak diperlukan (diputuskan 2026-10-16): Instagram dan Facebook
      direkam lewat jalur browser. Pemutar yang sudah login memuat MPD
      bertanda tangan; ekstensi melihatnya dan ODM merekamnya dengan
      cookie situs untuk alamat itu, seperti unduhan lain. Cookie tidak
      pernah diberikan ke yt-dlp. Referensi PyInstaLive (login dengan
      password, API privat) dan tiktok-live-recorder (API pihak ketiga
      berkunci) tidak dipakai karena alasan itu.

Nanti

- [ ] Batas durasi rekaman ("berhenti setelah 1 jam"), ide dari
      `--live-record-limit`. **K**
- [ ] Menulis MP4 terfragmentasi bertahap selama merekam, agar ruang disk
      saat penggabungan tidak perlu 2× ukuran rekaman. **S**

Batasan: rekaman mulai dari saat klik (ditambah cadangan yang masih ada di
playlist), siaran ber-DRM/berbayar tidak didukung, dan platform yang sering
berubah (TikTok, Instagram) bisa sesekali rusak sampai yt-dlp atau ODM
diperbarui. Merekam siaran orang lain bisa melanggar hak cipta, ketentuan
layanan, atau privasi: tanggung jawab pemakai.

## Fase 7: UI Fluent (Windows 11) dengan Wails

Tujuan: UI dengan tampilan Windows 11 (Fluent 2, Mica, ikon dan font
Windows), tetap Go dan satu exe. Opsi C dipilih pada 2026-10-16. Rencana
lengkap (alasan, arsitektur, build, uji, risiko) ada di
[FLUENT-UI.md](FLUENT-UI.md).

Pilihan teknis: Wails v3 (beta, karena v2 tidak punya banyak jendela dan
tray) + WebView2 + React 19 + Fluent UI React v9 (punya DataGrid dan Tree)
+ Vite. UI Fyne tetap menjadi bawaan sampai UI baru setara fiturnya.

- [ ] **Tahap 0** Prototipe jendela utama di cabang `ui-fluent`: daftar
      dari engine sungguhan, toolbar, kategori, status bar, Mica,
      gelap/terang. Diukur ukuran exe, memori, dan kelancaran daftar 1.000
      item; tangkapan layarnya dibandingkan dengan UI Fyne. Pengguna
      memutuskan lanjut atau tidak. **S**
- [ ] **Tahap 1** Fondasi: paket bersama `uicore` dan `i18n`, service dan
      event untuk semua fungsi engine yang dipakai UI, `go.mod` naik ke Go
      1.25, job CI (Node + Vitest + `-tags fluent`), `scripts/build.sh
      fluent`, `ODM-Fluent.exe` pratinjau. **S**
- [ ] **Tahap 2** Jendela utama setara: DataGrid (kolom, lebar, sortir,
      klik/Ctrl/Shift/Ctrl+A, klik kanan, klik ganda), toolbar, kategori
      dan antrean, pencarian, status bar, pembatas kecepatan, pintasan,
      menu. **B**
- [ ] **Tahap 3** Dialog dan jendela: Unduhan Baru (probe, kualitas stream,
      yt-dlp, live), Progres, Opsi (6 tab), Penjadwal, Kategori, Program
      pendamping, Unduhan selesai, Impor/Ekspor, pratinjau ZIP, perbarui
      alamat, Tentang, Pintasan. **B**
- [ ] **Tahap 4** Integrasi Windows: tray, notifikasi, `-hidden`, prompt dari
      ekstensi (selalu di atas), mulai bersama Windows, clipboard, cek
      pembaruan, suara. **S**
- [ ] **Tahap 5** Perapian: Narrator dan keyboard, DPI tinggi, Windows 10
      (tanpa Mica), terjemahan lengkap, performa dan memori. **S**
- [ ] **Tahap 6** Alih: UI Fluent menjadi `ODM.exe` bawaan di Windows; Fyne
      tetap untuk macOS/Linux. README, CHANGELOG, tangkapan layar. **K**

Nanti: frontend yang sama untuk web UI `-serve`, Wails untuk macOS/Linux,
title bar buatan sendiri.

## Catatan kerja

- Go portable: `C:\Users\rigaz\AppData\Local\go-sdk\go\bin`, dengan
  `GOTOOLCHAIN=local`. Test: `go test ./... -count=1`, lalu `go vet ./...`.
- Build desktop (Git Bash): `bash scripts/build.sh windows`, hasilnya di
  `dist/ODM.exe`. gcc dari WinLibs sudah ada di PATH user. Build pertama
  sekitar 6 menit (GLFW dikompilasi), berikutnya beberapa detik.
- Jangan build desktop dengan `go build -tags fyne` langsung: manifest
  bawaan WinLibs ikut tertanam dan bertabrakan dengan manifest ODM. Kalau
  perlu, tambahkan `-ldflags "-extldflags=-B<folder>/"` dengan
  `default-manifest.o` kosong, seperti yang dilakukan `build.sh`.
- Build tanpa UI: `go build -o odm.exe ./cmd/odm`.
- Test dengan UI: `go test -tags fyne ./...` (perlu `CGO_ENABLED=1`).
- Menjalankan aplikasi untuk dicek tanpa menyentuh data asli: set
  `APPDATA` ke folder lain (ODM memakai `%APPDATA%\odm`) dan beri `-dir`.
  Path folder itu harus pendek, karena socket kontrol gagal dibuat kalau
  path-nya lebih dari sekitar 100 karakter.
- Riwayat dan aturan lengkap ada di branch `docs/transfer-handoff`.
