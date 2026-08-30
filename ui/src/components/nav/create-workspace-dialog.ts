import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import * as v from "valibot";
import { createWorkspace } from "@/features/workspaces/api";
import { fetchWorkspaces, switchActiveWorkspace } from "@/store/workspaces";
import { navigate } from "@/routes/router";
import "../ui/dialog.ts";
import "../ui/field.ts";
import "../ui/input.ts";
import "../ui/button.ts";
import "../ui/spinner.ts";

const WorkspaceNameSchema = v.object({
  name: v.pipe(
    v.string(),
    v.minLength(2, "Name must be at least 2 characters"),
    v.maxLength(64, "Name must be at most 64 characters"),
  ),
});

/**
 * Dialog for creating a new workspace. On success it refetches the workspace
 * list and switches to the newly created workspace, then navigates to the
 * dashboard. Mirrors the setup wizard's org-name step.
 */
@localized()
@customElement("breeze-create-workspace-dialog")
export class BreezeCreateWorkspaceDialog extends LitElement {
  static styles = css`
    :host {
      display: contents;
    }
    form {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }
    .form-error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
  `;

  /** Controls dialog visibility. */
  @property({ type: Boolean })
  open = false;

  @state()
  private _name = "";

  @state()
  private _error = "";

  @state()
  private _submitting = false;

  protected updated(changedProps: Map<string, unknown>): void {
    // Reset form state whenever the dialog is (re)opened.
    if (changedProps.has("open") && this.open) {
      this._name = "";
      this._error = "";
      this._submitting = false;
    }
  }

  private _onSubmit = async (e: Event) => {
    e.preventDefault();
    this._error = "";
    const r = v.safeParse(WorkspaceNameSchema, { name: this._name });
    if (!r.success) {
      this._error = r.issues[0]?.message ?? "Invalid name";
      return;
    }
    this._submitting = true;
    try {
      const created = await createWorkspace(r.output.name);
      // Refresh the list so the new workspace appears, then switch to it by ID
      // (avoids name-match ambiguity if two workspaces share a name).
      await fetchWorkspaces();
      if (created?.id) {
        await switchActiveWorkspace(created.id);
      }
      this.open = false;
      navigate("/");
    } catch (err) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to create workspace");
    } finally {
      this._submitting = false;
    }
  };

  protected render() {
    return html`
      <breeze-dialog
        .open="${this.open}"
        heading="${msg("Create workspace")}"
        @close="${() => (this.open = false)}"
      >
        <form @submit="${this._onSubmit}" novalidate>
          <breeze-field
            label="${msg("Workspace name")}"
            .error="${this._error}"
            ?invalid="${!!this._error}"
          >
            <breeze-input
              id="name"
              name="name"
              type="text"
              placeholder="${msg("Acme Corp")}"
              .value="${this._name}"
              @input="${(
                e: Event,
              ) => (this._name = (e.target as HTMLInputElement).value)}"
              ?invalid="${!!this._error}"
            ></breeze-input>
          </breeze-field>
          ${this._error
            ? html`<div class="form-error">${this._error}</div>`
            : nothing}
        </form>
        <div slot="footer">
          <breeze-button
            variant="outline"
            type="button"
            @click="${() => (this.open = false)}"
          >${msg("Cancel")}</breeze-button>
          <breeze-button
            type="button"
            ?disabled="${this._submitting}"
            @click="${this._onSubmit}"
          >
            ${this._submitting
              ? html`<breeze-spinner></breeze-spinner><span>${
                msg("Creating…")
              }</span>`
              : msg("Create workspace")}
          </breeze-button>
        </div>
      </breeze-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-create-workspace-dialog": BreezeCreateWorkspaceDialog;
  }
}
