import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { TRAILING_AT_RE } from "@/features/chat/mention-utils";
import type { MentionResult } from "@/lib/mentions";
import "../../components/ui/button.ts";
import "../../components/ui/breeze-icon.ts";
import "@/components/mention/mention-popover.ts";
import "@/features/chat/components/chat-editor.ts";

/**
 * Comment composer: a rich input for task comments.
 *
 * Reuses the chat editor (<breeze-chat-editor>) so comments get the exact
 * same @-mention pings, chips, markdown, and Ctrl/Cmd+Enter-to-send UX as
 * chat. Dispatches `submit` (content string + optional parent_id) and
 * `cancel` (Escape when editing). Call setContent()/clear() for edit mode.
 */
@localized()
@customElement("breeze-comment-composer")
export class BreezeCommentComposer extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    .wrap {
      position: relative;
    }

    .composer {
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      transition: border-color var(--dur-fast) var(--ease-1);
      overflow: hidden;
    }
    .composer:focus-within {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }

    .footer {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-3);
      border-top: 1px solid var(--border);
    }
    .hint {
      flex: 1;
      font-size: var(--text-2xs);
      color: var(--muted-foreground);
    }
    kbd {
      display: inline-flex;
      align-items: center;
      padding: 0 var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      background: var(--muted);
      font-size: var(--text-2xs);
      font-family: inherit;
      line-height: 1.4;
    }
  `;

  @property()
  placeholder = msg("Write a comment...");

  @property({ type: Boolean })
  sending = false;

  /** When set, the composer is in edit mode (shows Save/Cancel). */
  @property({ type: Boolean })
  editing = false;

  @query("breeze-chat-editor")
  private _editor!:
    import("@/features/chat/components/chat-editor.ts").BreezeChatEditor;

  @state()
  private _value = "";

  @state()
  private _mentionOpen = false;

  @state()
  private _mentionQuery = "";

  @state()
  private _mentionLeft = 8;

  // Public API

  focus(): void {
    this._editor?.focus();
  }

  clear(): void {
    this._editor?.clear();
    this._value = "";
    this._mentionOpen = false;
  }

  setContent(markdown: string): void {
    this._editor?.setContent(markdown, null);
    this._value = markdown;
  }

  // Editor events

  private _onChange(e: CustomEvent) {
    this._value = e.detail.content as string;
    this._detectMention(this._value);
  }

  private _onSend() {
    const content = this._value.trim();
    if (!content || this.sending) return;
    this.dispatchEvent(
      new CustomEvent("submit", {
        detail: { content },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onEscape() {
    this.dispatchEvent(
      new CustomEvent("cancel", { bubbles: true, composed: true }),
    );
  }

  // Mention detection (mirrors message-input)

  private _detectMention(content: string) {
    const match = TRAILING_AT_RE.exec(content);
    if (match) {
      this._mentionQuery = match[1] ?? "";
      const cp = this._editor?.getCaretPosition();
      this._mentionLeft = cp?.left ?? 8;
      this._mentionOpen = true;
    } else {
      this._mentionOpen = false;
    }
  }

  private _onMentionPick(e: CustomEvent) {
    const result = e.detail as MentionResult;
    const type = result.type || "user";
    const id = result.id || "";
    const label = result.label || id;
    this._editor?.insertMention(type, id, label);
    this._mentionOpen = false;
    this._value = this._editor?.getContent() ?? this._value;
  }

  private _onMentionClose() {
    this._mentionOpen = false;
  }

  // Render

  protected render() {
    return html`
      <div class="wrap">
        <div class="composer">
          <breeze-chat-editor
            placeholder="${this.placeholder}"
            ?disabled="${this.sending}"
            ?suggest-open="${this._mentionOpen}"
            maxlength="10000"
            @breeze-change="${this._onChange}"
            @breeze-send="${this._onSend}"
            @breeze-escape="${this._onEscape}"
          ></breeze-chat-editor>
          <div class="footer">
            <span class="hint">
              ${this.editing
                ? nothing
                : html`<kbd>⌘</kbd> <kbd>↵</kbd> to send · type <kbd>@</kbd> to mention`}
            </span>
            ${this.editing
              ? html`
                <breeze-button
                  variant="ghost"
                  size="sm"
                  ?disabled="${this.sending}"
                  @click="${this._onEscape}"
                >${msg("Cancel")}</breeze-button>
                <breeze-button
                  size="sm"
                  ?disabled="${!this._value.trim() || this.sending}"
                  @click="${this._onSend}"
                >
                  ${this.sending ? msg("Saving...") : msg("Save")}
                  </breeze-button>
              `
              : html`
                <breeze-button
                  size="sm"
                  ?disabled="${!this._value.trim() || this.sending}"
                  @click="${this._onSend}"
                >
                  <breeze-icon name="arrow-right" size="14"></breeze-icon>
                  ${this.sending ? msg("Sending...") : msg("Comment")}
                </breeze-button>
              `}
          </div>
        </div>
        ${this._mentionOpen
          ? html`
            <breeze-mention-popover
              .query="${this._mentionQuery}"
              .left="${this._mentionLeft}"
              @pick="${this._onMentionPick}"
              @close="${this._onMentionClose}"
            ></breeze-mention-popover>
          `
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-comment-composer": BreezeCommentComposer;
  }
}
