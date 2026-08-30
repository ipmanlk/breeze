import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { SignalController } from "@/lib/signal-controller";
import {
  activeConversation,
  conversationList,
  highlightMessageId,
  showChatSearch,
} from "../store";
import { chatApi } from "../api";
import { formatMessageTime, stripHtml } from "../utils";
import { sanitizeHtml } from "@/lib/sanitize";
import { navigate } from "@/routes/router";
import { identify } from "@/lib/sdk-helpers";
import type { DtoMessageSearchItemResponse } from "@/api/types.gen";
import "@/components/ui/dialog.ts";
import "@/components/ui/button.ts";
import "@/components/ui/breeze-icon.ts";
import "@/components/ui/select.ts";
import { localized, msg } from "@lit/localize";

type SearchItem = DtoMessageSearchItemResponse;

function getScopeOptions() {
  return [
    { value: "all", label: msg("All conversations") },
    { value: "workspace", label: msg("Workspace") },
    { value: "project", label: msg("Projects") },
    { value: "dm", label: msg("DMs") },
  ];
}

/**
 * Chat search dialog.
 *
 * Features:
 *  - Search input with clear button
 *  - Scope selector (breeze-select)
 *  - Filter chips (has file, has link, pinned)
 *  - Results list with cursor-based pagination (Load more)
 *  - Click result → navigate to conversation + highlight message
 *  - Loading, empty, and no-results states
 */
@localized()
@customElement("breeze-chat-search-dialog")
export class BreezeChatSearchDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }
    .body-wrap {
      display: flex;
      flex-direction: column;
      min-height: 0;
      overflow: hidden;
    }
    .search-area {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
      padding-bottom: var(--space-3);
      border-bottom: 1px solid var(--border);
      flex-shrink: 0;
    }
    .search-row {
      display: flex;
      gap: var(--space-2);
    }
    .search-input-wrap {
      position: relative;
      flex: 1;
    }
    .search-input {
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-2-5) 0 var(--space-8);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      outline: none;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        box-shadow var(--dur-fast) var(--ease-1);
    }
    .search-input:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .search-input::placeholder {
      color: var(--muted-foreground);
    }
    .search-input-wrap > .search-icon {
      position: absolute;
      left: var(--space-2-5);
      top: 50%;
      transform: translateY(-50%);
      color: var(--muted-foreground);
      pointer-events: none;
    }
    .clear-btn {
      position: absolute;
      right: var(--space-1);
      top: 50%;
      transform: translateY(-50%);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-6);
      height: var(--space-6);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
    }
    .clear-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .filter-row {
      display: flex;
      flex-wrap: wrap;
      gap: var(--space-1-5);
    }
    .filter-chip {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      padding: var(--space-1) var(--space-2-5);
      border-radius: var(--radius-full);
      border: 1px solid var(--border);
      background: var(--muted);
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      font-family: inherit;
      cursor: pointer;
      transition:
        background var(--dur-fast) var(--ease-1),
        border-color var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .filter-chip:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .filter-chip.active {
      border-color: color-mix(in oklch, var(--primary) 30%, transparent);
      background: color-mix(in oklch, var(--primary) 10%, transparent);
      color: var(--primary);
    }
    /* results */
    .results-area {
      flex: 1;
      min-height: 0;
      overflow-y: auto;
      padding: var(--space-3) 0;
    }
    .state-msg {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-12) 0;
      color: var(--muted-foreground);
      text-align: center;
    }
    .state-msg-icon {
      opacity: 0.3;
    }
    .state-msg-sub {
      font-size: var(--text-sm);
      opacity: 0.7;
    }
    /* result item */
    .result-item {
      display: block;
      width: 100%;
      padding: var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--background);
      text-align: left;
      cursor: pointer;
      font-family: inherit;
      color: inherit;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        background var(--dur-fast) var(--ease-1);
      margin-bottom: var(--space-2);
    }
    .result-item:hover {
      border-color: color-mix(in oklch, var(--primary) 30%, transparent);
      background: var(--accent);
    }
    .result-meta {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0 var(--space-2);
      margin-bottom: var(--space-1-5);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .result-meta-dot {
      opacity: 0.4;
    }
    .result-conv {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      font-weight: 500;
      color: var(--foreground);
    }
    .result-sender {
      color: var(--muted-foreground);
    }
    .result-badge {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
    }
    .result-badge.pin {
      color: var(--primary);
    }
    .result-content {
      font-size: var(--text-sm);
      line-height: 1.5;
      color: var(--foreground);
      display: -webkit-box;
      -webkit-line-clamp: 3;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }
    .result-content mark {
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
      border-radius: var(--radius-sm);
      padding: var(--space-0-5) var(--space-1);
      font-weight: 500;
    }
    .load-more-wrap {
      text-align: center;
      padding: var(--space-2) 0;
    }
    .count-badge {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      width: 100%;
    }
  `;

  #signals = new SignalController(this);

  @state()
  private _query = "";

  @state()
  private _scope = "all";

  @state()
  private _hasAttachment = false;

  @state()
  private _hasLink = false;

  @state()
  private _isPinned = false;

  @state()
  private _items: SearchItem[] = [];

  @state()
  private _nextCursor = "";

  @state()
  private _hasMore = false;

  @state()
  private _loading = false;

  @state()
  private _loadingMore = false;

  private _debounceTimer = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(showChatSearch);
  }

  protected willUpdate(changed: Map<string, unknown>): void {
    if (
      changed.has("_query") ||
      changed.has("_scope") ||
      changed.has("_hasAttachment") ||
      changed.has("_hasLink") ||
      changed.has("_isPinned")
    ) {
      if (this._debounceTimer) clearTimeout(this._debounceTimer);
      this._debounceTimer = window.setTimeout(() => {
        this._debounceTimer = 0;
        this.#search();
      }, 300);
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
  }

  #reset() {
    this._query = "";
    this._scope = "all";
    this._hasAttachment = false;
    this._hasLink = false;
    this._isPinned = false;
    this._items = [];
    this._nextCursor = "";
    this._hasMore = false;
    this._loading = false;
    this._loadingMore = false;
  }

  #onClose() {
    showChatSearch.value = false;
    this.#reset();
  }

  async #search() {
    const q = this._query.trim();
    if (!q) {
      this._items = [];
      this._hasMore = false;
      this._nextCursor = "";
      return;
    }
    this._loading = true;
    try {
      const res = await chatApi.searchMessages({
        q,
        scope: this._scope !== "all" ? this._scope : undefined,
        has_attachment: this._hasAttachment || undefined,
        has_link: this._hasLink || undefined,
        is_pinned: this._isPinned || undefined,
        limit: 20,
      });
      this._items = identify<SearchItem[]>(res.items) ?? [];
      this._nextCursor = res.next_cursor;
      this._hasMore = res.has_more;
    } catch {
      this._items = [];
      this._hasMore = false;
    } finally {
      this._loading = false;
    }
  }

  async #loadMore() {
    if (!this._nextCursor || this._loadingMore) return;
    this._loadingMore = true;
    try {
      const res = await chatApi.searchMessages({
        q: this._query.trim(),
        scope: this._scope !== "all" ? this._scope : undefined,
        has_attachment: this._hasAttachment || undefined,
        has_link: this._hasLink || undefined,
        is_pinned: this._isPinned || undefined,
        cursor: this._nextCursor,
        limit: 20,
      });
      const newItems = identify<SearchItem[]>(res.items) ?? [];
      this._items = [...this._items, ...newItems];
      this._nextCursor = res.next_cursor;
      this._hasMore = res.has_more;
    } catch {
      // ignore
    } finally {
      this._loadingMore = false;
    }
  }

  #handleJump(item: SearchItem) {
    const msg = item.message;
    if (!msg?.id) return;

    highlightMessageId.value = msg.id;

    const convId = msg.conversation_id;
    if (convId) {
      const existing = conversationList.value.find((c) => c.id === convId);
      if (existing) {
        activeConversation.value = existing;
      }
      navigate(`/chat/${convId}`);
    }

    this.#onClose();
  }

  #toggleFilter(key: "attachment" | "link" | "pinned") {
    if (key === "attachment") this._hasAttachment = !this._hasAttachment;
    if (key === "link") this._hasLink = !this._hasLink;
    if (key === "pinned") this._isPinned = !this._isPinned;
  }

  protected render() {
    const isOpen = showChatSearch.value;
    const hasQuery = this._query.trim().length > 0;
    const total = this._items.length;

    return html`
      <breeze-dialog
        style="--dialog-w:42rem"
        .open="${isOpen}"
        heading="Search Messages"
        @close="${this.#onClose}"
      >
        <div class="body-wrap">
          <!-- Search controls -->
          <div class="search-area">
            <div class="search-row">
              <div class="search-input-wrap">
                <span class="search-icon">
                  <breeze-icon name="search" size="16"></breeze-icon>
                </span>
                <input
                  class="search-input"
                  type="text"
                  placeholder=${msg("Search messages, files, snippets…")}
                  .value="${this._query}"
                  autofocus
                  @input="${(e: Event) => {
                    this._query = (e.target as HTMLInputElement).value;
                  }}"
                />
                ${hasQuery
                  ? html`
                    <button
                      class="clear-btn"
                      @click="${() => {
                        this._query = "";
                      }}"
                      aria-label=${msg("Clear")}
                    >
                      <breeze-icon name="x" size="14"></breeze-icon>
                    </button>
                  `
                  : nothing}
              </div>
              <breeze-select
                .options="${getScopeOptions()}"
                .value="${this._scope}"
                @change="${(e: CustomEvent) => {
                  this._scope = e.detail as string;
                }}"
              ></breeze-select>
            </div>

            <div class="filter-row">
              <button
                class="filter-chip ${this._hasAttachment ? "active" : ""}"
                @click="${() => this.#toggleFilter("attachment")}"
              >
                <breeze-icon name="paperclip" size="12"></breeze-icon>
                Has file
              </button>
              <button
                class="filter-chip ${this._hasLink ? "active" : ""}"
                @click="${() => this.#toggleFilter("link")}"
              >
                <breeze-icon name="link" size="12"></breeze-icon>
                Has link
              </button>
              <button
                class="filter-chip ${this._isPinned ? "active" : ""}"
                @click="${() => this.#toggleFilter("pinned")}"
              >
                <breeze-icon name="pin" size="12"></breeze-icon>
                Pinned
              </button>
            </div>
          </div>

          <!-- Results -->
          <div class="results-area">
            ${this._loading
              ? html`
                <div class="state-msg">
                  <span class="state-msg-icon">
                    <breeze-icon name="loader-2" size="20"></breeze-icon>
                  </span>
                  <span>Searching messages…</span>
                </div>
              `
              : !hasQuery
              ? html`
                <div class="state-msg">
                  <span class="state-msg-icon">
                    <breeze-icon name="message-square" size="32"></breeze-icon>
                  </span>
                  <span>Type a query to search</span>
                  <span class="state-msg-sub">Search across channels, DMs, and file names</span>
                </div>
              `
              : this._items.length === 0
              ? html`
                <div class="state-msg">
                  <span class="state-msg-icon">
                    <breeze-icon name="search" size="32"></breeze-icon>
                  </span>
                  <span>No results found</span>
                  <span class="state-msg-sub">Try a different query or adjust filters</span>
                </div>
              `
              : html`
                ${this._items.map(
                  (item) => {
                    const msg = item.message;
                    if (!msg) return nothing;
                    const convName = item.conversation_name ?? "";
                    const senderName = msg.sender?.name ?? "Unknown";
                    const snippet = item.snippet;

                    return html`
                      <button
                        type="button"
                        class="result-item"
                        @click="${() => this.#handleJump(item)}"
                      >
                        <div class="result-meta">
                          <span class="result-conv">
                            <breeze-icon name="hash" size="12"></breeze-icon>
                            ${convName}
                          </span>
                          <span class="result-meta-dot">·</span>
                          <span class="result-sender">${senderName}</span>
                          <span class="result-meta-dot">·</span>
                          <span>${formatMessageTime(
                            msg.created_at ?? "",
                          )}</span>
                          ${msg.attachments && msg.attachments.length > 0
                            ? html`
                              <span class="result-meta-dot">·</span>
                              <span class="result-badge">
                                <breeze-icon name="paperclip" size="12"></breeze-icon>
                                ${msg.attachments.length}
                              </span>
                            `
                            : nothing} ${msg.pinned
                            ? html`
                              <span class="result-meta-dot">·</span>
                              <span class="result-badge pin">
                                <breeze-icon name="pin" size="12"></breeze-icon>
                                Pinned
                              </span>
                            `
                            : nothing}
                        </div>
                        <div class="result-content">
                          ${snippet
                            ? html`
                              <div>${unsafeHTML(sanitizeHtml(snippet))}</div>
                            `
                            : html`
                              <div>${stripHtml(msg.content ?? "").substring(
                                0,
                                300,
                              )}</div>
                            `}
                        </div>
                      </button>
                    `;
                  },
                )} ${this._hasMore
                  ? html`
                    <div class="load-more-wrap">
                      <breeze-button
                        variant="ghost"
                        size="sm"
                        ?disabled="${this._loadingMore}"
                        @click="${this.#loadMore}"
                      >
                        ${this._loadingMore ? "Loading…" : "Load more"}
                      </breeze-button>
                    </div>
                  `
                  : nothing}
              `}
          </div>
        </div>

        <div slot="footer" class="footer">
          ${total > 0
            ? html`
              <span class="count-badge">${total} result${total !== 1
                ? "s"
                : ""}</span>
            `
            : html`
              <span></span>
            `}
          <breeze-button
            variant="ghost"
            type="button"
            @click="${this.#onClose}"
          >
            Close
          </breeze-button>
        </div>
      </breeze-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-chat-search-dialog": BreezeChatSearchDialog;
  }
}
