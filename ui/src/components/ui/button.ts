import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

@customElement("breeze-button")
export class BreezeButton extends LitElement {
  static formAssociated = true;

  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
    }
    :host([fluid]) {
      width: 100%;
    }
    button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: var(--space-2);
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border-radius: var(--radius-md);
      font-size: var(--text-sm);
      font-weight: 500;
      font-family: inherit;
      line-height: 1;
      cursor: pointer;
      user-select: none;
      white-space: nowrap;
      border: 1px solid transparent;
      background: var(--primary);
      color: var(--primary-foreground);
      transition:
        background var(--dur-fast) var(--ease-1),
        opacity var(--dur-fast) var(--ease-1),
        transform var(--dur-fast) var(--ease-1);
    }
    button:active:not(:disabled) {
      transform: scale(0.97);
    }
    button:focus-visible {
      outline: none;
      box-shadow: 0 0 0 2px var(--ring);
      transition: box-shadow var(--dur-fast) var(--ease-1);
    }
    :host([variant="outline"]) button {
      background: var(--background);
      color: var(--foreground);
      border-color: var(--border);
    }
    :host([variant="ghost"]) button {
      background: transparent;
      color: var(--foreground);
      border-color: transparent;
    }
    :host([variant="destructive"]) button {
      background: var(--destructive);
      color: var(--destructive-foreground);
    }
    :host([size="icon"]) button {
      width: var(--control-h);
      padding: 0;
    }
    :host([size="sm"]) button {
      height: var(--control-h-sm);
      padding: 0 var(--space-3);
      font-size: var(--text-xs);
    }
    button:hover:not(:disabled) {
      opacity: 0.9;
    }
    :host([variant="outline"]) button:hover:not(:disabled),
    :host([variant="ghost"]) button:hover:not(:disabled) {
      background: var(--accent);
      opacity: 1;
    }
    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  `;

  @property({ reflect: true })
  variant: "" | "outline" | "ghost" | "destructive" = "";
  @property({ reflect: true })
  size: "" | "sm" | "icon" = "";
  @property({ type: Boolean, reflect: true })
  disabled = false;
  @property({ type: Boolean, reflect: true })
  fluid = false;
  @property()
  type: "button" | "submit" | "reset" = "submit";

  #internals: ElementInternals;

  constructor() {
    super();
    this.#internals = this.attachInternals();
  }

  #onClick() {
    if (this.disabled) return;
    if (this.type === "submit") {
      this.#internals.form?.requestSubmit();
    } else if (this.type === "reset") {
      this.#internals.form?.reset();
    }
  }

  protected render() {
    return html`
      <button type="${this.type}" ?disabled="${this.disabled}" @click="${this
        .#onClick}">
        <slot></slot>
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-button": BreezeButton;
  }
}
