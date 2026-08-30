import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { type MentionResult, searchMentions } from "@/lib/mentions";

/**
 * Reusable @-mention popover: shared by the chat composer and the task
 * comment composer so both support identical pings/mentions.
 *
 * Renders a keyboard-navigable list of mentionable entities (users,
 * @everyone, channels, projects, tasks) and dispatches a `pick` event with
 * the chosen result. Anchored above its host via `bottom: 100%`.
 */
@localized()
@customElement("breeze-mention-popover")
export class BreezeMentionPopover extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      position: absolute;
      bottom: 100%;
      z-index: 50;
      width: 20rem;
      margin-bottom: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      box-shadow: var(--shadow-lg);
      overflow: hidden;
      animation: menu-in var(--dur-fast) var(--ease-2);
      transform-origin: bottom left;
    }
    .loading,
    .empty {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-3);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .list {
      max-height: 13rem;
      overflow-y: auto;
      padding: var(--space-1);
    }
    .item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      text-align: left;
      cursor: pointer;
    }
    .item[data-active] {
      background: var(--accent);
    }
    .item-icon {
      display: inline-flex;
      align-items: center;
      flex-shrink: 0;
      color: var(--muted-foreground);
    }
    .item-icon[data-everyone] {
      color: var(--primary);
    }
    .item-label {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .item-meta {
      flex-shrink: 0;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
  `;

  @property()
  query = "";

  @property()
  left = 8;

  @state()
  private _results: MentionResult[] = [];

  @state()
  private _loading = false;

  @state()
  private _active = 0;

  @query(".list")
  private _listEl!: HTMLElement | null;

  #debounce: ReturnType<typeof setTimeout> | null = null;
  #abort: AbortController | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener("keydown", this.#onKey);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener("keydown", this.#onKey);
    if (this.#debounce) clearTimeout(this.#debounce);
    this.#abort?.abort();
  }

  protected willUpdate(changed: Map<string, unknown>): void {
    if (changed.has("query")) {
      this.#scheduleSearch();
    }
  }

  #scheduleSearch(): void {
    if (this.#debounce) clearTimeout(this.#debounce);
    this.#debounce = setTimeout(() => this.#search(), 150);
  }

  async #search(): Promise<void> {
    this.#abort?.abort();
    const ac = new AbortController();
    this.#abort = ac;
    this._loading = true;
    this._active = 0;
    try {
      const res = await searchMentions(
        this.query,
        ["everyone", "user", "channel", "project", "task"],
        10,
      );
      if (!ac.signal.aborted) {
        this._results = res.results || [];
      }
    } catch {
      if (!ac.signal.aborted) this._results = [];
    }
    if (!ac.signal.aborted) this._loading = false;
  }

  #onKey = (e: KeyboardEvent): void => {
    if (this._results.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      e.stopPropagation();
      this._active = (this._active + 1) % this._results.length;
      this.#scrollActiveIntoView();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      e.stopPropagation();
      this._active = (this._active - 1 + this._results.length) %
        this._results.length;
      this.#scrollActiveIntoView();
    } else if (e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      this.#pick(this._active);
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      this.#close();
    }
  };

  #scrollActiveIntoView(): void {
    const container = this._listEl;
    if (!container) return;
    const item = container.children[this._active] as HTMLElement | undefined;
    if (!item) return;
    const top = item.offsetTop;
    const bottom = top + item.offsetHeight;
    if (top < container.scrollTop) {
      container.scrollTop = top;
    } else if (bottom > container.scrollTop + container.clientHeight) {
      container.scrollTop = bottom - container.clientHeight;
    }
  }

  #pick(index: number): void {
    const result = this._results[index];
    if (!result) return;
    this.dispatchEvent(
      new CustomEvent("pick", {
        detail: result,
        bubbles: true,
        composed: true,
      }),
    );
  }

  #close(): void {
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  #iconName(type: string): string {
    switch (type) {
      case "user":
        return "user";
      case "channel":
        return "hash";
      case "project":
        return "folder-open";
      case "task":
        return "list-todo";
      default:
        return "at-sign";
    }
  }

  protected render() {
    return html`
      <div style="left: ${this.left}px">
        ${this._loading
          ? html`
            <div class="loading">${msg("Searching…")}</div>
          `
          : this._results.length === 0
          ? html`
            <div class="empty">${msg("No matches")}</div>
          `
          : html`
            <div class="list">
              ${this._results.map(
                (r, i) =>
                  html`
                    <button
                      class="item"
                      ?data-active="${i === this._active}"
                      @click="${() => this.#pick(i)}"
                      @mouseenter="${() => (this._active = i)}"
                    >
                      <span
                        class="item-icon"
                        ?data-everyone="${r.type === "everyone"}"
                      >
                        <breeze-icon name="${this.#iconName(
                          r.type || "",
                        )}" size="14"></breeze-icon>
                      </span>
                      <span class="item-label">${r.label}</span>
                      ${r.type === "task" && r.project_name
                        ? html`
                          <span class="item-meta">${r.project_name}</span>
                        `
                        : nothing}
                    </button>
                  `,
              )}
            </div>
          `}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-mention-popover": BreezeMentionPopover;
  }
}
