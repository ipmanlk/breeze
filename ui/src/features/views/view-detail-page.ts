import { localized, msg } from "@lit/localize";
import { getLocale } from "@/i18n";
import { logError } from "@/lib/log";
import { css, html, LitElement } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { currentPath, navigate } from "@/routes/router";
import { deleteView, fetchView } from "./store";
import type { View, ViewFilters } from "./types";
import { activeFilterEntries, humanizeKey, humanizeValue } from "./types";
import { getTasks } from "@/api";
import type { DtoTaskListResponse } from "@/api";
import "../../layouts/app-layout.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/skeleton.ts";
import "./components/save-view-dialog.ts";

type SortField = "priority" | "due_at" | "title" | "updated_at" | "project";
type SortDir = "asc" | "desc";

function toTaskParams(filters: ViewFilters): Record<string, string> {
  const params: Record<string, string> = {};
  if (filters.search) params.q = filters.search;
  if (filters.priority) params.priority = filters.priority;
  if (filters.status_id) params.status_id = filters.status_id;
  if (filters.cycle_id) params.cycle_id = filters.cycle_id;
  return params;
}

/**
 * View Detail Page: displays tasks for a saved view with sorting.
 */
@localized()
@customElement("breeze-view-detail-page")
export class BreezeViewDetailPage extends LitElement {
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
        flex: 1;
        overflow: hidden;
      }
      .header {
        display: flex;
        align-items: center;
        gap: var(--space-4);
        border-bottom: 1px solid var(--border);
        padding: var(--space-3) var(--space-6);
        flex-shrink: 0;
      }
      .header h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
      }
      .header .count {
        margin-left: auto;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .filters-bar {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        border-bottom: 1px solid var(--border);
        padding: var(--space-2) var(--space-6);
        flex-shrink: 0;
      }
      .filter-tag {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-1) var(--space-2);
        border-radius: var(--radius-full);
        border: 1px solid var(--border);
        background: var(--muted);
        font-size: var(--text-xs);
      }
      .filter-tag .key {
        color: var(--muted-foreground);
      }
      .content {
        flex: 1;
        overflow-y: auto;
        padding: var(--space-6);
      }
      .table {
        width: 100%;
        border-radius: var(--radius-lg);
        border: 1px solid var(--border);
        overflow: hidden;
      }
      .table-header {
        display: grid;
        grid-template-columns: 1fr 100px 100px 120px 80px;
        gap: var(--space-2);
        padding: var(--space-2) var(--space-4);
        border-bottom: 1px solid var(--border);
        background: color-mix(in oklch, var(--muted) 40%, transparent);
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--muted-foreground);
      }
      .table-header button {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: 0;
        border: none;
        background: transparent;
        color: inherit;
        font: inherit;
        cursor: pointer;
        text-align: left;
      }
      .table-header button:hover {
        color: var(--foreground);
      }
      .table-row {
        display: grid;
        grid-template-columns: 1fr 100px 100px 120px 80px;
        gap: var(--space-2);
        align-items: center;
        padding: var(--space-2-5) var(--space-4);
        border-bottom: 1px solid var(--border);
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .table-row:last-child {
        border-bottom: none;
      }
      .table-row:hover {
        background: color-mix(in oklch, var(--muted) 50%, transparent);
      }
      .table-row.completed {
        opacity: 0.5;
      }
      .task-cell {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        min-width: 0;
      }
      .task-status-dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        flex-shrink: 0;
      }
      .task-title {
        font-size: var(--text-sm);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }
      .priority-cell {
        font-size: var(--text-xs);
      }
      .project-cell,
      .date-cell {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }
      .empty {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: var(--space-8);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      .loading {
        display: flex;
        justify-content: center;
        padding: var(--space-8);
      }
      .skeleton {
        height: var(--space-12);
        border-radius: var(--radius-lg);
        background: var(--muted);
        animation: pulse var(--dur-slow) cubic-bezier(0.4, 0, 0.6, 1) infinite;
      }
      .kbd-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--space-6);
        height: var(--space-6);
        border: none;
        border-radius: var(--radius-md);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
      }
      .kbd-btn:hover {
        background: var(--muted);
        color: var(--foreground);
      }
      .kbd-menu {
        min-width: var(--space-36);
        padding: var(--space-1);
      }
      .kbd-item {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-1-5) var(--space-2);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: left;
        cursor: pointer;
      }
      .kbd-item:hover {
        background: var(--accent);
      }
      .kbd-item.destructive:hover {
        background: color-mix(in oklch, var(--destructive) 15%, transparent);
        color: var(--destructive);
      }
      @keyframes pulse {
        0%,
        100% {
          opacity: 1;
        }
        50% {
          opacity: 0.5;
        }
      }
      .sort-icon-inactive {
        opacity: 0.3;
      }
    `,
  ];

  @state()
  private _view: View | null = null;

  @state()
  private _tasks: DtoTaskListResponse[] = [];

  @state()
  private _isLoading = true;

  @state()
  private _sortField: SortField = "updated_at";

  @state()
  private _sortDir: SortDir = "desc";

  @state()
  private _showEditDialog = false;

  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(currentPath);
    this._loadView();
  }

  private async _loadView(): Promise<void> {
    const path = currentPath.value;
    const match = path.match(/^\/views\/([^/]+)/);
    const id = match?.[1] ?? "";

    if (!id) {
      navigate("/views");
      return;
    }

    this._isLoading = true;
    const view = await fetchView(id);
    if (!view) {
      navigate("/views");
      return;
    }

    this._view = view;
    await this._fetchTasks(view);
    this._isLoading = false;
  }

  private async _fetchTasks(view: View): Promise<void> {
    try {
      const { data } = await getTasks({
        query: toTaskParams(view.filters),
        throwOnError: true,
      });
      this._tasks = (data?.items as DtoTaskListResponse[]) ?? [];
    } catch (err) {
      logError("fetchTasks failed:", err);
      this._tasks = [];
    }
  }

  private _getSortedTasks(): DtoTaskListResponse[] {
    const dir = this._sortDir === "asc" ? 1 : -1;
    const arr = [...this._tasks];

    arr.sort((a, b) => {
      let cmp = 0;
      switch (this._sortField) {
        case "title":
          cmp = (a.title ?? "").localeCompare(b.title ?? "");
          break;
        case "priority": {
          const ranks: Record<string, number> = {
            urgent: 0,
            high: 1,
            medium: 2,
            low: 3,
            none: 4,
          };
          cmp = (ranks[a.priority ?? "none"] ?? 4) -
            (ranks[b.priority ?? "none"] ?? 4);
          break;
        }
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
        case "project":
          cmp = (a.project_name ?? "").localeCompare(b.project_name ?? "");
          break;
        case "updated_at":
        default:
          cmp = new Date(a.updated_at ?? 0).getTime() -
            new Date(b.updated_at ?? 0).getTime();
          break;
      }
      return dir * cmp;
    });

    return arr;
  }

  private _toggleSort(field: SortField): void {
    if (this._sortField === field) {
      this._sortDir = this._sortDir === "asc" ? "desc" : "asc";
    } else {
      this._sortField = field;
      this._sortDir = "asc";
    }
  }

  private _navigateToTask(task: DtoTaskListResponse): void {
    if (task.project_slug) {
      navigate(`/projects/${task.project_slug}?task=${task.id}`);
    }
  }

  private _renderSortIcon(field: SortField): ReturnType<typeof html> {
    if (this._sortField !== field) {
      return html`
        <breeze-icon name="arrow-up-down" size="12"
          class="sort-icon-inactive"></breeze-icon>
      `;
    }
    return html`
      <breeze-icon
        name="${this._sortDir === "asc" ? "arrow-up" : "arrow-down"}"
        size="12"
      ></breeze-icon>
    `;
  }

  private async _handleDelete(): Promise<void> {
    if (!this._view) return;
    const ok = await deleteView(this._view.id);
    if (ok) {
      navigate("/views");
    }
  }

  private _renderHeader(): ReturnType<typeof html> {
    return html`
      <div class="table-header">
        <button @click="${() => this._toggleSort("title")}">
          ${msg("Task")} ${this._renderSortIcon("title")}
        </button>
        <button @click="${() => this._toggleSort("priority")}">
          ${msg("Priority")} ${this._renderSortIcon("priority")}
        </button>
        <button @click="${() => this._toggleSort("project")}">
          ${msg("Project")} ${this._renderSortIcon("project")}
        </button>
        <button @click="${() => this._toggleSort("due_at")}">
          ${msg("Due date")} ${this._renderSortIcon("due_at")}
        </button>
        <button @click="${() => this._toggleSort("updated_at")}">
          ${msg("Updated")} ${this._renderSortIcon("updated_at")}
        </button>
      </div>
    `;
  }

  private _renderTaskRow(task: DtoTaskListResponse): ReturnType<typeof html> {
    const dueDate = task.due_at
      ? new Date(task.due_at).toLocaleDateString(getLocale(), {
        month: "short",
        day: "numeric",
      })
      : "—";
    const updated = task.updated_at
      ? new Date(task.updated_at).toLocaleDateString(getLocale(), {
        month: "short",
        day: "numeric",
      })
      : "—";

    const priorityColors: Record<string, string> = {
      urgent: "var(--destructive)",
      high: "var(--orange-500)",
      medium: "var(--yellow-500)",
      low: "var(--blue-500)",
      none: "var(--muted-foreground)",
    };

    return html`
      <div
        class="table-row ${task.completed_at ? "completed" : ""}"
        @click="${() => this._navigateToTask(task)}"
      >
        <div class="task-cell">
          <span
            class="task-status-dot"
            style="background: ${task.status_color ?? "#888"}"
          ></span>
          <span class="task-title">${task.title ?? msg("Untitled")}</span>
        </div>
        <div
          class="priority-cell"
          style="color: ${priorityColors[task.priority ?? "none"]}"
        >
          ${task.priority ?? msg("None")}
        </div>
        <div class="project-cell">${task.project_name ?? "—"}</div>
        <div class="date-cell">${dueDate}</div>
        <div class="date-cell">${updated}</div>
      </div>
    `;
  }

  protected render() {
    if (this._isLoading || !this._view) {
      return html`
        <breeze-app-layout>
          <div class="page page-enter">
            <div class="header">
              <breeze-skeleton
                variant="text"
                count="3"
                height="1.5rem"
              ></breeze-skeleton>
            </div>
          </div>
        </breeze-app-layout>
      `;
    }

    const sortedTasks = this._getSortedTasks();
    const activeFilters = activeFilterEntries(this._view.filters);

    return html`
      <breeze-app-layout>
        <div class="page page-enter">
          <div class="header">
            <breeze-button
              variant="ghost"
              size="sm"
              @click="${() => navigate("/views")}"
            >
              <breeze-icon name="arrow-left" size="16"></breeze-icon>
            </breeze-button>
            <breeze-icon
              name="${this._view.layout === "board" ? "layout-grid" : "list"}"
              size="18"
            ></breeze-icon>
            <h1>${this._view.name}</h1>
            <span class="count">${sortedTasks.length} ${sortedTasks.length === 1
              ? msg("task")
              : msg("tasks")}</span>

            <breeze-popover placement="bottom-end">
              <button slot="trigger" class="kbd-btn" title="${msg("Actions")}">
                <breeze-icon name="more-horizontal" size="16"></breeze-icon>
              </button>
              <div slot="content" class="kbd-menu">
                <button
                  class="kbd-item"
                  @click="${() => (this._showEditDialog = true)}"
                >
                  <breeze-icon name="pencil" size="14"></breeze-icon>
                  ${msg("Edit")}
                </button>
                <button
                  class="kbd-item destructive"
                  @click="${this._handleDelete}"
                >
                  <breeze-icon name="trash-2" size="14"></breeze-icon>
                  ${msg("Delete")}
                </button>
              </div>
            </breeze-popover>
          </div>

          ${activeFilters.length > 0
            ? html`
              <div class="filters-bar">
                ${activeFilters.map(
                  ([k, v]) =>
                    html`
                      <span class="filter-tag"><span class="key">${humanizeKey(
                        k,
                      )}:</span>
                        ${humanizeValue(k, v)}</span>
                    `,
                )}
              </div>
            `
            : null}

          <div class="content">
            ${sortedTasks.length === 0
              ? html`
                <div class="empty">${msg(
                  "No tasks match this view's filters.",
                )}</div>
              `
              : html`
                <div class="table">
                  ${this._renderHeader()}
                  <div class="table-body">
                    ${sortedTasks.map((task) => this._renderTaskRow(task))}
                  </div>
                </div>
              `}
          </div>
        </div>
      </breeze-app-layout>

      ${this._view
        ? html`
          <breeze-save-view-dialog
            .open="${this._showEditDialog}"
            .viewId="${this._view.id}"
            .viewName="${this._view.name}"
            .viewLayout="${this._view.layout}"
            .existingFilters="${this._view.filters}"
            @close="${() => (this._showEditDialog = false)}"
            @view-updated="${() => {
              this._showEditDialog = false;
              void this._loadView();
            }}"
            @view-deleted="${() => navigate("/views")}"
          ></breeze-save-view-dialog>
        `
        : null}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-view-detail-page": BreezeViewDetailPage;
  }
}
