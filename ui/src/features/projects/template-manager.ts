import { logError } from "@/lib/log";
import { html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import type { DtoTaskStatusResponse, DtoTaskTemplateResponse } from "@/api";
import {
  deleteProjectsByIdTemplatesByTemplateId,
  getProjectsByIdTemplates,
  patchProjectsByIdTemplatesByTemplateId,
  postProjectsByIdTemplates,
  postProjectsByIdTemplatesByTemplateIdInstantiate,
} from "@/api";
import { showToast } from "@/components/ui/toast-store";
import { PlumeInput } from "../../components/ui/input.ts";
import { PlumeSelect } from "../../components/ui/select.ts";
import "../../components/ui/button.ts";
import "../../components/ui/dialog.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/field.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("plume-template-manager")
export class PlumeTemplateManager extends LitElement {
  @property()
  projectId = "";
  @property()
  statuses: DtoTaskStatusResponse[] = [];

  @state()
  private _templates: DtoTaskTemplateResponse[] = [];
  @state()
  private _loading = true;
  @state()
  private _dialogOpen = false;
  @state()
  private _editTemplate: DtoTaskTemplateResponse | null = null;

  @query("#tmpl-name")
  private _tmplNameInput!: PlumeInput | null;
  @query("#tmpl-desc")
  private _tmplDescInput!: PlumeInput | null;
  @query("#tmpl-status")
  private _tmplStatusSelect!: PlumeSelect | null;
  @query("#tmpl-priority")
  private _tmplPrioritySelect!: PlumeSelect | null;
  @query("#tmpl-recurrence")
  private _tmplRecurrenceSelect!: PlumeSelect | null;
  @query("#tmpl-days")
  private _tmplDaysInput!: PlumeInput | null;

  createRenderRoot() {
    return this;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.#load();
  }

  async #load(): Promise<void> {
    if (!this.projectId) return;
    this._loading = true;
    try {
      const { data } = await getProjectsByIdTemplates({
        path: { id: this.projectId },
        throwOnError: true,
      });
      this._templates = data ?? [];
    } catch {
      this._templates = [];
    } finally {
      this._loading = false;
    }
  }

  render() {
    return html`
      <div class="tm-wrap">
        <div class="tm-header">
          <h3 class="tm-title">Task Templates</h3>
          <plume-button
            variant="outline"
            size="sm"
            @click="${() => this.#openCreate()}"
          >
            <plume-icon name="plus" size="14"></plume-icon>
            New template
          </plume-button>
        </div>
        ${this._loading
          ? html`<p class="tm-empty">Loading…</p>`
          : this._templates.length === 0
          ? html`
            <p
              class="tm-empty">No templates yet. Create one to quickly instantiate recurring tasks.</p>
          `
          : html`<div class="tm-list">
                ${this._templates.map((t) => this.#renderTemplate(t))}
              </div>`}
        ${this._dialogOpen ? this.#renderDialog() : nothing}
      </div>
    `;
  }

  #renderTemplate(t: DtoTaskTemplateResponse) {
    const recurrenceLabel =
      t.recurrence_pattern && t.recurrence_pattern !== "none"
        ? t.recurrence_pattern
        : "manual";
    return html`
      <div class="tm-item">
        <div class="tm-item-info">
          <span class="tm-item-name">${t.name}</span>
          <span class="tm-item-meta">
            ${t.priority} · ${recurrenceLabel}
            ${t.next_run_at
              ? ` · next: ${new Date(t.next_run_at).toLocaleDateString()}`
              : ""}
          </span>
        </div>
        <div class="tm-item-actions">
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => this.#instantiate(t.id!)}"
          >Use</plume-button>
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => this.#openEdit(t)}"
          >Edit</plume-button>
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => this.#delete(t.id!)}"
          >Delete</plume-button>
        </div>
      </div>
    `;
  }

  #renderDialog() {
    const t = this._editTemplate;
    const name = t?.name ?? "";
    const description = t?.description ?? "";
    const priority = t?.priority ?? "none";
    const statusId = t?.status_id ?? this.statuses[0]?.id ?? "";
    const recurrence = t?.recurrence_pattern ?? "none";
    const recurrenceDays = t?.recurrence_days ?? "";
    return html`
      <plume-dialog
        .open="${true}"
        heading="${t ? "Edit template" : "New template"}"
        @close="${() => (this._dialogOpen = false)}"
      >
        <form
          id="tmpl-form"
          @submit="${(e: Event) => {
            e.preventDefault();
            this.#save();
          }}"
        >
          <div class="tm-form">
            <plume-field label="Name">
              <plume-input
                id="tmpl-name"
                .value="${name}"
                placeholder=${msg("e.g. Weekly sprint review")}
              ></plume-input>
            </plume-field>
            <plume-field label="Description">
              <plume-input
                id="tmpl-desc"
                .value="${description}"
                placeholder=${msg("Template description")}
              ></plume-input>
            </plume-field>
            <div class="tm-form-row">
              <plume-field label="Status">
                <plume-select
                  id="tmpl-status"
                  .options="${this.statuses.map((s) => ({
                    label: s.name!,
                    value: s.id!,
                  }))}"
                  .value="${statusId}"
                ></plume-select>
              </plume-field>
              <plume-field label="Priority">
                <plume-select
                  id="tmpl-priority"
                  .options="${[
                    { label: msg("None"), value: "none" },
                    { label: msg("Low"), value: "low" },
                    { label: msg("Medium"), value: "medium" },
                    { label: msg("High"), value: "high" },
                    { label: msg("Urgent"), value: "urgent" },
                  ]}"
                  .value="${priority}"
                ></plume-select>
              </plume-field>
            </div>
            <div class="tm-form-row">
              <plume-field label="Recurrence">
                <plume-select
                  id="tmpl-recurrence"
                  .options="${[
                    { label: msg("None (manual)"), value: "none" },
                    { label: msg("Daily"), value: "daily" },
                    { label: msg("Weekly"), value: "weekly" },
                    { label: msg("Monthly"), value: "monthly" },
                  ]}"
                  .value="${recurrence}"
                ></plume-select>
              </plume-field>
              <plume-field
                label="Recurrence days (weekly: 0-6 comma-sep, monthly: day num)">
                <plume-input
                  id="tmpl-days"
                  .value="${recurrenceDays}"
                  placeholder=${msg("e.g. 1,3,5")}
                ></plume-input>
              </plume-field>
            </div>
          </div>
        </form>
        <div slot="footer" class="tm-dialog-footer">
          <plume-button variant="ghost" @click="${() => (this._dialogOpen =
            false)}">
            Cancel
          </plume-button>
          <plume-button variant="default" type="submit" form="tmpl-form">
            ${t ? "Save" : "Create"}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }

  #openCreate(): void {
    this._editTemplate = null;
    this._dialogOpen = true;
  }

  #openEdit(t: DtoTaskTemplateResponse): void {
    this._editTemplate = t;
    this._dialogOpen = true;
  }

  async #save(): Promise<void> {
    const name = this._tmplNameInput?.value?.trim();
    if (!name) return;
    const description = this._tmplDescInput?.value ?? "";
    const statusId = this._tmplStatusSelect?.value ?? "";
    const priority = this._tmplPrioritySelect?.value ?? "none";
    const recurrence = this._tmplRecurrenceSelect?.value ?? "none";
    const recurrenceDays = this._tmplDaysInput?.value ?? "";

    const body = {
      name,
      description,
      status_id: statusId,
      priority,
      assignee_ids: [] as string[],
      recurrence_pattern: recurrence,
      recurrence_days: recurrenceDays,
    };

    try {
      if (this._editTemplate) {
        await patchProjectsByIdTemplatesByTemplateId({
          path: { id: this.projectId, templateId: this._editTemplate.id! },
          body,
          throwOnError: true,
        });
        showToast(msg("Template updated"), { variant: "success" });
      } else {
        await postProjectsByIdTemplates({
          path: { id: this.projectId },
          body,
          throwOnError: true,
        });
        showToast(msg("Template created"), { variant: "success" });
      }
      this._dialogOpen = false;
      await this.#load();
    } catch (err) {
      logError("save template failed:", err);
      showToast(msg("Failed to save template"), { variant: "error" });
    }
  }

  async #delete(id: string): Promise<void> {
    if (!confirm(msg("Delete this template?"))) return;
    try {
      await deleteProjectsByIdTemplatesByTemplateId({
        path: { id: this.projectId, templateId: id },
        throwOnError: true,
      });
      showToast(msg("Template deleted"), { variant: "success" });
      await this.#load();
    } catch (err) {
      logError("delete template failed:", err);
      showToast(msg("Failed to delete template"), { variant: "error" });
    }
  }

  async #instantiate(id: string): Promise<void> {
    try {
      await postProjectsByIdTemplatesByTemplateIdInstantiate({
        path: { id: this.projectId, templateId: id },
        throwOnError: true,
      });
      showToast(msg("Task created from template"), { variant: "success" });
    } catch (err) {
      logError("instantiate failed:", err);
      showToast(msg("Failed to create task from template"), {
        variant: "error",
      });
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-template-manager": PlumeTemplateManager;
  }
}
