# Build Commands

**ALWAYS use `make` commands**, not raw `go build`, `deno task`, etc.

All code-generation targets require the corresponding tool to be already available in `PATH`:
- `make sqlc-gen` requires `sqlc`
- `make swagger-gen` requires `swag`
- `make api-types` requires Deno + the generated OpenAPI spec

If a tool is missing, stop and inform the user. Do not fetch or install tools automatically.

## Development

| Command     | Description                                    |
| ----------- | ---------------------------------------------- |
| `make dev`  | Full dev stack (UI + API with Air hot-reload)  |
| `make dev-ui` | Vite dev server only                         |
| `make dev-api` | Go backend with Air hot-reload only         |

## Building

| Command          | Description                              |
| ---------------- | ---------------------------------------- |
| `make build`     | Complete app (UI → Go binary)            |
| `make build-ui`  | UI only (outputs to ui/dist/)            |
| `make build-api` | Go binary only                           |
| `make full-build`| Clean + build                            |
| `make build-with-codegen` | `regen-all` then `build`. Use after schema/handler changes so generated code is never stale |

## Testing & Code Gen

| Command           | Description                                         |
| ----------------- | --------------------------------------------------- |
| `make test`       | Run Go tests (UI tests are `make test-ui`)          |
| `make test-go`    | Go tests only                                       |
| `make test-race`  | Go tests with the race detector (`-race`)           |
| `make test-ui`    | UI tests only                                       |
| `make sqlc-gen`   | Generate Go code from SQL queries                   |
| `make swagger-gen`| Generate OpenAPI spec from Go annotations           |
| `make api-types`  | Generate TypeScript types from OpenAPI spec          |
| `make regen-all`  | All generators (sqlc → swagger → api-types)        |

## Linting & Internationalization

| Command            | Description                                                        |
| ------------------ | ------------------------------------------------------------------ |
| `make lint-ui`     | Check `ui/src` for `.innerHTML` usage (must use `createElement` / `replaceChildren`) |
| `make i18n-extract`| Scan `ui/src` for `msg()` calls and update `ui/src/i18n/messages/*.xlf` |
| `make i18n-build`  | Merge translated `.xlf` into runtime modules `ui/src/i18n/locales/*.js` |

Run `i18n-extract` after adding/changing `msg()` strings; run `i18n-build`
after editing the `.xlf` translations. See `../i18n.md`.

## Database & Utilities

| Command       | Description                               |
| ------------- | ----------------------------------------- |
| `make seed`   | Seed database with sample data            |
| `make setup`  | Install all dependencies (Go + UI)        |
| `make clean`  | Remove bin/, tmp/, ui/dist/               |
| `make fmt-go` | Format Go code (`go fmt ./...`)           |
| `make fmt-ui` | Format TypeScript (`cd ui && deno fmt`)   |
