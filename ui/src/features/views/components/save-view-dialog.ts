import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import type { SelectOption } from "../../../components/ui/select";
import type { ViewFilters, ViewLayout } from "../types";
import { createView, updateView } from "../store";
import "../../../components/ui/dialog.ts";
import "../../../components/ui/input.ts";
import "../../../components/ui/select.ts";
import "../../../components/ui/button.ts";

function getLayoutOptions(): SelectOption[] {
  return [
    { value: "board", label: msg("Board") },
    { value: "list", label: msg("List") },
  ];
}

/**
 * Save / edit view dialog.
 *
 * In create mode: `viewId` is unset, name/layout start blank.
 * In edit mode: `viewId` is set, name/layout are pre-filled.
 */
@localized()
@customElement("plume-save-view-dialog")
export class PlumeSaveViewDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }
    .form {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: var(--space-1-5);
    }
    .field-label {
      font-size: var(--text-sm);
      font-weight: 500;
      color: var(--foreground);
    }
    .footer-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: var(--space-2);
    }
    .delete-btn {
      margin-right: auto;
    }
  `;

  @property({ type: Boolean })
  open = false;

  /** Set to edit an existing view. */
  @property()
  viewId = "";

  @property()
  projectId = "";

  /** Pre-filled view name (edit mode). */
  @property()
  viewName = "";

  /** Pre-filled layout (edit mode). */
  @property()
  viewLayout: ViewLayout = "board";

  /** Default layout when creating (used when viewLayout is not pre-filled). */
  @property()
  defaultLayout: ViewLayout = "board";

  /** The view's existing filters (edit mode). */
  @property({ attribute: false })
  existingFilters: ViewFilters = {};

  /** Filters to save: passed from the parent (project page). */
  @property({ attribute: false })
  filters: ViewFilters = {};

  @state()
  private _name = "";

  @state()
  private _layout: ViewLayout = "board";

  @state()
  private _saving = false;

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("open") && this.open) {
      this._name = this.viewName;
      this._layout = this.viewLayout || this.defaultLayout;
    }
  }

  private _onClose() {
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  private _getFilters(): ViewFilters {
    // Use passed-in filters if they have content, otherwise fall back to existing
    const f = this.filters ?? {};
    if (Object.keys(f).length > 0) return f;
    return this.existingFilters ?? {};
  }

  private async _save() {
    const name = this._name.trim();
    if (!name) return;

    this._saving = true;

    if (this.viewId) {
      // Edit mode: only send changed fields
      const patch: {
        name?: string;
        layout?: ViewLayout;
        filters?: ViewFilters;
      } = {};
      if (name !== this.viewName) patch.name = name;
      if (this._layout !== this.viewLayout) patch.layout = this._layout;
      const filters = this._getFilters();
      if (Object.keys(filters).length > 0) {
        patch.filters = filters;
      }
      const result = await updateView(this.viewId, patch);
      if (result) {
        this.dispatchEvent(
          new CustomEvent("view-updated", {
            detail: result,
            bubbles: true,
            composed: true,
          }),
        );
        this._onClose();
      }
    } else {
      const filters = this._getFilters();
      const result = await createView(
        name,
        this._layout,
        filters,
        this.projectId || undefined,
      );
      if (result) {
        this.dispatchEvent(
          new CustomEvent("view-created", {
            detail: result,
            bubbles: true,
            composed: true,
          }),
        );
        this._onClose();
      }
    }

    this._saving = false;
  }

  private async _delete() {
    if (!this.viewId) return;
    const { deleteView } = await import("../store");
    const ok = await deleteView(this.viewId);
    if (ok) {
      this.dispatchEvent(
        new CustomEvent("view-deleted", {
          detail: this.viewId,
          bubbles: true,
          composed: true,
        }),
      );
      this._onClose();
    }
  }

  private _onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      this._save();
    }
  }

  render() {
    const isEdit = !!this.viewId;
    return html`
      <plume-dialog
        .open="${this.open}"
        heading="${isEdit ? msg("Edit View") : msg("Save View")}"
        @close="${this._onClose}"
        @keydown="${this._onKeydown}"
      >
        <div class="form">
          <div class="field">
            <label class="field-label" for="view-name">${msg("Name")}</label>
            <plume-input
              id="view-name"
              placeholder="${msg("e.g. High Priority")}"
              .value="${this._name}"
              @input="${(e: Event) => {
                this._name = (e.target as HTMLInputElement).value;
              }}"
            ></plume-input>
          </div>
          <div class="field">
            <label class="field-label">${msg("Layout")}</label>
            <plume-select
              .options="${getLayoutOptions()}"
              .value="${this._layout}"
              @change="${(e: CustomEvent) => {
                this._layout = e.detail as ViewLayout;
              }}"
            ></plume-select>
          </div>
        </div>

        <div slot="footer" class="footer-actions">
          ${isEdit
            ? html`
              <plume-button
                class="delete-btn"
                variant="destructive"
                size="sm"
                ?disabled="${this._saving}"
                @click="${this._delete}"
              >
                ${msg("Delete")}
              </plume-button>
            `
            : nothing}
          <plume-button
            variant="outline"
            size="sm"
            @click="${this._onClose}"
          >
            ${msg("Cancel")}
          </plume-button>
          <plume-button
            size="sm"
            ?disabled="${!this._name.trim() || this._saving}"
            @click="${this._save}"
          >
            ${isEdit ? msg("Save") : msg("Save view")}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-save-view-dialog": PlumeSaveViewDialog;
  }
}
