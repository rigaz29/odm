# Mockup gaya UI ODM

Tiga arah desain untuk jendela utama ODM, dibuat sebagai mockup setelah UI
Fluent (Fase 7) dihentikan karena terasa kurang bagus (2026-10-16). Isinya
sama di ketiganya: 14 unduhan dengan semua status (mengunduh, merekam live,
dijeda, antre, gagal, selesai), teks bahasa Indonesia, mode gelap dan
terang. Ini hanya tampilan dengan data tiruan, belum terhubung ke engine.

Gaya yang dipilih akan dipasang di kerangka yang sudah teruji di Tahap 0
(Wails v3 + WebView2, engine di proses yang sama, daftar virtual 60 fps,
exe sekitar 15 MB tanpa CGO). Yang diganti hanya komponen dan tema.

| | Gaya | Pustaka | Kesan |
|---|---|---|---|
| **A** | shadcn/ui | Radix + Tailwind 4, Inter, ikon Lucide | Minimalis, tajam, padat, ala Linear/Vercel. Tabel dengan kotak centang dan badge status. |
| **B** | Material Design 3 | MUI 9, Roboto Flex, ikon Material Rounded | Ramah dan membulat, skema warna Google (Gmail/Drive). FAB "Tambah URL", navigasi berbentuk pil, daftar dua baris. |
| **D** | ODM Aurora (desain sendiri) | Tailwind 4, Plus Jakarta Sans, ikon Lucide | Premium dan khas: panel kaca di atas aurora, aksen ungu-sian. Kartu "Sedang berjalan" dengan bilah per koneksi dan grafik kecepatan, daftar ringkas untuk unduhan lainnya. |

Ringkasan (baris: A, B, D; kolom: gelap, terang):
[screenshots/overview.jpg](screenshots/overview.jpg)

Per gaya: `screenshots/a-dark.jpg`, `a-light.jpg`, `b-dark.jpg`,
`b-light.jpg`, `d-dark.jpg`, `d-light.jpg` (1280×760 pada skala 125%).

Catatan:
- Title bar dibuat sendiri supaya menyatu dengan desain. Title bar bawaan
  Windows juga bisa dipakai.
- Pada A, saat dipasang sungguhan, menu dan dialog memakai Radix (dasar
  shadcn/ui). Mockup ini cukup memakai kelas Tailwind yang sama.

## Menjalankan

```
cd mockups
npm install
npm run dev        # lalu buka a.html, b.html, d.html; ?theme=light untuk mode terang
```
