import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { OutsideClickController } from "@/lib/outside-click-controller";
import "./breeze-icon.ts";

export interface SelectOption {
  value: string;
  label: string;
  /** Optional CSS color for a dot indicator (status colors, etc.). */
  color?: string;
}

let _nextSelectId = 1;

const PANEL_MAX_H = 16; // rem: matches .panel max-height below
const PANEL_GAP = 4; // px between trigger and panel

/**
 * Breeze select: single-select popover dropdown.
 *
 * Uses `position: fixed` for the dropdown panel so it is never clipped by
 * overflow ancestors (dialogs, scroll containers, etc.). The panel is at
 * least as wide as the trigger but may grow wider to fit its content
 * (clamped to the viewport), and flips above the trigger when there is no
 * room below.
 *
 * Properties: `options`, `value`, `placeholder`, `searchable`.
 * Events: `change`: detail = selected value.
 *
 * Set `searchable` to render a search input at the top of the panel that
 * filters options by label: ideal for long option lists (members,
 * timezones, audit filters).
 *
 * Accessibility: role=combobox + aria-expanded / aria-haspopup=listbox on
 * trigger; role=listbox on panel; role=option + aria-selected on items;
 * ArrowUp/Down/Home/End navigates options; Enter/Space selects; Escape closes;
 * focus stays on trigger (aria-activedescendant pattern). When searchable,
 * focus moves to the search input on open.
 */
@localized()
@customElement("breeze-select")
export class BreezeSelect extends LitElement {
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
      justify-content: space-between;
      gap: var(--space-2);
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      white-space: nowrap;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .trigger:hover {
      background: var(--accent);
    }
    .trigger:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .trigger-label {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      flex: 1;
      min-width: 0;
      overflow: hidden;
    }
    .trigger-label .truncate {
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .placeholder {
      color: var(--muted-foreground);
    }
    .dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .panel {
      position: fixed;
      z-index: var(--z-dropdown);
      max-height: ${PANEL_MAX_H}rem;
      overflow: hidden;
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      padding: var(--space-1);
      display: flex;
      flex-direction: column;
    }
    .search {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-1-5) var(--space-2);
      margin-bottom: var(--space-1);
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
      max-height: 14rem;
      overflow-y: auto;
      overscroll-behavior: contain;
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
      white-space: nowrap;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .option:hover {
      background: var(--accent);
    }
    .option[data-active] {
      background: var(--accent);
    }
    .option .label {
      flex: 1;
    }
    .empty {
      padding: var(--space-3);
      text-align: center;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }
  `;

  @property({ type: Array, attribute: false })
  options: SelectOption[] = [];

  @property()
  value = "";

  @property()
  placeholder = msg("Select...");

  @property()
  name = "";

  /** Render a search input at the top of the panel that filters options by
   *  label. Use for long option lists. */
  @property({ type: Boolean })
  searchable = false;

  @state()
  private _open = false;

  @state()
  private _activeIndex = -1;

  @state()
  private _search = "";

  private _id = _nextSelectId++;

  private get _listboxId() {
    return `breeze-select-${this._id}-listbox`;
  }

  private get _triggerId() {
    return `breeze-select-${this._id}-trigger`;
  }

  private get _searchInputId() {
    return `breeze-select-${this._id}-search`;
  }

  private _optionId(index: number): string {
    return `breeze-select-${this._id}-option-${index}`;
  }

  private get _filteredOptions(): SelectOption[] {
    if (!this.searchable || !this._search) return this.options;
    const q = this._search.toLowerCase();
    return this.options.filter((o) => o.label.toLowerCase().includes(q));
  }

  @query(".trigger")
  private _trigger!: HTMLElement;

  @query(".panel")
  private _panel!: HTMLElement;

  private _outsideClick = new OutsideClickController(this, () => {
    this._open = false;
  });

  /**
   * Document-level Escape listener (safety net when trigger doesn't have
   * focus: e.g. programmatic focus shift).
   */
  private _onDocKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape" && this._open) {
      this._open = false;
      e.preventDefault();
      e.stopPropagation();
    }
  };

  private _onScroll = () => {
    if (this._open) this._positionPanel();
  };

  private _onResize = () => {
    if (this._open) this._positionPanel();
  };

  private _positionPanel() {
    if (!this._panel || !this._trigger) return;
    const rect = this._trigger.getBoundingClientRect();
    const panelW = this._panel.offsetWidth;
    const panelH = this._panel.offsetHeight;

    // Horizontal: align to trigger left, allow growing wider than the
    // trigger, clamped to the viewport (never forced to trigger width;
    // that truncated long option labels like member names).
    let left = rect.left;
    if (left + panelW > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - panelW - 8);
    }
    if (left < 8) left = 8;

    // Vertical: prefer below; flip above when there's no room below.
    const roomBelow = window.innerHeight - rect.bottom - PANEL_GAP;
    const roomAbove = rect.top - PANEL_GAP;
    let top: number;
    if (roomBelow >= panelH || roomBelow >= roomAbove) {
      top = rect.bottom + PANEL_GAP;
    } else {
      top = Math.max(8, rect.top - panelH - PANEL_GAP);
    }

    this._panel.style.top = `${top}px`;
    this._panel.style.left = `${left}px`;
    // At least as wide as the trigger; the panel's own content may make it
    // wider (clamped above). Never narrower than the trigger.
    this._panel.style.minWidth = `${rect.width}px`;
  }

  /** Initialise the active (keyboard-highlighted) index to the currently
   *  selected value, or 0 if nothing is selected and there are options. */
  private _initActiveIndex() {
    const opts = this._filteredOptions;
    if (opts.length === 0) {
      this._activeIndex = -1;
    } else {
      this._activeIndex = Math.max(
        opts.findIndex((o) => o.value === this.value),
        0,
      );
    }
  }

  private _scrollActiveIntoView() {
    if (!this._open || this._activeIndex < 0) return;
    requestAnimationFrame(() => {
      const id = this._optionId(this._activeIndex);
      this.shadowRoot?.getElementById(id)?.scrollIntoView({ block: "nearest" });
    });
  }

  private _focusSearch() {
    if (!this.searchable) return;
    requestAnimationFrame(() => {
      this.shadowRoot?.getElementById(this._searchInputId)?.focus();
    });
  }

  /**
   * Keyboard navigation on the trigger button.
   * Focus stays on the trigger (aria-activedescendant pattern): options
   * have tabindex="-1" and are not navigable by tab.
   */
  private _onTriggerKeydown = (e: KeyboardEvent) => {
    switch (e.key) {
      case "Escape":
        if (this._open) {
          this._open = false;
          e.preventDefault();
          e.stopPropagation();
        }
        break;

      case "ArrowDown":
        e.preventDefault();
        if (!this._open) {
          this._open = true;
          this._initActiveIndex();
          this._scrollActiveIntoView();
          this._focusSearch();
        } else if (this._filteredOptions.length > 0) {
          this._activeIndex = Math.min(
            this._activeIndex + 1,
            this._filteredOptions.length - 1,
          );
          this._scrollActiveIntoView();
        }
        break;

      case "ArrowUp":
        e.preventDefault();
        if (!this._open) {
          this._open = true;
          this._initActiveIndex();
          this._scrollActiveIntoView();
          this._focusSearch();
        } else if (this._filteredOptions.length > 0) {
          this._activeIndex = Math.max(this._activeIndex - 1, 0);
          this._scrollActiveIntoView();
        }
        break;

      case "Enter":
      case " ":
        e.preventDefault();
        if (
          this._open && this._activeIndex >= 0 &&
          this._activeIndex < this._filteredOptions.length
        ) {
          this._select(this._filteredOptions[this._activeIndex].value);
        } else if (!this._open) {
          this._open = true;
          this._initActiveIndex();
          this._scrollActiveIntoView();
          this._focusSearch();
        }
        break;

      case "Home":
        if (this._open && this._filteredOptions.length > 0) {
          e.preventDefault();
          this._activeIndex = 0;
          this._scrollActiveIntoView();
        }
        break;

      case "End":
        if (this._open && this._filteredOptions.length > 0) {
          e.preventDefault();
          this._activeIndex = this._filteredOptions.length - 1;
          this._scrollActiveIntoView();
        }
        break;
    }
  };

  private _onSearchKeydown = (e: KeyboardEvent) => {
    const opts = this._filteredOptions;
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        if (opts.length > 0) {
          this._activeIndex = this._activeIndex < opts.length - 1
            ? this._activeIndex + 1
            : 0;
          this._scrollActiveIntoView();
        }
        break;
      case "ArrowUp":
        e.preventDefault();
        if (opts.length > 0) {
          this._activeIndex = this._activeIndex > 0
            ? this._activeIndex - 1
            : opts.length - 1;
          this._scrollActiveIntoView();
        }
        break;
      case "Enter":
        e.preventDefault();
        if (this._activeIndex >= 0 && this._activeIndex < opts.length) {
          this._select(opts[this._activeIndex].value);
        }
        break;
      case "Escape":
        e.preventDefault();
        this._open = false;
        this._trigger?.focus();
        break;
    }
  };

  private _onSearchInput = (e: Event) => {
    this._search = (e.target as HTMLInputElement).value;
    this._activeIndex = this._filteredOptions.length > 0 ? 0 : -1;
  };

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("_open")) {
      if (this._open) {
        this._outsideClick.connect();
        document.addEventListener("keydown", this._onDocKeydown);
        window.addEventListener("scroll", this._onScroll, true);
        window.addEventListener("resize", this._onResize);
        this._search = "";
        if (this._activeIndex < 0) {
          this._initActiveIndex();
        }
        requestAnimationFrame(() => {
          this._positionPanel();
          this._scrollActiveIntoView();
          this._focusSearch();
        });
      } else {
        this._outsideClick.disconnect();
        document.removeEventListener("keydown", this._onDocKeydown);
        window.removeEventListener("scroll", this._onScroll, true);
        window.removeEventListener("resize", this._onResize);
        this._activeIndex = -1;
        this._search = "";
      }
    }
    if (changedProps.has("_search") && this._open) {
      // Reflow after filtering: reposition (height may change) + keep active in view.
      requestAnimationFrame(() => {
        this._positionPanel();
        this._scrollActiveIntoView();
      });
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onDocKeydown);
    window.removeEventListener("scroll", this._onScroll, true);
    window.removeEventListener("resize", this._onResize);
  }

  private _toggle() {
    this._open = !this._open;
    if (this._open && this._activeIndex < 0) {
      this._initActiveIndex();
    }
  }

  private _select(value: string) {
    this.value = value;
    this._open = false;
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: value,
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    const current = this.options.find((o) => o.value === this.value);
    const filtered = this._filteredOptions;
    const activeId = this._open && this._activeIndex >= 0 &&
        this._activeIndex < filtered.length
      ? this._optionId(this._activeIndex)
      : undefined;
    return html`
      <button
        class="trigger"
        type="button"
        id="${this._triggerId}"
        name="${this.name || nothing}"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded="${this._open}"
        aria-controls="${this._listboxId}"
        aria-label="${this.placeholder}"
        aria-activedescendant="${activeId || nothing}"
        ?data-open="${this._open}"
        @click="${this._toggle}"
        @keydown="${this._onTriggerKeydown}"
      >
        <span class="trigger-label">
          ${current?.color
            ? html`
              <span class="dot" style="background:${current.color}"></span>
            `
            : ""}
          <span class="${current ? "truncate" : "placeholder truncate"}">
            ${current?.label ?? this.placeholder}
          </span>
        </span>
        <breeze-icon name="chevron-down" size="14"></breeze-icon>
      </button>
      ${this._open
        ? html`
          <div class="panel" role="listbox" id="${this._listboxId}">
            ${this.searchable
              ? html`
                <div class="search">
                  <breeze-icon name="search" size="14"></breeze-icon>
                  <input
                    type="text"
                    id="${this._searchInputId}"
                    role="combobox"
                    aria-haspopup="listbox"
                    aria-expanded="true"
                    aria-controls="${this._listboxId}"
                    aria-autocomplete="list"
                    aria-activedescendant="${activeId || nothing}"
                    placeholder="${msg("Search...")}"
                    .value="${this._search}"
                    @click="${(e: Event) => e.stopPropagation()}"
                    @input="${this._onSearchInput}"
                    @keydown="${this._onSearchKeydown}"
                  />
                </div>
              `
              : nothing}
            <div class="list">
              ${filtered.length === 0
                ? html`<div class="empty">${msg("No results")}</div>`
                : filtered.map(
                  (o, i) =>
                    html`
                      <button
                        class="option"
                        type="button"
                        role="option"
                        id="${this._optionId(i)}"
                        tabindex="-1"
                        aria-selected="${o.value === this.value}"
                        ?data-active="${this._open && i === this._activeIndex}"
                        @click="${() => this._select(o.value)}"
                        @mouseenter="${() => {
                          this._activeIndex = i;
                        }}"
                      >
                        ${o.color
                          ? html`
                            <span
                              class="dot"
                              style="background:${o.color}"
                            ></span>
                          `
                          : ""}
                        <span class="label">${o.label}</span>
                        ${o.value === this.value
                          ? html`
                            <breeze-icon name="check" size="14"></breeze-icon>
                          `
                          : ""}
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
    "breeze-select": BreezeSelect;
  }
}
