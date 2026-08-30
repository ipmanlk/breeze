import { css, html, LitElement, nothing } from "lit";
import { customElement, property } from "lit/decorators.js";
import type { DtoCycleResponse } from "@/api";
import "../../components/ui/select.ts";
import "../../components/ui/breeze-icon.ts";
import { localized, msg } from "@lit/localize";

/**
 * Cycle Bar: active cycle indicator bar with progress and cycle filter select.
 *
 * Appears below the filter bar on the project detail page when cycles are
 * enabled. Shows the active cycle name, completion progress bar + counts,
 * and a select dropdown to filter tasks by cycle.
 *
 * Properties: `cycles`, `activeCycleId`.
 * Events: `cycle-change`: detail = cycle id (string) or null (for all tasks).
 */
@localized()
@customElement("breeze-cycle-bar")
export class BreezeCycleBar extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-shrink: 0;
      align-items: center;
      gap: var(--space-3);
      padding: var(--space-2) var(--space-6);
      border-bottom: 1px solid var(--border);
    }
    .cycle-info {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      font-size: var(--text-sm);
    }
    .cycle-name {
      font-weight: 500;
      color: var(--foreground);
      white-space: nowrap;
    }
    .cycle-count {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      white-space: nowrap;
    }
    .progress-track {
      height: var(--space-1-5);
      flex: 1;
      max-width: var(--space-40);
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
    .filter-wrap {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      margin-left: auto;
    }
    .filter-select {
      width: var(--cycle-w);
    }
  `;

  /** All cycles for the current project. */
  @property({ type: Array, attribute: false })
  cycles: DtoCycleResponse[] = [];

  /**
   * Currently selected cycle filter value:
   * - `null` → show all tasks
   * - `"__backlog__"` → show tasks with no cycle
   * - any other string → show tasks belonging to that cycle id
   */
  @property()
  activeCycleId: string | null = null;

  get #activeCycle(): DtoCycleResponse | undefined {
    return this.cycles.find((c) => c.is_active);
  }

  get #progress(): number {
    const ac = this.#activeCycle;
    if (!ac || !ac.task_count || ac.task_count <= 0) return 0;
    return Math.round(((ac.completed_task_count ?? 0) / ac.task_count) * 100);
  }

  get #selectValue(): string {
    return this.activeCycleId ?? "__all__";
  }

  get #selectOptions(): { value: string; label: string }[] {
    return [
      { value: "__all__", label: msg("All tasks") },
      { value: "__backlog__", label: msg("Backlog (no cycle)") },
      ...this.cycles.map((c) => ({
        value: c.id ?? "",
        label: `${c.name ?? "Unnamed"}${c.is_active ? " (active)" : ""}`,
      })),
    ];
  }

  #onCycleChange(e: CustomEvent): void {
    const val = e.detail as string;
    this.dispatchEvent(
      new CustomEvent("cycle-change", {
        detail: val === "__all__" ? null : val,
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    const ac = this.#activeCycle;
    const progress = this.#progress;
    const options = this.#selectOptions;

    return html`
      <div class="cycle-info">
        <span class="cycle-name">${ac?.name ?? "No active cycle"}</span>
        ${ac
          ? html`
            <span class="cycle-count">
              ${ac.completed_task_count ?? 0}/${ac.task_count ??
                0} · ${progress}%
            </span>
          `
          : nothing}
      </div>
      ${ac
        ? html`
          <div class="progress-track">
            <div class="progress-bar" style="width:${progress}%"></div>
          </div>
        `
        : nothing}
      <div class="filter-wrap">
        <breeze-select
          class="filter-select"
          .options="${options}"
          .value="${this.#selectValue}"
          @change="${this.#onCycleChange}"
        ></breeze-select>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-cycle-bar": BreezeCycleBar;
  }
}
