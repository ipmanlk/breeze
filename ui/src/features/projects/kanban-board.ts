import { html, LitElement } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";
import { fmtDate } from "@/lib/format/date";
import type { DtoTaskResponse, DtoTaskStatusResponse } from "@/api";
import { identify } from "@/lib/sdk-helpers";
import { createTask, moveTask } from "@/store/project-detail";
import { generateKeyBetween } from "@/lib/lexorank";
import {
  computeGap as dndComputeGap,
  computeGapY as dndComputeGapY,
} from "@/lib/dnd-gap";
import {
  draggable,
  dropTargetForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { autoScrollForElements } from "@atlaskit/pragmatic-drag-and-drop-auto-scroll/element";
import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/input.ts";
import "../../components/ui/label-chip.ts";
import { localized, msg } from "@lit/localize";

/**
 * Kanban board / column / card: all render in **light DOM**.
 *
 * `@atlaskit/pragmatic-drag-and-drop` uses native HTML drag events and looks up
 * `event.target` in a `WeakMap` registry + `event.target.closest()` to find drop
 * targets. Shadow DOM retargets `event.target` to the shadow host at each shadow
 * boundary, so the library could never find the draggable / drop target elements
 * that live inside a shadow root.
 *
 * By using `createRenderRoot() { return this; }` the template content stays in the
 * light DOM (document tree) where `event.target` is the actual element and
 * `closest()` can traverse up to find drop targets.
 *
 * Styles are injected via a single `<style>` tag rendered by the board (the
 * top-level kanban component). Since light DOM styles are global, all class names
 * are prefixed with `kb-` to avoid collisions with other components.
 */

const PRIORITY_DOT: Record<string, string> = {
  none: "var(--muted-foreground)",
  low: "oklch(0.7 0.15 240)",
  medium: "oklch(0.7 0.2 80)",
  high: "oklch(0.65 0.2 50)",
  urgent: "oklch(0.6 0.25 25)",
};

const PRIORITY_LABEL: Record<string, string> = {
  none: "None",
  low: "Low",
  medium: "Medium",
  high: "High",
  urgent: "Urgent",
};

function getInitials(name?: string): string {
  if (!name) return "??";
  return name.split(" ").map((n) => n[0]).join("").toUpperCase().slice(0, 2);
}

interface TaskDragData {
  taskId: string;
  statusId: string;
  positionKey: string;
}

/* Shared stylesheet (rendered once by the board) */

const KANBAN_STYLES = `
plume-kanban-board {
  display: flex;
  flex: 1;
  gap: var(--space-3);
  overflow-x: auto;
  padding-bottom: var(--space-4);
  min-height: 0;
}

plume-kanban-column {
  width: var(--kanban-col-w);
  min-width: var(--kanban-col-w);
  flex-shrink: 0;
  border-radius: var(--radius-lg);
  transition: background var(--dur-fast) var(--ease-1);
}
/* Drop target highlight */
plume-kanban-column[data-drop-target] {
  background: color-mix(in oklch, var(--primary) 8%, transparent);
  transition: background var(--dur-fast) var(--ease-1);
}
.kb-column {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
plume-kanban-column[data-over] {
  background: color-mix(in oklch, var(--primary) 5%, transparent);
  box-shadow: inset 0 0 0 1px
    color-mix(in oklch, var(--primary) 20%, transparent);
}

.kb-column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--space-1);
}
.kb-column-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.kb-status-dot {
  width: var(--space-2-5);
  height: var(--space-2-5);
  border-radius: var(--radius-full);
  flex-shrink: 0;
}
.kb-column-name {
  font-size: var(--text-sm);
  font-weight: 500;
  text-transform: capitalize;
  color: var(--foreground);
}
.kb-column-count {
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
  color: var(--muted-foreground);
}
.kb-column-menu {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--control-h-xs);
  height: var(--control-h-xs);
  border-radius: var(--radius-md);
  border: none;
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease-1);
}
.kb-column-menu:active {
  transform: scale(0.95);
  transition: var(--tr-transform);
}
.kb-column-menu:hover {
  background: var(--accent);
  color: var(--foreground);
}

.kb-cards {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-height: var(--space-12);
}
.kb-drop-indicator {
  position: absolute;
  left: 0;
  right: 0;
  height: var(--space-6);
  display: none;
  align-items: center;
  padding: 0 var(--space-1);
  pointer-events: none;
  z-index: var(--z-sticky);
}
.kb-drop-indicator-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
  flex-shrink: 0;
}
.kb-drop-indicator-line {
  height: var(--space-0-5);
  flex: 1;
  background: var(--primary);
  border-radius: var(--space-0-5);
  margin: 0 var(--space-0-5);
}

/* Add-task button: empty column (dashed border CTA) */
.kb-add-cta {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-1-5);
  height: var(--control-h);
  width: 100%;
  border: 1px dashed
    color-mix(in oklch, var(--muted-foreground) 25%, transparent);
  border-radius: var(--radius-lg);
  background: none;
  color: var(--muted-foreground);
  font-size: var(--text-sm);
  font-family: inherit;
  cursor: pointer;
  transition:
    border-color var(--dur-fast) var(--ease-1),
    background var(--dur-fast) var(--ease-1),
    color var(--dur-fast) var(--ease-1);
}
.kb-add-cta:active {
  transform: scale(0.97);
  transition: var(--tr-transform);
}
.kb-add-cta:hover {
  border-color: color-mix(in oklch, var(--primary) 40%, transparent);
  background: color-mix(in oklch, var(--accent) 50%, transparent);
  color: var(--foreground);
}

/* Add-task button: non-empty column (ghost style, no border) */
.kb-add-ghost {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: var(--space-1-5);
  height: var(--control-h);
  width: 100%;
  border: none;
  border-radius: var(--radius-lg);
  background: none;
  color: var(--muted-foreground);
  font-size: var(--text-sm);
  font-family: inherit;
  cursor: pointer;
  padding: 0 var(--space-2);
  transition:
    background var(--dur-fast) var(--ease-1),
    color var(--dur-fast) var(--ease-1);
}
.kb-add-ghost:active {
  transform: scale(0.97);
  transition: var(--tr-transform);
}
.kb-add-ghost:hover {
  background: var(--accent);
  color: var(--foreground);
}

.kb-drop-hint {
  display: flex;
  align-items: center;
  justify-content: center;
  height: var(--space-12);
  border: 2px dashed color-mix(in oklch, var(--primary) 50%, transparent);
  border-radius: var(--radius-lg);
  font-size: var(--text-sm);
  color: var(--muted-foreground);
}

.kb-composer {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--card);
  /* shadow-sm + subtle ring (ring-1 ring-foreground/5) */
  box-shadow:
    var(--shadow-xs),
    inset 0 0 0 1px color-mix(in oklch, var(--foreground) 5%, transparent);
}
.kb-composer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.kb-composer-hint {
  font-size: var(--text-2xs);
  color: var(--muted-foreground);
}
.kb-composer-hint kbd {
  background: var(--muted);
  padding: 0 var(--space-1);
  border-radius: var(--space-0-5);
  font-family: var(--font-mono);
}

plume-kanban-card {
  display: block;
}
.kb-card {
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--card);
  color: var(--card-foreground);
  font-size: var(--text-sm);
  cursor: grab;
  box-shadow: var(--shadow-xs);
  transition:
    box-shadow var(--dur-fast) var(--ease-1),
    border-color var(--dur-fast) var(--ease-1);
}
/* Card lift during drag */
plume-kanban-card[data-dragging] .kb-card {
  opacity: 0.4;
}
/* Subtask cards are indented with a left accent border + a ↳ parent
 * reference so the hierarchy is visible when "Show subtasks" is on. */
.kb-card.is-subtask {
  margin-left: var(--space-4);
  border-left: 2px solid var(--accent);
}
.kb-card-parent {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-xs);
  color: var(--muted-foreground);
  margin-bottom: var(--space-1);
  overflow: hidden;
}
.kb-card-parent-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kb-card:hover {
  box-shadow: var(--shadow-md);
  border-color: color-mix(in oklch, var(--foreground) 15%, transparent);
}
.kb-card:active {
  cursor: grabbing;
}
.kb-card-labels {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
  margin-top: var(--space-1-5);
}
.kb-card-title {
  font-weight: 500;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.kb-card-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-top: var(--space-2-5);
}
.kb-card-assignees {
  display: flex;
  /* first avatar anchors the stack; subsequent ones overlap */
}
.kb-avatar {
  width: var(--avatar-sm);
  height: var(--avatar-sm);
  border-radius: var(--radius-full);
  border: 2px solid var(--card);
  background: var(--muted);
  color: var(--muted-foreground);
  font-size: var(--text-2xs);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: calc(var(--space-1-5) * -1);
}
.kb-avatar:first-child {
  margin-left: 0;
}
.kb-avatar-more {
  font-size: var(--text-2xs);
  background: var(--muted);
}
.kb-card-due {
  font-size: var(--text-2xs);
  color: var(--muted-foreground);
}
.kb-card-due.overdue {
  color: var(--destructive);
  font-weight: 500;
}
.kb-priority {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-2xs);
  font-weight: 500;
  color: var(--muted-foreground);
  text-transform: capitalize;
}
.kb-priority-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
}
.kb-subtask-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-0-5);
  font-size: var(--text-2xs);
  font-weight: 500;
  color: var(--muted-foreground);
}

/* Subtask progress bar under card title */
.kb-subtask-progress-track {
  height: var(--space-0-5);
  border-radius: var(--radius-full);
  background: var(--muted);
  overflow: hidden;
  margin: var(--space-1) 0 0;
}
.kb-subtask-progress-fill {
  height: 100%;
  border-radius: var(--radius-full);
  background: var(--primary);
  transition: width var(--dur-normal) var(--ease-1);
}

/* Wrapper so badge sits inline with priority dot */
.kb-subtask-progress-wrap {
  display: inline-flex;
  align-items: center;
}
`;

const INDICATOR_HEIGHT = 24;

function positionFromGap(gapIndex: number, tasks: DtoTaskResponse[]): string {
  if (tasks.length === 0) return generateKeyBetween(null, null);
  if (gapIndex === 0) {
    return generateKeyBetween(null, tasks[0].position_key ?? "z");
  }
  if (gapIndex >= tasks.length) {
    return generateKeyBetween(
      tasks[tasks.length - 1].position_key ?? "0",
      null,
    );
  }
  const prev = tasks[gapIndex - 1].position_key ?? "0";
  const next = tasks[gapIndex].position_key ?? "z";
  if (prev >= next) return generateKeyBetween(null, null);
  return generateKeyBetween(prev, next);
}

/* Board */

@localized()
@customElement("plume-kanban-board")
export class PlumeKanbanBoard extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @property({ attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @property({ attribute: false })
  tasks: DtoTaskResponse[] = [];

  @property({ attribute: false })
  grouped: Map<string, DtoTaskResponse[]> = new Map();

  @property({ attribute: false })
  projectId = "";

  /** When true, subtasks (parent_task_id set) are shown indented with a
   * ↳ parent_title reference. Off by default: top-level tasks only. */
  @property({ attribute: false })
  showSubtasks = false;

  protected render() {
    return html`
      <style>
      ${KANBAN_STYLES}
      </style>
      ${this.statuses.map(
        (s) =>
          html`
            <plume-kanban-column
              .status="${s}"
              .tasks="${this.grouped.get(s.id ?? "") ?? []}"
              .projectId="${this.projectId}"
              .showSubtasks="${this.showSubtasks}"
            ></plume-kanban-column>
          `,
      )}
    `;
  }
}

/* Column */

@localized()
@customElement("plume-kanban-column")
export class PlumeKanbanColumn extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @property({ attribute: false })
  status: DtoTaskStatusResponse = {} as DtoTaskStatusResponse;

  @property({ attribute: false })
  tasks: DtoTaskResponse[] = [];

  @property({ attribute: false })
  projectId = "";

  /** Mirror of the board's showSubtasks flag: passed down so cards can
   * render the ↳ parent reference when subtasks are visible. */
  @property({ attribute: false })
  showSubtasks = false;

  @state()
  private _isOver = false;

  @state()
  private _isAdding = false;

  @state()
  private _draft = "";

  @state()
  private _isCreating = false;

  #columnRef = createRef<HTMLDivElement>();
  #cardsRef = createRef<HTMLDivElement>();
  #indicatorRef = createRef<HTMLDivElement>();
  #dragCleanup?: () => void;

  connectedCallback(): void {
    super.connectedCallback();
  }

  disconnectedCallback(): void {
    this.#dragCleanup?.();
    super.disconnectedCallback();
  }

  updated(changed: Map<string, unknown>) {
    // Setup drop target after first render (refs are now populated).
    // Re-setup when the status changes (different column identity).
    if (changed.has("status") || changed.has("projectId")) {
      this.#setupDropTarget();
    }
  }

  #setupDropTarget() {
    this.#dragCleanup?.();
    const el = this.#columnRef.value;
    if (!el) return;

    const getExcludeId = (
      sourceData: Record<string, unknown>,
    ): string | undefined => {
      return sourceData.statusId === this.status.id
        ? (sourceData.taskId as string)
        : undefined;
    };

    const updateIndicator = (clientY: number, excludeTaskId?: string) => {
      const container = this.#cardsRef.value;
      if (!container) return;
      const gap = dndComputeGap(container, clientY, "task-id", excludeTaskId);
      const y = dndComputeGapY(container, gap, "task-id", excludeTaskId, 8);
      const indicator = this.#indicatorRef.value;
      if (indicator) {
        indicator.style.top = `${y - INDICATOR_HEIGHT / 2}px`;
        indicator.style.display = "flex";
      }
    };

    const hideIndicator = () => {
      const indicator = this.#indicatorRef.value;
      if (indicator) indicator.style.display = "none";
    };

    this.#dragCleanup = combine(
      dropTargetForElements({
        element: el,
        canDrop: ({ source }) =>
          identify<TaskDragData>(source.data).taskId !== undefined,
        onDragEnter: ({ source, location }) => {
          this._isOver = true;
          this.setAttribute("data-over", "");
          updateIndicator(
            location.current.input.clientY,
            getExcludeId(identify<Record<string, unknown>>(source.data)),
          );
        },
        onDrag: ({ source, location }) => {
          updateIndicator(
            location.current.input.clientY,
            getExcludeId(identify<Record<string, unknown>>(source.data)),
          );
        },
        onDragLeave: () => {
          this._isOver = false;
          this.removeAttribute("data-over");
          hideIndicator();
        },
        onDrop: ({ source, location }) => {
          this._isOver = false;
          this.removeAttribute("data-over");
          hideIndicator();

          const sourceData = identify<TaskDragData>(source.data);
          const taskId = sourceData.taskId;
          const clientY = location.current.input.clientY;
          const isSameColumn = sourceData.statusId === this.status.id;
          const excludeId = isSameColumn ? taskId : undefined;

          const container = this.#cardsRef.value;
          if (!container || !taskId || !this.projectId) return;

          const gap = dndComputeGap(container, clientY, "task-id", excludeId);

          // Build task list in DOM order (matches gap computation)
          const cardElements = Array.from(
            container.querySelectorAll("[data-task-id]"),
          ) as HTMLElement[];
          const taskById = new Map(this.tasks.map((t) => [t.id, t]));
          const tasksInDomOrder: DtoTaskResponse[] = [];
          for (const el of cardElements) {
            const id = el.getAttribute("data-task-id");
            if (id && taskById.has(id)) {
              tasksInDomOrder.push(taskById.get(id)!);
            }
          }
          const effectiveTasks = excludeId
            ? tasksInDomOrder.filter((t) => t.id !== excludeId)
            : tasksInDomOrder;

          const positionKey = positionFromGap(gap, effectiveTasks);

          moveTask(this.projectId, taskId, {
            status_id: this.status.id ?? "",
            position_key: positionKey,
          });
        },
      }),
      autoScrollForElements({ element: el }),
    );
  }

  async #handleAdd() {
    const title = this._draft.trim();
    if (!title) return;
    this._isCreating = true;
    const result = await createTask(this.projectId, {
      title,
      status_id: this.status.id ?? "",
    });
    this._isCreating = false;
    if (result) {
      this._draft = "";
      this._isAdding = false;
    }
  }

  render() {
    const isEmpty = this.tasks.length === 0;
    const showComposer = this._isAdding;
    const showInlineCTA = isEmpty && !this._isOver && !showComposer;
    const showDropHint = isEmpty && this._isOver && !showComposer;
    const composerInCards = isEmpty && showComposer;

    return html`
      <div ${ref(this.#columnRef)} class="kb-column" data-status-id="${this
        .status.id}">
        <div class="kb-column-header">
          <div class="kb-column-title">
            <span
              class="kb-status-dot"
              style="background:${this.status.color}"
            ></span>
            <span class="kb-column-name">${this.status.name}</span>
            <span class="kb-column-count">${this.tasks.length}</span>
          </div>
          <button class="kb-column-menu" aria-label=${msg("Column options")}>
            <plume-icon name="more-horizontal" size="16"></plume-icon>
          </button>
        </div>

        <div ${ref(this.#cardsRef)} class="kb-cards">
          ${this.tasks.map(
            (t) => {
              const isSubtask = !!t.parent_task_id;
              // When showSubtasks is on, subtask cards render indented with
              // a ↳ parent reference so they're visually distinct from
              // top-level tasks. When off, subtasks aren't in the list at all
              // (the fetch defaults to top-level only).
              return html`
                <plume-kanban-card
                  .task="${t}"
                  .isSubtask="${this.showSubtasks && isSubtask}"
                ></plume-kanban-card>
              `;
            },
          )}

          <div
            ${ref(this.#indicatorRef)}
            class="kb-drop-indicator"
            style="display:none"
          >
            <span class="kb-drop-indicator-dot"></span>
            <span class="kb-drop-indicator-line"></span>
            <span class="kb-drop-indicator-dot"></span>
          </div>

          ${showInlineCTA
            ? html`
              <button
                class="kb-add-cta"
                @click="${() => {
                  this._isAdding = true;
                }}"
              >
                <plume-icon name="plus" size="16"></plume-icon>
                Add task
              </button>
            `
            : ""} ${showDropHint
            ? html`
              <div class="kb-drop-hint">${msg("Drop here")}</div>
            `
            : ""} ${composerInCards ? this.#renderComposer() : ""}
        </div>

        ${!isEmpty && !showComposer
          ? html`
            <button
              class="kb-add-ghost"
              @click="${() => {
                this._isAdding = true;
              }}"
            >
              <plume-icon name="plus" size="16"></plume-icon>
              Add task
            </button>
          `
          : ""} ${!isEmpty && showComposer ? this.#renderComposer() : ""}
      </div>
    `;
  }

  #renderComposer() {
    return html`
      <div class="kb-composer">
        <plume-input
          placeholder=${msg("Task title")}
          .value="${this._draft}"
          ?autofocus="${true}"
          @input="${(e: Event) => {
            this._draft = (e.target as HTMLInputElement).value;
          }}"
          @keydown="${(e: KeyboardEvent) => {
            if (e.key === "Enter" && this._draft.trim()) {
              this.#handleAdd();
            }
            if (e.key === "Escape") {
              this._isAdding = false;
              this._draft = "";
            }
          }}"
          @blur="${() => {
            if (!this._draft.trim()) {
              this._isAdding = false;
              this._draft = "";
            }
          }}"
        ></plume-input>
        <div class="kb-composer-actions">
          <span class="kb-composer-hint"><kbd>↵</kbd> add · <kbd>esc</kbd>
            close</span>
          <div style="display:flex;gap:var(--space-1)">
            <plume-button
              variant="outline"
              size="sm"
              @click="${() => {
                this._isAdding = false;
                this._draft = "";
              }}"
            >Cancel</plume-button>
            <plume-button
              size="sm"
              ?disabled="${!this._draft.trim() || this._isCreating}"
              @click="${this.#handleAdd}"
            >
              ${this._isCreating ? "Adding..." : "Add"}
            </plume-button>
          </div>
        </div>
      </div>
    `;
  }
}

/* Card */

@localized()
@customElement("plume-kanban-card")
export class PlumeKanbanCard extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @property({ attribute: false })
  task: DtoTaskResponse = {} as DtoTaskResponse;

  /** When true the card is rendered indented with a ↳ parent_title
   * reference (subtasks shown on the board via the showSubtasks toggle). */
  @property({ attribute: false })
  isSubtask = false;

  /** Number of direct subtasks this card's task has (0 if none). */
  get _subtaskCount() {
    return this.task.subtask_count ?? 0;
  }
  get _completedSubtaskCount() {
    return this.task.completed_subtask_count ?? 0;
  }

  #cardRef = createRef<HTMLDivElement>();
  #dragCleanup?: () => void;
  #wasDragging = false;

  disconnectedCallback(): void {
    this.#dragCleanup?.();
    super.disconnectedCallback();
  }

  updated(changed: Map<string, unknown>) {
    if (changed.has("task")) {
      this.#setupDraggable();
    }
  }

  #setupDraggable() {
    this.#dragCleanup?.();
    const el = this.#cardRef.value;
    if (!el || !this.task.id) return;

    const data: TaskDragData = {
      taskId: this.task.id,
      statusId: this.task.status_id ?? "",
      positionKey: this.task.position_key ?? "",
    };

    this.#dragCleanup = draggable({
      element: el,
      getInitialData: () => identify<Record<string, unknown>>(data),
      onDragStart: () => {
        this.setAttribute("data-dragging", "");
        this.#wasDragging = true;
      },
      onDrop: () => {
        this.removeAttribute("data-dragging");
        setTimeout(() => (this.#wasDragging = false), 0);
      },
    });
  }

  #onClick() {
    if (this.#wasDragging) return;
    this.dispatchEvent(
      new CustomEvent("task-click", {
        detail: this.task,
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    const t = this.task;
    const dueDate = t.due_at ? new Date(t.due_at) : null;
    const isOverdue = dueDate && dueDate < new Date();
    const hasPriority = t.priority && t.priority !== "none";
    const assignees = t.assignees ?? [];

    return html`
      <div
        ${ref(this.#cardRef)}
        class="kb-card${this.isSubtask ? " is-subtask" : ""}"
        data-task-id="${t.id}"
        @click="${this.#onClick}"
      >
        ${this.isSubtask && t.parent_title
          ? html`
            <div class="kb-card-parent" title="${t.parent_title}">
              <plume-icon name="corner-up-left" size="11"></plume-icon>
              <span class="kb-card-parent-title">${t.parent_title}</span>
            </div>
          `
          : ""}
        <div class="kb-card-title">${t.title}</div>
        ${this._subtaskCount > 0
          ? html`
            <div class="kb-subtask-progress-track">
              <div
                class="kb-subtask-progress-fill"
                style="width: ${this._subtaskCount > 0
                  ? Math.round(
                    (this._completedSubtaskCount / this._subtaskCount) * 100,
                  )
                  : 0}%"
              ></div>
            </div>
          `
          : ""}
        ${(t.labels ?? []).length > 0
          ? html`
            <div class="kb-card-labels">
              ${(t.labels ?? []).slice(0, 4).map(
                (l) =>
                  html`<plume-label-chip .label="${l}"></plume-label-chip>`,
              )}
            </div>
          `
          : ""}
        <div class="kb-card-meta">
          <div>
            ${hasPriority
              ? html`
                <span class="kb-priority">
                  <span
                    class="kb-priority-dot"
                    style="background:${PRIORITY_DOT[t.priority ?? "none"] ??
                      "var(--muted)"}"
                  ></span>
                  ${PRIORITY_LABEL[t.priority ?? "none"] ?? t.priority}
                </span>
              `
              : ""}
            ${this._subtaskCount > 0
              ? html`
                <div class="kb-subtask-progress-wrap"
                  title="${this._completedSubtaskCount} of ${this
                    ._subtaskCount} subtasks complete">
                  <span class="kb-subtask-badge">
                    <plume-icon name="list-checks" size="11"></plume-icon>
                    ${this._completedSubtaskCount}/${this._subtaskCount}
                  </span>
                </div>
              `
              : ""}
          </div>
          <div style="display:flex;align-items:center;gap:var(--space-1)">
            ${assignees.length > 0
              ? html`
                <div class="kb-card-assignees">
                  ${assignees.slice(0, 3).map(
                    (a) =>
                      html`
                        <div class="kb-avatar" title="${a.name ?? ""}">
                          ${getInitials(a.name)}
                        </div>
                      `,
                  )} ${assignees.length > 3
                    ? html`
                      <div class="kb-avatar kb-avatar-more">
                        +${assignees.length - 3}
                      </div>
                    `
                    : ""}
                </div>
              `
              : ""} ${dueDate
              ? html`
                <span class="kb-card-due ${isOverdue ? "overdue" : ""}">
                  ${fmtDate(dueDate)}
                </span>
              `
              : ""}
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-kanban-board": PlumeKanbanBoard;
    "plume-kanban-column": PlumeKanbanColumn;
    "plume-kanban-card": PlumeKanbanCard;
  }
}
