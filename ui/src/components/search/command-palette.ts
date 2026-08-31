import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import { getSearch } from "@/api";
import "../ui/dialog.ts";
import "../ui/plume-icon.ts";
import "../ui/spinner.ts";
import { getPrimaryNav } from "../nav/nav-config";

interface PaletteItem {
  id: string;
  label: string;
  url: string;
  type: "page" | "project" | "task" | "channel" | "direct_message" | "member";
  icon?: string;
  color?: string;
  subtitle?: string;
}

const NAV_ICON_MAP = new Map(
  [...getPrimaryNav()].map((i) => [i.url, i.icon]),
);

/**
 * Command Palette: shadcn-style command dialog.
 *
 * Design follows the shadcn command dialog:
 *  - Dialog anchored near the top (`top-[20%]`), `max-w-2xl`, `rounded-xl`, `p-0`.
 *  - Command root: `p-1`, `rounded-xl`, `bg-popover`.
 *  - Input: 32px InputGroup, `bg-input/30`, search icon left at 16px / 50% opacity.
 *  - List: `max-h-72`, hidden scrollbar.
 *  - Groups: `text-xs font-medium text-muted-foreground` headings (no uppercase).
 *  - Items: `px-2 py-1.5 gap-2`, 16px icons / 16px color badges, `bg-muted` when active.
 *  - Trailing check icon shown on the active item.
 *
 * Open with Cmd+K / Ctrl+K (or the `open-command-palette` event).
 */
@localized()
@customElement("plume-command-palette")
export class PlumeCommandPalette extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }

    /* Command root: flex size-full flex-col overflow-hidden rounded-xl p-1 */
    .command {
      display: flex;
      flex-direction: column;
      width: 100%;
      overflow: hidden;
      border-radius: var(--radius-xl);
      background: var(--popover);
      color: var(--popover-foreground);
      padding: var(--space-1);
    }

    /* Input wrapper: p-1 pb-0 */
    .command-input-wrapper {
      padding: var(--space-1) var(--space-1) 0;
    }

    /* InputGroup: h-8 rounded-lg border-input/30 bg-input/30 */
    .command-input-group {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      height: var(--control-h-sm);
      border-radius: var(--radius-lg);
      border: 1px solid color-mix(in oklch, var(--input) 30%, transparent);
      background: color-mix(in oklch, var(--input) 30%, transparent);
      padding: 0 var(--space-2);
    }
    .search-icon {
      color: var(--muted-foreground);
      opacity: 0.5;
      flex-shrink: 0;
    }
    .command-input {
      flex: 1;
      min-width: 0;
      border: none;
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      outline: none;
      padding: 0;
    }
    .command-input::placeholder {
      color: var(--muted-foreground);
    }

    /* List: max-h-72 no-scrollbar overflow-y-auto */
    .command-list {
      max-height: var(--command-list-h);
      overflow-x: hidden;
      overflow-y: auto;
      scroll-padding-block: var(--space-1);
      outline: none;
      scrollbar-width: none;
    }
    .command-list::-webkit-scrollbar {
      display: none;
    }

    /* Group: overflow-hidden p-1 */
    .command-group {
      overflow: hidden;
      padding: var(--space-1);
    }
    /* Heading: px-2 py-1.5 text-xs font-medium text-muted-foreground (NO uppercase) */
    .command-group-heading {
      padding: var(--space-1-5) var(--space-2);
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
    }

    /* Item: relative flex items-center gap-2 rounded-sm px-2 py-1.5 text-sm */
    .command-item {
      position: relative;
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border-radius: var(--radius-sm);
      color: var(--foreground);
      font-size: var(--text-sm);
      text-align: left;
      cursor: default;
      user-select: none;
      outline: none;
      border: none;
      background: transparent;
    }
    .command-item.selected {
      background: var(--muted);
      color: var(--foreground);
    }
    .command-item.selected .nav-icon {
      color: var(--foreground);
    }

    /* Nav icon: size-4 (16px); muted by default, foreground when active */
    .nav-icon {
      color: var(--muted-foreground);
      flex-shrink: 0;
    }

    /* Color badge: size-4 rounded text-[10px] font-semibold text-white */
    .badge {
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-sm);
      font-size: var(--text-2xs);
      font-weight: 600;
      color: #fff;
      flex-shrink: 0;
      overflow: hidden;
    }

    .item-label {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    /* Task item content: flex flex-col min-w-0 */
    .task-content {
      display: flex;
      flex-direction: column;
      min-width: 0;
      flex: 1;
    }
    .task-content .task-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .task-content .task-subtitle {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    /* Trailing check: ml-auto size-4, shown on the active item */
    .check {
      margin-left: auto;
      flex-shrink: 0;
      color: var(--foreground);
      opacity: 0;
    }
    .command-item.selected .check {
      opacity: 1;
    }

    /* Empty: py-6 text-center text-sm text-muted-foreground */
    .command-empty {
      padding: var(--space-6) 0;
      text-align: center;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }

    /* Loading: centered spinner */
    .command-loading {
      display: flex;
      justify-content: center;
      padding: var(--space-6) 0;
    }
  `;

  @property({ type: Boolean })
  open = false;

  @state()
  private _query = "";

  @state()
  private _debouncedQuery = "";

  @state()
  private _results: {
    id?: string;
    name?: string;
    url?: string;
    type?: string;
    color?: string;
    subtitle?: string;
  }[] = [];

  @state()
  private _isLoading = false;

  @state()
  private _selectedIndex = 0;

  @query(".command-item.selected")
  private _selectedItem!: HTMLElement | null;

  private _debounceTimer?: ReturnType<typeof setTimeout>;
  private _keydownHandler = this._onKeydown.bind(this);
  private _openHandler = this._open.bind(this);

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener("keydown", this._keydownHandler);
    document.addEventListener("open-command-palette", this._openHandler);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener("keydown", this._keydownHandler);
    document.removeEventListener("open-command-palette", this._openHandler);
    clearTimeout(this._debounceTimer);
  }

  private _open(): void {
    this.open = true;
    this._query = "";
    this._debouncedQuery = "";
    this._results = [];
    this._selectedIndex = 0;
    this._fetchResults();
  }

  private _onKeydown(e: KeyboardEvent): void {
    if ((e.metaKey || e.ctrlKey) && e.key === "k") {
      e.preventDefault();
      this.open ? this._close() : this._open();
      return;
    }
    if (!this.open) return;

    if (e.key === "Escape") {
      e.preventDefault();
      this._close();
      return;
    }

    const items = this._getItems();
    if (e.key === "ArrowDown") {
      e.preventDefault();
      this._selectedIndex = (this._selectedIndex + 1) %
        Math.max(items.length, 1);
      this._scrollActiveIntoView();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      this._selectedIndex =
        (this._selectedIndex - 1 + Math.max(items.length, 1)) %
        Math.max(items.length, 1);
      this._scrollActiveIntoView();
    } else if (e.key === "Enter" && items.length > 0) {
      e.preventDefault();
      this._select(items[this._selectedIndex]);
    }
  }

  private _close(): void {
    this.open = false;
    this._query = "";
    this._debouncedQuery = "";
    clearTimeout(this._debounceTimer);
  }

  private _onInput(e: Event): void {
    const value = (e.target as HTMLInputElement).value;
    this._query = value;
    this._selectedIndex = 0;

    clearTimeout(this._debounceTimer);
    this._debounceTimer = setTimeout(() => {
      this._debouncedQuery = value.trim();
      this._fetchResults();
    }, 250);
  }

  private async _fetchResults(): Promise<void> {
    const effectiveQuery = this._debouncedQuery;
    const shouldFetch = effectiveQuery.length === 0 ||
      effectiveQuery.length >= 2;

    if (!shouldFetch) {
      this._results = [];
      return;
    }

    this._isLoading = true;
    try {
      const { data } = await getSearch({
        query: {
          q: effectiveQuery,
          types: "project,task,channel,direct_message,member",
          limit: 10,
        },
        throwOnError: true,
      });
      this._results = (data?.results as {
        id?: string;
        name?: string;
        url?: string;
        type?: string;
        color?: string;
        subtitle?: string;
      }[]) ?? [];
    } catch {
      this._results = [];
    } finally {
      this._isLoading = false;
    }
  }

  private _getItems(): PaletteItem[] {
    const lowerQuery = this._query.toLowerCase();
    const items: PaletteItem[] = [];
    const seenUrls = new Set<string>();

    for (const item of [...getPrimaryNav()]) {
      if (seenUrls.has(item.url)) continue;
      if (lowerQuery && !item.title.toLowerCase().includes(lowerQuery)) {
        continue;
      }
      seenUrls.add(item.url);
      items.push({
        id: item.url,
        label: item.title,
        url: item.url,
        type: "page",
        icon: item.icon,
      });
    }

    for (const r of this._results) {
      // Map every backend search type to a palette item. Previously only
      // project + task were kept; channel / direct_message / member hits
      // were fetched and silently dropped.
      const type: PaletteItem["type"] = r.type === "project"
        ? "project"
        : r.type === "channel"
        ? "channel"
        : r.type === "direct_message"
        ? "direct_message"
        : r.type === "member"
        ? "member"
        : "task";
      items.push({
        id: r.id ?? "",
        label: r.name ?? "",
        url: r.url ?? "",
        type,
        color: r.color,
        subtitle: r.subtitle,
        icon: r.url ? NAV_ICON_MAP.get(r.url) : undefined,
      });
    }

    return items;
  }

  private _select(item: PaletteItem): void {
    navigate(item.url);
    this._close();
  }

  private _scrollActiveIntoView(): void {
    this.updateComplete.then(() => {
      this._selectedItem?.scrollIntoView({ block: "nearest" });
    });
  }

  private _renderNavItem(
    item: PaletteItem,
    index: number,
  ): ReturnType<typeof html> {
    const selected = index === this._selectedIndex;
    return html`
      <button
        type="button"
        class="command-item ${selected ? "selected" : ""}"
        @click="${() => this._select(item)}"
        @mouseenter="${() => {
          this._selectedIndex = index;
        }}"
      >
        <plume-icon
          class="nav-icon"
          name="${item.icon ?? "house"}"
          size="16"
        ></plume-icon>
        <span class="item-label">${item.label}</span>
        <plume-icon class="check" name="check" size="16"></plume-icon>
      </button>
    `;
  }

  private _renderProjectItem(
    item: PaletteItem,
    index: number,
  ): ReturnType<typeof html> {
    const selected = index === this._selectedIndex;
    return html`
      <button
        type="button"
        class="command-item ${selected ? "selected" : ""}"
        @click="${() => this._select(item)}"
        @mouseenter="${() => {
          this._selectedIndex = index;
        }}"
      >
        <div
          class="badge"
          style="background-color: ${item.color ?? "var(--primary)"}"
        >
          ${item.label.charAt(0).toUpperCase()}
        </div>
        <span class="item-label">${item.label}</span>
        <plume-icon class="check" name="check" size="16"></plume-icon>
      </button>
    `;
  }

  private _renderTaskItem(
    item: PaletteItem,
    index: number,
  ): ReturnType<typeof html> {
    const selected = index === this._selectedIndex;
    return html`
      <button
        type="button"
        class="command-item ${selected ? "selected" : ""}"
        @click="${() => this._select(item)}"
        @mouseenter="${() => {
          this._selectedIndex = index;
        }}"
      >
        <div
          class="badge"
          style="background-color: ${item.color ?? "var(--primary)"}"
        >
          ${item.subtitle?.charAt(0).toUpperCase() ?? "?"}
        </div>
        <div class="task-content">
          <span class="task-label">${item.label}</span>
          ${item.subtitle
            ? html`
              <span class="task-subtitle">${item.subtitle}</span>
            `
            : nothing}
        </div>
        <plume-icon class="check" name="check" size="16"></plume-icon>
      </button>
    `;
  }

  private _renderChatItem(
    item: PaletteItem,
    index: number,
  ): ReturnType<typeof html> {
    const selected = index === this._selectedIndex;
    return html`
      <button
        type="button"
        class="command-item ${selected ? "selected" : ""}"
        @click="${() => this._select(item)}"
        @mouseenter="${() => {
          this._selectedIndex = index;
        }}"
      >
        <plume-icon
          class="nav-icon"
          name="hash"
          size="16"
        ></plume-icon>
        <span class="item-label">${item.label}</span>
        <plume-icon class="check" name="check" size="16"></plume-icon>
      </button>
    `;
  }

  private _renderMemberItem(
    item: PaletteItem,
    index: number,
  ): ReturnType<typeof html> {
    const selected = index === this._selectedIndex;
    return html`
      <button
        type="button"
        class="command-item ${selected ? "selected" : ""}"
        @click="${() => this._select(item)}"
        @mouseenter="${() => {
          this._selectedIndex = index;
        }}"
      >
        <div
          class="badge"
          style="background-color: ${item.color ?? "var(--primary)"}"
        >
          ${item.label.charAt(0).toUpperCase()}
        </div>
        <span class="item-label">${item.label}</span>
        <plume-icon class="check" name="check" size="16"></plume-icon>
      </button>
    `;
  }

  protected render() {
    const items = this._getItems();
    const navItems = items.filter((i) => i.type === "page");
    const projectItems = items.filter((i) => i.type === "project");
    const taskItems = items.filter((i) => i.type === "task");
    const channelItems = items.filter(
      (i) => i.type === "channel" || i.type === "direct_message",
    );
    const memberItems = items.filter((i) => i.type === "member");

    return html`
      <plume-dialog
        .open="${this.open}"
        .noHeader="${true}"
        .noFooter="${true}"
        .showCloseButton="${false}"
        placement="top"
        style="--dialog-w: var(--command-w); --dialog-radius: var(--radius-xl); --dialog-body-padding: 0;"
        @close="${this._close}"
      >
        <div class="command">
          <div class="command-input-wrapper">
            <div class="command-input-group">
              <plume-icon class="search-icon" name="search" size="16"></plume-icon>
              <input
                class="command-input"
                type="text"
                placeholder="${msg("Search pages, projects, tasks, chats...")}"
                .value="${this._query}"
                @input="${this._onInput}"
                autocomplete="off"
                autofocus
              />
            </div>
          </div>

          <div class="command-list">
            ${this._isLoading
              ? html`
                <div class="command-loading">
                  <plume-spinner></plume-spinner>
                </div>
              `
              : items.length === 0
              ? html`
                <div class="command-empty">${msg("No results found.")}</div>
              `
              : html`
                ${navItems.length > 0
                  ? html`
                    <div class="command-group">
                      <div class="command-group-heading">${msg(
                        "Navigation",
                      )}</div>
                      ${navItems.map((item, i) => this._renderNavItem(item, i))}
                    </div>
                  `
                  : nothing} ${projectItems.length > 0
                  ? html`
                    <div class="command-group">
                      <div class="command-group-heading">${msg(
                        "Projects",
                      )}</div>
                      ${projectItems.map((item, i) =>
                        this._renderProjectItem(item, navItems.length + i)
                      )}
                    </div>
                  `
                  : nothing} ${taskItems.length > 0
                  ? html`
                    <div class="command-group">
                      <div class="command-group-heading">${msg("Tasks")}</div>
                      ${taskItems.map((item, i) =>
                        this._renderTaskItem(
                          item,
                          navItems.length + projectItems.length + i,
                        )
                      )}
                    </div>
                  `
                  : nothing} ${channelItems.length > 0
                  ? html`
                    <div class="command-group">
                      <div class="command-group-heading">${msg("Chats")}</div>
                      ${channelItems.map((item, i) =>
                        this._renderChatItem(
                          item,
                          navItems.length + projectItems.length +
                            taskItems.length + i,
                        )
                      )}
                    </div>
                  `
                  : nothing} ${memberItems.length > 0
                  ? html`
                    <div class="command-group">
                      <div class="command-group-heading">${msg("People")}</div>
                      ${memberItems.map((item, i) =>
                        this._renderMemberItem(
                          item,
                          navItems.length + projectItems.length +
                            taskItems.length + channelItems.length + i,
                        )
                      )}
                    </div>
                  `
                  : nothing}
              `}
          </div>
        </div>
      </plume-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-command-palette": PlumeCommandPalette;
  }
}
