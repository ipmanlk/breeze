import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import { reorderSections } from "@/store/dashboard";
import type { DtoDashboardSectionResponse } from "@/api";
import "./my-tasks-section.ts";
import "./due-soon-section.ts";
import "./activity-section.ts";
import "./stats-section.ts";
import "./projects-section.ts";
import "../../../components/ui/plume-icon.ts";
import "../../../components/ui/button.ts";
import "../../../components/ui/card.ts";
import { localized, msg } from "@lit/localize";

const SECTION_TYPES = [
  "due_soon",
  "my_tasks",
  "projects",
  "stats",
  "activity",
] as const;

function getSECTION_META(): Record<
  string,
  { label: string; description: string }
> {
  return {
    my_tasks: { label: msg("My Tasks"), description: "Tasks assigned to you" },
    due_soon: {
      label: msg("Due Soon"),
      description: "Upcoming and overdue tasks",
    },
    activity: {
      label: msg("Recent Activity"),
      description: "Latest workspace events",
    },
    stats: { label: msg("Your Stats"), description: "Task counts at a glance" },
    projects: {
      label: msg("Projects"),
      description: "Quick access to all projects",
    },
  };
}

const SECTION_ICONS: Record<string, string> = {
  my_tasks: "check-square",
  due_soon: "calendar-days",
  activity: "inbox",
  stats: "list-todo",
  projects: "folder",
};

@localized()
@customElement("plume-dashboard-home")
export class PlumeDashboardHome extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      height: 100%;
    }
    .page-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-4);
      padding: var(--space-4) var(--space-6);
      border-bottom: 1px solid var(--border);
      flex-shrink: 0;
    }
    .page-head h1 {
      margin: 0;
      font-size: var(--text-lg);
      font-weight: 600;
      font-family: var(--font-heading, inherit);
      color: var(--foreground);
    }
    .page-head p {
      margin: var(--space-1) 0 0;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
    .page-content {
      flex: 1;
      min-height: 0;
      overflow-y: auto;
      padding: var(--space-6);
    }
    .actions {
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .sections {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
    }
    .sections > * {
      animation: content-in var(--dur-normal) var(--ease-2) both;
    }
    .sections > *:nth-child(1) {
      animation-delay: calc(0 * var(--dur-instant));
    }
    .sections > *:nth-child(2) {
      animation-delay: calc(1 * var(--dur-instant));
    }
    .sections > *:nth-child(3) {
      animation-delay: calc(2 * var(--dur-instant));
    }
    .sections > *:nth-child(4) {
      animation-delay: calc(3 * var(--dur-instant));
    }
    .sections > *:nth-child(5) {
      animation-delay: calc(4 * var(--dur-instant));
    }
    .empty-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: var(--space-3);
      padding: var(--space-8);
      border: 1px dashed var(--border);
      border-radius: var(--radius-lg);
      background: var(--card);
      text-align: center;
    }
    .empty-state .icon-wrap {
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--avatar-lg);
      height: var(--avatar-lg);
      border-radius: var(--radius-full);
      background: var(--muted);
    }
    .empty-state .label {
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .empty-state .sublabel {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }

    /* Customize dropdown */
    .customize-panel {
      position: absolute;
      top: calc(100% + var(--space-1));
      right: 0;
      z-index: var(--z-dropdown);
      min-width: var(--menu-w);
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
    }
    .customize-panel .panel-title {
      padding: var(--space-1) var(--space-2);
      font-size: var(--text-xs);
      font-weight: 600;
      color: var(--muted-foreground);
    }
    .customize-panel .item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      text-align: left;
    }
    .customize-panel .item:hover {
      background: var(--accent);
    }
    .customize-panel .check {
      width: var(--space-4);
      height: var(--space-4);
      flex-shrink: 0;
    }
    .customize-panel .check.checked {
      opacity: 1;
    }
    .customize-panel .check.unchecked {
      opacity: 0;
    }
    .customize-panel .item-label {
      display: flex;
      flex-direction: column;
    }
    .customize-panel .item-desc {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }

    /* New dropdown */
    .new-dropdown {
      position: absolute;
      top: calc(100% + var(--space-1));
      right: 0;
      z-index: var(--z-dropdown);
      min-width: var(--menu-w-sm);
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
    }
    .new-dropdown button {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1) var(--space-2);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      text-align: left;
    }
    .new-dropdown button:hover {
      background: var(--accent);
    }

    .skel-card {
      padding: var(--space-6);
    }
    .skel-line {
      background: var(--muted);
      border-radius: var(--radius-sm);
    }
    .skel-line.skeleton-shimmer {
      background: linear-gradient(
        90deg,
        var(--muted) 0%,
        color-mix(in oklch, var(--muted) 60%, var(--foreground) 5%) 40%,
        var(--muted) 80%
      );
      background-size: 200% 100%;
    }
    .skel-title {
      height: var(--space-4);
      width: var(--space-24);
      margin-bottom: var(--space-4);
    }
    .skel-row-full {
      height: var(--space-8);
      width: 100%;
      margin-bottom: var(--space-2);
    }
    .skel-row-75 {
      height: var(--space-8);
      width: 75%;
      margin-bottom: var(--space-2);
    }
    .skel-row-50 {
      height: var(--space-8);
      width: 50%;
    }

    .backdrop {
      position: fixed;
      inset: 0;
      z-index: var(--z-overlay);
    }
  `;

  @property({ attribute: false })
  sections: DtoDashboardSectionResponse[] = [];

  @property({ attribute: false })
  isLoading = false;

  @property({ type: Boolean })
  showCustomize = false;

  @property({ type: Boolean })
  showNew = false;

  #onBackdropClick = () => {
    this.showCustomize = false;
    this.showNew = false;
  };

  #toggleSection(type: string) {
    const current = this.sections.map((s) => s.type ?? "").filter(Boolean);
    const set = new Set(current);
    if (set.has(type)) {
      set.delete(type);
    } else {
      set.add(type);
    }
    const ordered = SECTION_TYPES.filter((t) => set.has(t));
    reorderSections([...ordered]);
  }

  protected render() {
    const sectionTypes = this.sections
      .map((s) => s.type ?? "")
      .filter(Boolean);

    return html`
      ${this.showCustomize || this.showNew
        ? html`
          <div class="backdrop" @click="${this.#onBackdropClick}"></div>
        `
        : ""}

      <div class="page-head">
        <div>
          <h1>${msg("Dashboard")}</h1>
          <p>${msg("Here's what's happening across your workspace.")}</p>
        </div>
        <div class="actions">
          <div style="position:relative">
            <plume-button size="sm" @click="${() => {
              this.showNew = !this.showNew;
              this.showCustomize = false;
            }}">
              <plume-icon name="plus" size="16"></plume-icon>
              ${msg("New")}
            </plume-button>
            ${this.showNew
              ? html`
                <div class="new-dropdown">
                  <button
                    @click="${() => {
                      this.showNew = false;
                      navigate("/projects");
                    }}"
                  >
                    <plume-icon name="folder" size="16"></plume-icon>
                    ${msg("New project")}
                  </button>
                  <button
                    @click="${() => {
                      this.showNew = false;
                      navigate("/my-tasks");
                    }}"
                  >
                    <plume-icon name="check-square" size="16"></plume-icon>
                    ${msg("New task")}
                  </button>
                  <button
                    @click="${() => {
                      this.showNew = false;
                      navigate("/messages");
                    }}"
                  >
                    <plume-icon name="message-square" size="16"></plume-icon>
                    ${msg("New message")}
                  </button>
                </div>
              `
              : ""}
          </div>
          <div style="position:relative">
            <plume-button variant="outline" size="sm" @click="${() => {
              this.showCustomize = !this.showCustomize;
              this.showNew = false;
            }}">
              <plume-icon name="settings-2" size="16"></plume-icon>
              ${msg("Customize")}
            </plume-button>
            ${this.showCustomize
              ? html`
                <div class="customize-panel">
                  <div class="panel-title">${msg("Dashboard sections")}</div>
                  ${SECTION_TYPES.map((type) => {
                    const meta = getSECTION_META()[type] ?? {
                      label: type,
                      description: "",
                    };
                    const checked = sectionTypes.includes(type);
                    return html`
                      <button
                        class="item"
                        @click="${() => this.#toggleSection(type)}"
                      >
                        <plume-icon
                          name="check"
                          size="16"
                          style="opacity:${checked ? 1 : 0};flex-shrink:0"
                        ></plume-icon>
                        <div class="item-label">
                          <span>${meta.label}</span>
                          <span class="item-desc">${meta.description}</span>
                        </div>
                      </button>
                    `;
                  })}
                </div>
              `
              : ""}
          </div>
        </div>
      </div>

      <div class="page-content">
        ${this.isLoading
          ? html`
            <div class="sections">
              ${SECTION_TYPES.slice(0, 2).map(
                (_t) =>
                  html`
                    <plume-card class="skel-card">
                      <div class="skel-line skel-title skeleton-shimmer"></div>
                      <div class="skel-line skel-row-full skeleton-shimmer"></div>
                      <div class="skel-line skel-row-75 skeleton-shimmer"></div>
                      <div class="skel-line skel-row-50 skeleton-shimmer"></div>
                    </plume-card>
                  `,
              )}
            </div>
          `
          : this.sections.length === 0
          ? html`
            <div class="sections">
              <div class="empty-state">
                <div class="label">${msg("All sections hidden")}</div>
                <div class="sublabel">${msg(
                  'Click "Customize" to add sections back to your dashboard.',
                )}</div>
              </div>
            </div>
          `
          : html`
            <div class="sections">
              ${this.sections.map((s) => {
                const type = s.type ?? "";
                if (
                  !s.data ||
                  (Array.isArray(s.data) && s.data.length === 0)
                ) {
                  const label = getSECTION_META()[type]?.label ?? type;
                  const iconName = SECTION_ICONS[type] ?? "inbox";
                  return html`
                    <div class="empty-state">
                      <div class="icon-wrap">
                        <plume-icon
                          name="${iconName}"
                          size="20"
                          style="color:var(--muted-foreground)"
                        ></plume-icon>
                      </div>
                      <div class="label">${label}</div>
                      <div class="sublabel">${msg("Nothing to show yet.")}</div>
                    </div>
                  `;
                }
                return this.#renderSection(s);
              })}
            </div>
          `}
      </div>
    `;
  }

  #renderSection(s: DtoDashboardSectionResponse) {
    switch (s.type) {
      case "my_tasks":
        return html`
          <plume-my-tasks-section .data="${s.data}"></plume-my-tasks-section>
        `;
      case "due_soon":
        return html`
          <plume-due-soon-section .data="${s.data}"></plume-due-soon-section>
        `;
      case "activity":
        return html`
          <plume-activity-section
            .data="${s.data}"
          ></plume-activity-section>
        `;
      case "stats":
        return html`
          <plume-stats-section .data="${s.data}"></plume-stats-section>
        `;
      case "projects":
        return html`
          <plume-projects-section
            .data="${s.data}"
          ></plume-projects-section>
        `;
      default:
        return html`
        `;
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-dashboard-home": PlumeDashboardHome;
  }
}
