import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-shortcuts-dialog")
export class BreezeShortcutsDialog extends LitElement {
  @state()
  private _open = false;

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener("show-shortcuts", this._openHandler);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener("show-shortcuts", this._openHandler);
  }

  private _openHandler = () => {
    this._open = true;
  };

  private _close() {
    this._open = false;
  }

  private _row(keys: string[], desc: string) {
    return html`
      <div class="sd-row">
        <span class="sd-desc">${desc}</span>
        <span class="sd-keys">
          ${keys.map((k) => html`<kbd>${k}</kbd>`)}
        </span>
      </div>
    `;
  }

  render() {
    return html`
      <breeze-dialog
        .open="${this._open}"
        .heading="${msg("Keyboard shortcuts")}"
        @close="${this._close}"
      >
        <div class="sd-body">
          ${this._row(["C"], msg("Create task"))}
          ${this._row(["/"], msg("Focus search"))}
          ${this._row(["J"], msg("Navigate down"))}
          ${this._row(["K"], msg("Navigate up"))}
          ${this._row(["G", "P"], msg("Go to projects"))}
          ${this._row(["G", "I"], msg("Go to inbox"))}
          ${this._row(["G", "M"], msg("Go to my tasks"))}
          ${this._row(["G", "D"], msg("Go to dashboard"))}
          ${this._row(["⌘", "K"], msg("Command palette"))}
          ${this._row(["?"], msg("Show this help"))}
          ${this._row(["Esc"], msg("Close dialog"))}
        </div>
      </breeze-dialog>
    `;
  }

  static styles = css`
    .sd-body {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
      min-width: 20rem;
    }
    .sd-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-4);
    }
    .sd-desc {
      font-size: var(--text-sm);
      color: var(--foreground);
    }
    .sd-keys {
      display: flex;
      gap: var(--space-1);
    }
    kbd {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 1.5rem;
      height: 1.5rem;
      padding: 0 var(--space-1);
      font-size: var(--text-xs);
      font-family: var(--font-mono, monospace);
      color: var(--foreground);
      background: var(--muted);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
    }
  `;
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-shortcuts-dialog": BreezeShortcutsDialog;
  }
}
