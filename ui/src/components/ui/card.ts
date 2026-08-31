import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";

@customElement("plume-card")
export class PlumeCard extends LitElement {
  static styles = css`
    :host {
      display: block;
      box-sizing: border-box;
      max-width: 100%;
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--card);
      color: var(--card-foreground);
      padding: var(--space-6);
      box-shadow: var(--shadow-xs);
    }
  `;

  protected render() {
    return html`
      <slot></slot>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-card": PlumeCard;
  }
}
