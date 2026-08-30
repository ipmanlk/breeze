import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { OutsideClickController } from "@/lib/outside-click-controller";
import "./breeze-icon.ts";
import "./avatar.ts";

export interface ComboboxOption {
  value: string;
  label: string;
  subtitle?: string;
  avatarUrl?: string;
}

/**
 * Breeze combobox: searchable multi-select.
 *
 * The trigger shows selected items as overlapping avatars.
 * The dropdown panel has a search input and a checkbox-style option list.
 *
 * Properties: `options`, `value` (string[]), `placeholder`.
 * Events: `change`: detail = string[] of selected values.
 *
 * Accessibility: Implements WAI-ARIA combobox pattern with listbox popup.
 * Arrow keys navigate options, Enter/Backspace toggle selection, Escape closes.
 */
let _nextId = 0;

@localized()
@customElement("breeze-combobox")
export class BreezeCombobox extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      position: relative;
    }
    .trigger {
      display: flex;
      align-items: center;
      gap: var(--space-1-5);
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      cursor: pointer;
      white-space: nowrap;
      overflow: hidden;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .trigger:hover {
      background: var(--accent);
    }
    .trigger:focus-visible {
      outline: 2px solid var(--ring);
      outline-offset: 2px;
    }
    .chips {
      display: flex;
      align-items: center;
      gap: var(--space-1-5);
      flex: 1;
      min-width: 0;
      overflow: hidden;
    }
    .placeholder {
      color: var(--muted-foreground);
      font-size: var(--text-sm);
      display: flex;
      align-items: center;
      gap: var(--space-1-5);
    }
    .avatars {
      display: flex;
      align-items: center;
    }
    .avatars breeze-avatar {
      margin-left: calc(var(--space-1) * -1);
      border: 2px solid var(--background);
    }
    .avatars breeze-avatar:first-child {
      margin-left: 0;
    }
    .count {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      margin-left: var(--space-1);
    }
    .chevron {
      flex-shrink: 0;
      color: var(--muted-foreground);
    }
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    .panel {
      position: fixed;
      z-index: var(--z-dropdown);
      max-height: 16rem;
      overflow: hidden;
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
    }
    .search {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-3);
      border-bottom: 1px solid var(--border);
    }
    .search input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
    }
    .search input::placeholder {
      color: var(--muted-foreground);
    }
    .list {
      max-height: 10rem;
      overflow-y: auto;
      padding: var(--space-1);
    }
    .option {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
      text-align: left;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .option:hover,
    .option[data-highlighted] {
      background: var(--accent);
    }
    .checkbox {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      border: 1px solid var(--input);
      border-radius: var(--radius-sm);
      flex-shrink: 0;
    }
    .option[aria-selected="true"] .checkbox {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
    }
    .option .name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .empty {
      padding: var(--space-3);
      text-align: center;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }
  `;

  @property({ type: Array, attribute: false })
  options: ComboboxOption[] = [];

  @property({ type: Array, attribute: false })
  value: string[] = [];

  @property()
  placeholder = "Select...";

  @state()
  private _open = false;

  @state()
  private _search = "";

  @state()
  private _focusIndex = -1;

  private readonly _id = ++_nextId;

  private get _listboxId() {
    return `cb-list_${this._id}`;
  }

  private get _searchInputId() {
    return `cb-search_${this._id}`;
  }

  private _optionId(index: number): string {
    return `cb-opt_${this._id}_${index}`;
  }

  private get _filteredOptions(): ComboboxOption[] {
    return this._search
      ? this.options.filter((o) =>
        o.label.toLowerCase().includes(this._search.toLowerCase())
      )
      : this.options;
  }

  private _outsideClick = new OutsideClickController(this, () => {
    if (this._open) this._close();
  });

  @query(".trigger")
  private _trigger!: HTMLElement;

  @query(".panel")
  private _panel!: HTMLElement;

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("_open")) {
      if (this._open) {
        this._outsideClick.connect();
        requestAnimationFrame(() => this._positionPanel());
        this._addScrollListeners();
      } else {
        this._outsideClick.disconnect();
        this._removeScrollListeners();
      }
    }
    if (changedProps.has("_focusIndex") && this._focusIndex >= 0) {
      this._scrollIntoView();
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    this._removeScrollListeners();
  }

  private _addScrollListeners(): void {
    document.addEventListener("scroll", this._onScroll, true);
    window.addEventListener("resize", this._onScroll);
  }

  private _removeScrollListeners(): void {
    document.removeEventListener("scroll", this._onScroll, true);
    window.removeEventListener("resize", this._onScroll);
  }

  private _onScroll = (): void => {
    if (this._open) this._positionPanel();
  };

  private _positionPanel(): void {
    if (!this._panel || !this._trigger) return;
    const rect = this._trigger.getBoundingClientRect();
    const panelW = this._panel.offsetWidth;
    const panelH = this._panel.offsetHeight;

    let left = rect.left;
    // Clamp within the viewport so the panel never overflows the right edge.
    if (left + panelW > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - panelW - 8);
    }
    if (left < 8) left = 8;

    // Vertical: prefer below; flip above when there's no room below.
    const gap = 4;
    const roomBelow = window.innerHeight - rect.bottom - gap;
    const roomAbove = rect.top - gap;
    let top: number;
    if (roomBelow >= panelH || roomBelow >= roomAbove) {
      top = rect.bottom + gap;
    } else {
      top = Math.max(8, rect.top - panelH - gap);
    }

    this._panel.style.top = `${top}px`;
    this._panel.style.left = `${left}px`;
    this._panel.style.width = `${Math.max(rect.width, 200)}px`;
  }

  private _toggle() {
    const wasOpen = this._open;
    this._open = !this._open;
    if (this._open) {
      this._search = "";
      const filtered = this._filteredOptions;
      this._focusIndex = filtered.length > 0 ? 0 : -1;
      this.updateComplete.then(() => {
        const input = this.shadowRoot?.getElementById(
          this._searchInputId,
        ) as HTMLInputElement | null;
        input?.focus();
      });
    } else {
      this._focusIndex = -1;
      if (wasOpen) {
        this.updateComplete.then(() => {
          const trigger = this.shadowRoot?.querySelector(
            ".trigger",
          ) as HTMLElement | null;
          trigger?.focus();
        });
      }
    }
  }

  private _close() {
    if (!this._open) return;
    this._open = false;
    this._focusIndex = -1;
    this.updateComplete.then(() => {
      const trigger = this.shadowRoot?.querySelector(
        ".trigger",
      ) as HTMLElement | null;
      trigger?.focus();
    });
  }

  private _scrollIntoView() {
    if (this._focusIndex < 0) return;
    this.updateComplete.then(() => {
      const opt = this.shadowRoot?.getElementById(
        this._optionId(this._focusIndex),
      );
      opt?.scrollIntoView({ block: "nearest" });
    });
  }

  private _onTriggerKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      this._toggle();
    }
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!this._open) this._toggle();
    }
    if (e.key === "Escape" && this._open) {
      e.preventDefault();
      this._close();
    }
  }

  private _onSearchKeydown(e: KeyboardEvent) {
    const filtered = this._filteredOptions;
    if (filtered.length === 0) {
      if (e.key === "Escape") {
        e.preventDefault();
        this._close();
      }
      return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        this._focusIndex = this._focusIndex < filtered.length - 1
          ? this._focusIndex + 1
          : 0;
        break;
      case "ArrowUp":
        e.preventDefault();
        this._focusIndex = this._focusIndex > 0
          ? this._focusIndex - 1
          : filtered.length - 1;
        break;
      case "Enter":
        e.preventDefault();
        if (
          this._focusIndex >= 0 &&
          this._focusIndex < filtered.length
        ) {
          this._toggleValue(filtered[this._focusIndex].value);
        }
        break;
      case "Escape":
        e.preventDefault();
        this._close();
        break;
      case "Backspace":
        if (this._search === "" && this.value.length > 0) {
          e.preventDefault();
          this._toggleValue(this.value[this.value.length - 1]);
        }
        break;
      case "Home":
        e.preventDefault();
        this._focusIndex = 0;
        break;
      case "End":
        e.preventDefault();
        this._focusIndex = filtered.length - 1;
        break;
    }
  }

  private _toggleValue(id: string) {
    const next = this.value.includes(id)
      ? this.value.filter((v) => v !== id)
      : [...this.value, id];
    this.value = next;
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: next,
        bubbles: true,
        composed: true,
      }),
    );

    // After toggling, ensure focus stays on the search input
    if (this._open) {
      this.updateComplete.then(() => {
        const input = this.shadowRoot?.getElementById(
          this._searchInputId,
        ) as HTMLInputElement | null;
        input?.focus();
      });
    }
  }

  private _getInitials(name: string): string {
    return name
      .split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  }

  private _onSearchInput(e: Event) {
    const input = e.target as HTMLInputElement;
    this._search = input.value;
    const filtered = this._filteredOptions;
    this._focusIndex = filtered.length > 0
      ? Math.min(Math.max(this._focusIndex, 0), filtered.length - 1)
      : -1;
  }

  protected render() {
    const selected = this.options.filter((o) => this.value.includes(o.value));
    const filtered = this._filteredOptions;

    return html`
      <div
        class="trigger"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded="${this._open}"
        aria-controls="${this._listboxId}"
        aria-label="${this.placeholder}"
        tabindex="0"
        ?data-open="${this._open}"
        @click="${this._toggle}"
        @keydown="${this._onTriggerKeydown}"
      >
        <div class="chips">
          ${selected.length > 0
            ? html`
              <div class="avatars">
                ${selected.slice(0, 4).map(
                  (o) =>
                    html`
                      <breeze-avatar
                        size="sm"
                        src="${o.avatarUrl ?? ""}"
                        title="${o.label}"
                      >
                        ${this._getInitials(o.label)}
                      </breeze-avatar>
                    `,
                )} ${selected.length > 4
                  ? html`
                    <span class="count">+${selected.length - 4}</span>
                  `
                  : nothing}
              </div>
            `
            : html`
              <span class="placeholder">
                <breeze-icon name="user" size="14"></breeze-icon>
                ${this.placeholder}
              </span>
            `}
        </div>
        <breeze-icon class="chevron" name="chevron-down" size="14"></breeze-icon>
      </div>
      ${this._open
        ? html`
          <div class="panel">
            <div class="search">
              <breeze-icon name="search" size="14"></breeze-icon>
              <input
                type="text"
                id="${this._searchInputId}"
                placeholder="${msg("Search...")}"
                aria-controls="${this._listboxId}"
                aria-autocomplete="list"
                aria-activedescendant="${this._focusIndex >= 0 &&
                    this._focusIndex < filtered.length
                  ? this._optionId(this._focusIndex)
                  : nothing}"
                .value="${this._search}"
                @click="${(e: Event) => e.stopPropagation()}"
                @input="${this._onSearchInput}"
                @keydown="${this._onSearchKeydown}"
              />
            </div>
            <div
              class="list"
              role="listbox"
              id="${this._listboxId}"
              aria-multiselectable="true"
            >
              ${filtered.length === 0
                ? html`
                  <div class="empty">${msg("No results")}</div>
                `
                : filtered.map(
                  (o, idx) =>
                    html`
                      <button
                        class="option"
                        type="button"
                        role="option"
                        id="${this._optionId(idx)}"
                        aria-selected="${this.value.includes(o.value)
                          ? "true"
                          : "false"}"
                        ?data-highlighted="${this._focusIndex === idx}"
                        @click="${() => this._toggleValue(o.value)}"
                      >
                        <span class="checkbox">
                          ${this.value.includes(o.value)
                            ? html`
                              <breeze-icon
                                name="check"
                                size="12"
                              ></breeze-icon>
                            `
                            : ""}
                        </span>
                        <breeze-avatar
                          size="sm"
                          src="${o.avatarUrl ?? ""}"
                        >
                          ${this._getInitials(o.label)}
                        </breeze-avatar>
                        <span class="name">${o.label}</span>
                      </button>
                    `,
                )}
            </div>
          </div>
        `
        : ""}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-combobox": BreezeCombobox;
  }
}
