# Breeze Architecture

## Overview

Breeze is a single-binary project management application. The Go backend embeds
the compiled Lit SPA and serves it alongside a REST API. SQLite provides the
database with zero external dependencies.

## Request Flow

```
HTTP Request → chi.Router
  │
  ├── chimw.RequestID, Logger, Recoverer, CleanPath
  ├── SecurityHeaders (CSP, X-Content-Type-Options, etc.)
  ├── CORS (development, or whenever CORS_ORIGINS is set)
  ├── LocaleMiddleware (Accept-Language → locale in context)
  ├── middleware.RequireSetup (first-run gate)
  │     ├── bypasses /api/setup, /api/auth/login and the other public routes
  │     ├── returns JSON 412 for other /api/* when no org
  │     └── redirects SPA to /setup when no org

Public routes (no auth):
  ├── GET  /api/swagger/*  → Swagger UI (development only)
  ├── GET  /api/setup      → SetupHandler.Check (also POST Setup)
  ├── POST /api/auth/login → AuthHandler.Login
  ├── GET/POST /api/auth/password-reset/*  → reset flow
  ├── GET/POST /api/invites/{token}/*      → invite validation/accept
  ├── GET  /healthz        → liveness probe
  └── GET  /api/version    → build version (ldflags-injected)

Protected group (RequireAuth):
  ├── POST /api/auth/logout → AuthHandler.Logout
  └── GET  /api/auth/me    → AuthHandler.Me

NotFound → spaHandler (embedded Vite dist/)
```

All public endpoints and the protected group sit behind CSRF protection and a
JSON body-size limit. Upload and backup-restore routes use larger dedicated
limits (see `../ops/configuration.md`).

## Layered Architecture

### Dependency Flow

`internal/app/app.go` is the composition root (not `cmd/server/main.go`, which
only loads config and starts the app):

```
app.New(cfg) → store.NewDB(dbPath) → RunMigrations (goose)
        │
        ├── sqlc.New(conn) → *Queries (sqlc generated)
        │
        ├── store.NewUserStore(queries)        → port.UserRepository
        ├── store.NewOrgStore(queries)         → port.OrganizationRepository
        ├── store.NewAccountStore(queries)     → port.AccountRepository
        ├── store.NewSessionStore(queries)     → port.SessionRepository
        │
        ├── service.NewAuthService(deps)       → JWT (HS256) + argon2id via internal/auth
        ├── service.NewOrganizationService(deps)
        ├── service.NewProjectService(deps)
        ├── service.NewTaskService(deps)
        │
        ├── handler.NewAuthHandler(deps)
        ├── handler.NewSetupHandler(deps)
        │
        └── chi.NewRouter() ← wires all handlers + middleware
```

### Layer Responsibilities

| Layer      | Directory                      | Responsibility                        | Can Import                   |
| ---------- | ------------------------------ | ------------------------------------- | ---------------------------- |
| Domain     | `internal/domain/`             | Structs, enums, pure types            | Nothing from `internal/`     |
| Port       | `internal/port/`               | Consumer-side repo/service interfaces | Domain                       |
| Store      | `internal/store/`              | sqlc-generated code + thin wrappers   | Domain + port + sql          |
| Service    | `internal/service/`            | Business logic, validation            | Port interfaces (injected)   |
| Transport  | `internal/transport/`          | HTTP handlers, middleware, response    | Domain + port + auth         |
| Auth       | `internal/auth/`               | Argon2 hashing + session tokens       | Nothing                      |
| Storage    | `internal/storage/`            | Multi-backend file storage            | Domain                       |

### Layer Rules

1. **The port layer defines all repository/service interfaces.** Stores implement the repo ports; services consume them via constructor injection.
2. **Handlers receive interfaces, never concrete types.** No `*sql.DB` in handlers.
3. **Services take interfaces via constructor injection** (port interfaces).
4. **All DB access goes through sqlc-generated code.** No raw SQL outside `queries/*.sql`.
5. **Store wrappers convert sqlc types → domain types** (sqlc dates are `string`, domain dates are `time.Time`).
6. **JSON rendering uses `transport.JSON()` and `transport.ErrorJSON()`**, which wrap `go-chi/render`. DTOs use `dto.BindAndValidate()`.

## Middleware Chain

chi route groups control which middleware applies:

```
Global (all routes):
  1. chimw.RequestID, Logger, Recoverer, CleanPath
  2. SecurityHeaders, optional CORS, LocaleMiddleware
  3. RequireSetup: checks org existence (bypasses the public routes)

Public routes (no auth):
  4. CSRFProtection + LimitRequestBody + rate limiting per endpoint

Protected group:
  4. RequireAuth: validates the JWT session token from the `__Host-token` cookie
     Injects user_id, org_id, role, session_id into context
  5. CSRFProtection + LimitRequestBody
  6. Handler: business logic → Service → Store → SQLite

SPA routes (NotFound):
  4. RequireSetup redirects to /setup if no org
  5. spaHandler serves index.html or static assets
```

## Data Isolation

Every table includes an `org_id` column. Auth middleware injects `org_id` into
`context.Context`. Project-scoped repository queries include `WHERE org_id = ?`,
and the `RequireProjectPermission` middleware verifies the requested project
belongs to the caller's org (via `access.ResolveEffectiveRole`) before any
project-child handler runs. This closes the cross-org bypass for elevated org
roles.

Login searches across all orgs via `AccountRepository.GetByEmail` (accounts hold
the global credential). After login, all queries are org-scoped.

## OpenAPI / Swagger

Handlers annotated with `swaggo/swag`:

```go
// @Summary      Login
// @Router       /api/auth/login [post]
```

- `make swagger-gen` generates `api/swagger/swagger.json`
- Swagger UI at `/api/swagger/` (development only)
- `make api-types` generates TypeScript SDK into `ui/src/api/` (types.gen, sdk.gen, client.gen)
- Run `make swagger-gen && make api-types` after every handler change

## UI Architecture

The frontend is a Vite + Lit 3 + TypeScript SPA (Deno runtime). See
`../../ui/AGENTS.md` for the authoritative frontend rules.

```
ui/src/
├── components/         ← Lit UI primitives (breeze-button, breeze-input, …)
│   ├── ui/             ← leaf primitives
│   ├── nav/            ← app chrome (sidebar, workspace switcher, top bar)
│   └── search/         ← command palette
├── features/<name>/     ← Feature modules (components/, store.ts, index.ts)
├── routes/             ← Custom popstate router (src/routes/router.ts)
├── store/              ← @preact/signals-core singletons (auth, theme, setup, ws, …)
├── styles/             ← Design-token CSS + global styles
├── themes/             ← Theme token overrides
├── lib/                ← Pure utilities (signal-controller, format, markdown)
├── types/              ← Hand-written UI-only types (currently a placeholder)
├── i18n/               ← @lit/localize config, messages, locales
├── api/                ← Generated OpenAPI SDK (types.gen, sdk.gen, client.gen)
├── app-shell.ts        ← Light-DOM shell + route rendering + lazy page chunks
└── main.ts             ← Entry point (client config + interceptors)
```

### State Management

- **Server state:** fetched via the generated SDK (`@/api/sdk.gen`), wrapped in
  feature-local signal stores.
- **Client state:** `@preact/signals-core` signals for theme, auth status, setup
  status, WS state, UI toggles.
- **Reactivity:** `SignalController` (Lit) re-runs `willUpdate`/`render` when
  watched signals change.
- **API calls:** Generated SDK via `@/api/sdk.gen`, re-exported through
  `features/*/store.ts`.
- **No raw `fetch()`**: always use the generated SDK (the auth probe in
  `ws.ts` is the lone exception).

## Setup Flow

1. **App mounts**: `app-shell.ts` fires `checkSetup()`
2. **No org**: `RequireSetup` redirects SPA routes to `/setup`
3. **Setup wizard**: org name → admin name + email + password (POST `/api/setup`)
4. **Complete**: backend creates org + admin user, sets the `__Host-token`
   session cookie (JWT, 7-day)
5. **Redirect**: wizard navigates to `/login`. After login, the login page
   calls `fetchMe()` and redirects to `/` (or `?next=` when it is a safe
   same-origin path)
6. **Revisit `/setup`**: `setup-page.ts` shows "already configured" with a link
   to `/login`

## Testing Patterns

| Area                   | Pattern                                                              |
| ---------------------- | -------------------------------------------------------------------- |
| **Service tests**      | Mock repository interfaces via `mock_test.go`, test business logic   |
| **Handler tests**      | `httptest.NewRecorder` + mock repos + context injection for auth     |
| **WS hub tests**       | Behavioral tests with no network; assert room isolation, broadcast  |
| **WS handler tests**   | Real WebSocket via `httptest.Server` + `websocket.Dial`              |

No external test dependencies beyond stdlib.

## Embedded SPA

`ui/embed.go` uses `//go:embed dist` to embed the Vite build output. The router serves static assets from the embedded FS and falls back to `index.html` for SPA client-side routing.
