# Breeze UI: Lit + Vite + TypeScript

> **This is the source of truth for all frontend work.** UI-specific rules,
> patterns, and pitfalls live here and in `docs/`, **not** in the root
> `AGENTS.md` or root `docs/`.

## Quick reference

- **Stack & directory tree** → [overview.md](../docs/ui/overview.md)
- **Architecture & dependency graph** →
  [architecture.md](../docs/ui/architecture.md)
- **Components & conventions** → [components.md](../docs/ui/components.md)
- **Feature module pattern** → [features.md](../docs/ui/features.md)
- **Signal-based state management** → [store.md](../docs/ui/store.md)
- **Pure utility modules** → [lib.md](../docs/ui/lib.md)
- **ReactiveControllers (hooks)** → [hooks.md](../docs/ui/hooks.md)
- **Custom router & route definitions** → [routes.md](../docs/ui/routes.md)
- **Layout components** → [layouts.md](../docs/ui/layouts.md)
- **Theme token system** → [themes.md](../docs/ui/themes.md)
- **Global styles & design tokens** → [styles.md](../docs/ui/styles.md)
- **UI-only types** → [types.md](../docs/ui/types.md)
- **v1 → v2 porting guide** → [porting.md](../docs/ui/porting.md)
- **Code splitting (adding a lazy route/page)** →
  [code-splitting.md](../docs/ui/code-splitting.md)

### Pitfalls & patterns (read the one matching your task, don't load all)

- **Lit component patterns (read before any interactive component)** →
  [lit-patterns.md](../docs/ui/lit-patterns.md)
- **Design tokens (AGENTS rule: tokens only)** →
  [design-tokens.md](../docs/ui/design-tokens.md)
- **Theme-aware colors in shadow DOM (`light-dark()`)** →
  [theme-colors.md](../docs/ui/theme-colors.md)
- **Toggle/switch controls (shadcn Switch parity)** →
  [toggle-controls.md](../docs/ui/toggle-controls.md)
- **Infinite scroll & cursor pagination in Lit** →
  [infinite-scroll.md](../docs/ui/infinite-scroll.md)
- **Command palette (wiring, anatomy, extending)** →
  [command-palette.md](../docs/ui/command-palette.md)
- **Sidebar (shadcn spacing parity, header width, collapsed-dropdown clipping,
  hover-action rows)** → [sidebar.md](../docs/ui/sidebar.md)
- **`data-fullscreen` layout (fixed viewport, chat/kanban pages, prevents
  input-off-screen)** →
  [docs/layouts.md](../docs/ui/layouts.md#data-fullscreen-attribute)

## Highlights

- **Stack:** Lit 3 + Vite 8 + TypeScript 6 + `@preact/signals-core` (~1KB)
- **Styling:** CSS custom property tokens + scoped Lit styles, no Tailwind, no
  shadcn
- **Routing:** Custom `popstate` router, zero dependencies
- **State:** Module-level signal singletons, no global store library
- **API:** Auto-generated SDK via `@hey-api/openapi-ts`
- **Build:** `make dev-ui`, `make build-ui`, `make setup`

## Agent rules

1. Read the relevant `docs/*.md` (and the pitfalls doc matching your task type)
   before making changes in that area.
2. All custom element tags must start with `breeze-`. One element per file.
3. **Scoped styles preferred**; use
   `static styles = css\`...\``in
   shadow-DOM components. Never inline`<style>`in`render()`or
   sub-templates.
   **Exception, light-DOM components:** components that integrate with
   Atlaskit pragmatic-drag-and-drop (e.g.`app-shell.ts`,`chat-page.ts`,`kanban-board.ts`,`workspace-sidebar.ts`,`channel-item.ts`) render into
   the light DOM (`createRenderRoot()
   { return this;
   }`) because DnD requires
   it. These must (a) use a`breeze-`-prefixed or`cp-`-prefixed tag to
   namespace their global styles, and (b) inject any global CSS via a single
   shared`const
   APP_STYLES =
   \`...\``/`CP_STYLES`string rather than
   per-render duplication. No component should re-declare design-token values;
   always reference`var(--token)`.
4. **Design tokens only**: all values use `var(--token)` from
   `src/styles/index.css`. No hardcoded `rem`/`px`/`ms` or raw color literals
   (accepted exceptions: `1px` hairline borders, `@keyframes` animation
   durations, fixed-contrast colors like white-on-badge; see
   [design-tokens.md](../docs/ui/design-tokens.md)). Every shadow-DOM component
   includes `*, *::before, *::after { box-sizing: border-box; }`.
5. **Theme-aware colors** use `light-dark()` (resolves from inherited
   `color-scheme`), never `:host-context()` or `:host([data-theme])`. See
   [theme-colors.md](../docs/ui/theme-colors.md).
6. **Lit component patterns** (`?attr` for boolean attrs, `attribute: false` for
   Array/Object `@property`, `composedPath()` for outside-click,
   `changedProps.has()` guards, always-in-DOM for dialogs/toggles,
   `bubbles: true, composed: true` events) are enforced; see
   [lit-patterns.md](../docs/ui/lit-patterns.md).
7. No raw `fetch()`; use the generated SDK from `src/api/`. Never edit
   `src/api/` (auto-generated); the only API config outside it is `src/main.ts`.
8. All serialized field names are **snake_case** (matches API).
9. **Build:** type-check `cd ui && deno check src/main.ts`; build
   `cd ui && deno task build`; format `cd ui && deno fmt src/` (never format
   `src/api/` (auto-generated OpenAPI types, nor `src/i18n/locales/` or
   `src/i18n/locale-codes.ts`, auto-generated by `lit-localize`). Or via the
   repo Makefile: `make build-ui`. The `deno.json` `fmt.exclude` config enforces
   these exclusions automatically.
10. **Don't duplicate global styles**: scrollbar, `box-sizing`, `sr-only`,
    `prefers-reduced-motion` are already global in `src/styles/index.css`.
11. **Never start a dev server.** Do not run `make dev-ui`, `vite`, or any
    long-lived/watch process. Verify with `deno check src/main.ts` +
    `make build-ui` only. If runtime/visual confirmation is needed, ask the
    user; never run a dev server yourself.
12. **Adding npm deps**: `cd ui && deno add npm:<pkg>@<ver>` (never npm). Check
    for **transitive version conflicts**: a sub-dependency may pin a different
    major of a shared dep (e.g. `@atlaskit/pragmatic-drag-and-drop-hitbox` 1.x
    pulls `pragmatic-drag-and-drop` 1.x, while we use 2.x → two copies,
    type/runtime mismatches). Pick the latest version whose deps align with
    what's already installed (verify via `deno.lock`). Confirm with the user
    before installing anything not already in the project.
