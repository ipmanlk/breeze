import { css, html, LitElement } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import {
  deleteLabelsById,
  getLabels,
  patchLabelsById,
  postLabels,
} from "@/api";
import type { DtoLabelResponse } from "@/api";
import { showToast } from "@/components/ui/toast-store";
import { pageEnterStyles } from "@/styles/shared-animations";
import "../../components/ui/button.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/breeze-icon.ts";
import "../../layouts/app-layout.ts";
import { localized, msg } from "@lit/localize";

const PRESET_COLORS = [
  "#6366f1",
  "#ef4444",
  "#f97316",
  "#eab308",
  "#22c55e",
  "#06b6d4",
  "#ec4899",
  "#8b5cf6",
  "#14b8a6",
  "#78716c",
];

@localized()
@customElement("breeze-labels-settings-page")
export class BreezeLabelsSettingsPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      .page-head {
        display: flex;
        align-items: center;
        gap: var(--space-4);
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
      }
      .page-head h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--foreground);
      }
      .page-content {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-6);
      }
      .sections {
        max-width: var(--space-160);
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
      }

      /* Toolbar: search + create */
      .toolbar {
        display: flex;
        align-items: center;
        gap: var(--space-2);
      }
      .search-wrap {
        flex: 1;
        position: relative;
        display: flex;
        align-items: center;
      }
      .search-wrap breeze-icon {
        position: absolute;
        left: var(--space-2);
        color: var(--muted-foreground);
        pointer-events: none;
      }
      .search-input {
        width: 100%;
        height: var(--control-h);
        padding: 0 var(--space-2) 0 var(--space-7);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
      }
      .search-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }

      /* Create form */
      .add-form {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-2);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--card);
      }
      .add-form .name-input {
        flex: 1;
        min-width: 0;
        height: var(--control-h-sm, 1.75rem);
        padding: 0 var(--space-2);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
      }
      .add-form .name-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }
      .color-trigger {
        width: var(--space-7);
        height: var(--space-7);
        border-radius: var(--radius-full);
        border: 1px solid var(--border);
        cursor: pointer;
        flex-shrink: 0;
        padding: 0;
        transition: box-shadow var(--dur-fast) var(--ease-1);
      }
      .color-trigger:hover {
        box-shadow: 0 0 0 2px
          color-mix(in oklch, var(--foreground) 30%, transparent);
      }
      .palette {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-1-5);
        padding: var(--space-1);
        max-width: var(--space-44);
      }
      .swatch {
        width: var(--space-5);
        height: var(--space-5);
        border-radius: var(--radius-full);
        border: none;
        cursor: pointer;
        padding: 0;
        transition: box-shadow var(--dur-fast) var(--ease-1);
      }
      .swatch:hover {
        box-shadow: 0 0 0 1px
          color-mix(in oklch, var(--foreground) 30%, transparent);
      }
      .swatch.selected {
        box-shadow: 0 0 0 2px var(--foreground);
      }

      /* Label list */
      .label-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
      }
      .label-row {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-2) var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--card);
      }
      .label-row:hover {
        border-color: var(--ring);
      }
      .label-swatch {
        width: var(--space-4);
        height: var(--space-4);
        border-radius: var(--radius-full);
        flex-shrink: 0;
      }
      .label-name {
        flex: 1;
        font-size: var(--text-sm);
        font-weight: 500;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .editing-input {
        flex: 1;
        min-width: 0;
        height: var(--control-h-sm, 1.75rem);
        padding: 0 var(--space-2);
        border: 1px solid var(--input);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        outline: none;
      }
      .editing-input:focus {
        border-color: var(--ring);
        box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
      }
      .label-actions {
        display: flex;
        gap: var(--space-1);
        opacity: 0;
        transition: opacity var(--dur-fast) var(--ease-1);
      }
      .label-row:hover .label-actions,
      .label-row.editing .label-actions {
        opacity: 1;
      }
      .icon-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: var(--space-6);
        height: var(--space-6);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
      }
      .icon-btn:hover {
        background: var(--accent);
        color: var(--foreground);
      }
      .icon-btn.destructive:hover {
        color: var(--destructive);
      }
      .empty {
        padding: var(--space-8);
        text-align: center;
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      .muted-count {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .loading {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: var(--space-8);
        gap: var(--space-2);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
    `,
  ];

  /** When true, suppress <breeze-app-layout> and .page-head. */
  @property({ type: Boolean })
  embedded = false;

  @state()
  private _labels: DtoLabelResponse[] = [];
  @state()
  private _loading = true;
  @state()
  private _newName = "";
  @state()
  private _newColor = PRESET_COLORS[0];
  @state()
  private _creating = false;
  @state()
  private _search = "";
  @state()
  private _editingId: string | null = null;
  @state()
  private _editName = "";
  @state()
  private _editColor = "";
  @state()
  private _savingEdit: Record<string, boolean> = {};

  connectedCallback(): void {
    super.connectedCallback();
    this.#load();
  }

  async #load(): Promise<void> {
    this._loading = true;
    try {
      const { data } = await getLabels({ throwOnError: true });
      this._labels = data ?? [];
    } catch {
      showToast(msg("Failed to load labels"), { variant: "error" });
    } finally {
      this._loading = false;
    }
  }

  async #create(): Promise<void> {
    const name = this._newName.trim();
    if (!name) return;
    this._creating = true;
    try {
      const { data } = await postLabels({
        body: { name, color: this._newColor },
        throwOnError: true,
      });
      this._labels = [...this._labels, data];
      this._newName = "";
      this._newColor = PRESET_COLORS[0];
    } catch {
      showToast(msg("Failed to create label"), { variant: "error" });
    } finally {
      this._creating = false;
    }
  }

  #startEdit(label: DtoLabelResponse): void {
    this._editingId = label.id ?? null;
    this._editName = label.name ?? "";
    this._editColor = label.color ?? PRESET_COLORS[0];
    this.updateComplete.then(() => {
      this.shadowRoot?.querySelector<HTMLInputElement>(".editing-input")
        ?.focus();
    });
  }

  async #saveEdit(id: string): Promise<void> {
    const name = this._editName.trim();
    if (!name || !id) return;
    this._savingEdit = { ...this._savingEdit, [id]: true };
    try {
      const { data } = await patchLabelsById({
        path: { id },
        body: { name, color: this._editColor },
        throwOnError: true,
      });
      this._labels = this._labels.map((l) => l.id === id ? data : l);
      this._editingId = null;
    } catch {
      showToast(msg("Failed to update label"), { variant: "error" });
    } finally {
      this._savingEdit = { ...this._savingEdit, [id]: false };
    }
  }

  async #deleteLabel(id: string): Promise<void> {
    if (!id) return;
    try {
      await deleteLabelsById({ path: { id }, throwOnError: true });
      this._labels = this._labels.filter((l) => l.id !== id);
      if (this._editingId === id) this._editingId = null;
    } catch {
      showToast(msg("Failed to delete label"), { variant: "error" });
    }
  }

  private get _filteredLabels(): DtoLabelResponse[] {
    const q = this._search.trim().toLowerCase();
    if (!q) return this._labels;
    return this._labels.filter((l) => (l.name ?? "").toLowerCase().includes(q));
  }

  #renderColorPicker(
    value: string,
    onChange: (c: string) => void,
  ): unknown {
    return html`
      <breeze-popover>
        <button
          slot="trigger"
          class="color-trigger"
          type="button"
          style="background:${value}"
          aria-label=${msg("Pick color")}
        >
        </button>
        <div slot="content" class="palette">
          ${PRESET_COLORS.map(
            (c) =>
              html`
                <button
                  class="swatch ${value === c ? "selected" : ""}"
                  type="button"
                  title="${c}"
                  style="background:${c}"
                  @click="${() => onChange(c)}"
                >
                </button>
              `,
          )}
        </div>
      </breeze-popover>
    `;
  }

  protected render(): unknown {
    if (this._loading) {
      const spinner =
        html`<div class="loading"><breeze-spinner></breeze-spinner> ${
          msg("Loading labels…")
        }</div>`;
      if (this.embedded) return spinner;
      return html`<breeze-app-layout>${spinner}</breeze-app-layout>`;
    }

    const body = html`
      <div class="sections">
        <div class="add-form">
          ${this.#renderColorPicker(
            this._newColor,
            (c) => (this._newColor = c),
          )}
          <input
            class="name-input"
            placeholder=${msg("New label name…")}
            .value=${this._newName}
            @input=${(
              e: Event,
            ) => (this._newName = (e.target as HTMLInputElement).value)}
            @keydown=${(e: KeyboardEvent) => {
              if (e.key === "Enter") this.#create();
            }}
          />
          <breeze-button
            size="sm"
            ?disabled=${this._creating || !this._newName.trim()}
            @click=${() => this.#create()}
          >
            ${this._creating
              ? html`<breeze-spinner></breeze-spinner>`
              : html`<breeze-icon name="plus" size="14"></breeze-icon>`}
          </breeze-button>
        </div>

        <div class="toolbar">
          <div class="search-wrap">
            <breeze-icon name="search" size="14"></breeze-icon>
            <input
              class="search-input"
              type="text"
              placeholder=${msg("Search labels…")}
              .value=${this._search}
              @input=${(
                e: Event,
              ) => (this._search = (e.target as HTMLInputElement).value)}
            />
          </div>
          <span class="muted-count">
            ${this._filteredLabels.length}/${this._labels.length}
          </span>
        </div>

        ${this._filteredLabels.length === 0
          ? html`<div class="empty">
              ${
            this._labels.length === 0
              ? msg("No labels yet. Create one above.")
              : msg("No labels match your search.")
          }
          }
            </div>`
          : html`<div class="label-list">
              ${
            this._filteredLabels.map(
              (l) =>
                html`
                  <div class="label-row${this._editingId === l.id
                    ? " editing"
                    : ""}">
                    ${this._editingId === l.id
                      ? this.#renderColorPicker(
                        this._editColor,
                        (c) => (this._editColor = c),
                      )
                      : html`
                        <span
                          class="label-swatch"
                          style="background:${l.color ?? PRESET_COLORS[0]}"
                        ></span>
                      `}
                    ${this._editingId === l.id
                      ? html`
                        <input
                          class="editing-input"
                          .value=${this._editName}
                          @input=${(
                            e: Event,
                          ) => (this._editName =
                            (e.target as HTMLInputElement).value)}
                          @keydown=${(e: KeyboardEvent) => {
                            if (e.key === "Enter") this.#saveEdit(l.id!);
                            if (e.key === "Escape") this._editingId = null;
                          }}
                        />
                        <div class="label-actions">
                          <breeze-button
                            variant="ghost"
                            size="sm"
                            ?disabled=${this._savingEdit[l.id!]}
                            @click=${() => this.#saveEdit(l.id!)}
                          >${this._savingEdit[l.id!]
                            ? html`<breeze-spinner></breeze-spinner>`
                            : msg("Save")}</breeze-button>
                          <breeze-button
                            variant="ghost"
                            size="sm"
                            @click=${() => (this._editingId = null)}
                          >${msg("Cancel")}</breeze-button>
                        </div>
                      `
                      : html`
                        <span class="label-name">${l.name}</span>
                        <div class="label-actions">
                          <button
                            class="icon-btn"
                            type="button"
                            aria-label=${msg("Edit label")}
                            @click=${() => this.#startEdit(l)}
                          >
                            <breeze-icon name="pencil" size="14"></breeze-icon>
                          </button>
                          <button
                            class="icon-btn destructive"
                            type="button"
                            aria-label=${msg("Delete label")}
                            @click=${() => this.#deleteLabel(l.id!)}
                          >
                            <breeze-icon name="trash-2" size="14"></breeze-icon>
                          </button>
                        </div>
                      `}
                  </div>
                `,
            )
          }
            </div>`}
      </div>
    `;

    if (this.embedded) {
      return html`${body}`;
    }
    return html`
      <breeze-app-layout>
        <div class="page-enter">
          <div class="page-head">
            <h1>${msg("Labels")}</h1>
          </div>
          <div class="page-content">${body}</div>
        </div>
      </breeze-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-labels-settings-page": BreezeLabelsSettingsPage;
  }
}
