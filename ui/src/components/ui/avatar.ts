import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

@customElement("breeze-avatar")
export class BreezeAvatar extends LitElement {
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
      width: var(--avatar-md);
      height: var(--avatar-md);
      border-radius: var(--radius-full);
      background: var(--muted);
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      font-weight: 600;
      overflow: hidden;
      flex-shrink: 0;
    }
    :host([size="sm"]) {
      width: 1.5rem; /* 24px - matches shadcn */
      height: 1.5rem;
      font-size: var(--text-2xs);
    }
    :host([size="lg"]) {
      width: var(--avatar-lg);
      height: var(--avatar-lg);
      font-size: var(--text-sm);
    }
  `;

  @property({ reflect: true })
  size: "" | "sm" | "lg" = "";
  @property()
  src = "";
  @property()
  alt = "";

  protected render() {
    if (!this.src) {
      return html`
        <slot></slot>
      `;
    }
    return html`
      <img
        src="${this.src}"
        alt="${this.alt}"
        role="${!this.alt ? "presentation" : undefined}"
      />
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-avatar": BreezeAvatar;
  }
}
