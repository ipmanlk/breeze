import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized } from "@lit/localize";
import { currentPath, navigate } from "@/routes/router";
import { sidebarIsMobile, sidebarOpen } from "@/store/sidebar";
import { unreadCount } from "@/features/notifications/store";
import { SignalController } from "@/lib/signal-controller";
import type { NavItem } from "./nav-config";
import "../ui/breeze-icon.ts";
import "../ui/tooltip.ts";

function isActive(currentPath: string, itemUrl: string): boolean {
  if (itemUrl === "/") return currentPath === "/";
  return currentPath === itemUrl || currentPath.startsWith(`${itemUrl}/`);
}

/**
 * Sidebar nav group: optional label + a flush list of menu-button rows.
 * Mirrors shadcn `SidebarGroup` + `SidebarMenu` + `SidebarMenuButton`:
 * group has `p-2`, items are `gap-0` (flush), each button is `h-8 p-2 gap-2
 * rounded-md`, and the group label is `h-8 px-2 text-xs font-medium
 * text-sidebar-foreground/70` (no uppercase).
 */
@localized()
@customElement("breeze-nav-list")
export class BreezeNavList extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
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
    a {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      height: var(--space-8);
      padding: 0 var(--space-2);
      border-radius: var(--radius-md);
      font-size: var(--text-sm);
      color: var(--sidebar-foreground);
      text-decoration: none;
      cursor: pointer;
      outline: none;
      transition:
        background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    a:focus-visible {
      box-shadow: 0 0 0 2px
        color-mix(in oklch, var(--sidebar-ring) 40%, transparent);
    }
    a:hover {
      background: var(--sidebar-accent);
      color: var(--sidebar-accent-foreground);
    }
    a.active {
      background: var(--sidebar-accent);
      color: var(--sidebar-accent-foreground);
      font-weight: 500;
    }
    breeze-icon {
      flex-shrink: 0;
      opacity: 0.85;
    }
    .text {
      flex: 1;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .badge {
      flex-shrink: 0;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      height: var(--space-5);
      min-width: var(--space-5);
      padding: 0 var(--space-1-5);
      border-radius: var(--radius-full);
      background: var(--secondary);
      color: var(--secondary-foreground);
      font-size: var(--text-xs);
      font-weight: 500;
      font-variant-numeric: tabular-nums;
    }
    :host([data-collapsed="true"]) a {
      justify-content: center;
    }
    :host([data-collapsed="true"]) .text,
    :host([data-collapsed="true"]) .badge {
      display: none;
    }
  `;

  @property({ attribute: false })
  items: NavItem[] = [];

  @property()
  label = "";

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(currentPath, sidebarOpen, sidebarIsMobile, unreadCount);
  }

  protected willUpdate() {
    this.dataset.collapsed = String(
      !sidebarOpen.value && !sidebarIsMobile.value,
    );
  }

  protected render() {
    const path = currentPath.value;
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;
    return html`
      <div class="group">
        ${this.label
          ? html`
            <div class="group-label">${this.label}</div>
          `
          : ""}
        <div class="menu">
          ${this.items.map((item) => {
            const active = isActive(path, item.url);
            // Inbox shows the live unread count.
            const badge = item.url === "/inbox"
              ? (unreadCount.value > 0 ? unreadCount.value : undefined)
              : item.badge;
            const preloadProjectDetail = () => {
              import("../../features/projects/project-detail-page.ts");
            };
            const isProjectsLink = item.url === "/projects";
            const link = html`
              <a
                class="${active ? "active" : ""}"
                @click="${(e: Event) => {
                  e.preventDefault();
                  navigate(item.url);
                }}"
                href="${item.url}"
                @mouseenter="${isProjectsLink
                  ? preloadProjectDetail
                  : undefined}"
                @focus="${isProjectsLink ? preloadProjectDetail : undefined}"
              >
                <breeze-icon name="${item.icon}" size="16"></breeze-icon>
                <span class="text">${item.title}</span>
                ${badge != null
                  ? html`
                    <span class="badge">${badge}</span>
                  `
                  : ""}
              </a>
            `;
            return collapsed
              ? html`
                <breeze-tooltip
                  text="${item.title}"
                  side="right"
                  .hidden="${!collapsed}"
                >
                  ${link}
                </breeze-tooltip>
              `
              : link;
          })}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-nav-list": BreezeNavList;
  }
}
