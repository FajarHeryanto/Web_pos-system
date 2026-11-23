# POS Toko — Sistem Informasi Penjualan

Aplikasi Point of Sales berbasis web yang menghubungkan frontend HTML/Bootstrap, API Go/Gin, Supabase PostgreSQL, C Engine untuk kalkulasi CSV, dan COBOL Reporter untuk laporan batch.

## Fitur

- Login JWT dengan password bcrypt dan peran `admin` / `kasir`.
- Dashboard: pendapatan, jumlah transaksi/produk, stok menipis, grafik 7 hari, produk terlaris, transaksi terbaru.
- Katalog produk: cari/filter, tambah, ubah dan hapus (admin); indikator ketersediaan stok.
- POS: keranjang banyak produk, kuantitas, diskon persentase, Cash/Debit/Credit, transaksi database atomik, pengurangan stok, riwayat, dan cetak struk.
- Laporan: proses ekspor CSV dan kalkulasi melalui C, lalu laporan kategori/harian/Top 10 melalui COBOL atau generator Go cadangan di Vercel.
- Skema PostgreSQL dibuat otomatis saat API tersambung; `supabase/schema.sql` tersedia untuk referensi/manual setup.

## Struktur proyek

```text
pos-system/
├── api/index.go              # Vercel Serverless Function
├── go/                       # main.go, router, handlers, models, middleware, database, utils
│   └── utils/bin_cache/      # Binary Linux amd64 yang di-embed ke fungsi Go
├── go.mod                    # Modul di root agar builder Vercel menemukannya
├── c/processor.c             # C Engine
├── cobol/reporter.cob        # GnuCOBOL Reporter
├── web/                      # HTML, CSS, JS
├── supabase/schema.sql
├── bin/                       # Binary Linux hasil build dan library runtime opsional
├── data/                      # CSV/laporan lokal
├── Makefile
└── vercel.json
```

Modul Go diletakkan di root (bukan `go/go.mod`) agar Go builder Vercel dapat mengompilasi entry point `api/index.go` beserta paket yang digunakan. Dependensi database PostgreSQL menggunakan `pgx/v5/stdlib` pure-Go; build Go memakai `CGO_ENABLED=0`.

## Kebutuhan

- Go 1.22 atau lebih baru.
- GCC/kompiler C yang kompatibel dengan C99.
- GnuCOBOL 3.x (`cobc`) untuk laporan COBOL.
- Akun Supabase dengan database PostgreSQL.
- `make` (opsional, mempermudah build).

C Engine dan Go dapat dikembangkan terpisah. Untuk menjalankan semua tombol laporan secara lokal, binary `processor` dan `reporter` harus tersedia. Runtime serverless Go tidak otomatis memiliki GnuCOBOL; baca bagian deployment Vercel/hybrid.

## Menyiapkan Supabase

1. Buat proyek di Supabase.
2. Buka **Project Settings → Database → Connection string** (atau tombol **Connect** pada proyek), lalu pilih format **URI**.
3. Untuk Vercel/serverless, pilih **Transaction Pooler** bila tersedia. Connection string umumnya memakai port `6543`. Untuk pengembangan lokal, **Session Pooler** port `5432` juga dapat dipakai. Gunakan host dan nama pengguna persis seperti yang diberikan dashboard proyek; pengguna pooler biasanya memuat project reference.
4. Ganti placeholder password dengan database password. Bila password mengandung karakter khusus seperti `@`, `:`, `/`, `#`, atau `%`, URL-encode password tersebut.
5. Pastikan parameter TLS ada: `sslmode=require`.
6. Salin `.env.example` menjadi `.env`, lalu isi `DATABASE_URL`. Jangan commit `.env`.
7. Buat `JWT_SECRET` acak setidaknya 32 karakter. Contoh pada macOS/Linux/WSL:

   ```bash
   openssl rand -base64 48
   ```

8. Jalankan server Go. Saat koneksi pertama berhasil, aplikasi membuat tabel/index dan mengisi data demo yang belum ada.

Skema ada di `supabase/schema.sql`; biasanya tidak perlu dijalankan manual karena `go/database/db.go` mengelola migrasi idempoten saat startup.

### Akun demo

| Peran | Username | Password |
|---|---|---|
| Admin | `admin` | `admin123` |
| Kasir | `andi` | `kasir123` |
| Kasir | `budi` | `kasir123` |
| Kasir | `citra` | `kasir123` |
| Kasir | `dewi` | `kasir123` |

Ganti password demo dan rahasia JWT sebelum penggunaan produksi. Seed hanya menambah akun/produk yang belum ada; 100 transaksi simulasi dibuat hanya bila tabel transaksi masih kosong. Data demo bertanggal 1–19 November 2026 dan distribusi Cash/Debit/Credit sebesar 50/30/20.

## Menjalankan lokal

### Bash / macOS / Linux / WSL

```bash
cp .env.example .env
# Edit .env dan isi DATABASE_URL serta JWT_SECRET
make build
make run
```

Buka `http://localhost:8080`. `make build` mengompilasi C, COBOL, dan Go. Untuk menjalankan sebagian saja:

```bash
make c
make cobol
make go
```

### Windows

Gunakan terminal dengan `make`, GCC (misalnya MinGW-w64), Go, dan GnuCOBOL yang tersedia di `PATH`, lalu jalankan `make build` dan `make run`. Go menyesuaikan executable utama untuk Windows. Atau kompilasi engine secara langsung:

```text
gcc -std=c99 -O2 -Wall -Wextra -pedantic -o bin\processor.exe c\processor.c -lm
cobc -x -free -O2 -o bin\reporter.exe cobol\reporter.cob
go build -o bin\pos.exe .\go
```

File sementara lokal berada di `data/`. Jika lingkungan tidak memiliki `make`, perintah kompilasi langsung di atas dapat digunakan.

## Pengujian engine C mandiri

```bash
gcc -std=c99 -O2 -Wall -Wextra -pedantic -o bin/processor c/processor.c -lm
bin/processor data/transactions.csv data/processed_data.csv
```

C Engine membaca header:

```text
transaction_id,product_id,product_name,category,quantity,unit_price,discount_percent,payment_method,transaction_date,cashier_name
```

dan menulis:

```text
transaction_id,product_name,category,quantity,subtotal,discount_amount,net_total,payment_method,transaction_date
```

Argumen input/output wajib diberikan agar lokasi file lokal dan cloud dapat ditentukan oleh Go. Exit code: `0` sukses, `1` file input tidak ditemukan, `2` format/tanggal/nilai tidak valid, `3` gagal alokasi memori, `4` gagal menulis output.

Reporter COBOL dapat dijalankan langsung dengan:

```bash
cobc -x -free -O2 -o bin/reporter cobol/reporter.cob
bin/reporter data/processed_data.csv data/report_output.txt
```

Jika argumen tidak diberikan, reporter membaca `INPUT_CSV_PATH` dan `OUTPUT_REPORT_PATH`; fallback-nya `data/processed_data.csv` dan `data/report_output.txt`.

## API

Semua endpoint memakai format respons JSON:

```json
{"status":"success","message":"...","data":{},"errors":[]}
```

Endpoint selain login memerlukan header `Authorization: Bearer <JWT>`.

| Method | Endpoint | Akses | Ringkasan |
|---|---|---|---|
| POST | `/api/auth/login` | Publik | Login, JWT, profil user |
| GET | `/api/auth/me` | Login | Profil dari JWT |
| GET | `/api/products?search=&category=` | Login | Katalog/filter |
| GET | `/api/products/:id` | Login | Detail produk |
| POST | `/api/products` | Admin | Tambah produk; ID otomatis jika kosong |
| PUT | `/api/products/:id` | Admin | Ubah nama/kategori/harga/stok |
| DELETE | `/api/products/:id` | Admin | Hapus bila tidak punya histori transaksi |
| GET | `/api/transactions?limit=&date=&search=` | Login | Riwayat (maks. 500) |
| POST | `/api/transactions` | Login | Satu/multi-item; simpan dan kurangi stok atomik |
| DELETE | `/api/transactions/:id` | Admin | Hapus transaksi dan kembalikan stok atomik |
| POST | `/api/process` | Admin | Ekspor Supabase, jalankan C Engine |
| POST | `/api/reports/generate` | Admin | Buat laporan dengan COBOL atau fallback Go di Vercel |
| GET | `/api/reports/income` | Login | Isi laporan teks tersimpan |
| GET | `/api/dashboard` | Login | KPI, grafik, Top 5, aktivitas terbaru |

Untuk transaksi POS multi-item, body contoh:

```json
{
  "items": [
    {"product_id":"P001","quantity":2,"discount_percent":5},
    {"product_id":"P006","quantity":1,"discount_percent":5}
  ],
  "payment_method":"Cash",
  "transaction_date":"2026-11-19"
}
```

Backend menggunakan row lock saat checkout. Bila stok untuk satu item tidak cukup, semua item transaksi dibatalkan dan stok tidak berubah.

## Direktori kerja / deployment

`utils.GetWorkDir()` memakai `/tmp` saat `VERCEL` terisi, dan `data/` saat lokal. File kerja dibuat ulang saat fungsi dijalankan; `/tmp` serverless bersifat sementara dan tidak boleh dianggap penyimpanan arsip. Data transaksi permanen tetap ada di Supabase.

### Frontend Vercel + API Go serverless

1. Push proyek ke Git dan impor ke Vercel. Root Directory harus direktori proyek ini.
2. Tambahkan environment variables pada **Project Settings → Environment Variables**:
   - `DATABASE_URL`: URI Supabase dengan `sslmode=require`.
   - `JWT_SECRET`: rahasia acak minimal 32 karakter.
   - Opsional `POS_CORS_ORIGINS`: origin frontend yang diizinkan, jika API dipisah host.
3. Deploy. `vercel.json` mengarahkan `/api/*` dan `/health` ke `api/index.go`, sedangkan halaman dan aset dilayani dari `web/`.
4. Buka URL deployment. Vercel mengompilasi fungsi Go; binary Linux amd64 yang sudah di-embed ikut masuk ke fungsi. `make vercel-build` tetap mencoba membangun engine C/COBOL bila compiler tersedia di builder.

Go memakai modul di root dan koneksi `pgx` tanpa CGO. Fungsi serverless tetap tunduk pada batas waktu/deployment Vercel. File `/tmp` tidak persisten antar instance: saat laporan dibaca dari instance lain, API akan mengekspor transaksi ulang, menjalankan C Engine, lalu membuat ulang laporan.

### C / COBOL di Vercel: binary tersedia atau hybrid

GnuCOBOL umumnya tidak terpasang di runtime serverless. C Processor perlu disiapkan agar alur laporan dapat memproses transaksi; bila COBOL Reporter tidak dapat berjalan, generator Go mengambil alih pembuatan laporan.

**A. Binary Linux yang di-embed (pilihan utama untuk Vercel)**

Workflow `.github/workflows/build-linux-bin.yml` membangun engine Linux amd64 saat ada perubahan pada `c/**` atau `cobol/**`; workflow juga dapat dijalankan manual dari tab **Actions**. Selain menguji kedua engine, workflow menyalin binary ke `go/utils/bin_cache/`, menjalankan pemeriksaan Go (`go build`, `go vet`, serta uji format laporan), lalu meng-commit binary, pustaka runtime yang diperlukan, cache embed, dan `go.sum`. Pastikan repository mengizinkan workflow menulis ke repository (`contents: write`).

**Penting untuk paket ini:** berkas di `go/utils/bin_cache/` dalam ZIP awal adalah penanda, bukan executable. Jalankan workflow GitHub Actions sebelum deployment pertama, lalu pastikan commit hasil workflow sudah masuk ke branch yang dipakai Vercel. Alternatifnya, salin executable Linux asli yang valid ke `go/utils/bin_cache/processor-linux-amd64` dan `go/utils/bin_cache/reporter-linux-amd64`, lalu commit keduanya. Jangan menyalin binary Windows atau file teks.

Saat aplikasi berjalan di Vercel Linux amd64, fungsi Go mengambil binary yang di-embed, mengekstraknya secara atomik ke `/tmp` dengan izin eksekusi, lalu menjalankannya. Jika COBOL Reporter atau pustaka `libcob` tidak tersedia saat runtime, laporan dibuat oleh generator Go dengan susunan kolom dan bagian yang sama. Saat hasil sementara tidak ada di instance yang menerima permintaan berikutnya, API membuat ulang CSV dari Supabase dan menjalankan C Engine lagi. Jika GnuCOBOL membutuhkan pustaka dinamis, workflow menyertakan pustaka non-glibc ke `bin/lib/` dan API menambahkan folder itu ke `LD_LIBRARY_PATH`.

Setelah workflow selesai, pastikan commit baru ter-deploy di Vercel (otomatis melalui integrasi Git, atau lakukan redeploy manual). Workflow menjalankan `go mod tidy` untuk menyelaraskan `go.mod` dan menghasilkan `go.sum`; bila mengembangkan lokal, jalankan perintah yang sama dengan Go 1.22+ untuk menyegarkan dependensi.

**B. Mode hybrid**

Jika Anda ingin menjalankan COBOL asli pada host yang dikelola sendiri, tempatkan API Go bersama C/COBOL pada VM/container Linux dengan `DATABASE_URL`, `JWT_SECRET`, GCC, dan GnuCOBOL terpasang. Frontend dapat tetap dideploy di Vercel:

1. Ubah `window.POS_API_BASE` pada `web/js/config.js` menjadi origin API Go, misalnya `https://pos-api.example.com` (tanpa garis miring di akhir).
2. Set `POS_CORS_ORIGINS` pada backend menjadi origin frontend Vercel persis, misalnya `https://nama-proyek.vercel.app`.
3. Deploy frontend dan jalankan `go run ./go` atau binary Go hasil `make go` pada host backend dengan folder `bin/processor` dan `bin/reporter` tersedia.
4. Database tetap Supabase. Backend hybrid menyimpan file kerja di `data/` lokal; atur volume persistent bila ingin mempertahankan laporan, atau ekspor laporan sebelum restart.


Pada mode Vercel, jika binary C Processor belum disertakan, pemrosesan transaksi untuk laporan akan mengembalikan pesan bahwa C Engine belum tersedia. Jika hanya COBOL Reporter yang gagal, generator Go tetap dapat membuat laporan.

## Keamanan dan batas operasional

- Jangan commit `.env`, password Supabase, atau JWT secret.
- Password disimpan sebagai bcrypt hash; JWT berlaku 12 jam dan diverifikasi dengan HS256.
- Perubahan katalog, penghapusan transaksi, serta pemrosesan laporan hanya untuk admin. Kasir dapat mencatat transaksi.
- Checkout memakai transaksi PostgreSQL dan mengunci stok produk agar penjualan bersamaan tidak melewati stok tersedia.
- Data contoh memakai password publik untuk keperluan demonstrasi; ganti/hapus akun demo sebelum pemakaian nyata.
- Riwayat POS menyimpan satu baris per produk yang terjual. Tiap baris memiliki ID transaksi tersendiri, sehingga pembatalan baris mengembalikan kuantitas item terkait.

## Status verifikasi pada paket ini

C Engine sebelumnya telah diuji pada skenario sukses, input rusak, tanggal/metode bayar tidak valid, dan 50.000 record dengan GCC serta sanitizer melalui harness lokal. Lingkungan pembuatan paket ini tidak menyediakan Go compiler, `gofmt`, maupun `cobc`, sehingga `go build`, `go vet`, uji pembandingan fallback Go dengan COBOL, dan build penuh belum dapat dijalankan di sini. Workflow GitHub Actions menjalankan pemeriksaan tersebut setelah binary Linux asli dibuat; tinjau hasil workflow sebelum deploy. Uji login dan koneksi Supabase sebelum penggunaan.
