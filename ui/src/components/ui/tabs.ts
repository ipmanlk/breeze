import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

export interface TabItem {
  id: string;
  label: string;
}

@customElement("plume-tabs")
export class PlumeTabs extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      border-bottom: 1px solid var(--border);
      width: 100%;
    }
    .tab {
      position: relative;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: var(--space-2) var(--space-3);
      border: none;
      background: transparent;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
      font-weight: 500;
      font-family: inherit;
      cursor: pointer;
      white-space: nowrap;
      outline: none;
      user-select: none;
      transition: color var(--dur-fast) var(--ease-1);
    }
    .tab:focus-visible {
      outline: none;
      box-shadow: inset 0 0 0 2px var(--ring);
      border-radius: var(--radius-sm);
    }
    .tab::after {
      content: "";
      position: absolute;
      left: 0;
      right: 0;
      bottom: -1px;
      height: 2px;
      background: var(--primary);
      opacity: 0;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    .tab:hover {
      color: var(--foreground);
    }
    .tab[data-active] {
      color: var(--foreground);
    }
    .tab[data-active]::after {
      opacity: 1;
    }
  `;

  @property({ type: Array, attribute: false })
  tabs: TabItem[] = [];

  @property()
  value = "";

  connectedCallback() {
    super.connectedCallback();
    this.setAttribute("role", "tablist");
  }

  private _select(id: string) {
    if (id === this.value) return;
    this.value = id;
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: id,
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    return html`
      ${this.tabs.map(
        (t) =>
          html`
            <button
              class="tab"
              type="button"
              role="tab"
              id="tab-${t.id}"
              aria-selected="${this.value === t.id}"
              ?data-active="${this.value === t.id}"
              @click="${() => this._select(t.id)}"
            >
              ${t.label}
            </button>
          `,
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-tabs": PlumeTabs;
  }
}
