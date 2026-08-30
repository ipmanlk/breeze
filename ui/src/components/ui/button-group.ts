import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import "./breeze-icon.ts";
import "./popover.ts";

export interface ButtonGroupAction {
  label: string;
  /** Optional icon name from the lucide icon set. */
  icon?: string;
  /** Set to true for destructive actions (red hover). */
  destructive?: boolean;
  /** Event detail sent when the action is clicked. */
  value: string;
}

/**
 * Split button with a main action and a caret-triggered dropdown menu.
 *
 * Slots:
 *   - (default): content of the main button (icon + text)
 *
 * Events:
 *   - `button-group-main`: dispatched when the main button is clicked
 *   - `button-group-action`: dispatched when a menu item is clicked; detail = { value: string }
 *
 * Properties: `actions` (menu items), `size` (sm is the small variant).
 */
@localized()
@customElement("breeze-button-group")
export class BreezeButtonGroup extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      align-items: stretch;
    }
    .main-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-2);
      height: var(--control-h);
      padding: 0 var(--space-3);
      border: 1px solid var(--border);
      border-right: none;
      border-top-left-radius: var(--radius-md);
      border-bottom-left-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-weight: 500;
      font-family: inherit;
      line-height: 1;
      cursor: pointer;
      white-space: nowrap;
      transition:
        background var(--dur-fast) var(--ease-1),
        opacity var(--dur-fast) var(--ease-1);
    }
    .main-btn:hover {
      background: var(--accent);
    }
    :host([size="sm"]) .main-btn {
      height: var(--control-h-sm);
      font-size: var(--text-xs);
    }
    .caret {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-6);
      height: var(--control-h);
      border: 1px solid var(--border);
      border-top-right-radius: var(--radius-md);
      border-bottom-right-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      cursor: pointer;
      font-family: inherit;
      flex-shrink: 0;
      transition:
        background var(--dur-fast) var(--ease-1),
        opacity var(--dur-fast) var(--ease-1);
    }
    .caret:hover {
      background: var(--accent);
    }
    :host([size="sm"]) .caret {
      height: var(--control-h-sm);
    }
    .menu {
      min-width: 10rem;
      padding: var(--space-1);
    }
    .menu-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      text-align: left;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .menu-item:hover {
      background: var(--accent);
    }
    .menu-item.destructive:hover {
      background: color-mix(in oklch, var(--destructive) 15%, transparent);
      color: var(--destructive);
    }
  `;

  @property({ type: Array, attribute: false })
  actions: ButtonGroupAction[] = [];

  /** Renders buttons at a compact size (matches breeze-button size="sm"). */
  @property({ reflect: true })
  size: "default" | "sm" = "default";

  private _onMainClick() {
    this.dispatchEvent(
      new CustomEvent("button-group-main", {
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onActionClick(action: ButtonGroupAction) {
    this.dispatchEvent(
      new CustomEvent("button-group-action", {
        detail: { value: action.value },
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    return html`
      <button class="main-btn" @click="${this._onMainClick}">
        <slot></slot>
      </button>
      <breeze-popover placement="bottom-end">
        <button
          slot="trigger"
          class="caret"
          aria-label="${msg("More actions")}"
        >
          <breeze-icon name="chevron-down" size="12"></breeze-icon>
        </button>
        <div slot="content" class="menu">
          ${this.actions.map(
            (a) =>
              html`
                <button
                  class="menu-item ${a.destructive ? "destructive" : ""}"
                  @click="${() => this._onActionClick(a)}"
                >
                  ${a.icon
                    ? html`
                      <breeze-icon name="${a.icon}" size="14"></breeze-icon>
                    `
                    : null} ${a.label}
                </button>
              `,
          )}
        </div>
      </breeze-popover>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-button-group": BreezeButtonGroup;
  }
}
