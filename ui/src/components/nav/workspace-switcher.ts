import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { auth } from "@/store/auth";
import { sidebarIsMobile, sidebarOpen } from "@/store/sidebar";
import { navigate } from "@/routes/router";
import { localized, msg } from "@lit/localize";
import {
  switchActiveWorkspace,
  workspaces as workspacesSignal,
} from "@/store/workspaces";
import { SignalController } from "@/lib/signal-controller";
import { OutsideClickController } from "@/lib/outside-click-controller";
import type { DtoWorkspaceResponse } from "@/api";
import "../ui/plume-icon.ts";
import "../ui/tooltip.ts";
import "./create-workspace-dialog.ts";

/**
 * Workspace switcher: sidebar header.
 *
 * Shows the active workspace (org) avatar initial + name + slug, with a
 * dropdown of the user's workspaces (switch), an "Add workspace" action that
 * opens a create dialog, and a keyboard shortcut hint.
 *
 * The dropdown is in-DOM (no portal). When collapsed, the sidebar's
 * `overflow-x: hidden` would clip a right-opening panel: so in collapsed mode
 * we show a name tooltip instead (see docs/ui/sidebar.md "Collapsed dropdowns
 * clip").
 */
@localized()
@customElement("plume-workspace-switcher")
export class PlumeWorkspaceSwitcher extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      position: relative;
    }

    .trigger {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border-radius: var(--radius-md);
      border: none;
      background: transparent;
      color: var(--sidebar-foreground);
      font-size: var(--text-sm);
      line-height: var(--space-5);
      min-height: var(--space-12);
      cursor: pointer;
      outline: none;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .trigger:hover {
      background: var(--sidebar-accent);
    }
    .trigger:focus-visible {
      box-shadow: 0 0 0 2px
        color-mix(in oklch, var(--sidebar-ring) 40%, transparent);
    }
    :host([data-collapsed="true"]) .trigger {
      justify-content: center;
      padding: var(--space-2);
    }

    /* Collapsed logo wrapper: full-width centered row, matching the
      plume-nav-list / -views / -projects collapsed pattern (see
      docs/ui/sidebar.md "Collapsed items: icon-centered rows"). The logo
      is a fixed 32px square; without a full-width justify-center container
      it sits at flex-start and overflows asymmetrically (clipped right by
      the sidebar's overflow-x: hidden), so it looks off-center vs the nav
      icons which center symmetrically. */
    .collapsed-wrap {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
    }

    .logo {
      width: var(--space-8);
      height: var(--space-8);
      border-radius: var(--radius-md);
      background: var(--primary);
      color: var(--primary-foreground);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-sm);
      font-weight: 600;
      flex-shrink: 0;
    }

    .info {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-width: 0;
      text-align: left;
      line-height: var(--space-4);
    }
    .name {
      font-weight: 600;
      font-size: var(--text-sm);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .slug {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .chevron {
      margin-left: auto;
      color: var(--muted-foreground);
    }

    :host([data-collapsed="true"]) .info,
    :host([data-collapsed="true"]) .chevron {
      display: none;
    }

    /* ---- Dropdown ---- */
    .menu {
      position: absolute;
      z-index: var(--z-dropdown);
      min-width: var(--menu-w);
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      top: calc(100% + var(--space-1));
      left: 0;
      right: 0;
      max-height: 18rem;
      overflow-y: auto;
    }

    .menu-label {
      padding: var(--space-1-5) var(--space-2);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .menu-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      text-align: left;
      cursor: pointer;
      outline: none;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .menu-item:hover {
      background: var(--accent);
    }
    .menu-item[disabled] {
      opacity: 0.6;
      cursor: default;
    }
    .menu-item .ico-box {
      width: var(--space-6);
      height: var(--space-6);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-xs);
      font-weight: 600;
      flex-shrink: 0;
    }
    .menu-item .text {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
      line-height: var(--space-4);
    }
    .menu-item .sub {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .menu-item .muted {
      color: var(--muted-foreground);
    }
    .menu-item .check,
    .menu-item .shortcut {
      margin-left: auto;
      color: var(--muted-foreground);
      font-size: var(--text-xs);
    }
    .menu-item.active {
      background: var(--accent);
      font-weight: 500;
    }
    .sep {
      height: var(--space-px);
      background: var(--border);
      margin: var(--space-1) 0;
    }
  `;

  @state()
  private _open = false;

  @state()
  private _switching: string | null = null;

  @state()
  private _showCreate = false;

  #signals = new SignalController(this);

  private _outsideClick = new OutsideClickController(this, () => {
    this._open = false;
  });
  private _onKeydown = (e: KeyboardEvent): void => {
    if (e.key === "Escape") this._open = false;
  };

  constructor() {
    super();
    this.#signals.watch(
      auth,
      sidebarOpen,
      sidebarIsMobile,
      workspacesSignal,
    );
  }

  protected willUpdate(): void {
    this.dataset.collapsed = String(
      !sidebarOpen.value && !sidebarIsMobile.value,
    );
  }

  protected updated(changedProps: Map<string, unknown>): void {
    if (!changedProps.has("_open")) return;
    if (this._open) {
      this._outsideClick.connect();
      document.addEventListener("keydown", this._onKeydown);
    } else {
      this._outsideClick.disconnect();
      document.removeEventListener("keydown", this._onKeydown);
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onKeydown);
  }

  private _toggle(e: Event): void {
    e.stopPropagation();
    this._open = !this._open;
  }

  private _close(): void {
    this._open = false;
  }

  private async _switch(ws: DtoWorkspaceResponse): Promise<void> {
    if (this._switching) return;
    const id = ws.id ?? "";
    if (!id) return;
    if (id === workspacesSignal.value.activeOrgID) {
      this._close();
      return;
    }
    this._switching = id;
    try {
      await switchActiveWorkspace(id);
      // Navigate to the dashboard so no stale org-scoped route (a project,
      // chat conversation, etc.) from the previous workspace remains.
      navigate("/");
      this._close();
    } catch {
      // keep menu open on error so the user can retry
    } finally {
      this._switching = null;
    }
  }

  private _openCreate(): void {
    this._close();
    this._showCreate = true;
  }

  protected render(): unknown {
    const org = auth.value.user?.org ?? null;
    const name = org?.name ?? msg("Workspace");
    const slug = org?.slug ?? "";
    const initial = name.charAt(0).toUpperCase() || "W";
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;

    const logo = html`
      <span class="logo">${initial}</span>
    `;

    // When collapsed, the sidebar's `overflow-x: hidden` would clip a
    // right-opening in-DOM dropdown. Show a name tooltip instead of a
    // clipped dropdown.
    if (collapsed) {
      return html`
        <plume-tooltip text="${name}" side="right">
          <div class="collapsed-wrap">${logo}</div>
        </plume-tooltip>
      `;
    }

    const wsList = workspacesSignal.value.workspaces;
    const activeID = workspacesSignal.value.activeOrgID || org?.id || "";

    return html`
      <button
        type="button"
        class="trigger"
        @click="${(e: Event) => this._toggle(e)}"
      >
        ${logo}
        <span class="info">
          <span class="name">${name}</span>
          <span class="slug">${slug}</span>
        </span>
        <plume-icon
          class="chevron"
          name="chevrons-up-down"
          size="16"
        ></plume-icon>
      </button>
      ${this._open
        ? html`
          <div class="menu" role="menu">
            <div class="menu-label">${msg("Workspaces")}</div>
            ${wsList.map((ws) => this._renderWorkspaceItem(ws, activeID))}
            ${wsList.length > 0 ? html`<div class="sep"></div>` : ""}
            <button
              type="button"
              class="menu-item"
              @click="${() => {
                this._close();
                navigate("/settings/workspace");
              }}"
            >
              <span class="ico-box">
                <plume-icon name="settings" size="16"></plume-icon>
              </span>
              <span class="text">${msg("Workspace settings")}</span>
            </button>
            <button
              type="button"
              class="menu-item"
              @click="${() => this._openCreate()}"
            >
              <span class="ico-box">
                <plume-icon name="plus" size="16"></plume-icon>
              </span>
              <span class="text">${msg("Add workspace")}</span>
            </button>
          </div>
        `
        : ""}
      <plume-create-workspace-dialog
        .open="${this._showCreate}"
        @close="${() => (this._showCreate = false)}"
      ></plume-create-workspace-dialog>
    `;
  }

  private _renderWorkspaceItem(ws: DtoWorkspaceResponse, activeID: string) {
    const isActive = ws.id === activeID;
    const isSwitching = this._switching === ws.id;
    const initial = (ws.name ?? "").charAt(0).toUpperCase() || "W";
    return html`
      <button
        type="button"
        class="menu-item ${isActive ? "active" : ""}"
        ?disabled="${isSwitching}"
        @click="${() => this._switch(ws)}"
      >
        <span class="ico-box">${initial}</span>
        <span class="text">
          ${ws.name}
          ${ws.is_owner
            ? html`<span class="sub">${msg("Owner")}</span>`
            : html`<span class="sub muted">${ws.role}</span>`}
        </span>
        ${isActive
          ? html`<plume-icon class="check" name="check" size="16"></plume-icon>`
          : isSwitching
          ? html`<plume-icon class="check" name="loader-2" size="16"></plume-icon>`
          : ""}
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-workspace-switcher": PlumeWorkspaceSwitcher;
  }
}
