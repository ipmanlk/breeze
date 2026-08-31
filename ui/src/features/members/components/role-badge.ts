import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import "@/components/ui/plume-icon.ts";

/**
 * Inline role badge showing a user's org or project role.
 *
 * Roles: owner (primary), admin (orange), member (secondary), viewer (outline).
 * Theme-aware using `light-dark()` for the admin variant.
 */
@customElement("plume-role-badge")
export class PlumeRoleBadge extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      height: var(--badge-h, 1.25rem);
      padding: 0 var(--space-2);
      border-radius: var(--radius-full);
      font-size: var(--text-xs);
      font-weight: 500;
      text-transform: capitalize;
      white-space: nowrap;
      user-select: none;
      transition:
        background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    :host(.badge-update) {
      animation: badge-pop var(--dur-normal) var(--ease-spring);
    }
    /* owner: primary */
    :host([role="owner"]) {
      background: var(--primary);
      color: var(--primary-foreground);
    }
    /* admin: orange tint */
    :host([role="admin"]) {
      background: light-dark(
        oklch(0.95 0.02 70),
        oklch(0.25 0.03 70)
      );
      color: light-dark(
        oklch(0.45 0.12 55),
        oklch(0.85 0.08 70)
      );
    }
    /* member: secondary */
    :host([role="member"]) {
      background: var(--secondary);
      color: var(--secondary-foreground);
    }
    /* viewer: outline */
    :host([role="viewer"]) {
      background: transparent;
      color: var(--muted-foreground);
      border: 1px solid var(--border);
    }
    /* guest: muted with border + icon treatment */
    :host([role="guest"]) {
      background: color-mix(in oklch, var(--muted) 60%, transparent);
      color: var(--muted-foreground);
      border: 1px dashed var(--border);
      font-style: italic;
    }
    .badge-icon {
      display: inline-flex;
      align-items: center;
      opacity: 0.7;
    }
  `;

  @property({ reflect: true })
  role: "owner" | "admin" | "member" | "viewer" | "guest" | string = "member";

  protected render() {
    const label = this.role.charAt(0).toUpperCase() + this.role.slice(1);
    if (this.role === "guest") {
      return html`
        <span class="badge-icon">
          <plume-icon name="shield" size="10"></plume-icon>
        </span>
        ${label}
      `;
    }
    return html`
      ${label}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-role-badge": PlumeRoleBadge;
  }
}
