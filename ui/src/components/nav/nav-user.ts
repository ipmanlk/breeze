import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { createRef, ref } from "lit/directives/ref.js";
import { auth, logout } from "@/store/auth";
import { navigate } from "@/routes/router";
import { sidebarIsMobile, sidebarOpen } from "@/store/sidebar";
import { SignalController } from "@/lib/signal-controller";
import "../ui/plume-icon.ts";
import "../ui/avatar.ts";

function initials(name: string): string {
  return name
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

/**
 * Sidebar footer: the current user (avatar + name + email) with a menu
 * (Preferences, Sign out).
 *
 * Expanded: the menu is an absolutely-positioned panel opening upward, in-flow
 * inside this shadow root.
 *
 * Collapsed: the sidebar's `overflow-x: hidden` would clip a right-opening
 * in-flow panel (see docs/ui/sidebar.md "Collapsed dropdowns clip"). To keep
 * the menu reachable we render it via a portal to `document.body` with
 * `position: fixed`, anchored to the avatar: the one sanctioned exception to
 * the "in-shadow-DOM dropdown" rule documented in sidebar.md. The avatar is
 * also centered in a full-width row so it aligns with the other collapsed nav
 * icons (which center via a justify-center full-width row).
 */
@localized()
@customElement("plume-nav-user")
export class PlumeNavUser extends LitElement {
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
      position: relative;
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1-5) var(--space-2);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: var(--sidebar-foreground);
      font-size: var(--text-sm);
      line-height: var(--space-5);
      cursor: pointer;
      outline: none;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .trigger:hover,
    .trigger[aria-expanded="true"] {
      background: var(--sidebar-accent);
    }
    .trigger:focus-visible {
      box-shadow: 0 0 0 2px
        color-mix(in oklch, var(--sidebar-ring) 40%, transparent);
    }
    :host([data-collapsed="true"]) .trigger {
      justify-content: center;
    }

    /* Collapsed avatar wrapper: full-width centered row, matching the
      plume-nav-list / -views / -projects / workspace-switcher collapsed
      pattern (docs/ui/sidebar.md "Collapsed items: icon-centered rows"). */
    .collapsed-wrap {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
    }

    .info {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-width: 0;
      text-align: left;
      white-space: nowrap;
      overflow: hidden;
    }
    .name {
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .email {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .chevron {
      flex-shrink: 0;
    }
    :host([data-collapsed="true"]) .info,
    :host([data-collapsed="true"]) .chevron {
      display: none;
    }

    /* ---- Expanded menu (in-flow, opens upward) ---- */
    .menu {
      position: absolute;
      bottom: calc(100% + var(--space-1));
      left: 0;
      right: 0;
      min-width: var(--menu-w);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      padding: var(--space-1);
      z-index: var(--z-dropdown);
    }
    .menu .head {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2);
    }
    .menu button {
      display: block;
      width: 100%;
      text-align: left;
      padding: var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .menu button:hover {
      background: var(--accent);
    }
    .menu button.danger {
      color: var(--destructive);
    }
    .sep {
      height: var(--space-px);
      background: var(--border);
      margin: var(--space-1) 0;
    }
  `;

  /**
   * Global styles for the portaled (collapsed) menu. The panel lives in
   * document.body (light DOM), outside this shadow root, so it needs global
   * CSS: namespaced under `plume-nav-user-portal` to avoid collisions.
   */
  static portalStyles = `
    .plume-nav-user-portal {
      position: fixed;
      z-index: var(--z-dropdown);
      min-width: var(--menu-w);
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
    }
    .plume-nav-user-portal .head {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2);
    }
    .plume-nav-user-portal .info {
      display: flex;
      flex-direction: column;
      min-width: 0;
      text-align: left;
      white-space: nowrap;
      overflow: hidden;
    }
    .plume-nav-user-portal .name {
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .plume-nav-user-portal .email {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .plume-nav-user-portal button {
      display: block;
      width: 100%;
      text-align: left;
      padding: var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      cursor: pointer;
    }
    .plume-nav-user-portal button:hover {
      background: var(--accent);
    }
    .plume-nav-user-portal button.danger {
      color: var(--destructive);
    }
    .plume-nav-user-portal .sep {
      height: 1px;
      background: var(--border);
      margin: var(--space-1) 0;
    }
  `;

  @state()
  private _open = false;

  /** Ref to the avatar so the portaled (collapsed) menu can anchor to it. */
  #avatarRef = createRef<HTMLElement>();
  /** The portaled menu element (document.body child), if any. */
  #portal: HTMLDivElement | null = null;
  /** Last-rendered collapse state, to re-sync the portal when it changes. */
  #wasCollapsed = false;

  #signals = new SignalController(this);

  // Stable refs for document listeners (lit-patterns rule 15)
  private _onOutsideClick = (e: MouseEvent): void => {
    const target = e.target as Node | null;
    if (target && this.#portal?.contains(target)) return;
    if (!e.composedPath().includes(this)) this._open = false;
  };
  private _onKeydown = (e: KeyboardEvent): void => {
    if (e.key === "Escape") this._open = false;
  };
  // Reposition the fixed portal on scroll/resize so it tracks the avatar.
  private _onReposition = (): void => {
    this.#positionPortal();
  };

  constructor() {
    super();
    this.#signals.watch(auth, sidebarOpen, sidebarIsMobile);
  }

  protected willUpdate() {
    this.dataset.collapsed = String(
      !sidebarOpen.value && !sidebarIsMobile.value,
    );
  }

  protected updated(changedProps: Map<string, unknown>): void {
    const openChanged = changedProps.has("_open");
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;
    // Re-sync the portal when the open state OR the collapse state changes
    // (sidebar toggled while open: switch between in-flow .menu and the
    // portaled panel). The signal watch triggers a re-render on collapse
    // toggle, so we compare against the last-rendered collapse state.
    const collapseChanged = collapsed !== this.#wasCollapsed;
    this.#wasCollapsed = collapsed;

    if (openChanged) {
      if (this._open) {
        document.addEventListener("click", this._onOutsideClick);
        document.addEventListener("keydown", this._onKeydown);
        window.addEventListener("scroll", this._onReposition, true);
        window.addEventListener("resize", this._onReposition);
      } else {
        document.removeEventListener("click", this._onOutsideClick);
        document.removeEventListener("keydown", this._onKeydown);
        window.removeEventListener("scroll", this._onReposition, true);
        window.removeEventListener("resize", this._onReposition);
      }
    }
    if (openChanged || collapseChanged) {
      this.#syncPortal();
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener("click", this._onOutsideClick);
    document.removeEventListener("keydown", this._onKeydown);
    window.removeEventListener("scroll", this._onReposition, true);
    window.removeEventListener("resize", this._onReposition);
    this.#removePortal();
  }

  #toggle(e: Event): void {
    e.stopPropagation();
    this._open = !this._open;
  }

  #close(): void {
    this._open = false;
  }

  // Portal management for the collapsed menu
  #ensureStyles(): void {
    if (document.getElementById("plume-nav-user-portal-styles")) return;
    const style = document.createElement("style");
    style.id = "plume-nav-user-portal-styles";
    style.textContent = PlumeNavUser.portalStyles;
    document.head.appendChild(style);
  }

  #removePortal(): void {
    if (this.#portal) {
      this.#portal.remove();
      this.#portal = null;
    }
  }

  #syncPortal(): void {
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;
    // Only the collapsed menu is portaled (to escape overflow-x: hidden).
    // Expanded mode uses the in-flow .menu rendered into the shadow root.
    if (!this._open || !collapsed) {
      this.#removePortal();
      return;
    }
    this.#ensureStyles();

    const user = auth.value.user;
    if (!user) {
      this.#removePortal();
      return;
    }
    const init = initials(user.name ?? "");
    const email = user.email ?? "";

    if (!this.#portal) {
      this.#portal = document.createElement("div");
      this.#portal.className = "plume-nav-user-portal";
    }
    this.#portal.replaceChildren();
    // Light-DOM render of the menu contents. Uses plume-avatar (already a
    // custom element registered globally) so the avatar renders identically.
    const frag = document.createDocumentFragment();
    const head = document.createElement("div");
    head.className = "head";
    const av = document.createElement("plume-avatar");
    av.setAttribute("size", "sm");
    av.textContent = init;
    head.appendChild(av);
    const info = document.createElement("div");
    info.className = "info";
    const nameEl = document.createElement("span");
    nameEl.className = "name";
    nameEl.textContent = user.name ?? "";
    const emailEl = document.createElement("span");
    emailEl.className = "email";
    emailEl.textContent = email;
    info.appendChild(nameEl);
    info.appendChild(emailEl);
    head.appendChild(info);
    frag.appendChild(head);

    const sep1 = document.createElement("div");
    sep1.className = "sep";
    frag.appendChild(sep1);

    const prefBtn = document.createElement("button");
    prefBtn.type = "button";
    prefBtn.textContent = msg("Preferences");
    prefBtn.addEventListener("click", () => {
      this.#close();
      navigate("/preferences");
    });
    frag.appendChild(prefBtn);

    const sep2 = document.createElement("div");
    sep2.className = "sep";
    frag.appendChild(sep2);

    const signoutBtn = document.createElement("button");
    signoutBtn.type = "button";
    signoutBtn.className = "danger";
    signoutBtn.textContent = msg("Sign out");
    signoutBtn.addEventListener("click", () => {
      this.#close();
      void logout();
    });
    frag.appendChild(signoutBtn);

    this.#portal.appendChild(frag);
    document.body.appendChild(this.#portal);
    this.#positionPortal();
  }

  #positionPortal(): void {
    const avatarEl = this.#avatarRef.value;
    if (!this.#portal || !avatarEl) return;
    const rect = avatarEl.getBoundingClientRect();
    // Open to the right of the collapsed sidebar, vertically centered on the
    // avatar. Constrain within the viewport so it never overflows the right
    // edge.
    const menuW = this.#portal.offsetWidth;
    const gap = 8; // var(--space-2)
    let left = rect.right + gap;
    if (left + menuW > window.innerWidth - 8) {
      left = window.innerWidth - menuW - 8;
    }
    let top = rect.top + rect.height / 2 - this.#portal.offsetHeight / 2;
    top = Math.max(
      8,
      Math.min(top, window.innerHeight - this.#portal.offsetHeight - 8),
    );
    this.#portal.style.left = `${left}px`;
    this.#portal.style.top = `${top}px`;
  }

  protected render(): unknown {
    const user = auth.value.user;
    if (!user) {
      return html``;
    }
    const init = initials(user.name ?? "");
    const email = user.email ?? "";
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;

    const avatar = html`
      <plume-avatar size="sm" ${ref(this.#avatarRef)}>${init}</plume-avatar>
    `;

    return html`
      <button
        type="button"
        class="trigger"
        aria-expanded="${this._open ? "true" : "false"}"
        aria-haspopup="menu"
        title="${collapsed ? (user.name ?? msg("User")) : ""}"
        @click="${(e: Event) => this.#toggle(e)}"
      >
        ${collapsed
          ? html`<div class="collapsed-wrap">${avatar}</div>`
          : avatar}
        <div class="info">
          <span class="name">${user.name}</span>
          <span class="email">${email}</span>
        </div>
        <plume-icon class="chevron" name="chevron-up" size="16"></plume-icon>
      </button>
      ${this._open && !collapsed
        ? html`
          <div class="menu" role="menu">
            <div class="head">
              <plume-avatar size="sm">${init}</plume-avatar>
              <div class="info">
                <span class="name">${user.name}</span>
                <span class="email">${email}</span>
              </div>
            </div>
            <div class="sep"></div>
            <button type="button" @click="${() => {
              this.#close();
              navigate("/preferences");
            }}">${msg("Preferences")}</button>
            <div class="sep"></div>
            <button type="button" class="danger" @click="${() => {
              this.#close();
              void logout();
            }}">${msg("Sign out")}</button>
          </div>
        `
        : ""}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-nav-user": PlumeNavUser;
  }
}
