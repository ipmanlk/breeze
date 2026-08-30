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
import { BreezeInput } from "../../components/ui/input.ts";
import { BreezeSelect } from "../../components/ui/select.ts";
import "../../components/ui/button.ts";
import "../../components/ui/dialog.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/field.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-template-manager")
export class BreezeTemplateManager extends LitElement {
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
  private _tmplNameInput!: BreezeInput | null;
  @query("#tmpl-desc")
  private _tmplDescInput!: BreezeInput | null;
  @query("#tmpl-status")
  private _tmplStatusSelect!: BreezeSelect | null;
  @query("#tmpl-priority")
  private _tmplPrioritySelect!: BreezeSelect | null;
  @query("#tmpl-recurrence")
  private _tmplRecurrenceSelect!: BreezeSelect | null;
  @query("#tmpl-days")
  private _tmplDaysInput!: BreezeInput | null;

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
          <breeze-button
            variant="outline"
            size="sm"
            @click="${() => this.#openCreate()}"
          >
            <breeze-icon name="plus" size="14"></breeze-icon>
            New template
          </breeze-button>
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
          <breeze-button
            variant="ghost"
            size="sm"
            @click="${() => this.#instantiate(t.id!)}"
          >Use</breeze-button>
          <breeze-button
            variant="ghost"
            size="sm"
            @click="${() => this.#openEdit(t)}"
          >Edit</breeze-button>
          <breeze-button
            variant="ghost"
            size="sm"
            @click="${() => this.#delete(t.id!)}"
          >Delete</breeze-button>
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
      <breeze-dialog
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
            <breeze-field label="Name">
              <breeze-input
                id="tmpl-name"
                .value="${name}"
                placeholder=${msg("e.g. Weekly sprint review")}
              ></breeze-input>
            </breeze-field>
            <breeze-field label="Description">
              <breeze-input
                id="tmpl-desc"
                .value="${description}"
                placeholder=${msg("Template description")}
              ></breeze-input>
            </breeze-field>
            <div class="tm-form-row">
              <breeze-field label="Status">
                <breeze-select
                  id="tmpl-status"
                  .options="${this.statuses.map((s) => ({
                    label: s.name!,
                    value: s.id!,
                  }))}"
                  .value="${statusId}"
                ></breeze-select>
              </breeze-field>
              <breeze-field label="Priority">
                <breeze-select
                  id="tmpl-priority"
                  .options="${[
                    { label: msg("None"), value: "none" },
                    { label: msg("Low"), value: "low" },
                    { label: msg("Medium"), value: "medium" },
                    { label: msg("High"), value: "high" },
                    { label: msg("Urgent"), value: "urgent" },
                  ]}"
                  .value="${priority}"
                ></breeze-select>
              </breeze-field>
            </div>
            <div class="tm-form-row">
              <breeze-field label="Recurrence">
                <breeze-select
                  id="tmpl-recurrence"
                  .options="${[
                    { label: msg("None (manual)"), value: "none" },
                    { label: msg("Daily"), value: "daily" },
                    { label: msg("Weekly"), value: "weekly" },
                    { label: msg("Monthly"), value: "monthly" },
                  ]}"
                  .value="${recurrence}"
                ></breeze-select>
              </breeze-field>
              <breeze-field
                label="Recurrence days (weekly: 0-6 comma-sep, monthly: day num)">
                <breeze-input
                  id="tmpl-days"
                  .value="${recurrenceDays}"
                  placeholder=${msg("e.g. 1,3,5")}
                ></breeze-input>
              </breeze-field>
            </div>
          </div>
        </form>
        <div slot="footer" class="tm-dialog-footer">
          <breeze-button variant="ghost" @click="${() => (this._dialogOpen =
            false)}">
            Cancel
          </breeze-button>
          <breeze-button variant="default" type="submit" form="tmpl-form">
            ${t ? "Save" : "Create"}
          </breeze-button>
        </div>
      </breeze-dialog>
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
    "breeze-template-manager": BreezeTemplateManager;
  }
}
