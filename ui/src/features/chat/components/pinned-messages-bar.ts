import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { chatApi } from "../api";
import type { Message } from "../types";
import { wsMessageEvents } from "../store";
import { SignalController } from "@/lib/signal-controller";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-pinned-messages-bar")
export class BreezePinnedMessagesBar extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      border-bottom: 1px solid var(--border);
      background: color-mix(in oklch, var(--muted) 30%, transparent);
    }

    .bar {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-4);
      border: none;
      background: transparent;
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      font-family: inherit;
      text-align: left;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .bar:hover {
      background: color-mix(in oklch, var(--muted) 50%, transparent);
    }
    .pin-icon,
    .chevron {
      display: inline-flex;
      align-items: center;
      flex-shrink: 0;
    }
    .count {
      font-weight: 500;
    }
    .chevron {
      margin-left: auto;
    }

    .list {
      max-height: var(--space-80);
      overflow-y: auto;
      padding: var(--space-3) var(--space-2) var(--space-2);
    }

    .pinned-item {
      margin-bottom: var(--space-0-5);
    }
    .pinned-item:last-child {
      margin-bottom: 0;
    }
  `;

  @property()
  conversationId = "";

  @property()
  conversationType = "";

  @property()
  currentUserId = "";

  @state()
  private _pinned: Message[] = [];

  @state()
  private _expanded = false;

  private _loading = false;
  #signals = new SignalController(this);
  #lastEventSeq = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(wsMessageEvents);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    if (
      changed.has("conversationId") &&
      this.conversationId &&
      (this.conversationType === "channel" ||
        this.conversationType === "voice")
    ) {
      this._pinned = [];
      this.#loadPinned();
    }
    const events = wsMessageEvents.value;
    const newEvents = events.filter((e) => e.seq > this.#lastEventSeq);
    if (newEvents.length > 0) {
      this.#lastEventSeq = newEvents[newEvents.length - 1].seq;
      const hasPinEvent = newEvents.some(
        (e) => e.type === "message_pinned" || e.type === "message_unpinned",
      );
      if (hasPinEvent && this.conversationId) {
        this.#loadPinned();
      }
    }
  }

  async #loadPinned() {
    this._loading = true;
    try {
      const res = await chatApi.listPinned(this.conversationId);
      this._pinned = res.items || [];
    } catch {
      this._pinned = [];
    }
    this._loading = false;
  }

  private _toggle() {
    this._expanded = !this._expanded;
  }

  protected render() {
    if (this._loading || this._pinned.length === 0) return nothing;

    return html`
      <button class="bar" @click="${this._toggle}">
        <span class="pin-icon">
          <breeze-icon name="pin" size="12"></breeze-icon>
        </span>
        <span class="count">${this._pinned.length} ${msg("pinned")}</span>
        <span class="chevron">
          <breeze-icon
            name="${this._expanded ? "chevron-up" : "chevron-down"}"
            size="12"
          ></breeze-icon>
        </span>
      </button>
      ${this._expanded
        ? html`
          <div class="list">
            ${this._pinned.map(
              (m) =>
                html`
                  <div class="pinned-item">
                    <breeze-message-item
                      .message="${m}"
                      currentUserId="${this.currentUserId}"
                    ></breeze-message-item>
                  </div>
                `,
            )}
          </div>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-pinned-messages-bar": BreezePinnedMessagesBar;
  }
}
