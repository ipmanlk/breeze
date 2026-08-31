import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { conversationList, showCreateCategory } from "../store";
import { chatApi } from "../api";
import { projects } from "@/store/projects";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/combobox.ts";
import { localized, msg } from "@lit/localize";

/**
 * Create category dialog.
 *
 * Simple form: name input → creates a category-type conversation.
 * On success, prepends the new category to the conversation list and closes.
 */
@localized()
@customElement("plume-create-category-dialog")
export class PlumeCreateCategoryDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }
    .body {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
    .footer {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
      width: 100%;
    }
  `;

  #signals = new SignalController(this);

  @state()
  private _name = "";

  @state()
  private _creating = false;

  @state()
  private _error = "";

  @state()
  private _projectIds: string[] = [];

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(showCreateCategory);
  }

  #reset() {
    this._name = "";
    this._creating = false;
    this._error = "";
    this._projectIds = [];
  }

  #onClose() {
    showCreateCategory.value = false;
    this.#reset();
  }

  async #onSubmit(e: Event) {
    e.preventDefault();
    const name = this._name.trim();
    if (!name) {
      this._error = "Name is required.";
      return;
    }

    this._creating = true;
    this._error = "";

    try {
      const conv = await chatApi.createConversation({
        type: "category",
        name,
        project_ids: this._projectIds.length > 0 ? this._projectIds : undefined,
      });
      // Prepend to conversation list
      const current = conversationList.value;
      if (!current.find((c) => c.id === conv.id)) {
        conversationList.value = [conv, ...current];
      }
      showCreateCategory.value = false;
      this.#reset();
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to create category.");
    } finally {
      this._creating = false;
    }
  }

  #renderProjectPicker() {
    const allProjects = projects.value;
    const options = allProjects.projects.map((p) => ({
      value: p.id || "",
      label: p.name || "",
    })).filter((o) => o.value);

    return html`
      <div style="margin-top:var(--space-2)">
        <p class="hint" style="margin-bottom:var(--space-1)">
          Link to projects (optional):
        </p>
        <plume-combobox
          .options="${options}"
          .value="${this._projectIds}"
          placeholder=${msg("Select projects...")}
          @change="${(e: Event) => {
            this._projectIds = (e as CustomEvent).detail as string[];
          }}"
        ></plume-combobox>
      </div>
    `;
  }

  protected render() {
    const isOpen = showCreateCategory.value;

    return html`
      <plume-dialog
        style="--dialog-w:28rem"
        .open="${isOpen}"
        heading="Create category"
        @close="${this.#onClose}"
      >
        <div class="body">
          <plume-input
            placeholder=${msg("e.g. Engineering")}
            .value="${this._name}"
            autofocus
            @input="${(e: Event) => {
              this._name = (e.target as HTMLInputElement).value;
            }}"
          ></plume-input>
          ${this.#renderProjectPicker()} ${this._error
            ? html`
              <p class="error">${this._error}</p>
            `
            : ""}
        </div>
        <div slot="footer" class="footer">
          <plume-button
            variant="ghost"
            type="button"
            @click="${this.#onClose}"
          >
            Cancel
          </plume-button>
          <plume-button
            variant=""
            ?disabled="${this._creating || !this._name.trim()}"
            @click="${this.#onSubmit}"
          >
            ${this._creating ? "Creating..." : "Create"}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-create-category-dialog": PlumeCreateCategoryDialog;
  }
}
