# Routes

## Location

`src/routes/` contains a custom tiny router + route definitions.

## Router (`router.ts`)

A small custom router built on `popstate` + `history.pushState`. Zero
dependencies.

### API

```ts
import { currentPath, matchRoute, navigate } from "@/routes/router";

// Read current path (signal, reactive)
const path = currentPath.value;

// Navigate programmatically
navigate("/projects/123");

// Match a pattern with params
const params = matchRoute("/projects/:id", "/projects/123");
// → { id: '123' }
```

- `currentPath`: a `signal<string>` that updates on `popstate` and
  `navigate()`.
- `navigate(to)`: pushes to history, updates `currentPath`.
- `matchRoute(pattern, path)`: returns param map or null.

## Route definitions

Routes are matched inline in `src/app-shell.ts` `willUpdate()` via `matchRoute()`; there is
no per-file `Route` object. Lazy chunks are declared in `lazyPages` and
preloaded via `_ensure(tag, loader)`. To add a route, add a `matchRoute()`/
`path ===` branch in `PlumeApp.willUpdate()` and `render()`.

Example from `app-shell.ts`:
```ts
if (matchRoute("/projects/:slug", path)) {
  this._ensure("plume-project-detail-page", lazyPages.projectDetail);
}
```
