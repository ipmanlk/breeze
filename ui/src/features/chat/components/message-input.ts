import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import type { Conversation, MentionResult } from "../types";
import { chatApi } from "../api";
import { editMessage } from "../store";
import { SignalController } from "@/lib/signal-controller";
import { OutsideClickController } from "@/lib/outside-click-controller";
import { buildResolver, TRAILING_AT_RE } from "../mention-utils";
import "./reaction-picker.ts";
import "./chat-editor.ts";
import "@/components/mention/mention-popover.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-message-input")
export class BreezeMessageInput extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    .editor-wrap {
      position: relative;
    }

    /* Toolbar below the editor: attach + send */
    .toolbar {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      margin-top: var(--space-2);
    }
    .toolbar-spacer {
      flex: 1;
    }
    .icon-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-8);
      height: var(--space-8);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .icon-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .send-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      height: var(--space-8);
      padding: 0 var(--space-3);
      border: none;
      border-radius: var(--radius-md);
      background: var(--primary);
      color: var(--primary-foreground);
      font-size: var(--text-sm);
      font-weight: 500;
      cursor: pointer;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    .send-btn:hover {
      opacity: 0.9;
    }
    .send-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .emoji-wrap {
      position: relative;
      display: inline-flex;
    }
    .emoji-picker {
      position: absolute;
      bottom: calc(100% + var(--space-1));
      left: 0;
      z-index: 20;
    }
  `;

  @property({ type: Object, attribute: false })
  conversation!: Conversation;

  @state()
  private _value = "";

  @state()
  private _sending = false;

  @state()
  private _showEmoji = false;

  @state()
  private _mentionOpen = false;

  @state()
  private _mentionQuery = "";

  @state()
  private _mentionLeft = 8;

  @query("breeze-chat-editor")
  private _editor!: import("./chat-editor.ts").BreezeChatEditor;

  #signals = new SignalController(this);
  #editingId = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(editMessage);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#outsideClick.disconnect();
    document.removeEventListener("keydown", this.#onEscape);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    if (changed.has("conversation")) {
      this._value = "";
      this.#editingId = "";
      this._mentionOpen = false;
    }
  }

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has("_showEmoji")) {
      if (this._showEmoji) {
        this.#outsideClick.connect();
        document.addEventListener("keydown", this.#onEscape);
      } else {
        this.#outsideClick.disconnect();
        document.removeEventListener("keydown", this.#onEscape);
      }
    }
    if (changed.has("_sending") && !this._sending) {
      requestAnimationFrame(() => this._editor?.focus());
    }
    if (changed.has("conversation")) {
      this._editor?.clear();
    }
    if (editMessage.value && this.#editingId !== editMessage.value.id) {
      const msg = editMessage.value;
      this.#editingId = msg.id;
      this._value = msg.content || "";
      const resolver = msg.mentions ? buildResolver(msg.mentions) : null;
      this._editor?.setContent(msg.content || "", resolver);
      this._editor?.focus();
    } else if (!editMessage.value && this.#editingId) {
      this.#editingId = "";
      this._value = "";
      this._editor?.clear();
    }
  }

  // Editor event handlers

  private _onEditorChange(e: CustomEvent) {
    const content = e.detail.content as string;
    this._value = content;
    this._emitTyping();
    this._detectMention(content);
  }

  private _onEditorSend() {
    this._send();
  }

  private _onEditorEscape() {
    this._cancelEdit();
  }

  // Mention detection

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

  // Emoji

  private _toggleEmoji() {
    this._showEmoji = !this._showEmoji;
  }

  private _onEmojiPick(e: CustomEvent) {
    const emoji = e.detail.emoji as string;
    this._showEmoji = false;
    // Focus the editor so execCommand targets the contentEditable div.
    this._editor?.focus();
    document.execCommand("insertText", false, emoji);
    this._value = this._editor?.getContent() ?? this._value;
  }

  // Mention popover

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

  // Send / edit / cancel

  private _cancelEdit() {
    if (this.#editingId) {
      editMessage.value = null;
      this.#editingId = "";
      this._value = "";
      this._editor?.clear();
    } else {
      this.dispatchEvent(
        new CustomEvent("escape", { bubbles: true, composed: true }),
      );
    }
  }

  private async _send() {
    const content = this._value.trim();
    if (!content || this._sending) return;

    this._sending = true;
    try {
      if (this.#editingId) {
        const updated = await chatApi.editMessage(
          this.conversation.id,
          this.#editingId,
          content,
        );
        editMessage.value = null;
        this.#editingId = "";
        this.dispatchEvent(
          new CustomEvent("edited", {
            detail: { message: updated, conversationId: this.conversation.id },
            bubbles: true,
            composed: true,
          }),
        );
      } else {
        const msg = await chatApi.sendMessage(this.conversation.id, {
          content,
        });
        this.dispatchEvent(
          new CustomEvent("sent", {
            detail: { message: msg, conversationId: this.conversation.id },
            bubbles: true,
            composed: true,
          }),
        );
      }
      this._value = "";
      this._mentionOpen = false;
      this._editor?.clear();
    } catch {
      // error handled silently
    }
    this._sending = false;
  }

  // Outside click / escape for emoji picker

  #outsideClick = new OutsideClickController(this, () => {
    this._showEmoji = false;
  }, "mousedown");

  #onEscape = (e: KeyboardEvent): void => {
    if (e.key === "Escape") this._showEmoji = false;
  };

  private _emitTyping() {
    this.dispatchEvent(
      new CustomEvent("typing", { bubbles: true, composed: true }),
    );
  }

  // Render

  protected render() {
    const placeholder = this.#editingId
      ? "Edit message…"
      : this.conversation.type === "direct"
      ? `Message ${this.conversation.name || "user"}`
      : `Message #${this.conversation.name}`;

    const sendLabel = this.#editingId ? "Save" : "Send";

    return html`
      <div class="editor-wrap">
        <breeze-chat-editor
          placeholder="${placeholder}"
          ?disabled="${this._sending}"
          ?suggest-open="${this._mentionOpen}"
          maxlength="50000"
          @breeze-change="${this._onEditorChange}"
          @breeze-send="${this._onEditorSend}"
          @breeze-escape="${this._onEditorEscape}"
        ></breeze-chat-editor>
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
      <div class="toolbar">
        <button
          class="icon-btn"
          title=${msg("Attach files")}
          aria-label=${msg("Attach files")}
          @click="${() =>
            this.dispatchEvent(
              new CustomEvent("attach", { bubbles: true, composed: true }),
            )}"
        >
          <breeze-icon name="paperclip" size="16"></breeze-icon>
        </button>
        <div class="emoji-wrap">
          <button
            class="icon-btn"
            title=${msg("Insert emoji")}
            aria-label=${msg("Insert emoji")}
            @click="${this._toggleEmoji}"
          >
            <breeze-icon name="smile" size="16"></breeze-icon>
          </button>
          ${this._showEmoji
            ? html`
              <breeze-reaction-picker
                layout="grid"
                class="emoji-picker"
                @pick="${this._onEmojiPick}"
              ></breeze-reaction-picker>
            `
            : nothing}
        </div>
        <div class="toolbar-spacer"></div>
        <button
          class="send-btn"
          @click="${this._send}"
          ?disabled="${!this._value.trim() || this._sending}"
        >
          <breeze-icon name="arrow-right" size="16"></breeze-icon>
          ${sendLabel}
        </button>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-message-input": BreezeMessageInput;
  }
}
