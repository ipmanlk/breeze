# Plume UI: Overview

Frontend for Plume. Built with Lit 3 + Vite 8 + TypeScript 6.

## Stack

| Layer           | Choice                                                   |
| --------------- | -------------------------------------------------------- |
| Components      | Lit 3                                                    |
| Build           | Vite 8                                                   |
| Language        | TypeScript 6 (strict)                                    |
| Package manager | Deno 2 (`deno install`)                                  |
| State           | signals (`@preact/signals-core`)                         |
| Server state    | signal stores (feature-local `store.ts`)               |
| Routing         | custom `popstate` router (`src/routes/router.ts`)       |
| Styling         | CSS custom properties (tokens) + scoped Lit styles       |
| Forms           | native `<form>` + valibot validation                     |
| DnD             | `@atlaskit/pragmatic-drag-and-drop` (framework-agnostic) |
| Rich text       | `@tiptap/core` (framework-agnostic)                      |
| API SDK         | `@hey-api/openapi-ts` generated client                   |

## Directory structure

```
ui/
├── package.json         # deps (managed by deno install)
├── tsconfig.json        # TypeScript config
├── vite.config.ts       # Vite config with API proxy
├── index.html           # SPA entry → /src/main.ts
├── embed.go             # go:embed dist for Go binary
├── public/              # static assets
└── src/
    ├── main.ts          # bootstrap (sets API client config)
    ├── app-shell.ts     # root <plume-app> element
    ├── api/             # generated OpenAPI SDK (do not edit)
    ├── styles/          # global reset + design tokens
    ├── themes/          # theme token overrides
    ├── components/      # reusable UI building blocks
    │   ├── ui/          # leaf primitives (plume-button, etc.)
    │   ├── nav/         # app chrome (sidebar, top bar)
    │   └── search/      # global search
    ├── features/        # product modules (auth, dashboard, etc.)
    ├── routes/          # route definitions + custom router
    ├── layouts/         # composite layout elements
    ├── i18n/            # @lit/localize config, messages, locales
    ├── lib/             # utilities (formatters, signal-controller, markdown)
    ├── store/           # global signals
    └── types/           # UI-only type definitions
```

## Make targets

| Target          | Description                  |
| --------------- | ---------------------------- |
| `make dev-ui`   | Vite dev server on :5173     |
| `make dev`      | Vite + Air backend           |
| `make build-ui` | `tsc --noEmit && vite build` |
| `make setup`    | `deno install`               |

## Project rules

1. **All custom element tags start with `plume-`**, avoiding collisions.
2. **One element per file.** Filename = tag minus `plume-`.
3. **Scoped styles only.** `static styles = css\`...\``. No global CSS from
   components.
4. **No Tailwind, no shadcn.** CSS tokens + scoped Lit styles.
5. **No raw `fetch()` outside `src/api/`.** Use generated SDK.
6. **All serialized field names are snake_case** (matches API).
7. **`deno fmt` for formatting.** Not prettier.
8. **Heavy docs in `docs/`.** AGENTS.md links to them.
