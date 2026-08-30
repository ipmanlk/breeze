# Sidebar (shadcn parity)

Hard-won lessons from porting the shadcn `Sidebar` to Lit (`breeze-app-layout`

- the `breeze-nav-*` components). Read this before touching the sidebar or
  adding a sidebar section/item. The canonical implementation is
  `src/layouts/app-layout.ts` + `src/components/nav/`.

## The shadcn spacing model

shadcn's sidebar does **not** pad the sidebar column. Each section carries its
own `p-2` (8px). Match this exactly or the section gaps look wrong:

| shadcn part            | classes (relevant)                                        | Our token equivalent                                                           |
| ---------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------------ |
| `SidebarHeader/Footer` | `flex flex-col gap-2 p-2`                                 | wrapper `padding: var(--space-2)`                                              |
| `SidebarContent`       | `flex-1 gap-0 overflow-auto`                              | `.nav-scroll { flex:1; gap:0; overflow-y:auto }`                               |
| `SidebarGroup`         | `flex flex-col p-2`                                       | each `breeze-nav-*` root: `padding: var(--space-2)`                            |
| `SidebarMenu`          | `flex flex-col gap-0`                                     | `.menu { gap: 0 }`; items are **flush**                                       |
| `SidebarMenuButton`    | `h-8 p-2 gap-2 rounded-md`                                | `a { height: var(--space-8); padding: 0 var(--space-2); gap: var(--space-2) }` |
| `SidebarMenuButton sm` | `h-7 text-xs p-2 gap-2`                                   | `height: var(--space-7); font-size: var(--text-xs)`                            |
| `SidebarGroupLabel`    | `h-8 px-2 text-xs font-medium text-sidebar-foreground/70` | see below                                                                      |

**Do NOT** put padding/gap on the sidebar's inner column. Putting it there makes
the border touch the content and the section gaps inconsistent.

### Group label style (gotcha)

shadcn `SidebarGroupLabel` is
`h-8 px-2 text-xs font-medium
text-sidebar-foreground/70`, **not** uppercase,
**not** `text-2xs`. An easy mistake is to copy a "section heading" instinct
(`uppercase tracking-wide
text-2xs`). The correct style:

```css
.group-label {
  display: flex;
  align-items: center;
  height: var(--space-8); /* h-8, 32px */
  padding: 0 var(--space-2); /* px-2 */
  font-size: var(--text-xs); /* text-xs, 12px; not text-2xs */
  font-weight: 500; /* font-medium */
  color: color-mix(in oklch, var(--sidebar-foreground) 70%, transparent);
  /* NO text-transform, NO letter-spacing */
}
```

### Item color (gotcha)

Menu items inherit `text-sidebar-foreground` in shadcn (the `Sidebar` sets
`text-sidebar-foreground`; the button sets no color). Do **not** use
`--muted-foreground` for item text; that's the gray that makes items look faded
vs. React (especially jarring in dark mode, where `sidebar-foreground` is
near-white but `muted-foreground` is gray). Use:

```css
a {
  color: var(--sidebar-foreground);
}
a:hover,
a.active {
  color: var(--sidebar-accent-foreground);
}
```

## Header width: a flex child shrinks to content

### The bug

Wrapping the workspace switcher in a header with `display: flex` makes the
switcher (a flex item) shrink to its content width instead of filling the
sidebar:

```css
/* ❌ switcher ends up content-width, not full sidebar width */
.sidebar-head {
  display: flex;
  align-items: stretch;
}
```

A flex item's main-size defaults to content (`flex: 0 1 auto`);
`align-items:
stretch` only stretches the **cross** axis (height), not width.

### The fix

Make the header a block container (the block child fills width naturally), and
let the switcher own its own trigger sizing:

```css
.sidebar-head {
  padding: var(--space-2);
} /* display: block (default) */
```

If you must keep `display: flex`, give the child `flex: 1` or `width: 100%`
(`width` on a flex item sets `flex-basis`, so `width: 100%` fills it).

## Collapsed dropdowns clip: use a tooltip instead

### The bug

The sidebar has `overflow-x: hidden` (to clip the collapsing width). An in-DOM
dropdown that opens **to the right** when collapsed extends past the 3rem
sidebar and gets clipped; it never shows:

```css
/* ❌ clipped by .sidebar { overflow-x: hidden } when collapsed */
.menu {
  left: calc(var(--sidebar-w-icon) + var(--space-2));
  width: var(--menu-w);
}
```

Radix `DropdownMenu` portals to `document.body` in React, so it escapes the
overflow. Our Lit menus render in-shadow-DOM (no portal), so they can't.

### The fix

When collapsed, don't render a right-opening dropdown. Show a hover tooltip with
the label instead; the useful info is the name, and the menu items are usually
secondary:

```ts
if (collapsed) {
  return html`
    <breeze-tooltip text="${name}" side="right">${logo}</breeze-tooltip>
  `;
}
return html`
  <button class="trigger" @click="${toggle}">…</button>${menu}
`;
```

If you genuinely need a working collapsed dropdown, you must portal the panel to
`document.body` with `position: fixed` (out of scope for the sidebar switcher,
whose items are placeholders).

## Don't put `overflow: hidden` on a dropdown's ancestor

Related to the above: a header that holds a dropdown must **not** have
`overflow: hidden`, or the panel (positioned `top: 100%`, opening downward) is
clipped. The workspace switcher header originally had `overflow: hidden` left
over from a static brand logo; it silently broke the dropdown. Remove it
whenever a container hosts a dropdown.

## Hover-action rows: don't nest `<button>` in `<a>`

### The bug

A row that is an `<a>` (for navigation) with a hover "more"/"unpin" `<button>`
inside it is **invalid HTML** (interactive content can't nest in `<a>`), and the
button's click also fires the anchor's navigation:

```html
<!-- ❌ invalid nesting + click navigates -->
<a class="row" @click="${navigate}">…<button class="action">⋯</button></a>
```

### The fix

Wrap both in a `position: relative` div. The `<a>` fills it; the action button
is an absolutely-positioned **sibling**, shown on `.item:hover`:

```html
<div class="item">
  <!-- position: relative -->
  <a class="row" @click="${navigate}">badge + name</a>
  <button class="action">⋯</button>
  <!-- position: absolute; right; top:50% -->
</div>
```

```css
.item {
  position: relative;
  display: flex;
  width: 100%;
}
.action {
  display: none;
  position: absolute;
  right: var(--space-1);
  top: 50%;
  transform: translateY(-50%);
}
.item:hover .action {
  display: inline-flex;
}
```

The button is no longer inside the anchor, so clicking it never navigates (no
`stopPropagation` hack needed), and the HTML is valid. This is how
`breeze-nav-projects` (more) and `breeze-nav-views` (unpin) are built.

## Collapsed items: icon-centered rows

shadcn collapsed buttons are `size-8` (32px square) with the icon centered. Our
rows stay full-width with `justify-content: center` and the label hidden; the
icon centers in the full width. This is consistent across `breeze-nav-list` /
`-projects` / `-views` and avoids per-component centering math. Wrap the whole
row in `<breeze-tooltip text side="right">` when collapsed so hovering reveals
the name.

## Fetching sidebar data once

The sidebar is re-instantiated per page (each page wraps its content in
`<breeze-app-layout>`), so fetching in `connectedCallback` would re-fetch on
every navigation. Instead, fetch sidebar data (projects, pinned views, unread
count) **once** in the persistent `breeze-app` shell when auth resolves, guarded
by a `#loaded` flag. The nav components only **read** the signals and re-render
on change. See `src/app-shell.ts`.
