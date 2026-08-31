import { localized, msg } from "@lit/localize";
import { logError } from "@/lib/log";
import { css, html, LitElement } from "lit";
import { pageEnterStyles } from "@/styles/shared-animations";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { currentPath, navigate } from "@/routes/router";
import {
  deleteView,
  fetchGlobalViews,
  fetchPinnedViews,
  pinView,
  unpinView,
  views,
} from "./store";
import { activeFilterEntries, humanizeValue } from "./types";
import type { View } from "./types";
import { getProjects } from "@/api";
import "../../layouts/app-layout.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/card.ts";
import "../../components/ui/popover.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/skeleton.ts";
import "./components/save-view-dialog.ts";

/**
 * Views Page: saved views across workspace.
 */
@localized()
@customElement("plume-views-page")
export class PlumeViewsPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }
      .page {
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
      section {
        margin-bottom: var(--space-6);
      }
      section:last-child {
        margin-bottom: 0;
      }
      .section-title {
        font-size: var(--text-xs);
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--muted-foreground);
        margin-bottom: var(--space-3);
      }
      .grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
        gap: var(--space-3);
      }
      .grid > .view-card {
        animation: content-in var(--dur-normal) var(--ease-2) both;
      }
      .grid > .view-card:nth-child(1) {
        animation-delay: calc(0 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(2) {
        animation-delay: calc(4 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(3) {
        animation-delay: calc(8 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(4) {
        animation-delay: calc(12 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(5) {
        animation-delay: calc(16 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(6) {
        animation-delay: calc(20 * var(--dur-instant));
      }
      .grid > .view-card:nth-child(n+7) {
        animation-delay: calc(24 * var(--dur-instant));
      }
      .view-card {
        position: relative;
        display: block;
        padding: var(--space-4);
        border-radius: var(--radius-lg);
        border: 1px solid var(--border);
        background: var(--card);
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .view-card:active {
        transform: scale(0.98);
        transition: var(--tr-transform);
      }
      .view-card:hover {
        background: var(--accent);
      }
      .view-header {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: var(--space-2);
      }
      .view-actions {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        flex-shrink: 0;
      }
      .view-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .view-title plume-icon {
        color: var(--muted-foreground);
      }
      .pin-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--space-6);
        height: var(--space-6);
        border: none;
        border-radius: var(--radius-md);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          background var(--dur-fast) var(--ease-1),
          color var(--dur-fast) var(--ease-1);
      }
      .view-card:hover .pin-btn,
      .pin-btn.pinned {
        opacity: 1;
      }
      .pin-btn:active {
        transform: scale(0.9);
        transition: var(--tr-transform);
      }
      .pin-btn:hover {
        background: var(--muted);
        color: var(--foreground);
      }
      .pin-btn.pinned {
        color: var(--primary);
      }
      .pin-btn.pinned:hover {
        color: var(--foreground);
      }
      .kbd-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--space-6);
        height: var(--space-6);
        border: none;
        border-radius: var(--radius-md);
        background: transparent;
        color: var(--muted-foreground);
        cursor: pointer;
        opacity: 0;
        transition:
          opacity var(--dur-fast) var(--ease-1),
          background var(--dur-fast) var(--ease-1),
          color var(--dur-fast) var(--ease-1);
      }
      .view-card:hover .kbd-btn,
      .view-card:hover .pin-btn {
        opacity: 1;
      }
      .kbd-btn:hover {
        background: var(--muted);
        color: var(--foreground);
      }
      .kbd-menu {
        min-width: var(--space-36);
        padding: var(--space-1);
      }
      .kbd-item {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        width: 100%;
        padding: var(--space-1-5) var(--space-2);
        border: none;
        border-radius: var(--radius-sm);
        background: transparent;
        color: var(--foreground);
        font-size: var(--text-sm);
        font-family: inherit;
        text-align: left;
        cursor: pointer;
      }
      .kbd-item:hover {
        background: var(--accent);
      }
      .kbd-item.destructive:hover {
        background: color-mix(in oklch, var(--destructive) 15%, transparent);
        color: var(--destructive);
      }
      .filters {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-1);
        margin-top: var(--space-2);
      }
      .filter-tag {
        display: inline-flex;
        align-items: center;
        padding: var(--space-0-5) var(--space-1-5);
        border-radius: var(--radius-md);
        background: var(--muted);
        color: var(--muted-foreground);
        font-size: var(--text-xs);
      }
      .project-label {
        display: inline-flex;
        margin-top: var(--space-2);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .empty {
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      .loading {
        display: flex;
        justify-content: center;
        padding: var(--space-8);
      }
    `,
  ];

  @state()
  private _projectViews: Map<string, View[]> = new Map();

  @state()
  private _editingView: View | null = null;

  @state()
  private _showSaveDialog = false;

  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(views, currentPath);
    this._loadData();
  }

  private async _loadData(): Promise<void> {
    await Promise.all([
      fetchGlobalViews(),
      fetchPinnedViews(),
      this._fetchProjects(),
    ]);
  }

  private async _fetchProjects(): Promise<void> {
    try {
      const { data } = await getProjects({ throwOnError: true });
      const projects = data ?? [];
      await this._fetchProjectViews(projects.map((p) => p.id ?? ""));
    } catch (err) {
      logError("fetchProjects failed:", err);
    }
  }

  private async _fetchProjectViews(projectIds: string[]): Promise<void> {
    const { fetchProjectViews } = await import("./store");
    const viewsMap = new Map<string, View[]>();
    for (const id of projectIds) {
      const projectViews = await fetchProjectViews(id);
      if (projectViews.length > 0) {
        viewsMap.set(id, projectViews);
      }
    }
    this._projectViews = viewsMap;
  }

  private _navigateToView(view: View): void {
    if (view.project_slug) {
      navigate(`/projects/${view.project_slug}?view=${view.id}`);
    } else {
      navigate(`/views/${view.id}`);
    }
  }

  private async _togglePin(e: Event, view: View): Promise<void> {
    e.stopPropagation();
    const isPinned = views.value.pinnedViews.some((v) => v.id === view.id);
    if (isPinned) {
      await unpinView(view.id);
    } else {
      await pinView(view.id);
    }
  }

  private _renderViewCard(view: View): ReturnType<typeof html> {
    const isPinned = views.value.pinnedViews.some((v) => v.id === view.id);
    const filterEntries = activeFilterEntries(view.filters);

    return html`
      <div class="view-card" @click="${() => this._navigateToView(view)}">
        <div class="view-header">
          <div class="view-title">
            <plume-icon
              name="${view.layout === "board" ? "layout-grid" : "list"}"
              size="16"
            ></plume-icon>
            <span>${view.name}</span>
          </div>
          <div class="view-actions">
            <plume-popover>
              <button
                slot="trigger"
                class="kbd-btn"
                title="${msg("Actions")}"
              >
                <plume-icon name="more-horizontal" size="14"></plume-icon>
              </button>
              <div slot="content" class="kbd-menu">
                <button
                  class="kbd-item"
                  @click="${(e: Event) => {
                    e.stopPropagation();
                    this._editingView = view;
                    this._showSaveDialog = true;
                  }}"
                >
                  <plume-icon name="pencil" size="14"></plume-icon>
                  ${msg("Edit")}
                </button>
                <button
                  class="kbd-item destructive"
                  @click="${(e: Event) => {
                    e.stopPropagation();
                    this._deleteView(view);
                  }}"
                >
                  <plume-icon name="trash-2" size="14"></plume-icon>
                  ${msg("Delete")}
                </button>
              </div>
            </plume-popover>
            <button
              class="pin-btn ${isPinned ? "pinned" : ""}"
              @click="${(e: Event) => this._togglePin(e, view)}"
              title="${isPinned ? msg("Unpin") : msg("Pin")}"
            >
              <plume-icon
                name="${isPinned ? "star" : "pin"}"
                size="14"
              ></plume-icon>
            </button>
          </div>
        </div>

        ${filterEntries.length > 0
          ? html`
            <div class="filters">
              ${filterEntries.map(
                ([k, v]) =>
                  html`
                    <span class="filter-tag">${humanizeValue(k, v)}</span>
                  `,
              )}
            </div>
          `
          : null} ${view.project_name
          ? html`
            <span class="project-label">${view.project_name}</span>
          `
          : null}
      </div>
    `;
  }

  private async _deleteView(view: View): Promise<void> {
    const ok = await deleteView(view.id);
    if (ok) {
      // Remove from project views if present
      const pid = view.project_id;
      if (pid && this._projectViews.has(pid)) {
        const list = this._projectViews.get(pid)!.filter((v) =>
          v.id !== view.id
        );
        if (list.length > 0) {
          this._projectViews.set(pid, list);
        } else {
          this._projectViews.delete(pid);
        }
        this.requestUpdate();
      }
      // Re-fetch lists to stay in sync
      void fetchGlobalViews();
      void fetchPinnedViews();
    }
  }

  protected render() {
    const { globalViews, pinnedViews, isLoading } = views.value;
    const pinnedIds = new Set(pinnedViews.map((v) => v.id));

    const visibleGlobalViews = globalViews.filter((v) => !pinnedIds.has(v.id));
    const visibleProjectViews = Array.from(this._projectViews.entries())
      .map(([_, views]) => views.filter((v) => !pinnedIds.has(v.id)))
      .filter((views) => views.length > 0);

    if (isLoading) {
      return html`
        <plume-app-layout>
          <div class="loading">
            <plume-skeleton
              variant="card"
              count="3"
              width="100%"
              height="4rem"
            ></plume-skeleton>
          </div>
        </plume-app-layout>
      `;
    }

    return html`
      <plume-app-layout>
        <div class="page page-enter">
          <div class="page-head">
            <div>
              <h1>${msg("Views")}</h1>
              <p>${msg("Saved views across your workspace.")}</p>
            </div>
          </div>

          <div class="page-content">
            ${pinnedViews.length > 0
              ? html`
                <section>
                  <div class="section-title">${msg("Pinned")}</div>
                  <div class="grid">
                    ${pinnedViews.map((v) => this._renderViewCard(v))}
                  </div>
                </section>
              `
              : null}

            <section>
              <div class="section-title">${msg("Global Views")}</div>
              ${visibleGlobalViews.length > 0
                ? html`
                  <div class="grid">
                    ${visibleGlobalViews.map((v) => this._renderViewCard(v))}
                  </div>
                `
                : html`
                  <p class="empty">
                    ${visibleProjectViews.length === 0
                      ? msg(
                        "No views yet. Navigate to a project, set filters, then save.",
                      )
                      : msg("No global views.")}
                  </p>
                `}
            </section>

            ${visibleProjectViews.map((views) => {
              const projectName = views[0]?.project_name ?? msg("Project");
              return html`
                <section>
                  <div class="section-title">${projectName}</div>
                  <div class="grid">
                    ${views.map((v) => this._renderViewCard(v))}
                  </div>
                </section>
              `;
            })}
          </div>
        </div>
      </plume-app-layout>

      <plume-save-view-dialog
        .open="${this._showSaveDialog}"
        .viewId="${this._editingView?.id ?? ""}"
        .viewName="${this._editingView?.name ?? ""}"
        .viewLayout="${this._editingView?.layout ?? "board"}"
        .existingFilters="${this._editingView?.filters ?? {}}"
        @close="${() => {
          this._showSaveDialog = false;
          this._editingView = null;
        }}"
        @view-updated="${() => {
          this._showSaveDialog = false;
          this._editingView = null;
          void fetchGlobalViews();
          void fetchPinnedViews();
        }}"
      ></plume-save-view-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-views-page": PlumeViewsPage;
  }
}
