import { localized, msg } from "@lit/localize";
import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { getPrimaryNav } from "@/components/nav/nav-config";
import { currentPath } from "@/routes/router";
import { SignalController } from "@/lib/signal-controller";
import {
  cleanupSidebar,
  initSidebar,
  sidebarIsMobile,
  sidebarMobileOpen,
  sidebarOpen,
  toggleSidebar,
} from "@/store/sidebar";
import "../components/nav/nav-list.ts";
import "../components/nav/nav-projects.ts";
import "../components/nav/nav-views.ts";
import "../components/nav/nav-user.ts";
import "../components/nav/workspace-switcher.ts";
import "../components/top-bar.ts";

@localized()
@customElement("plume-app-layout")
export class PlumeAppLayout extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      min-height: 100svh;
    }

    /* ---- Sidebar ----
      Only the sidebar width transitions. Everything inside snaps.
      This matches shadcn: transition-[width] duration-200 ease-linear
      on the sidebar container. Inner content reflows naturally. */
    .sidebar {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      z-index: var(--z-sidebar);
      display: flex;
      flex-direction: column;
      width: var(--sidebar-w);
      border-right: 1px solid var(--sidebar-border);
      background: var(--sidebar);
      color: var(--sidebar-foreground);
      overflow-y: auto;
      overflow-x: hidden;
      transition: width var(--dur-slow) var(--ease-2);
      will-change: width;
    }
    :host([data-state="collapsed"]) .sidebar {
      width: var(--sidebar-w-icon);
    }

    .sidebar-inner {
      display: flex;
      flex-direction: column;
      height: 100%;
      /* No padding/gap here: each section (header, groups, footer) carries
        its own p-2, matching shadcn SidebarHeader/Group/Footer. */
    }

    /* ---- Sidebar header ---- */
    .sidebar-head {
      padding: var(--space-2);
    }

    /* ---- Nav scroll (content) ---- */
    .nav-scroll {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 0;
      overflow-y: auto;
      overflow-x: hidden;
      min-width: 0;
      min-height: 0;
    }

    /* ---- Sidebar footer ---- */
    .sidebar-foot {
      padding: var(--space-2);
    }

    /* ---- Sidebar rail ---- */
    .rail {
      position: absolute;
      top: 0;
      bottom: 0;
      right: -0.25rem;
      width: var(--space-2);
      cursor: e-resize;
      z-index: var(--z-sidebar);
    }
    .rail::after {
      content: "";
      position: absolute;
      top: 0;
      bottom: 0;
      left: 50%;
      width: var(--space-0-5);
      transform: translateX(-50%);
      transition: background var(--dur-fast) var(--ease-1);
    }
    .rail:hover::after {
      background: var(--sidebar-border);
    }

    /* ---- Main content ----
        Only margin-left transitions, in sync with sidebar width. */
    .main {
      display: flex;
      flex-direction: column;
      min-height: 100svh;
      margin-left: var(--sidebar-w);
      transition: margin-left var(--dur-slow) var(--ease-2);
    }
    /* Fullscreen mode: fix height to viewport so internal scroll areas
      (chat) are height-constrained: prevents content from growing the
      container and pushing fixed elements (e.g. chat input) off-screen. */
    :host([data-fullscreen]) .main {
      height: 100svh;
      min-height: 0;
    }
    :host([data-state="collapsed"]) .main {
      margin-left: var(--sidebar-w-icon);
    }
    .topbar-wrap {
      position: sticky;
      top: 0;
      z-index: calc(var(--z-sticky) + 1);
      background: var(--background);
      border-bottom: 1px solid var(--border);
    }
    .content {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-height: 0;
      padding: var(--space-4);
    }
    :host([data-fullscreen]) .content {
      overflow: hidden;
    }

    /* ---- Mobile ---- */
    .mobile-overlay {
      display: none;
    }

    @media (max-width: 48rem) {
      .sidebar {
        left: calc(var(--sidebar-w) * -1);
        width: var(--sidebar-w);
        transition: left var(--dur-slow) var(--ease-2);
      }
      :host([data-mobile-open="true"]) .sidebar {
        left: 0;
      }
      :host([data-state="collapsed"]) .sidebar {
        width: var(--sidebar-w);
      }
      .main,
      :host([data-state="collapsed"]) .main {
        margin-left: 0;
        transition: none;
      }
      .rail {
        display: none;
      }
      .mobile-overlay {
        display: block;
        position: fixed;
        inset: 0;
        z-index: var(--z-overlay);
        background: rgba(0, 0, 0, 0.4);
        opacity: 0;
        pointer-events: none;
        transition: opacity var(--dur-slow) var(--ease-2);
      }
      :host([data-mobile-open="true"]) .mobile-overlay {
        opacity: 1;
        pointer-events: auto;
      }
    }
  `;

  #signals = new SignalController(this);
  #prevPath = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(
      sidebarOpen,
      sidebarIsMobile,
      sidebarMobileOpen,
      currentPath,
    );
    initSidebar();
    this.#syncAttrs();
  }

  protected willUpdate() {
    this.#syncAttrs();
    // Auto-close mobile sidebar on route change only (not on every signal tick).
    const path = currentPath.value;
    if (path !== this.#prevPath) {
      this.#prevPath = path;
      if (sidebarIsMobile.value && sidebarMobileOpen.value) {
        sidebarMobileOpen.value = false;
      }
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    cleanupSidebar();
  }

  #syncAttrs() {
    const open = sidebarOpen.value;
    const mobile = sidebarIsMobile.value;
    const mobileOpen = sidebarMobileOpen.value;
    this.dataset.state = mobile
      ? mobileOpen ? "expanded" : "collapsed"
      : open
      ? "expanded"
      : "collapsed";
    this.dataset.mobileOpen = String(mobileOpen);
  }

  protected render() {
    return html`
      <nav class="sidebar" aria-label="${msg("Main navigation")}">
        <div class="sidebar-inner">
          <div class="sidebar-head">
            <plume-workspace-switcher></plume-workspace-switcher>
          </div>
          <div class="nav-scroll">
            <plume-nav-list .items="${getPrimaryNav()}"></plume-nav-list>
            <plume-nav-views></plume-nav-views>
            <plume-nav-projects></plume-nav-projects>
          </div>
          <div class="sidebar-foot">
            <plume-nav-user></plume-nav-user>
          </div>
        </div>
        <div
          class="rail"
          @click="${toggleSidebar}"
          aria-label="${msg("Toggle sidebar")}"
          title="${msg("Toggle sidebar")}"
        ></div>
      </nav>
      <div class="mobile-overlay" @click="${toggleSidebar}"></div>
      <div class="main">
        <div class="topbar-wrap" role="banner">
          <plume-top-bar></plume-top-bar>
        </div>
        <div class="content"><slot></slot></div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-app-layout": PlumeAppLayout;
  }
}
