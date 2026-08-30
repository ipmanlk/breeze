import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { currentPath, navigate } from "@/routes/router";
import { projects } from "@/store/projects";
import { sidebarIsMobile, sidebarOpen } from "@/store/sidebar";
import { SignalController } from "@/lib/signal-controller";
import "../ui/breeze-icon.ts";
import "../ui/tooltip.ts";

/**
 * Sidebar "Projects" group: lists the org's projects (color badge + initial
 * + name) and an "Add project" action.
 *
 * Spacing matches shadcn: group `p-2`, items flush (`gap-0`), each row is
 * `size="sm"` (`h-7 text-xs p-2 gap-2 rounded-md`). Project data is fetched
 * once at the app level (`app-shell.ts`); this component only reads the
 * `projects` signal.
 */
@localized()
@customElement("breeze-nav-projects")
export class BreezeNavProjects extends LitElement {
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

    .badge {
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-sm);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-2xs);
      font-weight: 600;
      color: #fff; /* fixed contrast on saturated color (design-tokens.md) */
      flex-shrink: 0;
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

    /* hover "more" action: presentational only */
    .action {
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
    .item:hover .action {
      display: inline-flex;
    }
    .action:hover {
      background: var(--sidebar);
      color: var(--sidebar-foreground);
    }
    :host([data-collapsed="true"]) .action {
      display: none !important;
    }

    .plus-slot {
      width: var(--space-4);
      height: var(--space-4);
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
    }
  `;

  #signals = new SignalController(this);

  constructor() {
    super();
    this.#signals.watch(currentPath, projects, sidebarOpen, sidebarIsMobile);
  }

  protected willUpdate(): void {
    this.dataset.collapsed = String(
      !sidebarOpen.value && !sidebarIsMobile.value,
    );
  }

  protected render(): unknown {
    const path = currentPath.value;
    const collapsed = !sidebarOpen.value && !sidebarIsMobile.value;
    const items = projects.value.projects;

    const projectRow = (
      p: { slug?: string; name?: string; color?: string },
    ) => {
      const slug = p.slug ?? "";
      const name = p.name ?? "";
      const active = path === `/projects/${slug}`;
      const initial = name.charAt(0).toUpperCase();
      const row = html`
        <div class="item">
          <a
            class="row ${active ? "active" : ""}"
            href="/projects/${slug}"
            @click="${(e: Event) => {
              e.preventDefault();
              navigate(`/projects/${slug}`);
            }}"
          >
            <span class="badge" style="background-color: ${p
              .color}" aria-hidden>
              ${initial}
            </span>
            <span class="name">${name}</span>
          </a>
          <button
            type="button"
            class="action"
            aria-label="${msg("More")}"
            @click="${(e: Event) => {
              e.preventDefault();
              e.stopPropagation();
            }}"
          >
            <breeze-icon name="more-horizontal" size="16"></breeze-icon>
          </button>
        </div>
      `;
      return collapsed
        ? html`
          <breeze-tooltip text="${name}" side="right">${row}</breeze-tooltip>
        `
        : row;
    };

    const addRow = html`
      <a
        class="row"
        href="/projects?create=1"
        @click="${(e: Event) => {
          e.preventDefault();
          navigate("/projects?create=1");
        }}"
      >
        <span class="plus-slot">
          <breeze-icon name="plus" size="16"></breeze-icon>
        </span>
        <span class="name">${msg("Add project")}</span>
      </a>
    `;
    const addRowWrapped = collapsed
      ? html`
        <breeze-tooltip text="Add project" side="right">${addRow}</breeze-tooltip>
      `
      : addRow;

    return html`
      <div class="group">
        <div class="group-label">${msg("Projects")}</div>
        <div class="menu">
          ${items.map(projectRow)} ${addRowWrapped}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-nav-projects": BreezeNavProjects;
  }
}
