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
```

`EXPIRE_TOKEN` adalah masa berlaku access token dalam detik, sedangkan `EXPIRE_REFRESH_TOKEN` dalam jam. Atur secret JWT yang unik dan kuat di environment; jangan gunakan default untuk deployment.

## Database

Jalankan migrasi SQL pada database PostgreSQL sebelum memulai aplikasi, dengan urutan:

1. `internal/database/migrations/20261027022509_create_users_table.up.sql`
2. `internal/database/migrations/20261029110000_create_token_revocations_table.up.sql`
3. `internal/database/migrations/20261103035647_conversations.up.sql`
4. `internal/database/migrations/20261103042002_conversations_members.up.sql`
5. `internal/database/migrations/20261103060346_messages.up.sql`
6. `internal/database/migrations/20261103061534_messages_receipts.up.sql`
7. `internal/database/migrations/20261103061749_user_devices.up.sql`
8. `internal/database/migrations/20261103070000_chat_hardening.up.sql`
9. `internal/database/migrations/20261103133900_contacts.up.sql`
10. `internal/database/migrations/20261103140000_group_invitations.up.sql`

Migrasi `20261103070000_chat_hardening` menambah constraint pasangan pengguna direct. Setiap percakapan `direct` harus dibuat dengan `direct_user_low` dan `direct_user_high` berisi ID pengguna terurut (nilai rendah terlebih dahulu); keduanya harus menjadi anggota percakapan. Migrasi `20261103133900_contacts` menambah pertemanan dan mensyaratkan kontak berstatus `accepted` untuk chat direct. Percakapan direct lama yang tidak memiliki relasi kontak accepted tidak tampil/dapat digunakan sampai kedua pengguna berteman. Untuk pengiriman ulang yang idempotent, klien perlu mengirim `client_message_id` yang sama pada retry. Lampiran menyimpan `storage_key` dan metadata di database; isi file tetap perlu disimpan di object storage.

Comman membuat dan menjalankan migration:
1. create : `migrate create -ext sql -dir internal/database/migrations [nama migration]`
2. running: `migrate -path internal/database/migrations -database "postgres://devuser:[password]@[ip database]:[port database]/[name-database]?sslmode=disable" [up/down]`
Aplikasi belum menjalankan migrasi database secara otomatis.

### Chat API

Dokumentasi API lengkap (payload, query, respons, error, dan event WebSocket) tersedia di [docs/API.md](./docs/API.md).

Semua endpoint chat dan kontak membutuhkan access token pada header `Authorization: Bearer <token>`.

- `GET /api/v1/contacts` mengambil daftar teman yang sudah menerima permintaan pertemanan.
- `GET /api/v1/contacts/requests` mengambil permintaan masuk dan keluar; field `direction` menunjukkan arahnya.
- `POST /api/v1/contacts/:user_id/requests` mengirim permintaan. Jika kedua pengguna saling mengirim permintaan, hubungan langsung diterima.
- `POST /api/v1/contacts/:user_id/accept` menerima permintaan dari pengguna tersebut.
- `DELETE /api/v1/contacts/:user_id/requests` menolak permintaan masuk dari pengguna tersebut.
- `DELETE /api/v1/contacts/:user_id` menghapus pertemanan yang sudah diterima.

- `POST /api/v1/conversations` membuat percakapan. Body berisi `type` (`direct` atau `group`), `member_ids` (ID pengguna selain pembuat; minimal satu), dan `title` opsional untuk grup. Direct yang sama akan menggunakan percakapan yang sudah ada.
- `GET /api/v1/conversations` mengambil daftar percakapan beserta pesan terakhir dan jumlah pesan belum dibaca.
- `POST /api/v1/conversations/:conversation_id/messages` mengirim pesan teks dengan `client_message_id` UUID yang harus dipakai ulang saat retry dan `body`. Pengiriman media menunggu integrasi upload storage.
- `GET /api/v1/conversations/:conversation_id/messages?limit=50&before_id=<uuid>` mengambil pesan terbaru. Nilai `next_before_id` pada respons dapat dipakai untuk mengambil halaman berikutnya.
- `POST /api/v1/conversations/:conversation_id/delivered` dengan body `{"through_message_id":"<uuid>"}` mencatat pesan sampai cursor sebagai terkirim ke pengguna ini.
- `POST /api/v1/conversations/:conversation_id/read` dengan body `{"through_message_id":"<uuid>"}` menandai pesan sampai cursor sebagai dibaca.
- `PATCH /api/v1/messages/:message_id` dengan body `{"body":"teks baru"}` mengedit pesan milik pengguna.
- `DELETE /api/v1/messages/:message_id` menghapus pesan milik pengguna secara soft-delete.
- `POST /api/v1/conversations/:conversation_id/invitations` dengan body `{"user_id":123}` mengundang teman ke grup; hanya owner/admin grup yang dapat mengundang.
- `GET /api/v1/group-invitations` mengambil undangan grup yang menunggu respons.
- `POST /api/v1/group-invitations/:invitation_id/accept` menerima undangan dan menambahkan pengguna ke grup.
- `DELETE /api/v1/group-invitations/:invitation_id` menolak undangan.
- `DELETE /api/v1/conversations/:conversation_id/members/:member_id` mengeluarkan anggota. Anggota dapat keluar sendiri; admin/owner dapat mengeluarkan anggota sesuai rolenya. Owner belum dapat keluar karena transfer kepemilikan belum tersedia.

Percakapan direct hanya dapat dibuat dan digunakan selama kedua pengguna berteman; percakapan grup tetap dapat melibatkan pengguna yang bukan kontak. Implementasi ini menyediakan alur REST dan broadcast realtime melalui WebSocket; upload object storage belum diimplementasikan.

### WebSocket Realtime

Hubungkan klien ke `ws://<host>/api/v1/ws` (atau `wss://` di deployment TLS). Klien yang dapat mengirim header dapat memakai `Authorization: Bearer <access-token>`. Browser dapat mengirim frame pertama `{"type":"auth","token":"<access-token>"}` setelah koneksi terbuka. Token diperiksa saat koneksi dan secara berkala; koneksi ditutup saat token kedaluwarsa atau dicabut. Origin browser dibatasi oleh `CORS_ALLOW_ORIGINS`.

Frame JSON yang didukung:

- `conversation.join` dengan `conversation_id` untuk bergabung sekaligus mengambil histori terbaru; gunakan `limit` (maksimal 100) dan `before_id` untuk pagination.
- `conversation.history` mengambil halaman histori tanpa bergabung.
- `conversation.leave` meninggalkan room.
- `message.send` dengan `conversation_id`, `client_message_id`, `message_type: "text"`, dan `body`.
- `receipt.delivered` atau `receipt.read` dengan `conversation_id` dan `through_message_id`.
- `message.edit` dengan `message_id` dan `body`; `message.delete` dengan `message_id`.

Server mengirim event `auth.ok`, `conversation.history`, `message.created`, `message.updated`, `message.deleted`, `receipt.delivered`, `receipt.read`, dan `error`. Hanya anggota aktif yang bergabung ke room menerima broadcast. WebSocket dan REST memakai service yang sama untuk validasi akses dan penyimpanan pesan.

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