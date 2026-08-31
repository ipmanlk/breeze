import { logError } from "@/lib/log";
import { html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import type { DtoCustomFieldResponse } from "@/api";
import {
  deleteProjectsByIdCustomFieldsByFieldId,
  getProjectsByIdCustomFields,
  patchProjectsByIdCustomFieldsByFieldId,
  postProjectsByIdCustomFields,
} from "@/api";
import { showToast } from "@/components/ui/toast-store";
import { PlumeInput } from "../../components/ui/input.ts";
import { PlumeSelect } from "../../components/ui/select.ts";
import "../../components/ui/button.ts";
import "../../components/ui/dialog.ts";
import "../../components/ui/plume-icon.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("plume-custom-field-manager")
export class PlumeCustomFieldManager extends LitElement {
  @property()
  projectId = "";

  @state()
  private _fields: DtoCustomFieldResponse[] = [];
  @state()
  private _loading = true;
  @state()
  private _dialogOpen = false;
  @state()
  private _editField: DtoCustomFieldResponse | null = null;

  @query("#cf-name")
  private _cfNameInput!: PlumeInput | null;
  @query("#cf-type")
  private _cfTypeSelect!: PlumeSelect | null;
  @query("#cf-options")
  private _cfOptionsInput!: PlumeInput | null;

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
      const { data } = await getProjectsByIdCustomFields({
        path: { id: this.projectId },
        throwOnError: true,
      });
      this._fields = data ?? [];
    } catch {
      this._fields = [];
    } finally {
      this._loading = false;
    }
  }

  render() {
    return html`
      <div class="cfm-wrap">
        <div class="cfm-header">
          <h3 class="cfm-title">Custom Fields</h3>
          <plume-button
            variant="outline"
            size="sm"
            @click="${() => this.#openCreate()}"
          >
            <plume-icon name="plus" size="14"></plume-icon>
            New field
          </plume-button>
        </div>
        ${this._loading
          ? html`<p class="cfm-empty">Loading…</p>`
          : this._fields.length === 0
          ? html`
            <p
              class="cfm-empty">No custom fields yet. Add fields to track extra metadata on tasks.</p>
          `
          : html`<div class="cfm-list">
                ${this._fields.map((f) => this.#renderField(f))}
              </div>`}
        ${this._dialogOpen ? this.#renderDialog() : nothing}
      </div>
    `;
  }

  #renderField(f: DtoCustomFieldResponse) {
    return html`
      <div class="cfm-item">
        <div class="cfm-item-info">
          <span class="cfm-item-name">${f.name}</span>
          <span class="cfm-item-meta">
            ${f.field_type}${f.options && f.options.length > 0
              ? ` · ${f.options.join(", ")}`
              : ""}
          </span>
        </div>
        <div class="cfm-item-actions">
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => this.#openEdit(f)}"
          >Edit</plume-button>
          <plume-button
            variant="ghost"
            size="sm"
            @click="${() => this.#delete(f.id!)}"
          >Delete</plume-button>
        </div>
      </div>
    `;
  }

  #renderDialog() {
    const f = this._editField;
    const name = f?.name ?? "";
    const fieldType = f?.field_type ?? "text";
    const options = f?.options?.join(", ") ?? "";
    return html`
      <plume-dialog
        .open="${true}"
        heading="${f ? "Edit field" : "New custom field"}"
        @close="${() => (this._dialogOpen = false)}"
      >
        <form
          id="cf-form"
          @submit="${(e: Event) => {
            e.preventDefault();
            this.#save();
          }}"
        >
          <div class="cfm-form">
            <plume-field label="Name">
              <plume-input
                id="cf-name"
                .value="${name}"
                placeholder=${msg("e.g. Story Points")}
              ></plume-input>
            </plume-field>
            ${!f
              ? html`
                <plume-field label="Type">
                  <plume-select
                    id="cf-type"
                    .options="${[
                      { label: msg("Text"), value: "text" },
                      { label: msg("Number"), value: "number" },
                      { label: msg("Select"), value: "select" },
                      { label: msg("Date"), value: "date" },
                    ]}"
                    .value="${fieldType}"
                  ></plume-select>
                </plume-field>
              `
              : nothing}
            <plume-field label="Options (comma-separated, for select type)">
              <plume-input
                id="cf-options"
                .value="${options}"
                placeholder=${msg("e.g. Low, Medium, High")}
              ></plume-input>
            </plume-field>
          </div>
        </form>
        <div slot="footer" class="cfm-dialog-footer">
          <plume-button variant="ghost" @click="${() => (this._dialogOpen =
            false)}">
            Cancel
          </plume-button>
          <plume-button variant="default" type="submit" form="cf-form">
            ${f ? "Save" : "Create"}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }

  #openCreate(): void {
    this._editField = null;
    this._dialogOpen = true;
  }

  #openEdit(f: DtoCustomFieldResponse): void {
    this._editField = f;
    this._dialogOpen = true;
  }

  async #save(): Promise<void> {
    const name = this._cfNameInput?.value?.trim();
    if (!name) return;
    const fieldType = this._cfTypeSelect?.value ?? "text";
    const optionsStr = this._cfOptionsInput?.value ?? "";
    const options = optionsStr
      .split(",")
      .map((s) => s.trim())
      .filter((s) => s.length > 0);

    try {
      if (this._editField) {
        await patchProjectsByIdCustomFieldsByFieldId({
          path: { id: this.projectId, fieldId: this._editField.id! },
          body: { name, options, position: this._editField.position ?? 0 },
          throwOnError: true,
        });
        showToast(msg("Field updated"), { variant: "success" });
      } else {
        await postProjectsByIdCustomFields({
          path: { id: this.projectId },
          body: { name, field_type: fieldType, options, position: 0 },
          throwOnError: true,
        });
        showToast(msg("Field created"), { variant: "success" });
      }
      this._dialogOpen = false;
      await this.#load();
    } catch (err) {
      logError("save field failed:", err);
      showToast(msg("Failed to save field"), { variant: "error" });
    }
  }

  async #delete(id: string): Promise<void> {
    if (
      !confirm(
        msg("Delete this custom field? Values on tasks will be removed."),
      )
    ) return;
    try {
      await deleteProjectsByIdCustomFieldsByFieldId({
        path: { id: this.projectId, fieldId: id },
        throwOnError: true,
      });
      showToast(msg("Field deleted"), { variant: "success" });
      await this.#load();
    } catch (err) {
      logError("delete field failed:", err);
      showToast(msg("Failed to delete field"), { variant: "error" });
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-custom-field-manager": PlumeCustomFieldManager;
  }
}
