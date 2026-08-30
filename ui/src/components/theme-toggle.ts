import { localized, msg } from "@lit/localize";
import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { theme, toggleTheme } from "@/store/theme";
import { SignalController } from "@/lib/signal-controller";
import "./ui/breeze-icon.ts";

@localized()
@customElement("breeze-theme-toggle")
export class BreezeThemeToggle extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
    }
    button {
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
    }
    button:hover {
      background: var(--accent);
    }
  `;

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(theme);
  }

  protected render() {
    const icon = theme.value === "light" ? "moon" : "sun";
    return html`
      <button type="button" @click="${toggleTheme}" aria-label="${msg(
        "Toggle theme",
      )}">
        <breeze-icon name="${icon}" size="18"></breeze-icon>
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-theme-toggle": BreezeThemeToggle;
  }
}
