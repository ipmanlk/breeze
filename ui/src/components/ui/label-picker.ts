import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { getLabels } from "@/api";
import type { DtoLabelResponse } from "@/api";
import { showToast } from "@/components/ui/toast-store";
import "./label-chip.ts";
import "./breeze-icon.ts";
import "./popover.ts";

/**
 * Label picker: renders the labels currently attached to a task as chips,
 * with an "+ Add" trigger that opens a popover listing every org label to
 * toggle membership.
 *
 * Emits `change` with the new full set of selected label IDs whenever the
 * user toggles a label. The parent is responsible for persisting via
 * `putProjectsByIdTasksByTaskIdLabels`.
 */
@localized()
@customElement("breeze-label-picker")
export class BreezeLabelPicker extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }
    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: var(--space-1);
      align-items: center;
    }
    .add-trigger {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      height: 1.25rem;
      padding: 0 var(--space-1-5);
      border-radius: var(--radius-full);
      border: 1px dashed var(--border);
      background: transparent;
      color: var(--muted-foreground);
      font-size: var(--text-2xs, 0.6875rem);
      cursor: pointer;
      transition: color var(--dur-fast) var(--ease-1),
        border-color var(--dur-fast) var(--ease-1);
    }
    .add-trigger:hover {
      color: var(--foreground);
      border-color: var(--ring);
    }
    .popover-content {
      min-width: 11rem;
      max-height: 14rem;
      overflow-y: auto;
      padding: var(--space-1);
    }
    .popover-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border-radius: var(--radius-sm);
      border: none;
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      text-align: left;
      cursor: pointer;
    }
    .popover-item:hover {
      background: var(--accent);
    }
    .popover-item .name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .popover-item .check {
      width: var(--space-3);
      height: var(--space-3);
      color: var(--primary);
    }
    .popover-item .swatch {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .popover-empty {
      padding: var(--space-3) var(--space-2);
      color: var(--muted-foreground);
      font-size: var(--text-xs);
      text-align: center;
    }
  `;

  /** Label IDs currently selected on the task. */
  @property({ attribute: false })
  selected: DtoLabelResponse[] = [];

  @state()
  private _allLabels: DtoLabelResponse[] = [];

  @state()
  private _loading = false;

  private _loaded = false;

  connectedCallback(): void {
    super.connectedCallback();
    this._loadLabels();
  }

  private async _loadLabels(): Promise<void> {
    if (this._loaded || this._loading) return;
    this._loading = true;
    try {
      const { data } = await getLabels({ throwOnError: true });
      this._allLabels = data ?? [];
      this._loaded = true;
    } catch {
      showToast(msg("Failed to load labels"), { variant: "error" });
    } finally {
      this._loading = false;
    }
  }

  private _toggle(label: DtoLabelResponse): void {
    const isSelected = this.selected.some((l) => l.id === label.id);
    let next: DtoLabelResponse[];
    if (isSelected) {
      next = this.selected.filter((l) => l.id !== label.id);
    } else {
      next = [...this.selected, label];
    }
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: { labels: next, labelIds: next.map((l) => l.id ?? "") },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _remove(id: string): void {
    const next = this.selected.filter((l) => l.id !== id);
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: { labels: next, labelIds: next.map((l) => l.id ?? "") },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    return html`
      <div class="chips">
        ${this.selected.map(
          (l) =>
            html`
              <breeze-label-chip
                .label="${l}"
                removable
                @remove="${(e: CustomEvent) => this._remove(e.detail.id)}"
              ></breeze-label-chip>
            `,
        )}
        <breeze-popover
          close-on-select="false"
          @click="${() => {
            if (!this._loaded) this._loadLabels();
          }}"
        >
          <button slot="trigger" class="add-trigger" type="button">
            <breeze-icon name="plus" size="11"></breeze-icon>
            ${msg("Add label")}
          </button>
          <div slot="content" class="popover-content">
            ${this._loading
              ? html`<div class="popover-empty">${msg("Loading…")}</div>`
              : this._allLabels.length === 0
              ? html`<div class="popover-empty">
                  ${msg("No labels yet. Create some in Settings → Labels.")}
                </div>`
              : this._allLabels.map(
                (l) => {
                  const checked = this.selected.some(
                    (s) => s.id === l.id,
                  );
                  return html`
                    <button
                      class="popover-item"
                      type="button"
                      @click="${() => this._toggle(l)}"
                    >
                      <span
                        class="swatch"
                        style="background: ${l.color ?? "#6366f1"}"
                      ></span>
                      <span class="name">${l.name}</span>
                      ${checked
                        ? html`
                          <breeze-icon
                            class="check"
                            name="check"
                            size="14"
                          ></breeze-icon>
                        `
                        : nothing}
                    </button>
                  `;
                },
              )}
          </div>
        </breeze-popover>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-label-picker": BreezeLabelPicker;
  }
}
