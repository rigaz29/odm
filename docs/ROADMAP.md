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
- [ ] Verifikasi checksum SHA-256/MD5 setelah selesai. **K–S**
- [ ] Jalankan pemindai antivirus setelah selesai (program dan argumen bisa
      diatur). **K**

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
- [ ] Perbandingan nama antrean konsisten (tidak peka huruf besar). **K**

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
