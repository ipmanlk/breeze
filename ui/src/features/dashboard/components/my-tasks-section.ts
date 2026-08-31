import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import "../../../components/ui/plume-icon.ts";
import "../../../components/ui/card.ts";
import { localized, msg } from "@lit/localize";

interface DashboardTask {
  id: string;
  title: string;
  priority: string;
  status_id: string;
  status_name: string;
  status_color: string;
  project_id: string;
  project_name: string;
  project_slug: string;
  due_at?: string;
}

function getPRIORITY_CONFIG(): Record<
  string,
  { icon: string; label: string }
> {
  return {
    urgent: { icon: "alert-circle", label: msg("Urgent") },
    high: { icon: "arrow-up", label: msg("High") },
    medium: { icon: "minus", label: msg("Medium") },
    low: { icon: "arrow-down", label: msg("Low") },
  };
}

@localized()
@customElement("plume-my-tasks-section")
export class PlumeMyTasksSection extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .title {
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .view-all {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      background: none;
      border: none;
      cursor: pointer;
      padding: var(--space-1) var(--space-2);
      border-radius: var(--radius-sm);
      font-family: inherit;
    }
    .view-all:hover {
      background: var(--accent);
    }
    .task-list {
      list-style: none;
      margin: 0;
      padding: var(--space-2) 0 0;
      display: flex;
      flex-direction: column;
      gap: var(--space-0-5);
    }
    .task-item {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      padding: var(--space-1) var(--space-2);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: inherit;
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      text-align: left;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .task-item:hover {
      background: var(--accent);
    }
    .status-dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .task-title {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .task-project {
      flex-shrink: 0;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: var(--space-0-5);
      height: var(--avatar-sm);
      padding: 0 var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-full);
      font-size: var(--text-xs);
      flex-shrink: 0;
    }
    .empty {
      padding: var(--space-4) 0;
      text-align: center;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
  `;

  @property({ attribute: false })
  data: unknown = null;

  protected render() {
    const tasks = (this.data as DashboardTask[] | null) ?? [];
    if (tasks.length === 0) {
      return html`
        <plume-card>
          <div class="header">
            <span class="title">${msg("My Tasks")}</span>
          </div>
          <div class="empty">${msg("No open tasks assigned to you.")}</div>
        </plume-card>
      `;
    }

    return html`
      <plume-card>
        <div class="header">
          <span class="title">${msg("My Tasks")}</span>
          <button class="view-all" @click="${() => navigate("/my-tasks")}">
            ${msg("View all")}
            <plume-icon name="arrow-right" size="12"></plume-icon>
          </button>
        </div>
        <ul class="task-list">
          ${tasks.slice(0, 8).map(
            (task) =>
              html`
                <li>
                  <button
                    class="task-item"
                    @click="${() =>
                      navigate(
                        `/projects/${task.project_slug}?task=${task.id}`,
                      )}"
                  >
                    <span
                      class="status-dot"
                      style="background:${task.status_color}"
                    ></span>
                    <span class="task-title">${task.title}</span>
                    <span class="task-project">${task.project_name}</span>
                    ${task.priority !== "none"
                      ? (() => {
                        const pc = getPRIORITY_CONFIG()[task.priority] ?? {
                          icon: "minus",
                          label: task.priority,
                        };
                        return html`
                          <span class="badge">
                            <plume-icon name="${pc
                              .icon}" size="12"></plume-icon>
                            ${pc.label}
                          </span>
                        `;
                      })()
                      : ""}
                  </button>
                </li>
              `,
          )}
        </ul>
      </plume-card>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-my-tasks-section": PlumeMyTasksSection;
  }
}
