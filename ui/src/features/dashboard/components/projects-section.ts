import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { navigate } from "@/routes/router";
import "../../../components/ui/plume-icon.ts";
import "../../../components/ui/button.ts";
import "../../../components/ui/card.ts";
import { localized, msg } from "@lit/localize";

interface Project {
  id: string;
  name: string;
  slug: string;
  color: string;
  icon: string;
  task_count: number;
  member_count: number;
}

@localized()
@customElement("plume-projects-section")
export class PlumeProjectsSection extends LitElement {
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
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr));
      gap: var(--space-2);
      padding-top: var(--space-2);
    }
    .project-card {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      padding: var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background: transparent;
      color: inherit;
      font-family: inherit;
      cursor: pointer;
      text-align: left;
      width: 100%;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .project-card:hover {
      background: var(--accent);
    }
    .project-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--space-8);
      height: var(--space-8);
      border-radius: var(--radius-lg);
      font-size: var(--text-sm);
      font-weight: 600;
      color: white;
      flex-shrink: 0;
    }
    .project-info {
      flex: 1;
      min-width: 0;
    }
    .project-name {
      font-size: var(--text-sm);
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .project-meta {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      margin-top: var(--space-0-5);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .project-meta plume-icon {
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
    const projects = (this.data as Project[] | null) ?? [];
    if (projects.length === 0) {
      return html`
        <plume-card>
          <div class="header">
            <span class="title">${msg("Projects")}</span>
          </div>
          <div class="empty">${msg("No projects yet.")}</div>
        </plume-card>
      `;
    }

    return html`
      <plume-card>
        <div class="header">
          <span class="title">${msg("Projects")}</span>
          <button
            class="view-all"
            @click="${() => navigate("/projects")}"
          >
            ${msg("All projects")}
            <plume-icon name="arrow-right" size="12"></plume-icon>
          </button>
        </div>
        <div class="grid">
          ${projects.map(
            (p) =>
              html`
                <button
                  class="project-card"
                  @click="${() => navigate(`/projects/${p.slug}`)}"
                >
                  <div
                    class="project-icon"
                    style="background:${p.color}"
                  >
                    ${p.icon ? p.icon.charAt(0).toUpperCase() : html`
                      <plume-icon name="folder" size="16" style="color:white"></plume-icon>
                    `}
                  </div>
                  <div class="project-info">
                    <div class="project-name">${p.name}</div>
                    <div class="project-meta">
                      <span style="display:flex;align-items:center;gap:var(--space-0-5)">
                        <plume-icon name="folder" size="12"></plume-icon>
                        ${p.task_count} tasks
                      </span>
                      <span style="display:flex;align-items:center;gap:var(--space-0-5)">
                        <plume-icon name="users" size="12"></plume-icon>
                        ${p.member_count}
                      </span>
                    </div>
                  </div>
                </button>
              `,
          )}
        </div>
      </plume-card>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-projects-section": PlumeProjectsSection;
  }
}
