# Voice Channels

Voice channels provide real-time audio communication for project teams.

## Overview

Voice channels are a special type of conversation (`type: "voice"`) that supports:

- **Multi-participant voice chat**: Up to 25 participants per channel by
  default (configurable via `VOICE_MAX_PARTICIPANTS`)
- **Mute/Deafen controls**: Individual audio controls per participant
- **Speaking indicators**: Visual feedback when someone is talking
- **WebRTC-based**: Uses `pion/webrtc` for the media plane

## Architecture

### Components

1. **SFU (Selective Forwarding Unit)**: Routes audio between participants
2. **Voice Service**: Business logic for join/leave/mute/kick operations
3. **WebSocket Protocol**: Signaling for WebRTC setup
4. **Database**: `voice_participants` table tracks who's in which channel

### Data Flow

```
Client                    Server                        Peer
  |                         |                            |
  |-- voice_join --------->|                            |
  |<-- voice_join_result --|                            |
  |   (SDP offer + ICE)     |                            |
  |                         |                            |
  |-- voice_signal(answer)->|                            |
  |                         |                            |
  |<-- voice_signal(offer) |-- CreateSubscriber -------->|
  |   (for each existing)   |                            |
```

## WebSocket Events

### Client → Server

| Event | Payload | Description |
|-------|---------|-------------|
| `voice_join` | `{ conversation_id }` | Join a voice channel |
| `voice_leave` | `{ conversation_id }` | Leave voice channel |
| `voice_signal` | `{ conversation_id, type, sdp?, candidate? }` | WebRTC signaling |
| `voice_mute` | `{ conversation_id, muted }` | Toggle mute |
| `voice_deafen` | `{ conversation_id, deafened }` | Toggle deafen |

### Server → Client

| Event | Payload | Description |
|-------|---------|-------------|
| `voice_join_result` | `{ participants, ice_servers, sdp_offer }` | Join successful |
| `voice_state_update` | `{ conversation_id, participants }` | Participant list changed |
| `voice_speaking` | `{ conversation_id, user_id, speaking }` | Speaking indicator |
| `voice_mute` | `{ conversation_id, user_id, muted }` | Someone muted |
| `voice_deafen` | `{ conversation_id, user_id, deafened }` | Someone deafened |
| `voice_kick` | `{ conversation_id }` | Kicked from channel |

## REST API

### List Participants

```
GET /api/conversations/{id}/voice/participants
```

Returns all participants in a voice channel.

## Configuration

Environment variables (see `../ops/configuration.md` for full reference):

| Variable                 | Default                        | Description                                       |
| ------------------------ | ------------------------------ | ------------------------------------------------- |
| `STUN_URLS`              | `stun:stun.l.google.com:19302` | Comma-separated STUN servers                      |
| `TURN_ENABLED`           | `false`                        | Enable TURN relay                                 |
| `TURN_HOST`              | `localhost`                    | TURN server host                                  |
| `TURN_PORT`              | `3478`                         | TURN server port                                  |
| `TURN_USER`              | (none)                         | Static TURN username (legacy)                     |
| `TURN_PASS`              | (none)                         | Static TURN password (legacy)                     |
| `TURN_SECRET`            | (none)                         | Shared secret for ephemeral REST creds (RFC 5766) |
| `TURN_CREDENTIAL_TTL`   | `12h`                          | Lifetime of ephemeral TURN credentials             |
| `VOICE_MAX_PARTICIPANTS` | `25`                           | Max concurrent participants per voice channel     |

## Permissions

- **Join**: Requires `can_view` permission on the channel
- **Kick**: Requires `can_manage` permission

## Technical Details

### WebRTC Signaling Flow

1. Client sends `voice_join` with conversation ID
2. Server creates publisher PeerConnection and sends SDP offer
3. Client creates answer and sends back via `voice_signal`
4. ICE candidates exchanged via `voice_signal` events
5. For each existing participant, server creates subscriber connections

### Audio Processing

- Codec: Opus (48kHz, 2 channels)
- Packet forwarding: RTP packets forwarded without decoding
- Speaking detection: Based on audio level (RFC 6464)

## Multi-Tab / Multi-Device Support

Voice sessions are **connection-scoped**: each WebSocket connection owns at
most one voice session per channel. The model is:

- **One active session per user per channel.** If a second tab (or device)
  joins a channel the user is already in, the new connection **takes over**:
  the old session's SFU peer connections are torn down, the DB row is
  reassigned to the new connection, and the old tab receives a `voice_kick`
  event (with `reason: "taken_over"`) so it can reset its UI and release its
  microphone.
- **Signaling is routed to the owning connection** (`RoomKeyConnection`),
  not broadcast to every tab for the user. This prevents ICE candidates and
  subscriber offers from being delivered to tabs that don't own the peer
  connection.
- **A tab closing only exits channels it owns.** On WS disconnect,
  `LeaveByConnection` removes only the sessions whose `connection_id` matches
  the disconnecting connection; the user's other tabs keep their sessions.
- **Same-tab rejoin is idempotent.** A duplicate `voice_join` from the same
  connection returns current state without recreating the SFU publisher.

## Participant Cap

Each voice channel enforces a configurable participant cap (default **25**).
This bounds SFU cost:
since the SFU creates bidirectional subscriber peer connections (O(n²) PCs),
the cap prevents runaway resource use.

- The cap is enforced in `Join`; exceeding it returns a `voice channel is full`
  conflict error.
- A returning user (tab takeover) is **always allowed** even when the channel
  is full, since takeover replaces rather than adds a slot.
- The current `n/max` count is shown in the voice channel header.

## TURN Server Support

For clients behind symmetric NAT, a TURN relay is required. Plume supports
the **RFC 5766 TURN REST API** ephemeral credential scheme (the industry
standard used by coturn, LiveKit, mediasoup, and Janus):

- Set `TURN_SECRET` to a shared secret matching your coturn
  `static-auth-secret` (or `use-auth-secret`).
- On each `voice_join`, the server generates a **per-user, time-limited**
  credential: `username = "<unix-expiry>:<userID>"`,
  `password = base64(HMAC-SHA1(secret, username))`.
- The credential is bound to the joining user and expires after
  `TURN_CREDENTIAL_TTL` (default 12h), so a leaked credential has a bounded
  lifetime and cannot be reused by a different account.
- Without a secret, static `TURN_USER`/`TURN_PASS` long-term credentials are
  used (less secure; credentials are shared across all clients).

`TURN_ENABLED` accepts standard boolean forms (`1`/`0`, `true`/`false`,
`True`/`False`) via Go's `strconv.ParseBool`.

### Multiple transports (UDP / TCP / TLS)

By default Plume advertises a single plain UDP TURN URL built from
`TURN_HOST`/`TURN_PORT` (`turn:host:3478`). For clients behind corporate
firewalls that block UDP, or that block non-standard ports, set `TURN_URLS` to an
explicit, comma-separated list of TURN URIs. The same per-user credential
(when `TURN_SECRET` is set) is applied to every URL, so a client can fall back
across transports. RFC 5766 recommends advertising multiple URIs:

```bash
TURN_ENABLED=true
TURN_SECRET=<same-as-coturn-static-auth-secret>
TURN_URLS=turn:turn.example.com:3478,turn:turn.example.com:3478?transport=tcp,turns:turn.example.com:443?transport=tcp
```

When `TURN_URLS` is set it takes precedence over `TURN_HOST`/`TURN_PORT`.
Each coturn instance can serve all three transports from one process
(`listening-port`, `tls-listening-port`, and the `tcp` relay option).

Example coturn config (`turnserver.conf`) serving UDP + TCP + TLS:

```
listening-port=3478
tls-listening-port=5349
realm=plume.local
use-auth-secret
static-auth-secret=<same-as-TURN_SECRET>
# TLS cert for turns: (turns:...:443?transport=tcp)
cert=/etc/turn/cert.pem
pkey=/etc/turn/pkey.pem
```

## Future Enhancements

- [x] TURN server support for symmetric NAT (RFC 5766 REST API)
- [x] Multiple TURN transports (UDP/TCP/TLS via `TURN_URLS`)
- [x] Multi-tab / multi-device voice sessions (connection-scoped)
- [x] Participant cap (configurable)
- [ ] Screen sharing
- [ ] Push-to-talk
- [ ] Voice activity detection (VAD) optimization
- [ ] Recording capabilities
