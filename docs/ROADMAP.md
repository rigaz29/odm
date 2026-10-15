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

- [ ] Tema terang dan gelap mengikuti setelan Windows, pilihan warna
      aksen. **S**
- [ ] Kolom bisa disembunyikan/ditampilkan; lebar dan sortir diingat. **S**
- [x] Status bar: total kecepatan, jumlah aktif, status limiter. **K**
      Kiri: jumlah item, hasil pencarian, jumlah yang dipilih. Kanan:
      jumlah aktif dan kecepatan total, jumlah antre, batas kecepatan.
- [ ] Dialog Add URL dengan kartu hasil probe; jendela progress dengan
      grafik kecepatan. **S**
- [x] Shortcut keyboard: Del, Ctrl+N, Ctrl+F, Space. **K** Ditambah
      Shift+Del (hapus beserta file), label shortcut di menu, dan Help →
      Keyboard shortcuts. Sekalian menghapus item "Quit" ganda di menu Tasks.
- [ ] Web UI diseragamkan dengan desktop, responsif, mode terang. **S**
- [ ] Bahasa Indonesia sebagai bahasa kedua. **B**

## Fase 4: perapian teknis

- [ ] Test engine memakai data acak, bukan pola `byte(i)` yang berulang
      tiap 256 byte. **K**
- [ ] Batas stall untuk body manifest dan kunci AES. **K**
- [ ] Map `bypass` di extension dibersihkan dari entri lama. **K**
- [ ] `/api/settings` bisa diubah sebagian (PATCH). **K**
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
