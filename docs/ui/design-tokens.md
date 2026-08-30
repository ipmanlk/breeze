# Design Tokens & Token Compliance

How to keep `ui/` components free of hardcoded values, per the design-tokens
rule in `../../ui/AGENTS.md`.

> All values use design tokens (`var(--token)`) from `ui/src/styles/index.css`;
> no hardcoded `rem`/`px`/`ms` or raw color literals. Every shadow-DOM component
> includes `*, *::before, *::after { box-sizing: border-box; }`.

Read this when styling any new component. For theme-aware _colors_ specifically,
see `theme-colors.md`.

---

## Token hierarchy

Tokens in `src/styles/index.css` are layered; reuse before you create:

| Layer      | Examples                                      | When to use                            |
| ---------- | --------------------------------------------- | -------------------------------------- |
| Primitives | `--space-*`, `--text-*`, `--radius-*`         | Spacing, type scale, corner radius     |
| Semantic   | `--control-h`, `--avatar-md`, `--shadow-*`    | Control heights, avatar sizes, shadows |
| Component  | `--switch-w`, `--command-w`, `--kanban-col-w` | Component-specific dimensions          |

**Rule:** prefer an existing token. Only add a component token when a component
needs a size that no primitive/semantic token matches.

## When to add a new token

Add a component token when a component has a fixed size that no existing token
covers. Example: the switch track is `18.4px` tall, which sits between
`--space-4` (16px) and `--space-5` (20px):

```css
/* index.css: Component section */
--switch-w: 2rem; /* 32px; track width  */
--switch-h: 1.15rem; /* 18.4px; track height */
```

```css
/* switch.ts */
.track {
  width: var(--switch-w);
  height: var(--switch-h);
}
```

Do **not** reach for `--space-8` "because it happens to equal 2rem"; a semantic
component token documents _why_ that size exists and lets you retheme it without
grepping every component.

## Accepted exceptions (no tokens exist)

These literals are fine because there is no token for them and they are
inherently non-themeable. They match established components (`dialog.ts`,
`input.ts`, `spinner.ts`):

| Literal                                        | Where it's OK                      | Precedent                           |
| ---------------------------------------------- | ---------------------------------- | ----------------------------------- |
| `1px solid <token>`                            | Hairline borders                   | `dialog.ts`, `input.ts`             |
| `1.5s` / `0.7s` in `animation:` / `@keyframes` | Animation durations                | `spinner.ts`, `view-detail-page.ts` |
| `transparent`                                  | Inset spacers (e.g. switch border) | none |

> Transitions must use the `--duration-fast` / `--duration-normal` tokens +
> `--ease-out`. Raw `200ms`/`150ms` in transitions is a violation (use the
> token). Only `@keyframes` _animation_ durations are exempt.

## Pixel-grid layouts (timeline / gantt): read the token in JS

Some layouts are inherently pixel-based (a gantt where each day is a fixed width
and bar positions are `offset * dayWidth`). Define the day width as a
**component token** in `index.css` (in `rem`, never `px`), then **read it at
runtime** so the JS math stays token-driven, with no hardcoded `rem` in code:

```css
/* index.css */
--tl-day-w: 1.75rem; /* 28px per day at day zoom */
--tl-week-w: 0.875rem; /* 14px per day at week zoom */
```

```ts
// timeline-view.ts: read the active token, compute positions in rem
protected firstUpdated() { this.#readDayWidth(); }
protected updated(changed: Map<string, unknown>) {
  if (changed.has("_zoom")) this.#readDayWidth();
}
#readDayWidth() {
  const token = this._zoom === "day" ? "--tl-day-w" : "--tl-week-w";
  this.#dayW = parseFloat(getComputedStyle(this).getPropertyValue(token)) || 1.75;
  this.requestUpdate();
}
// bar: style="left:${offset * this.#dayW}rem;width:${span * this.#dayW}rem"
```

Custom properties inherit through shadow boundaries, so
`getComputedStyle(this).getPropertyValue('--tl-day-w')` works inside a
shadow-DOM component. This keeps the grid fully token-driven without `28`/`14`
literals in JS. See `features/projects/timeline-view.ts`.

## Data-visualization colors

Priority/status bars on the timeline, priority dots on the kanban, and filter
swatches use `oklch(...)` literals with alpha (e.g.
`oklch(0.62 0.12 240 / 0.5)`). These are data-visualization colors (semantic,
not UI chrome) and have no token; this is the established pattern across
`kanban-board.ts`, `filter-bar.ts`, and `timeline-view.ts`. Prefer a token where
one exists (`--destructive` for the overdue/today line, `--primary` for the
default bar), and use `oklch` literals only for the priority palette.

## Fixed-contrast colors

A color that is intentionally **not** themeable (e.g. white text on a saturated
project/task color badge) may use a fixed value:

```css
.badge {
  color: #fff;
  background-color: var(--user-color);
}
```

This mirrors React's `text-white`. It is the same category as the
`rgba(0,0,0,…)` literals inside the `--shadow-*` token definitions: a deliberate
fixed contrast, not a themeable color. **Do not** use this as an excuse to
hardcode themeable colors (foreground/background/border/muted); those always
take tokens.

## Don't duplicate global styles

Some styling is already applied globally in `index.css` via a universal
selector. Re-declaring it per-component both hardcodes values and diverges:

```css
/* ❌ DON'T: index.css already does this via `*::-webkit-scrollbar` */
.list::-webkit-scrollbar {
  width: 6px;
}
.list::-webkit-scrollbar-thumb {
  background-color: var(--border);
  border-radius: 3px;
}

/* ✅ DO: just scroll; the global thin scrollbar applies */
.list {
  overflow-y: auto;
}
```

Check `index.css` for global rules (scrollbar, `box-sizing`, `sr-only`,
`prefers-reduced-motion`) before re-implementing them.

## Required in every shadow-DOM component

```ts
static styles = css`
  *, *::before, *::after { box-sizing: border-box; }
  :host { /* ... */ }
`;
```

Light-DOM components (DnD) are the only exception; see `lit-patterns.md`.

## Never edit `src/api/`

It's auto-generated (`make api-types`). The only API config outside it is
`src/main.ts` (client `baseUrl`/`credentials`).
