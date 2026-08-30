import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

/**
 * Lightweight tooltip: shows on hover via CSS only.
 * Place as a wrapper around the target element.
 * Usage:
 *   <breeze-tooltip text="Home" side="right">
 *     <breeze-icon name="house" size="16"></breeze-icon>
 *   </breeze-tooltip>
 */
@customElement("breeze-tooltip")
export class BreezeTooltip extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      position: relative;
      width: 100%;
    }
    .tip {
      position: absolute;
      z-index: var(--z-tooltip);
      padding: var(--space-1) var(--space-2);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      border: 1px solid var(--border);
      box-shadow: var(--shadow-md);
      font-size: var(--text-xs);
      font-weight: 500;
      white-space: nowrap;
      pointer-events: none;
      opacity: 0;
      transform: scale(0.96);
      transition:
        opacity var(--dur-fast) var(--ease-1),
        transform var(--dur-fast) var(--ease-1);
      transition-delay: var(--dur-glacial);
    }
    :host(:hover) .tip {
      opacity: 1;
      transform: scale(1);
      transition-delay: var(--dur-glacial);
    }
    :host(:not(:hover)) .tip {
      transition-delay: 0ms;
    }
    /* side positioning */
    .tip[data-side="right"] {
      left: calc(100% + var(--space-2));
      top: 50%;
      transform: translateY(-50%) scale(0.96);
    }
    :host(:hover) .tip[data-side="right"] {
      transform: translateY(-50%) scale(1);
    }
    .tip[data-side="top"] {
      bottom: calc(100% + var(--space-2));
      left: 50%;
      transform: translateX(-50%) scale(0.96);
    }
    :host(:hover) .tip[data-side="top"] {
      transform: translateX(-50%) scale(1);
    }
    .tip[data-side="bottom"] {
      top: calc(100% + var(--space-2));
      left: 50%;
      transform: translateX(-50%) scale(0.96);
    }
    :host(:hover) .tip[data-side="bottom"] {
      transform: translateX(-50%) scale(1);
    }
    .tip[data-side="left"] {
      right: calc(100% + var(--space-2));
      top: 50%;
      transform: translateY(-50%) scale(0.96);
    }
    :host(:hover) .tip[data-side="left"] {
      transform: translateY(-50%) scale(1);
    }
  `;

  @property()
  text = "";
  @property()
  side: "top" | "right" | "bottom" | "left" = "right";
  @property({ type: Boolean, reflect: true })
  hidden = false;

  protected render() {
    return html`
      <slot></slot>
      ${!this.hidden
        ? html`
          <div class="tip" data-side="${this.side}">${this.text}</div>
        `
        : ""}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-tooltip": BreezeTooltip;
  }
}
