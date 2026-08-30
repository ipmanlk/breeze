import { css, html, LitElement } from "lit";
import { customElement, property, query } from "lit/decorators.js";

/**
 * Breeze input: styled shadow-DOM input.
 * Uses formAssociated + ElementInternals for native form participation
 * (appears in form.elements, supports setFormValue for FormData).
 * Renders via default Lit shadow root (no delegatesFocus: keeps
 * input:focus working correctly).
 */
@customElement("breeze-input")
export class BreezeInput extends LitElement {
  static formAssociated = true;

  static styles = css`
    :host {
      display: block;
      width: 100%;
    }
    input {
      display: block;
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border-radius: var(--radius-md);
      border: 1px solid var(--input);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      box-sizing: border-box;
      outline: none;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        box-shadow var(--dur-fast) var(--ease-1);
    }
    input.shake {
      animation: shake var(--dur-normal) var(--ease-1);
    }
    input::placeholder {
      color: var(--muted-foreground);
    }
    input:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    input:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
    :host([invalid]) input {
      border-color: var(--destructive);
    }
  `;

  @property()
  id = "";
  @property({ reflect: true })
  name = "";
  @property()
  type: "text" | "email" | "password" | "search" | "date" | "number" = "text";
  @property({ type: Number })
  min?: number;
  @property()
  placeholder = "";
  @property()
  value = "";
  @property({ type: Boolean })
  disabled = false;
  @property({ type: Boolean, reflect: true })
  invalid = false;
  @property({ type: Boolean })
  required = false;
  @property({ type: Boolean })
  autocomplete = false;
  @property({ type: Number })
  maxlength?: number;
  @property({ type: Boolean })
  autofocus = false;

  @query("input")
  private _input!: HTMLInputElement | null;

  #internals: ElementInternals;

  constructor() {
    super();
    this.#internals = this.attachInternals();
  }

  connectedCallback() {
    super.connectedCallback();
    this.#internals.setFormValue(this.value);
  }

  protected render() {
    return html`
      <input
        id="${this.id || undefined}"
        name="${this.name || undefined}"
        type="${this.type}"
        min="${this.min ?? undefined}"
        maxlength="${this.maxlength ?? undefined}"
        placeholder="${this.placeholder || undefined}"
        .value="${this.value}"
        ?disabled="${this.disabled}"
        ?required="${this.required}"
        ?autofocus="${this.autofocus}"
        autocomplete="${this.autocomplete ? "on" : "off"}"
        @input="${this.#onInput}"
        @keydown="${this.#onKeydown}"
        @blur="${this.#onBlur}"
      />
    `;
  }

  #onInput(e: Event) {
    // Stop the native 'input' event from bubbling out of the shadow root;
    // it is `composed: true` by default and would reach the parent, causing
    // the parent's @input handler to fire twice (once for the native event,
    // once for our clean re-dispatch below).
    e.stopPropagation();
    this.value = (e.target as HTMLInputElement).value;
    this.#internals.setFormValue(this.value);
    this.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
  }

  #onBlur() {
    // Re-dispatch a 'blur' event so parents can react to focus loss.
    // The native 'blur' event is composed:false + bubbles:false, so it never
    // reaches the light DOM parent. 'focusout' is the bubbling version but
    // we dispatch a plain 'blur' for API consistency.
    this.dispatchEvent(new Event("blur"));
  }

  #onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      this.#internals.form?.requestSubmit();
    }
  }

  /** Focus the inner input. Use this instead of the native `autofocus`
   *  attribute, which logs a warning when the document already has a
   *  focused element (e.g. when a dialog opens over an active element). */
  focus(): void {
    this._input?.focus();
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-input": BreezeInput;
  }
}
