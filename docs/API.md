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

Protected endpoints return `401` if the token is missing, invalid, expired, or revoked. Chat/contact validation errors generally return `400`; authorization errors return `403`; missing resources return `404`; conflicts return `409`; unexpected server failures return `500`.

## Authentication and profile

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/registers` | No | Register and issue access/refresh tokens |
| `POST` | `/sessions` | No | Login and issue access/refresh tokens |
| `POST` | `/sessions/refresh` | Refresh token | Rotate the token pair; the used refresh token is revoked |
| `DELETE` | `/sessions` | Access token | Revoke the current access token |
| `GET` | `/sessions/check-session` | Access token | Check the current authenticated session |
| `GET` | `/profile` | Access token | Get the current user's profile |

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
    "created_at": "2026-11-03T10:00:00Z",
    "updated_at": "2026-11-03T10:00:00Z"
  }
}
```

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
| `POST` | `/conversations/{conversation_id}/messages` | — | Send a message |
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

Returns an array of active conversations. Each item includes its members, last message fields, and `unread_message_count`. Direct conversations are omitted if the contact relationship is no longer accepted.

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
      "read_count": 0
    }
  ],
  "next_before_id": "123e4567-e89b-12d3-a456-426614174001"
}
```

Use `next_before_id` as the next request's `before_id`. It is an empty string when no further page is indicated.

### Send message

`POST /conversations/{conversation_id}/messages`

```json
{
  "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
  "type": "text",
  "body": "Hello",
  "reply_to_message_id": "123e4567-e89b-12d3-a456-426614174003"
}
```

`client_message_id` is a required UUID idempotency key. Reuse it for retries of the same message. Reusing the key with different content returns `409`. Although the request schema recognizes `text`, `image`, `video`, and `file`, currently only non-empty `text` messages are implemented; other types are rejected. Media upload/storage is not implemented.

`reply_to_message_id` is optional (omit it or send `null` for a normal message). If provided, it must identify a message in the same conversation; otherwise the API returns `422 Unprocessable Entity`. The response, conversation history, and `message.created` event include a `reply_to` summary with the original message ID, sender ID, username, body, type, and `is_deleted`. A deleted original message remains available as a tombstone with an empty body and `is_deleted: true`.

Success is `201 Created`, with the created message object in `data`.

### Delivery and read receipts

Both `POST /conversations/{conversation_id}/delivered` and `POST /conversations/{conversation_id}/read` take:

```json
{
  "through_message_id": "123e4567-e89b-12d3-a456-426614174001"
}
```

The cursor must reference a message in the conversation. The response includes `conversation_id`, `through_message_id`, and `marked_count`.

### Edit and delete message

`PATCH /messages/{message_id}`:

```json
{
  "body": "Updated text"
}
```

The body must not be empty. Only the original sender can edit or delete a message. `DELETE /messages/{message_id}` has no body and soft-deletes the message; its body is cleared and `deleted_at` is set.

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

Send message:

```json
{
  "type": "message.send",
  "conversation_id": "123e4567-e89b-12d3-a456-426614174000",
  "client_message_id": "123e4567-e89b-12d3-a456-426614174002",
  "message_type": "text",
  "body": "Hello",
  "reply_to_message_id": "123e4567-e89b-12d3-a456-426614174003"
}
```

`receipt.delivered` and `receipt.read` use `conversation_id` and `through_message_id`. Sending messages and receipts requires having joined that conversation. `message.edit` uses `message_id` and `body`; `message.delete` uses `message_id`.

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
| `receipt.read` | Read receipt update |
| `error` | Request failed; `error` contains a message |

`message.created` is sent to every connected socket for active members of the conversation, including sockets not joined to that room. Other room broadcasts (message edits/deletes and receipts) are still sent only to joined subscribers who remain active members. Messages sent through REST also emit `message.created`. These are online WebSocket events, not browser/OS push notifications; offline delivery requires a separate push notification service.
