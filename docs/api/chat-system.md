# Chat System Reference

## Overview

Plume's chat is a real-time messaging system with **global channels** (optionally linked to projects) and **direct messages (DMs)**. Each operates within an org boundary. All chat tables carry `org_id` for multi-tenancy.

## Core Domain Types

All in `internal/domain/message.go`:

```
ConversationType = "direct" | "group" | "channel" | "voice" | "category"
NotificationLevel= "all"   | "mentions"    | "nothing"
PresenceStatus   = "online"| "away"        | "offline" | "dnd"
MentionType      = "user"  | "project"     | "task" | "channel" | "everyone"
```

### Key Structs

| Struct                  | Purpose                                                                                                                                         |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `Conversation`          | A channel, DM, or group DM. Has `CategoryID`, `ProjectIDs` (links), `Name`, `Type`, unread/member counts, mute/notif preferences                |
| `ConversationMember`    | User membership in a conversation with `Muted`, `LastReadAt`, joined user info                                                                  |
| `Message`               | A message with optional `ParentID` (thread reply), `ForwardedMessageID`, `Pinned`, attachments, reactions                                       |
| `MessageAttachment`     | File attachment linked to a message                                                                                                             |
| `ReactionGroup`         | Aggregated reaction counts per emoji                                                                                                            |
| `UserPresence`          | Online/away/offline/dnd status per user per org                                                                                                     |
| `UserChannelPreference` | Per-member mute and notification level per conversation                                                                                         |
| `PermissionRule`        | Role + permission + allow mapping for category defaults and channel overrides                                                                   |
| `UserPermissionOverride`| Per-user permission override for a channel (user_id + permission + allow)                                                                       |
| `ChannelAccessEntry`    | User + access source ("explicit" membership or "project" link) for the channel info dialog                                                      |
| `ChannelPermissions`    | Resolved boolean permissions (can_view, can_send, can_manage, can_permissions)                                                                  |

## Channel Architecture

### Global Channels (with optional project links)

- Channels are **global**: they exist at the org level, not per-project
- A channel can be linked to **zero or more projects** via `channel_project_links`
- Project members (owner/admin/member) have implicit access to linked channels; guests and viewers get no implicit access
- Channel project links are inherited from parent category: a channel in a category with project links inherits them
- Route: `/chat` (single chat page, no project-specific chat routes)

### Categories

- Categories group channels (e.g. "Engineering", "General")
- Categories have **default permissions** (`channel_permissions`) that apply to all child channels
- Each category has a kind (`text` or `voice`)
- Categories can be linked to **zero or more projects** via `channel_project_links`
- Child channels inherit project links from their parent category

### Direct Messages

- Type `"direct"` (1-on-1) or `"group"` (3+ members)
- No `Name` (empty string; display name derived from member names)
- DM creation is idempotent: `FindDMByUsers` checks for existing DM first
- Group DM requires at least 3 members (creator + 2 others)

### Permission System

See `internal/service/channel_permission.go` for the full resolution logic.

**Permission resolution priority** (highest to lowest):
0. Per-user override for channel (user_id + permission + allow; explicit allow/deny for a specific user)
1. Channel override for user's specific role
2. Channel override for `everyone` role
3. Per-user override for parent category (inherited)
4. Category default for user's specific role
5. Category default for `everyone` role
6. Per-user override for grandparent category (inherited)
7. Grandparent default for user's specific role
8. Grandparent default for `everyone` role
9. Fallback default by org role (owner/admin = all, member = view+send, viewer = view only, guest = nothing)

**Access check** (`UserHasAccess`):
1. Explicit `conversation_members` membership (resolved permissions apply; override can deny)
2. Per-user override on channel (allow/deny for specific user)
3. Parent category per-user override (inherited)
4. Implicit access via linked projects and parent chain (owner/admin/member only)
5. Guests/viewers get no implicit access; requires explicit membership or override

## Route Registration

All chat routes live under the auth-protected middleware group in
`internal/app/app.go`. Four permission tiers:

| Tier   | Permission            | Routes                                                                                    |
| ------ | --------------------- | ----------------------------------------------------------------------------------------- |
| Read   | `chat:read`           | List conversations, messages, replies, pinned, members, read status, categories, presence  |
| Send   | `chat:send`           | Send/edit/delete messages, pin/unpin, reactions, upload files, set presence               |
| Create | `chat:channel.create` | POST `/api/conversations` (new channel/DM/group)                                          |
| Manage | `chat:channel.manage` | Update/delete conversations, reorder, members CRUD, project links, permissions CRUD |

**Full route map (from `app.go`):**

```
PermChatRead:
  GET   /api/conversations[?scope=&cursor=&limit=]
  GET   /api/conversations/search[?q=&scope=&conversation_id=&sender_id=&has_attachment=&has_link=&is_pinned=&after=&before=&cursor=&limit=]
  GET   /api/conversations/by-parent?parent_id=
  GET   /api/conversations/{id}
  GET   /api/conversations/{id}/members
  GET   /api/conversations/{id}/projects
  GET   /api/conversations/{id}/access
  GET   /api/conversations/{id}/my-permissions
  GET   /api/conversations/{id}/permissions
  GET   /api/conversations/{id}/user-overrides
  POST  /api/conversations/{id}/read
  PATCH /api/conversations/{id}/mute
  PATCH /api/conversations/{id}/notification-level
  GET   /api/conversations/{id}/pinned
  GET   /api/conversations/{id}/messages[?before=&limit=]
  GET   /api/conversations/{id}/messages/{msg_id}/replies
  GET   /api/mentions/search[?q=&types=&limit=]
  GET   /api/chat/presence

PermChatSend:
  POST   /api/conversations/{id}/messages
  PATCH  /api/conversations/{id}/messages/{msg_id}
  DELETE /api/conversations/{id}/messages/{msg_id}
  POST   /api/conversations/{id}/messages/{msg_id}/pin
  DELETE /api/conversations/{id}/messages/{msg_id}/pin
  POST   /api/conversations/{id}/messages/{msg_id}/reactions
  DELETE /api/conversations/{id}/messages/{msg_id}/reactions/{emoji}
  POST   /api/conversations/{id}/attachments
  GET    /api/conversations/{id}/attachments/{att_id}/download
  PUT    /api/chat/presence/me

PermChatChannelCreate:
  POST /api/conversations

PermChatChannelManage:
  PATCH  /api/conversations/{id}
  DELETE /api/conversations/{id}
  PATCH  /api/conversations/{id}/position
  POST   /api/conversations/{id}/members
  DELETE /api/conversations/{id}/members/{user_id}
  PUT    /api/conversations/{id}/projects
  PUT    /api/conversations/{id}/permissions
  PUT    /api/conversations/{id}/user-overrides
```

> **Categories are conversations.** A category is a `conversations` row with
> `type='category'`. There are **no** dedicated `/api/chat/categories*`
> endpoints; category CRUD (create via `POST /api/conversations` with
> `type: "category"`, rename/update via `PATCH /api/conversations/{id}`,
> delete via `DELETE /api/conversations/{id}`) and category permission
> management (`GET/PUT /api/conversations/{id}/permissions`, `user-overrides`,
> `projects`) all reuse the conversation endpoints with the category's
> conversation ID. The `channel_permissions` table applies to both category
> and channel rows; all chat tables live in the consolidated initial schema
> (`00001_initial.sql`).

## Pagination

Cursor-based. Uses `before` (messages) or `cursor` (conversations) query params.

**Messages**: `?before={cursor}&limit=50`, encoded as base64 JSON
`{"c":"2024-01-01 00:00:00","i":"msg-id"}`. Returns descending chronological
order (newest first). Loaded via infinite scroll (IntersectionObserver +
cursor state; see [`pagination.md`](./pagination.md)).

**Conversations**: `?cursor={cursor}&limit=20`, same pattern.

## WebSocket Protocol

`internal/transport/ws/` is a fan-out hub with rooms. Upgrade endpoint at
`GET /api/ws` (requires auth context from middleware).

### Room keys

```
org:{orgID}
org:{orgID}:user:{userID}
org:{orgID}:conversation:{convID}
```

### On connect

- Hub registers client, subscribes to `RoomKeyOrg` and `RoomKeyUser`
- Sends `connected` message with `user_id` and `session_id`

### Client-to-server (incoming)

| Type                       | Effect                                                                 |
| -------------------------- | ---------------------------------------------------------------------- |
| `ping`                     | Server replies `pong`                                                  |
| `typing_start`             | Server broadcasts `typing` to conversation room                        |
| `typing_stop`              | Server broadcasts `typing` (is_typing=false) to conversation room      |
| `conversation_subscribe`   | Client joins conversation room (subscribes for message broadcasts)     |
| `conversation_unsubscribe` | Client leaves conversation room                                        |
| `message_new`              | Not handled as incoming; sent via HTTP POST and broadcast server-side |

### Server-to-client (outgoing broadcast)

| Type                       | Payload                                           | Trigger                   |
| -------------------------- | ------------------------------------------------- | ------------------------- |
| `message_new`              | `{ message, conversation_id }`                    | HTTP POST message         |
| `message_updated`          | `{ message, conversation_id }`                    | HTTP PATCH message        |
| `message_deleted`          | `{ message_id, conversation_id }`                 | HTTP DELETE message       |
| `message_pinned`           | `{ message, conversation_id }`                    | POST pin                  |
| `message_unpinned`         | `{ message_id, conversation_id }`                 | DELETE pin                |
| `message_reaction_added`   | `{ message_id, conversation_id, user_id, emoji }` | POST reaction             |
| `message_reaction_removed` | `{ message_id, conversation_id, user_id, emoji }` | DELETE reaction           |
| `typing`                   | `{ conversation_id, user_id, name, is_typing }`   | typing_start/stop events  |
| `presence_updated`         | `{ user_id, org_id, status }`                     | PUT presence/me           |
| `notification_new`         | `{ id, type, title, body, link, ... }`            | Any notification creation |

### Hub behavior

- Single goroutine event loop (`Hub.Run()`); channels-based fanout
- `Hub.Broadcast(roomKey, eventType, payload)` marshals to JSON and fans out to
  all clients in that room
- On the client, signal-watched effects in Lit components re-fetch or patch
  their signal state when WS events arrive (see `../ui/store.md` and
  `ui/src/store/ws.ts`)

## Notifications & Mentions

Defined in `internal/service/message.go` `sendNotifications()`:

- **DM/Group DM conversations**: Notify ALL non-muted members with
  `notif_chat_dm`
- **Channel conversations**: Only notify on @user mention, @everyone, or thread
  reply
- **Thread replies**: Notify parent author regardless of channel/DM type
- Notification content parsed from TipTap HTML:
  `data-type="user" data-id="{userId}"`
- Notifications respect user `NotificationLevel` (all/mentions/nothing) and
  `Muted` flag
- Sent via `notifSvc.Notify()` which creates a DB record + broadcasts
  `notification_new` to the user's room

### Delivery fan-out

`Notify()` also delivers via two **best-effort** side channels, each gated
on the user's own preference and the server being configured for it. Neither
can block or fail the in-app notification that already landed:

| Channel | Gated on | When it fires |
| ------- | -------- | ------------- |
| Email   | `email_notifications` pref + `SMTP_HOST` set | Per-notification email copy |
| Browser push (Web Push) | `desktop_notifications` pref + VAPID keys set | OS notification via service worker, even when the tab is closed |

See `../ops/configuration.md` for SMTP and VAPID env vars. When neither is
configured Plume is air-gapped-friendly: both are silent no-ops and the
in-app WS notification + bell badge are the only delivery path.

## Message Editing

- Only the sender can edit
- Content must be non-empty
- Edit window controlled by `organization.message_edit_window_minutes`:
  - `0` (default) = unlimited editing
  - `> 0` = edit window in minutes from `message.created_at`
- Soft-delete (set `deleted_at`, content is not removed from DB)

## File Attachments

- Max 50MB per file (enforced in handler)
- Multipart upload via POST, stored in `UPLOAD_DIR` (local filesystem)
- Client uploads first, receives attachment IDs, then sends message with
  `attachment_ids`
- Images render inline (320px thumbnail), other files show as download chips
- Storage backend is injectable via `storage.Storage` interface

## Service Layer Conventions

Services live in `internal/service/` and take `port.*` interfaces (not concrete
store types).

| Service                    | Takes                                                                                                      | Key Methods                                                                                                                                           |
| -------------------------- | ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ConversationService`      | convRepo, userRepo, msgRepo, prefRepo, linkRepo, permRepo, notifSvc, channelPermSvc, broadcaster, logger                       | CreateChannel (handles `channel`/`voice`/`category`), CreateDM, CreateGroupDM, ListMyConversations, ListByParent, GetByID, GetChannelProjectLinks, SetChannelProjectLinks, ListAccess, UpdateConversation |
| `MessageService`           | msgRepo, convRepo, orgRepo, projectRepo, taskRepo, userRepo, attRepo, pendingAttRepo, reactionRepo, prefRepo, notifSvc, channelPermSvc, broadcaster, userPrefRepo, i18n, logger  | SendMessage, EditMessage, DeleteMessage, PinMessage, AddReaction, RemoveReaction, SearchMessages                                                                      |
| `MentionService`           | mentionSearchRepo, convRepo                                                                  | Search (@mention autocomplete across users, channels, projects, tasks)                                                                                |
| `PresenceService`          | presenceRepo, broadcaster                                                                                  | SetStatus (Upsert + broadcast), ListForOrg                                                                                                            |
| `ChannelPermissionService` | permRepo, convRepo, linkRepo, userRepo                                                                     | ResolvePermissions, UserHasAccess, GetPermissions, SetPermissions, GetUserOverrides, SetUserOverrides, GetUsersWithProjectAccess, ListAccess           |

> **No `ChannelCategoryService` exists.** Categories are `conversations` rows
> (`type='category'`); they are created/updated/deleted through
> `ConversationService` and their permissions managed through
> `ChannelPermissionService`, both keyed by the category's conversation ID.

## DB Schema

All chat tables live in the consolidated initial schema
(`internal/store/migration/00001_initial.sql`).

### Chat tables

| Table                      | Key Columns                                                                                                                       |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `conversations`            | id, org_id, parent_id, name, topic, type, created_by, position_key (Lexorank)                                                     |
| `conversation_members`     | conversation_id, user_id, org_id, joined_at, last_read_at, muted                                                                  |
| `messages`                 | id, conversation_id, org_id, sender_id, content (HTML), parent_id, forwarded_message_id, pinned, pinned_by, edited_at, deleted_at |
| `message_attachments`      | id, message_id, file_name, file_size, content_type, storage_path                                                                  |
| `message_reactions`        | message_id, user_id, org_id, emoji (PK on message_id+user_id+emoji)                                                               |
| `user_presence`            | user_id, org_id, status (online/away/offline/dnd), last_seen                                                                     |
| `user_channel_preferences` | user_id, conversation_id, org_id, notification_level, muted, last_read_at                                                         |
| `voice_participants`       | id, conversation_id, org_id, user_id, muted, deafened, joined_at (UNIQUE on conversation_id+user_id)                              |

> Categories are `conversations` rows with `type='category'` and `parent_id=NULL`. Channels
> have `parent_id` pointing to a category. Sidebar ordering uses Lexorank `position_key`
> (fractional indexing; same approach as `tasks.position_key`), so reordering is O(1).

> Voice channel architecture is documented in [`voice-channels.md`](./voice-channels.md).

### Permission tables

| Table                          | Key Columns                                                     |
| ------------------------------ | --------------------------------------------------------------- |
| `channel_project_links`        | channel_id, project_id (PK on both)                             |
| `channel_permissions`          | channel_id, role, permission, allow (applies to **both** category-type and channel conversations) |
| `channel_user_overrides`       | channel_id, user_id, permission, allow                          |

`channel_id` in these tables is the conversation ID of either
a channel or a category (the resolution service walks the `parent_id` chain).
