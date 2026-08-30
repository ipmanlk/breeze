import { css, html, LitElement, nothing } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import type { DtoLabelResponse } from "@/api";
import "./breeze-icon.ts";

/**
 * Label chip: a small colored pill with the label's name. Used on kanban
 * cards, list rows, the task detail sidebar, and the filter bar.
 *
 * Set `removable` to render an "x" that dispatches a `remove` event.
 */
@localized()
@customElement("breeze-label-chip")
export class BreezeLabelChip extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
    }
    .chip {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      max-width: 100%;
      height: 1.25rem;
      padding: 0 var(--space-1-5);
      border-radius: var(--radius-full);
      border: 1px solid color-mix(in oklch, var(--chip-color) 45%, transparent);
      background: color-mix(in oklch, var(--chip-color) 16%, transparent);
      color: var(--chip-color);
      font-size: var(--text-2xs, 0.6875rem);
      font-weight: 500;
      line-height: 1;
      white-space: nowrap;
      overflow: hidden;
    }
    .chip .name {
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .chip .dot {
      width: var(--space-1);
      height: var(--space-1);
      border-radius: var(--radius-full);
      background: var(--chip-color);
      flex-shrink: 0;
    }
    .chip .remove {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-3);
      height: var(--space-3);
      border: none;
      background: transparent;
      color: inherit;
      cursor: pointer;
      padding: 0;
      margin-right: calc(var(--space-1) * -1);
      opacity: 0.7;
    }
    .chip .remove:hover {
      opacity: 1;
    }
  `;

  /** The label to render. */
  @property({ attribute: false })
  label?: DtoLabelResponse;

  /** Render an explicit color (falls back to label.color, then a default). */
  @property()
  color?: string;

  /** Show an "x" button that dispatches `remove` with the label id. */
  @property({ type: Boolean })
  removable = false;

  /** Compact mode: only the dot, no name (for dense card layouts). */
  @property({ type: Boolean })
  compact = false;

  protected render() {
    const l = this.label;
    const color = this.color ?? l?.color ?? "#6366f1";
    const name = l?.name ?? "";
    return html`
      <span
        class="chip"
        style="--chip-color: ${color}"
        title="${name}"
      >
        <span class="dot"></span>
        ${this.compact ? nothing : html`<span class="name">${name}</span>`}
        ${this.removable && l?.id
          ? html`
            <button
              class="remove"
              type="button"
              aria-label="${msg("Remove label")} ${name}"
              @click="${(e: Event) => {
                e.stopPropagation();
                this.dispatchEvent(
                  new CustomEvent("remove", {
                    detail: { id: l.id },
                    bubbles: true,
                    composed: true,
                  }),
                );
              }}"
            >
              <breeze-icon name="x" size="11"></breeze-icon>
            </button>
          `
          : nothing}
      </span>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-label-chip": BreezeLabelChip;
  }
}
