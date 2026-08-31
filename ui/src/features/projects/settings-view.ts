import { logError } from "@/lib/log";
import { fmtDate, fmtDateYear } from "@/lib/format/date";
import { html, LitElement, nothing } from "lit";
import { showToast } from "@/components/ui/toast-store";
import { customElement, property, state } from "lit/decorators.js";
import type {
  DtoCycleResponse,
  DtoProjectResponse,
  DtoTaskStatusResponse,
} from "@/api";
import {
  hasProjectPermission,
  projectDetail,
  updateProject,
} from "@/store/project-detail";
import { ProjectPermission } from "@/lib/permissions";
import { postProjectsByIdArchive, postProjectsByIdUnarchive } from "@/api";
import {
  activateCycle,
  completeCycle,
  cycles,
  deleteCycle,
  fetchCycles,
} from "./cycles-store";
import { SignalController } from "@/lib/signal-controller";
import "./status-settings.ts";
import "./cycle-dialog.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/select.ts";
import "../../components/ui/switch.ts";
import "../../components/ui/dialog.ts";
import "./template-manager.ts";
import "./custom-field-manager.ts";
import { localized, msg } from "@lit/localize";

/**
 * Settings view: General info, Cycles enable/config, Status CRUD, and the
 * Cycles list.
 *
 * **Light DOM** because it hosts `<plume-status-settings>`, which uses
 * `@atlaskit/pragmatic-drag-and-drop` and needs an unbroken light-DOM chain
 * (no shadow boundary between document and the draggable rows). Styles are
 * global, prefixed `sv-`.
 *
 * Properties: `project`, `statuses`. Reads the shared `cycles` store (fetched
 * here when the project has cycles enabled).
 */
@localized()
@customElement("plume-settings-view")
export class PlumeSettingsView extends LitElement {
  /** Light DOM: hosts the DnD-bearing status-settings. */
  createRenderRoot() {
    return this;
  }

  @property({ type: Object, attribute: false })
  project: DtoProjectResponse | null = null;

  @property({ type: Array, attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @state()
  private _autoGenerate = false;
  @state()
  private _handling = "next_cycle";
  @state()
  private _savingSettings = false;
  @state()
  private _enabling = false;
  @state()
  private _cycleDialogOpen = false;
  @state()
  private _editCycle: DtoCycleResponse | null = null;
  @state()
  private _complete: { cycle: DtoCycleResponse; moveToCycleId: string } | null =
    null;

  #signals = new SignalController(this);
  #lastProjectId = "";
  #syncedProjectId = "";

  constructor() {
    super();
    this.#signals.watch(cycles);
    this.#signals.watch(projectDetail);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    const pid = this.project?.id ?? "";
    // Re-seed local form state when the project identity changes (e.g. after
    // enabling cycles, which swaps in a new project object from the store).
    if (changed.has("project") && pid !== this.#syncedProjectId) {
      this.#syncedProjectId = pid;
      this._autoGenerate = this.project?.auto_generate_cycles ?? false;
      this._handling = this.project?.incomplete_task_handling ?? "next_cycle";
    }
    if (pid && pid !== this.#lastProjectId) {
      this.#lastProjectId = pid;
      if ((this.project?.cycle_duration ?? 0) > 0) {
        void fetchCycles(pid);
      }
    }
  }

  get #projectId() {
    return this.project?.id ?? "";
  }

  get #hasCycles() {
    return (this.project?.cycle_duration ?? 0) > 0;
  }

  async #enableCycles() {
    if (!this.#projectId) return;
    this._enabling = true;
    await updateProject(this.#projectId, { cycle_duration: 14 });
    this._enabling = false;
    if (this.#projectId) void fetchCycles(this.#projectId, true);
  }

  async #saveSettings() {
    if (!this.#projectId) return;
    this._savingSettings = true;
    await updateProject(this.#projectId, {
      auto_generate_cycles: this._autoGenerate,
      incomplete_task_handling: this._handling,
    });
    this._savingSettings = false;
  }

  async #activate(id: string) {
    await activateCycle(this.#projectId, id);
  }
  async #deleteCycle(id: string) {
    if (!confirm(msg("Delete this cycle? Tasks will be unassigned."))) return;
    await deleteCycle(this.#projectId, id);
  }

  async #archive() {
    if (
      !confirm(
        msg(
          "Archive this project? It will be hidden from the project list but remain accessible.",
        ),
      )
    ) return;
    try {
      await postProjectsByIdArchive({
        path: { id: this.#projectId },
        throwOnError: true,
      });
      window.location.reload();
    } catch (err) {
      logError("archive failed:", err);
      showToast(msg("Failed to archive project"), { variant: "error" });
    }
  }

  async #unarchive() {
    try {
      await postProjectsByIdUnarchive({
        path: { id: this.#projectId },
        throwOnError: true,
      });
      window.location.reload();
    } catch (err) {
      logError("unarchive failed:", err);
      showToast(msg("Failed to unarchive project"), { variant: "error" });
    }
  }

  #exportTasks(format: string): void {
    window.open(
      `/api/projects/${this.#projectId}/tasks/export?format=${format}`,
      "_blank",
    );
  }

  #exportTime(format: string): void {
    window.open(
      `/api/projects/${this.#projectId}/time-entries/export?format=${format}`,
      "_blank",
    );
  }
  async #confirmComplete() {
    if (!this._complete) return;
    await completeCycle(this.#projectId, this._complete.cycle.id!, {
      move_to_cycle_id: this._complete.moveToCycleId || undefined,
    });
    this._complete = null;
  }

  protected render() {
    const p = this.project;
    if (!p) return nothing;
    const canManageProject = hasProjectPermission(
      ProjectPermission.ProjectManage,
    );
    const canManageStatuses = hasProjectPermission(
      ProjectPermission.ProjectStatusManage,
    );
    const canManageCycles = hasProjectPermission(
      ProjectPermission.ProjectCycleManage,
    );
    const list = cycles.value.projectId === this.#projectId
      ? cycles.value.cycles
      : [];

    const SV = `
plume-settings-view { display: block; }
.sv-wrap {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  width: 100%;
  max-width: 42rem;
}
.sv-h2 {
  font-size: var(--text-lg);
  font-weight: 600;
  color: var(--foreground);
  margin: 0;
}
.sv-section { display: flex; flex-direction: column; gap: var(--space-2); }
.sv-kv { display: flex; flex-direction: column; gap: var(--space-0-5); }
.sv-label { font-size: var(--text-sm); font-weight: 500; color: var(--foreground); }
.sv-value { font-size: var(--text-sm); color: var(--muted-foreground); }
.sv-help { font-size: var(--text-sm); color: var(--muted-foreground); margin: 0; }
.sv-row { display: flex; align-items: center; gap: var(--space-2); }
.sv-field-label { font-size: var(--text-xs); font-weight: 500; color: var(--muted-foreground); margin-bottom: var(--space-1); }
.sv-select { width: 100%; max-width: var(--space-64); }
.sv-danger-zone { padding-top: var(--space-4); border-top: 1px solid var(--border); }

.sv-cycles-head { display: flex; align-items: center; justify-content: space-between; }
.sv-cycle-list { display: flex; flex-direction: column; gap: var(--space-2); margin-top: var(--space-3); }
.sv-cycle {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: var(--space-3);
}
.sv-cycle-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--space-1); }
.sv-cycle-title { display: flex; align-items: center; gap: var(--space-2); }
.sv-cycle-name { font-size: var(--text-sm); font-weight: 500; color: var(--foreground); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sv-badge-active {
  font-size: var(--text-2xs); font-weight: 500;
  color: oklch(0.55 0.15 145);
  background: color-mix(in oklch, oklch(0.55 0.15 145) 15%, transparent);
  padding: 0 var(--space-1); border-radius: var(--radius-sm);
}
.sv-badge-done { font-size: var(--text-2xs); color: var(--muted-foreground); }
.sv-cycle-dates { font-size: var(--text-2xs); color: var(--muted-foreground); }
.sv-progress { display: flex; align-items: center; gap: var(--space-2); }
.sv-progress-track {
  height: var(--space-1-5);
  flex: 1; max-width: var(--space-32);
  background: var(--secondary);
  border-radius: var(--radius-full);
  overflow: hidden;
}
.sv-progress-bar { height: 100%; background: var(--primary); border-radius: var(--radius-full); }
.sv-progress-count { font-size: var(--text-2xs); color: var(--muted-foreground); }
.sv-goal { font-size: var(--text-2xs); color: var(--muted-foreground); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sv-cycle-actions { display: flex; align-items: center; gap: var(--space-1); flex-shrink: 0; }
.sv-cycle-btn { height: var(--control-h-sm); padding: 0 var(--space-2); border: none; border-radius: var(--radius-sm); background: transparent; color: var(--muted-foreground); font-size: var(--text-xs); font-family: inherit; cursor: pointer; }
.sv-cycle-btn:hover { background: var(--accent); color: var(--foreground); }
.sv-cycle-btn.warn:hover { color: oklch(0.6 0.16 65); }
.sv-cycle-icon { display: inline-flex; align-items: center; justify-content: center; width: var(--space-7); height: var(--space-7); border: none; border-radius: var(--radius-sm); background: transparent; color: var(--muted-foreground); cursor: pointer; }
.sv-cycle-icon:hover { background: var(--accent); color: var(--foreground); }
.sv-cycle-icon.destructive:hover { color: var(--destructive); }
.sv-complete-body { display: flex; flex-direction: column; gap: var(--space-3); }
.sv-complete-footer { display: flex; gap: var(--space-2); width: 100%; }
.sv-empty { font-size: var(--text-sm); color: var(--muted-foreground); margin-top: var(--space-2); }
`;

    return html`
      <style>
      ${SV}
      </style>
      <div class="sv-wrap">
        <div class="sv-section">
          <h2 class="sv-h2">${msg("General")}</h2>
          <div class="sv-kv">
            <span class="sv-label">${msg("Name")}</span>
            <span class="sv-value">${p.name}</span>
          </div>
          <div class="sv-kv">
            <span class="sv-label">${msg("Slug")}</span>
            <span class="sv-value">${p.slug}</span>
          </div>
          ${p.cycle_duration
            ? html`
              <div class="sv-kv">
                <span class="sv-label">${msg("Cycle duration")}</span>
                <span class="sv-value">${p.cycle_duration} days</span>
              </div>
            `
            : nothing}
        </div>

        ${canManageProject
          ? (!this.#hasCycles
            ? html`
              <div class="sv-section">
                <h2 class="sv-h2">${msg("Cycles")}</h2>
                <p class="sv-help">
                  ${msg(
                    "Cycles help you plan and track work in time-boxed iterations.",
                  )}
                </p>
                <plume-button
                  variant="outline"
                  ?disabled="${this._enabling}"
                  @click="${this.#enableCycles}"
                >${this._enabling
                  ? msg("Enabling…")
                  : msg("Enable cycles (14-day)")}
                </plume-button>
              </div>
            `
            : html`
              <div class="sv-section">
                <h2 class="sv-h2">${msg("Cycle config")}</h2>
                <label class="sv-row">
                  <plume-switch
                    ?checked="${this._autoGenerate}"
                    @change="${(
                      e: CustomEvent,
                    ) => (this._autoGenerate = e.detail.checked)}"
                  ></plume-switch>
                  <span class="sv-label">${msg(
                    "Auto-generate next cycle on completion",
                  )}</span>
                </label>
                <div>
                  <div class="sv-field-label">${msg(
                    "Incomplete task handling",
                  )}</div>
                  <plume-select
                    class="sv-select"
                    .options="${[
                      { value: "next_cycle", label: msg("Move to next cycle") },
                      {
                        value: "backlog",
                        label: msg("Move to backlog (no cycle)"),
                      },
                    ]}"
                    .value="${this._handling}"
                    @change="${(e: CustomEvent) => (this._handling = e.detail)}"
                  ></plume-select>
                </div>
                <plume-button
                  variant="outline"
                  size="sm"
                  ?disabled="${this._savingSettings}"
                  @click="${this.#saveSettings}"
                >${this._savingSettings
                  ? msg("Saving…")
                  : msg("Save cycle settings")}
                </plume-button>
              </div>
            `)
          : nothing} ${canManageStatuses
          ? html`
            <plume-status-settings
              .projectId="${this.#projectId}"
              .statuses="${this.statuses}"
            ></plume-status-settings>
          `
          : nothing} ${this.#hasCycles && canManageCycles
          ? html`
            <div class="sv-section">
              <div class="sv-cycles-head">
                <h2 class="sv-h2">${msg("Cycles")}</h2>
                <plume-button
                  variant="outline"
                  size="sm"
                  @click="${() => {
                    this._editCycle = null;
                    this._cycleDialogOpen = true;
                  }}"
                >
                  <plume-icon name="plus" size="14"></plume-icon>
                  ${msg("New cycle")}
                </plume-button>
              </div>
              ${list.length === 0
                ? html`
                  <p class="sv-empty">${msg(
                    "No cycles yet. Create one to start.",
                  )}</p>
                `
                : html`
                  <div class="sv-cycle-list">
                    ${list.map((c) => this.#renderCycle(c))}
                  </div>
                `}
            </div>
          `
          : nothing}

        ${canManageProject
          ? html`
            <div class="sv-section">
              <h2 class="sv-h2">${msg("Task Templates")}</h2>
              <plume-template-manager
                .projectId="${this.#projectId}"
                .statuses="${this.statuses}"
              ></plume-template-manager>
            </div>
            <div class="sv-section">
              <h2 class="sv-h2">${msg("Custom Fields")}</h2>
              <plume-custom-field-manager
                .projectId="${this.#projectId}"
              ></plume-custom-field-manager>
            </div>
          `
          : nothing}

        <div class="sv-section">
          <h2 class="sv-h2">${msg("Export")}</h2>
          <div class="sv-row">
            <plume-button
              variant="outline"
              size="sm"
              @click="${() => this.#exportTasks("csv")}"
            >${msg("Export tasks (CSV)")}</plume-button>
            <plume-button
              variant="outline"
              size="sm"
              @click="${() => this.#exportTasks("json")}"
            >${msg("Export tasks (JSON)")}</plume-button>
            <plume-button
              variant="outline"
              size="sm"
              @click="${() => this.#exportTime("csv")}"
            >${msg("Export time (CSV)")}</plume-button>
          </div>
        </div>

        ${canManageProject
          ? html`
            <div class="sv-section sv-danger-zone">
              <h2 class="sv-h2">${msg("Danger Zone")}</h2>
              ${p.is_archived
                ? html`
                  <div class="sv-row">
                    <span class="sv-help">${msg(
                      "This project is archived.",
                    )}</span>
                    <plume-button
                      variant="default"
                      size="sm"
                      @click="${() => this.#unarchive()}"
                    >${msg("Unarchive")}</plume-button>
                  </div>
                `
                : html`
                  <div class="sv-row">
                    <span
                      class="sv-help">${msg(
                        "Archive this project: it stays accessible but hidden and read-only.",
                      )}</span>
                    <plume-button
                      variant="outline"
                      size="sm"
                      @click="${() => this.#archive()}"
                    >${msg("Archive project")}</plume-button>
                  </div>
                `}
            </div>
          `
          : nothing}
      </div>

      <plume-cycle-dialog
        .open="${this._cycleDialogOpen}"
        .project="${this.project}"
        .cycle="${this._editCycle}"
        @close="${() => (this._cycleDialogOpen = false)}"
        @saved="${() => {
          this._cycleDialogOpen = false;
          void fetchCycles(this.#projectId, true);
        }}"
      ></plume-cycle-dialog>

      <plume-dialog
        .open="${!!this._complete}"
        heading="${`${msg("Complete")} “${this._complete?.cycle.name ?? ""}”`}"
        @close="${() => (this._complete = null)}"
      >
        ${this._complete
          ? html`
            <div class="sv-complete-body">
              <p class="sv-help" style="margin:0">
                ${`${this._complete.cycle.task_count ?? 0} ${
                  msg("tasks in this cycle")
                } (${this._complete.cycle.completed_task_count ?? 0} ${
                  msg("completed")
                }).`}
              </p>
              <div>
                <div class="sv-field-label">${msg(
                  "Move incomplete tasks to",
                )}</div>
                <plume-select
                  class="sv-select"
                  .options="${[
                    { value: "", label: msg("Auto (next cycle)") },
                    ...list
                      .filter(
                        (c) =>
                          c.id !== this._complete!.cycle.id &&
                          !c.is_completed && !c.is_active,
                      )
                      .map((c) => ({ value: c.id ?? "", label: c.name ?? "" })),
                  ]}"
                  .value="${this._complete.moveToCycleId}"
                  @change="${(e: CustomEvent) => (this._complete = {
                    ...this._complete!,
                    moveToCycleId: e.detail,
                  })}"
                ></plume-select>
              </div>
            </div>
            <div class="sv-complete-footer" slot="footer">
              <span style="flex:1"></span>
              <plume-button variant="outline" size="sm" @click="${() => (this
                ._complete = null)}"
              >${msg("Cancel")}</plume-button>
              <plume-button size="sm" @click="${this
                .#confirmComplete}">${msg("Complete cycle")}</plume-button>
            </div>
          `
          : nothing}
      </plume-dialog>
    `;
  }

  #renderCycle(c: DtoCycleResponse) {
    const total = c.task_count ?? 0;
    const done = c.completed_task_count ?? 0;
    const progress = total > 0 ? Math.round((done / total) * 100) : 0;
    const starts = c.starts_at ? new Date(c.starts_at) : null;
    const ends = c.ends_at ? new Date(c.ends_at) : null;

    return html`
      <div class="sv-cycle">
        <div class="sv-cycle-main">
          <div class="sv-cycle-title">
            ${c.is_active
              ? html`
                <plume-icon
                  name="circle-check"
                  size="16"
                  style="color:oklch(0.55 0.15 145)"
                ></plume-icon>
              `
              : c.is_completed
              ? html`
                <plume-icon
                  name="circle-check"
                  size="16"
                  style="color:var(--muted-foreground)"
                ></plume-icon>
              `
              : html`
                <plume-icon
                  name="circle"
                  size="16"
                  style="color:var(--muted-foreground)"
                ></plume-icon>
              `}
            <span class="sv-cycle-name">${c.name}</span>
            ${c.is_active
              ? html`
                <span class="sv-badge-active">${msg("Active")}</span>
              `
              : nothing} ${c.is_completed && !c.is_active
              ? html`
                <span class="sv-badge-done">${msg("Completed")}</span>
              `
              : nothing}
          </div>
          ${starts && ends
            ? html`
              <span class="sv-cycle-dates">
                ${fmtDate(starts)}: ${fmtDateYear(ends)}
              </span>
            `
            : nothing}
          <div class="sv-progress">
            <div class="sv-progress-track">
              <div class="sv-progress-bar" style="width:${progress}%"></div>
            </div>
            <span class="sv-progress-count">${done}/${total}</span>
          </div>
          ${c.goal
            ? html`
              <span class="sv-goal">${c.goal}</span>
            `
            : nothing}
        </div>
        <div class="sv-cycle-actions">
          ${!c.is_completed && !c.is_active
            ? html`
              <button class="sv-cycle-btn" type="button" @click="${() =>
                this.#activate(c.id!)}">
                ${msg("Activate")}
              </button>
            `
            : nothing} ${c.is_active
            ? html`
              <button class="sv-cycle-btn warn" type="button" @click="${() => (this
                ._complete = { cycle: c, moveToCycleId: "" })}">
                ${msg("Complete")}
              </button>
            `
            : nothing}
          <button
            class="sv-cycle-icon"
            type="button"
            aria-label=${msg("Edit cycle")}
            @click="${() => {
              this._editCycle = c;
              this._cycleDialogOpen = true;
            }}"
          >
            <plume-icon name="pencil" size="14"></plume-icon>
          </button>
          <button
            class="sv-cycle-icon destructive"
            type="button"
            aria-label=${msg("Delete cycle")}
            @click="${() => this.#deleteCycle(c.id!)}"
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
    "plume-settings-view": PlumeSettingsView;
  }
}
