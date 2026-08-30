import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { fmtDate } from "@/lib/format/date";
import type { DtoTaskResponse, DtoTaskStatusResponse } from "@/api";
import {
  batchUpdateTasks,
  projectDetail,
  updateTask,
} from "@/store/project-detail";
import { showToast } from "@/components/ui/toast-store";
import { SignalController } from "@/lib/signal-controller";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/label-chip.ts";
import "../../components/ui/button.ts";
import { localized, msg } from "@lit/localize";

type SortField = "title" | "status" | "priority" | "due_at";
type SortDir = "asc" | "desc";

const PRIORITY_RANK: Record<string, number> = {
  urgent: 4,
  high: 3,
  medium: 2,
  low: 1,
  none: 0,
};

const PRIORITIES: { value: string; label: string; color: string }[] = [
  {
    value: "none",
    label: msg("No priority"),
    color: "var(--muted-foreground)",
  },
  { value: "low", label: msg("Low"), color: "oklch(0.65 0.12 240)" },
  { value: "medium", label: msg("Medium"), color: "oklch(0.75 0.14 85)" },
  { value: "high", label: msg("High"), color: "oklch(0.68 0.16 55)" },
  { value: "urgent", label: msg("Urgent"), color: "var(--destructive)" },
];

const HEADERS: { field: SortField; label: string }[] = [
  { field: "title", label: msg("Title") },
  { field: "status", label: msg("Status") },
  { field: "priority", label: msg("Priority") },
  { field: "due_at", label: msg("Due") },
];

/**
 * List view: sortable task table with inline status / priority editors.
 *
 * Sorting is owned by the parent (URL-synced `sortField`/`sortDir`); this
 * component sorts client-side from the passed `tasks` and dispatches
 * `sort-change` on header click. Inline status/priority edits call
 * `updateTask` from the project-detail store directly (same pattern as the
 * kanban board calling `moveTask`).
 *
 * Events:
 *  - `task-click` : detail = DtoTaskResponse (row click; suppressed while a
 *    popover is open / a select is being used)
 *  - `sort-change`: detail = { field: SortField, dir: SortDir }
 */
@localized()
@customElement("breeze-list-view")
export class BreezeListView extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      width: 100%;
    }
    .wrap {
      width: 100%;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: var(--text-sm);
    }
    thead th {
      text-align: left;
      padding: 0 var(--space-4) var(--space-2) 0;
      font-weight: 500;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
      cursor: pointer;
      white-space: nowrap;
      transition: color var(--dur-fast) var(--ease-1);
    }
    thead th:active {
      transform: scale(0.97);
      transition: var(--tr-transform);
    }
    thead th:hover {
      color: var(--foreground);
    }
    .th-inner {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
    }
    .th-inner breeze-icon {
      color: var(--muted-foreground);
      opacity: 0.5;
    }
    .th-inner.active breeze-icon {
      opacity: 1;
      color: var(--foreground);
    }
    tbody tr {
      cursor: pointer;
      border-bottom: 1px solid var(--border);
      transition: background var(--dur-fast) var(--ease-1);
    }
    tbody tr:active {
      transform: scale(0.99);
      transition: var(--tr-transform);
    }
    tbody tr:hover {
      background: color-mix(in oklch, var(--muted) 40%, transparent);
    }
    tbody tr:last-child {
      border-bottom: none;
    }
    td {
      padding: var(--space-2) var(--space-4) var(--space-2) 0;
      vertical-align: middle;
    }
    .col-title {
      font-weight: 500;
      color: var(--foreground);
      max-width: var(--list-title-w);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .col-title-content {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      min-width: 0;
    }
    /* Subtask rows are indented under their parent with a ↳ arrow prefix. */
    .col-title-content.is-subtask {
      padding-left: var(--space-6);
    }
    .col-subtask-arrow {
      color: var(--muted-foreground);
      flex-shrink: 0;
      font-size: 0.85em;
    }
    tr.is-subtask .col-title-text {
      color: var(--muted-foreground);
    }
    .col-title-text {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      flex: 1;
      min-width: 0;
    }
    .col-title-labels {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      flex-shrink: 0;
    }
    .cell-stop {
      /* stop row-click from firing when interacting with the inline select */
      cursor: default;
    }
    .th-check,
    .cell-check {
      width: var(--space-8);
      padding-right: 0;
      text-align: center;
    }
    .cell-check input,
    .th-check input {
      cursor: pointer;
      width: var(--space-4);
      height: var(--space-4);
    }
    tbody tr.selected {
      background: color-mix(in oklch, var(--primary) 8%, transparent);
    }

    .bulk-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-3);
      padding: var(--space-2) var(--space-3);
      margin-bottom: var(--space-2);
      border: 1px solid color-mix(in oklch, var(--primary) 30%, var(--border));
      border-radius: var(--radius-lg);
      background: color-mix(in oklch, var(--primary) 6%, var(--card));
    }
    .bulk-info {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      font-size: var(--text-sm);
    }
    .bulk-count {
      font-weight: 600;
      color: var(--foreground);
    }
    .bulk-link {
      border: none;
      background: transparent;
      color: var(--primary);
      font-size: var(--text-xs);
      cursor: pointer;
      padding: 0;
    }
    .bulk-link:hover {
      text-decoration: underline;
    }
    .bulk-actions {
      display: flex;
      gap: var(--space-2);
    }
    .bulk-btn {
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
    }
    .bulk-btn:hover:not(:disabled) {
      background: var(--accent);
    }
    .bulk-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    /* inline select trigger (status / priority) */
    .sel-trigger {
      display: inline-flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-1-5);
      width: 100%;
      height: var(--control-h-sm);
      padding: 0 var(--space-2);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-xs);
      font-family: inherit;
      cursor: pointer;
      white-space: nowrap;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .sel-trigger:active {
      transform: scale(0.95);
      transition: var(--tr-transform);
    }
    .sel-trigger:hover {
      background: var(--accent);
    }
    .sel-trigger .label {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .sel-trigger .placeholder {
      color: var(--muted-foreground);
    }
    .sel-trigger .dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .sel-trigger breeze-icon {
      color: var(--muted-foreground);
      opacity: 0.5;
      flex-shrink: 0;
    }

    .pop {
      min-width: var(--space-40);
      max-height: var(--space-56);
      overflow-y: auto;
    }
    .opt {
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
      white-space: nowrap;
    }
    .opt:hover {
      background: var(--accent);
    }
    .opt .dot {
      width: var(--space-2-5);
      height: var(--space-2-5);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .opt .name {
      flex: 1;
    }
    .opt .check {
      color: var(--primary);
    }
    .opt .swatch {
      width: var(--space-2-5);
      height: var(--space-2-5);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }

    .due {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .due.overdue {
      color: var(--destructive);
      font-weight: 500;
    }

    .empty {
      display: flex;
      align-items: center;
      justify-content: center;
      padding: var(--space-12) var(--space-6);
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }
  `;

  @property({ type: Array, attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @property({ type: Array, attribute: false })
  tasks: DtoTaskResponse[] = [];

  @property()
  projectId = "";

  @property()
  sortField: SortField = "title";

  @property()
  sortDir: SortDir = "asc";

  // Multi-select / bulk edit state
  @state()
  private _selectedIds = new Set<string>();
  @state()
  private _bulkBusy = false;
  /** Index into the sorted task array for j/k keyboard navigation. -1 = none. */
  @state()
  private _focusIndex = -1;

  private _onShortcutNext = () => {
    const sorted = this._sorted();
    if (sorted.length === 0) return;
    this._focusIndex = Math.min(this._focusIndex, sorted.length - 1);
    const next = Math.min(this._focusIndex + 1, sorted.length - 1);
    this._focusIndex = next;
    this._scrollRowIntoView(next);
    this._onRowClick(sorted[next]);
  };

  private _onShortcutPrev = () => {
    const sorted = this._sorted();
    if (sorted.length === 0) return;
    this._focusIndex = Math.min(this._focusIndex, sorted.length - 1);
    const prev = Math.max(this._focusIndex - 1, 0);
    this._focusIndex = prev;
    this._scrollRowIntoView(prev);
    this._onRowClick(sorted[prev]);
  };

  private _scrollRowIntoView(index: number): void {
    const rows = this.renderRoot?.querySelectorAll("tbody tr");
    if (rows && rows[index]) {
      rows[index].scrollIntoView({ block: "nearest" });
    }
  }

  connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener("shortcut-next", this._onShortcutNext);
    document.addEventListener("shortcut-prev", this._onShortcutPrev);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener("shortcut-next", this._onShortcutNext);
    document.removeEventListener("shortcut-prev", this._onShortcutPrev);
  }

  #signals = new SignalController(this);

  constructor() {
    super();
    // Re-render when the shared project-detail store updates (inline status /
    // priority edits mutate it via `updateTask`). The parent also passes a
    // fresh `tasks` array on its own re-render.
    this.#signals.watch(projectDetail);
  }

  private _sorted(): DtoTaskResponse[] {
    const dir = this.sortDir === "asc" ? 1 : -1;
    const statusName = new Map(this.statuses.map((s) => [s.id, s.name ?? ""]));
    const arr = [...this.tasks];
    arr.sort((a, b) => {
      switch (this.sortField) {
        case "title":
          return dir * (a.title ?? "").localeCompare(b.title ?? "");
        case "priority":
          return dir *
            ((PRIORITY_RANK[a.priority ?? "none"] ?? 0) -
              (PRIORITY_RANK[b.priority ?? "none"] ?? 0));
        case "due_at": {
          const ad = a.due_at ? new Date(a.due_at).getTime() : null;
          const bd = b.due_at ? new Date(b.due_at).getTime() : null;
          if (ad == null && bd == null) return 0;
          if (ad == null) return dir;
          if (bd == null) return -dir;
          return dir * (ad - bd);
        }
        case "status":
          return dir *
            (statusName.get(a.status_id) ?? "").localeCompare(
              statusName.get(b.status_id) ?? "",
            );
        default:
          return 0;
      }
    });
    return arr;
  }

  private _handleSort(field: SortField) {
    const dir: SortDir = field === this.sortField && this.sortDir === "asc"
      ? "desc"
      : "asc";
    this.dispatchEvent(
      new CustomEvent("sort-change", {
        detail: { field, dir },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _sortIcon(field: SortField) {
    if (this.sortField !== field) {
      return html`
        <breeze-icon name="arrow-up-down" size="12"></breeze-icon>
      `;
    }
    return this.sortDir === "asc"
      ? html`
        <breeze-icon name="arrow-up" size="12"></breeze-icon>
      `
      : html`
        <breeze-icon name="arrow-down" size="12"></breeze-icon>
      `;
  }

  private _onRowClick(t: DtoTaskResponse) {
    this.dispatchEvent(
      new CustomEvent("task-click", {
        detail: t,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _changeStatus(taskId: string, statusId: string) {
    if (this.projectId) {
      void updateTask(this.projectId, taskId, { status_id: statusId });
    }
  }

  private _changePriority(taskId: string, priority: string) {
    if (this.projectId) void updateTask(this.projectId, taskId, { priority });
  }

  /** Close the enclosing single-select popover after a choice (Radix closes
   * on select; breeze-popover stays open by default, so we close it). */
  private _closeSelect(e: Event) {
    const pop = (e.target as HTMLElement | null)?.closest("breeze-popover") as
      | ({ open: boolean })
      | null;
    if (pop) pop.open = false;
  }

  // Multi-select / bulk edit
  private _toggleSelect(id: string, checked: boolean): void {
    const next = new Set(this._selectedIds);
    if (checked) next.add(id);
    else next.delete(id);
    this._selectedIds = next;
  }

  private _toggleSelectAll(checked: boolean): void {
    this._selectedIds = checked
      ? new Set(this._sorted().map((t) => t.id ?? "").filter(Boolean))
      : new Set();
  }

  private _clearSelection(): void {
    this._selectedIds = new Set();
  }

  private get _allSelected(): boolean {
    const sorted = this._sorted();
    return sorted.length > 0 &&
      sorted.every((t) => this._selectedIds.has(t.id ?? ""));
  }

  private async _bulkUpdate(
    patch: Parameters<typeof batchUpdateTasks>[2],
    successMsg: string,
  ): Promise<void> {
    if (!this.projectId || this._selectedIds.size === 0) return;
    this._bulkBusy = true;
    const ids = [...this._selectedIds];
    try {
      await batchUpdateTasks(this.projectId, ids, patch);
      showToast(`${successMsg} (${ids.length})`, { variant: "success" });
      this._clearSelection();
    } catch {
      showToast(msg("Bulk update failed"), { variant: "error" });
    } finally {
      this._bulkBusy = false;
    }
  }

  private _bulkSetStatus(statusId: string): void {
    void this._bulkUpdate({ status_id: statusId }, "Status updated");
  }

  private _bulkSetPriority(priority: string): void {
    void this._bulkUpdate({ priority }, "Priority updated");
  }

  private _renderStatusSelect(t: DtoTaskResponse) {
    const current = this.statuses.find((s) => s.id === t.status_id);
    return html`
      <breeze-popover>
        <button slot="trigger" class="sel-trigger" type="button">
          <span class="label">
            ${current
              ? html`
                <span class="dot" style="background:${current.color}"></span>
                <span class="truncate">${current.name}</span>
              `
              : html`
                <span class="placeholder">${msg("Select…")}</span>
              `}
          </span>
          <breeze-icon name="chevron-down" size="12"></breeze-icon>
        </button>
        <div slot="content" class="pop">
          ${this.statuses.map((s) =>
            html`
              <button
                class="opt"
                type="button"
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._changeStatus(t.id!, s.id!);
                  this._closeSelect(e);
                }}"
              >
                <span class="dot" style="background:${s.color}"></span>
                <span class="name">${s.name}</span>
                ${s.id === t.status_id
                  ? html`
                    <breeze-icon class="check" name="check" size="14"></breeze-icon>
                  `
                  : nothing}
              </button>
            `
          )}
        </div>
      </breeze-popover>
    `;
  }

  private _renderPrioritySelect(t: DtoTaskResponse) {
    const value = t.priority ?? "none";
    const current = PRIORITIES.find((p) => p.value === value);
    return html`
      <breeze-popover>
        <button slot="trigger" class="sel-trigger" type="button">
          <span class="label">
            ${value && value !== "none"
              ? html`
                <span class="dot" style="background:${current?.color}"></span>
                <span style="color:${current?.color}">${current?.label}</span>
              `
              : html`
                <span class="placeholder">${msg("Set priority")}</span>
              `}
          </span>
          <breeze-icon name="chevron-down" size="12"></breeze-icon>
        </button>
        <div slot="content" class="pop">
          ${PRIORITIES.map((p) =>
            html`
              <button
                class="opt"
                type="button"
                @click="${(e: Event) => {
                  e.stopPropagation();
                  this._changePriority(t.id!, p.value);
                  this._closeSelect(e);
                }}"
              >
                <span class="swatch" style="background:${p.color}"></span>
                <span class="name" style="color:${p.color}">${p.label}</span>
                ${p.value === value
                  ? html`
                    <breeze-icon class="check" name="check" size="14"></breeze-icon>
                  `
                  : nothing}
              </button>
            `
          )}
        </div>
      </breeze-popover>
    `;
  }

  private _renderBulkBar(selectedCount: number, totalCount: number): unknown {
    return html`
      <div class="bulk-bar">
        <div class="bulk-info">
          <span class="bulk-count">${selectedCount} selected</span>
          <button class="bulk-link" type="button"
            @click="${() =>
              this._toggleSelectAll(true)}">Select all (${totalCount})</button>
          <button class="bulk-link" type="button" @click="${() =>
            this._clearSelection()}">Clear</button>
        </div>
        <div class="bulk-actions">
          <breeze-popover>
            <button slot="trigger" class="bulk-btn" type="button"
              ?disabled="${this._bulkBusy}">
              Status <breeze-icon name="chevron-down" size="12"></breeze-icon>
            </button>
            <div slot="content" class="pop">
              ${this.statuses.map(
                (s) =>
                  html`
                    <button class="opt" type="button" @click="${(e: Event) => {
                      this._closeSelect(e);
                      this._bulkSetStatus(s.id ?? "");
                    }}">
                      <span class="dot" style="background:${s.color}"></span>
                      <span class="name">${s.name}</span>
                    </button>
                  `,
              )}
            </div>
          </breeze-popover>
          <breeze-popover>
            <button slot="trigger" class="bulk-btn" type="button"
              ?disabled="${this._bulkBusy}">
              Priority <breeze-icon name="chevron-down" size="12"></breeze-icon>
            </button>
            <div slot="content" class="pop">
              ${PRIORITIES.map(
                (p) =>
                  html`
                    <button class="opt" type="button" @click="${(e: Event) => {
                      this._closeSelect(e);
                      this._bulkSetPriority(p.value);
                    }}">
                      <span class="swatch" style="background:${p.color}"></span>
                      <span class="name" style="color:${p.color}">${p
                        .label}</span>
                    </button>
                  `,
              )}
            </div>
          </breeze-popover>
        </div>
      </div>
    `;
  }

  protected render() {
    if (this.tasks.length === 0) {
      return html`
        <div class="empty">
          No tasks yet. Create one from the Board view.
        </div>
      `;
    }

    const sorted = this._sorted();
    const selectedCount = this._selectedIds.size;

    return html`
      ${selectedCount > 0
        ? this._renderBulkBar(selectedCount, sorted.length)
        : nothing}
      <div class="wrap">
        <table>
          <thead>
            <tr>
              <th class="th-check">
                <input
                  type="checkbox"
                  .checked="${this._allSelected}"
                  @change="${(e: Event) =>
                    this._toggleSelectAll(
                      (e.target as HTMLInputElement).checked,
                    )}"
                  aria-label=${msg("Select all tasks")}
                />
              </th>
              ${HEADERS.map((h) =>
                html`
                  <th @click="${() => this._handleSort(h.field)}">
                    <span
                      class="th-inner ${this.sortField === h.field
                        ? "active"
                        : ""}"
                    >
                      ${h.label} ${this._sortIcon(h.field)}
                    </span>
                  </th>
                `
              )}
            </tr>
          </thead>
          <tbody>
            ${sorted.map((t) => {
              const dueDate = t.due_at ? new Date(t.due_at) : null;
              const isOverdue = dueDate && dueDate < new Date();
              const selected = this._selectedIds.has(t.id ?? "");
              const isSubtask = !!t.parent_task_id;
              return html`
                <tr
                  class="${selected ? "selected" : ""}${isSubtask
                    ? " is-subtask"
                    : ""}"
                  @click="${() => this._onRowClick(t)}"
                >
                  <td class="cell-check" @click="${(e: Event) =>
                    e.stopPropagation()}">
                    <input
                      type="checkbox"
                      .checked="${selected}"
                      @change="${(e: Event) =>
                        this._toggleSelect(
                          t.id ?? "",
                          (e.target as HTMLInputElement).checked,
                        )}"
                      aria-label="${`${msg("Select")} ${t.title}`}"
                    />
                  </td>
                  <td class="col-title">
                    <div class="col-title-content${isSubtask
                      ? " is-subtask"
                      : ""}">
                      ${isSubtask
                        ? html`<span class="col-subtask-arrow">↳</span>`
                        : nothing}
                      <span class="col-title-text">${t.title}</span>
                      ${(t.labels ?? []).length > 0
                        ? html`
                          <span class="col-title-labels">
                            ${(t.labels ?? []).slice(0, 3).map(
                              (l) =>
                                html`
                                  <breeze-label-chip
                                    .label="${l}"
                                    compact
                                  ></breeze-label-chip>
                                `,
                            )}
                          </span>
                        `
                        : nothing}
                    </div>
                  </td>
                  <td class="cell-stop" @click="${(e: Event) =>
                    e.stopPropagation()}">
                    ${this._renderStatusSelect(t)}
                  </td>
                  <td class="cell-stop" @click="${(e: Event) =>
                    e.stopPropagation()}">
                    ${this._renderPrioritySelect(t)}
                  </td>
                  <td>
                    ${dueDate
                      ? html`
                        <span class="due ${isOverdue ? "overdue" : ""}">
                          ${fmtDate(dueDate)}
                        </span>
                      `
                      : html`
                        <span class="due">—</span>
                      `}
                  </td>
                </tr>
              `;
            })}
          </tbody>
        </table>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-list-view": BreezeListView;
  }
}
