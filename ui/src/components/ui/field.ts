import { css, html, LitElement } from "lit";
import { customElement, property, state } from "lit/decorators.js";

@customElement("breeze-field")
export class BreezeField extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
    }
    label {
      font-size: var(--text-sm);
      font-weight: 500;
      color: var(--foreground);
      line-height: 1;
      cursor: pointer;
    }
    :host([invalid]) label {
      color: var(--destructive);
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
      opacity: 1;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    .error:empty {
      opacity: 0;
    }
  `;

  @property()
  label = "";
  @property()
  error = "";
  @property({ type: Boolean, reflect: true })
  invalid = false;

  @state()
  private _for = "";

  #onLabelClick() {
    const input = this.querySelector("[name]") as HTMLElement | null;
    input?.focus();
  }

  #onSlotChange(e: Event) {
    const slot = e.target as HTMLSlotElement;
    const els = slot.assignedElements();
    const first = els.find((el) => el.hasAttribute("name")) as
      | HTMLElement
      | null;
    if (first && first.id) {
      this._for = first.id;
    }
  }

  protected render() {
    return html`
      <label for="${this._for}" @click="${this.#onLabelClick}">${this
        .label}</label>
      <slot @slotchange="${this.#onSlotChange}"></slot>
      <div class="error">${this.error}</div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-field": BreezeField;
  }
}
