import { html, LitElement, nothing, type PropertyValues } from "lit";
import { customElement, state } from "lit/decorators.js";
import { currentPath, matchRoute, navigate } from "@/routes/router";
import {
  applyWsTaskEvent,
  fetchProjectDetail,
  fetchTasks,
  hasProjectPermission,
  projectDetail,
  removeWsTask,
  selectTask,
  tasksByStatus,
} from "@/store/project-detail";
import { ProjectPermission } from "@/lib/permissions";
import { cycles, fetchCycles } from "./cycles-store";
import type { DtoProjectResponse, DtoTaskResponse } from "@/api";
import { getProjectsById } from "@/api";
import { SignalController } from "@/lib/signal-controller";
import { sendWsMessage, wsClient } from "@/store/ws";
import { fetchProjectViews, updateView } from "@/features/views/store";
import type { View as SavedView, ViewFilters } from "@/features/views/types";
import "../../layouts/app-layout.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/button.ts";
import "../../components/ui/spinner.ts";
import "./kanban-board.ts";
import "./task-dialog.ts";
import "./task-detail-dialog.ts";
import "./filter-bar.ts";
import "./list-view.ts";
import "./settings-view.ts";
import "./project-members-view.ts";
import type { ProjectFilters } from "./filter-bar";
import "./cycle-bar.ts";
import "./cycles-view.ts";
import "../views/components/save-view-dialog.ts";
import "../../components/ui/button-group.ts";
import { localized, msg } from "@lit/localize";

/**
 * Light DOM: required for @atlaskit/pragmatic-drag-and-drop.
 * The kanban board (a child of this component) uses native drag events that
 * rely on `event.target` not being retargeted by shadow DOM.
 *
 * `breeze-app-layout` (used below) keeps its shadow DOM: that's fine because
 * it uses `<slot>` to project this component's content, and slotted content
 * stays in the light DOM (document tree) where `event.target` is not retargeted.
 *
 * Styles are injected via a `<style>` tag. Since light DOM styles are global,
 * all class names are prefixed with `pdp-` to avoid collisions.
 */

/** UUID v4 pattern: for resolving /projects/:id links (e.g. from mention chips). */
const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Build a ViewFilters from current URL params. */
function urlToViewFilters(url: URL): ViewFilters {
  const f: ViewFilters = {};
  const { searchParams } = url;
  if (searchParams.get("search")) f.search = searchParams.get("search")!;
  if (searchParams.get("priority")) f.priority = searchParams.get("priority")!;
  if (searchParams.get("status_id")) {
    f.status_id = searchParams.get("status_id")!;
  }
  if (searchParams.get("assignee_id")) {
    f.assignee_id = searchParams.get("assignee_id")!;
  }
  if (searchParams.get("cycle_id")) f.cycle_id = searchParams.get("cycle_id")!;
  const labelIds = searchParams.get("label_ids");
  if (labelIds) {
    f.label_ids = labelIds.split(",").filter(Boolean);
  }
  return f;
}

/** Compare two ViewFilters for equality. */
function filtersEqual(a: ViewFilters, b: ViewFilters): boolean {
  const aLabels = (a.label_ids ?? []).slice().sort().join(",");
  const bLabels = (b.label_ids ?? []).slice().sort().join(",");
  return (
    (a.search ?? "") === (b.search ?? "") &&
    (a.priority ?? "") === (b.priority ?? "") &&
    (a.status_id ?? "") === (b.status_id ?? "") &&
    (a.assignee_id ?? "") === (b.assignee_id ?? "") &&
    (a.cycle_id ?? "") === (b.cycle_id ?? "") &&
    aLabels === bLabels
  );
}

type Layout = "board" | "list" | "settings" | "cycles" | "members";

const PDP_STYLES = `
breeze-project-detail-page {
  display: contents;
}
.pdp-page {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.pdp-header {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  border-bottom: 1px solid var(--border);
  padding: var(--space-3) var(--space-6);
  /* Cancel .content padding-top so header padding is the only space above */
  margin-top: calc(-1 * var(--space-4));
}
.pdp-header h1 {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: 600;
}
.pdp-header-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.pdp-project-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: var(--space-8);
  height: var(--space-8);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: 600;
  color: white;
  flex-shrink: 0;
}
.pdp-tabs {
  display: flex;
  gap: var(--space-1);
  border-bottom: 1px solid var(--border);
  padding: var(--space-1) var(--space-6) 0;
}
.pdp-sticky-wrap {
  position: sticky;
  top: var(--topbar-h, 3rem);
  z-index: var(--z-sticky);
  background: var(--background);
}
.pdp-tab {
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-sm);
  font-weight: 500;
  color: var(--muted-foreground);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-family: inherit;
  position: relative;
  transition:
    color var(--dur-fast) var(--ease-1),
    border-color var(--dur-fast) var(--ease-1);
}
.pdp-tab:active {
  transform: scale(0.95);
  transition: var(--tr-transform);
}
.pdp-unsaved-dot {
  position: absolute;
  top: var(--space-1);
  right: var(--space-1);
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
}
.pdp-tab:hover {
  color: var(--foreground);
}
.pdp-tab.active {
  color: var(--foreground);
  border-bottom-color: var(--primary);
}
.pdp-filter-bar {
  padding: var(--space-2) var(--space-6);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.pdp-subtask-toggle {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  flex-shrink: 0;
  height: var(--space-7);
  padding: 0 var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--background);
  color: var(--muted-foreground);
  font-size: var(--text-xs);
  cursor: pointer;
  transition: all var(--dur-fast) var(--ease-1);
}
.pdp-subtask-toggle:hover {
  background: var(--accent);
  color: var(--accent-foreground);
}
.pdp-subtask-toggle.active {
  background: var(--accent);
  color: var(--accent-foreground);
  border-color: var(--accent);
}
.pdp-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: var(--space-6);
  min-height: 0;
}
.pdp-empty {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  flex-direction: column;
  gap: var(--space-4);
}
.pdp-empty p {
  font-size: var(--text-sm);
  color: var(--muted-foreground);
}

.pdp-list-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-sm);
}
.pdp-list-table th {
  text-align: left;
  padding: var(--space-2) var(--space-4);
  font-weight: 500;
  color: var(--muted-foreground);
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  white-space: nowrap;
}
.pdp-list-table th:hover {
  color: var(--foreground);
}
.pdp-list-table td {
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--border);
}
.pdp-list-table tr {
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease-1);
}
.pdp-list-table tbody tr:active {
  transform: scale(0.99);
  transition: var(--tr-transform);
}
.pdp-list-table tbody tr:hover {
  background: var(--accent);
}
.pdp-list-table tbody tr:last-child td {
  border-bottom: none;
}
.pdp-task-title {
  font-weight: 500;
  max-width: var(--space-24);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pdp-status-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-xs);
  padding: var(--space-0-5) var(--space-2);
  border-radius: var(--radius-full);
  border: 1px solid var(--border);
}
.pdp-status-dot {
  width: var(--space-1-5);
  height: var(--space-1-5);
  border-radius: var(--radius-full);
  flex-shrink: 0;
}
.pdp-priority-badge {
  font-size: var(--text-xs);
  color: var(--muted-foreground);
}
.pdp-due-date {
  font-size: var(--text-xs);
  color: var(--muted-foreground);
}
.pdp-due-date.overdue {
  color: var(--destructive);
  font-weight: 500;
}
`;

@localized()
@customElement("breeze-project-detail-page")
export class BreezeProjectDetailPage extends LitElement {
  /** Light DOM: required for @atlaskit/pragmatic-drag-and-drop. */
  createRenderRoot() {
    return this;
  }

  @state()
  private _layout: Layout = "board";

  /** When true the board fetches subtasks too and renders them indented
   * under their parent with a ↳ parent reference. Off by default so the
   * board stays clean (top-level tasks only). */
  @state()
  private _showSubtasks = false;

  @state()
  private _showCreate = false;

  @state()
  private _sortField: "title" | "status" | "priority" | "due_at" = "title";

  @state()
  private _sortDir: "asc" | "desc" = "asc";

  @state()
  private _filters: ProjectFilters = {
    search: "",
    priority: "",
    status_id: "",
    assignee_id: null,
    label_ids: [],
  };

  @state()
  private _cycleFilter: string | null = null;

  @state()
  private _projectViews: SavedView[] = [];

  @state()
  private _viewId = "";

  @state()
  private _showSaveViewDialog = false;

  #signals = new SignalController(this);
  #slug = "";
  #resolving = false;
  #loadedProjectId: string | null = null;
  #wsProjectId: string | null = null;
  #wsPrev: WebSocket | null = null;
  #wsMessageHandler: ((e: MessageEvent) => void) | null = null;

  // Bound handler for the global "create-task" keyboard shortcut ("c").
  // Stored so it can be removed on disconnect.
  #createTaskHandler: (() => void) | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(currentPath, projectDetail, cycles, wsClient);
    this.#loadProject();
    this.#hydrateTaskFromUrl();
    this.#loadViewIdFromUrl();
    this.#createTaskHandler = () => {
      // Only open the create dialog when actually viewing a project board/
      // list (not on settings/members tabs where creating a task
      // is not meaningful).
      if (this._layout === "settings" || this._layout === "members") return;
      this._showCreate = true;
    };
    document.addEventListener("create-task", this.#createTaskHandler);
  }

  willUpdate() {
    this.#loadProject();
    this.#loadViewIdFromUrl();
    const url = new URL(window.location.href);

    // Read URL params
    const layoutParam = url.searchParams.get("layout") as Layout | null;
    if (
      layoutParam &&
      ["board", "list", "settings", "cycles", "members"].includes(layoutParam)
    ) {
      this._layout = layoutParam;
    }
    const sortParam = url.searchParams.get("sort") as
      | "title"
      | "status"
      | "priority"
      | "due_at"
      | null;
    if (sortParam) this._sortField = sortParam;
    const dirParam = url.searchParams.get("dir") as "asc" | "desc" | null;
    if (dirParam) this._sortDir = dirParam;

    const cycleParam = url.searchParams.get("cycle_id");
    if (cycleParam !== null) this._cycleFilter = cycleParam;

    this._showSubtasks = url.searchParams.get("show_subtasks") === "1";

    // Sync filter state from URL params (so loading a saved view applies its filters)
    this._filters = {
      search: url.searchParams.get("search") ?? "",
      priority: url.searchParams.get("priority") ?? "",
      status_id: url.searchParams.get("status_id") ?? "",
      assignee_id: url.searchParams.get("assignee_id") || null,
      label_ids: url.searchParams.get("label_ids")
        ? url.searchParams.get("label_ids")!.split(",").filter(Boolean)
        : [],
    };

    // Fetch cycles if project has cycles enabled
    const proj = projectDetail.value.project;
    if (proj?.id && (proj.cycle_duration ?? 0) > 0 && proj.id !== this.#slug) {
      void fetchCycles(proj.id);
    }

    // Fetch project views when project is loaded
    if (proj?.id && proj.id !== this.#loadedProjectId) {
      this.#loadedProjectId = proj.id;
      void this._fetchProjectViews(proj.id);
      this.#subscribeProjectRoom(proj.id);
    }

    // If show_subtasks was hydrated from URL but tasks were already loaded
    // (by fetchProjectDetail) without include_subtasks, refetch with the param.
    if (this._showSubtasks && proj?.id) {
      const tasks = projectDetail.value.tasks;
      if (tasks.length > 0 && !tasks.some((t) => t.parent_task_id)) {
        fetchTasks(proj.id, { include_subtasks: "true" });
      }
    }
  }

  #loadProject() {
    const match = matchRoute("/projects/:slug", currentPath.value);
    const slug = match?.["slug"] ?? "";
    if (!slug || slug === this.#slug) return;

    // Resolve UUIDs to slugs (e.g. links from mention chips).
    // Fetch by ID, update the URL, and load directly: single pass, no redirect.
    if (UUID_RE.test(slug)) {
      if (this.#resolving) return;
      this.#resolving = true;
      this.#resolveAndLoad(slug);
      return;
    }

    this.#resolving = false;
    this.#slug = slug;
    fetchProjectDetail(slug);
  }

  async #resolveAndLoad(uuid: string): Promise<void> {
    try {
      const { data } = await getProjectsById({
        path: { id: uuid },
        throwOnError: true,
      });
      const project = data as DtoProjectResponse;
      const slug = project?.slug;
      if (slug) {
        // Set slug BEFORE calling update / fetch so the guard above
        // (slug === this.#slug) prevents a redundant second load.
        this.#slug = slug;
        this.#resolving = false;
        const url = new URL(window.location.href);
        url.pathname = `/projects/${slug}`;
        window.history.replaceState(null, "", url.toString());
        currentPath.value = url.pathname;
        fetchProjectDetail(slug);
        return;
      }
    } catch {
      // Not found by UUID: fall through
    }
    this.#resolving = false;
    this.#slug = uuid;
    fetchProjectDetail(uuid);
  }

  #hydrateTaskFromUrl() {
    const url = new URL(window.location.href);
    const taskId = url.searchParams.get("task");
    if (taskId) {
      selectTask(taskId);
    }
  }

  // WebSocket: project room subscription.
  // Subscribing to the project room lets the open task dialog receive
  // live comment_new / comment_updated / comment_deleted events for the
  // task being viewed (mirrors chat's conversation room subscription).
  #subscribeProjectRoom(projectId: string): void {
    // Unsubscribe from a previous project room if the project changed.
    if (this.#wsProjectId && this.#wsProjectId !== projectId) {
      this.#unsubscribeProjectRoom();
    }
    this.#wsProjectId = projectId;
    if (wsClient.value) {
      sendWsMessage({
        type: "project_subscribe",
        payload: { project_id: projectId },
      });
    }
  }

  #unsubscribeProjectRoom(): void {
    if (this.#wsProjectId && wsClient.value) {
      sendWsMessage({
        type: "project_unsubscribe",
        payload: { project_id: this.#wsProjectId },
      });
    }
    this.#wsProjectId = null;
  }

  protected updated(changedProps: PropertyValues): void {
    // Re-subscribe to the project room after a WebSocket reconnect.
    // Note: wsClient is a signal (not a Lit property), so changedProps
    // does not carry wsClient changes. The internal guard
    // (ws !== this.#wsPrev) below prevents duplicate listeners.
    void changedProps;
    const ws = wsClient.value;
    if (ws && ws !== this.#wsPrev) {
      // Detach the previous handler if any.
      if (this.#wsPrev && this.#wsMessageHandler) {
        this.#wsPrev.removeEventListener("message", this.#wsMessageHandler);
      }
      this.#wsPrev = ws;
      this.#wsMessageHandler = (e) => this.#onWsMessage(e);
      ws.addEventListener("message", this.#wsMessageHandler);
      if (this.#loadedProjectId) {
        sendWsMessage({
          type: "project_subscribe",
          payload: { project_id: this.#loadedProjectId },
        });
      }
    } else if (!ws && this.#wsPrev) {
      if (this.#wsPrev && this.#wsMessageHandler) {
        this.#wsPrev.removeEventListener("message", this.#wsMessageHandler);
      }
      this.#wsMessageHandler = null;
      this.#wsPrev = null;
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#unsubscribeProjectRoom();
    if (this.#wsPrev && this.#wsMessageHandler) {
      this.#wsPrev.removeEventListener("message", this.#wsMessageHandler);
    }
    this.#wsMessageHandler = null;
    if (this.#createTaskHandler) {
      document.removeEventListener("create-task", this.#createTaskHandler);
      this.#createTaskHandler = null;
    }
  }

  // WebSocket: live task updates (project room broadcasts)
  #onWsMessage(e: MessageEvent): void {
    let data: { type?: string; payload?: Record<string, unknown> };
    try {
      data = JSON.parse(e.data);
    } catch {
      return;
    }
    const payload = data.payload ?? {};
    switch (data.type) {
      case "task_created":
      case "task_updated":
      case "task_moved": {
        const task = payload.task as DtoTaskResponse | undefined;
        if (task?.id) {
          applyWsTaskEvent(task);
        }
        break;
      }
      case "task_deleted": {
        const task = payload.task as DtoTaskResponse | undefined;
        if (task?.id) {
          removeWsTask(task.id);
        }
        break;
      }
      case "task_activity_recorded": {
        const p = payload as { task_id?: string } | undefined;
        if (p?.task_id) {
          document.dispatchEvent(
            new CustomEvent("breeze-task-activity-recorded", {
              detail: { taskId: p.task_id },
            }),
          );
        }
        break;
      }
    }
  }

  #onTaskClick(task: DtoTaskResponse) {
    if (!task.id) return;
    selectTask(task.id);
    const url = new URL(window.location.href);
    url.searchParams.set("task", task.id);
    window.history.replaceState(null, "", url.toString());
  }

  #onTaskDetailClose() {
    selectTask(null);
    const url = new URL(window.location.href);
    url.searchParams.delete("task");
    window.history.replaceState(null, "", url.toString());
  }

  #setLayout(layout: Layout) {
    this._layout = layout;
    const url = new URL(window.location.href);
    url.searchParams.set("layout", layout);
    window.history.replaceState(null, "", url.toString());
  }

  #onSortChange(e: CustomEvent<{ field: string; dir: string }>) {
    this._sortField = e.detail.field as
      | "title"
      | "status"
      | "priority"
      | "due_at";
    this._sortDir = e.detail.dir as "asc" | "desc";
    const url = new URL(window.location.href);
    url.searchParams.set("sort", this._sortField);
    url.searchParams.set("dir", this._sortDir);
    window.history.replaceState(null, "", url.toString());
  }

  /** Toggle showing subtasks on the board. Refetches the project task list
   * with include_subtasks=true (or the default top-level-only when off). */
  #toggleShowSubtasks() {
    this._showSubtasks = !this._showSubtasks;
    const projectId = projectDetail.value.project?.id;
    if (!projectId) return;
    const url = new URL(window.location.href);
    if (this._showSubtasks) {
      url.searchParams.set("show_subtasks", "1");
    } else {
      url.searchParams.delete("show_subtasks");
    }
    window.history.replaceState(null, "", url.toString());
    fetchTasks(
      projectId,
      this._showSubtasks ? { include_subtasks: "true" } : undefined,
    );
  }

  #onFiltersChange(e: CustomEvent<{ filters: ProjectFilters }>) {
    this._filters = e.detail.filters;
    const url = new URL(window.location.href);
    if (this._filters.search) {
      url.searchParams.set("search", this._filters.search);
    } else url.searchParams.delete("search");
    if (this._filters.priority) {
      url.searchParams.set("priority", this._filters.priority);
    } else url.searchParams.delete("priority");
    if (this._filters.status_id) {
      url.searchParams.set("status_id", this._filters.status_id);
    } else url.searchParams.delete("status_id");
    if (this._filters.assignee_id) {
      url.searchParams.set("assignee_id", this._filters.assignee_id);
    } else url.searchParams.delete("assignee_id");
    if (this._filters.label_ids.length > 0) {
      url.searchParams.set("label_ids", this._filters.label_ids.join(","));
    } else url.searchParams.delete("label_ids");
    window.history.replaceState(null, "", url.toString());
  }

  #onCycleChange(e: CustomEvent) {
    this._cycleFilter = e.detail;
    const url = new URL(window.location.href);
    if (this._cycleFilter) {
      url.searchParams.set("cycle_id", this._cycleFilter);
    } else {
      url.searchParams.delete("cycle_id");
    }
    window.history.replaceState(null, "", url.toString());
  }

  #getFilteredTasks(tasks: DtoTaskResponse[]): DtoTaskResponse[] {
    return tasks.filter((t) => {
      // Search filter
      if (this._filters.search) {
        const search = this._filters.search.toLowerCase();
        const title = (t.title ?? "").toLowerCase();
        if (!title.includes(search)) {
          return false;
        }
      }

      // Priority filter
      if (this._filters.priority && t.priority !== this._filters.priority) {
        return false;
      }

      // Status filter
      if (this._filters.status_id && t.status_id !== this._filters.status_id) {
        return false;
      }

      // Assignee filter - check if any assignee matches
      if (this._filters.assignee_id !== null) {
        const hasAssignee = t.assignees?.some(
          (a) => a.id === this._filters.assignee_id,
        );
        if (!hasAssignee) {
          return false;
        }
      }

      // Label filter - task must have every selected label
      if (this._filters.label_ids.length > 0) {
        const taskLabelIds = new Set((t.labels ?? []).map((l) => l.id));
        const hasAll = this._filters.label_ids.every((id) =>
          taskLabelIds.has(id)
        );
        if (!hasAll) return false;
      }

      // Cycle filter
      if (this._cycleFilter) {
        if (this._cycleFilter === "__backlog__") {
          if (t.cycle_id) return false;
        } else {
          if (t.cycle_id !== this._cycleFilter) return false;
        }
      }

      return true;
    });
  }

  #loadViewIdFromUrl() {
    const url = new URL(window.location.href);
    this._viewId = url.searchParams.get("view") ?? "";
  }

  async _fetchProjectViews(projectId: string): Promise<void> {
    const views = await fetchProjectViews(projectId);
    this._projectViews = views;
    this.requestUpdate();
  }

  #handleViewTabClick(viewId: string) {
    const view = this._projectViews.find((v) => v.id === viewId);
    const url = new URL(window.location.href);
    if (view) {
      url.searchParams.set("view", view.id);
      url.searchParams.set("layout", view.layout);
      // Apply all saved filters to URL
      const f = view.filters;
      if (f.search) url.searchParams.set("search", f.search);
      else url.searchParams.delete("search");
      if (f.priority) url.searchParams.set("priority", f.priority);
      else url.searchParams.delete("priority");
      if (f.status_id) url.searchParams.set("status_id", f.status_id);
      else url.searchParams.delete("status_id");
      if (f.assignee_id) url.searchParams.set("assignee_id", f.assignee_id);
      else url.searchParams.delete("assignee_id");
      if (f.cycle_id) url.searchParams.set("cycle_id", f.cycle_id);
      else url.searchParams.delete("cycle_id");
    }
    window.history.replaceState(null, "", url.toString());
    this._viewId = viewId;
    if (view) {
      this._layout = (view.layout as Layout) ?? "board";
    }
  }

  #handleResetToSavedView() {
    const activeView = this._projectViews.find((v) => v.id === this._viewId);
    if (!activeView) return;
    const url = new URL(window.location.href);
    // Clear filter params first
    url.searchParams.delete("search");
    url.searchParams.delete("priority");
    url.searchParams.delete("status_id");
    url.searchParams.delete("assignee_id");
    url.searchParams.delete("cycle_id");
    // Set from saved view
    if (activeView.filters.search) {
      url.searchParams.set("search", activeView.filters.search);
    }
    if (activeView.filters.priority) {
      url.searchParams.set("priority", activeView.filters.priority);
    }
    if (activeView.filters.status_id) {
      url.searchParams.set("status_id", activeView.filters.status_id);
    }
    if (activeView.filters.assignee_id) {
      url.searchParams.set("assignee_id", activeView.filters.assignee_id);
    }
    if (activeView.filters.cycle_id) {
      url.searchParams.set("cycle_id", activeView.filters.cycle_id);
    }
    if (activeView.layout) url.searchParams.set("layout", activeView.layout);
    window.history.replaceState(null, "", url.toString());
    // Re-read filters from URL (next willUpdate)
    this._layout = (activeView.layout as Layout) ?? "board";
  }

  async #handleSaveChanges() {
    const activeView = this._projectViews.find((v) => v.id === this._viewId);
    if (!activeView) return;
    const url = new URL(window.location.href);
    const currentFilters = urlToViewFilters(url);
    const newLayout = (url.searchParams.get("layout") as "board" | "list") ??
      activeView.layout;
    await updateView(activeView.id, {
      name: activeView.name,
      layout: newLayout,
      filters: currentFilters,
    });
    // Re-fetch to update UI
    void this._fetchProjectViews(activeView.project_id ?? "");
  }

  #onSaveViewCreated(e: CustomEvent) {
    const view = e.detail as SavedView;
    if (view?.id) {
      this._showSaveViewDialog = false;
      void this._fetchProjectViews(
        this.#loadedProjectId ?? "",
      );
      this.#handleViewTabClick(view.id);
    }
  }

  protected render() {
    const { project, statuses, tasks, isLoading } = projectDetail.value;

    if (isLoading || !project) {
      return html`
        <style>
        ${PDP_STYLES}
        </style>
        <breeze-app-layout>
          <div class="pdp-empty page-enter">
            ${isLoading
              ? html`
                <breeze-spinner></breeze-spinner>
              `
              : html`
                <p>Project not found</p>
                <breeze-button
                  variant="outline"
                  size="sm"
                  @click="${() => navigate("/projects")}"
                >
                  Back to projects
                </breeze-button>
              `}
          </div>
        </breeze-app-layout>
      `;
    }

    const canManage = hasProjectPermission(ProjectPermission.ProjectManage) ||
      hasProjectPermission(ProjectPermission.ProjectStatusManage) ||
      hasProjectPermission(ProjectPermission.ProjectCycleManage);
    const canManageMembers = hasProjectPermission(
      ProjectPermission.ProjectMembersManage,
    );
    const canCreateTasks = hasProjectPermission(ProjectPermission.TaskCreate);
    const hasCycles = (project.cycle_duration ?? 0) > 0;
    const filteredTasks = this.#getFilteredTasks(tasks);
    const grouped = tasksByStatus(filteredTasks);

    // Views integration
    const activeView = this._projectViews.find((v) => v.id === this._viewId) ??
      null;
    const surl = new URL(window.location.href);
    const currentFilters = urlToViewFilters(surl);
    const effectiveLayout: Layout = activeView
      ? (activeView.layout as Layout)
      : this._layout;

    const hasUnsavedFilters = activeView
      ? !filtersEqual(currentFilters, activeView.filters) ||
        this._layout !== activeView.layout
      : !!(
        this._filters.search ||
        this._filters.priority ||
        this._filters.status_id ||
        this._filters.assignee_id ||
        this._cycleFilter
      );

    const tabs: { id: string; label: string; isView?: boolean }[] = [
      { id: "board", label: msg("Board") },
      { id: "list", label: msg("List") },
      ...(hasCycles ? [{ id: "cycles", label: msg("Cycles") }] : []),
      ...(canManageMembers ? [{ id: "members", label: msg("Members") }] : []),
      ...(canManage ? [{ id: "settings", label: msg("Settings") }] : []),
      ...this._projectViews.map((v) => ({
        id: v.id,
        label: v.name,
        isView: true as const,
      })),
    ];

    return html`
      <style>
      ${PDP_STYLES}
      </style>
      <breeze-app-layout>
        <div class="pdp-page page-enter">
          <div class="pdp-header">
            <breeze-button
              variant="ghost"
              size="icon"
              @click="${() => navigate("/projects")}"
            >
              <breeze-icon name="arrow-left" size="16"></breeze-icon>
            </breeze-button>
            <div
              class="pdp-project-icon"
              style="background:${project.color ?? "var(--muted)"}"
            >
              ${(project.icon ?? project.name ?? "?").charAt(0).toUpperCase()}
            </div>
            <h1>${project.name}</h1>
            <div class="pdp-header-actions">
              ${hasUnsavedFilters && activeView
                ? html`
                  <breeze-button-group
                    size="sm"
                    .actions="${[
                      {
                        label: msg("Save as new view"),
                        value: "save-as-new",
                      },
                      {
                        label: msg("Reset to saved"),
                        value: "reset",
                      },
                    ]}"
                    @button-group-main="${this.#handleSaveChanges}"
                    @button-group-action="${(e: CustomEvent) => {
                      if (
                        e.detail?.value === "save-as-new"
                      ) {
                        this._showSaveViewDialog = true;
                      } else if (
                        e.detail?.value === "reset"
                      ) {
                        this.#handleResetToSavedView();
                      }
                    }}"
                  >
                    <breeze-icon name="save" size="14"></breeze-icon>
                    Save changes
                  </breeze-button-group>
                `
                : hasUnsavedFilters && !activeView
                ? html`
                  <breeze-button
                    size="sm"
                    variant="outline"
                    @click="${() => (this._showSaveViewDialog = true)}"
                  >
                    <breeze-icon name="save" size="14"></breeze-icon>
                    Save view
                  </breeze-button>
                `
                : nothing} ${canCreateTasks
                ? html`
                  <breeze-button
                    size="sm"
                    @click="${() => {
                      this._showCreate = true;
                    }}"
                  >
                    <breeze-icon name="plus" size="16"></breeze-icon>
                    New task
                  </breeze-button>
                `
                : nothing}
            </div>
          </div>

          <div class="pdp-sticky-wrap">
            <div class="pdp-tabs">
              ${tabs.map(
                (t) => {
                  const isActive = t.isView
                    ? activeView?.id === t.id
                    : !activeView && effectiveLayout === t.id;
                  return html`
                    <button
                      class="pdp-tab ${isActive ? "active" : ""}"
                      @click="${() => {
                        if (t.isView) {
                          this.#handleViewTabClick(t.id);
                        } else {
                          this._viewId = "";
                          const u = new URL(window.location.href);
                          u.searchParams.delete("view");
                          window.history.replaceState(null, "", u.toString());
                          this.#setLayout(t.id as Layout);
                        }
                      }}"
                    >
                      ${t
                        .label} ${t.isView && activeView?.id === t.id &&
                          hasUnsavedFilters
                        ? html`
                          <span class="pdp-unsaved-dot"></span>
                        `
                        : nothing}
                    </button>
                  `;
                },
              )}
            </div>

            ${(effectiveLayout === "board" || effectiveLayout === "list")
              ? html`
                <div class="pdp-filter-bar">
                  <breeze-filter-bar
                    .projectId="${project.id ?? ""}"
                    .statuses="${statuses}"
                    .filters="${this._filters}"
                    @filters-change="${this.#onFiltersChange}"
                  ></breeze-filter-bar>
                  ${effectiveLayout === "board"
                    ? html`
                      <button
                        class="pdp-subtask-toggle${this._showSubtasks
                          ? " active"
                          : ""}"
                        type="button"
                        title="${this._showSubtasks
                          ? "Hide subtasks"
                          : "Show subtasks on the board"}"
                        @click="${() => this.#toggleShowSubtasks()}"
                      >
                        <breeze-icon name="list-checks" size="14"></breeze-icon>
                        ${this._showSubtasks ? "Subtasks on" : "Subtasks"}
                      </button>
                    `
                    : nothing}
                </div>
              `
              : nothing} ${hasCycles && effectiveLayout !== "settings" &&
                effectiveLayout !== "cycles"
              ? html`
                <breeze-cycle-bar
                  .cycles="${cycles.value.projectId === project.id
                    ? cycles.value.cycles
                    : []}"
                  .activeCycleId="${this._cycleFilter}"
                  @cycle-change="${this.#onCycleChange}"
                ></breeze-cycle-bar>
              `
              : nothing}
          </div>

          <div class="pdp-content">
            ${effectiveLayout === "board"
              ? html`
                <breeze-kanban-board
                  .statuses="${statuses}"
                  .tasks="${filteredTasks}"
                  .grouped="${grouped}"
                  .projectId="${project.id ?? ""}"
                  .showSubtasks="${this._showSubtasks}"
                  @task-click="${(e: CustomEvent) =>
                    this.#onTaskClick(e.detail)}"
                ></breeze-kanban-board>
              `
              : effectiveLayout === "list"
              ? html`
                <breeze-list-view
                  .statuses="${statuses}"
                  .tasks="${filteredTasks}"
                  .projectId="${project.id ?? ""}"
                  .sortField="${this._sortField}"
                  .sortDir="${this._sortDir}"
                  @task-click="${(e: CustomEvent) =>
                    this.#onTaskClick(e.detail)}"
                  @sort-change="${this.#onSortChange}"
                ></breeze-list-view>
              `
              : effectiveLayout === "cycles"
              ? html`
                <breeze-cycles-view
                  .cycles="${cycles.value.projectId === project.id
                    ? cycles.value.cycles
                    : []}"
                ></breeze-cycles-view>
              `
              : effectiveLayout === "members"
              ? html`
                <breeze-project-members-view
                  .projectId="${project.id ?? ""}"
                  .canManage="${canManageMembers}"
                ></breeze-project-members-view>
              `
              : effectiveLayout === "settings"
              ? html`
                <breeze-settings-view
                  .project="${project}"
                  .statuses="${statuses}"
                ></breeze-settings-view>
              `
              : nothing}
          </div>
        </div>

        <breeze-task-dialog
          .open="${this._showCreate}"
          .project="${project}"
          .statuses="${statuses}"
          @close="${() => (this._showCreate = false)}"
        ></breeze-task-dialog>

        <breeze-task-detail-dialog
          .task="${tasks.find(
            (t) => t.id === projectDetail.value.selectedTaskId,
          ) ?? null}"
          .project="${project}"
          .statuses="${statuses}"
          @close="${this.#onTaskDetailClose}"
        ></breeze-task-detail-dialog>

        <breeze-save-view-dialog
          .open="${this._showSaveViewDialog}"
          .projectId="${project.id ?? ""}"
          .filters="${currentFilters}"
          .defaultLayout="${effectiveLayout === "list" ? "list" : "board"}"
          @close="${() => (this._showSaveViewDialog = false)}"
          @view-created="${this.#onSaveViewCreated}"
        ></breeze-save-view-dialog>
      </breeze-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-project-detail-page": BreezeProjectDetailPage;
  }
}
