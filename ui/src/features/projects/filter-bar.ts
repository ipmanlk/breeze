import { css, html, LitElement } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { getLabels, getProjectsByIdMembers } from "@/api";
import type { DtoLabelResponse, DtoTaskStatusResponse } from "@/api";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/avatar.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/label-chip.ts";
import { localized, msg } from "@lit/localize";

export interface ProjectFilters {
  search: string;
  priority: string;
  status_id: string;
  assignee_id: string | null;
  label_ids: string[];
}

function getPriorities(): {
  value: string;
  label: string;
  color: string;
  dotColor: string;
}[] {
  return [
    {
      value: "urgent",
      label: msg("Urgent"),
      color: "var(--destructive)",
      dotColor: "var(--destructive)",
    },
    {
      value: "high",
      label: msg("High"),
      color: "oklch(0.7 0.15 60)",
      dotColor: "oklch(0.7 0.15 60)",
    },
    {
      value: "medium",
      label: msg("Medium"),
      color: "oklch(0.75 0.12 85)",
      dotColor: "oklch(0.75 0.12 85)",
    },
    {
      value: "low",
      label: msg("Low"),
      color: "oklch(0.65 0.1 240)",
      dotColor: "oklch(0.65 0.1 240)",
    },
    {
      value: "none",
      label: msg("No priority"),
      color: "var(--muted-foreground)",
      dotColor: "var(--muted-foreground)",
    },
  ];
}

/**
 * Filter Bar: project task filtering with search, priority, status, assignee.
 */
@localized()
@customElement("breeze-filter-bar")
export class BreezeFilterBar extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--space-2);
    }
    .search-box {
      position: relative;
      width: var(--space-60);
    }
    .search-box breeze-icon {
      position: absolute;
      left: var(--space-2);
      top: 50%;
      transform: translateY(-50%);
      color: var(--muted-foreground);
      pointer-events: none;
    }
    .search-box input {
      width: 100%;
      height: var(--space-7);
      padding: 0 var(--space-2) 0 var(--space-7);
      border-radius: var(--radius-md);
      border: 1px solid var(--input);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-xs);
      outline: none;
    }
    .search-box input:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .search-clear {
      position: absolute;
      right: var(--space-1-5);
      top: 50%;
      transform: translateY(-50%);
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-md);
      color: var(--muted-foreground);
      cursor: pointer;
      border: none;
      background: transparent;
    }
    .search-clear:hover {
      color: var(--foreground);
    }
    .filter-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      height: var(--space-7);
      padding: 0 var(--space-2);
      border-radius: var(--radius-md);
      border: 1px solid var(--border);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-xs);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .filter-btn.tag-enter {
      animation: list-item-in var(--dur-normal) var(--ease-2);
    }
    .filter-btn.tag-exit {
      animation: content-out var(--dur-fast) var(--ease-3);
    }
    .filter-btn:hover {
      background: var(--accent);
    }
    .filter-btn .label {
      color: var(--muted-foreground);
    }
    .filter-btn .value {
      color: var(--foreground);
    }
    .filter-btn .dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
    }
    .filter-btn .filter-chips {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
    }
    .filter-btn .filter-chips-more {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .filter-btn .filter-clear {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border-radius: var(--radius-sm);
      padding: 1px;
      margin-left: calc(var(--space-1) * -0.5);
      color: var(--muted-foreground);
      transition: color var(--dur-fast) var(--ease-1),
        background var(--dur-fast) var(--ease-1);
    }
    .filter-btn .filter-clear:hover {
      color: var(--foreground);
      background: var(--accent);
    }
    .filter-btn breeze-icon {
      width: var(--space-3);
      height: var(--space-3);
      color: var(--muted-foreground);
    }
    .clear-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      height: var(--space-7);
      padding: 0 var(--space-2);
      border-radius: var(--radius-md);
      border: none;
      background: transparent;
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      cursor: pointer;
    }
    .clear-btn:hover {
      color: var(--foreground);
    }
    .popover-content {
      min-width: var(--space-44);
      max-height: var(--space-48);
      overflow-y: auto;
    }
    .popover-content.assignee-popover {
      width: var(--filter-w);
    }
    .popover-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border-radius: var(--radius-sm);
      border: none;
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      text-align: left;
      cursor: pointer;
    }
    .popover-item:hover {
      background: var(--accent);
    }
    .popover-item .dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .popover-item .check {
      margin-left: auto;
      width: var(--space-3);
      height: var(--space-3);
      color: var(--primary);
    }
    .popover-item .name {
      flex: 1;
    }
    .member-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      flex: 1;
    }
    .member-item breeze-avatar {
      width: var(--space-6);
      height: var(--space-6);
      font-size: var(--text-2xs);
    }
    .member-name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  `;

  @property()
  projectId = "";

  @property({ attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @property({ attribute: false })
  filters: ProjectFilters = {
    search: "",
    priority: "",
    status_id: "",
    assignee_id: null,
    label_ids: [],
  };

  @state()
  private _searchInput = "";

  @state()
  private _members: { id: string; name: string | null }[] = [];

  @state()
  private _labels: DtoLabelResponse[] = [];

  private _debounceTimer?: ReturnType<typeof setTimeout>;

  connectedCallback(): void {
    super.connectedCallback();
    this._searchInput = this.filters.search;
    this._fetchMembers();
    this._fetchLabels();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
      this._debounceTimer = undefined;
    }
  }

  private async _fetchLabels(): Promise<void> {
    try {
      const { data } = await getLabels({ throwOnError: true });
      this._labels = data ?? [];
    } catch {
      this._labels = [];
    }
  }

  private async _fetchMembers(): Promise<void> {
    if (!this.projectId) return;
    try {
      const { data } = await getProjectsByIdMembers({
        path: { id: this.projectId },
        query: { limit: 100 },
        throwOnError: true,
      });
      this._members =
        ((data as { items?: { id: string; name: string | null }[] })?.items) ??
          [];
    } catch {
      this._members = [];
    }
  }

  private _updateFilters(partial: Partial<ProjectFilters>): void {
    this.filters = { ...this.filters, ...partial };
    this.dispatchEvent(
      new CustomEvent("filters-change", {
        detail: { filters: this.filters },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onSearchInput(e: Event): void {
    const value = (e.target as HTMLInputElement).value;
    this._searchInput = value;

    clearTimeout(this._debounceTimer);
    this._debounceTimer = setTimeout(() => {
      this._updateFilters({ search: value });
    }, 200);
  }

  private _clearSearch(): void {
    this._searchInput = "";
    this._updateFilters({ search: "" });
  }

  private _onKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape") {
      this._clearSearch();
    }
    if (e.key === "Enter") {
      this._updateFilters({ search: this._searchInput });
    }
  }

  private _clearAll(): void {
    this._searchInput = "";
    this._updateFilters({
      search: "",
      priority: "",
      status_id: "",
      assignee_id: null,
      label_ids: [],
    });
  }

  private _getActiveCount(): number {
    let count = 0;
    if (this.filters.search) count++;
    if (this.filters.priority) count++;
    if (this.filters.status_id) count++;
    if (this.filters.assignee_id) count++;
    if (this.filters.label_ids.length > 0) count++;
    return count;
  }

  private _renderPriorityFilter(): ReturnType<typeof html> {
    const priorities = getPriorities();
    const selected = priorities.find((p) => p.value === this.filters.priority);

    return html`
      <breeze-popover>
        <button slot="trigger" class="filter-btn">
          ${selected
            ? html`
              <span class="dot" style="background: ${selected.dotColor}"></span>
              <span class="value" style="color: ${selected.color}">${selected
                .label}</span>
              <span
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._updateFilters({ priority: "" });
                }}"
              >
                <breeze-icon name="x" size="12"></breeze-icon>
              </span>
            `
            : html`
              <span class="label">${msg("Priority")}</span>
              <breeze-icon name="chevron-down" size="12"></breeze-icon>
            `}
        </button>
        <div slot="content" class="popover-content">
          ${priorities.map(
            (p) =>
              html`
                <button
                  class="popover-item"
                  @click="${() =>
                    this._updateFilters({
                      priority: p.value === this.filters.priority
                        ? ""
                        : p.value,
                    })}"
                >
                  <span class="dot" style="background: ${p.dotColor}"></span>
                  <span class="name" style="color: ${p.color}">${p.label}</span>
                  ${p.value === this.filters.priority
                    ? html`
                      <breeze-icon class="check" name="check" size="14"></breeze-icon>
                    `
                    : null}
                </button>
              `,
          )}
        </div>
      </breeze-popover>
    `;
  }

  private _renderStatusFilter(): ReturnType<typeof html> {
    const selected = this.statuses.find((s) => s.id === this.filters.status_id);

    return html`
      <breeze-popover>
        <button slot="trigger" class="filter-btn">
          ${selected
            ? html`
              <span
                class="dot"
                style="background: ${selected.color}"
              ></span>
              <span class="value">${selected.name}</span>
              <span
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._updateFilters({ status_id: "" });
                }}"
              >
                <breeze-icon name="x" size="12"></breeze-icon>
              </span>
            `
            : html`
              <span class="label">${msg("Status")}</span>
              <breeze-icon name="chevron-down" size="12"></breeze-icon>
            `}
        </button>
        <div slot="content" class="popover-content">
          ${this.statuses.map(
            (s) =>
              html`
                <button
                  class="popover-item"
                  @click="${() =>
                    this._updateFilters({
                      status_id: s.id === this.filters.status_id ? "" : s.id,
                    })}"
                >
                  <span class="dot" style="background: ${s.color}"></span>
                  <span class="name">${s.name}</span>
                  ${s.id === this.filters.status_id
                    ? html`
                      <breeze-icon class="check" name="check" size="14"></breeze-icon>
                    `
                    : null}
                </button>
              `,
          )}
        </div>
      </breeze-popover>
    `;
  }

  private _renderAssigneeFilter(): ReturnType<typeof html> {
    const selected = this._members.find((m) =>
      m.id === this.filters.assignee_id
    );

    return html`
      <breeze-popover>
        <button slot="trigger" class="filter-btn">
          ${selected
            ? html`
              <span class="value">${selected.name?.split(" ")[0]}</span>
              <span
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._updateFilters({ assignee_id: null });
                }}"
              >
                <breeze-icon name="x" size="12"></breeze-icon>
              </span>
            `
            : html`
              <breeze-icon name="user" size="12"></breeze-icon>
              <span class="label">${msg("Assignee")}</span>
              <breeze-icon name="chevron-down" size="12"></breeze-icon>
            `}
        </button>
        <div slot="content" class="popover-content assignee-popover">
          <div class="popover-scroll">
            ${this._members.map(
              (m) =>
                html`
                  <button
                    class="popover-item"
                    @click="${() =>
                      this._updateFilters({
                        assignee_id: m.id === this.filters.assignee_id
                          ? null
                          : m.id,
                      })}"
                  >
                    <div class="member-item">
                      <breeze-avatar .name="${m.name ??
                        "?"}" size="sm"></breeze-avatar>
                      <span class="member-name">${m.name ??
                        msg("Unknown")}</span>
                    </div>
                    ${m.id === this.filters.assignee_id
                      ? html`
                        <breeze-icon class="check" name="check" size="14"></breeze-icon>
                      `
                      : null}
                  </button>
                `,
            )}
          </div>
          ${this._members.length === 0
            ? html`
              <div class="popover-item" disabled>${msg("No members")}</div>
            `
            : null}
        </div>
      </breeze-popover>
    `;
  }

  private _toggleLabelFilter(id: string): void {
    const set = new Set(this.filters.label_ids);
    if (set.has(id)) set.delete(id);
    else set.add(id);
    this._updateFilters({ label_ids: [...set] });
  }

  private _renderLabelFilter(): ReturnType<typeof html> {
    const selectedLabels = this._labels.filter((l) =>
      this.filters.label_ids.includes(l.id ?? "")
    );

    return html`
      <breeze-popover close-on-select="false">
        <button slot="trigger" class="filter-btn">
          ${selectedLabels.length > 0
            ? html`
              <span class="label">${msg("Labels:")}</span>
              <div class="filter-chips">
                ${selectedLabels.slice(0, 2).map(
                  (l) =>
                    html`
                      <breeze-label-chip
                        .label="${l}"
                      ></breeze-label-chip>
                    `,
                )}
                ${selectedLabels.length > 2
                  ? html`<span class="filter-chips-more">+${
                    selectedLabels.length - 2
                  }</span>`
                  : null}
              </div>
              <breeze-icon name="chevron-down" size="12"></breeze-icon>
              <span
                class="filter-clear"
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._updateFilters({ label_ids: [] });
                }}"
              >
                <breeze-icon name="x" size="12"></breeze-icon>
              </span>
            `
            : html`
              <breeze-icon name="tag" size="12"></breeze-icon>
              <span class="label">${msg("Labels")}</span>
              <breeze-icon name="chevron-down" size="12"></breeze-icon>
            `}
        </button>
        <div slot="content" class="popover-content">
          ${this._labels.length === 0
            ? html`<div class="popover-item" disabled>
                ${msg("No labels — create some in Settings → Labels")}
              </div>`
            : this._labels.map(
              (l) => {
                const checked = this.filters.label_ids.includes(l.id ?? "");
                return html`
                  <button
                    class="popover-item"
                    @click="${() => this._toggleLabelFilter(l.id ?? "")}"
                  >
                    <span
                      class="dot"
                      style="background: ${l.color ?? "#6366f1"}"
                    ></span>
                    <span class="name">${l.name}</span>
                    ${checked
                      ? html`
                        <breeze-icon class="check" name="check" size="14"></breeze-icon>
                      `
                      : null}
                  </button>
                `;
              },
            )}
        </div>
      </breeze-popover>
    `;
  }

  protected render() {
    const activeCount = this._getActiveCount();

    return html`
      <div class="search-box">
        <breeze-icon name="search" size="14"></breeze-icon>
        <input
          type="text"
          placeholder=${msg("Search tasks...")}
          .value="${this._searchInput}"
          @input="${this._onSearchInput}"
          @keydown="${this._onKeydown}"
        />
        ${this._searchInput
          ? html`
            <button class="search-clear" @click="${this._clearSearch}">
              <breeze-icon name="x" size="12"></breeze-icon>
            </button>
          `
          : null}
      </div>

      ${this._renderPriorityFilter()} ${this._renderStatusFilter()} ${this
        ._renderAssigneeFilter()} ${this._renderLabelFilter()} ${activeCount > 0
        ? html`
          <button class="clear-btn" @click="${this._clearAll}">
            <breeze-icon name="x" size="12"></breeze-icon>
            Clear
          </button>
        `
        : null}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-filter-bar": BreezeFilterBar;
  }
}
