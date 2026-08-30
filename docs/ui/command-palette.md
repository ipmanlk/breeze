# Command Palette (`breeze-command-palette`)

Global search/command dialog for Breeze UI. The canonical implementation
lives in `ui/src/components/search/command-palette.ts` (a Lit element). This
doc covers how it is wired, its anatomy, and how to extend or restyle it.

---

## Location

| File                                       | Purpose                                                   |
| ------------------------------------------ | --------------------------------------------------------- |
| `src/components/search/command-palette.ts` | The `breeze-command-palette` element                      |
| `src/components/ui/dialog.ts`              | `breeze-dialog`: provides `placement="top"` used here    |
| `src/components/nav/nav-config.ts`         | `primaryNav` / `secondaryNav`: the page items            |
| `src/components/top-bar.ts`                | Search trigger button (dispatches `open-command-palette`) |
| `src/styles/index.css`                     | `--command-*` design tokens                               |

## Wiring

The palette is a drop-in element with **no props**. It is mounted once per
authenticated layout in `src/app-shell.ts`:

```ts
import "./components/search/command-palette.ts";
// …inside the authenticated shell render():
<breeze-command-palette></breeze-command-palette>;
```

It listens on `document` for opening events and keyboard shortcuts, so a single
instance serves the whole app.

## Opening the palette

Three ways, mirroring the React version:

1. **Keyboard**: `Cmd+K` (macOS) / `Ctrl+K` (others) toggles open/close.
   `Escape` closes. Arrow keys navigate; `Enter` selects.
2. **Event**: dispatch `open-command-palette` from anywhere:
   ```ts
   document.dispatchEvent(
     new CustomEvent("open-command-palette", { bubbles: true, composed: true }),
   );
   ```
3. **Top-bar search button**: `breeze-top-bar`'s search trigger dispatches the
   event above on click.

## Component API

The element is intentionally zero-config; all state is internal (`@state`).
There is currently no `@property` surface beyond `open` (used internally).

| Field                         | Kind                 | Notes                                                                                  |
| ----------------------------- | -------------------- | -------------------------------------------------------------------------------------- |
| `open`                        | `@property(Boolean)` | Reflects dialog visibility. Prefer the event/keyboard APIs over setting this directly. |
| `filters-change`-style events | none                 | Selection calls `navigate(item.url)` directly and closes.                        |

If you need to drive it externally, prefer dispatching `open-command-palette`
rather than toggling `open` (the element resets query/results/selection in
`_open()`).

## Anatomy (CSS class structure)

Rendered DOM (inside `breeze-dialog`):

```
breeze-dialog[placement=top]           # top-anchored, 42rem, rounded-xl, p-0
└─ .command                            # p-1, rounded-xl, bg-popover, overflow-hidden
   ├─ .command-input-wrapper           # p-1 pb-0
   │  └─ .command-input-group          # h-8, bg-input/30, border-input/30, rounded-lg
   │     ├─ breeze-icon.search-icon    # 16px, opacity 0.5, left
   │     └─ input.command-input        # text-sm, transparent
   └─ .command-list                    # max-h-72, hidden scrollbar
      ├─ .command-empty                # shown when no items
      ├─ .command-group
      │  ├─ .command-group-heading     # text-xs font-medium muted-foreground
      │  └─ button.command-item         # px-2 py-1.5 gap-2; .selected -> bg-muted
      │     ├─ breeze-icon.nav-icon | div.badge
      │     ├─ .item-label | .task-content (label + subtitle)
      │     └─ breeze-icon.check        # ml-auto, shown when .selected
      └─ …
```

### Item variants

| Type         | Leading element                                               | Content                                  |
| ------------ | ------------------------------------------------------------- | ---------------------------------------- |
| `page` (nav) | `breeze-icon.nav-icon` (16px, muted → foreground when active) | `.item-label`                            |
| `project`    | `.badge` (16px, color bg, first letter of label)              | `.item-label`                            |
| `task`       | `.badge` (16px, color bg, first letter of subtitle)           | `.task-content` (label + muted subtitle) |

All variants end with `.check` (16px), visible only on the active item.

## Design tokens

Defined in `src/styles/index.css`. Override any of these (e.g. per-theme) to
resize the palette without touching component code:

| Token              | Default | Meaning                                        |
| ------------------ | ------- | ---------------------------------------------- |
| `--command-w`      | `42rem` | Dialog width (`max-w-2xl`)                     |
| `--command-list-h` | `18rem` | Results list max height (`max-h-72`)           |
| `--command-top`    | `20vh`  | Top offset for `placement="top"` (`top-[20%]`) |

Dialog tokens consumed (from `breeze-dialog`, overridable via inline `style`):
`--dialog-w`, `--dialog-radius`, `--dialog-body-padding`.

The palette sets these inline on `<breeze-dialog>`:

```
--dialog-w: var(--command-w);
--dialog-radius: var(--radius-xl);
--dialog-body-padding: 0;
```

## Data sources

- **Page items**: from `primaryNav` + `secondaryNav` (`nav-config.ts`). The
  element filters them client-side against the current query (substring match on
  `title`). De-duplicated by `url`.
- **Project / task items**: from the generated SDK `getSearch` API (`@/api`),
  debounced 250ms. Only fetched when the dialog is open and the trimmed query is
  empty **or** ≥ 2 chars (matches the React hook). Types requested:
  `project,task,channel,direct_message,member`.

```ts
const { data } = await getSearch({
  query: {
    q: effectiveQuery,
    types: "project,task,channel,direct_message,member",
    limit: 10,
  },
  throwOnError: true,
});
```

Results are mapped to `PaletteItem` (`type` coerced to `"project"` | `"task"`).
`NAV_ICON_MAP` looks up a nav icon when `result.url` matches a known nav route.

## Extending

### Add a new item type / group

The grouping happens in `render()` via three filtered arrays (`navItems`,
`projectItems`, `taskItems`) and three render helpers (`_renderNavItem`,
`_renderProjectItem`, `_renderTaskItem`).

To add, e.g., a **Chats** group:

1. Extend the `PaletteItem["type"]` union:
   `"page" | "project" | "task" | "chat"`.
2. In `_getItems()`, map search results of that `type` into items (instead of
   coercing everything non-project to `task`):
   ```ts
   const type: PaletteItem["type"] = r.type === "project"
     ? "project"
     : r.type === "task"
     ? "task"
     : r.type === "channel"
     ? "chat"
     : "task";
   ```
3. Add `chatItems = items.filter(i => i.type === "chat")` in `render()`.
4. Add a `_renderChatItem()` helper and a `.command-group` block (copy the
   existing group block). Mind the running `selectedIndex` offset; the active
   index is global across **all** groups, so pass the cumulative offset as the
   starting index (see how `taskItems` adds
   `navItems.length + projectItems.length + i`).
5. Update the empty check (`isEmpty` = all four arrays empty).

> The global `_selectedIndex` spans every item in order. When inserting a group,
> keep the render order and the offset arithmetic in sync, or arrow navigation
> will select the wrong row.

### Add static command actions (not from search)

cmdk-style "run a command" items (e.g. "New project", "Toggle theme") don't come
from `getSearch`. Add a static list and prepend it as its own group:

```ts
const COMMANDS: { id: string; label: string; icon: string; run: () => void }[] =
  [
    {
      id: "new-project",
      label: "New project",
      icon: "plus",
      run: () => navigate("/projects/new"),
    },
  ];
```

Render them first (before Navigation), and in `_select()` branch on type:

```ts
private _select(item: PaletteItem) {
  if (item.type === "action") { item.run?.(); }
  else { navigate(item.url); }
  this._close();
}
```

Add `"action"` to the `type` union and a `run?` field on `PaletteItem`.

### Change keyboard behavior

All keys are handled in `_onKeydown`. To add, e.g., `Tab` cycling or vim-style
`j`/`k`, extend that method. Always call `this._scrollActiveIntoView()` after
changing `_selectedIndex` so the active row stays visible (the list has a hidden
scrollbar).

### Restyle

All styling lives in
`static styles = css\`...\``(rule 15: scoped styles,
design tokens only, no hardcoded`rem`/`px`/color literals). To retheme, prefer
overriding the tokens above over editing component CSS. The one fixed literal
is`color:
#fff`on`.badge`(white text on saturated project/task colors is
intentionally not themeable (white text on saturated project/task colors, the
same idea as Tailwind's `text-white`).

## Reusing top placement for other dialogs

`breeze-dialog` now supports `placement="top"` (anchored at `var(--command-top)`
from the top, horizontally centered). Any dialog that should sit near the top
(search-like, quick-action) can use it:

```html
<breeze-dialog placement="top" .open="${open}" @close="${…}">
  …
</breeze-dialog>
```

Combine with `--dialog-w`, `--dialog-radius`, `--dialog-body-padding` inline
overrides to control size/corners/padding.

## Build & verify

```bash
cd ui && deno check src/main.ts   # type-check (strict)
cd ui && deno task build          # tsc && vite build
cd ui && deno fmt src/            # format (never format src/api/)

Or via the repo Makefile: `make build-ui`.

## Reference (Lit implementation)

The canonical implementation lives in `ui/src/components/search/command-palette.ts`
(a Lit element). It uses `breeze-dialog` for the overlay, `nav-config.ts` for the
page items, and the generated SDK search API for search results.
```
