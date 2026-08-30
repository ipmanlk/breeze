import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";

@customElement("breeze-spinner")
export class BreezeSpinner extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      flex-shrink: 0;
    }
    svg {
      width: var(--space-4);
      height: var(--space-4);
      animation: spin var(--dur-slow) linear infinite;
    }
    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }
  `;

  protected render() {
    return html`
      <svg viewBox="0 0 24 24" fill="none">
        <circle
          cx="12"
          cy="12"
          r="10"
          stroke="var(--muted)"
          stroke-width="3"
          opacity="0.25"
        />
        <path
          d="M12 2a10 10 0 0 1 10 10"
          stroke="var(--primary)"
          stroke-width="3"
          stroke-linecap="round"
        />
      </svg>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-spinner": BreezeSpinner;
  }
}
