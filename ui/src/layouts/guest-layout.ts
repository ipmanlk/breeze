import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import "../components/theme-toggle.ts";

@customElement("breeze-guest-layout")
export class BreezeGuestLayout extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      position: relative;
      min-height: 100svh;
      background: var(--background);
    }
    .toggle {
      position: absolute;
      top: var(--space-4);
      right: var(--space-4);
    }
    .center {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100svh;
      padding: var(--space-4);
    }
  `;

  protected render() {
    return html`
      <div class="toggle">
        <breeze-theme-toggle></breeze-theme-toggle>
      </div>
      <div class="center"><slot></slot></div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-guest-layout": BreezeGuestLayout;
  }
}
