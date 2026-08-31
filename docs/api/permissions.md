# Permissions System

## Overview

Plume uses a three-axis permission model:

1. **Org role** (on `users` table): the user's base role within the organization
2. **Project membership** (on `project_members` table): per-project role escalation
3. **Channel overrides** (on `channel_permissions` and `channel_user_overrides` tables): per-channel permission rules

### Org Roles

| Role     | Description                                                                                    |
| -------- | ---------------------------------------------------------------------------------------------- |
| `owner`  | Full access to everything. Created on setup. Can delete the org.                               |
| `admin`  | Full access to everything except `org:delete`. Can manage members and settings.                |
| `member` | Default role. Can see and work in all projects. Cannot manage org settings or delete projects. |
| `viewer` | Read-only by default. Only sees projects explicitly added to `project_members`.                |
| `guest`  | External collaborator. No implicit org-wide access. Only sees projects/channels explicitly granted. |

### Project Membership

The `project_members` table stores per-project role assignments. A user can have
different roles in different projects:

```
User A: org = viewer
  project_members: Apple → viewer, Banana → admin

  Apple  → read-only
  Banana → full management
  Cherry → 403 (not a member)
```

## Effective Permission Resolution

For a given user + project, the effective role is resolved by the
`RequireProjectPermission` middleware (in `internal/transport/middleware/permission.go`)
and read by `handler.EnsureProjectAccess` (in `internal/transport/handler/guard.go`):

```
1. If org role in {owner, admin, member} → use that (implicit access)
2. If org role in {viewer, guest}:
   a. Found in project_members → use project_role
   b. Not found → 403 Forbidden
```

### Permission → Action Mapping

| Permission               | Owner | Admin | Member | Viewer | Guest |
| ------------------------ | ----- | ----- | ------ | ------ | ----- |
| `org:manage`             | ✓     | ✓     |        |        |       |
| `org:delete`             | ✓     |       |        |        |       |
| `org:members.manage`     | ✓     | ✓     |        |        |       |
| `org:members.view`       | ✓     | ✓     | ✓      |        |       |
| `org:members.invite`     | ✓     | ✓     | ✓      |        |       |
| `project:create`         | ✓     | ✓     |        |        |       |
| `project:manage`         | ✓     | ✓     |        |        |       |
| `project:delete`         | ✓     | ✓     |        |        |       |
| `project:view`           | ✓     | ✓     | ✓      | ✓*     | ✓*    |
| `project:status.manage`  | ✓     | ✓     |        |        |       |
| `project:cycle.manage`   | ✓     | ✓     |        |        |       |
| `project:members.manage` | ✓     | ✓     |        |        |       |
| `task:create`            | ✓     | ✓     | ✓      |        |       |
| `task:edit`              | ✓     | ✓     | ✓      |        |       |
| `task:delete`            | ✓     | ✓     |        |        |       |
| `task:view`              | ✓     | ✓     | ✓      | ✓*     | ✓*    |
| `attachment:create`      | ✓     | ✓     | ✓      |        |       |
| `attachment:delete`      | ✓     | ✓     |        |        |       |
| `time:create`            | ✓     | ✓     | ✓      |        |       |
| `time:delete`            | ✓     | ✓     |        |        |       |
| `notification:view`      | ✓     | ✓     | ✓      | ✓      | ✓     |
| `chat:read`              | ✓     | ✓     | ✓      | ✓      | ✓*    |
| `chat:send`              | ✓     | ✓     | ✓      |        |       |
| `chat:channel.create`    | ✓     | ✓     | ✓      |        |       |
| `chat:channel.manage`    | ✓     | ✓     |        |        |       |

> \* Must be an explicit project member (in `project_members` table) or channel member/override to access.

## Enforcement Points

Two layers enforce permissions:

### Layer 1: Middleware (`RequirePermission`)

Applied at the route-group level in `app.go`. Checks the user's org role against
the required permission. Fast, no database calls. Example:

```go
r.Group(func(r chi.Router) {
    r.Use(middleware.RequirePermission(domain.PermTaskCreate))
    r.Post("/api/projects/{id}/tasks", taskHandler.Create)
})
```

### Layer 2: Handler Guard (`handler.EnsureProjectAccess`)

Called inside handlers that access project-scoped data. Checks project
membership for viewer/guest users. Example:

```go
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
    projectID := chi.URLParam(r, "id")
	if err := handler.EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
        transport.ErrorJSON(w, r, http.StatusForbidden, "forbidden", err.Error())
        return
    }
    // ...
}
```

This is a no-op for owner/admin/member org roles. Viewers and guests hit the database check.

## Channel-level permissions

Chat channels (and categories) support per-channel rules layered on top of org
roles. Rules are stored in `channel_permissions` (role rules, including a
special `everyone` role) and `channel_user_overrides` (per-user allow/deny),
and apply to both category-type and channel conversations. Four permissions
exist:

| Permission             | Governs                                          |
| ---------------------- | ------------------------------------------------ |
| `channel:view`         | See and read the channel                          |
| `channel:send`         | Post messages in the channel                      |
| `channel:manage`       | Edit/reorder/delete the channel, manage members   |
| `channel:permissions`  | Edit the channel's permission rules and overrides |

These are *not* in the org-role `RolePermissions` map above; they are resolved
per user per channel by `ChannelPermissionService`
(`internal/service/channel_permission.go`). For each of the four permissions,
resolution order (first match wins):

1. **Per-user override** on the channel (`channel_user_overrides`)
2. **Role-specific rule** on the channel for the user's org role
   (`channel_permissions`)
3. **`everyone` rule** on the channel
4. Walk up the **parent category chain**, repeating steps 1–3 at each level
5. **Org-role fallback** when no rule matched anywhere:
   - `owner` / `admin` → all four allowed
   - `member` → view + send
   - `viewer` → view only
   - `guest` → nothing

**Owners and admins bypass channel-level rules entirely.** They hold all four
permissions on every channel in their org regardless of any deny rule or
override (elevated-role immunity), so they can always read, post in, manage,
and re-permission channels they have never joined. Guests get nothing unless
explicit rules or overrides grant it.

Two additional membership rules sit on top of channel permissions:

- **Sending requires explicit membership.** A project-linked non-member may
  *read* a channel (subject to `channel:view`) but cannot post until added as
  a member (`conversation_members`).
- **Attachment download needs only view access**, matching message reads.

Rules are managed via `GET/PUT /api/conversations/{id}/permissions` and
`GET/PUT /api/conversations/{id}/user-overrides`; the resolved set for the
caller is exposed at `GET /api/conversations/{id}/my-permissions`. The WS room
access checker applies the same resolution when subscribing clients to
conversation rooms.

## Adding a New Permission

### Go side

1. Add the constant to `internal/domain/permission.go`:

```go
const (
    PermNewAction Permission = "new:action"
)
```

2. Add it to the `RolePermissions` map for each role that should have it:

```go
var RolePermissions = map[Role][]Permission{
    RoleOwner:  { /* ... existing ... */, PermNewAction },
    RoleAdmin:  { /* ... existing ... */, PermNewAction },
    RoleMember:  { /* ... existing ... */, PermNewAction },
}
```

3. Add the middleware to the route in `internal/app/app.go`:

```go
r.Group(func(r chi.Router) {
    r.Use(middleware.RequirePermission(domain.PermNewAction))
    r.Post("/api/new-action", handler.NewAction)
})
```

## Testing

| Area          | File                                     | What                                                      |
| ------------- | ---------------------------------------- | --------------------------------------------------------- |
| Unit          | `domain/permission_test.go`              | Permission constants, role→permission mapping correctness |
| Handler guard | `handler/guard.go`                       | EffectiveRole with all org-role + project-members combos  |

## Intentionally Open Routes

A small number of routes are registered **without** `RequirePermission`
middleware. Each is intentionally open to every authenticated user because
the resource is personal to the caller (scoped by `userID` from the session)
or org-wide by design, and adding a permission gate would add no value:

| Route                                | Why no permission middleware                              |
| ----------------------------------- | --------------------------------------------------------- |
| `GET /healthz`                      | Unauthenticated liveness probe for load balancers.       |
| `GET /api/version`                  | Unauthenticated build-version endpoint.                  |
| `GET /api/dashboard`               | Every user has their own dashboard; service scopes to caller. |
| `GET /api/tasks` (My Issues)        | Cross-project task list scoped to the caller's assignments. |
| `PATCH /api/dashboard/visibility`   | Toggles the caller's own dashboard widget visibility.    |

Other routes are also **authed-but-ungated by design**. They sit inside the
`RequireAuth` group without a `RequirePermission` gate because the resource is
personal to the caller or self-service for any authenticated member:

- `GET /api/attachments/{id}/download`: the handler verifies the caller's
  access to the attachment's project before streaming bytes.
- `/api/push/*` (vapid-public-key, subscribe/unsubscribe): users manage their
  own push subscriptions.
- Account self-service: `PATCH /api/account`, `POST /api/account/change-password`,
  `GET /api/avatars/{id}`.
- `/api/workspaces*` (list, create, switch): any authenticated account manages
  its own workspace memberships.

The authenticated routes in these lists rely on the outer **`RequireAuth`**
middleware (which validates the session and loads `userID`/`orgID` into the
context) and on the service layer to scope every query to that `userID`. They
are *not* a permission bypass; they are cases where the permission model offers
no finer grain than "is this user authenticated and in this org?". `/healthz` and
`/api/version` are fully public (no session required) and expose no
user/org-scoped data.

If a future route in this table ever needs per-resource access control
(e.g. viewing another user's dashboard), it must be moved into a
permission-gated group.
