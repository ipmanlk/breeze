# Components

## Directory structure

```
src/components/
├── ui/          # leaf primitives (zero domain knowledge)
│   ├── avatar.ts
│   ├── plume-icon.ts
│   ├── button.ts / button-group.ts
│   ├── card.ts
│   ├── combobox.ts
│   ├── date-field.ts
│   ├── dialog.ts
│   ├── field.ts
│   ├── input.ts
│   ├── label-chip.ts / label-picker.ts
│   ├── popover.ts
│   ├── select.ts
│   ├── shortcuts-dialog.ts
│   ├── skeleton.ts / spinner.ts / stepper.ts
│   ├── switch.ts / tabs.ts / tooltip.ts
│   └── toast-host.ts / toast-store.ts / toast-host-mount.ts
├── nav/             # app chrome (workspace switcher, nav-list, top bar)
├── search/          # command palette
├── mention/         # @mention components
├── plume-task-editor.ts
├── theme-switcher.ts
├── theme-toggle.ts
├── motion-settings.ts
└── top-bar.ts
```

## Conventions

- One element per file. Filename = element tag name minus `plume-`.
- All custom element tags prefixed with `plume-` (e.g., `<plume-button>`,
  `<plume-spinner>`).
- Scoped styles via `static styles = css\`...\``; never global CSS from
  components.
- Props, no store access. Events, not method calls.

## Anatomy

```ts
import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

@customElement("plume-button")
export class PlumeButton extends LitElement {
  @property({ reflect: true })
  accessor variant: "primary" | "ghost" = "primary";
  @property({ type: Boolean, reflect: true })
  accessor disabled = false;

  static styles = css`
    :host {
      display: inline-flex;
      height: 2.25rem;
      align-items: center;
      padding: 0 var(--space-3);
      border-radius: var(--radius-md);
      background: var(--color-primary);
      color: var(--color-primary-fg);
      cursor: pointer;
    }
    :host([variant="ghost"]) {
      background: transparent;
      color: var(--color-text);
    }
    :host([disabled]) {
      opacity: 0.5;
      cursor: not-allowed;
    }
  `;

  render() {
    return html`
      <slot></slot>
    `;
  }
}
```

## Rules

1. **Accessor keyword** on `@property` fields (e.g.,
   `@property() accessor foo`).
2. **Slots over render props**: use `<slot>` for content projection.
3. **Events**: dispatch via
   `this.dispatchEvent(new CustomEvent('plume-<verb>', { detail, bubbles: true, composed: true }))`.
4. **Design tokens**: no hardcoded colors/spacing. Use `var(--color-*)`,
   `var(--space-*)`, `var(--radius-*)`.
5. **`::part()`**: expose parts for consumer overrides via `exportparts`.
6. **No Tailwind, no shadcn**: CSS tokens + scoped styles only.
