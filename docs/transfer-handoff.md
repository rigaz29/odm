# Catatan penerusan mesin unduhan

Ditulis 2026-10-14. Branch ini `docs/transfer-handoff`, dibuat dari `engine-local-test` pada `35b963ba7bd012dd2ff8e6d4ec15fdd0c69ba146`. Commit dokumen ini tidak mengubah kode Go.

Dokumen ini untuk Ryan Bagas dan untuk AI yang meneruskan pekerjaan di `C:\Users\rigaz\odm`. Isinya perilaku transfer yang diamati, keadaan kode, dan aturan kerja. Tidak ada daftar alamat, tidak ada keluaran decompiler, dan tidak ada salinan kode produk lain.

## Baca ini dulu

- ODM adalah implementasi sendiri. `DISCLAIMER.md` melarang menyalin kode, biner, atau data dari produk lain, dan melarang menyiasati proteksi. Lisensi, trial, dan serial tidak dianalisis dan tidak boleh dianalisis untuk pekerjaan ini.
- Pull request ke `troke12/odm` berbahasa Inggris, satu perilaku per PR, dari `main` upstream. Isi PR: Summary, Behavior, Test plan. Jangan menyebut Internet Download Manager, Gopeed, atau grab di badan PR.
- Jangan menggabungkan sebelas PR yang masih terbuka. Jangan menumpuk branch fitur. Perbaikan yang hanya ada di PR yang sudah terbuka dikerjakan di branch PR itu.
- `engine-local-test` adalah branch uji lokal yang sudah berisi kesebelas perubahan mesin. Branch itu sudah di-push ke fork (`origin`), dan bukan pull request ke upstream.
- Branch dokumen ini duduk di atas `engine-local-test` supaya catatannya cocok dengan kode yang dites. Jangan buka branch ini sebagai pull request ke `troke12/odm`. PR baru tetap dari `main` upstream `5350f79`.
- Artefak analisis di `C:\rea-work\` tetap di mesin lokal. Jangan di-commit. `odm.exe` di root repo adalah hasil build, juga jangan di-commit.
- FTP, pipeline perintah FTP, RTMP, dan pengambilalihan ekor di bawah 65 byte tidak masuk antrean ODM.

## Repositori

| Peran | Lokasi |
| --- | --- |
| Checkout | `C:\Users\rigaz\odm` |
| Fork | `origin` = `https://github.com/rigaz29/odm.git` |
| Upstream | `upstream` = `https://github.com/troke12/odm.git` |
| Identitas git | Ryan Bagas `<ryanbagas27@gmail.com>` |
| `gh` | `C:\Program Files\GitHub CLI\gh.exe`, akun `rigaz29` |
| Modul | `odm`, Go 1.23 |
| `main` upstream dan fork | `5350f79` (`chore(release): v0.2.0`) |

`main` upstream belum memuat commit fitur di bawah. Setiap branch fitur mendokumentasikan dirinya sendiri di `README.md` dan `CHANGELOG.md` bagian `## [Unreleased]`. Menggabungkan dua branch selalu bentrok di kedua berkas itu: pertahankan setiap butir, jangan menggandakan butir yang sama.

## Perintah yang dipakai di mesin ini

Go tidak ada di `PATH`. SDK portabel ada di `C:\Users\rigaz\AppData\Local\go-sdk\go\bin`. Setiap shell PowerShell mulai di `C:\Users\rigaz`. Masuk ke repo dulu. PowerShell 5.1 tidak punya `&&`. Operator `>` menulis UTF-16; badan commit dan PR ditulis lewat `[System.IO.File]::WriteAllText` dengan `UTF8Encoding $false`. `@{u}` di PowerShell adalah hashtable; kutip saat dipakai di git.

```powershell
Set-Location C:\Users\rigaz\odm
$env:GOTOOLCHAIN = "local"
$env:PATH = "C:\Users\rigaz\AppData\Local\go-sdk\go\bin;" + $env:PATH
go test ./... -count=1 -timeout 240s
go vet ./...
```

Jangan memakai `go test -race`. cgo mati secara bawaan, jadi race detector gagal.

Build desktop butuh compiler C karena Fyne memakai cgo:

```powershell
go build -tags fyne -o odm.exe ./cmd/odm
```

Tanpa compiler C, biner headless:

```powershell
go build -o odm.exe ./cmd/odm
```

Build desktop belum pernah dijalankan di mesin ini. Jangan menganggap biner Fyne sudah terbukti jalan. Pemakaian yang sudah diketahui dari kode: `.\odm.exe`, `-hidden`, `-add URL -no-ui`.

Pada `35b963b`, sebelum commit dokumen ini, `go test ./... -count=1` lulus 99 tes di 15 paket dan `go vet ./...` lulus. Commit dokumen tidak mengubah paket Go, jadi hasil itu masih menggambarkan mesinnya.

## Cara analisis dilakukan

Salinan yang dianalisis adalah `C:\rea-work\IDMan.exe`, 6.189.056 byte, SHA-256 `2aab3f864f0df739e2c41642f1607195d18c173014f54ec914c8388d77ad1981`. Berkas aslinya `C:\Program Files (x86)\Internet Download Manager\IDMan.exe`. PE 32-bit native, machine `0x14C`.

Berkas sejenis di folder instalasi: `IDMGrHlp.exe`, `idmBroker.exe`, `IDMIECC.dll`, `IDMNetMon.dll`, `downlWithIDM.dll`, serta driver `idmtdi*.sys` dan `idmwfp*.sys`.

Alat:

- REA 6.4.0, peluncur `C:\Users\rigaz\AppData\Roaming\npm\rea.ps1`. `rea` tidak ada di `PATH`.
- Ghidra `12.1.4_PUBLIC` di `C:\tools\ghidra_12.1.4_PUBLIC`.
- JDK `21.0.12.1` di `C:\Program Files\Java\jdk-21.0.12.1`.
- Lingkungan yang dipakai: `GHIDRA_INSTALL_DIR`, `JAVA_HOME`, `REA_ANALYSIS_PROVIDER=ghidra`, `REA_GHIDRA_STARTUP_TIMEOUT_MS=900000`.
- Headless: `support\analyzeHeadless.bat`. Folder induk proyek harus sudah ada. Jangan memanggilnya lewat `cmd /c "... < nul"`.
- Proyek yang sudah jadi: `C:\rea-work\ghidra-proj\IDMan`. Jalankan berikutnya memakai `-process IDMan.exe -noanalysis` terhadap proyek itu. Jangan mengimpor ulang selama proyeknya masih ada.
- Skrip yang sudah ada: `C:\rea-work\scripts\DumpEngine.java`, `DumpMore.java`, `DumpMore2.java`, `DumpMore3.java`, `DumpCaller.java`. Keluaran mereka: `engine-decomp.txt`, `engine-more.txt`, `engine-more2.txt`, `engine-more3.txt`, `engine-caller.txt`. Snapshot REA: `C:\rea-work\idman-snapshot.json`.

Pada 2026-10-14 Ghidra tidak dijalankan ulang. Proyek dan dump masih ada, dan pertanyaan yang tersisa (jumlah koneksi bawaan, rumus ukuran bagian) tidak muncul sebagai string biasa. Yang diulang adalah pemindaian string ASCII pada salinan `IDMan.exe`, plus pembacaan ulang bagian dump yang menjelaskan penyambungan berkas lama, pengambilalihan ekor, dan restart karena timeout. Angka 29 detik pada loop `select` berasal dari pembacaan sebelumnya dan tidak diukur ulang pada penulisan ini.

## Perilaku transfer yang diamati

### Lapisan jaringan

Orkestrasi transfer ada di `IDMan.exe` dan memakai soket sendiri (`send`, `recv`, `select`). Impor `InternetOpen` tidak ada. `WinHttpOpen`, `WinHttpGetProxyForUrl`, `WinHttpGetIEProxyConfigForCurrentUser`, dan `WinHttpDetectAutoProxyConfigUrl` dipakai untuk proxy IE dan skrip PAC, bukan untuk badan unduhan.

TLS memakai OpenSSL yang dibundel: `TLS_method`, `SSL_connect`, `SSL_read`, `SSL_write`, `SSL_CTX_set_alpn_protos`. Negosiasi ALPN untuk HTTP/2 punya pesan gagal sendiri. Di dekat pengenal `h2` ada token versi `1.60.0`, dan pesan sesi HTTP/2 (magic klien, flooding, preface `PRI * HTTP/2.0`) cocok dengan tumpukan HTTP/2 yang dibundel. Jadi HTTP/1.1 adalah klien soket sendiri, sedangkan HTTP/2 dan TLS berdiri di atas pustaka yang ikut terpasang.

Proxy HTTP memakai `CONNECT host:port HTTP/1.1`, dengan `Proxy-Authorization` Basic atau Digest. SOCKS punya jalur sendiri, termasuk gagal otorisasi dan tipe yang tidak dikenal.

### Bentuk permintaan HTTP

Baris permintaan yang terlihat: `%s %s HTTP/1.1`, plus `Host`, `User-Agent`, `Accept`, dan `Connection: Keep-Alive`. Ada juga `HEAD`.

Empat templat `Range` 64-bit:

- `bytes=0-%I64d`
- `bytes=0-`
- `bytes=%I64d-`
- `bytes=%I64d-%I64d`

`Accept-Encoding` punya tiga bentuk: `identity`, nilai yang disisipkan (`%s`), dan `gzip, deflate`. `identity` adalah bentuk yang membuat badan range tetap byte berkas. Parser jawaban mengenal `Content-Range`, `ETag`, `Content-Length`, `Content-Encoding` (`gzip`, `deflate`, `br` / brotli), `Transfer-Encoding: chunked`, dan `multipart/byteranges`.

Nama `If-Range`, `If-Match`, `If-None-Match`, `If-Modified-Since`, dan `Range` ada di daftar header klien. Pemindaian string tidak membuktikan syarat pengiriman `If-Range` baris demi baris. Pembacaan perilaku sebelumnya mengikat validator (ETag / waktu ubah) ke lanjut unduhan dan ke keputusan saat data baru tidak sama dengan data lama.

Otorisasi origin mencakup Digest (`Authorization: Digest username=...`). ODM saat ini hanya mengisi Basic dari login per host di pengaturan.

### Menyimpan dan menyambung byte

Bagian yang diunduh adalah byte mentah. Tidak ada langkah kompresi untuk menggabungkan bagian. `Accept-Encoding: identity` pada permintaan range menjaga agar yang ditulis adalah byte berkas, bukan muatan gzip.

Penulisan langsung memakai `SetFilePointer`, `SetEndOfFile`, `WriteFile`, `LockFile` / `UnlockFile`, `GetOverlappedResult`, `CreateFileMappingA`, dan `MapViewOfFile`. Pelanggaran urutan bagian bisa mengembalikan ukuran bagian (`Try rollback, set chunksize`). Rumus ukuran bagian itu tidak berhasil dipulihkan.

Ada jalur kedua untuk unduhan lama. Sebelum lanjut, program membaca ulang berkas bagian yang sudah ada. Catatan di registry menyimpan id bagian, posisi awal 64-bit, dan id berikutnya. Berkas bagian dibuka dengan nama dasar ditambah nomor. Bagian-bagian itu disalin mentah menjadi satu berkas. Pesan galat pada jalur ini berbicara tentang membuka berkas lokal, membaca ukuran bagian berikutnya, alokasi memori, serta gagal baca atau tulis saat bagian digabung. Ini jalur kompatibilitas untuk tata letak lama, bukan encoder.

Saat sebuah bagian selesai dan antrean bagian kosong, log menyebut bagian terakhir. Posisi akhir dibandingkan dengan ukuran yang diharapkan.

### Jadwal koneksi

Jumlah koneksi adalah pengaturan pengguna ("Max. connections number"), termasuk batas per server. Tidak ada satu konstanta bawaan yang berhasil dipulihkan dari string maupun dari dump yang ada. Jangan menebak angka itu lalu menyalinnya ke ODM. ODM memakai 8 koneksi bawaan dan menolak angka di atas 32 (`settings.go`).

Saat satu koneksi selesai dan sisa tetangganya lebih pendek dari 65 byte (`0x41`), atau sisa itu sudah ditandai, sisa itu diberikan ke koneksi yang baru selesai. Soket lama dikirimi `QUIT` dan ditandai selesai. Ambang 65 byte ini tidak dibawa ke ODM. ODM memotong ekor hanya bila sisa bagian setidaknya dua kali `MinChunkSize` (64 KiB) dan perkiraan selesai bagian lambat setidaknya dua kali bagian tercepat.

Loop kesiapan memakai `__ftime32_s`. Satu koneksi yang melewati batas waktunya diulang sendiri; koneksi lain tidak ikut diulang. Pembacaan sebelumnya menempatkan batas itu sedikit di atas 29 detik. Pada penulisan ini yang dikonfirmasi ulang adalah pesan restart karena timeout dan bahwa pemeriksaan membandingkan waktu yang sudah lewat dengan batas yang disimpan pada koneksi itu. Angka 29 detik tidak diukur ulang.

Dua pesan ke pengguna membedakan dua reaksi saat byte yang baru tidak sama dengan byte yang sudah tersimpan: lanjut dengan satu koneksi, atau mulai lagi dari byte pertama. ODM mengikuti kedua reaksi itu, dengan ambang tumpang-tindih 64 byte milik ODM sendiri.

### Di luar mesin HTTP ODM

FTP punya `TYPE`, `CWD`, `SIZE`, `REST %I64d`, `RETR`, dan `LIST`. `SIZE` yang tidak cocok bisa memakai ukuran lama. Pipeline perintah terlihat pada pembacaan sebelumnya. Semua itu tetap di luar ODM.

`Change chunk size` adalah paket RTMP tipe 1 (ukuran big-endian), bukan ukuran segmen HTTP. Ada juga jalur RTMPT (`POST /fcs/ident2`, `POST /open/1`, `POST /%s/%s/%ld`) dan paket Flash/Flex (report, bandwidth, shared object, invoke). Pembacaan sebelumnya juga melihat penugasan ulang video setelah lebih dari 10 perbandingan keyframe gagal. Jangan menjadikan angka-angka itu spesifikasi ODM.

Pengambilan URL dan cookie terpisah dari mesin transfer. `IDMNetMon.dll` memakai soket dan named pipe. `downlWithIDM.dll` memakai `InternetGetCookieA`.

## Keadaan ODM sebelum PR mesin

Sudah ada di `5350f79`, sebelum PR di bawah:

- Rencana range, bawaan 8 koneksi, maksimum 32, bagian minimum 64 KiB. Ekor yang lebih kecil dari itu dilebur ke bagian sebelumnya (`NewPlan`).
- `Split` memotong setengah sisa bagian yang belum selesai untuk koneksi yang menganggur.
- Lanjut unduhan dari JSON. Cadangan satu aliran bila server tidak mendukung range.
- Pemeriksaan `Content-Range`.
- Klien bersama di `buildClient`: `MaxIdleConns` 64, `MaxIdleConnsPerHost` 16, `IdleConnTimeout` 30 detik, `ForceAttemptHTTP2` true. Proxy `system`, `none`, `http`, atau `socks5`.
- UI Fyne di belakang `-tags fyne`. `golang.org/x/sys` untuk OS, bukan untuk transfer.
- Tidak ada pustaka grab, Gopeed, atau IDM. Mesinnya `net/http` ditambah kode di `internal/engine`.

## Sebelas pull request

Semua dari `main`, semua masih terbuka, tidak ada yang digabung di GitHub. Jangan menggabungkannya di upstream.

| PR | Branch | HEAD | Isi |
| --- | --- | --- | --- |
| [#1](https://github.com/troke12/odm/pull/1) | `engine-stall-timeout` | `57e7ff5` | Batas 30 detik mencakup tunggu header dan setiap baca badan. Stall mengulang dari `chunk.Next()`. Jeda pengguna bukan stall. |
| [#2](https://github.com/troke12/odm/pull/2) | `engine-if-range` | `86521ce` | Lanjut kirim `If-Range`. Validator beda membuang sebagian berkas dan mulai sekali dari byte 0. `200` yang menutup seluruh berkas disimpan bila validator masih sama. |
| [#3](https://github.com/troke12/odm/pull/3) | `engine-conflict-fallback` | `a831a0d` | Tumpang-tindih 64 byte. Byte beda: simpan awalan yang cocok, lanjut satu koneksi. Beda lagi: sekali dari byte 0. Beda lagi: gagal. |
| [#4](https://github.com/troke12/odm/pull/4) | `engine-http1-ranges` | `0a4f8e6` | Beberapa koneksi range memakai tiruan transport HTTP/1.1. Probe dan satu aliran tetap di klien bersama. |
| [#5](https://github.com/troke12/odm/pull/5) | `engine-range-identity` | `8f2548e` | Range mengisi `Accept-Encoding: identity` hanya bila header itu masih kosong. Header pengguna dipertahankan. Satu aliran tidak dipaksa identity. |
| [#6](https://github.com/troke12/odm/pull/6) | `engine-slow-chunk` | `10d7a5e` | Ekor bagian yang ETAnya minimal 2x bagian tercepat, jendela 200 ms, sisa minimal 2x 64 KiB. Sebelum pace dikenal, sisa terbesar yang dipakai. |
| [#7](https://github.com/troke12/odm/pull/7) | `engine-short-body` | `774597e` | EOF sebelum bagian selesai adalah `errShortBody`. Byte yang sudah tertulis tetap. Ulang dari `Next()` sampai 8 kali. |
| [#8](https://github.com/troke12/odm/pull/8) | `engine-probe-byte` | `5093e62` | Probe `Range: bytes=0-0` plus identity. `206` dengan total di `Content-Range` berarti range didukung. `200` berarti satu aliran. |
| [#9](https://github.com/troke12/odm/pull/9) | `engine-clamp-end` | `cf9e76e` | `ownedSpan` memotong buffer ke `End` bagian saat ini, sebelum tulis dan sebelum `Done` naik. |
| [#10](https://github.com/troke12/odm/pull/10) | `engine-ignored-range` | `45eeab5` | `200` untuk range yang lebih kecil menyimpan awalan berurutan dan lanjut satu koneksi. |
| [#11](https://github.com/troke12/odm/pull/11) | `engine-ramp-connections` | `ffd4737` | Koneksi dibuka satu per satu, setelah setiap koneksi yang sudah terbuka menyimpan byte berkas. |

Perintah PR yang sudah terbukti:

```powershell
gh pr create --repo troke12/odm --base main --head rigaz29:<branch> --body-file <path>
```

Push pernah gagal karena DNS lalu berhasil di ulang. `gh pr edit` bisa mengubah badan PR sebelum commit-nya sampai di remote. Cek SHA remote setelah push.

## Urutan gabungan di `engine-local-test`

`engine-local-test` dilacak `origin/engine-local-test` dan sudah di-push. Bukan pull request. HEAD `35b963b`.

Urutan orang-tua pertama:

1. `5093e62` probe, fast-forward
2. `ee87b10` identity
3. `0a2b162` HTTP/1.1
4. `dc08885` badan terputus
5. `373c2c5` clamp
6. `3791521` stall, termasuk header
7. `3eff102` ekor lambat
8. `f8e6d66` ramp
9. `6e84d81` If-Range
10. `716eabb` tes tidak menghitung probe satu byte sebagai transfer
11. `f086a3d` konflik
12. `35b963b` range yang diabaikan

Tiga commit tes (`716eabb` dan penyesuaian di dalam merge konflik serta range-diabaikan) hanya ada di branch gabungan, tidak di PR GitHub. Jangan "memperbaiki" tes itu kembali ke pola PR yang berdiri sendiri.

### Satu loop `Start`

`Download.Start` mengizinkan sampai 5 pass. Tidak ada loop pekerja kedua. Setelah loop habis tanpa selesai, gagal dengan `errConflictGaveUp`.

`prepare` hanya pada pass pertama dan setelah representasi berubah. Perubahan representasi memulai ulang dari byte 0, mengosongkan `single`, `restarted`, dan `alone`. Perubahan kedua pada `Start` yang sama gagal `errRepresentationChanged` ("file on the server changed").

`connLimit` menjadi 1 bila range tidak didukung, `single`, `alone`, atau batas di bawah 1.

Urutan keputusan setelah pekerja berhenti: jeda, lalu perubahan representasi, lalu range yang diabaikan, lalu konflik, lalu selesai atau gagal. Jeda tetap menyimpan niat: `planAlone` bila range diabaikan, `adoptSingle` bila konflik dan belum `single`.

Konflik dan range-yang-diabaikan tidak memanggil `prepare` lagi. Hanya perubahan representasi yang probe ulang.

### Bendera yang harus tetap terpisah

| Bendera | Arti | JSON |
| --- | --- | --- |
| `single` | Dua koneksi tidak sepakat. Pass berikut dan lanjut unduhan tetap satu koneksi. | `single` |
| `restarted` | Satu koneksi itu juga tidak sepakat, sebagian berkas dibuang. Sekali per `Start`. | tidak disimpan |
| `alone` | Server menjawab range kecil dengan seluruh berkas. | `alone` |
| `sawIgnored` | Pass ini yang melihat jawaban itu. | tidak disimpan |
| `pass` | Naik saat jawaban itu terlihat, supaya baca yang masih memegang rencana lama tidak menulis. | tidak disimpan |
| `ETag`, `LastModified` | Validator representasi. | `etag`, `last_modified` |

`single` tidak dihapus di awal `Start`. State lama tanpa `single` atau `alone` tetap multi-koneksi. `Readdress` menyalin validator probe yang baru.

Hanya ada satu field `transferCancel`. Jangan mendeklarasikannya lagi saat menggabungkan branch.

### Jawaban 206, 200, dan tumpang-tindih

`boundaryOverlap` = 64. Hanya pada HTTP 206. Permintaan mundur 64 byte dari `chunk.Next()`, dibandingkan dengan byte yang sudah ada di `reqStart`. Badan 200 tidak memakai 64 byte itu.

`206` yang `Content-Range`-nya tidak mulai di `reqStart` adalah galat mismatch.

`200` dengan `Content-Length` yang diketahui, lebih dari 0, dan bukan ukuran berkas, tetap galat ("server ignored Range").

`200` yang rentangnya sudah seluruh berkas ditulis dari byte 0 hanya bila `wholeFileOK`: awal 0, akhir `total-1`, `total > 0`, dan ETag (atau `Last-Modified` bila tidak ada ETag) masih sama, serta `Content-Length` negatif atau sama dengan total.

Validator di jawaban ada dan berbeda dari yang disimpan: `errRepresentationChanged`, termasuk untuk 200 seluruh berkas. Validator yang tidak ada di jawaban bukan perbedaan.

`200` untuk rentang yang lebih kecil, validator tidak beda:

- Belum `alone`: kembalikan `errIgnoredRange` tanpa menulis badan itu pada offset range. `noteIgnoredRange` menyetel `alone`, `sawIgnored`, menaikkan `pass`, dan membatalkan pass. Setelah pekerja berhenti, `planAlone` menyimpan awalan berurutan (`contiguousDone`; lubang menghentikan awalan) sebagai satu bagian.
- Sudah `alone`: `discardPrefix` membuang byte awal yang sudah tersimpan, lalu sisanya ditulis pada offset itu. `discardPrefix` memakai `body.Read`, belum `readWithIdle`. Menunggu awalan yang besar belum kena batas stall.

Bila awalan itu sudah seluruh berkas, `finish` dipanggil.

`trySplit` mengembalikan nil saat `alone`. Mode `single` karena konflik masih boleh dipecah sampai pass berikut memaksa satu koneksi.

### Byte yang dihitung

Hanya byte `ownedSpan` setelah tulis, dan setelah `End` dibaca ulang, yang menaikkan `Done`, `notePace`, `limiter.Wait`, dan `noteReady`. Byte lewatan `End` tidak dihitung. `notePace` hanya bila hitungan lebih dari 0.

`d.pass` yang berbeda dari `pass` yang dibaca di awal `fetch` mengembalikan `errIgnoredRange` tanpa menulis.

`fetchWithRetry` langsung kembali pada nil, `errIgnoredRange`, `errRepresentationChanged`, dan `errConflict`. Galat lain mundur 300 ms, digandakan, sampai 8 detik, maksimal `MaxRetries` = 8. `retryBase` nol memakai 300 ms. EOF bersih sebelum bagian selesai adalah `errShortBody`. EOF pada byte terakhir menyelesaikan bagian.

`os.File.WriteAt` aman dipanggil bersamaan. `Limiter.Wait` aman bila limiter nil. `doneLocked` menjumlahkan `Done` tiap bagian, jadi 64 byte tumpang-tindih terhitung dua kali pada progres. State ditulis kira-kira setiap 250 ms lewat `notify`, setelah `WriteAt` dan sebelum fsync wajib.

### Stall dan klien segmen

`exchange(ctx, client, req)` memakai klien dari `segmentClient`, bukan selalu `d.client`. Klien nil panik seperti `client.Do` dulu. Tunggu header dan jeda baca memakai `DefaultStallTimeout` 30 detik lewat `time.AfterFunc` dan `readWithIdle`. `Do` yang berhasil tidak boleh membatalkan context permintaan; pembatalan ada di `Body.Close` (`cancelOnClose`). Context induk yang dibatalkan adalah jeda, bukan stall.

`segmentTransport` menyalin transport, `ForceAttemptHTTP2` false, dan `TLSNextProto` peta kosong yang tidak nil. Ukuran pool idle tidak diubah. Transport yang bukan `*http.Transport` dibiarkan. Probe dan satu aliran tetap di klien bersama, yang boleh HTTP/2.

### Ramp

Satu pekerja dulu. `noteReady` setelah byte berkas pertama pekerja itu tersimpan. Pekerja berikut hanya bila setiap pekerja yang sudah dibuka sudah menyimpan byte, sampai `Connections`. Jawaban yang ditolak tidak menyimpan apa pun, jadi koneksi tambahan tidak dibuka. Penghitung ramp tidak disimpan. `Start` berikut meramp lagi.

Pada `httptest` yang instan, bagian 64 KiB bisa selesai dalam satu baca sebelum goroutine berikut jalan, jadi puncak koneksi yang terbang bisa tetap 1. Tes konflik tidak boleh menuntut puncak minimal 2.

### Probe

`Inspect` mengirim `Range: bytes=0-0` dan `Accept-Encoding: identity`. Probe sendiri tidak kena batas stall. `416` pada berkas kosong gagal. `Content-Length` yang tidak diketahui gagal probe. `Set-Cookie` probe menimpa `Cookie` pemanggil bila cookie probe tidak kosong. Tes yang menjawab setiap permintaan harus mengenali `bytes=0-0` sebagai probe dan tidak menghitungnya sebagai transfer. Tes yang mencocokkan awalan `Range` harus mengizinkan mundur 64 byte.

Helper tes di `probe_test.go` bernama `requestedSpan`. Nama `wholeFileRange` milik `download.go`.

## Ekstensi browser

Sudah ada di `main`. Bukan bagian PR mesin.

`extension/background.js` `handOff` mengirim:

- cookie dari `chrome.cookies.getAll` untuk URL itu, disusun ulang, bukan header `Cookie` permintaan yang sedang lewat
- `referrer`
- `navigator.userAgent`

`chrome.webRequest.onHeadersReceived` hanya membaca header jawaban (`Content-Type`, `Content-Length`, `Content-Range`) untuk mendeteksi media. Ambang media `MIN_MEDIA_BYTES` 512 KiB. Tidak ada `onBeforeSendHeaders` atau `onSendHeaders`.

Izin manifes: `downloads`, `contextMenus`, `storage`, `nativeMessaging`, `cookies`, `webRequest`. Host native `com.ochi.odm`. Id Chromium dikunci `key` manifes. Id Firefox `odm@troke.id`.

`cmd/odm/native.go` `relayDownload` sudah menerima `m.Headers` plus `Referer`, `Cookie`, dan `User-Agent`. Ekstensi tidak mengisi peta `headers`. Di mesin, `applyHeaders` jalan dulu, lalu bila `d.cookies` tidak kosong, header `Cookie` ditimpa.

Akibatnya Authorization, header khusus situs, dan `Cookie` hidup dari permintaan halaman tidak ikut ke unduhan. Yang ikut adalah toples cookie untuk URL itu, perujuk, dan user agent.

## Batas yang diketahui

Belum dikerjakan, dan jangan dimulai kecuali diminta:

- Item 8 antrean: SHA-256 opsional. Beda hash mengunduh sekali lagi dari byte 0. Tidak menggantikan pemeriksaan tumpang-tindih 64 byte.

Sudah terlihat, belum dijadikan PR:

- Pekerja yang gagal keras bisa membuat bagian saudaranya belum selesai. Ulang yang terbatas sudah ada.
- `429` dan `Retry-After` tidak ditangani khusus.
- Probe tanpa `Content-Length` gagal.
- Probe tidak kena batas stall.
- `discardPrefix` tidak kena batas stall.
- Cookie probe bisa mengganti `Cookie` yang dibawa pemanggil.
- State sekitar 250 ms tidak menjamin byte sudah di-fsync.
- Header permintaan hidup dari browser belum diteruskan.

## Cara menambah satu perilaku

1. `git fetch upstream`.
2. Branch baru dari `upstream/main` (`5350f79` sampai upstream berubah). Jangan dari `engine-local-test` dan jangan dari branch dokumen ini.
3. Satu perilaku. Tes di `internal/engine`. `go test ./...` dan `go vet ./...` dengan `GOTOOLCHAIN=local`.
4. Changelog dan README hanya untuk perilaku itu, di bawah `## [Unreleased]`.
5. Push ke `origin`, lalu `gh pr create` ke `troke12/odm` bila diminta. Bahasa Inggris.
6. Untuk uji Windows gabungan, gabungkan branch itu ke `engine-local-test` secara lokal. Konflik `download.go` biasanya kedua sisi menambah helper: simpan keduanya. Loop tulis hanya menghitung byte `ownedSpan`. `exchange` harus tetap menerima klien segmen. Jangan mengembalikan `ch.Done += int64(n)` tanpa clamp.
7. Jangan membuka `engine-local-test` atau branch dokumen ini sebagai PR upstream.

Berkas yang paling sering disentuh: `internal/engine/download.go`, `plan.go`, `probe.go`, `manager.go`, `types.go`, `http.go`, plus `*_test.go`, `README.md`, `CHANGELOG.md`.

## Peta berkas mesin

| Berkas | Peran |
| --- | --- |
| `internal/engine/download.go` | `Start`, `fetch`, stall, overlap, konflik, range diabaikan, ramp, state |
| `internal/engine/plan.go` | `NewPlan`, `Split` |
| `internal/engine/probe.go` | `Inspect`, `ParseContentRange` |
| `internal/engine/manager.go` | `buildClient`, `segmentTransport`, antrian, `Readdress` |
| `internal/engine/types.go` | konstanta, `Chunk`, pace |
| `internal/engine/http.go` | `applyHeaders` |
| `internal/engine/limiter.go` | batas laju |
| `internal/engine/store.go` | simpan state |
| `cmd/odm/native.go` | jembatan ekstensi |
| `extension/background.js` | serah terima unduhan |

Konstanta ODM yang disengaja, jangan diganti agar "sama dengan program lain":

- stall 30 detik
- overlap 64 byte
- bagian minimum 64 KiB
- ekor lambat 2x dan jendela 200 ms
- koneksi bawaan 8, atap 32
- ulang maksimum 8, jeda awal 300 ms, atap 8 detik
- ramp tanpa jeda tambahan
