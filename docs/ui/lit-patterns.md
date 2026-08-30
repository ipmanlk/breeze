# Lit Component Patterns & Pitfalls

Hard-won lessons from building dialogs, dropdowns, selects, comboboxes, and tabs
in the Lit v2 UI. **Read this before adding any new interactive UI component.**
Each pitfall below caused real bugs in production.

## Quick Reference: all rules at a glance

Before writing any Lit component, verify against this checklist. Each item links
to the full section with examples and bug stories.

| #  | Rule                                                                                                             | Section                                                                          |
| -- | ---------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| 1  | Use `static styles = css\`...\``; never inline`<style>`in`render()`                                              | [Styling §1](#rule-1-static-styles-for-shadow-dom-components)                    |
| 2  | All values use `var(--token)` from `index.css`; no hardcoded rem/px/ms                                          | [Styling §2](#rule-2-design-tokens-via-css-custom-properties)                    |
| 3  | Every shadow-DOM component includes `box-sizing: border-box` reset                                               | [Styling §3](#rule-3-box-sizing-border-box-in-every-shadow-root)                 |
| 4  | Dynamic values use `style="--var: ${val}"`; not `<style>` blocks                                                | [Styling §4](#rule-4-dynamic-values-via-inline-style-with-css-custom-properties) |
| 5  | Light DOM components (`createRenderRoot`) are the only `<style>` exception                                       | [Styling §5](#rule-5-light-dom-components--the-exception)                        |
| 6  | `?attr="${val}"` for boolean attributes; never `${val \|\| ""}`                                                 | [Boolean attrs](#boolean-attribute-bindings--the-1-pitfall)                      |
| 7  | `attribute: false` on all `type: Array` / `type: Object` properties                                              | [attribute: false](#attribute-false-for-arrays-and-objects)                      |
| 8  | `composedPath()` for outside-click; never `contains(e.target)`                                                  | [Dropdown panels](#dropdown-panels--outside-click-in-shadow-dom)                 |
| 9  | `changedProps.has(...)` guard in `updated()` / `willUpdate()`                                                    | [updated()](#updated--check-changedprops)                                        |
| 10 | Always-in-DOM for dialogs/toggles; never conditional render                                                     | [Always-in-DOM](#always-in-dom-pattern-for-toggled-components)                   |
| 11 | `bubbles: true, composed: true` on all custom events                                                             | [Data flow](#data-flow-properties-in-events-out)                                 |
| 12 | Clean up document listeners in `disconnectedCallback()`                                                          | [Dropdown panels](#dropdown-panels--outside-click-in-shadow-dom)                 |
| 13 | Use native `<dialog>` + `showModal()`; no custom overlays                                                       | [Dialogs](#dialogs--use-native-dialog)                                           |
| 14 | Tabs render only the tab list; content is a parent sibling                                                      | [Tabs](#tabs--separate-list-from-content)                                        |
| 15 | Arrow function class fields for document-level listeners (stable ref)                                            | [Dropdown panels](#dropdown-panels--outside-click-in-shadow-dom)                 |
| 16 | DnD with @atlaskit: container is the drop target, rows are draggables (mirror the kanban); never make a row both | [DnD](#drag-and-drop-dnd--when-to-use-which)                                     |
| 17 | Close single-select popovers on select; breeze-popover stays open                                               | [Single-select popovers](#single-select-popovers-must-close-on-select)           |
| 18 | Submit via `@click`; not `type="submit" form=`                                                                  | [breeze-button submit](#breeze-button-form-submission)                           |
| 19 | Never combine `private` with `#private` (TS18010)                                                                | [TS: private vs #private](#typescript-private-vs-private)                        |

**These rules are enforced in `../../ui/AGENTS.md` (rules 3–6). Violations will cause
bugs.**

## Table of Contents

- [Styling in Lit: the complete approach](#styling-in-lit--the-complete-approach)
- [Boolean attribute bindings: the #1 pitfall](#boolean-attribute-bindings--the-1-pitfall)
- [Box-sizing doesn't cross shadow boundaries](#box-sizing-doesnt-cross-shadow-boundaries)
- [Dialogs: use native `<dialog>`](#dialogs--use-native-dialog)
- [Dropdown panels: outside-click in shadow DOM](#dropdown-panels--outside-click-in-shadow-dom)
- [Select / Combobox: trigger styling](#select--combobox--trigger-styling)
- [Tabs: separate list from content](#tabs--separate-list-from-content)
- [Always-in-DOM pattern for toggled components](#always-in-dom-pattern-for-toggled-components)
- [Drag-and-drop (DnD): when to use which](#drag-and-drop-dnd--when-to-use-which)
- [Single-select popovers must close on select](#single-select-popovers-must-close-on-select)
- [breeze-button form submission](#breeze-button-form-submission)
- [TypeScript: `private` vs `#private`](#typescript-private-vs-private)
- [Data flow: properties in, events out](#data-flow-properties-in-events-out)
- [Form-associated custom elements](#form-associated-custom-elements)
- [SignalController: bridging Preact signals to Lit](#signalcontroller--bridging-preact-signals-to-lit)
- [`static styles`: not inline `<style>` tags](#static-styles--not-inline-style-tags)
- [`updated()`: check `changedProps`](#updated--check-changedprops)
- [`attribute: false` for arrays and objects](#attribute-false-for-arrays-and-objects)

---

## Styling in Lit: the complete approach

This section defines how ALL styling is done in the UI codebase. Read this first
The patterns below are enforced in AGENTS.md.

### Rule 1: `static styles` for shadow-DOM components

All static CSS lives in
`static styles = css\`...\``; never inline `<style>` tags in `render()` or
sub-template methods.

```ts
// ✅ CORRECT
static styles = css`
  .my-grid { display: grid; grid-template-columns: 1fr 1fr; }
`;

protected render() {
  return html`<div class="my-grid">...</div>`;
}
```

```ts
// ❌ WRONG: re-processed on every render
protected render() {
  return html`
    <style>.my-grid { display: grid; }</style>
    <div class="my-grid">...</div>
  `;
}
```

`static styles` is deduplicated, processed once at class definition time, and
optimized by Lit's styling system. Inline `<style>` tags re-process on every
render.

### Rule 2: Design tokens via CSS custom properties

All colors, spacing, sizing, shadows, transitions, and z-index values use tokens
from `src/styles/index.css`: **never** hardcoded `rem`/`px`/`ms` or raw color
literals.

CSS custom properties (CSS variables) **inherit through shadow DOM boundaries**.
A `--primary` defined on `:root` is available inside every shadow root. This is
how theming works:

```css
/* ✅ CORRECT: use design tokens */
.trigger {
  height: var(--control-h);
  border: 1px solid var(--input);
  background: var(--background);
  box-shadow: var(--shadow-md);
  transition: background var(--duration-fast) var(--ease-out);
}
```

```css
/* ❌ WRONG: hardcoded values */
.trigger {
  height: 2.25rem; /* use var(--control-h) */
  border: 1px solid #e2e8f0; /* use var(--input) */
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08); /* use var(--shadow-md) */
  transition: background 120ms ease; /* use tokens */
}
```

See `src/styles/index.css` for the full token reference (spacing scale, radius,
typography, motion, elevation, z-index layers, container widths).

### Rule 3: `box-sizing: border-box` in every shadow root

The global `* { box-sizing: border-box }` reset in `index.css` does **not**
cross shadow DOM boundaries. Every shadow-DOM component must include it:

```ts
static styles = css`
  *, *::before, *::after { box-sizing: border-box; }
  /* ...rest of styles... */
`;
```

### Rule 4: Dynamic values via inline `style` with CSS custom properties

For values that change at runtime (e.g., a column count, a dynamic color from
data), use the `style` attribute with CSS custom properties: never inline
`<style>` blocks:

```ts
// ✅ CORRECT: CSS custom property for dynamic value
html`
  <div style="--cols: ${this.columnCount}">...</div>
`;

// In static styles:
// .grid { grid-template-columns: repeat(var(--cols, 2), 1fr); }
```

```ts
// ❌ WRONG: inline <style> with dynamic value
html`
  <style>
  .grid { grid-template-columns: repeat(${this.columnCount}, 1fr); }
  </style>
`;
```

### Rule 5: Light DOM components: the exception

Components that override `createRenderRoot() { return this; }` use light DOM
(not shadow DOM). This is required for `@atlaskit/pragmatic-drag-and-drop`
compatibility (DnD relies on `event.target` not being retargeted).

Light DOM components **cannot** use `static styles` (no shadow root to attach
styles to). They use `<style>` tags in their `render()` templates instead. This
is the **only** exception to Rule 1:

```ts
// Light DOM component: <style> in render() is allowed here
createRenderRoot() { return this; }

protected render() {
  return html`
    <style>
      .kb-board { display: flex; gap: var(--space-4); }
    </style>
    <div class="kb-board">...</div>
  `;
}
```

All class names in light DOM components must be **prefixed** to avoid collisions
with global styles: `kb-` (kanban), `pdp-` (project-detail), `app-` (app-shell).

### Shadow DOM vs Light DOM: decision guide

| Use shadow DOM (default)      | Use light DOM (`createRenderRoot`) |
| ----------------------------- | ---------------------------------- |
| UI primitives (button, input) | DnD-interactive components         |
| Dialogs, dropdowns, selects   | Components using `event.target`    |
| Anything with internal markup | Components needing global CSS      |

Shadow DOM: encapsulated styles, retargeted events, `static styles`. Light DOM:
global styles, real `event.target`, `<style>` in templates.

### Summary checklist

- [ ] `static styles` used (not inline `<style>`): unless light DOM
- [ ] `box-sizing: border-box` included in shadow-DOM styles
- [ ] All values use `var(--token)` from `index.css`: no hardcoded values
- [ ] Dynamic values use inline `style="--var: ${value}"`: not `<style>` blocks
- [ ] Light DOM class names are prefixed (`kb-`, `pdp-`, etc.)

---

## Boolean attribute bindings: the #1 pitfall

**This bug appeared three separate times** during development (tabs, combobox
checkboxes, dropdown triggers). It is the most common Lit mistake.

### The bug

```ts
// ❌ WRONG: always sets the attribute
html`
  <div data-active="${this.isActive || ""}"></div>
`;
```

When `this.isActive` is `false`, the expression evaluates to `""` (empty
string). Lit renders `data-active=""`: the attribute IS present. CSS
`[data-active]` matches any value including empty string, so the selector always
applies.

This caused:

- All tabs showing as selected simultaneously
- All combobox checkboxes filled with primary color
- Dropdown triggers showing open-state styling when closed

### The fix

Use Lit's `?` boolean attribute binding. It adds the attribute when the value is
truthy and **removes** it when falsy:

```ts
// ✅ CORRECT: attribute only present when truthy
html`
  <div ?data-active="${this.isActive}"></div>
`;
```

### Rule

**Never** use `${expr || ""}` for data attributes that drive CSS. Always use
`?attr="${expr}"`.

---

## Box-sizing doesn't cross shadow boundaries

### The bug

`index.css` has a global reset:

```css
*,
*::before,
*::after {
  box-sizing: border-box;
}
```

But this `*` selector only applies to the DOM tree it's defined in. **Shadow DOM
roots are separate DOM trees**: the reset does NOT reach inside them.

This caused dropdown panels with `width: 100%` + `border: 1px` to actually be
`100% + 2px` wide, creating horizontal scrollbars inside dialogs.

### The fix

Add the reset to every shadow-DOM component's `static styles`:

```ts
static styles = css`
  *, *::before, *::after {
    box-sizing: border-box;
  }
  /* ...rest of styles... */
`;
```

### Rule

**Every** shadow-DOM component must include `box-sizing: border-box` in its own
styles. Never assume global CSS reaches into shadow roots.

---

## Dialogs: use native `<dialog>`

### Why native `<dialog>`

The HTML `<dialog>` element with `showModal()` provides:

- **Top-layer rendering**: always above page content, no z-index management
- **`::backdrop`** pseudo-element for the overlay scrim
- **Built-in focus trap**: Tab key cycles within the dialog
- **Escape-to-close**: browser handles it natively
- **`close` event**: fires on Escape or programmatic close

No custom overlay, no z-index, no focus-trap JS, no Escape listener.

### Pattern

```ts
@customElement("breeze-dialog")
export class BreezeDialog extends LitElement {
  static styles = css`
    :host {
      display: contents;
    }
    dialog {
      width: 36rem;
      max-width: calc(100vw - var(--space-8));
      max-height: 85vh;
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      box-shadow: var(--shadow-lg);
      padding: 0;
    }
    dialog::backdrop {
      background: color-mix(in oklch, black 50%, transparent);
    }
    /* Only flex when open: UA sets display:none when closed */
    dialog[open] {
      display: flex;
      flex-direction: column;
    }
    .header {
      /* ... */
    }
    .body {
      padding: var(--space-4) var(--space-5);
    }
    .footer {
      /* ... */
    }
  `;

  @property({ type: Boolean })
  open = false;
  @property()
  heading = "";

  @query("dialog")
  private _dialog!: HTMLDialogElement;

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("open") && this._dialog) {
      if (this.open && !this._dialog.open) this._dialog.showModal();
      else if (!this.open && this._dialog.open) this._dialog.close();
    }
  }
}
```

### Pitfalls

1. **Don't set `display: flex` on the base `dialog` rule.** The UA stylesheet
   sets `display: none` on `dialog:not([open])`. Author styles override UA
   styles regardless of specificity, so `dialog { display: flex }` makes the
   dialog always visible. Only set display on `dialog[open]`.

2. **Guard `showModal()` / `close()`.** `showModal()` throws if the dialog is
   already open. `close()` is safe to call when closed (no-op). Always check
   `this._dialog.open` before calling `showModal()`.

3. **Sync `open` on native close.** When the user presses Escape, the `<dialog>`
   fires a `close` event. Handle it to set `this.open = false` and dispatch your
   own `close` event:

   ```ts
   private _onClose() {
     if (this.open) {
       this.open = false;
       this.dispatchEvent(new CustomEvent("close", { bubbles: true, composed: true }));
     }
   }
   ```

4. **Backdrop click = click on the dialog itself.** When the user clicks the
   backdrop, `event.target` is the `<dialog>` element (not a child):

   ```ts
   private _onDialogClick(e: MouseEvent) {
     if (e.target === this._dialog) this._dialog.close();
   }
   ```

5. **No `overflow: hidden` on the dialog.** It clips absolutely-positioned
   dropdown panels (selects, comboboxes) rendered inside the dialog body.
   Instead, apply `border-radius` to header/footer individually for corner
   clipping.

### Usage from a feature component

```ts
html`
  <breeze-dialog
    .open="${this._showDialog}"
    heading="Create task"
    @close="${() => this._showDialog = false}"
  >
    <!-- default slot = body -->
    <div class="my-body">...</div>

    <!-- named slot = footer -->
    <div class="my-footer" slot="footer">
      <span class="spacer" style="flex:1"></span>
      <breeze-button variant="ghost" @click="${() => this._showDialog = false}">
        Cancel
      </breeze-button>
      <breeze-button @click="${this._submit}">Create</breeze-button>
    </div>
  </breeze-dialog>
`;
```

---

## Dropdown panels: outside-click in shadow DOM

### The bug

```ts
// ❌ WRONG: doesn't work with shadow DOM
private _onOutsideClick = (e: MouseEvent) => {
  if (!this.contains(e.target as Node)) this._open = false;
};
```

Events from shadow DOM are retargeted: `e.target` is the host element, not the
inner button. `this.contains(this)` returns `false` (a node doesn't contain
itself), so clicks inside the component incorrectly close the panel.

### The fix

Use `composedPath()` which returns the full event path through shadow
boundaries:

```ts
// ✅ CORRECT: works with shadow DOM
private _onOutsideClick = (e: MouseEvent) => {
  if (!e.composedPath().includes(this)) this._open = false;
};
```

### Full pattern for a dropdown

```ts
@state() private _open = false;

private _onOutsideClick = (e: MouseEvent) => {
  if (!e.composedPath().includes(this)) this._open = false;
};
private _onKeydown = (e: KeyboardEvent) => {
  if (e.key === "Escape") this._open = false;
};

protected updated(changedProps: Map<string, unknown>) {
  // Guard with changedProps: see "updated(): check changedProps" section
  if (!changedProps.has("_open")) return;
  if (this._open) {
    document.addEventListener("click", this._onOutsideClick);
    document.addEventListener("keydown", this._onKeydown);
  } else {
    document.removeEventListener("click", this._onOutsideClick);
    document.removeEventListener("keydown", this._onKeydown);
  }
}

disconnectedCallback() {
  super.disconnectedCallback();
  // Always clean up: component might be removed while panel is open
  document.removeEventListener("click", this._onOutsideClick);
  document.removeEventListener("keydown", this._onKeydown);
}
```

### Rule

**Always** clean up document-level listeners in `disconnectedCallback()`. The
component can be removed from the DOM while the panel is open.

---

## Select / Combobox: trigger styling

### Keep triggers simple

Match the outline button style: don't add hover rings or focus shadows:

```css
.trigger {
  height: var(--control-h);
  border: 1px solid var(--input);
  border-radius: var(--radius-md);
  background: var(--background);
  transition: background var(--duration-fast) var(--ease-out);
}
.trigger:hover {
  background: var(--accent);
}
/* No box-shadow ring, no border-color change on hover/open */
```

### Combobox trigger height

Use a **fixed** `height: var(--control-h)`, never `min-height`. Wrapping chips
make the trigger taller than adjacent selects, breaking grid alignment. Show
selected items as **overlapping avatars** instead:

```css
.trigger {
  height: var(--control-h); /* fixed, not min-height */
  overflow: hidden; /* clip extra avatars */
}
.avatars breeze-avatar {
  margin-left: calc(var(--space-1) * -1); /* overlap */
  border: 2px solid var(--background); /* separator */
}
.avatars breeze-avatar:first-child {
  margin-left: 0;
}
```

### Panel width

```css
.panel {
  position: absolute;
  top: calc(100% + var(--space-1));
  left: 0;
  width: 100%; /* match trigger width */
  max-width: 100%; /* never exceed trigger */
  overflow: hidden; /* clip long content */
  box-sizing: border-box; /* include border in width */
}
```

**Never** use `min-width` larger than the trigger: it causes horizontal scroll
inside dialogs.

---

## Tabs: separate list from content

### The bug

Putting tab content in a `<slot>` inside the tabs component creates nested
flex/scroll contexts and re-render timing issues. The tab list and content panel
fight over layout.

### The fix

**`breeze-tabs` renders ONLY the tab list** (triggers). Content is a sibling in
the parent, not slotted into the tabs component. This matches shadcn's pattern
where `TabsList` and `TabsContent` are separate siblings:

```ts
// breeze-tabs: tab list ONLY, no content slot
render() {
  return html`
    ${this.tabs.map(t => html`
      <button
        class="tab"
        ?data-active="${this.value === t.id}"
        @click="${() => this._select(t.id)}"
      >${t.label}</button>
    `)}
  `;
}
```

```ts
// Parent: tabs + content as siblings
html`
  <breeze-tabs
    .tabs="${tabs}"
    .value="${this._tab}"
    @change="${(e: CustomEvent) => this._tab = e.detail}"
  ></breeze-tabs>

  <div class="tab-content">
    ${this._tab === "properties" ? this._renderProperties() : nothing}
  </div>
`;
```

### Underline indicator

Use a `::after` pseudo-element with opacity transition (matches shadcn):

```css
.tab {
  position: relative;
  color: var(--muted-foreground);
}
.tab::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px; /* cover the host's border-bottom */
  height: 2px;
  background: var(--primary);
  opacity: 0;
  transition: opacity var(--duration-fast) var(--ease-out);
}
.tab[data-active] {
  color: var(--foreground);
}
.tab[data-active]::after {
  opacity: 1;
}
```

---

## Always-in-DOM pattern for toggled components

### The bug

Conditionally rendering a dialog/dropdown with `${show ? html\`<breeze-dialog>\`
: ""}` creates and destroys the element on every toggle. This causes:

- Property binding timing races (`.open=true` may not be processed before first
  render)
- The dialog only shows on hard refresh after first open/close cycle
- Lost internal state on close/reopen

### The fix

**Always render the component.** Toggle visibility via a property:

```ts
// ✅ CORRECT: always in DOM, .open controls visibility
html`
  <breeze-dialog .open="${this._show}">...</breeze-dialog>
`;

// ❌ WRONG: conditional rendering
html`
  ${this._show
    ? html`
      <breeze-dialog>...</breeze-dialog>
    `
    : nothing}
`;
```

For `breeze-dialog`, the native `<dialog>` element is always in the DOM.
`showModal()` / `close()` control visibility: no create/destroy cycle.

### Rule

**Never** conditionally render interactive containers (dialogs, popovers,
dropdowns). Always keep them in the DOM and use a property to toggle state.

---

---

## Drag-and-drop (DnD): when to use which

Two DnD engines coexist in the UI:

### @atlaskit/pragmatic-drag-and-drop: cross-container DnD (the board)

Used by the kanban board (`features/projects/kanban-board.ts`): cards drag
between columns with auto-scroll, a drop indicator, and lexorank positioning.
@atlaskit reads `event.target` and walks `closest()` to resolve draggables /
drop targets, so shadow DOM **retargets** `event.target` at each boundary and
breaks the registry lookup. **Every ancestor up to the document must be light
DOM**: not just the DnD component. `breeze-project-detail-page` is light DOM
(`createRenderRoot`) precisely so the kanban's DnD chain is unbroken.

```
document ─┬─ breeze-project-detail-page (light DOM: REQUIRED for DnD chain)
          └─ breeze-kanban-board / -column / -card (light DOM: @atlaskit)
```

Reorder within a column uses the dropTarget's `onDrop` + `attachClosestEdge` /
`extractClosestEdge` from
`@atlaskit/pragmatic-drag-and-drop-hitbox/closest-edge`.

### @atlaskit for same-list reorder too (status settings): container is the drop target

A single-list reorder (project statuses) also uses @atlaskit, **not** native
HTML5 DnD: native DnD doesn't fire on touch, and @atlaskit gives touch + a
polished lift/indicator for free. The trap that made an earlier attempt fail
silently (no placement highlight, no reorder): making **each row both a
draggable AND a drop target**. Don't. Mirror the working kanban instead: **the
list container is the single drop target; each row is only a draggable.** The
container computes the drop as a gap between rows (top/bottom of each row's
midpoint) and draws one indicator line: exactly how the kanban column computes
gaps between its cards.

```
breeze-status-settings (light DOM)
  └ .ss-rows  ← dropTargetForElements (single drop target)
      ├ breeze-status-row × N  ← draggable each (data-status-id)
      └ .ss-indicator  ← absolute line, shown during drag
```

- Row
  `draggable({ element, getInitialData: () => ({ statusId }), onDragStart:
  set data-dragging (dim), onDrop: clear })`.
- Container
  `dropTargetForElements({ canDrop: source.data.statusId != null,
  onDragEnter/onDrag: position the indicator via computeGap/computeGapY over
  [data-status-id] elements (excluding the source), onDrop: build the new order
  and persist })`.
- On drop: `newOrder = others.slice(0, gap) + [source] + others.slice(gap)`;
  apply optimistically (`setProjectStatuses`), PUT the statuses whose `position`
  changed, then `refreshStatuses` (awaited: see the stale-refetch note below).

The drop handler reads `this.statuses` directly (the live property, always
current at drop time), so the drop target only needs re-wiring when `projectId`
changes: `statuses` changes don't require it.

**Why the per-row-both approach failed:** with each row also a drop target, the
source row's `canDrop` returns false for itself and the registry/`onDragEnter`
never reliably fired in this app: no highlight, no drop. One container drop
target + N draggables is the proven shape (it's the kanban), and it works on
touch.

**Touch / auto-scroll:** @atlaskit touch drag works out of the box. For a list
longer than the viewport, add
`autoScrollForElements({ element: <scroll
ancestor> })` to the drop target's
`combine(...)`; the short status list doesn't need it.

---

## Single-select popovers must close on select

`breeze-popover` is multi-select-friendly: it only closes on **outside** click /
Escape, so it stays open after an in-panel click. That's right for filter bars,
but wrong for a single-select (status / priority pickers in a table): Radix
closes on select, and users expect the dropdown to dismiss once they choose.

Close it explicitly from the option's click handler:

```ts
private _closeSelect(e: Event) {
  const pop = (e.target as HTMLElement | null)?.closest("breeze-popover") as
    | ({ open: boolean })
    | null;
  if (pop) pop.open = false;
}
// @click=${(e) => { e.stopPropagation(); this._change(...); this._closeSelect(e); }}
```

`closest("breeze-popover")` works because the option is slotted into the
popover's content slot and stays in its light-DOM subtree.

---

## breeze-button form submission

`breeze-button` is form-associated (`ElementInternals`), but the established,
reliable pattern (see `task-dialog.ts`) is to call the submit method directly
from `@click`: **not** `type="submit" form="<id>"`. The footer button lives in
the dialog's footer slot, a sibling of the form (not a descendant), so form
association is fragile. Direct `@click` always works:

```ts
html`
  <breeze-dialog .open="${open}" heading="...">
    <form @submit="${this._submit}" id="x-form">...fields...</form>
    <div slot="footer">
      <breeze-button variant="ghost" @click="${() => (open =
        false)}">Cancel</breeze-button>
      <breeze-button ?disabled="${busy}" @click="${this._submit}"
      >Save</breeze-button>
    </div>
  </breeze-dialog>
`;
```

`@submit` on the form still gives you Enter-to-submit in text fields; the
button's `@click` covers the click path. `_submit` calls `e.preventDefault()`.

---

## TypeScript: `private` vs `#private`

You **cannot** combine the `private` keyword with a `#`-prefixed private
identifier: TS error 18010:

```ts
// ❌ TS18010: An accessibility modifier cannot be used with a private identifier
private async #handleAdd() { ... }

// ✅ use one or the other
async #handleAdd() { ... }      // truly private (not accessible externally)
private _handleAdd() { ... }     // conventional private (TS-only)
```

In this codebase we use `#name` for internal fields/methods (no `private`
keyword) and `_name` for things tests/external code may legitimately touch.
Never write `private #foo`.

---

## Data flow: properties in, events out

Lit components should follow unidirectional data flow:

```
Parent → [properties] → Component → [events] → Parent
```

### Properties (input)

```ts
@property({ type: Boolean })
open = false;
@property({ type: Object, attribute: false })
project: DtoProjectResponse | null = null;
@property({ type: Array, attribute: false })
statuses: DtoTaskStatusResponse[] = [];
```

> **Note:** Array and Object properties must include `attribute: false`: see
> the [`attribute: false` section](#attribute-false-for-arrays-and-objects).

### Events (output)

```ts
this.dispatchEvent(
  new CustomEvent("change", {
    detail: value,
    bubbles: true,
    composed: true, // cross shadow boundary
  }),
);
```

### Parent wiring

```ts
html`
  <breeze-task-dialog
    .open="${this._showCreate}"
    .project="${project}"
    .statuses="${statuses}"
    @close="${() => this._showCreate = false}"
    @created="${(e: CustomEvent) => this._onTaskCreated(e.detail)}"
  ></breeze-task-dialog>
`;
```

### Rules

1. **`bubbles: true, composed: true`** on all custom events: they need to cross
   shadow DOM boundaries to reach the parent.
2. **Never** let a child component mutate parent state directly. Dispatch an
   event and let the parent decide.
3. Use `.property` (property binding) for objects/arrays, `?attr` for booleans,
   and `@event` for listeners.

---

## Form-associated custom elements

`breeze-button` and `breeze-input` use `formAssociated = true` +
`ElementInternals` for native form participation.

```ts
static formAssociated = true;

#internals: ElementInternals;

constructor() {
  super();
  this.#internals = this.attachInternals();
}

// Submit the associated form
this.#internals.form?.requestSubmit();

// Participate in FormData
this.#internals.setFormValue(this.value);
```

### Inside shadow DOM

Form-associated custom elements work within shadow roots
`ElementInternals.form` finds the nearest ancestor `<form>` in the same shadow
tree. A `<form>` and `<breeze-button type="submit">` in the same shadow root
will work.

### `type` default

`breeze-button` defaults to `type="submit"`. For non-submit buttons (Cancel,
toggle, etc.), there's no `type` attribute needed: the parent uses `@click`
handlers. But if the button is inside a `<form>`, set `type="button"` to prevent
accidental form submission.

---

## SignalController: bridging Preact signals to Lit

Page-level components (dashboard, project detail, projects list) need to react
to global state stored in `@preact/signals-core` signals. Lit doesn't know about
signals natively, so we use a `ReactiveController` bridge.

### Pattern

```ts
import { SignalController } from "@/lib/signal-controller";

class MyPage extends LitElement {
  #signals = new SignalController(this);

  connectedCallback() {
    super.connectedCallback();
    this.#signals.watch(currentPath, projectDetail);
  }
}
```

When any watched signal changes, the controller calls `host.requestUpdate()`,
triggering a re-render. Effects are created on `hostConnected` and disposed on
`hostDisconnected`: no manual `effect()` / `dispose()` bookkeeping.

### Why a controller (not manual `effect()`)

Manual signal wiring requires three pieces of boilerplate per component:

```ts
// ❌ WRONG: manual boilerplate, prone to leaks
let dispose: (() => void) | undefined;

connectedCallback() {
  super.connectedCallback();
  dispose = effect(() => {
    currentPath.value;       // touch to register dependency
    projectDetail.value;
    this.requestUpdate();
  });
}

disconnectedCallback() {
  super.disconnectedCallback();
  dispose?.();   // easy to forget → memory leak
}
```

The `SignalController` eliminates all of this. One line to create, one line to
watch. Auto-starts on connect, auto-disposes on disconnect.

### Rules

1. **Always use `SignalController`** for signal → Lit bridging. Never write
   manual `effect()` + `requestUpdate()` + `dispose()` in components.
2. Call `watch()` in `connectedCallback()` (after `super.connectedCallback()`).
3. The controller is a `ReactiveController`: it's automatically cleaned up when
   the host disconnects. No manual disposal needed.

---

## `static styles`: not inline `<style>` tags

### The bug

Putting `<style>` tags inside `render()` templates:

```ts
// ❌ WRONG: re-processed on every render
protected render() {
  return html`
    <style>
      .my-grid { display: grid; grid-template-columns: 1fr 1fr; }
    </style>
    <div class="my-grid">...</div>
  `;
}
```

Lit's `static styles` are deduplicated, processed once at class definition time,
and optimized by the styling system. Inline `<style>` tags in templates are
re-processed on **every render**, which is wasteful and goes against Lit's
design.

### The fix

Move all static CSS to `static styles`:

```ts
// ✅ CORRECT: processed once, deduplicated
static styles = css`
  .my-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
`;

protected render() {
  return html`<div class="my-grid">...</div>`;
}
```

For sub-templates (methods that return `html` fragments), don't add `<style>`
tags: the styles are already in `static styles` and apply to the entire shadow
root regardless of which template method renders the element.

### When to use inline styles

Only for **truly dynamic** styles that depend on runtime values:

```ts
html`
  <div style="--col-count: ${this.columnCount}">...</div>
`;
```

Use CSS custom properties for dynamic values: never inline `<style>` blocks.

### Rule

**Never** put `<style>` tags in `render()` or sub-template methods. All static
CSS goes in `static styles = css\`...\``.

---

## `updated()`: check `changedProps`

### The bug

```ts
// ❌ WRONG: runs on EVERY update, not just when _open changes
protected updated() {
  if (this._open) {
    document.addEventListener("click", this._onOutsideClick);
  } else {
    document.removeEventListener("click", this._onOutsideClick);
  }
}
```

`updated()` fires after **every** property change that triggers a re-render.
Without checking `changedProps`, this code runs when `value` changes, when
`options` changes, when any `@state` changes: not just when `_open` toggles.
This causes unnecessary `addEventListener` / `removeEventListener` calls on
every update (safe because duplicate adds are no-ops, but wasteful).

### The fix

Always accept the `changedProps` parameter and guard with `has()`:

```ts
// ✅ CORRECT: only runs when _open actually changes
protected updated(changedProps: Map<string, unknown>) {
  if (!changedProps.has("_open")) return;
  if (this._open) {
    document.addEventListener("click", this._onOutsideClick);
    document.addEventListener("keydown", this._onKeydown);
  } else {
    document.removeEventListener("click", this._onOutsideClick);
    document.removeEventListener("keydown", this._onKeydown);
  }
}
```

### `willUpdate` vs `updated`

| Lifecycle    | When it runs      | Use for                         |
| ------------ | ----------------- | ------------------------------- |
| `willUpdate` | Before DOM update | Computing state from properties |
| `updated`    | After DOM update  | Side effects (DOM, listeners)   |

- Use `willUpdate(changedProps)` when you need to derive state **before**
  rendering (e.g., resetting form fields when `open` transitions to true).
- Use `updated(changedProps)` for side effects that need the DOM to exist (e.g.,
  calling `showModal()` on a `<dialog>`, adding document listeners).

Both receive `changedProps: Map<string, unknown>`: always check
`changedProps.has("propName")` to avoid unnecessary work.

### Rule

**Always** accept `changedProps` in `updated()` and `willUpdate()`, and guard
side effects with `changedProps.has(...)`.

---

## `attribute: false` for arrays and objects

### The bug

```ts
// ❌ WRONG: Lit observes the attribute, but arrays can't be set via attributes
@property({ type: Array })
options: SelectOption[] = [];
```

By default, `@property()` observes the corresponding HTML attribute. For arrays
and objects, this means:

1. Lit registers an attribute observer on `options`
2. If someone writes `<breeze-select options="...">`, Lit tries to parse it
3. The parsed value is likely garbage (not a real array)

While this doesn't cause bugs when using property binding (`.options="${...}"`),
it's wasteful and misleading: the attribute observer fires on every attribute
change but can never produce a meaningful value.

### The fix

Set `attribute: false` for all array and object properties:

```ts
// ✅ CORRECT: no attribute observation, property-only binding
@property({ type: Array, attribute: false })
options: SelectOption[] = [];

@property({ type: Object, attribute: false })
project: DtoProjectResponse | null = null;
```

This tells Lit: "this property is only set via JavaScript, don't observe or
reflect any HTML attribute."

### Rule

**Every** `@property` with `type: Array` or `type: Object` must include
`attribute: false`. These types are non-serializable and should only be set via
property binding (`.prop="${value}"`).
