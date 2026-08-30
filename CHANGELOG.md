# Changelog

All notable changes to Breeze are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

The running build version is available at `GET /api/version` (injected via
ldflags at build time).

## [Unreleased]

## [0.1.1] - 2026-08-30

### Fixed
- Docker build: added `apt-get update` before installing `unzip` (apt lists
  were already cleaned in a prior RUN step, causing the build to fail)
- Docker build: run `deno install` before `deno task build` so UI dependencies
  (tsc, vite, etc.) are available during the build
- Middleware: static assets (`/assets/*.js`, `/assets/*.css`, `/favicon.svg`,
  etc.) are now exempt from the setup redirect, so the SPA loads correctly
  before initial setup is complete
- `.dockerignore`: exclude non-essential paths (docs, CI, e2e tests, agent
  tooling, dev configs) to slim the build context

## [0.1.0] - 2026-08-30

Initial public release. **Breeze is in beta (0.x): APIs, data schemas, and
behavior may change between releases without notice.** Pin an exact version
in production deployments and back up your data before upgrading.

### Security
- Channel access: project-linked channels no longer bypass `channel:view`
  deny rules or per-user overrides; cross-org channel IDs are rejected
  (fail-closed) before any membership or permission lookup
- Closed IDOR reads on task custom-field values and task labels: both now
  enforce the caller's task-level access like the write paths
- View pin/unpin is org-scoped; task-dependency list endpoints enforce
  project access for viewer/guest roles
- Message search post-filters results through the conversation access layer
  so per-channel denies apply to search hits
- Global search scopes channels to visible channels, DMs to own
  conversations, and projects/recent projects to memberships for viewer/guest
- Dashboard "projects" section is membership-scoped for viewer/guest roles
- WS typing indicators now require conversation access, not just send
  resolution; voice join resolves permissions with the caller's real org role
  (previously an empty role denied everyone / failed open under overrides)
- Invite-token validation endpoint rate-limited; loud startup warning when
  `CORS_ORIGINS=*` reflects credentialed requests

### Changed
- Channel management policy: owners/admins bypass all channel-level rules and
  can manage any channel in their org (rename/delete/members/permissions);
  management endpoints key off resolved `channel:manage`, not view access.
  Attachment download moved from the send tier to the read tier (view ⇒
  download). Sending still requires explicit channel membership
- Seeder requires `-force` and refuses to run with `APP_ENV=production`;
  seeded account credentials documented in `cmd/seed/main.go`
- Docker images accept `GIT_VERSION` build arg and create `/data` owned by
  the runtime user; CI verifies generated TS API types (`make api-types`)
- Docs reorganized: backend docs moved to `docs/api/`, frontend docs moved
  from `ui/docs/` to `docs/ui/`, deployment docs to `docs/ops/`; `docs/README.md`
  is the new index, and the redundant `docs/ui-standards.md` was removed

### Fixed
- Removed internal audit/ticket references from comments, migrations, and the
  public OpenAPI spec; corrected stale migration headers and false comments
- Docs overhauled for accuracy: setup (JWT secret length), configuration
  (`PORT`, log level, `AUDIT_*` vars), permissions matrix + new channel
  permission section, WebSocket/pagination/chat docs de-Reacted,
  `ui-standards.md` rewritten for the Lit stack, `.env.example` completed
- UI: removed unshipped "Timeline" placeholder tab; members page shows a
  visible error state instead of a silent empty list; channel context menu
  gated by resolved `can_manage`; reconnect comment matches actual behavior;
  `deno check` now passes via aligned compiler options

### Security
- Rate-limited `POST /api/setup`
- Message attachment downloads hardened against stored XSS: blocked-type
  downgrade + `X-Content-Type-Options: nosniff`
- AccessChecker nil-guards now fail CLOSED (deny) instead of fail-open
- CSRF defense-in-depth: Origin/Referer check middleware on state-changing
  methods; rejects cross-origin + `Origin: null`
- Differentiated request body limits: 64KB JSON default, 50 MiB uploads
  (`MAX_UPLOAD_SIZE` default), 200MB backup restore (was uniform 1MB)

### Added
- Internationalization (i18n): French (fr) as launch locale. All
  user-facing strings in the SPA (693 messages) wrapped in `msg()` for
  `@lit/localize` runtime localization. Backend notification/email/push/error
  strings localized via `go-i18n`. Backend catalogs fully translated to
  French; frontend XLIFF catalog fully translated (693/693). Locale is
  per-user via `user_preferences.language` (existing col). Authenticated
  users' stored language preference overrides Accept-Language for error
  responses; per-recipient notifications resolve the recipient's language.
- Plural-boundary test: `TestBundlePlural_FrenchZeroOne` validates French
  `one={0,1}` vs English `one={1}`.
- Notification labels ("Task assignments", "Direct messages", etc.) moved
  from backend `NotificationLabels` map / `GET /settings/notifications`
  `label` field to frontend `getNotificationLabel()` helper using `msg()`.
  Breaking: `label` field removed from `DtoNotificationPreferenceResponse`.
  Frontend now localizes these labels via the i18n pipeline (6 new strings).

### Data correctness
- Lexorank position-key race closed via transactional `GeneratePositionKey` /
  `GenerateSubtaskPositionKey`
- `TaskStore.Create`/`Update` now atomic (task + assignees in one tx)
- `BatchUpdate` now atomic via `RunInTransaction`
- `ReorderSubtasks` wrapped in a transaction
- FTS5 search for tasks + projects (replaces `instr()` scans)
- Added `idx_tasks_org_project` + covering + `idx_conversations_active_ordered`
  partial index
- `time_entries.user_id` + `comments.author_id` gain `ON DELETE CASCADE`
- `ListByIDs` org-scoped in SQL; added `ListByIDsFull` for mutation paths
- Fixed chi route shadowing where `tasks/batch` + `tasks/reorder` 405'd (bonus)

### Features
- Audit log extended to task events (`task_created`/`task_deleted`)
- Task activity tracks title/description/label changes
- Audit log retention: `AUDIT_RETENTION` + periodic purge
- Recurring template errors logged + persisted in `last_error` column
- WS `handleVoiceJoin` now checks conversation access

### Infrastructure
- Structured JSON logging in production
- Build versioning: ldflags + `GET /api/version` + CHANGELOG
- `make build-with-codegen` target; `regen-all` ordering fixed (sqlc first)
