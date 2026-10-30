# Lechat Backend

REST API backend untuk autentikasi dan profil pengguna, dibuat dengan Go, Gin, GORM, dan PostgreSQL.

## Persyaratan

- Go 1.26 atau lebih baru
- PostgreSQL

## Konfigurasi

Buat file `.env` di root project. Contoh:

```env
APP_PORT=8989

DATABASE_HOST=127.0.0.1
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=your-password
DATABASE_NAME=lechat
SSL_MODE=disable

JWT_SECRET_KEY=replace-with-a-long-random-secret
ISSUER_SECRET_JWT=lechat-backend
EXPIRE_TOKEN=40
EXPIRE_REFRESH_TOKEN=24

CORS_ALLOW_ORIGINS=http://localhost:5173
CORS_ALLOW_METHODS=GET,POST,PUT,DELETE
CORS_ALLOW_HEADERS=Origin,Content-Type,Accept,Authorization
CORS_EXPOSE_HEADERS=Content-Length
CORS_ALLOW_CREDENTIALS=true
CORS_MAX_AGE=12h

OTEL_SERVICE_NAME=lechat-backend
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
DB_SLOW_QUERY_THRESHOLD=200ms
```

`EXPIRE_TOKEN` adalah masa berlaku access token dalam detik, sedangkan `EXPIRE_REFRESH_TOKEN` dalam jam. Atur secret JWT yang unik dan kuat di environment; jangan gunakan default untuk deployment.

## OpenTelemetry

Aplikasi mengirim traces dan metrics menggunakan OTLP over HTTP. Jalankan OpenTelemetry Collector atau backend yang menerima OTLP pada endpoint yang dikonfigurasi. `OTEL_EXPORTER_OTLP_ENDPOINT` default ke `http://localhost:4318`; `OTEL_SERVICE_NAME` default ke `lechat-backend`. Ikuti format OTLP HTTP exporter untuk konfigurasi autentikasi/headers bila diperlukan. Jika Collector tidak tersedia, telemetry tidak akan tersimpan dan exporter dapat mencatat error.

Instrumentasi mencakup trace request Gin dan operasi GORM (query, create, update, delete, row, dan raw), metrik jumlah/durasi/status HTTP, durasi/jumlah/error operasi database, slow query, serta penggunaan dan waktu tunggu connection pool. Atribut metrik menggunakan route, jenis operasi, dan tabel; SQL beserta nilai parameter tidak ditambahkan ke span atau metrik.

`DB_SLOW_QUERY_THRESHOLD` mengatur ambang slow query untuk log GORM dan metrik OpenTelemetry, dengan default `200ms`. Setel sebagai durasi Go, misalnya `500ms` atau `1s`.

## Database

Jalankan migrasi SQL pada database PostgreSQL sebelum memulai aplikasi, dengan urutan:

1. `internal/database/migrations/20261027022509_create_users_table.up.sql`
2. `internal/database/migrations/20261029110000_create_token_revocations_table.up.sql`

Aplikasi belum menjalankan migrasi database secara otomatis.

## Menjalankan aplikasi

Dari root project:

```powershell
go mod download
go run .
```

Server berjalan pada port `APP_PORT` (default `8989`). Endpoint health check tersedia di `GET /health`.

## API

Semua endpoint API menggunakan prefix `/api/v1`.

### Register

`POST /api/v1/registers`

Body:

```json
{
  "username": "jane",
  "email": "jane@example.com",
  "password": "secret1",
  "phone": "08123456789"
}
```

Jika berhasil, respons memuat `token` (access token) dan `refresh_token`.

### Login

`POST /api/v1/sessions`

Body:

```json
{
  "email": "jane@example.com",
  "password": "secret1"
}
```

Respons sukses memuat `token` (access token) dan `refresh_token`.

### Refresh sesi

`POST /api/v1/sessions/refresh`

Gunakan **refresh token** dari register/login pada header `Authorization`. Jangan mengirim access token untuk endpoint ini.

```http
Authorization: Bearer <refresh_token>
```

Jika valid, API mengembalikan pasangan token baru dan mencabut refresh token yang digunakan. Refresh token hanya dapat dipakai sekali.

### Check session

`GET /api/v1/sessions/check-session`

Memeriksa apakah sesi masih aktif. Membutuhkan access token:

```http
Authorization: Bearer <token>
```

### Profile

`GET /api/v1/profile`

Mengambil profil pengguna yang identitasnya berasal dari access token. Membutuhkan header `Authorization: Bearer <token>`.

### Logout

`DELETE /api/v1/sessions`

Mencabut access token yang digunakan untuk request. Membutuhkan header `Authorization: Bearer <token>`. Access token yang sudah dicabut tidak dapat dipakai lagi pada endpoint terproteksi. Refresh token diterbitkan terpisah; logout saat ini tidak mencabut refresh token tersebut.

Endpoint refresh dan endpoint dalam group `authenticated` memvalidasi signature, masa berlaku, tipe token, dan status pencabutan token. API mengembalikan `401 Unauthorized` jika token hilang atau tidak valid.

## Logging

Log aplikasi dan GORM ditulis ke folder `logs/` dalam file harian (`log-YY-MM-DD.log`). GORM mencatat error SQL dan query lambat lebih dari 200 ms. Parameter query disamarkan pada log GORM. Pesan `SLOW SQL` menandakan query melewati ambang waktu tersebut; itu tidak selalu berarti query gagal. Periksa status HTTP dan error GORM untuk mengetahui hasilnya.

## Menjalankan test

Jalankan semua unit test:

```powershell
go test ./...
```

Test endpoint autentikasi berada di `unit_test/autentication_test.go`.