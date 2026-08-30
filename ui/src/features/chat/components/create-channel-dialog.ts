import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { conversationList, showCreateChannel } from "../store";
import { chatApi } from "../api";
import { projects } from "@/store/projects";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/breeze-icon.ts";
import "@/components/ui/combobox.ts";
import { localized, msg } from "@lit/localize";

/**
 * Create channel dialog.
 *
 * Creates a text or voice channel, optionally under a category.
 */
@localized()
@customElement("breeze-create-channel-dialog")
export class BreezeCreateChannelDialog extends LitElement {
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
    .kind-row {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: var(--space-2);
    }
    .kind-btn {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--space-1);
      padding: var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: transparent;
      color: inherit;
      font-family: inherit;
      font-size: var(--text-sm);
      text-align: left;
      cursor: pointer;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        background var(--dur-fast) var(--ease-1);
    }
    .kind-btn:hover {
      background: var(--accent);
    }
    .kind-btn.selected {
      border-color: var(--primary);
      background: color-mix(in oklch, var(--primary) 5%, transparent);
    }
    .kind-btn-name {
      font-weight: 500;
    }
    .kind-btn-desc {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
    .hint {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
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
  private _kind: "channel" | "voice" = "channel";

  @state()
  private _projectIds: string[] = [];

  @state()
  private _creating = false;

  @state()
  private _error = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(showCreateChannel);
  }

  #getCategoryId(): string | null {
    return showCreateChannel.value.categoryId;
  }

  #reset() {
    this._name = "";
    this._kind = "channel";
    this._projectIds = [];
    this._creating = false;
    this._error = "";
  }

  #onClose() {
    showCreateChannel.value = { open: false, categoryId: null };
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
      const categoryId = this.#getCategoryId();
      const conv = await chatApi.createConversation({
        type: this._kind,
        name: name.toLowerCase().replace(/\s+/g, "-"),
        parent_id: categoryId ?? undefined,
        project_ids: this._projectIds.length > 0 ? this._projectIds : undefined,
      });
      // Append to conversation list
      const current = conversationList.value;
      if (!current.find((c) => c.id === conv.id)) {
        conversationList.value = [...current, conv];
      }
      showCreateChannel.value = { open: false, categoryId: null };
      this.#reset();
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to create channel.");
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
      <div>
        <p class="hint">${msg("Link to projects (optional):")}</p>
        <breeze-combobox
          .options="${options}"
          .value="${this._projectIds}"
          placeholder=${msg("Select projects...")}
          @change="${(e: Event) => {
            this._projectIds = (e as CustomEvent).detail as string[];
          }}"
        ></breeze-combobox>
      </div>
    `;
  }

  protected render() {
    const isOpen = showCreateChannel.value.open;
    const categoryId = this.#getCategoryId();

    return html`
      <breeze-dialog
        style="--dialog-w:28rem"
        .open="${isOpen}"
        heading="${msg("Create channel")}"
        @close="${this.#onClose}"
      >
        <div class="body">
          <div class="kind-row">
            <button
              type="button"
              class="kind-btn ${this._kind === "channel" ? "selected" : ""}"
              @click="${() => {
                this._kind = "channel";
              }}"
            >
              <breeze-icon name="hash" size="16"></breeze-icon>
              <span class="kind-btn-name">${msg("Text")}</span>
              <span class="kind-btn-desc">${msg(
                "Send messages, images, files",
              )}</span>
            </button>
            <button
              type="button"
              class="kind-btn ${this._kind === "voice" ? "selected" : ""}"
              @click="${() => {
                this._kind = "voice";
              }}"
            >
              <breeze-icon name="volume-2" size="16"></breeze-icon>
              <span class="kind-btn-name">${msg("Voice")}</span>
              <span class="kind-btn-desc">${msg(
                "Voice chat with participants",
              )}</span>
            </button>
          </div>

          <breeze-input
            placeholder=${msg("new-channel")}
            .value="${this._name}"
            maxlength="100"
            autofocus
            @input="${(e: Event) => {
              this._name = (e.target as HTMLInputElement).value;
            }}"
          ></breeze-input>
          <p class="hint">
            Lowercase, hyphens for spaces. Example: <code>team-updates</code>
          </p>

          ${this.#renderProjectPicker()} ${categoryId
            ? html`
              <p class="hint">Will be created in the selected category.</p>
            `
            : html`
              <p class="hint">No category selected. Channel will be uncategorized.</p>
            `} ${this._error
            ? html`
              <p class="error">${this._error}</p>
            `
            : ""}
        </div>
        <div slot="footer" class="footer">
          <breeze-button
            variant="ghost"
            type="button"
            @click="${this.#onClose}"
          >
            Cancel
          </breeze-button>
          <breeze-button
            ?disabled="${this._creating || !this._name.trim()}"
            @click="${this.#onSubmit}"
          >
            ${this._creating ? "Creating..." : "Create"}
          </breeze-button>
        </div>
      </breeze-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-create-channel-dialog": BreezeCreateChannelDialog;
  }
}
