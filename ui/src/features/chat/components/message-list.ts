import { css, html, LitElement, nothing, type PropertyValues } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import type { Message, ReactionGroup } from "../types";
import { chatApi } from "../api";
import { sameDay } from "../utils";
import { highlightMessageId, wsMessageEvents } from "../store";
import { SignalController } from "@/lib/signal-controller";
import "./message-item.ts";
import { localized, msg } from "@lit/localize";

/**
 * Scrollable message list.
 *
 * Messages are in normal column order (oldest at top, newest at bottom).
 * After loading, the list auto-scrolls to the bottom.
 * The scroll container is `flex: 1` with `overflow-y: auto`: the input
 * area (a sibling in the parent chat-area) stays fixed at the bottom.
 */
@localized()
@customElement("breeze-message-list")
export class BreezeMessageList extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }
    .scroll-container {
      flex: 1;
      overflow-y: auto;
      overflow-x: hidden;
      display: flex;
      flex-direction: column;
      padding-bottom: var(--space-4);
    }
    [data-msg-id].highlight {
      background: color-mix(in oklch, var(--primary) 10%, transparent);
      border-left: 3px solid var(--primary);
      border-radius: var(--radius-md);
      transition: background var(--dur-slow) var(--ease-1);
    }
    .msg-enter {
      animation: list-item-in var(--dur-normal) var(--ease-2);
    }
    .msg-exit {
      animation: content-out var(--dur-fast) var(--ease-3);
    }
    .skeleton-msg {
      display: flex;
      gap: var(--space-3);
      padding: var(--space-1) var(--space-2);
      margin-bottom: var(--space-1);
    }
    .skeleton-msg .sk-avatar {
      width: var(--space-8);
      height: var(--space-8);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .skeleton-msg .sk-body {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
    }
    .skeleton-msg .sk-line {
      height: var(--space-3);
      border-radius: var(--radius-sm);
    }
    .skeleton-msg .sk-line:first-child {
      width: 30%;
    }
    .skeleton-msg .sk-line:last-child {
      width: 70%;
    }
    .load-more {
      display: flex;
      align-items: center;
      justify-content: center;
      padding: var(--space-2);
    }
    .load-more-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      padding: var(--space-1) var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      cursor: pointer;
      font-family: inherit;
    }
    .load-more-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .empty {
      display: flex;
      flex: 1;
      align-items: center;
      justify-content: center;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
  `;

  @property()
  conversationId = "";

  @property()
  currentUserId = "";

  @query(".scroll-container")
  private _scrollEl!: HTMLDivElement;

  @state()
  private _messages: Message[] = [];

  @state()
  private _loading = true;

  @state()
  private _loadingMore = false;

  @state()
  private _hasMore = true;

  @state()
  private _cursor: string | undefined;

  #signals = new SignalController(this);
  #lastEventSeq = 0;
  #lastMessageCount = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(wsMessageEvents, highlightMessageId);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    if (changed.has("conversationId") && this.conversationId) {
      this._messages = [];
      this._hasMore = true;
      this._cursor = undefined;
      this._loading = true;
      this.#lastEventSeq = 0;
      this.#lastMessageCount = 0;
      this.#loadMessages();
    }
    // Process any new WS events
    this.#processWsEvents();
  }

  protected firstUpdated() {
    if (this.conversationId && this._messages.length === 0) {
      this.#loadMessages();
    }
  }

  #highlightedId = "";

  protected updated(changedProps: PropertyValues): void {
    // highlightMessageId is a signal (not a Lit property), so changedProps
    // does not carry signal changes. The internal guard
    // (highlightId !== this.#highlightedId) below prevents redundant scrolls.
    void changedProps;
    const highlightId = highlightMessageId.value;
    if (highlightId && highlightId !== this.#highlightedId) {
      this.#highlightedId = highlightId;
      requestAnimationFrame(() => {
        const el = this.shadowRoot?.querySelector(
          `[data-msg-id="${highlightId}"]`,
        );
        if (el) {
          el.scrollIntoView({ behavior: "smooth", block: "center" });
          el.classList.add("highlight");
          setTimeout(() => el.classList.remove("highlight"), 1500);
        }
      });
      highlightMessageId.value = null;
    }
  }

  async #loadMessages() {
    if (!this.conversationId) return;
    this._loading = true;
    try {
      const res = await chatApi.listMessages(this.conversationId, {
        limit: 50,
      });
      this._messages = res.items || [];
      this._hasMore = res.has_more;
      this._cursor = res.next_cursor;
    } catch {
      this._messages = [];
    }
    this._loading = false;
    this.#scrollToBottom();
  }

  async #loadMore() {
    if (this._loadingMore || !this._hasMore || !this._cursor) return;
    this._loadingMore = true;
    const el = this._scrollEl;
    const prevScrollHeight = el?.scrollHeight ?? 0;
    const prevScrollTop = el?.scrollTop ?? 0;
    try {
      const res = await chatApi.listMessages(this.conversationId, {
        before: this._cursor,
        limit: 50,
      });
      this._messages = [...(res.items || []), ...this._messages];
      this._hasMore = res.has_more;
      this._cursor = res.next_cursor;
    } catch {
      // ignore
    }
    this._loadingMore = false;
    // Preserve scroll position after prepending older messages
    requestAnimationFrame(() => {
      if (el) {
        const diff = el.scrollHeight - prevScrollHeight;
        el.scrollTop = prevScrollTop + diff;
      }
    });
  }

  #scrollToBottom() {
    requestAnimationFrame(() => {
      const el = this._scrollEl;
      if (el) {
        el.scrollTop = el.scrollHeight;
      }
    });
  }

  #processWsEvents() {
    const events = wsMessageEvents.value;
    // Seq-based: robust against the ring buffer trimming old entries
    // (array length alone would desync once trimming kicks in).
    const newEvents = events.filter((e) => e.seq > this.#lastEventSeq);
    if (newEvents.length === 0) return;
    this.#lastEventSeq = newEvents[newEvents.length - 1].seq;

    let shouldScrollBottom = false;

    for (const evt of newEvents) {
      switch (evt.type) {
        case "message_new": {
          const msg = evt.payload.message as Message;
          if (!msg || msg.conversation_id !== this.conversationId) continue;
          if (this._messages.some((m) => m.id === msg.id)) continue;
          this._messages = [...this._messages, msg];
          shouldScrollBottom = true;
          break;
        }
        case "message_updated": {
          const msg = evt.payload.message as Message;
          if (!msg || msg.conversation_id !== this.conversationId) continue;
          this._messages = this._messages.map((m) =>
            m.id === msg.id ? { ...m, ...msg } : m
          );
          break;
        }
        case "message_deleted": {
          const p = evt.payload as {
            conversation_id: string;
            message_id: string;
          };
          if (p.conversation_id !== this.conversationId) continue;
          this._messages = this._messages.filter(
            (m) => m.id !== p.message_id,
          );
          break;
        }
        case "message_reaction_added":
        case "message_reaction_removed": {
          const p = evt.payload as {
            conversation_id: string;
            message_id: string;
            emoji: string;
            user_id: string;
          };
          if (p.conversation_id !== this.conversationId) continue;
          this._messages = this._messages.map((m) => {
            if (m.id !== p.message_id) return m;
            const reactions: ReactionGroup[] = m.reactions ?? [];
            const existingIdx = reactions.findIndex(
              (r) => r.emoji === p.emoji,
            );
            if (evt.type === "message_reaction_added") {
              if (existingIdx >= 0) {
                const updated = [...reactions];
                updated[existingIdx] = {
                  ...updated[existingIdx],
                  count: updated[existingIdx].count + 1,
                  user_ids: [...updated[existingIdx].user_ids, p.user_id],
                };
                return { ...m, reactions: updated };
              }
              return {
                ...m,
                reactions: [
                  ...reactions,
                  {
                    emoji: p.emoji,
                    count: 1,
                    user_ids: [p.user_id],
                    mine: p.user_id === this.currentUserId,
                  },
                ],
              };
            }
            if (existingIdx >= 0) {
              const updated = [...reactions];
              const newCount = updated[existingIdx].count - 1;
              if (newCount <= 0) {
                updated.splice(existingIdx, 1);
              } else {
                updated[existingIdx] = {
                  ...updated[existingIdx],
                  count: newCount,
                  user_ids: updated[existingIdx].user_ids.filter(
                    (id) => id !== p.user_id,
                  ),
                };
              }
              return { ...m, reactions: updated };
            }
            return m;
          });
          break;
        }
        case "message_pinned":
        case "message_unpinned": {
          const p = evt.payload as {
            conversation_id: string;
            message?: { id: string; pinned_by?: string };
            message_id?: string;
          };
          if (p.conversation_id !== this.conversationId) continue;
          const targetId = p.message?.id || p.message_id;
          if (!targetId) continue;
          const isPinned = evt.type === "message_pinned";
          this._messages = this._messages.map((m) =>
            m.id === targetId ? { ...m, pinned: isPinned } : m
          );
          break;
        }
      }
    }

    if (shouldScrollBottom) {
      this.#scrollToBottom();
    }
  }

  protected render() {
    if (this._loading) {
      return html`
        <div class="scroll-container">
          ${[1, 2, 3, 4, 5, 6, 7, 8].map(() =>
            html`
              <div class="skeleton-msg">
                <div class="sk-avatar skeleton-shimmer"></div>
                <div class="sk-body">
                  <div class="sk-line skeleton-shimmer"></div>
                  <div class="sk-line skeleton-shimmer"></div>
                </div>
              </div>
            `
          )}
        </div>
      `;
    }

    if (this._messages.length === 0) {
      return html`
        <div class="empty">
          <p>${msg("No messages yet. Start the conversation!")}</p>
        </div>
      `;
    }

    const messages = this._messages;
    const prevCount = this.#lastMessageCount;
    this.#lastMessageCount = messages.length;
    const newCount = Math.max(0, messages.length - prevCount);

    return html`
      <div class="scroll-container">
        ${this._hasMore
          ? html`
            <div class="load-more">
              <button
                class="load-more-btn"
                @click="${() => this.#loadMore()}"
                ?disabled="${this._loadingMore}"
              >
                ${this._loadingMore
                  ? msg("Loading...")
                  : msg("Load older messages")}
              </button>
            </div>
          `
          : nothing} ${messages.map((m, i) => {
            const prev = i > 0 ? messages[i - 1] : null;
            const showDaySep = !prev ||
              !sameDay(m.created_at, prev.created_at);
            const grouped = !!prev &&
              prev.sender_id === m.sender_id &&
              !showDaySep &&
              new Date(m.created_at).getTime() -
                    new Date(prev.created_at).getTime() <
                5 * 60 * 1000;
            const isNew = newCount > 0 && i >= messages.length - newCount;
            return html`
              <div data-msg-id="${m.id}" class="${isNew ? "msg-enter" : ""}">
                <breeze-message-item
                  .message="${m}"
                  .showDaySeparator="${showDaySep}"
                  .grouped="${grouped}"
                  .currentUserId="${this.currentUserId}"
                ></breeze-message-item>
              </div>
            `;
          })}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-message-list": BreezeMessageList;
  }
}
