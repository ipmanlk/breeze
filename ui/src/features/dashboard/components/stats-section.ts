import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import "../../../components/ui/breeze-icon.ts";
import "../../../components/ui/card.ts";

interface DashboardStats {
  assigned_count: number;
  overdue_count: number;
  due_this_week_count: number;
  completed_count: number;
  total_projects: number;
}

function getSTAT_ITEMS(): {
  key: keyof DashboardStats;
  label: string;
  icon: string;
  color: string;
}[] {
  return [
    {
      key: "assigned_count",
      label: msg("Assigned"),
      icon: "list-todo",
      color: "var(--color-blue-400, #60a5fa)",
    },
    {
      key: "overdue_count",
      label: msg("Overdue"),
      icon: "alert-circle",
      color: "var(--color-red-400, #f87171)",
    },
    {
      key: "due_this_week_count",
      label: msg("Due this week"),
      icon: "clock",
      color: "var(--color-amber-400, #fbbf24)",
    },
    {
      key: "completed_count",
      label: msg("Completed"),
      icon: "check-circle",
      color: "var(--color-emerald-400, #34d399)",
    },
    {
      key: "total_projects",
      label: msg("Projects"),
      icon: "folder-kanban",
      color: "var(--color-violet-400, #a78bfa)",
    },
  ];
}

@localized()
@customElement("breeze-stats-section")
export class BreezeStatsSection extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr));
      gap: var(--space-3);
    }
    .stat {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--space-1);
      padding: var(--space-3);
      border-radius: var(--radius-lg);
    }
    .stat-value {
      font-size: var(--text-lg);
      font-weight: 600;
      font-variant-numeric: tabular-nums;
    }
    .stat-label {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
  `;

  @property({ attribute: false })
  data: unknown = null;

  protected render() {
    const stats = (this.data as DashboardStats | null) ?? {
      assigned_count: 0,
      overdue_count: 0,
      due_this_week_count: 0,
      completed_count: 0,
      total_projects: 0,
    };

    return html`
      <breeze-card style="border-style:dashed">
        <div class="grid">
          ${getSTAT_ITEMS().map(
            ({ key, label, icon, color }) =>
              html`
                <div class="stat">
                  <breeze-icon
                    name="${icon}"
                    size="20"
                    style="color:${color}"
                  ></breeze-icon>
                  <span class="stat-value">${stats[
                    key as keyof DashboardStats
                  ]}</span>
                  <span class="stat-label">${label}</span>
                </div>
              `,
          )}
        </div>
      </breeze-card>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-stats-section": BreezeStatsSection;
  }
}
