import { css, html, LitElement, nothing } from "lit";
import { customElement, property } from "lit/decorators.js";
import { contentEnterStyles } from "@/styles/shared-animations";
import { fmtDate, fmtDateYear } from "@/lib/format/date";
import type { DtoCycleResponse } from "@/api";
import "../../components/ui/breeze-icon.ts";
import { localized, msg } from "@lit/localize";

/**
 * Cycles View: full-page cycles overview shown via the "Cycles" tab
 * on the project detail page. Displays active, upcoming, and completed
 * cycles with progress bars.
 *
 * Properties: `cycles`.
 */
@localized()
@customElement("breeze-cycles-view")
export class BreezeCyclesView extends LitElement {
  static styles = [
    contentEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: flex;
        flex-direction: column;
        gap: var(--space-8);
        width: 100%;
      }
      .section-title {
        font-size: var(--text-sm);
        font-weight: 600;
        color: var(--foreground);
        margin: 0 0 var(--space-3);
      }
      .active-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
      }
      .grid-list {
        display: grid;
        gap: var(--space-3);
        grid-template-columns: 1fr;
      }
      @media (min-width: 640px) {
        .grid-list {
          grid-template-columns: repeat(2, 1fr);
        }
      }
      @media (min-width: 1024px) {
        .grid-list {
          grid-template-columns: repeat(3, 1fr);
        }
      }

      /* Card */
      .card {
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        padding: var(--space-4);
      }
      .card-title-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        margin-bottom: var(--space-2);
      }
      .card-icon {
        flex-shrink: 0;
        color: var(--muted-foreground);
      }
      .card-icon.active {
        color: oklch(0.55 0.15 145);
      }
      .card-name {
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .badge-active {
        font-size: var(--text-2xs);
        font-weight: 500;
        color: oklch(0.5 0.18 145);
        background: color-mix(in oklch, oklch(0.55 0.15 145) 15%, transparent);
        padding: 0 var(--space-1-5);
        border-radius: var(--radius-sm);
        line-height: 1.4;
      }
      .card-dates {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .card-goal {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        margin-top: var(--space-1);
        display: -webkit-box;
        -webkit-line-clamp: 2;
        -webkit-box-orient: vertical;
        overflow: hidden;
      }
      .card-progress {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        margin-top: var(--space-3);
      }
      .progress-track {
        flex: 1;
        height: var(--space-1-5);
        background: var(--secondary);
        border-radius: var(--radius-full);
        overflow: hidden;
      }
      .progress-bar {
        height: 100%;
        border-radius: var(--radius-full);
        background: var(--primary);
        transition: width var(--dur-normal) var(--ease-1);
      }
      .progress-count {
        font-size: var(--text-2xs);
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .card-footnote {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        margin-top: var(--space-2);
      }

      /* Empty state */
      .empty {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-12) 0;
      }
      .empty-icon {
        color: color-mix(in oklch, var(--muted-foreground) 40%, transparent);
      }
      .empty-text {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        margin: 0;
      }
    `,
  ];

  @property({ type: Array, attribute: false })
  cycles: DtoCycleResponse[] = [];

  get #active(): DtoCycleResponse[] {
    return this.cycles.filter((c) => c.is_active);
  }

  get #upcoming(): DtoCycleResponse[] {
    return this.cycles.filter((c) => !c.is_active && !c.is_completed);
  }

  get #completed(): DtoCycleResponse[] {
    return this.cycles.filter((c) => c.is_completed);
  }

  #progress(c: DtoCycleResponse): number {
    if (!c.task_count || c.task_count <= 0) return 0;
    return Math.round(((c.completed_task_count ?? 0) / c.task_count) * 100);
  }

  #renderCard(c: DtoCycleResponse, compact: boolean) {
    const progress = this.#progress(c);
    const isActive = c.is_active ?? false;
    const starts = c.starts_at ? new Date(c.starts_at) : null;
    const ends = c.ends_at ? new Date(c.ends_at) : null;

    return html`
      <div class="card">
        <div class="card-title-row">
          ${isActive
            ? html`
              <breeze-icon
                class="card-icon active"
                name="circle-check"
                size="16"
              ></breeze-icon>
            `
            : html`
              <breeze-icon
                class="card-icon"
                name="circle"
                size="16"
              ></breeze-icon>
            `}
          <span class="card-name">${c.name}</span>
          ${isActive
            ? html`
              <span class="badge-active">${msg("Active")}</span>
            `
            : nothing}
        </div>
        ${starts && ends
          ? html`
            <span class="card-dates">${fmtDate(starts)}: ${fmtDateYear(
              ends,
            )}</span>
          `
          : nothing} ${c.goal
          ? html`
            <p class="card-goal">${c.goal}</p>
          `
          : nothing}
        <div class="card-progress">
          <div class="progress-track">
            <div class="progress-bar" style="width:${progress}%"></div>
          </div>
          <span class="progress-count">
            ${c.completed_task_count ?? 0}/${c.task_count ?? 0}
          </span>
        </div>
        ${!compact
          ? html`
            <div class="card-footnote">${progress}% complete</div>
          `
          : nothing}
      </div>
    `;
  }

  protected render() {
    const active = this.#active;
    const upcoming = this.#upcoming;
    const completed = this.#completed;

    if (this.cycles.length === 0) {
      return html`
        <div class="empty">
          <breeze-icon class="empty-icon" name="circle" size="32"></breeze-icon>
          <p class="empty-text">${msg("No cycles yet")}</p>
        </div>
      `;
    }

    return html`
      <div class="content-enter">
        ${active.length > 0
          ? html`
            <section>
              <h2 class="section-title">${msg("Active")}</h2>
              <div class="active-list">
                ${active.map((c) => this.#renderCard(c, false))}
              </div>
            </section>
          `
          : nothing} ${upcoming.length > 0
          ? html`
            <section>
              <h2 class="section-title">${msg("Upcoming")}</h2>
              <div class="grid-list">
                ${upcoming.map((c) => this.#renderCard(c, true))}
              </div>
            </section>
          `
          : nothing} ${completed.length > 0
          ? html`
            <section>
              <h2 class="section-title">${msg("Completed")}</h2>
              <div class="grid-list">
                ${completed.map((c) => this.#renderCard(c, true))}
              </div>
            </section>
          `
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-cycles-view": BreezeCyclesView;
  }
}
