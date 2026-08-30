# Workspaces (Multi-Workspace Support)

> **Status:** Implemented. Multi-workspace support is live. The global
> `accounts` table and per-org memberships are in the schema, the
> `/api/workspaces` routes (list/create/switch) are registered in
> `internal/app/app.go`, and the switcher state lives in
> `ui/src/store/workspaces.ts`. This doc describes the design as implemented.
> Read together with `./architecture.md` (layered architecture, auth flow)
> and `./permissions.md` (role model).

## Goal

A single Breeze instance hosts **many organizations** ("workspaces"). One
account (one email + one password) can belong to several workspaces, with an
independent role per workspace. The sidebar workspace switcher lets a user
list their workspaces, switch the active one, and create a new one.

Before multi-workspace support the app was single-org-per-instance:
`RequireSetup` allowed exactly one `organizations` row; `users.org_id` +
`users.password_hash` were conflated (identity lived on the per-org `users`
row); login did `GetByEmailAnyOrg … LIMIT 1` (ambiguous when one email maps to
multiple users); and the `/auth/me` response carried a single `org`.

## Data model

### New `accounts` table: global **credential** (single source of truth for auth)

```
accounts (
  id            TEXT PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
)
```

`accounts` owns the **credential** (`password_hash`) and the globally-unique
**login key** (`email`). This is the single source of truth for "can this
person sign in" and "what is their password", fixing the current ambiguity
where `GetByEmailAnyOrg … LIMIT 1` picks an arbitrary per-org row.

### `users` becomes a **membership** (account ↔ org + role), keeps display columns

```
users (
  id            TEXT PRIMARY KEY,
  account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  org_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email         TEXT NOT NULL,          -- denormalized display copy of accounts.email
  name          TEXT NOT NULL,          -- denormalized display copy
  avatar_url    TEXT,                   -- denormalized display copy
  role          TEXT NOT NULL DEFAULT 'member',
  is_active     INTEGER NOT NULL DEFAULT 1,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(org_id, account_id),          -- one membership per account per org
  UNIQUE(org_id, email)                -- retained for back-compat display lookups
)
```

**Only `password_hash` is removed** from `users` (via `ALTER TABLE … DROP
COLUMN`, supported by modernc.org/sqlite v1.51). Display columns (`email`,
`name`, `avatar_url`) stay on `users` as denormalized copies so the ~10
existing display/join queries (conversations, messages, members, presence,
search, …) need **no** join changes. They are synced from `accounts` at
membership-creation time (setup, invite-accept, create-workspace). Email/name
are not user-editable today, so there is no drift path in v1.

### `organizations`: allow many; setup no longer globally unique

`organizations.slug` stays unique. The **"at most one org"** guard moves out
of the schema and into the setup flow (a first-run flag), so additional orgs
can be created post-setup.

### `sessions`: track the **active workspace**

`sessions.org_id` becomes the *active* workspace the session is scoped to. It
already exists; the change is semantic: switching workspace = revoke old
session + issue a new one scoped to the target org (simplest and safest;
avoids mutating JWT claims in place).

### Backfill migration (single pass)

The schema changes landed as part of the consolidated initial schema
(`internal/store/migration/00001_initial.sql`):

1. `CREATE TABLE accounts (...)`.
2. Backfill `accounts` from distinct `(email, password_hash)` in `users`:
   `INSERT INTO accounts(id,email,password_hash,created_at,updated_at)
   SELECT lower(hex(randomblob(16))), email, password_hash, MIN(created_at),
   MIN(updated_at) FROM users GROUP BY email`.
3. `ALTER TABLE users ADD COLUMN account_id TEXT`.
4. `UPDATE users SET account_id = (SELECT id FROM accounts WHERE
   accounts.email = users.email)`.
5. `ALTER TABLE users DROP COLUMN password_hash` (modernc sqlite supports it;
   not indexed, not in any FK/index → safe).
6. `CREATE INDEX idx_users_account_id ON users(account_id)`.
7. `CREATE INDEX idx_accounts_email ON accounts(email)`.

Only the auth/user queries change (drop `password_hash` from SELECTs/INSERTs,
add account-scoped lookups). The 10 display/join queries that read
`u.name/u.email/u.avatar_url` are **untouched**.

## Domain layer

- `domain.Account` (new): `ID, Email, PasswordHash, CreatedAt, UpdatedAt`.
- `domain.User` (modified): drop `PasswordHash`; add `AccountID`. Display
  columns (`Email`, `Name`, `AvatarURL`) stay.
- `domain.Workspace` (new): the switcher list item (org + the account's role
  in it, with the `IsOwner` flag). Kept in `domain` so the
  service returns it and the DTO maps it.
- `domain.CtxActiveOrgID` is **not** needed; `CtxOrgID` is reused: it now
  means "the workspace this session is scoped to".

## Port layer

- `port.AccountRepository` (new): `GetByEmail`, `GetByID`, `Create`,
  `UpdatePassword`.
- `port.UserRepository` (modified): drop `PasswordHash` from `User`; add
  `ListByAccount(ctx, accountID) ([]*User, error)` and
  `GetByOrgAndAccount(ctx, orgID, accountID) (*User, error)`. Remove
  `GetByEmailAnyOrg` (replaced by `AccountRepository.GetByEmail`).
- `port.OrganizationService` (extended): `CreateWorkspace(ctx, accountID, name,
  role)` (post-setup org creation by an authenticated user), `ListForAccount`,
  `Switch(ctx, accountID, orgID)` (issues new session).
- `port.AuthService` (extended): `Login` returns the account + its membership
  list so the frontend can pick a default workspace; new
  `SwitchWorkspace(ctx, accountID, orgID) (session, token, error)`.

## Service layer

- `OrganizationService.Create` (setup path) now: create `accounts` row
  (the email is new at setup) → create `organizations` → create `users`
  membership as owner (copying name/email/avatar from the request) →
  post-create hook (`#general` channel). The first-run guard stays
  `orgRepo.Exists()` ("any org yet?") for `/api/setup`.
- `OrganizationService.CreateWorkspace` (new, authenticated): requires the
  caller's account; creates org + a `users` owner-membership for the calling
  account (display columns copied from the caller's current membership);
  seeds `#general`. Gated by being signed in (no role requirement; anyone can
  spin up their own workspace).
- `OrganizationService.ListForAccount` (new): joins
  `users`+`organizations` for an account → the switcher list.
- `OrganizationService.SwitchWorkspace` (new): verify membership (account in
  target org, active) → revoke current session → create a new `sessions` row
  scoped to the target org + that membership's role → return JWT.
- `AuthService.Login`: look up `accounts.email`; verify password against
  `accounts.password_hash`; load memberships via `ListByAccount`; scope the
  session to the most-recent membership and return the full membership list
  so the UI can offer switching. **No** `GetByEmailAnyOrg` ambiguity.

## Transport layer

New handler `WorkspaceHandler` (`internal/transport/handler/workspace.go`)
with swagger annotations:

| Method | Path | Auth | Role | Description |
| --- | --- | --- | --- | --- |
| GET | `/api/workspaces` | yes | any | List the caller's workspaces (membership+org+role) |
| POST | `/api/workspaces` | yes | any | Create a new workspace (name) → owner membership |
| POST | `/api/workspaces/{id}/switch` | yes | member of `{id}` | Switch active workspace → new session cookie |

`/api/setup` stays first-run only. `/api/auth/me` response gains
`workspaces: [...]` and `active_org_id`. Login response also gains the same.

Wiring (`app.go`): add `accountRepo`, extend `orgService`, add
`WorkspaceHandler`, register the `/api/workspaces*` group under `RequireAuth`.

### Middleware impact

- `RequireSetup`: change "no org → needs setup" to "no account → needs setup".
  Public-path bypass list unchanged.
- `RequireAuth`: unchanged; still injects `org_id`/`role`/`user_id` from the
  session; the session now simply points at the *active* workspace.
- `RequirePermission` / `RequireProjectPermission`: unchanged; they read the
  org-role from context, which is now per-membership but flows identically.
- Data isolation (`WHERE org_id = ?` everywhere): **unchanged**; every table
  still carries `org_id`, and the active-org scoping still works because the
  session's `org_id` is the active workspace.

## Frontend

- `store/workspaces.ts` (new): `workspaces` signal + `activeOrgID`; fetched
  once in `app-shell.ts` alongside the existing sidebar data fetch.
- `components/nav/workspace-switcher.ts`: replace the placeholder dropdown
  with the real list. Switch = `POST /api/workspaces/{id}/switch` → refresh
  auth + reload sidebar data (projects, views, unread, WS). "Add workspace"
  opens a small dialog (`breeze-dialog`) posting to `POST /api/workspaces`.
  Collapsed mode keeps the existing tooltip (logo centered; see the centering
  fix).
- `store/auth.ts`: `fetchMe`/`login` now consume `workspaces` + `active_org_id`
  from the response. On switch, call the API then `fetchMe()` to refresh.
- Routes: no new top-level routes needed for v1 (switching is in-place). The
  dashboard reloads on switch because sidebar data signals change.

## Migrations & codegen order

1. Schema changes (`accounts` table, `users.account_id`, drop
   `password_hash`) live in the consolidated initial schema
   (`00001_initial.sql`), no separate migration file.
2. `make build` (runs migrations on startup is runtime; for codegen we just
   need the schema dir current; `sqlc.yaml` points `schema:` at the migration
   dir, so the new file is picked up).
3. Update `internal/store/queries/*.sql` (orgs, users, sessions) for the
   account/user split.
4. `make sqlc-gen` → regenerates `internal/store/sqlc`.
5. Update store wrappers (`org_store.go`, `user_store.go`, new
   `account_store.go`, `session_store.go`) to map new sqlc types → domain.
6. `make swagger-gen && make api-types` after handler changes.

## Tests

- `service/organization_test.go`: add `CreateWorkspace`, `ListForAccount`,
  `SwitchWorkspace` cases (mock repos).
- `service/auth_test.go`: login with single vs multiple memberships; switch.
- `handler/workspace_test.go` (new): httptest + mock service for the three
  endpoints.

## Out of scope (v1)

- Workspace deletion / transfer ownership.
- Per-workspace display name overrides (account name is shared).
- SSO / magic links.

Invite-by-email **is** handled: an invite accept with an email that already
has an account links the new membership to that existing account (keeping its
password) instead of creating a duplicate credential (see
`internal/service/invite.go`).
