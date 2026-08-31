import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import { getTasks } from "@/api";
import type { DtoTaskListResponse } from "@/api";
import { fmtDate } from "@/lib/format/date";
import { loadWithMinTime } from "@/lib/async";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/select.ts";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/spinner.ts";
import "@/layouts/app-layout.ts";

function getPRIORITY_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "", label: msg("All") },
    { value: "urgent", label: msg("Urgent") },
    { value: "high", label: msg("High") },
    { value: "medium", label: msg("Medium") },
    { value: "low", label: msg("Low") },
    { value: "none", label: msg("None") },
  ];
}

function getGROUP_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "", label: msg("No grouping") },
    { value: "status", label: msg("Status") },
    { value: "priority", label: msg("Priority") },
    { value: "project", label: msg("Project") },
  ];
}

interface PriorityConfig {
  icon: string;
  label: string;
  order: number;
}

function getPRIORITY_CONFIG(): Record<string, PriorityConfig> {
  return {
    urgent: { icon: "arrow-up", label: msg("Urgent"), order: 0 },
    high: { icon: "arrow-up", label: msg("High"), order: 1 },
    medium: { icon: "minus", label: msg("Medium"), order: 2 },
    low: { icon: "arrow-down", label: msg("Low"), order: 3 },
    none: { icon: "minus", label: msg("None"), order: 4 },
  };
}

type SortField = "priority" | "due_at" | "title" | "updated_at" | "project";
type SortDir = "asc" | "desc";

function getSORT_HEADERS(): { field: SortField; label: string }[] {
  return [
    { field: "title", label: msg("Task") },
    { field: "priority", label: msg("Priority") },
    { field: "project", label: msg("Project") },
    { field: "due_at", label: msg("Due date") },
    { field: "updated_at", label: msg("Updated") },
  ];
}

@localized()
@customElement("plume-my-tasks-page")
export class PlumeMyTasksPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }
      .page {
        display: flex;
        flex-direction: column;
        height: 100%;
      }

      /* Header: matches members-page pattern */
      .page-head {
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
      }
      .page-head-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .page-head-left h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        font-family: var(--font-heading, inherit);
        color: var(--foreground);
      }
      .page-head-left p {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .page-head-right {
        display: flex;
        align-items: center;
        gap: var(--space-2);
      }
      .group-select {
        width: var(--space-36);
      }

      /* Toolbar: inside the scrollable content */
      .toolbar {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        margin-bottom: var(--space-4);
      }
      .search-input {
        flex: 1;
        max-width: var(--space-80);
      }
      .priority-filter {
        width: var(--space-32);
      }
      .task-count {
        margin-left: auto;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        white-space: nowrap;
      }

      /* Scrollable content area: matches members-page pattern */
      .page-content {
        flex: 1;
        padding: var(--space-6);
        overflow: auto;
        min-height: 0;
      }

      /* Table */
      .table {
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        overflow: hidden;
      }
      .table-header {
        display: grid;
        grid-template-columns: 1fr 100px 100px 120px 80px;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-4);
        background: color-mix(in oklch, var(--muted) 40%, transparent);
        border-bottom: 1px solid var(--border);
      }
      .sort-btn {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        border: none;
        background: transparent;
        font-size: var(--text-xs);
        font-weight: 500;
        font-family: inherit;
        color: var(--muted-foreground);
        cursor: pointer;
        padding: 0;
        text-align: left;
        transition: color var(--dur-fast) var(--ease-1);
      }
      .sort-btn:hover {
        color: var(--foreground);
      }
      .sort-btn .sort-icon-inactive {
        opacity: 0.3;
      }
      .table-row {
        width: 100%;
        display: grid;
        grid-template-columns: 1fr 100px 100px 120px 80px;
        gap: var(--space-2);
        align-items: center;
        padding: var(--space-2-5) var(--space-4);
        font-size: var(--text-sm);
        text-align: left;
        border: none;
        background: transparent;
        font-family: inherit;
        color: inherit;
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .table-row + .table-row {
        border-top: 1px solid var(--border);
      }
      .table-row:hover {
        background: var(--accent);
      }
      .table-row.completed {
        opacity: 0.5;
        transition: opacity var(--dur-normal) var(--ease-1);
      }

      /* Task cell */
      .task-cell {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        min-width: 0;
      }
      .status-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        flex-shrink: 0;
      }
      .task-title {
        font-weight: 500;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .completed-icon {
        color: light-dark(oklch(0.65 0.18 160), oklch(0.72 0.15 155));
        flex-shrink: 0;
      }

      /* Priority badge */
      .priority-badge {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        height: var(--space-5);
        padding: 0 var(--space-1-5);
        border: 1px solid var(--border);
        border-radius: var(--radius-full);
        font-size: var(--text-2xs, 11px);
        color: var(--foreground);
      }

      /* Misc cells */
      .cell-muted {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      /* Grouped view */
      .groups {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
      }
      .group-header {
        margin: 0 0 var(--space-2);
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .group-header .count {
        margin-left: var(--space-1);
        font-size: var(--text-xs);
      }
      .group-list {
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        overflow: hidden;
      }
      .group-list .table-row:first-child {
        /* no special top on first row in groups */
      }

      /* Empty state */
      .empty {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: var(--space-16) 0;
        color: var(--muted-foreground);
      }
      .empty-text {
        font-size: var(--text-sm);
        margin-top: var(--space-2);
      }

      /* Skeleton */
      .skeleton-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .skeleton-row {
        height: var(--space-12);
        border-radius: var(--radius-lg);
        background: var(--muted);
        animation: pulse var(--dur-slow) infinite;
      }
      .skeleton-row.skeleton-shimmer {
        background: linear-gradient(
          90deg,
          var(--muted) 0%,
          color-mix(in oklch, var(--muted) 60%, var(--foreground) 5%) 40%,
          var(--muted) 80%
        );
        background-size: 200% 100%;
        animation: shimmer var(--dur-slow) var(--ease-1) infinite;
      }
      @keyframes pulse {
        50% {
          opacity: 0.5;
        }
      }
    `,
  ];

  @state()
  private _search = "";

  @state()
  private _priority = "";

  @state()
  private _groupBy = "";

  @state()
  private _showCompleted = false;

  @state()
  private _sortField: SortField = "updated_at";

  @state()
  private _sortDir: SortDir = "desc";

  @state()
  private _tasks: DtoTaskListResponse[] = [];

  @state()
  private _loading = true;

  private _debounceTimer = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#load();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
  }

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has("_search")) {
      if (this._debounceTimer) clearTimeout(this._debounceTimer);
      this._debounceTimer = window.setTimeout(() => {
        this._debounceTimer = 0;
        this.#load();
      }, 200);
    }
    if (
      changed.has("_priority") ||
      changed.has("_groupBy") ||
      changed.has("_showCompleted")
    ) {
      this.#load();
    }
  }

  #buildQuery(): Record<string, unknown> {
    const query: Record<string, unknown> = { limit: 50 };
    if (this._search) query.q = this._search;
    if (this._priority) query.priority = this._priority;
    if (this._groupBy) query.group_by = this._groupBy;
    if (this._showCompleted) query.show_completed = true;
    return query;
  }

  async #load() {
    const isInitial = this._tasks.length === 0;

    if (isInitial) {
      await loadWithMinTime(
        async () => {
          const { data } = await getTasks({
            query: this.#buildQuery(),
            throwOnError: true,
          });
          this._tasks = (data?.items ?? []) as DtoTaskListResponse[];
        },
        (loading) => {
          this._loading = loading;
        },
        250,
      );
      return;
    }

    this._loading = true;
    try {
      const { data } = await getTasks({
        query: this.#buildQuery(),
        throwOnError: true,
      });
      this._tasks = (data?.items ?? []) as DtoTaskListResponse[];
    } catch {
      this._tasks = [];
    }
    this._loading = false;
  }

  #toggleSort(field: SortField) {
    if (this._sortField === field) {
      this._sortDir = this._sortDir === "asc" ? "desc" : "asc";
    } else {
      this._sortField = field;
      this._sortDir = "asc";
    }
  }

  #clearFilters() {
    this._search = "";
    this._priority = "";
    this._groupBy = "";
    this._showCompleted = false;
  }

  get #sorted(): DtoTaskListResponse[] {
    const items = [...this._tasks];
    const dir = this._sortDir === "asc" ? 1 : -1;
    items.sort((a, b) => {
      let cmp = 0;
      switch (this._sortField) {
        case "priority":
          cmp = (getPRIORITY_CONFIG()[a.priority ?? "none"]?.order ?? 4) -
            (getPRIORITY_CONFIG()[b.priority ?? "none"]?.order ?? 4);
          break;
        case "due_at": {
          const da = a.due_at
            ? new Date(a.due_at).getTime()
            : Number.MAX_SAFE_INTEGER;
          const db = b.due_at
            ? new Date(b.due_at).getTime()
            : Number.MAX_SAFE_INTEGER;
          cmp = da - db;
          break;
        }
        case "title":
          cmp = (a.title ?? "").localeCompare(b.title ?? "");
          break;
        case "project":
          cmp = (a.project_id ?? "").localeCompare(b.project_id ?? "");
          break;
        case "updated_at":
        default:
          cmp = new Date(a.updated_at ?? 0).getTime() -
            new Date(b.updated_at ?? 0).getTime();
          break;
      }
      return cmp * dir;
    });
    return items;
  }

  get #grouped():
    | Map<string, { label: string; items: DtoTaskListResponse[] }>
    | null {
    if (!this._groupBy) return null;
    const map = new Map<
      string,
      { label: string; items: DtoTaskListResponse[] }
    >();
    for (const t of this.#sorted) {
      let key = "";
      let label = "";
      switch (this._groupBy) {
        case "status":
          key = t.status_id ?? "";
          label = t.status_name ?? key;
          break;
        case "priority":
          key = t.priority ?? "none";
          label = getPRIORITY_CONFIG()[key]?.label ?? "None";
          break;
        case "project":
          key = t.project_id ?? "";
          label = t.project_name ?? key;
          break;
      }
      if (!map.has(key)) map.set(key, { label, items: [] });
      map.get(key)!.items.push(t);
    }
    return map;
  }

  #openTask(task: DtoTaskListResponse) {
    navigate(
      `/projects/${task.project_slug ?? task.project_id}?task=${task.id}`,
    );
  }

  #renderSortHeader(field: SortField, label: string) {
    const active = this._sortField === field;
    return html`
      <button class="sort-btn" type="button" @click="${() =>
        this.#toggleSort(field)}">
        ${label} ${active
          ? html`
            <plume-icon name="${this._sortDir === "asc"
              ? "arrow-up"
              : "arrow-down"}" size="12"></plume-icon>
          `
          : html`
            <plume-icon
              class="sort-icon-inactive"
              name="arrow-up-down"
              size="12"
            ></plume-icon>
          `}
      </button>
    `;
  }

  #renderTaskRow(task: DtoTaskListResponse) {
    const pc = getPRIORITY_CONFIG()[task.priority ?? "none"] ??
      getPRIORITY_CONFIG().none;
    const dueDate = task.due_at ? fmtDate(task.due_at) : "\u2014";
    const updated = task.updated_at ? fmtDate(task.updated_at) : "\u2014";

    return html`
      <button
        type="button"
        class="table-row ${task.completed_at ? "completed" : ""}"
        @click="${() => this.#openTask(task)}"
      >
        <div class="task-cell">
          <span class="status-dot" style="background:${task.status_color ??
            "#888"}"></span>
          <span class="task-title">
            ${task.completed_at
              ? html`
                <plume-icon class="completed-icon" name="check-circle" size="14"></plume-icon>
              `
              : nothing} ${task.title ?? msg("Untitled")}
          </span>
        </div>
        <span class="priority-badge">
          <plume-icon name="${pc.icon}" size="10"></plume-icon>
          ${pc.label}
        </span>
        <span class="cell-muted">${task.project_name ?? task.project_id}</span>
        <span class="cell-muted">${dueDate}</span>
        <span class="cell-muted">${updated}</span>
      </button>
    `;
  }

  #renderFlat() {
    return html`
      <div class="table">
        <div class="table-header">
          ${getSORT_HEADERS().map((h) =>
            this.#renderSortHeader(h.field, h.label)
          )}
        </div>
        ${this.#sorted.map((t) => this.#renderTaskRow(t))}
      </div>
    `;
  }

  #renderGrouped() {
    const grouped = this.#grouped;
    if (!grouped) return nothing;
    return html`
      <div class="groups">
        ${[...grouped.entries()].map(([key, group]) =>
          html`
            <div>
              <h2 class="group-header">
                ${key ? group.label : msg("No group")}
                <span class="count">(${group.items.length})</span>
              </h2>
              <div class="group-list">
                ${group.items.map((t) => this.#renderTaskRow(t))}
              </div>
            </div>
          `
        )}
      </div>
    `;
  }

  protected render() {
    const hasFilters = this._search || this._priority || this._showCompleted ||
      this._groupBy;

    return html`
      <plume-app-layout>
        <div class="page page-enter">
          <div class="page-head">
            <div class="page-head-row">
              <div class="page-head-left">
                <h1>${msg("My Tasks")}</h1>
                <p>${msg("Tasks assigned to you across all projects.")}</p>
              </div>
              <div class="page-head-right">
                <plume-button
                  variant="${this._showCompleted ? "" : "outline"}"
                  size="sm"
                  @click="${() => {
                    this._showCompleted = !this._showCompleted;
                  }}"
                >
                  <plume-icon name="check-circle" size="16"></plume-icon>
                  ${msg("Completed")}
                </plume-button>
                <plume-select
                  class="group-select"
                  .options="${getGROUP_OPTIONS()}"
                  .value="${this._groupBy}"
                  placeholder="${msg("No grouping")}"
                  @change="${(e: CustomEvent) => {
                    this._groupBy = e.detail as string;
                  }}"
                ></plume-select>
              </div>
            </div>
          </div>

          <div class="page-content">
            <div class="toolbar">
              <plume-input
                class="search-input"
                type="search"
                placeholder="${msg("Search tasks...")}"
                .value="${this._search}"
                @input="${(e: Event) => {
                  this._search = (e.target as HTMLInputElement).value;
                }}"
              ></plume-input>
              <plume-select
                class="priority-filter"
                .options="${getPRIORITY_OPTIONS()}"
                .value="${this._priority}"
                placeholder="${msg("All")}"
                @change="${(e: CustomEvent) => {
                  this._priority = e.detail as string;
                }}"
              ></plume-select>
              ${hasFilters
                ? html`
                  <plume-button variant="ghost" size="sm" @click="${this
                    .#clearFilters}">
                    <plume-icon name="x" size="16"></plume-icon>
                    ${msg("Clear")}
                  </plume-button>
                `
                : nothing}
              <span class="task-count">
                ${this._loading
                  ? html`
                    <plume-spinner></plume-spinner>
                  `
                  : html`
                    ${this._tasks.length} ${this._tasks.length === 1
                      ? msg("task")
                      : msg("tasks")}
                  `}
              </span>
            </div>

            ${this._loading && this._tasks.length === 0
              ? html`
                <div class="skeleton-list">
                  ${Array.from({ length: 5 }).map(() =>
                    html`
                      <div class="skeleton-row skeleton-shimmer"></div>
                    `
                  )}
                </div>
              `
              : this._groupBy
              ? this.#renderGrouped()
              : this.#sorted.length === 0
              ? html`
                <div class="empty">
                  <plume-icon name="list" size="32"></plume-icon>
                  <span class="empty-text">
                    ${hasFilters
                      ? msg("No tasks match your filters.")
                      : msg("No tasks assigned to you.")}
                  </span>
                </div>
              `
              : this.#renderFlat()}
          </div>
        </div>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-my-tasks-page": PlumeMyTasksPage;
  }
}
