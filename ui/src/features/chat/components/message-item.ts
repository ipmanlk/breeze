import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { OutsideClickController } from "@/lib/outside-click-controller";
import type { Message, ReactionGroup } from "../types";
import { formatDateSeparator, formatMessageTime, stripHtml } from "../utils";
import { renderMarkdownWithMentions } from "@/lib/markdown";
import "./reaction-picker.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("plume-message-item")
export class PlumeMessageItem extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    /* Day separator: centered label with lines on both sides */
    .day-sep {
      display: flex;
      align-items: center;
      margin: var(--space-3) 0;
      padding: 0 var(--space-2);
    }
    .day-sep::before,
    .day-sep::after {
      content: "";
      flex: 1;
      height: 1px;
      background: var(--border);
    }
    .day-sep span {
      padding: 0 var(--space-3);
      font-size: var(--text-2xs, 0.6875rem);
      font-weight: 600;
      color: var(--muted-foreground);
      white-space: nowrap;
    }

    /* Message row: relative so the hover action bar can anchor to it */
    .message {
      position: relative;
      display: flex;
      gap: var(--space-3);
      padding: var(--space-1) var(--space-2);
      border-radius: var(--radius-md);
      transition: background var(--dur-fast) var(--ease-1);
    }
    .message[data-highlight] {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      transition: background 0s;
    }
    .message.flash-update {
      animation: flash-highlight var(--dur-slow) var(--ease-1);
    }
    @keyframes flash-highlight {
      0% {
        background: color-mix(in oklch, var(--primary) 20%, transparent);
      }
      100% {
        background: transparent;
      }
    }
    .message:hover {
      background: color-mix(in oklch, var(--accent) 30%, transparent);
    }

    /* Avatar column */
    .avatar-col {
      width: var(--control-h);
      flex-shrink: 0;
      padding-top: var(--space-0-5);
    }
    .avatar {
      width: var(--space-8);
      height: var(--space-8);
      border-radius: var(--radius-full);
      background: var(--muted);
      color: var(--muted-foreground);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-xs);
      font-weight: 600;
      overflow: hidden;
    }
    .grouped-time {
      display: flex;
      height: var(--space-5);
      align-items: center;
      justify-content: center;
      font-size: var(--text-2xs);
      white-space: nowrap;
      color: transparent;
    }
    .message:hover .grouped-time {
      color: var(--muted-foreground);
      opacity: 0.6;
    }

    /* Body */
    .body {
      flex: 1;
      min-width: 0;
    }
    .header-row {
      display: flex;
      align-items: baseline;
      gap: var(--space-2);
      margin-bottom: var(--space-0-5);
    }
    .sender {
      font-size: var(--text-sm);
      font-weight: 600;
      color: var(--foreground);
    }
    .time {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .edited {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .content {
      font-size: var(--text-sm);
      color: var(--foreground);
      line-height: 1.5;
      word-wrap: break-word;
      overflow-wrap: break-word;
    }
    .content p {
      margin: 0;
    }
    .content a {
      color: var(--primary);
      text-decoration: underline;
    }
    .content .mention-link {
      text-decoration: none;
    }
    .content .mention-chip {
      display: inline-block;
      border-radius: var(--radius-sm);
      padding: 0 var(--space-1);
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .content .mention-user,
    .content .mention-everyone {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
    }
    .content .mention-channel {
      background: color-mix(in oklch, #6366f1 15%, transparent);
      color: light-dark(#4f46e5, #a5b4fc);
    }
    .content .mention-project {
      background: color-mix(in oklch, #f59e0b 15%, transparent);
      color: light-dark(#b45309, #fcd34d);
    }
    .content .mention-task {
      background: color-mix(in oklch, #10b981 15%, transparent);
      color: light-dark(#059669, #6ee7b7);
    }
    .content code {
      background: var(--muted);
      padding: 1px 4px;
      border-radius: var(--radius-sm);
      font-size: var(--text-xs);
      font-family: var(--font-mono);
    }
    .content pre {
      background: var(--muted);
      padding: var(--space-2);
      border-radius: var(--radius-md);
      overflow-x: auto;
      margin: var(--space-1) 0;
    }
    .content pre code {
      background: none;
      padding: 0;
    }

    /* Attachments */
    .attachments {
      display: flex;
      flex-wrap: wrap;
      gap: var(--space-2);
      margin-top: var(--space-2);
    }
    .attachment-chip {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      padding: var(--space-1-5) var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: color-mix(in oklch, var(--muted) 50%, transparent);
      font-size: var(--text-xs);
      text-decoration: none;
      color: var(--foreground);
    }
    .attachment-chip:hover {
      background: var(--muted);
    }

    /* Reactions */
    .reactions {
      display: flex;
      flex-wrap: wrap;
      gap: var(--space-1-5);
      margin-top: var(--space-1);
    }

    /* Forwarded message */
    .forwarded {
      display: flex;
      flex-direction: column;
      gap: var(--space-0-5);
      padding: var(--space-1) var(--space-2);
      margin-bottom: var(--space-1);
      border-left: 2px solid var(--border);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .forwarded span:first-child {
      font-weight: 500;
      color: var(--foreground);
    }
    .reaction {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      height: var(--space-7);
      padding: 0 var(--space-2);
      border: 1px solid var(--border);
      border-radius: var(--radius-full);
      font-size: var(--text-xs);
      background: color-mix(in oklch, var(--muted) 40%, transparent);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .reaction:active {
      transform: scale(0.9);
      transition: var(--tr-transform);
    }
    .reaction.pop {
      animation: badge-pop var(--dur-normal) var(--ease-spring);
    }
    .reaction:hover {
      background: var(--muted);
    }
    .reaction[data-mine] {
      border-color: color-mix(in oklch, var(--primary) 50%, transparent);
      background: color-mix(in oklch, var(--primary) 10%, transparent);
      color: var(--primary);
    }
    .reaction-count {
      font-weight: 500;
    }

    /* Hover action bar: floats above the message's top-right */
    .action-row {
      position: absolute;
      top: calc(-1 * var(--space-3));
      right: var(--space-3);
      z-index: 10;
      display: flex;
      align-items: center;
      gap: 1px;
      padding: 2px;
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      box-shadow: var(--shadow-sm);
      opacity: 0;
      pointer-events: none;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    .message:hover .action-row {
      opacity: 1;
      pointer-events: auto;
    }
    .action-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-7);
      height: var(--space-7);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
    }
    .action-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .action-btn[data-danger]:hover {
      color: var(--destructive);
    }

    .picker {
      position: absolute;
      bottom: calc(100% + var(--space-1));
      right: var(--space-2);
      z-index: 20;
    }
  `;

  @property({ type: Object, attribute: false })
  message!: Message;

  @property({ type: Boolean })
  showDaySeparator = false;

  @property({ type: Boolean })
  isHighlighted = false;

  @property({ type: Boolean })
  grouped = false;

  @property()
  currentUserId = "";

  @state()
  private _showPicker = false;

  #outsideClick = new OutsideClickController(this, () => {
    this._showPicker = false;
  }, "mousedown");

  #onEscape = (e: KeyboardEvent): void => {
    if (e.key === "Escape") this._showPicker = false;
  };

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has("_showPicker")) {
      if (this._showPicker) {
        this.#outsideClick.connect();
        document.addEventListener("keydown", this.#onEscape);
      } else {
        this.#outsideClick.disconnect();
        document.removeEventListener("keydown", this.#onEscape);
      }
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#outsideClick.disconnect();
    document.removeEventListener("keydown", this.#onEscape);
  }

  private _togglePicker() {
    this._showPicker = !this._showPicker;
  }

  private _onPickerPick(e: CustomEvent) {
    const emoji = e.detail.emoji as string;
    this._showPicker = false;
    this.dispatchEvent(
      new CustomEvent("reaction", {
        detail: { message: this.message, emoji },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onReply() {
    this.dispatchEvent(
      new CustomEvent("reply", {
        detail: this.message,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onEdit() {
    this.dispatchEvent(
      new CustomEvent("edit", {
        detail: this.message,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onDelete() {
    this.dispatchEvent(
      new CustomEvent("delete", {
        detail: this.message,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onPin() {
    this.dispatchEvent(
      new CustomEvent("pin", {
        detail: this.message,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onReaction(emoji: string) {
    this.dispatchEvent(
      new CustomEvent("reaction", {
        detail: { message: this.message, emoji },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _getInitials(name: string): string {
    return name
      .split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  }

  protected render() {
    const { message, showDaySeparator, isHighlighted, grouped, currentUserId } =
      this;
    const senderName = message.sender?.name || message.sender_id || "Unknown";
    const isOwn = message.sender_id === currentUserId;

    return html`
      ${showDaySeparator
        ? html`
          <div class="day-sep">
            <span>${formatDateSeparator(message.created_at)}</span>
          </div>
        `
        : nothing}

      <div
        class="message"
        ?data-highlight="${isHighlighted}"
      >
        <div class="avatar-col">
          ${!grouped
            ? html`
              <div class="avatar">
                ${message.sender?.name
                  ? this._getInitials(message.sender.name)
                  : "?"}
              </div>
            `
            : html`
              <div class="grouped-time">
                ${formatMessageTime(message.created_at)}
              </div>
            `}
        </div>

        <div class="body">
          ${!grouped
            ? html`
              <div class="header-row">
                <span class="sender">${senderName}</span>
                <span class="time">${formatMessageTime(
                  message.created_at,
                )}</span>
                ${message.edited_at
                  ? html`
                    <span class="edited">(edited)</span>
                  `
                  : nothing}
              </div>
            `
            : nothing} ${message.forwarded_message
            ? html`
              <div class="forwarded">
                <span>${message.forwarded_message.sender?.name ||
                  "Unknown"}</span>
                <span>${stripHtml(
                  message.forwarded_message.content || "",
                )}</span>
              </div>
            `
            : nothing}

          <div class="content">
            ${unsafeHTML(
              renderMarkdownWithMentions(
                message.content || "",
                message.mentions,
              ),
            )}
          </div>

          ${message.attachments && message.attachments.length > 0
            ? html`
              <div class="attachments">
                ${message.attachments.map(
                  (a) =>
                    html`
                      <span class="attachment-chip">
                        ${a.content_type?.startsWith("image/") ? "🖼" : "📎"} ${a
                          .file_name}
                      </span>
                    `,
                )}
              </div>
            `
            : nothing} ${message.reactions && message.reactions.length > 0
            ? html`
              <div class="reactions">
                ${message.reactions.map(
                  (r: ReactionGroup) =>
                    html`
                      <span
                        class="reaction"
                        ?data-mine="${r.mine}"
                        @click="${() => this._onReaction(r.emoji)}"
                      >
                        <span>${r.emoji}</span>
                        <span class="reaction-count">${r.count}</span>
                      </span>
                    `,
                )}
              </div>
            `
            : nothing}
        </div>

        <div class="action-row">
          <button
            class="action-btn"
            @click="${this._togglePicker}"
            title="${msg("React")}"
            aria-label=${msg("Add reaction")}
          >
            <plume-icon name="smile-plus" size="14"></plume-icon>
          </button>
          <button
            class="action-btn"
            @click="${this._onReply}"
            title="${msg("Reply")}"
            aria-label=${msg("Reply")}
          >
            <plume-icon name="reply" size="14"></plume-icon>
          </button>
          ${isOwn
            ? html`
              <button
                class="action-btn"
                @click="${this._onEdit}"
                title="${msg("Edit")}"
                aria-label=${msg("Edit")}
              >
                <plume-icon name="pencil" size="14"></plume-icon>
              </button>
            `
            : nothing}
          <button
            class="action-btn"
            @click="${this._onPin}"
            title="${message.pinned ? msg("Unpin") : msg("Pin")}"
            aria-label="${message.pinned ? msg("Unpin") : msg("Pin")}"
          >
            <plume-icon name="${message.pinned
              ? "pin-off"
              : "pin"}" size="14"></plume-icon>
          </button>
          ${isOwn
            ? html`
              <button
                class="action-btn"
                data-danger
                @click="${this._onDelete}"
                title="${msg("Delete")}"
                aria-label=${msg("Delete")}
              >
                <plume-icon name="trash-2" size="14"></plume-icon>
              </button>
            `
            : nothing}
        </div>
        ${this._showPicker
          ? html`
            <plume-reaction-picker
              class="picker"
              @pick="${this._onPickerPick}"
            ></plume-reaction-picker>
          `
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-message-item": PlumeMessageItem;
  }
}
