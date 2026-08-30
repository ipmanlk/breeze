# AGENTS.md: Breeze

## Project

Breeze is a self-hosted project management tool that ships as a **single Go binary** with an embedded Lit/Vite SPA. SQLite database, zero external deps.

**Core philosophy:** Clean layered architecture, test-first, OpenAPI-driven type generation. **All serialized field names (Go json tags, REST DTOs, WS payloads) must use snake_case, never camelCase.**

## Tech Stack

| Layer          | Technology                                              |
| -------------- | ------------------------------------------------------- |
| Backend        | Go + `chi` router + `go-chi/render` + stdlib `net/http` |
| Database       | SQLite via `modernc.org/sqlite` (pure Go, no CGO)       |
| Query gen      | `sqlc`: type-safe code generation from SQL             |
| Migrations     | `pressly/goose`: embedded, auto-run on startup         |
| Auth           | `golang-jwt/jwt/v5` + **argon2id** (HttpOnly cookies)   |
| API docs       | `swaggo/swag`: OpenAPI/Swagger 2.0 from Go annotations |
| Frontend       | Vite + Lit 3 + TypeScript (Deno runtime)                |
| State (server) | Signal stores (feature-local `store.ts`)               |
| State (client) | `@preact/signals-core`                                  |
| Router         | Custom `popstate` router (in `src/routes/`)             |
| UI primitives  | CSS custom property tokens + scoped Lit styles          |

## Architecture & Conventions

**Heavy documentation lives in `docs/`. AGENTS.md links to it, never embed full docs here.**

Full doc index: [`docs/README.md`](docs/README.md).

| Section | Covers |
| ------- | ------ |
| [`docs/api/`](docs/api/architecture.md) | Go backend: architecture, permissions, pagination, chat/WS/voice |
| [`docs/ui/`](docs/ui/overview.md)       | Frontend SPA (stack, patterns, pitfalls)                  |
| [`docs/ops/`](docs/ops/configuration.md)| Env vars, self-hosting, backups, upgrades                   |
| [`docs/i18n.md`](docs/i18n.md)         | Internationalization (frontend + backend)                |

Frontend authoring rules live in [`ui/AGENTS.md`](ui/AGENTS.md), not in
`docs/`.

### Layer Rules (quick ref)

| Layer         | Directory             | Depends on         | Contains                                                                 |
| ------------- | --------------------- | ------------------ | ------------------------------------------------------------------------ |
| **Domain**    | `internal/domain/`    | nothing            | Pure types per entity group                                              |
| **Port**      | `internal/port/`      | domain             | Consumer-side interfaces (repo, service)                                 |
| **Store**     | `internal/store/`     | domain, port       | sqlc-generated code + wrappers implementing port interfaces              |
| **Service**   | `internal/service/`   | domain, port       | Business logic, takes port interfaces as constructor args                |
| **Transport** | `internal/transport/` | domain, port, auth | HTTP handlers, middleware, response helpers, DTOs                        |
| **Auth**      | `internal/auth/`      | nothing            | Argon2 password hashing + opaque session token service                   |

Key: Handlers never touch `*sql.DB` or `*sqlc.Queries`. Service owns business logic. Store owns data access.

## Directory Structure

```
/
├── Makefile
├── cmd/server/main.go       # Entry point (wires DB → repos → services → handlers)
├── cmd/seed/main.go         # DB seeder
├── internal/
│   ├── domain/              # Pure types (user, org, project, task, session, etc.)
│   ├── apperr/              # Sentinel errors
│   ├── port/                # Repository & service interfaces
│   ├── auth/                # Argon2id + session tokens
│   ├── store/               # sqlc-generated code + wrappers (queries/, migrations/)
│   ├── service/             # Business logic
│   ├── transport/           # HTTP handlers, middleware, response helpers, DTOs
│   └── storage/             # File storage interface + local implementation
├── ui/                      # Vite + Lit SPA (has its own AGENTS.md; frontend rules live there)
│   ├── embed.go             # go:embed dist
│   ├── AGENTS.md            # UI rules + pitfalls (read this for frontend work)
│   └── src/                 # components/, features/, store/, styles/, routes/, api/
├── api/swagger/             # Generated OpenAPI spec
├── docs/                    # All docs (api/, ui/, ops/; see docs/README.md)
└── data/                    # Runtime data (gitignored)
```

## Developer Rules

1. **ALWAYS use `make` commands** for building, testing, generating code, and running tasks. Never `go build`, `deno task build`, etc. directly. See `docs/api/build-commands.md` for full reference.
2. **Only use tools already available in `PATH` (e.g. `sqlc`, `air`, `swag`).** If a required tool is missing, inform the user and stop work. Never run `go run github.com/...@latest` or install anything without explicit user confirmation.
3. **NEVER use npm.** All UI dep management via `cd ui && deno add npm:<package>`.
4. **NEVER use `npx`.** Use `deno run npm:<package>@<version>` or `deno x npm:<package>@<version>`.
5. **ALWAYS format:** `go fmt ./...` for Go, `deno fmt` for TS. Do NOT format `ui/src/api/` (auto-generated).
6. **ALWAYS rebuild with `make build`** after migration or sqlc changes.
7. **ALWAYS write tests.** Meaningful tests, not coverage padding.
8. **ALL database access through sqlc-generated code.** No raw SQL outside `queries/*.sql`.
9. **Handlers never receive `*sql.DB`.** They take interfaces from the domain package.
10. **Frontend API calls use generated OpenAPI SDK.** Every Go handler must have swagger annotations (`@Summary`, `@Router`, `@Tags`). After handler changes: `make swagger-gen && make api-types`. Frontend imports from `@/api/types.gen` and `@/api/sdk.gen`. No raw `fetch()`. No `lib/api-client.ts`.
11. **Heavy documentation goes in `docs/`.** AGENTS.md links to it, do not embed full pattern docs here.
12. **Never start a dev server.** Do not run `make dev`, `make dev-ui`, `make dev-api`, `air`, `vite`, or any other long-lived/watch process. For verification use build + type-check only (`make build`, `make build-ui`, `go test ./...`, `deno check`). If runtime/visual confirmation is needed, **ask the user**, never run a dev server yourself.
13. **NEVER commit without explicit user confirmation.** Never run `git commit` unless the user explicitly asks for it each time. Even if the user has confirmed commits in previous conversations, always ask for confirmation before each commit.

## Agent-Only Notes

### Environment Variables

Full reference in `docs/ops/configuration.md`. Quick list:

| Variable     | Default            | Description                |
| ------------ | ------------------ | -------------------------- |
| `PORT`       | `8080`             | HTTP listen port           |
| `DB_PATH`    | `./data/breeze.db` | SQLite database path       |
| `UPLOAD_DIR` | `./data/uploads`   | File storage root          |
| `JWT_SECRET` | (required)         | Secret key for JWT signing |

Loadable from `.env` file (takes precedence: real env > `.env`). Copy `.env.example` to `.env` to get started.

### API Client Config

API client config (`baseUrl`, `credentials: "include"`) is set in `ui/src/main.ts` using `VITE_API_BASE_URL` env var. This is outside the generated `ui/src/api/` directory, so regenerating the SDK with `make api-types` will never wipe it.
