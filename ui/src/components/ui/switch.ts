import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

/**
 * Plume switch: a toggle control matching the shadcn `Switch` component.
 *
 * Default size mirrors shadcn `data-size=default`: 32 × 18.4px track with a
 * 16px thumb. The thumb is vertically centered via flexbox (`align-items:
 * center`) and slides horizontally with `transform: translateX(...)`, exactly
 * like Radix's `SwitchPrimitive`. The track uses a transparent 1px border so
 * the thumb keeps a symmetric ~1px inset on the edge it touches.
 *
 * Theme-aware thumb/track colors use the `light-dark()` CSS function, which
 * resolves from the inherited `color-scheme: light dark` on `:root`: this
 * mirrors shadcn's `dark:` variants (`bg-background` / `dark:bg-foreground` /
 * `dark:bg-primary-foreground`) using the real design tokens.
 *
 * Usage:
 *   <plume-switch ?checked="${v}" @change="${e => v = e.detail.checked}">
 *   </plume-switch>
 *
 * Events: `change`: `{ detail: { checked: boolean } }`, `bubbles + composed`.
 */
@customElement("plume-switch")
export class PlumeSwitch extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      vertical-align: middle;
    }
    /* Track: inline-flex items-center, rounded-full, border-transparent.
      shadcn default: h-[18.4px] w-[32px]. */
    .track {
      position: relative;
      display: inline-flex;
      align-items: center;
      flex-shrink: 0;
      width: var(--switch-w);
      height: var(--switch-h);
      border: 1px solid transparent;
      border-radius: var(--radius-full);
      /* bg-input / dark:bg-input/80 */
      background: light-dark(
        var(--input),
        color-mix(in oklch, var(--input) 80%, transparent)
      );
      cursor: pointer;
      outline: none;
      transition:
        background var(--dur-fast) var(--ease-1),
        border-color var(--dur-fast) var(--ease-1);
    }
    .track[aria-checked="true"] {
      background: var(--primary);
      border-color: var(--primary);
    }
    .track:focus-visible {
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 50%, transparent);
    }
    :host([disabled]) .track {
      opacity: 0.5;
      cursor: not-allowed;
    }
    /* Thumb: block, size-4 (16px), rounded-full, bg-background.
      Unchecked: translate-x-0. Checked: translate-x-[calc(100%-2px)]. */
    .thumb {
      pointer-events: none;
      display: block;
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-full);
      /* bg-background / dark:bg-foreground */
      background: light-dark(var(--background), var(--foreground));
      transform: translateX(0);
      transition: transform var(--dur-normal) var(--ease-spring);
    }
    .track[aria-checked="true"] .thumb {
      transform: translateX(calc(100% - 2px));
      /* dark:data-checked:bg-primary-foreground */
      background: light-dark(
        var(--background),
        var(--primary-foreground)
      );
    }
  `;

  @property({ type: Boolean, reflect: true })
  checked = false;

  @property({ type: Boolean, reflect: true })
  disabled = false;

  private _toggle() {
    if (this.disabled) return;
    this.checked = !this.checked;
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: { checked: this.checked },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onKeydown(e: KeyboardEvent) {
    if (this.disabled) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      this._toggle();
    }
  }

  protected render() {
    return html`
      <span
        class="track"
        role="switch"
        tabindex="${this.disabled ? -1 : 0}"
        aria-checked="${this.checked}"
        aria-disabled="${this.disabled}"
        @click="${this._toggle}"
        @keydown="${this._onKeydown}"
      >
        <span class="thumb"></span>
      </span>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-switch": PlumeSwitch;
  }
}
