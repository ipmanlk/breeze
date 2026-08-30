# Themes

## Location

`src/themes/` contains CSS custom property overrides for theme variants.

## Theme structure

Each file is a `[data-theme="<name>"]` selector block of CSS custom properties,
loaded dynamically by `ThemeController`.

### Current themes

- `default.css`: light mode (default in `index.css`)
- `dark.css`: explicit dark mode

## How it works

1. `initTheme()` in `src/store/theme.ts` reads `localStorage` (key `"theme"`)
   and applies it to `document.documentElement.dataset.theme`.
2. Global CSS in `src/styles/index.css` defines all tokens for
   `[data-theme='light']` and `[data-theme='dark']`.
3. Theme files in `src/themes/` provide additional overrides loaded on demand.

### Color themes

In addition to light/dark, there are accent color themes managed via
`data-color` attribute:

| Color theme | Accent       |
| ----------- | ------------ |
| `default`   | Blue         |
| `zinc`      | Neutral gray |
| `rose`      | Rose/red     |
| `green`     | Emerald      |
| `violet`    | Purple       |

Color theme tokens are defined inline in `src/styles/index.css` under
`[data-color="<name>"]` selectors.
