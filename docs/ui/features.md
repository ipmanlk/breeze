# Features

## Location

`src/features/` contains product modules, one directory per domain.

## Current modules

```
auth/          → login-page, forgot-password-page, reset-password-page
chat/          → chat-page, inbox-page (heaviest: WS, rich text, DnD)
comments/      → task comment components
dashboard/     → dashboard-page
members/       → members-page, invite-accept-page (+ api.ts, store.ts)
my-tasks/      → my-tasks-page (cross-project "My Issues")
notifications/ → inbox-page
projects/      → projects-page, project-detail-page, kanban-board, list-view…
settings/      → user-settings, workspace-settings, labels, audit-log
setup/         → setup-page
views/         → views-page, view-detail-page (+ store.ts, types.ts)
voice/         → voice channel UI
workspaces/    → workspace switcher UI
```

## Pattern

Features are not forced into one fixed shape. A feature module typically has a
page element and may add `components/`, a feature-local `store.ts`, an
`api.ts`/`types.ts`, and an `index.ts` public entry when other features need to
import from it:

```
features/<name>/
├── index.ts              # public API, only when others import it
├── <name>-page.ts        # root page element for this feature
├── components/           # feature-specific components
├── api.ts                # API call wrappers, when the feature has custom calls
├── store.ts              # feature-local signals
└── types.ts              # feature-local types (e.g. ViewFilters)
```

Small features (auth, setup) are a single page file with no `store.ts`; larger
ones (projects, chat) group their logic under `components/` and `store.ts`.

## Rules

- Features never import from each other's internals.
- Cross-feature access goes through each feature's `index.ts` public API when
  one exists; otherwise through the shared stores/api client.
- Feature pages are lazy-loaded by `src/routes/` definitions (see
  `code-splitting.md`).