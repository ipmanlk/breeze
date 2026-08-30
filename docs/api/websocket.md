# WebSocket Architecture

## Overview

Real-time bidirectional communication for authenticated users. Follows the same
layered architecture as the HTTP REST layer:

| Layer     | Location                           | Contains                                                               |
| --------- | ---------------------------------- | ---------------------------------------------------------------------- |
| Domain    | `internal/domain/ws.go`            | `WsMessageType` constants, room key helpers (pure types, no json tags) |
| Port      | `internal/port/service.go`         | `Broadcaster` interface for service-to-client event emission           |
| Transport | `internal/transport/ws/`           | Hub, Client, wire-format types with json tags, protocol helpers        |
| Transport | `internal/transport/handler/ws.go` | HTTP upgrade handler (RequireAuth-protected route)                     |

## Key Design Decisions

### Wire format DTOs live in transport, not domain

WS message types (`pingPayload`, `pongPayload`, `connectedPayload`,
`errorPayload`) with their `json` tags live in
`internal/transport/ws/protocol.go`. This mirrors the HTTP pattern where
request/response DTOs live in `internal/transport/dto/`. The domain package has
only pure constants (`WsMessageType`) and room key helpers, no json tags, no
transport concerns.

### Broadcaster interface separates services from wire format

```go
// internal/port/service.go
type Broadcaster interface {
    Broadcast(roomKey string, eventType string, payload any) error
}
```

The Hub implementation marshals the payload internally via `MarshalEvent`.
Services pass domain types and never touch json serialization. This matches the
HTTP pattern where handlers (not services) own DTO conversion.

### Hub core is channel-based (single event-loop goroutine)

- `Hub.Run()` is a single goroutine reading from channels
- Register, Unregister, Subscribe, Unsubscribe, Broadcast, DisconnectUser all go
  through channels
- Room membership and the client registry are owned by the Run goroutine; no
  locks around that shared state
- `removeClientFromAllRooms` is called only from within `Run()`
- One exception: `ConnectionTracker` (per-user connection counts used for
  multi-tab presence) uses a `sync.Mutex` because it is updated outside the
  event loop; shutdown is guarded by a `sync.Once`

### Client auto-subscribes to org room on connect

```go
client := ws.NewClient(h.hub, conn, userID, orgID, sessionID, h.log)
h.hub.Register(client)
h.hub.Subscribe(client, domain.RoomKeyOrg(orgID))
```

Every authenticated client is automatically subscribed to `org:{orgID}`.
Services can broadcast to this room for org-wide events.

## Message Protocol

### Envelope format

All messages use a unified envelope pattern:

```json
{
  "type": "pong",
  "payload": { "timestamp": 1712345678000 }
}
```

### Message types

| Direction       | Type        | Payload                       | Purpose                     |
| --------------- | ----------- | ----------------------------- | --------------------------- |
| Client → Server | `ping`      | `{ timestamp: number }`       | Heartbeat / latency check   |
| Server → Client | `pong`      | `{ timestamp: number }`       | Echoes ping timestamp       |
| Server → Client | `connected` | `{ userId, sessionId }`       | Confirms successful upgrade |
| Server → Client | `error`     | `{ code, message, reqType? }` | Error response              |

### Room keys

```
org:{orgID}                       · org-scoped broadcasts
org:{orgID}:user:{userID}         · direct-to-user messages (notifications)
org:{orgID}:conn:{connID}         · direct-to-connection messages (voice signaling)
org:{orgID}:conversation:{convID} · chat conversations
org:{orgID}:project:{projectID}   · project-scoped events (task comments)
```

Clients are auto-subscribed to their org, user, and connection rooms on connect. Chat
clients subscribe/unsubscribe to conversation rooms with
`conversation_subscribe` / `conversation_unsubscribe`; project views
subscribe/unsubscribe to project rooms with `project_subscribe` /
`project_unsubscribe` to receive live `comment_new` / `comment_updated` /
`comment_deleted` events for the open task.

### Task lifecycle events (project room)

`TaskService` broadcasts task mutations to the project room so all clients
viewing the same board/list see changes in real time without polling.

| Server → Client event | Payload              | When emitted                              |
| --------------------- | -------------------- | ----------------------------------------- |
| `task_created`        | `{ task: Task }`      | Task created (incl. duplicate, move-in)   |
| `task_updated`        | `{ task: Task }`      | Task fields changed (status, priority, etc)|
| `task_moved`          | `{ task: Task }`      | Task moved between statuses / reordered   |
| `task_deleted`        | `{ task: Task }`      | Task deleted (incl. move-out from project) |

The frontend `project-detail-page` listens for these events and patches the
local task list in the `projectDetail` signal optimistically; no refetch is
needed for the originating client or any other client in the same project
room. Broadcasts are best-effort: a failure is logged but never blocks the
mutation that triggered it.

## Flow

### Connection lifecycle

```
Client                    Server
  │                         │
  │──── GET /api/ws ──────→│  RequireAuth middleware validates session cookie
  │                         │  Injects user_id, org_id, session_id into context
  │←─── 101 Switching ────│  websocket.Accept()
  │                         │  Client created, registered with Hub
  │                         │  Subscribed to org room
  │←─── connected event ──│  { type: "connected", userId, sessionId }
  │                         │  ReadPump + WritePump goroutines started
  │                         │
  │──── ping ────────────→│  ReadPump receives, dispatches
  │←─── pong ────────────│  WritePump sends response
  │                         │
  │──── disconnect ──────→│  ReadPump returns → Unregister → rooms cleaned
```

### Service event emission (future)

```go
// In any service:
s.broadcaster.Broadcast(
    domain.RoomKeyUser(orgID, userID),
    "notification_new",
    domain.Notification{Title: "...", Body: "..."},
)
```

The Hub marshals the payload, fans out to all clients subscribed to
`org:{orgID}:user:{userID}`.

## Testing Strategy

### Hub tests (behavioral, no network)

`internal/transport/ws/hub_test.go` covers:

- Client subscribed to a room receives broadcast
- Client NOT subscribed does NOT receive (room isolation)
- Multiple clients in same room all receive
- Unsubscribe stops delivery
- DisconnectUser removes all room subscriptions + send chan closed
- Unregister removes from all rooms + send chan closed
- Broadcast to empty room does not panic
- Concurrent register, subscribe, and broadcast
- Client in multiple rooms receives from all subscribed rooms
- Broadcast to a user room delivers only to that user

### Handler integration tests (real WebSocket)

`internal/transport/handler/ws_test.go` (httptest + websocket.Dial) covers:

- Upgrade and read connected event with correct payload
- Unauthenticated request is rejected
- Ping-pong round trip with matching timestamps
- Invalid JSON returns error envelope
- Unknown message type returns error envelope
- Multiple pings maintain order
- Broadcast to org room reaches subscribed client
- Project room subscribe/unsubscribe reaches project-scoped events
- Client disconnect cleans up server state
- Concurrent pings produce unique responses

## Presence System

Presence statuses: `online`, `away`, `offline`, `dnd`.

The WS handler at `internal/transport/handler/ws.go` sets online on connect and offline on disconnect using a **background context** (not the request context, which expires after the HTTP upgrade). Errors during presence writes are logged, not silently discarded.

The `Conversation` domain model has a `PartnerUserID` field hydrated by the conversation service for DM conversations. Frontend looks up DM partner presence via `presence[c.partner_user_id]?.status`, not `presence[c.name]` (which is always `""` for DMs). The `'dnd'` status is allowed by the DB CHECK constraint on `user_presence.status`.

## Frontend

The frontend integration is a signals-based module, no component wrapper, no
framework hooks. Everything lives in `ui/src/store/ws.ts`.

### Connection management

- `connectWs()` / `disconnectWs()` are called once at the app level
  (`app-shell.ts`) when the user is authenticated; the socket persists across
  page navigations.
- State lives in module-level `@preact/signals-core` signals:
  - `wsClient`: the live `WebSocket` instance (or `null`)
  - `connectionStatus`: `"disconnected" | "connecting" | "connected"`
  - `wsUserId`: the user ID from the `connected` envelope
- Login, setup, and unauthenticated pages never connect; `disconnectWs()`
  clears state on logout.

### Listener API

Features subscribe to server events by reading the `wsClient` signal and adding
their own `message` listeners:

```ts
effect(() => {
  const ws = wsClient.value;
  if (!ws) return;
  const onMessage = (ev: MessageEvent) => { /* parse envelope, update signal */ };
  ws.addEventListener("message", onMessage);
  return () => ws.removeEventListener("message", onMessage);
});
```

When the socket reconnects, `wsClient` changes identity, so signal-watched
effects re-run and re-attach listeners to the new socket. Outgoing messages go
through `sendWsMessage(data)` (no-op unless the socket is open).

### Reconnection

Exponential backoff: 1s → 2s → 4s → … capped at 30s (plus random jitter), with
a hard cap of 10 attempts before resetting. Before each attempt the client
probes `GET /api/auth/me`; a 401 means the session is gone, so reconnecting
stops and a `breeze:session-expired` event triggers logout + redirect.

### Architecture

```
        app-shell.ts (calls connectWs() when authenticated)
                      │
             ui/src/store/ws.ts   ← module-level singleton
                      │
    ┌─────────────────┼──────────────────┐
    │                 │                  │
wsClient         connectionStatus     wsUserId
 (signal)          (signal)           (signal)
    │
    └── features add message listeners via signal-watched effects;
        sendWsMessage() writes to the socket
```
