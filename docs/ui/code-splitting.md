# Code Splitting (Route-Level Lazy Loading)

How to add a new page as its own chunk so a cold visit only downloads the code
for the route it lands on. Read this when adding a new authenticated route/page
to `ui/`.

**Strategy: dynamic `import()` only.** Vite/Rollup turns every dynamic
`import()` into its own chunk automatically, no `manualChunks`, no
`rollupOptions` tuning, no vendor splitting. We just stop forcing every page
into the entry bundle.

The canonical, working implementation is `src/app-shell.ts`; read it alongside
this doc.

---

## When to split vs. keep static

| Keep **static** (in entry bundle)              | Split into a chunk (lazy `import()`)           |
| ---------------------------------------------- | ---------------------------------------------- |
| First-paint-critical screens: `login`, `setup` | Everything only seen **after auth**            |
|                                              | Any heavy page (kanban/DnD, rich text, charts) |

Rule of thumb: if a logged-out visitor can land on it, keep it static (a spinner
before the login form is unacceptable). Everything post-auth gets split; that's
where the real weight is.

## What NOT to split

- **UI primitives** (`plume-button`, `plume-dialog`, …): tiny and shared
  everywhere; splitting adds chunk overhead + round trips.
- **Stores / API client / layouts**: needed on every authenticated screen;
  splitting them only adds latency.
- **The router** (`currentPath`, `matchRoute`, `navigate`): stays as-is; lazy
  loading is purely about _when_ page modules register their custom elements.

## The pattern (copy from `app-shell.ts`)

### 1. Lazy loader map

Plain functions, no registry needed; `import()` is browser-memoized:

```ts
const lazyPages = {
  dashboard: () => import("./features/dashboard/dashboard-page.ts"),
  projects: () => import("./features/projects/projects-page.ts"),
  projectDetail: () => import("./features/projects/project-detail-page.ts"),
  inbox: () => import("./features/notifications/inbox-page.ts"),
} as const;
```

### 2. Loading gate: `_ready` Set + `_ensure` helper

Pages are custom elements, so wait on `customElements.whenDefined` before
rendering the tag. A new `Set` (not `.add()` in place) triggers the update:

```ts
@state() private _ready = new Set<string>();

private async _ensure(tag: string, loader: () => Promise<unknown>): Promise<void> {
  if (this._ready.has(tag)) return;
  await Promise.all([loader(), customElements.whenDefined(tag)]);
  this._ready = new Set(this._ready).add(tag); // new Set → triggers update
}
```

### 3. Kick off the load in `willUpdate()` (before render)

Only for the authenticated, setup-done state:

```ts
protected willUpdate(): void {
  const path = currentPath.value;
  if (setupRequired.value || auth.value.isLoading || !auth.value.isAuthenticated) return;

  if (path === "/projects") this._ensure("plume-projects-page", lazyPages.projects);
  else if (matchRoute("/projects/:slug", path))
    this._ensure("plume-project-detail-page", lazyPages.projectDetail);
  else if (path === "/inbox") this._ensure("plume-inbox-page", lazyPages.inbox);
  else this._ensure("plume-dashboard-page", lazyPages.dashboard);
}
```

### 4. Render: spinner until ready, then the element

```ts
if (path === "/inbox") {
  return this._ready.has("plume-inbox-page")
    ? html`
      <style>
      ${APP_STYLES}
      </style>
      <plume-inbox-page></plume-inbox-page>
      <plume-command-palette></plume-command-palette>
    `
    : html`
      <style>
      ${APP_STYLES}
      </style>
      <div class="app-loader">
        <plume-spinner></plume-spinner>
      </div>
    `;
}
```

Keep `login` and `setup` branches **static** (no gate); they render immediately
on first paint.

## Constraints (don't break these)

1. **No blank screen on navigation**: every lazy route shows `<plume-spinner>`
   until the page element is defined, never an empty `<plume-app>`.
2. **Light DOM preserved**: `app-shell.ts` uses
   `createRenderRoot() { return this; }` for DnD compatibility. Lazy loading
   must not change this; only the _imports_ become dynamic, the render structure
   is unchanged.
3. **DnD must keep working**: kanban lives under `plume-project-detail-page`
   in light DOM. Splitting is a transport concern, not a DOM concern, but verify
   drag still works after adding/splitting a page.
4. **Signals/router unchanged**: `currentPath`, `matchRoute`, `navigate` stay
   as-is.
5. **No HMR/dev regression**: dynamic `import()` works in Vite dev; confirm
   `make dev-ui` still hot-reloads.

## Preloading (optional, low-risk, additive)

`import()` in the background without blocking render; the browser memoizes the
promise, so the later navigation is instant. Worst case is an unused chunk
fetch; safe to ship before profiling.

- **Default landing:** after `auth` resolves to authenticated, background-import
  the dashboard chunk (already in `app-shell.ts`' `willUpdate` via
  `lazyPages.dashboard()`).
- **Hover/focus preload:** on `mouseenter`/`focus` of a nav link,
  background-import that page's chunk:
  ```ts
  @mouseenter="${() => lazyPages.projectDetail()}"
  ```

## Adding a new lazy page: checklist

1. Create the page element (e.g. `features/<name>/<name>-page.ts`) with
   `@customElement("plume-<name>-page")`.
2. Add it to `lazyPages` in `app-shell.ts`.
3. Add a `willUpdate()` branch calling
   `_ensure("plume-<name>-page", lazyPages.<name>)`.
4. Add a `render()` branch: spinner-then-element (mount
   `<plume-command-palette>` alongside it on authenticated routes; matches
   existing pages).
5. Audit for stray static imports:
   ```bash
   grep -rn "import.*features/" ui/src --include="*.ts"
   ```
   Only `login-page` and `setup-page` should be static; any _other_ file
   statically importing a page module forces it back into the entry bundle.
6. Build + verify (below).

## Verify after building

```bash
make build-ui
ls -la ui/dist/assets/
```

Expected: one `index-*.js` entry (Lit + router + signals + shell + login +
setup) plus one chunk per lazy page (e.g. `dashboard-page-*.js`,
`project-detail-page-*.js`, `inbox-page-*.js`).

Behavior checks:

- [ ] `make dev-ui`: cold `/login` shows the form with **no spinner**.
- [ ] Navigate to the new route: brief spinner, then the page.
- [ ] Direct cold URL load of the new route: spinner, then page (router reads
      `window.location.pathname` on boot).
- [ ] Back/forward buttons work (`popstate` drives `currentPath`).
- [ ] Devtools Network tab: landing on `/login` does **not** fetch the new
      page's chunk.
- [ ] If the page uses DnD, drag still works (light-DOM regression risk).

## When to revisit manual chunking

Only if profiling shows a heavy dep (e.g. DnD) **duplicated** across multiple
page chunks, or a shared lib pulled into the entry that only one page uses. At
this scale Vite's automatic splitting is good enough.
