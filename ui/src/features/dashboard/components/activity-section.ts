import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import { timeAgoShort } from "@/lib/format/time-ago";
import "../../../components/ui/breeze-icon.ts";
import "../../../components/ui/card.ts";
import { localized, msg } from "@lit/localize";

interface Activity {
  id: string;
  type: string;
  title: string;
  body: string;
  link: string;
  entity_type: string;
  entity_id: string;
  actor_name: string;
  project_slug: string;
  is_unread: boolean;
  created_at: string;
}

const TYPE_ICONS: Record<string, string> = {
  task_assigned: "user-plus",
  task_status_changed: "clipboard-list",
  task_mentioned: "message-square",
  task_due_soon: "bell",
  task_overdue: "bell",
  cycle_started: "bell",
  cycle_ending: "bell",
  comment_added: "message-square",
};

@localized()
@customElement("breeze-activity-section")
export class BreezeActivitySection extends LitElement {
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
      font-size: var(--text-sm);
      font-weight: 500;
    }
    .list {
      display: flex;
      flex-direction: column;
      gap: var(--space-0-5);
      padding-top: var(--space-2);
    }
    .activity-item {
      display: flex;
      align-items: flex-start;
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
    .activity-item:hover {
      background: var(--accent);
    }
    .activity-item.unread {
      font-weight: 500;
    }
    .activity-title {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .activity-actor {
      flex-shrink: 0;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .activity-time {
      flex-shrink: 0;
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .unread-dot {
      width: var(--space-1-5);
      height: var(--space-1-5);
      border-radius: var(--radius-full);
      background: var(--primary);
      flex-shrink: 0;
      margin-top: var(--space-0-5);
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
    const activities = (this.data as Activity[] | null) ?? [];
    if (activities.length === 0) {
      return html`
        <breeze-card>
          <div class="title">${msg("Recent Activity")}</div>
          <div class="empty">${msg("No recent activity.")}</div>
        </breeze-card>
      `;
    }

    return html`
      <breeze-card>
        <div class="title">${msg("Recent Activity")}</div>
        <ul class="list">
          ${activities.map((a) => {
            const iconName = TYPE_ICONS[a.type] ?? "bell";
            return html`
              <li>
                <button
                  class="activity-item ${a.is_unread ? "unread" : ""}"
                  @click="${() =>
                    navigate(
                      a.project_slug
                        ? `/projects/${a.project_slug}?task=${a.entity_id}`
                        : a.link,
                    )}"
                >
                  <breeze-icon
                    name="${iconName}"
                    size="14"
                    style="color:var(--muted-foreground);flex-shrink:0;margin-top:var(--space-0-5)"
                  ></breeze-icon>
                  <span class="activity-title">${a.title}</span>
                  ${a.actor_name
                    ? html`
                      <span class="activity-actor">${a.actor_name}</span>
                    `
                    : ""}
                  <span class="activity-time">${timeAgoShort(
                    a.created_at,
                    14,
                  )}</span>
                  ${a.is_unread
                    ? html`
                      <span class="unread-dot"></span>
                    `
                    : ""}
                </button>
              </li>
            `;
          })}
        </ul>
      </breeze-card>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-activity-section": BreezeActivitySection;
  }
}
