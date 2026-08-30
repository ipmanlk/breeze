# Theme-Aware Colors in Shadow DOM

How to make a Lit component's colors respond to light/dark mode. Read this
whenever a component needs a `dark:` variant of any shadcn class.

For the _token_ rules (which tokens to use) see `design-tokens.md`. This doc is
about the _mechanism_ of switching them by theme.

---

## The problem

shadcn/Tailwind uses `dark:` variants
(`bg-foreground dark:bg-primary-foreground`). In Lit shadow DOM the theme lives
on `:root[data-theme="dark"]` (set by `store/theme.ts`), which is **outside**
the component's shadow root. Two naive approaches fail:

| Approach                                | Why it fails                                                                                                   |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `:host([data-theme="dark"]) .x`         | The `data-theme` attribute is on `:root`, not on this host. `:host(...)` only matches the host element itself. |
| `:host-context([data-theme="dark"]) .x` | `:host-context()` is **non-standard and deprecated**. Unreliable across browsers; avoid it.                    |

You cannot, from inside a shadow root, select an ancestor of the host.

## The solution: `light-dark()`

The CSS `light-dark(lightValue, darkValue)` function resolves from the inherited
`color-scheme` property. `index.css` sets:

```css
:root {
  color-scheme: light dark;
}
```

`color-scheme` is inherited and **pierces shadow boundaries**, so `light-dark()`
works inside any component's shadow DOM. Use it to map shadcn's two-value
pattern to the real tokens:

### Recipe: shadcn `dark:` → `light-dark()`

| shadcn class                                     | Lit equivalent                                                                 |
| ------------------------------------------------ | ------------------------------------------------------------------------------ |
| `bg-background` / `dark:bg-foreground`           | `light-dark(var(--background), var(--foreground))`                             |
| `bg-background` / `dark:bg-primary-foreground`   | `light-dark(var(--background), var(--primary-foreground))`                     |
| `bg-input` / `dark:bg-input/80`                  | `light-dark(var(--input), color-mix(in oklch, var(--input) 80%, transparent))` |
| `text-foreground` / `dark:text-muted-foreground` | `light-dark(var(--foreground), var(--muted-foreground))`                       |

`color-mix(in oklch, <token> <pct>, transparent)` reproduces Tailwind's
`/opacity` modifier (`bg-input/80`).

### Worked example: switch thumb/track (`components/ui/switch.ts`)

shadcn Switch:

- track: `bg-input dark:bg-input/80`, checked `bg-primary`
- thumb: `bg-background dark:bg-foreground`, checked
  `dark:bg-primary-foreground`

```css
.track {
  background: light-dark(
    var(--input),
    color-mix(in oklch, var(--input) 80%, transparent)
  );
}
.track[aria-checked="true"] {
  background: var(--primary);
}

.thumb {
  background: light-dark(var(--background), var(--foreground));
}
.track[aria-checked="true"] .thumb {
  transform: translateX(calc(100% - 2px));
  background: light-dark(var(--background), var(--primary-foreground));
}
```

No `:host`, no `:host-context`, no JS theme reads. Pure CSS, theme-correct.

## When NOT to use `light-dark()`

For a color that is intentionally the **same in both themes** (fixed contrast,
e.g. white text on a saturated badge), use a plain fixed value. See
`design-tokens.md` → "Fixed-contrast colors".

## Color themes (default/zinc/rose/green/violet)

`light-dark()` switches on `color-scheme` (light/dark), **not** on the
`data-color` accent theme. That's correct: accent themes only change
`--primary`/`--ring` etc., which you already reference via tokens. The
light/dark switch is orthogonal and handled by `light-dark()`. You rarely need
to special-case `data-color`.

## Quick check

- Need light/dark variants? → `light-dark(tokenLight, tokenDark)`
- Need an opacity modifier (`bg-x/80`)? →
  `color-mix(in oklch, var(--x) 80%, transparent)`
- Accent-color dependent? → just reference `var(--primary)` etc. (the theme
  overrides them)
