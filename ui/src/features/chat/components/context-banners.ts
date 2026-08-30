import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import type { Message } from "../types";
import { stripHtml } from "../utils";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-reply-banner")
export class BreezeReplyBanner extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      height: var(--control-h-sm);
      padding: 0 var(--space-3);
      border-top: 1px solid var(--border);
      background: var(--muted);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .label {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      color: var(--primary);
      font-weight: 500;
      white-space: nowrap;
    }
    .preview {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .cancel {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-5);
      height: var(--space-5);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
    }
    .cancel:hover {
      background: var(--accent);
      color: var(--foreground);
    }
  `;

  @property({ type: Object, attribute: false })
  message!: Message;

  private _onCancel() {
    this.dispatchEvent(
      new CustomEvent("cancel", { bubbles: true, composed: true }),
    );
  }

  protected render() {
    return html`
      <span class="label">
        <breeze-icon name="reply" size="14"></breeze-icon>
        Replying
      </span>
      <span class="preview">${stripHtml(this.message.content || "")}</span>
      <button class="cancel" @click="${this
        ._onCancel}" aria-label=${msg("Cancel reply")}>
        <breeze-icon name="x" size="14"></breeze-icon>
      </button>
    `;
  }
}

@localized()
@customElement("breeze-edit-banner")
export class BreezeEditBanner extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      height: var(--control-h-sm);
      padding: 0 var(--space-3);
      border-top: 1px solid var(--border);
      background: var(--muted);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .label {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      color: var(--warning);
      font-weight: 500;
      white-space: nowrap;
    }
    .cancel {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-5);
      height: var(--space-5);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
      margin-left: auto;
    }
    .cancel:hover {
      background: var(--accent);
      color: var(--foreground);
    }
  `;

  private _onCancel() {
    this.dispatchEvent(
      new CustomEvent("cancel", { bubbles: true, composed: true }),
    );
  }

  protected render() {
    return html`
      <span class="label">
        <breeze-icon name="check" size="14"></breeze-icon>
        Editing
      </span>
      <span class="preview">Press Enter to save, Escape to cancel</span>
      <button class="cancel" @click="${this
        ._onCancel}" aria-label=${msg("Cancel edit")}>
        <breeze-icon name="x" size="14"></breeze-icon>
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-reply-banner": BreezeReplyBanner;
    "breeze-edit-banner": BreezeEditBanner;
  }
}
