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
  status_name: string;
  project_id: string;
  project_name: string;
  project_slug: string;
  due_at?: string;
}

function formatRelative(t: string): string {
  const d = new Date(t);
  const now = new Date();
  const diff = d.getTime() - now.getTime();
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24));
  if (days < 0) return `${Math.abs(days)}d overdue`;
  if (days === 0) return msg("Today");
  if (days === 1) return msg("Tomorrow");
  return `In ${days}d`;
}

@localized()
@customElement("plume-due-soon-section")
export class PlumeDueSoonSection extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }
    .title {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .section-label {
      margin-bottom: var(--space-1);
      font-size: var(--text-xs);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted-foreground);
    }
    .section-label.overdue {
      color: var(--destructive);
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
    .task-title {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .task-due {
      flex-shrink: 0;
      font-size: var(--text-xs);
    }
    .task-due.overdue {
      color: var(--destructive);
    }
    .task-due.upcoming {
      color: var(--muted-foreground);
    }
    .list {
      display: flex;
      flex-direction: column;
      gap: var(--space-0-5);
      padding-top: var(--space-2);
    }
    .empty {
      padding: var(--space-4) 0;
      text-align: center;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .groups {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
      padding-top: var(--space-2);
    }
  `;

  @property({ attribute: false })
  data: unknown = null;

  protected render() {
    const tasks = (this.data as DashboardTask[] | null) ?? [];
    if (tasks.length === 0) {
      return html`
        <plume-card>
          <div class="title">
            <plume-icon name="calendar-days" size="16"></plume-icon>
            Due Soon
          </div>
          <div class="empty">Nothing due: you're all caught up.</div>
        </plume-card>
      `;
    }

    const overdue = tasks.filter(
      (t) => t.due_at && new Date(t.due_at) < new Date(),
    );
    const upcoming = tasks.filter(
      (t) => t.due_at && new Date(t.due_at) >= new Date(),
    );

    return html`
      <plume-card>
        <div class="title">
          <plume-icon name="calendar-days" size="16"></plume-icon>
          Due Soon
        </div>
        <div class="groups">
          ${overdue.length > 0
            ? html`
              <div>
                <div class="section-label overdue">
                  Overdue (${overdue.length})
                </div>
                <div class="list">
                  ${overdue.map(
                    (task) =>
                      html`
                        <button
                          class="task-item"
                          @click="${() =>
                            navigate(
                              `/projects/${task.project_slug}?task=${task.id}`,
                            )}"
                        >
                          <plume-icon
                            name="clock"
                            size="12"
                            style="color:var(--destructive);flex-shrink:0"
                          ></plume-icon>
                          <span class="task-title">${task.title}</span>
                          ${task.due_at
                            ? html`
                              <span class="task-due overdue">
                                ${formatRelative(task.due_at)}
                              </span>
                            `
                            : ""}
                        </button>
                      `,
                  )}
                </div>
              </div>
            `
            : ""} ${upcoming.length > 0
            ? html`
              <div>
                <div class="section-label">
                  Upcoming (${upcoming.length})
                </div>
                <div class="list">
                  ${upcoming.map(
                    (task) =>
                      html`
                        <button
                          class="task-item"
                          @click="${() =>
                            navigate(
                              `/projects/${task.project_slug}?task=${task.id}`,
                            )}"
                        >
                          <plume-icon
                            name="calendar-days"
                            size="12"
                            style="color:var(--muted-foreground);flex-shrink:0"
                          ></plume-icon>
                          <span class="task-title">${task.title}</span>
                          ${task.due_at
                            ? html`
                              <span class="task-due upcoming">
                                ${formatRelative(task.due_at)}
                              </span>
                            `
                            : ""}
                        </button>
                      `,
                  )}
                </div>
              </div>
            `
            : ""}
        </div>
      </plume-card>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-due-soon-section": PlumeDueSoonSection;
  }
}
