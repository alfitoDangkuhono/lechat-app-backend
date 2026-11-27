# API Reference

Base path: `/api/v1`. JSON endpoints use `Content-Type: application/json`. Health check tersedia di `GET /health` tanpa autentikasi.

## Authentication and response format

Except for registration and login, HTTP endpoints require an access token:

```http
Authorization: Bearer <access-token>
```

Successful responses use this envelope:

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

Errors use the same envelope with an error message and an HTTP status code:

```json
{
  "code": 400,
  "message": "invalid chat request"
}
```

Protected endpoints return `401` if the token is missing, invalid, expired, or revoked. Chat/contact validation errors generally return `400`; authorization errors return `403`; missing resources return `404`; conflicts return `409`; oversized encrypted payloads return `413`; message rate limiting returns `429`; unexpected server failures return `500`.

## Authentication and profile

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/registers` | No | Register and issue access/refresh tokens |
| `POST` | `/sessions` | No | Login and issue access/refresh tokens |
| `POST` | `/sessions/refresh` | Refresh token | Rotate the token pair; the used refresh token is revoked |
| `DELETE` | `/sessions` | Access token | Revoke the current access token |
| `GET` | `/sessions/check-session` | Access token | Check the current authenticated session |
| `GET` | `/profile` | Access token | Get the current user's profile |
| `POST` | `/profile` | Access token | Update the current user's profile |
| `POST` | `/profile/avatar/upload-url` | Access token | Create a presigned avatar upload URL and save its S3 object key |
| `GET` | `/profile/avatar/download-url` | Access token | Create a short-lived URL for the current user's avatar |

### Register — `POST /registers`

Request:

```json
{
  "username": "jane",
  "email": "jane@example.com",
  "password": "secret1",
  "phone": "08123456789"
}
```

`username`, `email`, `password`, and `phone` are required. The email must be valid; password must contain at least 6 characters and phone at least 10 characters.

### Login — `POST /sessions`

Request:

```json
{
  "email": "jane@example.com",
  "password": "secret1"
}
```

### Refresh — `POST /sessions/refresh`

Send the **refresh token**, not the access token:

```http
Authorization: Bearer <refresh-token>
```

Register, login, and refresh return `data.token` and `data.refresh_token`.

### Profile — `GET /profile`

Send the access token in the `Authorization` header. The profile belongs to the authenticated user and includes `username`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 42,
    "username": "jane",
    "email": "jane@example.com",
    "first_name": "Jane",
    "last_name": "Doe",
    "phone": "08123456789",
    "about": "Hello, I'm Jane.",
    "avatar_url": "https://s3.example/...",
    "avatar_url_expires_in": 900,
    "public_key": "BGx4...=",
    "created_at": "2026-11-03T10:00:00Z",
    "updated_at": "2026-11-03T10:00:00Z"
  }
}
```

`public_key` is `null` until the user uploads one through `PUT /keys/me`.

### Update profile — `POST /profile`

Send the access token and all four required editable fields. The user ID is taken from the token; do not send `id`, `username`, or `email` in the request:

```http
POST /api/v1/profile
Authorization: Bearer <access-token>
Content-Type: application/json
```

```json
{
  "first_name": "Jane",
  "last_name": "Doe",
  "phone": "08123456789",
  "about": "Hello, I'm Jane."
}
```

`first_name`, `last_name`, `phone`, and `about` must all be non-empty strings. On success, the response is `200` with the updated profile in `data` (including read-only `id`, `username`, and `email`). Missing or invalid JSON fields return `400`; missing or invalid access tokens return `401`; an internal update failure returns `500`. The frontend should prevent submitting an unchanged profile: the current backend reports that case as `500`, not as a dedicated validation/conflict response.

### Upload and display a profile avatar

The database column `users.avatar_url` stores the private S3 object key, not a public URL. The column already exists in the users migration. `GET /profile` signs that key and returns a short-lived URL as `avatar_url` (or `null` if there is no avatar); the object key is not returned to the frontend. The URL is valid for 15 minutes, as indicated by `avatar_url_expires_in`.

1. Request an upload URL using the authenticated user's access token:

   ```http
   POST /api/v1/profile/avatar/upload-url
   Authorization: Bearer <access-token>
   Content-Type: application/json
   ```

   ```json
   {
     "content_type": "image/png"
   }
   ```

   Only `image/jpeg`, `image/png`, and `image/webp` are accepted. The API generates a user-scoped object key and stores it in `users.avatar_url`. The response is valid for 15 minutes:

   ```json
   {
     "code": 200,
     "message": "success",
     "data": {
       "upload_url": "https://s3.example/...",
       "method": "PUT",
       "headers": {
         "Content-Type": "image/png"
       },
       "avatar_url": "avatars/42/9e266a6e3b094fb48af6151ec6ee7d2c.png",
       "expires_in": 900
     }
   }
   ```

2. Upload the raw image bytes directly to `upload_url` using the returned `method` and `headers`. Do not send the file as multipart form data to the backend:

   ```http
   PUT <upload_url>
   Content-Type: image/png

   <raw image bytes>
   ```

   The signed headers must be sent exactly as returned. A successful S3 response is typically `200`. The browser must be allowed by the S3 bucket's CORS policy to send `PUT` with `Content-Type`.

   The backend's `CORS_ALLOW_ORIGINS` only controls requests to this API; it does not configure the object-storage server. For RustFS, set the S3 API's allowed browser origins in the RustFS server environment and restart RustFS:

   ```env
   RUSTFS_CORS_ALLOWED_ORIGINS=http://localhost:5173
   ```

   Use the exact frontend origin (scheme, hostname, and port), without a trailing slash. Multiple RustFS origins can be comma-separated. RustFS documents this setting at [CORS Configuration](https://docs.rustfs.com/en/administration/cors). Do not confuse it with `RUSTFS_CONSOLE_CORS_ALLOWED_ORIGINS`, which applies to the RustFS Console.

   For AWS S3 or another S3-compatible service configured with bucket-level CORS, use a rule such as:

   ```json
   {
     "CORSRules": [
       {
         "AllowedOrigins": ["http://localhost:5173"],
         "AllowedMethods": ["GET", "PUT", "HEAD"],
         "AllowedHeaders": ["*"],
         "ExposeHeaders": ["ETag"],
         "MaxAgeSeconds": 3000
       }
     ]
   }
   ```

   Set that rule on the bucket used by `S3_BUCKET`. The browser's preflight `OPTIONS` response must contain `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods` including `PUT`, and `Access-Control-Allow-Headers` permitting `content-type`.

3. Use `data.avatar_url` from `GET /profile` directly as the image `src`. When that URL expires, request a fresh private download URL:

   ```http
   GET /api/v1/profile/avatar/download-url
   Authorization: Bearer <access-token>
   ```

   ```json
   {
     "code": 200,
     "message": "success",
     "data": {
       "download_url": "https://s3.example/...",
       "expires_in": 900
     }
   }
   ```

The profile and download URLs expire after 15 minutes; do not persist them. Refresh an expired URL through the download URL endpoint or by requesting `GET /profile` again. The upload path is saved when the upload URL is issued, so an abandoned or failed S3 upload can leave the database pointing to an object that does not exist until the upload is retried. Unsupported content types return `400`; missing profile/avatar returns `404`; invalid authentication returns `401`.

## Public keys (E2EE)

Direct chat messages are end-to-end encrypted. The backend stores and validates public keys but never receives private keys, plaintext, or session keys.

| Method | Path | Auth | Description |
|---|---|---|---|
| `PUT` | `/keys/me` | Access token | Upload or rotate the caller's public key |
| `GET` | `/users/{user_id}/key` | Access token | Get a user's public key by numeric user ID |

### Upload public key — `PUT /keys/me`

```json
{
  "public_key": "<base64 raw P-256 public key>"
}
```

The value must be standard (not URL-safe) base64 that decodes to exactly 65 bytes, starts with `0x04`, and is a valid P-256 curve point. Invalid keys return `400` with message `invalid public key`. Success is `204 No Content` with no response body. Re-uploading the identical key is a no-op (still `204`); the key timestamp only moves when the key actually changes. `public_key` in `GET /profile` always matches this endpoint's value for the same user.

### Get public key — `GET /users/{user_id}/key`

`{user_id}` is the numeric user ID (same as `user_id`/`sender_id` in chat payloads). Success:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "public_key": "<base64 raw P-256 public key>",
    "updated_at": "2026-11-19T07:00:00Z"
  }
}
```

`updated_at` comes from the dedicated key-rotation timestamp: it changes only when the public key really changes and is not shifted by profile updates, avatar changes, logins, or identical re-uploads. Clients can safely compare `message.created_at < key.updated_at` to show "this message was encrypted for another device".

Returns `400` for a non-numeric ID and `404` when the user does not exist or has not uploaded a key. The backend never creates keys on behalf of clients. The legacy `GET /contacts/{user_id}/keys` endpoint still exists. Note that encryption AAD and fingerprints still use canonical usernames, not user IDs.

## Contacts

All contact endpoints require an access token. Contact requests are sent by the target user's exact phone number. Accept, reject, and remove operations use positive user IDs. Direct conversations are available only while both participants have an accepted contact relationship.

| Method | Path | Description |
|---|---|---|
| `GET` | `/contacts` | List accepted contacts |
| `GET` | `/contacts/requests` | List pending incoming and outgoing requests |
| `POST` | `/contacts/requests` | Send a contact request using `{"phone":"..."}` |
| `POST` | `/contacts/{user_id}/accept` | Accept a request from that user |
| `DELETE` | `/contacts/{user_id}/requests` | Reject an incoming request from that user |
| `DELETE` | `/contacts/{user_id}` | Remove an accepted contact |

The accept, reject, and remove endpoints do not require a JSON body. A successful mutation returns `data` with `user_id` and `status` (`pending`, `accepted`, `rejected`, or `removed`). If both users send requests to each other, the relationship is automatically accepted. Sending a duplicate pending request returns `409`. A phone number that matches no account returns `404`; duplicate phone numbers return `409` to avoid sending a request to the wrong account.

`GET /contacts/requests` includes `direction` (`incoming` or `outgoing`) for each pending item. Accepted contact objects include `accepted_at`.

## Conversations and messages

All endpoints in this section require an access token. UUID path values and message cursors must be valid UUIDs.

| Method | Path | Query | Description |
|---|---|---|---|
| `POST` | `/conversations` | — | Create a direct or group conversation |
| `GET` | `/conversations` | — | List active conversations |
| `GET` | `/conversations/{conversation_id}/members` | — | List active members of a group |
| `GET` | `/conversations/{conversation_id}/messages` | `limit`, `before_id` | Get a message page |
| `POST` | `/conversations/{conversation_id}/messages` | — | Send a message (direct: encrypted payload; group: legacy body) |
| `POST` | `/conversations/{conversation_id}/messages/ack` | — | Acknowledge stored ciphertext for received messages |
| `POST` | `/conversations/{conversation_id}/delivered` | — | Mark messages delivered through a cursor |
| `POST` | `/conversations/{conversation_id}/read` | — | Mark messages read through a cursor |
| `PATCH` | `/messages/{message_id}` | — | Edit the caller's message |
| `DELETE` | `/messages/{message_id}` | — | Soft-delete the caller's message |

### Create conversation

`POST /conversations`

```json
{
  "type": "group",
  "title": "Weekend plans",
  "member_ids": [12, 24]
}
```

- `type` is `direct` or `group`; `member_ids` is required and must contain at least one other user ID.
- The authenticated user is added automatically. Repeated IDs are deduplicated; group `member_ids` must not include the caller.
- A direct conversation must contain exactly one other user and requires an accepted contact relationship. Creating it again reuses the existing direct conversation.
- A group requires at least one other user, and every requested member must be an accepted contact of the caller. Pending requests are not sufficient. `title` is optional; an omitted or empty title defaults to `Grup baru`.
- Success is `201 Created`, with the conversation in `data.conversation` (`id`, `type`, `title`, and `member_count`). `data.id` is also included for compatibility.

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "conversation": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "type": "group",
      "title": "Tim Produk",
      "member_count": 3
    }
  }
}
```

### List conversations

`GET /conversations`

Returns an array of active conversations. Each item includes its members, last message fields, and `unread_message_count`. Direct conversations are omitted if the contact relationship is no longer accepted. When the last message is encrypted, `last_message_body` is `null` and `last_message_encrypted` is `true`; clients should render a neutral preview such as "Pesan terenkripsi" (encrypted direct messages expose no plaintext preview).

### List message history

`GET /conversations/{conversation_id}/messages?limit=50&before_id=<message-uuid>`

- `limit` is optional, defaults to `50`, and must be between `1` and `100`.
- `before_id` is optional. It points to a message in the same conversation; results are older than that message.
- Results are ordered newest first.

Response `data`:

```json
{
  "items": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174001",
      "conversation_id": "123e4567-e89b-12d3-a456-426614174000",
      "sender_id": 7,
      "from_username": "nadia",
      "to_username": "raka",
      "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
      "type": "text",
      "body": "Hello",
      "reply_to_message_id": "123e4567-e89b-12d3-a456-426614174003",
      "reply_to": {
        "id": "123e4567-e89b-12d3-a456-426614174003",
        "sender_id": 19,
        "username": "andi",
        "body": "Let's start at 3?",
        "type": "text",
        "is_deleted": false
      },
      "edited_at": null,
      "deleted_at": null,
      "created_at": "2026-11-03T07:00:00Z",
      "updated_at": "2026-11-03T07:00:00Z",
      "delivered_count": 1,
      "read_count": 0,
      "receipt_status": "read"
    }
  ],
  "next_before_id": "123e4567-e89b-12d3-a456-426614174001"
}
```

Use `next_before_id` as the next request's `before_id`. It is an empty string when no further page is indicated.

Encrypted direct messages replace `body` with the encrypted payload — `encryption_version: 1`, `ciphertext` (standard base64 of ciphertext+GCM tag), and `iv` (standard base64, decodes to 12 bytes) — and omit `body` entirely:

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174009",
  "conversation_id": "123e4567-e89b-12d3-a456-426614174000",
  "sender_id": 7,
  "from_username": "nadia",
  "to_username": "raka",
  "client_message_id": "123e4567-e89b-12d3-a456-42661417400a",
  "type": "text",
  "encryption_version": 1,
  "ciphertext": "<base64 ciphertext+tag>",
  "iv": "<base64 12 bytes>",
  "created_at": "2026-11-19T07:01:00Z"
}
```

`from_username` is resolved from the authenticated sender and `to_username` from the other direct-conversation member (it is empty for group messages). Legacy direct messages without `encryption_version` keep their `body` plaintext and must be rendered as history, not re-encrypted. `reply_to.body` is empty when the replied-to message is encrypted.

### Send message

`POST /conversations/{conversation_id}/messages`

Which payload shape is accepted is decided by the stored `conversation.type`, never by a client claim:

- **Direct** — encrypted payload is mandatory; `body` must not be sent:

  ```json
  {
    "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
    "type": "text",
    "encryption_version": 1,
    "ciphertext": "<base64 ciphertext+tag>",
    "iv": "<base64 12 bytes>",
    "reply_to_message_id": "123e4567-e89b-12d3-a456-426614174003"
  }
  ```

- **Group** — legacy plaintext format; encryption fields are rejected:

  ```json
  {
    "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
    "type": "text",
    "body": "Hello",
    "reply_to_message_id": "123e4567-e89b-12d3-a456-426614174003"
  }
  ```

`client_message_id` is a required UUID idempotency key. Reuse it for retries of the same message — retries must resend the exact same `ciphertext` and `iv`. Reusing the key with different content returns `409`. Although the request schema recognizes `text`, `image`, `video`, and `file`, currently only `text` messages are implemented; other types are rejected. Media upload/storage is not implemented.

Direct-message validation: `encryption_version` must be exactly `1`; `ciphertext` is standard base64 decoding to 16–49152 bytes (16 bytes minimum covers the GCM tag; larger returns `413 payload too large`); `iv` is standard base64 decoding to exactly 12 bytes. Sending a non-empty `body` for a direct conversation returns `400` with `direct messages must be encrypted`; sending encryption fields for a group returns `400` with `group conversations do not accept encrypted messages yet`. Message sending is rate limited to 30 messages per 10 seconds per user (`429 rate limited`).

`reply_to_message_id` is optional (omit it or send `null` for a normal message). If provided, it must identify a message in the same conversation; otherwise the API returns `422 Unprocessable Entity`. The response, conversation history, and `message.created` event include a `reply_to` summary with the original message ID, sender ID, username, body, type, and `is_deleted`. A deleted original message remains available as a tombstone with an empty body and `is_deleted: true`.

Success is `201 Created`, with the created message object in `data`.

### Message storage acknowledgment

`POST /conversations/{conversation_id}/messages/ack`

```json
{
  "ids": ["123e4567-e89b-12d3-a456-426614174009", "123e4567-e89b-12d3-a456-42661417400a"]
}
```

Clients store ciphertext locally (e.g., IndexedDB) and acknowledge after successfully decrypting. `ids` must contain 1–500 UUIDs. The caller must be an active conversation member and may only acknowledge messages they did not send; foreign or unknown IDs are ignored silently. Success is `204 No Content` with no body, and the request is idempotent. Acknowledgment only records ciphertext receipt — it never deletes conversation history and is separate from `delivered`/`read` receipts.

### Delivery and read receipts

Both `POST /conversations/{conversation_id}/delivered` and `POST /conversations/{conversation_id}/read` take:

```json
{
  "through_message_id": "123e4567-e89b-12d3-a456-426614174001"
}
```

The cursor must reference a message in the conversation. The response includes `conversation_id`, `through_message_id`, and `marked_count`.

When read status is updated through WebSocket or REST, the server sends a `receipt.read` event to online senders of non-deleted messages covered through that cursor. The event has the conversation ID and `data.through_message_id`. Message history includes `receipt_status` for outgoing messages: `sent`, `delivered`, or `read`; incoming messages omit this field.

### Edit and delete message

`PATCH /messages/{message_id}`:

```json
{
  "body": "Updated text"
}
```

The body must not be empty. Only the original sender can edit or delete a message. Encrypted direct messages (with `encryption_version`) cannot be edited — the API returns `403`; hide the edit action for them in the UI. `DELETE /messages/{message_id}` has no body and soft-deletes the message; its body is cleared, `deleted_at` is set, and for encrypted messages the stored `ciphertext`/`iv`/`encryption_version` are wiped.

## Group invitations and members

| Method | Path | Description |
|---|---|---|
| `GET` | `/conversations/{conversation_id}/members` | List active members of a group |
| `POST` | `/conversations/{conversation_id}/invitations` | Invite a contact to a group |
| `GET` | `/group-invitations` | List pending invitations for the caller |
| `POST` | `/group-invitations/{invitation_id}/accept` | Accept an invitation |
| `DELETE` | `/group-invitations/{invitation_id}` | Decline an invitation |
| `DELETE` | `/conversations/{conversation_id}/members/{member_id}` | Leave or remove a member |

Invite request:

```json
{
  "user_id": 24
}
```

Only group owners and admins can invite. The invitee must be an accepted contact of the inviter. Only the invitee can accept or decline; accepting adds them to the group. A pending duplicate invitation returns `409`.

Members may leave themselves using their own `member_id`. Owners/admins may remove members subject to role restrictions; an owner cannot leave or be removed because ownership transfer is not available. Successful membership responses include `conversation_id`, `user_id`, and `status`.

### List group members

`GET /conversations/{conversation_id}/members` requires an access token and an active membership in the group. It returns active members only; members who have left are excluded. A direct conversation ID is rejected.

Success is `200 OK`, with member objects in `data`:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "user_id": 24,
      "username": "nadia",
      "first_name": "Nadia",
      "last_name": "Putri",
      "role": "member"
    }
  ]
}
```

The caller receives `403 Forbidden` if they are not an active member of the group.

## WebSocket realtime

Connect to `ws://<host>/api/v1/ws` or `wss://<host>/api/v1/ws` under TLS. Authenticate using either the `Authorization` handshake header with a Bearer access token or, for browser clients, send this as the first frame after connecting:

```json
{"type":"auth","token":"<access-token>"}
```

The server validates the token at connect time and periodically afterward. The browser `Origin`, when present, must match an entry in `CORS_ALLOW_ORIGINS`. WebSocket frames are JSON and limited to 64 KiB.

### Client frames

Join a conversation and fetch the newest history page:

```json
{
  "type": "conversation.join",
  "conversation_id": "123e4567-e89b-12d3-a456-426614174000",
  "limit": 50
}
```

`conversation.history` accepts the same `conversation_id`, optional `limit`, and optional `before_id`, but does not join the room. `limit` defaults to 50 and must be 1–100. `conversation.leave` accepts `conversation_id`.

Send message. For a **direct** conversation, send the encrypted payload and no `body`; for a **group**, use the legacy `body` format (encryption fields are rejected). The server picks the rule from the stored conversation type:

```json
{
  "type": "message.send",
  "conversation_id": "123e4567-e89b-12d3-a456-426614174000",
  "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
  "message_type": "text",
  "encryption_version": 1,
  "ciphertext": "<base64 ciphertext+tag>",
  "iv": "<base64 12 bytes>"
}
```

The same validations as the REST endpoint apply (`encryption_version` = 1, standard-base64 `ciphertext` of 16–49152 decoded bytes, 12-byte `iv`, rate limit 30 messages/10 s, dedup on `client_message_id` requiring identical ciphertext+IV on retry). Oversized frames are rejected by the 64 KiB frame limit.

`receipt.delivered` and `receipt.read` use `conversation_id` and `through_message_id`. Sending messages and receipts requires having joined that conversation. `message.edit` uses `message_id` and `body`; `message.delete` uses `message_id`. Encrypted messages cannot be edited (`forbidden`).

### Server events

Events are JSON objects with `type`, optional `conversation_id`, and optional `data` or `error`.

| Event | Meaning |
|---|---|
| `auth.ok` | Authentication succeeded; `data.user_id` identifies the caller |
| `conversation.history` | History page; `data` includes `items`, `next_before_id`, and `joined` |
| `conversation.left` | Caller left the conversation room |
| `message.created` | A message was persisted and sent to all online WebSocket connections of active conversation members, whether or not each connection joined the room |
| `message.updated` | A message was edited |
| `message.deleted` | A message was soft-deleted |
| `receipt.delivered` | Delivery receipt update |
| `receipt.read` | Read receipt update, sent to online senders of messages covered by the cursor |
| `error` | Request failed; `error` contains a stable error code |

Error frames carry stable codes: `bad_request`, `invalid_reply`, `forbidden`, `not_found`, `conflict`, `encryption_required` (plaintext body sent to a direct conversation), `group_encryption_unsupported` (encryption fields sent to a group), `invalid_public_key`, `payload_too_large`, and `rate_limited`.

`message.created` is sent to every connected socket for active members of the conversation, including sockets not joined to that room. Message edit/delete and `receipt.delivered` events are sent only to joined subscribers who remain active members. Messages sent through REST also emit `message.created`. These are online WebSocket events, not browser/OS push notifications; offline delivery requires a separate push notification service.

`receipt.read` is sent to every online WebSocket connection belonging to the senders of non-deleted messages covered by the reader's cursor. Its `data` contains `through_message_id`; the outer event contains `conversation_id`. This event is delivered independently of whether the sender joined the room.
