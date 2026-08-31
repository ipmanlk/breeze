import { localized, msg } from "@lit/localize";
import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { sidebarIsMobile, toggleSidebar } from "@/store/sidebar";
import { SignalController } from "@/lib/signal-controller";
import "./ui/plume-icon.ts";
import "./theme-switcher.ts";

@localized()
@customElement("plume-top-bar")
export class PlumeTopBar extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      height: var(--topbar-h);
      padding: 0 var(--space-4);
      background: var(--background);
    }
    .trigger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--control-h-sm);
      height: var(--control-h-sm);
      border-radius: var(--radius-md);
      border: 1px solid transparent;
      background: transparent;
      color: var(--foreground);
      cursor: pointer;
      margin-left: calc(var(--space-1) * -1);
      transition: background var(--dur-fast) var(--ease-1);
      outline: none;
    }
    .trigger:hover {
      background: var(--accent);
    }
    .trigger:focus-visible {
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 40%, transparent);
    }
    .search {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: var(--search-w);
      height: var(--control-h-sm);
      padding: 0 var(--space-3);
      border-radius: var(--radius-md);
      border: 1px solid var(--border);
      background: var(--background);
      color: var(--muted-foreground);
      font-size: var(--text-sm);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .search:hover {
      background: var(--accent);
    }
    .kbd-group {
      margin-left: auto;
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
    }
    kbd {
      font-family: var(--font-mono);
      font-size: var(--text-xs);
      padding: 0 var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      background: var(--muted);
      color: var(--muted-foreground);
    }
    .right {
      margin-left: auto;
      display: flex;
      align-items: center;
      gap: var(--space-1);
    }
    .sr-only {
      position: absolute;
      width: var(--space-px);
      height: var(--space-px);
      padding: 0;
      margin: calc(var(--space-px) * -1);
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border-width: 0;
    }
    @media (max-width: 48rem) {
      .search {
        width: auto;
        flex: 1;
      }
      .search span:nth-of-type(2) {
        display: none;
      }
      .kbd-group {
        display: none;
      }
    }
  `;

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(sidebarIsMobile);
  }

  protected render() {
    return html`
      <button
        class="trigger"
        type="button"
        @click="${toggleSidebar}"
        aria-label="${msg("Toggle Sidebar")}"
        title="${msg("Toggle Sidebar")}"
      >
        <plume-icon name="panel-left" size="18"></plume-icon>
        <span class="sr-only">${msg("Toggle Sidebar")}</span>
      </button>
      <button
        class="search"
        type="button"
        aria-label="${msg("Search")}"
        @click="${() =>
          document.dispatchEvent(
            new CustomEvent("open-command-palette", {
              bubbles: true,
              composed: true,
            }),
          )}"
      >
        <plume-icon name="search" size="16"></plume-icon>
        <span>${msg("Search...")}</span>
        <div class="kbd-group">
          <kbd>⌘</kbd>
          <kbd>K</kbd>
        </div>
      </button>
      <div class="right">
        <plume-theme-switcher></plume-theme-switcher>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-top-bar": PlumeTopBar;
  }
}
