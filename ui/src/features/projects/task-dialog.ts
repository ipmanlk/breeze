import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import type {
  DtoCreateTaskRequest,
  DtoCycleResponse,
  DtoProjectMemberResponse,
  DtoProjectResponse,
  DtoTaskStatusResponse,
} from "@/api";
import { getProjectsByIdCycles, getProjectsByIdMembers } from "@/api";
import { createTask } from "@/store/project-detail";
import "../../components/ui/dialog.ts";
import "../../components/ui/select.ts";
import "../../components/ui/combobox.ts";
import "../../components/ui/tabs.ts";
import "../../components/ui/input.ts";
import "../../components/ui/field.ts";
import "../../components/ui/button.ts";
import "../../components/ui/plume-icon.ts";
import { localized, msg } from "@lit/localize";

function getPriorities() {
  return [
    {
      value: "none",
      label: msg("No priority"),
      color: "var(--muted-foreground)",
    },
    { value: "low", label: msg("Low"), color: "oklch(0.7 0.15 250)" },
    { value: "medium", label: msg("Medium"), color: "oklch(0.8 0.15 85)" },
    { value: "high", label: msg("High"), color: "oklch(0.7 0.18 50)" },
    { value: "urgent", label: msg("Urgent"), color: "oklch(0.6 0.22 27)" },
  ];
}

/**
 * Plume task dialog: "Create task" modal.
 *
 * Properties:
 *  - `open`      : controls visibility (delegated to `plume-dialog`)
 *  - `project`   : the project context (for cycles, project ID)
 *  - `statuses`  : available task statuses
 *  - `defaultStatusId`: pre-selected status
 *
 * Events:
 *  - `close`  : dispatched when dialog closes
 *  - `created`: detail = created DtoTaskResponse
 *
 * Fetches project members and cycles when the dialog opens.  Form state is
 * reset on every open.
 */
@localized()
@customElement("plume-task-dialog")
export class PlumeTaskDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    .td-body {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }
    .td-footer {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
    }
    .td-error {
      color: var(--destructive);
      font-size: var(--text-sm);
    }
    .td-spacer {
      flex: 1;
    }
    .td-tab-content {
      flex: 1;
      min-height: var(--space-80);
      max-height: var(--space-96);
      overflow-y: auto;
      padding-top: var(--space-4);
    }
    .td-tab-content::-webkit-scrollbar {
      width: var(--space-1);
    }
    .td-tab-content::-webkit-scrollbar-track {
      background: transparent;
    }
    .td-tab-content::-webkit-scrollbar-thumb {
      background: var(--border);
      border-radius: var(--radius-full);
    }
    .td-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      column-gap: var(--space-4);
      row-gap: var(--space-5);
    }
    .td-grid .full {
      grid-column: 1 / -1;
    }
    .td-desc {
      width: 100%;
      min-height: var(--space-32);
      padding: var(--space-3);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      resize: vertical;
      outline: none;
      box-sizing: border-box;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        box-shadow var(--dur-fast) var(--ease-1);
    }
    .td-desc:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .td-desc::placeholder {
      color: var(--muted-foreground);
    }
  `;

  @property({ type: Boolean })
  open = false;

  @property({ type: Object, attribute: false })
  project: DtoProjectResponse | null = null;

  @property({ type: Array, attribute: false })
  statuses: DtoTaskStatusResponse[] = [];

  @property()
  defaultStatusId = "";

  /** When set, the created task becomes a subtask of this parent. */
  @property()
  parentId = "";

  // Form state
  @state()
  private _title = "";
  @state()
  private _description = "";
  @state()
  private _statusId = "";
  @state()
  private _priority = "none";
  @state()
  private _assigneeIds: string[] = [];
  @state()
  private _cycleId: string | null = null;
  @state()
  private _estimate: number | null = null;
  @state()
  private _startedAt = "";
  @state()
  private _dueAt = "";
  @state()
  private _tab = "properties";
  @state()
  private _submitting = false;
  @state()
  private _error = "";

  // Fetched data
  @state()
  private _members: DtoProjectMemberResponse[] = [];
  @state()
  private _cycles: DtoCycleResponse[] = [];

  private _prevOpen = false;
  private _loadedProjectId = "";

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("open")) {
      if (this.open && !this._prevOpen) {
        this._reset();
        this._loadData();
      }
      this._prevOpen = this.open;
    }
  }

  private _reset() {
    this._title = "";
    this._description = "";
    this._statusId = this.defaultStatusId || this.statuses[0]?.id || "";
    this._priority = "none";
    this._assigneeIds = [];
    this._cycleId = null;
    this._estimate = null;
    this._startedAt = "";
    this._dueAt = "";
    this._tab = "properties";
    this._error = "";
    this._submitting = false;
  }

  private async _loadData() {
    const pid = this.project?.id;
    if (!pid || pid === this._loadedProjectId) return;
    this._loadedProjectId = pid;

    const hasCycles = (this.project?.cycle_duration ?? 0) > 0;
    const [membersRes, cyclesRes] = await Promise.all([
      getProjectsByIdMembers({ path: { id: pid }, throwOnError: true })
        .catch(() => null),
      hasCycles
        ? getProjectsByIdCycles({ path: { id: pid }, throwOnError: true })
          .catch(() => null)
        : Promise.resolve(null),
    ]);

    this._members = membersRes?.data?.items ?? [];
    this._cycles = (cyclesRes?.data) ?? [];
  }

  private get _isValid() {
    return this._title.trim() && this._statusId;
  }

  private async _submit() {
    if (!this._isValid || !this.project?.id) return;
    this._submitting = true;
    this._error = "";

    const body: DtoCreateTaskRequest = {
      title: this._title.trim(),
      description: this._description,
      status_id: this._statusId,
      priority: this._priority,
      assignee_ids: this._assigneeIds,
    };
    if (this.parentId) body.parent_task_id = this.parentId;
    if (this._cycleId) body.cycle_id = this._cycleId;
    if (this._estimate != null) body.estimate = this._estimate;
    if (this._startedAt) {
      body.started_at = new Date(this._startedAt).toISOString();
    }
    if (this._dueAt) body.due_at = new Date(this._dueAt).toISOString();

    try {
      const task = await createTask(this.project.id, body);
      if (!task?.id) throw new Error("No task ID returned");

      this.dispatchEvent(
        new CustomEvent("created", {
          detail: task,
          bubbles: true,
          composed: true,
        }),
      );
      this.open = false;
    } catch {
      this._error = msg("Failed to create task");
    } finally {
      this._submitting = false;
    }
  }

  protected render() {
    const hasCycles = (this.project?.cycle_duration ?? 0) > 0;
    const statusOptions = this.statuses.map((s) => ({
      value: s.id ?? "",
      label: s.name ?? "",
      color: s.color,
    }));
    const memberOptions = this._members.map((m) => ({
      value: m.id ?? "",
      label: m.name ?? "Unknown",
      avatarUrl: m.avatar_url,
    }));
    const cycleOptions = [
      { value: "", label: msg("No cycle") },
      ...this._cycles.map((c) => ({ value: c.id ?? "", label: c.name ?? "" })),
    ];

    return html`
      <plume-dialog
        .open="${this.open}"
        heading="${this.parentId ? "Create subtask" : "Create task"}"
        @close="${() => {
          this.dispatchEvent(
            new CustomEvent("close", { bubbles: true, composed: true }),
          );
        }}"
      >
        <div class="td-body">
          <plume-field label="Task title">
            <plume-input
              placeholder=${msg("Enter task title...")}
              .value="${this._title}"
              @input="${(
                e: Event,
              ) => (this._title = (e.target as HTMLInputElement).value)}"
            ></plume-input>
          </plume-field>

          <plume-tabs
            .tabs="${[
              { id: "properties", label: msg("Properties") },
              { id: "description", label: msg("Description") },
            ]}"
            .value="${this._tab}"
            @change="${(e: CustomEvent) => (this._tab = e.detail)}"
          ></plume-tabs>

          <div class="td-tab-content" role="tabpanel"
            aria-labelledby="tab-${this._tab}">
            ${this._tab === "properties"
              ? this._renderProperties(
                statusOptions,
                memberOptions,
                cycleOptions,
                hasCycles,
              )
              : this._tab === "description"
              ? this._renderDescription()
              : nothing}
          </div>
        </div>

        <div class="td-footer" slot="footer">
          ${this._error
            ? html`
              <span class="td-error">${this._error}</span>
            `
            : nothing}
          <span class="td-spacer"></span>
          <plume-button
            variant="ghost"
            @click="${() => (this.open = false)}"
          >Cancel</plume-button>
          <plume-button
            ?disabled="${this._submitting || !this._isValid}"
            @click="${this._submit}"
          >${this._submitting ? "Creating..." : "Create task"}</plume-button>
        </div>
      </plume-dialog>
    `;
  }

  private _renderProperties(
    statusOptions: { value: string; label: string; color?: string }[],
    memberOptions: { value: string; label: string; avatarUrl?: string }[],
    cycleOptions: { value: string; label: string }[],
    hasCycles: boolean,
  ) {
    return html`
      <div class="td-grid">
        <plume-field label="Status">
          <plume-select
            .options="${statusOptions}"
            .value="${this._statusId}"
            @change="${(e: CustomEvent) => (this._statusId = e.detail)}"
          ></plume-select>
        </plume-field>
        <plume-field label="Priority">
          <plume-select
            .options="${getPriorities()}"
            .value="${this._priority}"
            @change="${(e: CustomEvent) => (this._priority = e.detail)}"
          ></plume-select>
        </plume-field>
        <plume-field label="Assignees" class="full">
          <plume-combobox
            .options="${memberOptions}"
            .value="${this._assigneeIds}"
            placeholder=${msg("Assignees")}
            @change="${(e: CustomEvent) => (this._assigneeIds = e.detail)}"
          ></plume-combobox>
        </plume-field>
        <plume-field label="Start date">
          <plume-input
            type="datetime-local"
            .value="${this._startedAt}"
            @input="${(
              e: Event,
            ) => (this._startedAt = (e.target as HTMLInputElement).value)}"
          ></plume-input>
        </plume-field>
        <plume-field label="Due date">
          <plume-input
            type="datetime-local"
            .value="${this._dueAt}"
            @input="${(
              e: Event,
            ) => (this._dueAt = (e.target as HTMLInputElement).value)}"
          ></plume-input>
        </plume-field>
        ${hasCycles
          ? html`
            <plume-field label="Cycle">
              <plume-select
                .options="${cycleOptions}"
                .value="${this._cycleId ?? ""}"
                @change="${(
                  e: CustomEvent,
                ) => (this._cycleId = e.detail || null)}"
              ></plume-select>
            </plume-field>
          `
          : nothing}
        <plume-field label="Estimate (hours)">
          <plume-input
            type="number"
            min="0"
            placeholder="0"
            .value="${this._estimate?.toString() ?? ""}"
            @input="${(e: Event) => {
              const v = (e.target as HTMLInputElement).value;
              this._estimate = v ? Number(v) : null;
            }}"
          ></plume-input>
        </plume-field>
      </div>
    `;
  }

  private _renderDescription() {
    return html`
      <textarea
        class="td-desc"
        placeholder=${msg("Add description...")}
        .value="${this._description}"
        @input="${(
          e: Event,
        ) => (this._description = (e.target as HTMLTextAreaElement).value)}"
      ></textarea>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-task-dialog": PlumeTaskDialog;
  }
}
