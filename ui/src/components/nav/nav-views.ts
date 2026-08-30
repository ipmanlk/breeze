import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { currentPath, navigate } from "@/routes/router";
import { unpinView, views } from "@/features/views/store";
import { sidebarIsMobile, sidebarOpen } from "@/store/sidebar";
import { SignalController } from "@/lib/signal-controller";
import "../ui/breeze-icon.ts";
import "../ui/tooltip.ts";

/**
 * Sidebar "Views" group: pinned views with a layout icon (board/list) and a
 * hover unpin (×) action. Hidden when there are no pinned views.
 *
 * Spacing matches shadcn: group `p-2`, items flush (`gap-0`), each row is
 * `size="sm"` (`h-7 text-xs p-2 gap-2 rounded-md`). Pinned views are fetched
 * once at the app level (`app-shell.ts`).
 */
@localized()
@customElement("breeze-nav-views")
export class BreezeNavViews extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }
    :host([hidden]) {
      display: none;
    }
    .group {
      display: flex;
      flex-direction: column;
      gap: 0;
      padding: var(--space-2);
    }
    .group-label {
      display: flex;
      align-items: center;
      height: var(--space-8);
      padding: 0 var(--space-2);
      font-size: var(--text-xs);
      font-weight: 500;
      color: color-mix(in oklch, var(--sidebar-foreground) 70%, transparent);
      white-space: nowrap;
      overflow: hidden;
    }
    :host([data-collapsed="true"]) .group-label {
      display: none;
    }
    .menu {
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .item {
      position: relative;
      display: flex;
      width: 100%;
    }
    .row {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      height: var(--space-7);
      padding: 0 var(--space-2);
      border-radius: var(--radius-md);
      font-size: var(--text-xs);
      color: var(--sidebar-foreground);
      text-decoration: none;
      cursor: pointer;
      outline: none;
      transition:
        background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .item:hover .row,
    .row:hover,
    .row.active {
      background: var(--sidebar-accent);
      color: var(--sidebar-accent-foreground);
    }
    .row.active {
      font-weight: 500;
    }
    .row:focus-visible {
      box-shadow: 0 0 0 2px
        color-mix(in oklch, var(--sidebar-ring) 40%, transparent);
    }
    :host([data-collapsed="true"]) .row {
      justify-content: center;
    }

    .name {
      flex: 1;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    :host([data-collapsed="true"]) .name {
      display: none;
    }

    .unpin {
      display: none;
      align-items: center;
      justify-content: center;
      width: var(--space-5);
      height: var(--space-5);
      position: absolute;
      right: var(--space-1);
      top: 50%;
      transform: translateY(-50%);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
    }
    .item:hover .unpin {
      display: inline-flex;
    }
    .unpin:hover {
      background: var(--sidebar);
      color: var(--sidebar-foreground);
    }
    :host([data-collapsed="true"]) .unpin {
      display: none !important;
    }
  `;

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(currentPath, views, sidebarOpen, sidebarIsMobile);
  }

  protected willUpdate(): void {
    this.dataset.collapsed = String(
      !sidebarOpen.value && !sidebarIsMobile.value,
    );
    this.hidden = views.value.pinnedViews.length === 0;
  }

  protected render(): unknown {
    const path = currentPath.value;
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;
    const currentViewId =
      new URLSearchParams(window.location.search).get("view") ?? "";

    return html`
      <div class="group">
        <div class="group-label">${msg("Views")}</div>
        <div class="menu">
          ${views.value.pinnedViews.map((v) => {
            const isProjectView = !!(v.project_id && v.project_slug);
            const href = isProjectView
              ? `/projects/${v.project_slug}?view=${v.id}`
              : `/views/${v.id}`;
            const active = isProjectView
              ? path === `/projects/${v.project_slug}` &&
                currentViewId === v.id
              : path === href;
            const icon = v.layout === "board" ? "layout-grid" : "list";

            const row = html`
              <div class="item">
                <a
                  class="row ${active ? "active" : ""}"
                  href="${href}"
                  @click="${(e: Event) => {
                    e.preventDefault();
                    navigate(href);
                  }}"
                >
                  <breeze-icon name="${icon}" size="16"></breeze-icon>
                  <span class="name">${v.name}</span>
                </a>
                <button
                  type="button"
                  class="unpin"
                  aria-label="${msg("Unpin view")}"
                  @click="${(e: Event) => {
                    e.preventDefault();
                    e.stopPropagation();
                    unpinView(v.id);
                  }}"
                >
                  <breeze-icon name="x" size="12"></breeze-icon>
                </button>
              </div>
            `;
            return collapsed
              ? html`
                <breeze-tooltip text="${v
                  .name}" side="right">${row}</breeze-tooltip>
              `
              : row;
          })}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-nav-views": BreezeNavViews;
  }
}
