# Toggle / Switch Controls

How to build an on/off toggle that is visually identical to the shadcn `Switch`.
Read this when you need any switch/toggle control.

**Prefer reusing `<breeze-switch>`** (`components/ui/switch.ts`) instead of
hand-rolling. Only build your own if you need a different size/variant.

---

## The classic pitfall: absolute-positioned thumb

The natural (wrong) attempt positions the thumb absolutely:

```css
/* ❌ WRONG: thumb ends up off-center */
.track {
  position: relative;
  width: 2rem;
  height: 1.25rem;
  border: 1px solid var(--border);
}
.thumb {
  position: absolute;
  top: calc(var(--space-5) / 2 - var(--space-4) / 2);
  left: calc(var(--space-5) / 2 - var(--space-4) / 2);
  width: var(--space-4);
  height: var(--space-4);
}
```

Why it breaks:

- The track has a **1px border** with `box-sizing: border-box`, so the content
  box is 2px smaller than `width`/`height`. The `calc()` ignores the border, so
  the thumb sits slightly off-center vertically, exactly the "ball not
  centered" bug.
- `top`/`left` math is fragile; any border/padding change desyncs it.

## The correct model (mirrors Radix `SwitchPrimitive`)

shadcn's Switch centers the thumb with **flexbox** and moves it with
**`transform: translateX()`**. Copy that exactly:

```css
/* ✅ CORRECT */
.track {
  position: relative;
  display: inline-flex;
  align-items: center; /* ← vertical centering, no math */
  flex-shrink: 0;
  width: var(--switch-w);
  height: var(--switch-h);
  border: 1px solid transparent; /* transparent: takes space, invisible */
  border-radius: var(--radius-full);
  background: light-dark(
    var(--input),
    color-mix(in oklch, var(--input) 80%, transparent)
  );
}
.track[aria-checked="true"] {
  background: var(--primary);
}

.thumb {
  pointer-events: none;
  display: block;
  width: var(--space-4); /* size-4 = 16px */
  height: var(--space-4);
  border-radius: var(--radius-full);
  background: light-dark(var(--background), var(--foreground));
  transform: translateX(0);
  transition: transform var(--duration-fast) var(--ease-out);
}
.track[aria-checked="true"] .thumb {
  transform: translateX(calc(100% - 2px)); /* shadcn's exact value */
}
```

### Why each piece matters

| Piece                                       | Reason                                                                                                                        |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `display: inline-flex; align-items: center` | Thumb is a flex child → always perfectly vertically centered.                                                                 |
| `transform: translateX(calc(100% - 2px))`   | `100%` is the **thumb's own width** (16px); minus 2px = 14px slide. Matches shadcn `translate-x-[calc(100%-2px)]`.            |
| `border: 1px solid transparent`             | Keeps a symmetric ~1px inset on the edge the thumb touches, without a visible border line (shadcn uses `border-transparent`). |
| No `box-shadow` on thumb                    | shadcn's thumb has no shadow (only `ring-0`). Don't add one.                                                                  |

### Size tokens

Add component tokens for non-standard sizes (the switch track is 18.4px tall,
between `--space-4` and `--space-5`):

```css
/* index.css */
--switch-w: 2rem; /* 32px */
--switch-h: 1.15rem; /* 18.4px */
```

shadcn also has `size="sm"` (`w-[24px] h-[14px]`, thumb `size-3`). Add
`--switch-sm-w` / `--switch-sm-h` if you need it.

## Theme colors

The thumb/track colors use `light-dark()`; see `theme-colors.md`. Do **not**
try `:host([data-theme])` or `:host-context()`.

## Accessibility (required)

```ts
render() {
  return html`
    <span class="track" role="switch" tabindex="0"
          aria-checked="${this.checked}"
          @click="${this._toggle}"
          @keydown="${this._onKeydown}">
      <span class="thumb"></span>
    </span>
  `;
}
// Enter / Space toggle; dispatch `change` { detail: { checked } } (bubbles, composed)
```

## Usage in a page

```ts
import "../../components/ui/switch.ts";

<breeze-switch .checked="${this._unreadOnly}" @change="${this._toggleUnread}">
</breeze-switch>

private _toggleUnread(e: CustomEvent) {
  this._unreadOnly = (e.detail as { checked: boolean }).checked;
}
```

Don't hand-roll a `<span role="switch">` with custom thumb CSS; reuse
`breeze-switch`.
