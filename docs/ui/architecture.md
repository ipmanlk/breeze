# Architecture

## Dependency graph (strict, acyclic)

```
routes/  →  features/  →  layouts/
             features/  →  components/  →  lib/  →  types/
             features/  →  store/  →  api/
components/ →  lib/  →  types/
lib/  →  types/  (pure, no framework imports)
```

Lower layers never import higher. `lib/` is pure functions with no Lit or DOM
imports, at the bottom of the dependency graph. Controllers that bridge signals
to Lit (`SignalController`, `OutsideClickController`) live in `lib/` (they only
import `lit` and signals, not higher layers).

## Layer descriptions

| Layer         | Import from      | Contains                                                                                                                                                    |
| ------------- | ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `lib/`        | `types/`         | Pure utilities: `format/` (date/number), `markdown.ts`, `sanitize.ts`, `lexorank.ts`, `schemas/` (valibot). No Lit imports. No DOM access.               |
| `types/`      | nothing          | Hand-written UI-only types for route params, event payloads, store shapes, theme names. Generated API types go in `src/api/types.gen.ts`; don't duplicate. |
| `store/`      | `api/`           | Module-level signal singletons for auth, theme, setup, ws, workspaces. Server-state cache (signal key → T).                                                 |
| `api/`        | (generated)      | `@hey-api/openapi-ts` generated SDK client. Only layer that does raw HTTP.                                                                                  |
| `hooks/`      | `store/`         | Empty for now; `ReactiveController` helpers (`SignalController`, `OutsideClickController`) live in `lib/` and feature folders.                               |
| `components/` | `lib/`           | Reusable UI primitives (`ui/`) and app chrome (`nav/`, `search/`). Props, no store access. Events, not method calls.                                        |
| `layouts/`    | `components/`    | Composite shell layouts using named `<slot>`s for the router.                                                                                               |
| `features/`   | everything below | Product modules (one per domain). Never import from each other's internals; only via `index.ts`.                                                           |
| `routes/`     | `features/`      | Route definitions with pattern, lazy loader, and optional guard.                                                                                            |

## Request flow

```
User action → element event → feature page → API call → signal update → re-render
```

1. User interacts with a UI element (button click, form submit)
2. Element dispatches a custom event
3. Feature page handler calls a store action or API function
4. Response updates a signal
5. Lit elements subscribed via `effect()` or `signal.watch()` re-render

## App shell flow (`app-shell.ts`)

The root `<breeze-app>` element coordinates the full boot sequence:

1. **Init theme** → reads localStorage, applies data attributes
2. **Check setup** → `GET /setup` to see if first-time setup is needed
3. **Fetch me** → `GET /auth/me` to hydrate auth state
4. **Render guard** → shows spinner until all checks resolve, then renders the
   appropriate page based on auth + setup state

## Testing

- `lib/`, `store/`: Vitest, no DOM required.
- Components: `@open-wc/testing` fixture() + Vitest browser mode.
