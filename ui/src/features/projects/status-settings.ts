import { html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";
import { identify } from "@/lib/sdk-helpers";
import { computeGap, computeGapY } from "@/lib/dnd-gap";
import type { DtoTaskStatusResponse } from "@/api";
import {
  deleteProjectsByIdStatusesByStatusId,
  postProjectsByIdStatuses,
  putProjectsByIdStatusesByStatusId,
} from "@/api";
import { refreshStatuses, setProjectStatuses } from "@/store/project-detail";
import {
  draggable,
  dropTargetForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/dialog.ts";
import { localized, msg } from "@lit/localize";

/**
 * Status settings: CRUD + drag-to-reorder for a project's task statuses.
 *
 * **Light DOM** (hosted by the light-DOM `plume-settings-view`): required for
 * `@atlaskit/pragmatic-drag-and-drop`, which reads `event.target` / `closest()`
 * and breaks across shadow boundaries. Styles are global, prefixed `ss-`.
 *
 * Reorder: the `.ss-rows` container is the **single drop target**, rows are
 * **draggables** (mirroring the kanban pattern). Gaps computed between row
 * midpoints. On drop: optimistic local + store update (`setProjectStatuses`),
 * PUT each status whose position changed, then `refreshStatuses` to reconcile.
 */

function getColorPalette() {
  return [
    { label: msg("Gray"), value: "oklch(0.55 0.02 260)" },
    { label: msg("Blue"), value: "oklch(0.62 0.19 255)" },
    { label: msg("Green"), value: "oklch(0.62 0.19 145)" },
    { label: msg("Yellow"), value: "oklch(0.75 0.15 95)" },
    { label: msg("Orange"), value: "oklch(0.65 0.17 65)" },
    { label: msg("Red"), value: "oklch(0.55 0.22 25)" },
    { label: msg("Purple"), value: "oklch(0.58 0.18 295)" },
    { label: msg("Teal"), value: "oklch(0.62 0.12 195)" },
    { label: msg("Pink"), value: "oklch(0.62 0.18 355)" },
  ];
}

function getCategories() {
  return [
    { value: "todo", label: msg("Todo") },
    { value: "in_progress", label: msg("In Progress") },
    { value: "done", label: msg("Done") },
    { value: "canceled", label: msg("Canceled") },
  ];
}

function titleCase(s: string): string {
  return s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

/* gap helpers (adapted from kanban-board.ts) */

interface StatusDragData {
  statusId: string;
}

const SS_STYLES = `
plume-status-settings { display: block; }
plume-status-row { display: block; }
.ss-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-2);
}
.ss-title {
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--foreground);
  margin: 0;
}
.ss-rows {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}
.ss-rows[data-over] {
  background: color-mix(in oklch, var(--primary) 5%, transparent);
  box-shadow: inset 0 0 0 1px
    color-mix(in oklch, var(--primary) 20%, transparent);
  border-radius: var(--radius-lg);
}
.ss-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--card);
  padding: var(--space-2) var(--space-3);
  transition: opacity var(--dur-fast) var(--ease-1), transform var(--dur-normal) var(--ease-2);
}
.ss-row[data-dragging] { opacity: 0.4; transform: scale(1.02); }
.ss-grip {
  color: var(--muted-foreground);
  cursor: grab;
  flex-shrink: 0;
  display: inline-flex;
  touch-action: none;
}
.ss-grip:active { cursor: grabbing; }
.ss-dot {
  width: var(--space-3);
  height: var(--space-3);
  border-radius: var(--radius-full);
  flex-shrink: 0;
}
.ss-name {
  flex: 1;
  font-size: var(--text-sm);
  color: var(--card-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ss-category {
  font-size: var(--text-xs);
  color: var(--muted-foreground);
}
.ss-row-actions {
  display: inline-flex;
  align-items: center;
  gap: var(--space-0-5);
  opacity: 0;
  transition: opacity var(--dur-fast) var(--ease-1);
}
.ss-row:hover .ss-row-actions { opacity: 1; }
.ss-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-5);
  height: var(--space-5);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
}
.ss-icon-btn:hover { background: var(--accent); color: var(--foreground); }
.ss-icon-btn.destructive:hover { color: var(--destructive); }

.ss-indicator {
  position: absolute;
  left: 0;
  right: 0;
  height: var(--space-0-5);
  display: none;
  align-items: center;
  padding: 0 var(--space-1);
  pointer-events: none;
  z-index: var(--z-sticky);
}
.ss-indicator-line {
  height: var(--space-0-5);
  flex: 1;
  background: var(--primary);
  border-radius: var(--space-0-5);
}
.ss-indicator-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
  flex-shrink: 0;
}

/* add composer */
.ss-add-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--card);
  padding: var(--space-2) var(--space-3);
}
.ss-add-row .ss-dot { width: var(--space-3); height: var(--space-3); }
.ss-add-input {
  flex: 1;
  height: var(--control-h-sm);
  border: none;
  background: transparent;
  color: var(--foreground);
  font-size: var(--text-sm);
  font-family: inherit;
  outline: none;
  padding: 0;
}
.ss-add-input::placeholder { color: var(--muted-foreground); }
.ss-swatch-btn {
  width: var(--space-5);
  height: var(--space-5);
  border-radius: var(--radius-full);
  border: 1px solid var(--border);
  cursor: pointer;
  flex-shrink: 0;
  padding: 0;
}
.ss-swatch-btn:hover {
  box-shadow: 0 0 0 2px color-mix(in oklch, var(--foreground) 30%, transparent);
}
.ss-cat-trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  height: var(--control-h-sm);
  padding: 0 var(--space-2);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--foreground);
  font-size: var(--text-xs);
  font-family: inherit;
  cursor: pointer;
}
.ss-cat-trigger:hover { background: var(--accent); }
.ss-pop { display: flex; flex-direction: column; gap: var(--space-0-5); }
.ss-palette {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1-5);
  padding: var(--space-1);
  max-width: var(--space-44);
}
.ss-swatch {
  width: var(--space-5);
  height: var(--space-5);
  border-radius: var(--radius-full);
  border: none;
  cursor: pointer;
  padding: 0;
  transition: box-shadow var(--dur-fast) var(--ease-1);
}
.ss-swatch:hover {
  box-shadow: 0 0 0 1px color-mix(in oklch, var(--foreground) 30%, transparent);
}
.ss-swatch.selected { box-shadow: 0 0 0 2px var(--foreground); }
.ss-menu-item {
  display: flex;
  width: 100%;
  padding: var(--space-1-5) var(--space-2);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--foreground);
  font-size: var(--text-xs);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}
.ss-menu-item:hover { background: var(--accent); }
.ss-add-btn {
  height: var(--control-h-sm);
  padding: 0 var(--space-2);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--foreground);
  font-size: var(--text-xs);
  font-family: inherit;
  cursor: pointer;
}
.ss-add-btn:hover { background: var(--accent); }
`;

@localized()
@customElement("plume-status-settings")
export class PlumeStatusSettings extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @property()
  projectId = "";

  @property({ type: Array, attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @state()
  private _adding = false;
  @state()
  private _newName = "";
  @state()
  private _newColor = "oklch(0.62 0.19 255)";
  @state()
  private _newCategory = "todo";
  @state()
  private _creating = false;
  @state()
  private _deleting: DtoTaskStatusResponse | null = null;

  #listRef = createRef<HTMLDivElement>();
  #indicatorRef = createRef<HTMLDivElement>();
  #dropCleanup?: () => void;

  disconnectedCallback(): void {
    this.#dropCleanup?.();
    super.disconnectedCallback();
  }

  protected updated(changed: Map<string, unknown>) {
    // (Re)wire the drop target once the container exists, and whenever the
    // project context changes. `statuses` changes don't require re-wiring;
    // the drop handler reads `this.statuses` directly.
    if (changed.has("projectId") || !this.#dropCleanup) {
      this.#setupDropTarget();
    }
  }

  #setupDropTarget() {
    this.#dropCleanup?.();
    const el = this.#listRef.value;
    if (!el) return;

    const updateIndicator = (clientY: number, excludeId?: string) => {
      const container = this.#listRef.value;
      if (!container) return;
      const gap = computeGap(container, clientY, "status-id", excludeId);
      const y = computeGapY(container, gap, "status-id", excludeId);
      const indicator = this.#indicatorRef.value;
      if (indicator) {
        indicator.style.top = `${y - 2}px`;
        indicator.style.display = "flex";
      }
    };
    const hideIndicator = () => {
      const indicator = this.#indicatorRef.value;
      if (indicator) indicator.style.display = "none";
    };

    this.#dropCleanup = dropTargetForElements({
      element: el,
      canDrop: ({ source }) =>
        identify<StatusDragData>(source.data).statusId !== undefined,
      onDragEnter: ({ source, location }) => {
        el.setAttribute("data-over", "");
        updateIndicator(
          location.current.input.clientY,
          identify<StatusDragData>(source.data).statusId,
        );
      },
      onDrag: ({ source, location }) => {
        updateIndicator(
          location.current.input.clientY,
          identify<StatusDragData>(source.data).statusId,
        );
      },
      onDragLeave: () => {
        el.removeAttribute("data-over");
        hideIndicator();
      },
      onDrop: ({ source, location }) => {
        el.removeAttribute("data-over");
        hideIndicator();

        const sourceId = identify<StatusDragData>(source.data).statusId;
        if (!sourceId) return;
        const container = this.#listRef.value;
        if (!container) return;

        const current = this.statuses;
        const fromStatus = current.find((s) => s.id === sourceId);
        if (!fromStatus) return;
        const others = current.filter((s) => s.id !== sourceId);
        const gap = computeGap(
          container,
          location.current.input.clientY,
          "status-id",
          sourceId,
        );
        const clamped = Math.max(0, Math.min(gap, others.length));
        const newOrder = [
          ...others.slice(0, clamped),
          fromStatus,
          ...others.slice(clamped),
        ];

        // Optimistic: update the local prop (guaranteed re-render here) AND the
        // shared store (so board/list/timeline + persistence stay in sync).
        this.statuses = newOrder;
        setProjectStatuses(newOrder);

        // Persist each status whose position changed, then reconcile.
        const puts: Promise<unknown>[] = [];
        for (let i = 0; i < newOrder.length; i++) {
          const s = newOrder[i];
          if (s.position !== i) {
            puts.push(
              putProjectsByIdStatusesByStatusId({
                path: { id: this.projectId, statusId: s.id! },
                body: {
                  name: s.name ?? "",
                  color: s.color ?? "",
                  category: s.category ?? "todo",
                  position: i,
                },
              }),
            );
          }
        }
        Promise.all(puts).finally(() => refreshStatuses(this.projectId));
      },
    });
  }

  async #handleAdd() {
    const name = this._newName.trim();
    if (!name) return;
    this._creating = true;
    try {
      await postProjectsByIdStatuses({
        path: { id: this.projectId },
        body: {
          name,
          color: this._newColor,
          category: this._newCategory,
          position: this.statuses.length,
        },
      });
      await refreshStatuses(this.projectId);
      this._adding = false;
      this._newName = "";
    } catch {
      // ignore: keep composer open
    } finally {
      this._creating = false;
    }
  }

  async #confirmDelete() {
    if (!this._deleting?.id) return;
    try {
      await deleteProjectsByIdStatusesByStatusId({
        path: { id: this.projectId, statusId: this._deleting.id },
      });
      await refreshStatuses(this.projectId);
    } catch {
      // ignore
    }
    this._deleting = null;
  }

  protected render() {
    return html`
      <style>
      ${SS_STYLES}
      </style>
      <div>
        <div class="ss-section-head">
          <h2 class="ss-title">${msg("Statuses")}</h2>
          ${!this._adding
            ? html`
              <plume-button
                variant="outline"
                size="sm"
                @click="${() => (this._adding = true)}"
              >
                <plume-icon name="plus" size="14"></plume-icon>
                Add status
              </plume-button>
            `
            : nothing}
        </div>

        <div ${ref(this.#listRef)} class="ss-rows">
          ${this._adding ? this.#renderAddRow() : nothing} ${this.statuses.map(
            (s) =>
              html`
                <plume-status-row
                  .status="${s}"
                  @request-delete="${(
                    e: CustomEvent,
                  ) => (this._deleting = e.detail as DtoTaskStatusResponse)}"
                ></plume-status-row>
              `,
          )}
          <div
            ${ref(this.#indicatorRef)}
            class="ss-indicator"
            style="display:none"
          >
            <span class="ss-indicator-dot"></span>
            <span class="ss-indicator-line"></span>
            <span class="ss-indicator-dot"></span>
          </div>
        </div>
      </div>

      <plume-dialog
        .open="${!!this._deleting}"
        heading="${`Delete "${this._deleting?.name ?? ""}" status?`}"
        @close="${() => (this._deleting = null)}"
      >
        <p style="font-size:var(--text-sm);color:var(--muted-foreground);margin:0">
          Tasks in this status will need to be moved. This action cannot be undone.
        </p>
        <div slot="footer" style="display:flex;gap:var(--space-2);width:100%">
          <span style="flex:1"></span>
          <plume-button variant="outline" size="sm" @click="${() => (this
            ._deleting = null)}"
          >Cancel</plume-button>
          <plume-button variant="destructive" size="sm" @click="${this
            .#confirmDelete}"
          >Delete status</plume-button>
        </div>
      </plume-dialog>
    `;
  }

  #renderAddRow() {
    const catLabel =
      getCategories().find((c) => c.value === this._newCategory)?.label ??
        "Todo";
    return html`
      <div class="ss-add-row">
        <span class="ss-dot" style="background:${this._newColor}"></span>
        <input
          class="ss-add-input"
          placeholder=${msg("Status name")}
          .value="${this._newName}"
          autofocus
          @input="${(
            e: Event,
          ) => (this._newName = (e.target as HTMLInputElement).value)}"
          @keydown="${(e: KeyboardEvent) => {
            if (e.key === "Enter") this.#handleAdd();
            if (e.key === "Escape") {
              this._adding = false;
              this._newName = "";
            }
          }}"
        />
        ${this.#renderColorPicker(this._newColor, (c) => (this._newColor = c))}
        <plume-popover>
          <button slot="trigger" class="ss-cat-trigger" type="button">
            ${catLabel}
            <plume-icon name="chevron-down" size="12"></plume-icon>
          </button>
          <div slot="content" class="ss-pop">
            ${getCategories().map(
              (c) =>
                html`
                  <button
                    class="ss-menu-item"
                    type="button"
                    @click="${(e: Event) => {
                      e.stopPropagation();
                      this._newCategory = c.value;
                    }}"
                  >
                    ${c.label}
                  </button>
                `,
            )}
          </div>
        </plume-popover>
        <button
          class="ss-add-btn"
          type="button"
          ?disabled="${this._creating || !this._newName.trim()}"
          @click="${this.#handleAdd}"
        >
          Add
        </button>
        <button
          class="ss-icon-btn"
          type="button"
          aria-label=${msg("Cancel")}
          @click="${() => {
            this._adding = false;
            this._newName = "";
          }}"
        >
          <plume-icon name="trash-2" size="14"></plume-icon>
        </button>
      </div>
    `;
  }

  #renderColorPicker(value: string, onChange: (c: string) => void) {
    return html`
      <plume-popover>
        <button
          slot="trigger"
          class="ss-swatch-btn"
          type="button"
          style="background:${value}"
          aria-label=${msg("Pick color")}
        >
        </button>
        <div slot="content" class="ss-palette">
          ${getColorPalette().map(
            (c) =>
              html`
                <button
                  class="ss-swatch ${value === c.value ? "selected" : ""}"
                  type="button"
                  title="${c.label}"
                  style="background:${c.value}"
                  @click="${(e: Event) => {
                    e.stopPropagation();
                    onChange(c.value);
                  }}"
                >
                </button>
              `,
          )}
        </div>
      </plume-popover>
    `;
  }
}

/**
 * Status row: a draggable list item (@atlaskit). The list container
 * (`plume-status-settings`) is the drop target; the row only needs to be
 * draggable and carry `data-status-id` so the container can compute gaps.
 */
@localized()
@customElement("plume-status-row")
export class PlumeStatusRow extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @property({ type: Object, attribute: false })
  status: DtoTaskStatusResponse = {} as DtoTaskStatusResponse;

  #rowRef = createRef<HTMLDivElement>();
  #dragCleanup?: () => void;

  disconnectedCallback(): void {
    this.#dragCleanup?.();
    super.disconnectedCallback();
  }

  protected updated(changed: Map<string, unknown>) {
    if (changed.has("status")) {
      this.#setupDraggable();
    }
  }

  #setupDraggable() {
    this.#dragCleanup?.();
    const el = this.#rowRef.value;
    if (!el || !this.status.id) return;

    const data: StatusDragData = { statusId: this.status.id };
    this.#dragCleanup = draggable({
      element: el,
      getInitialData: () => identify<Record<string, unknown>>(data),
      onDragStart: () => this.setAttribute("data-dragging", ""),
      onDrop: () => this.removeAttribute("data-dragging"),
    });
  }

  protected render() {
    return html`
      <div ${ref(this.#rowRef)} class="ss-row" data-status-id="${this.status
        .id}">
        <span class="ss-grip">
          <plume-icon name="grip-vertical" size="14"></plume-icon>
        </span>
        <span class="ss-dot" style="background:${this.status.color}"></span>
        <span class="ss-name">${this.status.name}</span>
        <span class="ss-category">${titleCase(
          this.status.category ?? "",
        )}</span>
        <div class="ss-row-actions">
          <button
            class="ss-icon-btn destructive"
            type="button"
            aria-label=${msg("Delete status")}
            @click="${(e: Event) => {
              e.stopPropagation();
              this.dispatchEvent(
                new CustomEvent("request-delete", {
                  detail: this.status,
                  bubbles: true,
                  composed: true,
                }),
              );
            }}"
          >
            <plume-icon name="trash-2" size="14"></plume-icon>
          </button>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-status-settings": PlumeStatusSettings;
    "plume-status-row": PlumeStatusRow;
  }
}
