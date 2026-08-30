import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, query, state } from "lit/decorators.js";
import {
  applyPreset,
  currentPreset,
  palette,
  theme,
  THEME_PRESETS,
} from "@/store/theme";
import { SignalController } from "@/lib/signal-controller";
import { OutsideClickController } from "@/lib/outside-click-controller";
import "./ui/breeze-icon.ts";

interface PaletteGroup {
  id: string;
  label: string;
  presetIds: string[];
}

function getPaletteGroups(): PaletteGroup[] {
  return [
    {
      id: "breeze",
      label: "Breeze",
      presetIds: [
        "light",
        "paper",
        "dark",
        "noir",
      ],
    },
    { id: "github-dark", label: "GitHub", presetIds: ["github-dark"] },
    {
      id: "solarized",
      label: "Solarized",
      presetIds: ["solarized-light", "solarized-dark"],
    },
    { id: "dracula", label: "Dracula", presetIds: ["dracula"] },
    { id: "nord", label: "Nord", presetIds: ["nord"] },
    { id: "monokai", label: "Monokai", presetIds: ["monokai"] },
    {
      id: "catppuccin",
      label: "Catppuccin",
      presetIds: ["catppuccin-latte", "catppuccin-mocha"],
    },
    { id: "tokyo-night", label: "Tokyo Night", presetIds: ["tokyo-night"] },
    { id: "one-dark", label: "One Dark", presetIds: ["one-dark"] },
    {
      id: "gruvbox",
      label: "Gruvbox",
      presetIds: ["gruvbox", "gruvbox-light"],
    },
    {
      id: "rose-pine",
      label: "Rosé Pine",
      presetIds: ["rose-pine", "rose-pine-dawn"],
    },
  ];
}

@localized()
@customElement("breeze-theme-switcher")
export class BreezeThemeSwitcher extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      position: relative;
    }
    .trigger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--control-h);
      height: var(--control-h);
      border-radius: var(--radius-md);
      border: 1px solid transparent;
      background: transparent;
      color: var(--foreground);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
      position: relative;
    }
    .trigger:hover {
      background: var(--accent);
    }
    .trigger-dot {
      position: absolute;
      bottom: 6px;
      right: 6px;
      width: 6px;
      height: 6px;
      border-radius: var(--radius-full);
      box-shadow: 0 0 0 1.5px var(--background);
    }
    .panel {
      position: fixed;
      z-index: var(--z-dropdown);
      width: 200px;
      max-height: min(calc(100vh - 80px), 420px);
      overflow-y: auto;
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-lg);
      padding: var(--space-1);
      transform-origin: top center;
      animation: panel-in var(--dur-fast) var(--ease-1);
    }
    @keyframes panel-in {
      from {
        opacity: 0;
        transform: translateY(-4px) scale(0.97);
      }
      to {
        opacity: 1;
        transform: translateY(0) scale(1);
      }
    }
    .group-label {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-2) var(--space-1);
      font-size: var(--text-2xs);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted-foreground);
    }
    .group-label::after {
      content: "";
      flex: 1;
      height: var(--space-px);
      background: var(--border);
    }
    .group-label:first-child {
      padding-top: var(--space-1);
    }
    .preset {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
      text-align: left;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .preset:hover {
      background: var(--accent);
    }
    .preset[data-active] {
      background: var(--accent);
    }
    .preset-dot {
      width: 8px;
      height: 8px;
      border-radius: var(--radius-full);
      flex-shrink: 0;
      cursor: pointer;
      transition: transform var(--dur-fast) var(--ease-1);
    }
    .preset:hover .preset-dot {
      transform: scale(1.25);
    }
    .preset-label {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .preset-check {
      color: var(--primary);
      flex-shrink: 0;
    }
  `;

  @state()
  private _open = false;

  @state()
  private _previewId: string | null = null;

  @query(".trigger")
  private _trigger!: HTMLElement;

  @query(".panel")
  private _panel!: HTMLElement;

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(theme, palette, currentPreset);
  }

  private _outsideClick = new OutsideClickController(this, () => {
    this._clearPreview();
    this._open = false;
  });

  private _onKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      this._clearPreview();
      this._open = false;
    }
  };

  private _onScroll = () => {
    if (this._open) this._positionPanel();
  };

  private _onResize = () => {
    if (this._open) this._positionPanel();
  };

  private _positionPanel() {
    if (!this._panel || !this._trigger) return;
    const rect = this._trigger.getBoundingClientRect();
    this._panel.style.top = `${rect.bottom + 4}px`;
    this._panel.style.left = `${Math.max(8, rect.left + rect.width - 200)}px`;
  }

  private _applyAttrs(
    preset: { palette: string; mode: string; color: string },
  ): void {
    document.documentElement.dataset.palette = preset.palette;
    document.documentElement.dataset.theme = preset.mode;
    document.documentElement.dataset.color = preset.color;
  }

  private _restoreAttrs(): void {
    const current = THEME_PRESETS.find((p) => p.id === currentPreset.value);
    if (current) this._applyAttrs(current);
  }

  private _preview(id: string): void {
    const preset = THEME_PRESETS.find((p) => p.id === id);
    if (!preset) return;
    this._previewId = id;
    this._applyAttrs(preset);
  }

  private _clearPreview(): void {
    if (!this._previewId) return;
    this._previewId = null;
    this._restoreAttrs();
  }

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("_open")) {
      if (this._open) {
        this._clearPreview();
        this._outsideClick.connect();
        document.addEventListener("keydown", this._onKeydown);
        window.addEventListener("scroll", this._onScroll, true);
        window.addEventListener("resize", this._onResize);
        requestAnimationFrame(() => this._positionPanel());
      } else {
        this._clearPreview();
        this._outsideClick.disconnect();
        document.removeEventListener("keydown", this._onKeydown);
        window.removeEventListener("scroll", this._onScroll, true);
        window.removeEventListener("resize", this._onResize);
      }
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._clearPreview();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onKeydown);
    window.removeEventListener("scroll", this._onScroll, true);
    window.removeEventListener("resize", this._onResize);
  }

  private _toggle() {
    this._clearPreview();
    this._open = !this._open;
  }

  private _select(id: string) {
    applyPreset(id);
    this._previewId = null;
    this._open = false;
  }

  private _renderPreset(id: string) {
    const preset = THEME_PRESETS.find((p) => p.id === id);
    if (!preset) return nothing;

    const active = currentPreset.value === id;
    return html`
      <button
        class="preset"
        type="button"
        ?data-active="${active}"
        @click="${() => this._select(id)}"
      >
        <span
          class="preset-dot"
          style="background:${preset.colorHex}"
          @mouseenter="${() => this._preview(id)}"
          @mouseleave="${this._clearPreview}"
          title="${msg("Preview theme")}"
        ></span>
        <span class="preset-label">${preset.label}</span>
        ${active
          ? html`
            <breeze-icon class="preset-check" name="check" size="14"></breeze-icon>
          `
          : nothing}
      </button>
    `;
  }

  protected render() {
    const current = THEME_PRESETS.find((p) => p.id === currentPreset.value);
    const icon = theme.value === "light" ? "moon" : "sun";

    return html`
      <button
        class="trigger"
        type="button"
        @click="${this._toggle}"
        aria-label="${msg("Switch theme")}"
        title="${msg("Switch theme")}"
      >
        <breeze-icon name="${icon}" size="18"></breeze-icon>
        ${current
          ? html`
            <span class="trigger-dot" style="background:${current
              .colorHex}"></span>
          `
          : nothing}
      </button>
      ${this._open
        ? html`
          <div class="panel">
            ${getPaletteGroups().map((g) =>
              html`
                <div class="group-label">${g.label}</div>
                ${g.presetIds.map((id) => this._renderPreset(id))}
              `
            )}
          </div>
        `
        : ""}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-theme-switcher": BreezeThemeSwitcher;
  }
}
