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
- [x] Upstream v0.3.0-rc1 (HLS/DASH) digabung ke `main`.
- [x] File `.odmpart` ditutup saat download berhenti; test ffmpeg DASH di
      Windows (PR #12 dan #13, sudah di-merge upstream).
- [x] Review engine, 9 bug diperbaiki: limiter macet di bawah 64 KB/s, file
      rusak pada server tanpa Range, state download hilang atau hidup lagi,
      crash saat membaca pengaturan, `settings.json` tidak atomik, proxy baru
      tidak berlaku, tanpa timeout koneksi/header, nama file Windows, stall
      pada segmen stream.

## Langkah 0: persiapan

- [ ] **0.1** Merge upstream v0.3.1 ke `main` (perekaman live HLS, popup
      media di extension, playlist publik tidak boleh ke jaringan lokal, nama
      device Windows). **K**
- [ ] **0.2** Pasang compiler C (gcc) supaya UI desktop Fyne bisa di-build
      dan dicek di mesin ini. **K**

## Fase 1: fitur cepat

- [ ] Cari di daftar: Ctrl+F dan kotak filter di toolbar. **K**
- [ ] "Stop all" menghentikan juga antrean yang berjalan (sekarang sama
      dengan "Pause all"). **K**
- [ ] Speed limiter on/off cepat dari menu dan tray. **K**
- [ ] Notifikasi Windows dan suara saat download selesai atau gagal. **K**
- [ ] Batch download dengan pola (`foto[001-120].jpg`, `[a-z]`) dan batch
      dari clipboard. **S**
- [ ] Ingat folder simpan terakhir; folder sementara terpisah. **K**
- [ ] Verifikasi checksum SHA-256/MD5 setelah selesai. **K–S**
- [ ] Jalankan pemindai antivirus setelah selesai (program dan argumen bisa
      diatur). **K**

## Fase 2: fitur besar

- [ ] Antrean sinkronisasi: cek berkala apakah file di server berubah
      (ETag/Last-Modified), lalu unduh ulang. **S–B**
- [ ] Scheduler: ulang tiap N menit/jam; sleep/hibernate setelah selesai. **S**
- [ ] Kategori bisa ditambah dan diedit (ekstensi dan folder). **S**
- [ ] Preview isi ZIP sebelum diunduh (Range ke central directory). **S–B**
- [ ] Drop target melayang untuk link dari browser (cek dukungan Fyne
      dulu). **S**
- [ ] Engine: `429`/`Retry-After`, file tanpa `Content-Length`, Digest
      auth. **S**

## Fase 3: modernisasi dan perapian UI

- [ ] Tema terang dan gelap mengikuti setelan Windows, pilihan warna
      aksen. **S**
- [ ] Kolom bisa disembunyikan/ditampilkan; lebar dan sortir diingat. **S**
- [ ] Status bar: total kecepatan, jumlah aktif, status limiter. **K**
- [ ] Dialog Add URL dengan kartu hasil probe; jendela progress dengan
      grafik kecepatan. **S**
- [ ] Shortcut keyboard: Del, Ctrl+N, Ctrl+F, Space. **K**
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
- Build desktop: `go build -tags fyne -o odm.exe ./cmd/odm` (perlu gcc).
  Build tanpa UI: `go build -o odm.exe ./cmd/odm`.
- Riwayat dan aturan lengkap ada di branch `docs/transfer-handoff`.
