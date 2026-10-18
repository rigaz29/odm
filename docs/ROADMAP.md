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
- [x] `fsync` berkala sebelum progres disimpan, agar resume setelah listrik
      mati tidak meninggalkan lubang di file. **K** Saat unduhan berjalan,
      progres disimpan paling sering tiap 5 detik, selalu setelah
      `.odmpart` di-flush; state diambil sebelum flush, jadi hanya
      menghitung byte yang sudah ditulis. Pause, error, dan perubahan lain
      langsung di-flush lalu disimpan. File state `<id>.json` juga
      di-flush sebelum menggantikan yang lama.
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

## Fase 7: UI baru (shadcn/ui + sentuhan Aurora) dengan Wails

Tujuan: jendela yang modern, rapi, dan konsisten, tetap Go dan satu exe.
Gaya dipilih pengguna pada 2026-10-16: **A (shadcn/ui) ditambah sentuhan D
(Aurora)**, lihat mockup `mockups/ad.html` di cabang `ui-mockups`. Rencana
lengkap (riwayat, bahasa desain, arsitektur, build, uji, risiko) ada di
[GUI.md](GUI.md).

Pilihan teknis: Wails v3 + WebView2 + React 19 + Tailwind 4 + shadcn/ui
(Radix) + TanStack Virtual + Lucide + Inter. UI Fyne tetap menjadi
bawaan sampai UI baru setara fiturnya.

- [x] **Tahap 0** Kerangka dan gaya. **S** Prototipe Fluent (cabang
      `ui-fluent`) membuktikan kerangka Wails: exe 15,4 MB tanpa CGO,
      scroll 1.023 item 60 fps, memori ±365 MB (Fyne 338 MB). Tampilan
      Fluent ditolak. Tiga mockup (cabang `ui-mockups`) dibandingkan, dan
      gaya A+D dipilih.
- [x] **Tahap 1** Fondasi: cabang `ui-next`, `internal/fluent` →
      `internal/gui` (tag `gui`), Tailwind 4 + komponen shadcn/ui + token
      tema A+D, title bar buatan sendiri dengan wilayah non-client (seret,
      klik ganda, snap layout), katalog terjemahan bersama `internal/i18n`,
      service ditambah data koneksi, riwayat kecepatan, dan lama rekaman,
      jendela utama pertama sesuai mockup dengan data engine, job CI,
      `scripts/build.sh gui`, `ODM-Next.exe` pratinjau. **S** Hasilnya:
      exe 14,7 MB tanpa CGO, scroll 1.000 item 60 fps. Title bar dicek di
      exe: seret, klik ganda, tombol jendela, dan snap layout. Wilayah
      non-client Wails baru berfungsi dengan `WebView2CompositionHosting`.
      Tes `internal/i18n` memeriksa terjemahan kedua UI.
- [x] **Tahap 2** Jendela utama setara: tabel (sortir, lebar dan kolom
      tampil/sembunyi diingat, klik/Ctrl/Shift/Ctrl+A, klik kanan, klik
      ganda, pintasan), kartu "Sedang berjalan" (paling banyak 3, +N
      lainnya, bisa diciutkan), sidebar dengan jumlah, tab filter, palet
      Ctrl K, status bar, pembatas kecepatan. **B** Hasilnya: kolom
      Antrean, Terakhir dicoba, dan Keterangan ditambahkan. Lebar kolom,
      kolom yang tampil, dan urutan tetap tersimpan setelah restart. Saat
      tabel lebih lebar dari jendela, kolom centang, Nama, dan menu baris
      tetap terlihat. Menu baris kini punya Salin alamat, Pindah ke
      antrean, Naikkan/Turunkan, dan Hapus + hapus file (dengan
      konfirmasi). Menu "Lainnya" berisi aksi untuk semua unduhan,
      pembatas, kolom, dan sidebar. Palet Ctrl K menjalankan perintah dan
      menerima alamat yang ditempel. Pembatas kecepatan ada di status bar.
      Tombol Home/End/PgUp/PgDn dan Ctrl+C juga ditambahkan. Sortir lewat
      header ternyata tidak pernah berfungsi di TanStack Table (kolom
      tanpa accessor), jadi kini halaman mengurutkan sendiri dan TanStack
      Table dilepas.
- [x] **Tahap 3** Dialog dan jendela: Unduhan Baru (probe, kualitas
      stream, yt-dlp, live), Progres (peta koneksi, grafik), Opsi, Penjadwal,
      Kategori, Program pendamping, Unduhan selesai, Impor/Ekspor,
      pratinjau ZIP, perbarui alamat, Tentang, Pintasan. **B** Dikerjakan
      dalam tiga bagian:
      - 3a: kerangka multi-jendela (satu halaman, `?view=`, bingkai
        Windows), Unduhan Baru, Progres, dan Unduhan selesai. Logika
        probe dan pembuatan unduhan dipindah ke Go (`internal/gui/add.go`).
      - 3b: jendela Opsi dengan 8 bagian. Tema dan bahasa kini tersimpan
        di settings ODM dan langsung berlaku di semua jendela.
      - 3c: Penjadwal, Antrean baru/hapus, Site Grabber, Impor/Ekspor,
        Pintasan, cek pembaruan, dan Tentang.

      Ditemukan bahwa UI baru belum menjalankan aksi penjadwal: membuka
      file, peringatan batas unduhan, dan hitung mundur 30 detik sebelum
      keluar/tidur/matikan komputer. Sekarang sudah ditangani. Semua diuji
      di exe dengan klik dan keyboard nyata: hand-off dari browser lewat
      socket kontrol, unduhan sampai jendela selesai, validasi jam di
      Penjadwal, batas unduhan, hitung mundur yang dibatalkan, dan ekspor
      lewat dialog Save As.
- [x] **Tahap 4** Integrasi Windows: tray, notifikasi, `-hidden`, prompt dari
      ekstensi (selalu di atas), mulai bersama Windows, clipboard, cek
      pembaruan, suara. **S** Hasilnya:
      - tray dengan menu seperti UI Fyne, dan menutup jendela
        menyembunyikannya ke tray;
      - `-hidden`, notifikasi Windows saat unduhan selesai atau gagal,
        suara, dan pertanyaan pertama kali soal program pendamping;
      - ikon ODM yang digambar di kode untuk tray, jendela, dan taskbar.

      Prompt dari ekstensi, mulai bersama Windows, dan cek pembaruan
      sudah ada sejak Tahap 3; pemantau clipboard berjalan di `main.go`
      untuk UI mana pun. Unduhan yang gagal sebelum sempat terlihat
      berjalan (misalnya dari browser) kini tetap diberi notifikasi. Diuji
      di exe: `-hidden`, menu tray, tutup ke tray, instans kedua, dan
      notifikasi di pusat notifikasi. Kunci registry toast uji (HKCU) lalu
      dihapus lagi. Toast belum berikon karena Wails mengambilnya dari
      resource ikon exe (ID 3); diselesaikan di Tahap 5.
- [x] **Tahap 5** Perapian: Narrator dan keyboard, DPI tinggi, animasi
      halus, terjemahan lengkap, performa dan memori. **S** Hasilnya:
      - ikon ODM di resource exe (ID 3), sehingga Explorer, taskbar, dan
        notifikasi Windows berikon; `scripts/winres.sh` membuat ulang
        `rsrc_windows_amd64.syso` untuk kedua UI;
      - halaman hanya menerima perubahan daftar, bukan seluruh daftar.
        Dengan 1.200 unduhan dan 4 berjalan, kiriman turun dari 777 KB
        ke 4 KB, CPU ODM dari 24% ke 12% satu core, WebView2 dari
        sekitar 700 MB ke 470 MB. Daftar diam memakan 0,1%. Uji 10 menit
        tidak menunjukkan kebocoran memori;
      - Narrator membaca jendela utama: grid dengan jumlah baris penuh,
        baris yang dipilih, dan baris aktif keyboard, ditambah landmark,
        judul, dan nama tombol. axe bersih di semua jendela, terang dan
        gelap;
      - daftar menjadi satu perhentian Tab (panah dan Shift+panah memilih,
        Ctrl+A memilih semua), menu baris lewat tombol Menu atau
        Shift+F10, cincin fokus keyboard,
        dan kontras teks redup serta lencana dinaikkan ke 4,5:1;
      - animasi masuk dan keluar untuk menu, popover, tooltip, palet, dan
        dialog, yang mati bila Windows meminta gerak dikurangi;
      - setiap jendela dipas ke work area layar, sehingga tidak terpotong
        pada skala 150% atau 200%.

      Keputusan: WebView2 jendela utama tidak lagi di-host lewat
      composition, karena dalam mode itu Wails beta.30 tidak meneruskan
      UI Automation dan Narrator tidak bisa membaca apa pun. Seret
      jendela, klik ganda, ubah ukuran, Win+panah, dan seret ke tepi
      layar tetap berfungsi; snap layout di tombol Maximize (dan Win+Z)
      dilepas. Terjemahan lengkap dilewati atas permintaan; kunci baru
      tetap diterjemahkan dan diperiksa tes i18n.
- [x] **Tahap 6** Alih: UI baru menjadi `ODM.exe` bawaan di Windows; Fyne
      tetap untuk macOS/Linux. README, CHANGELOG, tangkapan layar. **K**
      Hasilnya (digabung dari `ui-next` ke `main` pada 2026-10-17):
      - `scripts/build.sh windows`, CI, dan rilis membangun `ODM.exe`
        Windows dengan tag `gui` tanpa cgo dan MinGW (exe 15,6 MB,
        sebelumnya 28,6 MB); `windows-fyne` masih membangun UI Fyne untuk
        Windows dan tetap diuji di CI;
      - tanpa WebView2 Runtime, ODM menjelaskan apa yang kurang dan
        menawarkan halaman unduhan Microsoft, alih-alih jendela kosong;
      - README (fitur desktop, WebView2, build dengan Node dan Go 1.25,
        pintasan, struktur kode, tangkapan layar UI baru) dan CHANGELOG.

- [x] **Tahap 7** Linux dan macOS, penyesuaian minimal: UI baru bisa jalan
      di sana tanpa dipoles (notifikasi "kalau bisa", target build
      `linux-gui`/`darwin-gui` dan artefak pratinjau CI, bingkai jendela
      sistem, clipboard lewat Wails). Fyne tetap jadi rilis sampai UI baru
      dicoba. Rencana dan keputusan: `docs/GUI.md`, "Rencana Linux dan
      macOS". **S** Selesai 2026-10-18: semua butir (1–5) di kode, Linux
      memakai GTK 4 + WebKitGTK 6.0, dan uji jalan di WSL Ubuntu 26.04
      lolos (X11 dan Wayland), lalu nama ikon tray di Linux dibetulkan.
      macOS hanya dibangun dan diuji di CI.
- [x] **Tahap 8** Alih di Linux dan macOS: UI baru menjadi rilis bawaan
      di sana, atas permintaan pemilik setelah uji jalan Tahap 7. **K**
      Selesai 2026-10-18:
      - `scripts/build.sh linux`, `darwin`, dan `darwin-intel` membangun UI
        baru; `linux-fyne`, `darwin-fyne`, dan `windows-fyne` membangun UI
        Fyne;
      - rilis memakai tag `gui` untuk Linux (amd64, arm64) dan macOS (Apple
        silicon, Intel); UI Fyne tetap dirilis untuk Linux tanpa WebKitGTK
        6.0 (misalnya Ubuntu 22.04) sebagai `odm-linux-*-fyne`, dibangun di
        Ubuntu 22.04;
      - CI membangun UI baru persis seperti rilis, termasuk Linux ARM dan
        Intel Mac, dan UI Fyne sebagai cadangan;
      - tema dan bahasa pilihan di UI Fyne ikut terbawa di start pertama,
        kini juga di Linux dan macOS (sebelumnya hanya jalur Windows);
      - README dan CHANGELOG.

Nanti: halaman yang sama untuk web UI `-serve`.

## Fase 8: unduhan torrent

Tujuan: ODM bisa mengunduh dari file `.torrent` dan tautan magnet seperti
unduhan lain: masuk antrean, terlihat progresnya, bisa dijeda dan
dilanjutkan, lalu ikut mengunggah (seeding) secukupnya setelah selesai.
Mesinnya libtorrent.

Rancangan (disepakati 2026-10-18; pemilik memilih libtorrent saja, tanpa
anacrolix/torrent):

- Mesin: **libtorrent** ([libtorrent.org](https://www.libtorrent.org/),
  libtorrent-rasterbar 2.1, BSD-3), yang dipakai qBittorrent dan Deluge.
  Isinya lengkap:
  - magnet, DHT, PEX, uTP, enkripsi, torrent v2, dan pilih file;
  - batas kecepatan per torrent;
  - proxy SOCKS5 untuk peer;
  - data lanjutan (*resume data*);
  - UPnP dan NAT-PMP.
- libtorrent berjalan sebagai program pendamping **`odm-libtorrent`**,
  bukan ditanam lewat cgo:
  - `odm-libtorrent` adalah program C++ kecil di atas libtorrent, dengan
    kodenya di repo (`helper/libtorrent/`, CMake + vcpkg, dibangun
    statis);
  - ODM berbicara dengannya lewat baris JSON di stdin/stdout, tanpa port
    kontrol. Program itu berhenti saat stdin tertutup. Di Windows ia juga
    diikat ke job object ODM, supaya ikut mati saat ODM crash;
  - alasannya: libtorrent butuh C++, Boost, dan OpenSSL. Kalau ditanam,
    ketujuh build rilis ODM butuh cgo dan toolchain C++, padahal
    `ODM.exe` Windows sekarang dibangun tanpa cgo. Crash di kode C++ juga
    akan menjatuhkan seluruh ODM. Sebagai program terpisah, `ODM.exe`
    tidak berubah dan tidak membesar, dan crash hanya menghentikan
    torrent;
  - di sisi ODM, paket Go baru `internal/torrent` menjalankan program itu
    dan menerjemahkan protokolnya. Paket ini dipasang dari `main` lewat
    hook seperti `EnsureTool` dan `MarkFile`, jadi engine tetap hanya
    memakai pustaka standar.
- Distribusinya mengikuti ffmpeg ringan:
  - workflow sendiri membangun lima target: Windows x64, Linux x64/ARM64
    (dibangun di Ubuntu 22.04), dan macOS Apple silicon/Intel;
  - rilisnya pre-release dengan tag `odm-libtorrent-p<protokol>-<versi>-<n>`
    (misalnya `odm-libtorrent-p1-2.1.2-1`), yang tidak pernah menjadi
    rilis terbaru yang dibaca cek pembaruan ODM. Paketnya berisi SHA-256
    serta lisensi libtorrent (BSD-3), Boost (BSL-1.0), dan OpenSSL
    (Apache-2.0);
  - nomor protokol ada di prefix tag, jadi ODM versi lama tidak pernah
    mengunduh `odm-libtorrent` yang protokolnya tidak ia kenal. Saat mulai,
    program itu juga menyebut versi protokolnya;
  - `tools.Keeper` mengunduhnya saat torrent pertama ditambahkan, dengan
    pertanyaan dan progres seperti yt-dlp dan ffmpeg, lalu
    memperbaruinya. Jendela Setup mencantumkannya sebagai pilihan, bukan
    bagian dari "pasang semua".
- Biaya:
  - `ODM.exe` tidak membesar;
  - ukuran unduhan `odm-libtorrent` dan lama build-nya diukur di awal
    Tahap 1. vcpkg mengompilasi Boost dan OpenSSL, jadi build pertama
    lama, lalu di-cache;
  - repo mendapat kode C++ pertamanya;
  - torrent baru bisa dipakai setelah `odm-libtorrent` terunduh.
- Satu unduhan = satu torrent, berupa satu file atau satu folder:
  - selama diunduh, isinya ada di `<nama>.odmpart` (file atau folder) di
    folder tujuan, lalu diganti nama saat selesai, dengan aturan nama
    ganda yang sama seperti unduhan lain;
  - ODM yang menentukan jalur setiap file, lalu memberikannya ke
    libtorrent;
  - kategori dipilih dari ekstensi file terbesar;
  - file yang tidak dicentang tetapi ikut tertulis karena berbagi piece di
    batasnya dihapus setelah selesai.
- Status menjadi `complete` begitu semua file yang dipilih selesai dan
  terverifikasi. Seeding ditandai terpisah (`Seeding` di `Info`). Jadi
  filter Selesai, notifikasi, dan aksi selesai (buka file, matikan
  komputer) berlaku saat unduhan selesai, bukan saat seeding berhenti.
- Seeding berjalan sampai rasio 1,0 atau 30 menit, mana yang lebih dulu:
  - targetnya bisa diubah di Options, termasuk "tidak seeding" dan
    "sampai dihentikan";
  - tiap unduhan bisa dihentikan lewat "Hentikan seeding";
  - seeding yang belum mencapai target dilanjutkan setelah ODM dibuka
    lagi;
  - seeding tidak dihitung dalam batas unduhan bersamaan.
- Jaringan:
  - `odm-libtorrent` hanya jalan saat ada torrent yang diunduh atau
    di-seed, lalu berhenti. Pengguna yang tidak memakai torrent tidak
    membuka port dan tidak menanggung memorinya;
  - port masuk dipilih acak sekali lalu disimpan, dan dibuka di router
    lewat UPnP/NAT-PMP;
  - Windows Firewall bertanya sekali untuk `odm-libtorrent.exe` saat
    torrent pertama dimulai, dan ODM memberi tahu itu sebelumnya.
    `ODM.exe` sendiri tetap tidak membuka port ke luar, dan ODM tidak
    menambah aturan firewall sendiri. Koneksi masuk dan UPnP bisa
    dimatikan di Options;
  - batas kecepatan untuk semua unduhan dan batas per unduhan berlaku
    juga untuk torrent. Batas unggah diatur terpisah di Options;
  - proxy ODM diteruskan ke libtorrent. Proxy SOCKS5 dipakai untuk semua
    koneksi torrent, termasuk ke peer. Proxy HTTP dan proxy sistem hanya
    dipakai untuk tracker dan web seed; koneksi ke peer berjalan langsung
    dan peer melihat IP pengguna. Ini ditulis di Options dan README;
  - tracker dan web seed tidak boleh dipakai torrent dari internet untuk
    menjangkau jaringan lokal. `ssrf_mitigation` libtorrent dinyalakan,
    dan ODM membuang tracker serta web seed yang menunjuk ke alamat lokal.
    Peer di jaringan lokal tetap boleh, karena itu bagian dari
    protokolnya.
- Cakupan:
  - UI baru, CLI, dan API;
  - UI Fyne tidak ikut, karena kini hanya cadangan. Torrent yang sudah ada
    tetap tampil di daftarnya, tanpa jendela khusus.

Tahap 1: program `odm-libtorrent` dan rilisnya

- [ ] Uji coba dulu: bangun libtorrent 2.1 statis dengan vcpkg di kelima
      target, lalu ukur ukuran program dan lama build-nya. Hasilnya
      dilaporkan sebelum lanjut. **K**
- [ ] Program `odm-libtorrent` (C++, `helper/libtorrent/`). **B** Isinya:
      - perintah dan kejadian dalam baris JSON di stdin/stdout, dengan
        versi protokol. Perintahnya: tambah torrent (metainfo atau
        magnet, jalur file dari ODM, pilihan file), tunggu metadata,
        status (byte dan piece terverifikasi, peer, seed, kecepatan, total
        unggahan), jeda, lanjut, ubah pilihan file, batas per torrent,
        hapus, dan pengaturan sesi (port, UPnP, koneksi masuk, batas
        kecepatan, proxy);
      - data lanjutan libtorrent disimpan di folder data ODM, supaya
        torrent berlanjut tanpa diperiksa ulang setelah ODM dibuka lagi;
      - masa hidupnya terikat ke ODM: berhenti saat stdin tertutup, dan
        di Windows ikut job object ODM.
- [ ] Workflow `odm-libtorrent.yml` untuk kelima target. **S** Isinya:
      - dipicu tag `odm-libtorrent-p<protokol>-<versi>-<n>`;
      - dirilis sebagai pre-release yang bukan terbaru, dengan SHA-256,
        lisensi, dan README di paketnya;
      - vcpkg di-cache;
      - CI biasa membangun versi Linux (dengan cache), supaya test Go di
        Tahap 2 bisa memakainya.

Tahap 2: engine

- [ ] `internal/torrent` dan `tools.Keeper`. **S** Isinya:
      - menjalankan `odm-libtorrent` dan menerjemahkan protokolnya;
      - memeriksa versi protokol program;
      - entri `odm-libtorrent` di `tools.Keeper`: diunduh saat dibutuhkan,
        dengan progres, lalu diperbarui seperti yt-dlp;
      - `odm-libtorrent` dicantumkan di jendela Setup.
- [ ] Jenis unduhan `Torrent` di engine (seperti `Site` dan `Stream`).
      **B** Isinya:
      - masukan file `.torrent` (dari disk atau URL) dan magnet (`btih`
        dan `btmh`);
      - pilihan file;
      - progres dari piece yang sudah terverifikasi, plus jumlah peer dan
        seed;
      - jeda, lanjut, dan lanjut setelah ODM ditutup;
      - antrean, batas kecepatan (semua unduhan dan per unduhan), dan aksi
        selesai.
- [ ] Penamaan dan keamanan. **S** Isinya:
      - nama dari torrent disaring per bagian jalur: `SanitizeName`, `..`,
        jalur absolut, nama terlarang Windows, karakter arah teks;
      - file atau folder `.odmpart`, lalu diganti nama saat selesai;
      - Mark-of-the-Web untuk tiap file (`MarkFile`);
      - metainfo disimpan sebagai `<id>.torrent` di samping state, supaya
        magnet tidak perlu mengambil metadata lagi.
- [ ] `-add` dan API menerima magnet dan URL `.torrent`. `-add` tanpa UI
      selesai saat unduhannya selesai, tanpa seeding. **K**
- [ ] Test tanpa internet: seeder dan ODM di localhost di dalam test,
      dengan `odm-libtorrent` hasil build CI. **S** Kasusnya: satu file,
      banyak file, pilihan file, jeda, restart, piece rusak, dan
      `odm-libtorrent` yang crash. Test dilewati, dengan keterangan, kalau
      programnya tidak ada.

Tahap 3: seeding dan jaringan

- [ ] Seeding dengan target rasio dan waktu, tanda `Seeding`, aksi
      "Hentikan seeding", dan lanjut setelah restart. **S**
- [ ] `odm-libtorrent` hanya jalan saat dibutuhkan. **S** Juga: port
      tetap, UPnP/NAT-PMP, koneksi masuk yang bisa dimatikan, batas
      unggah, dan proxy ODM. Torrent privat (flag `private`) tidak
      diumumkan lewat DHT dan PEX.
- [ ] Tracker dan web seed ke jaringan lokal: `ssrf_mitigation` dan
      penyaringan oleh ODM, seperti di rancangan. **K**

Tahap 4: UI baru

- [ ] Unduhan Baru untuk torrent. **S** Isinya:
      - kalau `odm-libtorrent` belum ada, tawaran untuk mengunduhnya
        dulu;
      - magnet menunggu metadata dulu: "Mengambil info torrent…" dengan
        jumlah peer. Bisa dibatalkan, atau "Tambahkan dan unduh semua";
      - lalu daftar file bercentang dengan ukurannya, plus folder,
        kategori, dan antrean;
      - "Buka file .torrent…" di menu ⋯, dan file bisa diseret ke jendela.
- [ ] Daftar dan Progres. **S** Isinya:
      - lencana Seeding dengan kecepatan unggah;
      - menu baris: Hentikan seeding, Buka folder, Salin tautan magnet;
      - jendela Progres dengan peta piece, peer dan seed, kecepatan unduh
        dan unggah, rasio, dan daftar file.
- [ ] Options → Torrent dan jendela selesai. **S** Isinya:
      - target seeding, port, UPnP, koneksi masuk, batas unggah, batas
        peer, buka URL `.torrent` sebagai torrent, dan catatan soal IP dan
        proxy;
      - jendela Unduhan selesai membuka folder untuk torrent banyak file;
      - terjemahan Indonesia.
- [ ] Jalan masuk lain. **S** Isinya:
      - pemantau clipboard mengenali magnet (sekarang hanya alamat dengan
        host);
      - URL yang ternyata file `.torrent` (`application/x-bittorrent`)
        dibuka sebagai torrent;
      - `ODM.exe file.torrent` dan `ODM.exe magnet:…` membuka Unduhan Baru,
        juga saat ODM sudah jalan. Saat ini instans kedua hanya memanggil
        `/api/show`, jadi alamat atau file-nya perlu diteruskan.

Tahap 5: ekstensi dan asosiasi

- [ ] Ekstensi: "Download with ODM" menerima tautan `magnet:`, dan
      `.torrent` yang ditangkap dibuka sebagai torrent. Add-on Firefox
      ikut ditandatangani ulang di rilis berikutnya. **K**
- [ ] Windows, opsional di Options: ODM sebagai pembuka `magnet:` dan
      `.torrent` (di HKCU, dihapus lagi saat dimatikan). Windows tetap
      menanyakan pilihan aplikasinya ke pengguna; ODM tidak mengubah
      aplikasi bawaan sendiri. **S**

Tahap 6: uji jalan dan rilis

- [ ] Uji di exe dengan torrent dan magnet resmi Ubuntu atau Debian. **S**
      Yang dicek: unduhan pertama `odm-libtorrent`, unduh, jeda, tutup
      ODM, lanjut, seeding sampai target, dialog firewall, proxy SOCKS5,
      dan pemakaian memori. Linux diuji lewat harness WSL.
- [ ] Rilis. **K** `odm-libtorrent` dirilis lebih dulu (push tagnya
      ditanyakan ke pemilik), baru ODM yang memakainya. README (fitur,
      firewall, IP dan proxy, lisensi) dan CHANGELOG ikut diperbarui.

Nanti

- [ ] Prioritas per file, dan unduh berurutan untuk menonton sambil
      mengunduh (keduanya sudah ada di libtorrent). **S**
- [ ] Asosiasi `magnet:` dan `.torrent` di Linux dan macOS (butuh file
      `.desktop` dan `.app`). **S**
- [ ] Unggah file `.torrent` di web UI `-serve`. **K**

Batasan:
- Mengunduh dan mengunggah konten berhak cipta lewat torrent bisa
  melanggar hukum; itu tanggung jawab pemakai.
- Peer dan tracker melihat IP pengguna, kecuali lewat proxy SOCKS5.
- Sebagian ISP memperlambat atau memblokir lalu lintas torrent.

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
