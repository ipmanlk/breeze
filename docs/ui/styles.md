# Styles

## Location

`src/styles/` contains global CSS.

## Files

| File        | Description                                                             |
| ----------- | ----------------------------------------------------------------------- |
| `index.css` | Reset + design tokens. Single global stylesheet loaded by `index.html`. |

## Design tokens

All tokens are defined as CSS custom properties on `:root`:

### Spacing scale

`--space-1` through `--space-12` (0.25rem → 3rem)

### Border radius

`--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-xl`, `--radius-full`

### Typography

- `--font-sans`, `--font-mono`
- `--text-xs` through `--text-2xl`
- `--leading-tight`, `--leading-normal`

### Motion

- `--ease-out`
- `--duration-fast` (120ms), `--duration-normal` (200ms)

### Layout

- `--sidebar-w` (16rem), `--sidebar-w-collapsed` (3rem)
- `--topbar-h` (3rem)

### Semantic colors

Defined per theme (`light`/`dark`) and per color accent
(`default`/`zinc`/`rose`/`green`/`violet`):

- `--background`, `--foreground`
- `--primary`, `--primary-foreground`
- `--secondary`, `--secondary-foreground`
- `--muted`, `--muted-foreground`
- `--accent`, `--accent-foreground`
- `--destructive`, `--destructive-foreground`
- `--warning`, `--warning-foreground`
- `--border`, `--input`, `--ring`
- `--sidebar`, `--sidebar-foreground`, `--sidebar-accent`, `--sidebar-border`,
  `--sidebar-primary`, `--sidebar-ring`

## Rules

- Component styles go in each element's `static styles`, not in global CSS.
- Only `index.css` is a global stylesheet.
- No Tailwind CSS; all styling uses CSS custom properties + scoped Lit styles.
