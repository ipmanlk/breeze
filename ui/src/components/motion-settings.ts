import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { type MotionSettings, motionSettings } from "@/store/motion";
import "../components/ui/switch.ts";
import "../components/ui/button.ts";
import "../components/ui/breeze-icon.ts";

interface MotionGroup {
  key: keyof MotionSettings;
  label: string;
  description: string;
}

function getMotionGroups(): MotionGroup[] {
  return [
    {
      key: "page",
      label: msg("Page transitions"),
      description: msg("Route changes, tab crossfades"),
    },
    {
      key: "feedback",
      label: msg("UI feedback"),
      description: msg("Hover, focus, active states"),
    },
    {
      key: "layout",
      label: msg("Layout"),
      description: msg("Sidebar collapse, panel slide"),
    },
    {
      key: "overlay",
      label: msg("Dialogs & overlays"),
      description: msg("Dialog enter/exit, menus"),
    },
    {
      key: "list",
      label: msg("Lists & content"),
      description: msg("List items, content area changes"),
    },
    {
      key: "loading",
      label: msg("Loading states"),
      description: msg("Skeleton shimmer, spinners"),
    },
    {
      key: "dnd",
      label: msg("Drag & drop"),
      description: msg("Kanban card lifts, reorder"),
    },
    {
      key: "notify",
      label: msg("Notifications"),
      description: msg("Toast, badge pulse"),
    },
    {
      key: "chat",
      label: msg("Chat messages"),
      description: msg("New message slide, typing"),
    },
    {
      key: "voice",
      label: msg("Voice indicators"),
      description: msg("Speaking glow, mute toggle"),
    },
  ];
}

/**
 * Motion settings panel: reusable component that shows a master toggle,
 * speed slider, and per-group toggles. Read/writes the motionSettings store.
 *
 * Usage:
 *   <breeze-motion-settings></breeze-motion-settings>
 */
@localized()
@customElement("breeze-motion-settings")
export class BreezeMotionSettings extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }

    .section {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }

    .section-header {
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
    }
    .section-header h2 {
      margin: 0;
      font-size: var(--text-base);
      font-weight: 600;
      color: var(--foreground);
    }
    .section-header p {
      margin: 0;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }

    .field-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-4);
      padding: var(--space-3) 0;
      border-bottom: 1px solid var(--border);
    }
    .field-row:last-child {
      border-bottom: none;
    }

    .field-label {
      display: flex;
      flex-direction: column;
      gap: var(--space-0-5);
    }
    .field-label .label {
      font-size: var(--text-sm);
      font-weight: 500;
      color: var(--foreground);
    }
    .field-label .description {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }

    .field-control {
      flex-shrink: 0;
    }

    .speed-row {
      display: flex;
      align-items: center;
      gap: var(--space-3);
    }
    .speed-label {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
      min-width: 2.5rem;
      text-align: right;
    }
    input[type="range"] {
      -webkit-appearance: none;
      appearance: none;
      width: 8rem;
      height: 4px;
      border-radius: 2px;
      background: var(--border);
      outline: none;
    }
    input[type="range"]::-webkit-slider-thumb {
      -webkit-appearance: none;
      appearance: none;
      width: 14px;
      height: 14px;
      border-radius: 50%;
      background: var(--primary);
      cursor: pointer;
      border: 2px solid var(--background);
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
    }
    input[type="range"]::-moz-range-thumb {
      width: 14px;
      height: 14px;
      border-radius: 50%;
      background: var(--primary);
      cursor: pointer;
      border: 2px solid var(--background);
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
    }

    .advanced-toggle {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) 0;
      cursor: pointer;
    }
    .advanced-toggle:hover {
      color: var(--foreground);
    }
    .advanced-toggle .chevron {
      transition: transform var(--dur-fast) var(--ease-1);
    }
    .advanced-toggle .chevron.open {
      transform: rotate(90deg);
    }

    .advanced-groups {
      overflow: hidden;
      max-height: 0;
      opacity: 0;
      transition:
        max-height var(--dur-slow) var(--ease-2),
        opacity var(--dur-fast) var(--ease-1);
    }
    .advanced-groups.open {
      max-height: 600px;
      opacity: 1;
    }

    .reset-btn {
      margin-top: var(--space-2);
    }

    .reduced-banner {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-3);
      border-radius: var(--radius-md);
      background: color-mix(in oklch, var(--warning) 15%, transparent);
      border: 1px solid color-mix(in oklch, var(--warning) 30%, transparent);
      font-size: var(--text-xs);
      color: var(--warning-foreground);
      margin-bottom: var(--space-3);
    }
    .reduced-banner breeze-icon {
      flex-shrink: 0;
    }
  `;

  @state()
  private _settings: MotionSettings = { ...motionSettings.value };

  @state()
  private _showAdvanced = false;

  private _reducedMotion = false;

  connectedCallback(): void {
    super.connectedCallback();
    this._reducedMotion = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;
  }

  private _setMaster(e: CustomEvent) {
    const checked = (e.detail as { checked: boolean }).checked;
    this._settings = { ...this._settings, global: checked };
    motionSettings.set(this._settings);
    this.requestUpdate();
  }

  private _setGroup(key: keyof MotionSettings, e: CustomEvent) {
    const checked = (e.detail as { checked: boolean }).checked;
    this._settings = { ...this._settings, [key]: checked };
    motionSettings.set(this._settings);
  }

  private _setScale(e: Event) {
    const value = parseFloat((e.target as HTMLInputElement).value);
    this._settings = { ...this._settings, scale: value };
    motionSettings.set(this._settings);
  }

  private _reset() {
    this._settings = {
      global: true,
      page: true,
      feedback: true,
      layout: true,
      overlay: true,
      list: true,
      loading: true,
      dnd: true,
      notify: true,
      chat: true,
      voice: true,
      scale: 1,
    };
    motionSettings.set(this._settings);
  }

  protected render() {
    const s = this._settings;

    return html`
      <div class="section">
        <div class="section-header">
          <h2>${msg("Animation & Motion")}</h2>
          <p>${msg("Control how Breeze animates and transitions")}</p>
        </div>

        ${this._reducedMotion
          ? html`
            <div class="reduced-banner">
              <breeze-icon name="alert-circle" size="14"></breeze-icon>
              <span>
                ${msg(
                  "Your system has reduced motion enabled. Breeze animations are disabled. You can override below.",
                )}
              </span>
            </div>
          `
          : nothing}

        <!-- Master toggle -->
        <div class="field-row">
          <div class="field-label">
            <span class="label">${msg("Enable animations")}</span>
            <span class="description">
              ${msg("Master toggle — disables all animation when off")}
            </span>
          </div>
          <div class="field-control">
            <breeze-switch
              .checked="${s.global}"
              @change="${this._setMaster}"
            ></breeze-switch>
          </div>
        </div>

        <!-- Speed slider -->
        <div class="field-row">
          <div class="field-label">
            <span class="label">${msg("Speed")}</span>
            <span class="description">
              ${msg("Adjust animation speed")} (${s.scale.toFixed(2)}×)
            </span>
          </div>
          <div class="field-control">
            <div class="speed-row">
              <span class="speed-label">0.25×</span>
              <input
                type="range"
                min="0.25"
                max="3"
                step="0.25"
                .value="${String(s.scale)}"
                @input="${this._setScale}"
              />
              <span class="speed-label">3×</span>
            </div>
          </div>
        </div>

        <!-- Advanced toggle -->
        <div
          class="advanced-toggle"
          @click="${() => (this._showAdvanced = !this._showAdvanced)}"
          role="button"
          tabindex="0"
          @keydown="${(e: KeyboardEvent) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              this._showAdvanced = !this._showAdvanced;
            }
          }}"
        >
          <span class="chevron${this._showAdvanced ? " open" : ""}">
            <breeze-icon name="chevron-right" size="14"></breeze-icon>
          </span>
          <span
            style="font-size:var(--text-sm);font-weight:500;color:var(--muted-foreground)"
          >
            ${msg("Advanced")}
          </span>
        </div>

        <div class="advanced-groups${this._showAdvanced ? " open" : ""}">
          ${getMotionGroups().map(
            (g) =>
              html`
                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${g.label}</span>
                    <span class="description">${g.description}</span>
                  </div>
                  <div class="field-control">
                    <breeze-switch
                      .checked="${s[g.key] as boolean}"
                      ?disabled="${!s.global}"
                      @change="${(e: CustomEvent) => this._setGroup(g.key, e)}"
                    ></breeze-switch>
                  </div>
                </div>
              `,
          )}
        </div>

        <div class="reset-btn">
          <breeze-button variant="outline" size="sm" @click="${this._reset}">
          ${msg("Reset to defaults")}
          </breeze-button>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-motion-settings": BreezeMotionSettings;
  }
}
