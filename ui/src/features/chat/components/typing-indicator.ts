import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized } from "@lit/localize";

@localized()
@customElement("breeze-typing-indicator")
export class BreezeTypingIndicator extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      padding: var(--space-1) var(--space-2);
      border-radius: var(--radius-md);
      background: var(--popover);
      box-shadow: var(--shadow-sm);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      pointer-events: auto;
    }
    .dots {
      display: flex;
      gap: 3px;
    }
    .dot {
      width: 5px;
      height: 5px;
      border-radius: 50%;
      background: var(--muted-foreground);
      animation: bounce var(--dur-slow) ease-in-out infinite;
    }
    .dot:nth-child(2) {
      animation-delay: var(--dur-instant);
    }
    .dot:nth-child(3) {
      animation-delay: calc(2 * var(--dur-instant));
    }
    @keyframes bounce {
      0%,
      80%,
      100% {
        opacity: 0.3;
        transform: translateY(0);
      }
      40% {
        opacity: 1;
        transform: translateY(-4px);
      }
    }
  `;

  @property({ type: Array, attribute: false })
  names: string[] = [];

  protected render() {
    if (this.names.length === 0) return "";

    const text = this.names.length === 1
      ? `${this.names[0]} is typing`
      : this.names.length === 2
      ? `${this.names[0]} and ${this.names[1]} are typing`
      : `${this.names[0]} and ${this.names.length - 1} others are typing`;

    return html`
      <span class="dots">
        <span class="dot"></span>
        <span class="dot"></span>
        <span class="dot"></span>
      </span>
      <span>${text}</span>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-typing-indicator": BreezeTypingIndicator;
  }
}
