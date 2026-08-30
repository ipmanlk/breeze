import { logError } from "@/lib/log";
import { signal } from "@preact/signals-core";
import { showToast } from "@/components/ui/toast-store";
import { msg } from "@lit/localize";
import {
  deleteProjectsByIdTasksByTaskId,
  deleteProjectsByIdTasksByTaskIdDependenciesByBlocksTaskId,
  getProjectsByIdMyAccess,
  getProjectsByIdStatuses,
  getProjectsByIdTasks,
  getProjectsByIdTasksByTaskIdDependenciesBlocked,
  getProjectsByIdTasksByTaskIdDependenciesBlocking,
  getProjectsBySlugBySlug,
  patchProjectsByIdTasksByTaskIdPosition,
  postProjectsByIdTasks,
  postProjectsByIdTasksBatch,
  postProjectsByIdTasksByTaskIdDependencies,
  postProjectsByIdTasksByTaskIdDuplicate,
  postProjectsByIdTasksByTaskIdMove,
  postProjectsByIdTasksByTaskIdSubtasksReorder,
  putProjectsById,
  putProjectsByIdTasksByTaskId,
  putProjectsByIdTasksByTaskIdLabels,
} from "@/api";
import type {
  DtoBatchUpdateRequest,
  DtoCreateTaskRequest,
  DtoLabelResponse,
  DtoMoveTaskRequest,
  DtoMoveToProjectRequest,
  DtoProjectAccessResponse,
  DtoProjectResponse,
  DtoTaskResponse,
  DtoTaskStatusResponse,
  DtoUpdateProjectRequest,
  DtoUpdateTaskRequest,
} from "@/api";
import type { ProjectPermissionKey } from "@/lib/permissions";

export interface ProjectDetailState {
  project: DtoProjectResponse | null;
  statuses: DtoTaskStatusResponse[];
  tasks: DtoTaskResponse[];
  isLoading: boolean;
  selectedTaskId: string | null;
  /** Effective role + permissions the current user has on this project. */
  access: DtoProjectAccessResponse | null;
}

export const projectDetail = signal<ProjectDetailState>({
  project: null,
  statuses: [],
  tasks: [],
  isLoading: true,
  selectedTaskId: null,
  access: null,
});

export function selectTask(taskId: string | null) {
  projectDetail.value = { ...projectDetail.value, selectedTaskId: taskId };
}

/**
 * Returns true if the current user has the given project-scoped permission on
 * the loaded project. Decided entirely by the backend `my-access` response;
 * the frontend holds no role→permission map.
 */
export function hasProjectPermission(perm: ProjectPermissionKey): boolean {
  return projectDetail.value.access?.permissions?.includes(perm) ?? false;
}

/** Effective role the current user has on the loaded project (if any). */
export function projectEffectiveRole(): string | undefined {
  return projectDetail.value.access?.role;
}

export async function fetchProjectDetail(slug: string): Promise<void> {
  projectDetail.value = { ...projectDetail.value, isLoading: true };

  try {
    const { data: projectData } = await getProjectsBySlugBySlug({
      path: { slug },
      throwOnError: true,
    });
    const project = projectData ?? null;

    if (!project || !project.id) {
      projectDetail.value = {
        project: null,
        statuses: [],
        tasks: [],
        isLoading: false,
        selectedTaskId: null,
        access: null,
      };
      return;
    }

    const projectId = project.id;

    const [statusesRes, tasksRes, accessRes] = await Promise.all([
      getProjectsByIdStatuses({
        path: { id: projectId },
        throwOnError: true,
      }),
      getProjectsByIdTasks({
        path: { id: projectId },
        throwOnError: true,
      }),
      getProjectsByIdMyAccess({ path: { id: projectId }, throwOnError: true }),
    ]);

    projectDetail.value = {
      project,
      statuses: (statusesRes.data) ??
        [],
      tasks: (tasksRes.data) ?? [],
      isLoading: false,
      selectedTaskId: projectDetail.value.selectedTaskId,
      access: (accessRes.data) ?? null,
    };
  } catch (err) {
    logError("fetchProjectDetail failed:", err);
    projectDetail.value = {
      project: null,
      statuses: [],
      tasks: [],
      isLoading: false,
      selectedTaskId: null,
      access: null,
    };
  }
}

export async function fetchTasks(
  projectId: string,
  params?: Record<string, string>,
): Promise<void> {
  try {
    const { data } = await getProjectsByIdTasks({
      path: { id: projectId },
      query: params as Record<string, string | undefined> | undefined,
      throwOnError: true,
    });
    projectDetail.value = {
      ...projectDetail.value,
      tasks: data ?? [],
    };
  } catch (err) {
    // Keep existing tasks on error
    logError("fetchTasks failed:", err);
  }
}

export async function deleteTask(
  projectId: string,
  taskId: string,
): Promise<void> {
  try {
    await deleteProjectsByIdTasksByTaskId({
      path: { id: projectId, taskId },
      throwOnError: true,
    });
    await fetchTasks(projectId);
  } catch (err) {
    logError("deleteTask failed:", err);
    showToast(msg("Failed to delete task"), { variant: "error" });
    await fetchTasks(projectId);
  }
}

export async function createTask(
  projectId: string,
  input: DtoCreateTaskRequest,
): Promise<DtoTaskResponse | null> {
  try {
    const { data } = await postProjectsByIdTasks({
      path: { id: projectId },
      body: input,
      throwOnError: true,
    });
    await fetchTasks(projectId);
    return data;
  } catch (err) {
    logError("createTask failed:", err);
    showToast(msg("Failed to create task"), { variant: "error" });
    return null;
  }
}

export async function moveTask(
  projectId: string,
  taskId: string,
  target: DtoMoveTaskRequest,
): Promise<void> {
  // Optimistic: patch the task's status + position_key instantly so the card
  // doesn't snap back to its old position while the request is in flight.
  const prev = projectDetail.value.tasks;
  projectDetail.value = {
    ...projectDetail.value,
    tasks: prev.map((t) =>
      t.id === taskId
        ? {
          ...t,
          status_id: target.status_id,
          position_key: target.position_key,
        }
        : t
    ),
  };
  try {
    await patchProjectsByIdTasksByTaskIdPosition({
      path: { id: projectId, taskId },
      body: target,
      throwOnError: true,
    });
    // Success: keep optimistic state, no refetch needed.
  } catch (err) {
    logError("moveTask failed:", err);
    showToast(msg("Failed to move task"), { variant: "error" });
    // Revert on error by refetching the authoritative state.
    projectDetail.value = { ...projectDetail.value, tasks: prev };
    await fetchTasks(projectId);
  }
}

export async function updateTask(
  projectId: string,
  taskId: string,
  patch: DtoUpdateTaskRequest,
): Promise<void> {
  // Optimistic: apply the patch fields to the local task instantly.
  const prev = projectDetail.value.tasks;
  projectDetail.value = {
    ...projectDetail.value,
    tasks: prev.map((t) =>
      t.id === taskId ? { ...t, ...patch } as DtoTaskResponse : t
    ),
  };
  try {
    await putProjectsByIdTasksByTaskId({
      path: { id: projectId, taskId },
      body: patch,
      throwOnError: true,
    });
    // Success: keep optimistic state, no refetch needed.
  } catch (err) {
    logError("updateTask failed:", err);
    showToast(msg("Failed to update task"), { variant: "error" });
    // Revert on error by refetching the authoritative state.
    projectDetail.value = { ...projectDetail.value, tasks: prev };
    await fetchTasks(projectId);
  }
}

/**
 * Replace a task's label set. Optimistically patches the in-store task so the
 * chips update instantly, then refetches to reconcile with the server's
 * authoritative label objects (name/color).
 */
export async function setTaskLabels(
  projectId: string,
  taskId: string,
  labelIds: string[],
): Promise<void> {
  // Optimistic update: stamp the requested IDs onto the local task so the
  // picker reflects the change before the round-trip. Colors/names are
  // resolved by the subsequent fetchTasks.
  const prev = projectDetail.value.tasks;
  const optimisticLabels: DtoLabelResponse[] = labelIds.map((id) => {
    const existing = prev
      .find((t) => t.id === taskId)
      ?.labels?.find((l) => l.id === id);
    return existing ?? ({ id, name: "", color: "" } as DtoLabelResponse);
  });
  projectDetail.value = {
    ...projectDetail.value,
    tasks: prev.map((t) =>
      t.id === taskId ? { ...t, labels: optimisticLabels } : t
    ),
  };
  try {
    await putProjectsByIdTasksByTaskIdLabels({
      path: { id: projectId, taskId },
      body: { label_ids: labelIds },
      throwOnError: true,
    });
    await fetchTasks(projectId);
  } catch (err) {
    logError("setTaskLabels failed:", err);
    showToast(msg("Failed to update labels"), { variant: "error" });
    // Revert on error by refetching the authoritative state.
    await fetchTasks(projectId);
  }
}

/**
 * Apply a partial update to many tasks at once (bulk edit). Sends only the
 * fields present in `patch`; absent fields are left untouched server-side.
 * Refetches the task list on success so chips/cards reflect the new state.
 */
export async function batchUpdateTasks(
  projectId: string,
  taskIds: string[],
  patch: Omit<DtoBatchUpdateRequest, "task_ids">,
): Promise<void> {
  try {
    await postProjectsByIdTasksBatch({
      path: { id: projectId },
      body: { ...patch, task_ids: taskIds },
      throwOnError: true,
    });
    await fetchTasks(projectId);
  } catch (err) {
    logError("batchUpdateTasks failed:", err);
    showToast(msg("Failed to update tasks"), { variant: "error" });
    await fetchTasks(projectId);
  }
}

/**
 * Duplicate a task in its own project. The backend returns the new task;
 * we refetch the list so the copy appears in the board/list with full
 * label + assignee resolution.
 */
export async function duplicateTask(
  projectId: string,
  taskId: string,
  includeSubtasks = false,
): Promise<DtoTaskResponse | null> {
  try {
    const { data } = await postProjectsByIdTasksByTaskIdDuplicate({
      path: { id: projectId, taskId },
      query: includeSubtasks ? { include_subtasks: true } : undefined,
      throwOnError: true,
    });
    await fetchTasks(projectId);
    return data;
  } catch (err) {
    logError("duplicateTask failed:", err);
    showToast(msg("Failed to duplicate task"), { variant: "error" });
    return null;
  }
}

export async function reorderSubtasks(
  projectId: string,
  taskId: string,
  operations: Array<
    { task_id: string; position_key: string }
  >,
): Promise<void> {
  try {
    await postProjectsByIdTasksByTaskIdSubtasksReorder({
      path: { id: projectId, taskId },
      body: { operations },
      throwOnError: true,
    });
    await fetchTasks(projectId);
  } catch (err) {
    logError("reorderSubtasks failed:", err);
    showToast(msg("Failed to reorder subtasks"), { variant: "error" });
  }
}

/**
 * Move a task to a different project + status. Clears cycle + parent
 * server-side. Refetches the source project's task list so the moved task
 * disappears from the current board.
 */
export async function moveTaskToProject(
  fromProjectId: string,
  taskId: string,
  target: DtoMoveToProjectRequest,
): Promise<boolean> {
  try {
    await postProjectsByIdTasksByTaskIdMove({
      path: { id: fromProjectId, taskId },
      body: target,
      throwOnError: true,
    });
    await fetchTasks(fromProjectId);
    return true;
  } catch (err) {
    logError("moveTaskToProject failed:", err);
    showToast(msg("Failed to move task"), { variant: "error" });
    return false;
  }
}

/**
 * Fetch the tasks blocking the given task (the task is blocked by these) and
 * the tasks the given task is blocking. Returns { blocking, blocked }.
 */
export async function fetchTaskDependencies(
  projectId: string,
  taskId: string,
): Promise<{ blocking: DtoTaskResponse[]; blocked: DtoTaskResponse[] }> {
  try {
    const [bRes, dRes] = await Promise.all([
      getProjectsByIdTasksByTaskIdDependenciesBlocking({
        path: { id: projectId, taskId },
        throwOnError: true,
      }),
      getProjectsByIdTasksByTaskIdDependenciesBlocked({
        path: { id: projectId, taskId },
        throwOnError: true,
      }),
    ]);
    return {
      blocking: (bRes.data) ?? [],
      blocked: (dRes.data) ?? [],
    };
  } catch (err) {
    logError("fetchTaskDependencies failed:", err);
    return { blocking: [], blocked: [] };
  }
}

/** Record that a task is blocked by another task. */
export async function addTaskDependency(
  projectId: string,
  taskId: string,
  blocksTaskId: string,
): Promise<boolean> {
  try {
    await postProjectsByIdTasksByTaskIdDependencies({
      path: { id: projectId, taskId },
      body: { blocks_task_id: blocksTaskId },
      throwOnError: true,
    });
    return true;
  } catch (err) {
    logError("addTaskDependency failed:", err);
    showToast(msg("Failed to add dependency"), { variant: "error" });
    return false;
  }
}

/** Remove a blocking edge between two tasks. */
export async function removeTaskDependency(
  projectId: string,
  taskId: string,
  blocksTaskId: string,
): Promise<boolean> {
  try {
    await deleteProjectsByIdTasksByTaskIdDependenciesByBlocksTaskId({
      path: { id: projectId, taskId, blocksTaskId },
      throwOnError: true,
    });
    return true;
  } catch (err) {
    logError("removeTaskDependency failed:", err);
    showToast(msg("Failed to remove dependency"), { variant: "error" });
    return false;
  }
}

/** Get tasks grouped by status, sorted by position_key */
export function tasksByStatus(tasks: DtoTaskResponse[]): Map<
  string,
  DtoTaskResponse[]
> {
  const map = new Map<string, DtoTaskResponse[]>();
  for (const t of tasks) {
    const sid = t.status_id ?? "";
    const list = map.get(sid);
    if (list) list.push(t);
    else map.set(sid, [t]);
  }
  for (const list of map.values()) {
    list.sort((a, b) =>
      (a.position_key ?? "") < (b.position_key ?? "") ? -1 : 1
    );
  }
  return map;
}

/**
 * Apply a real-time task update from a WebSocket broadcast (task_created,
 * task_updated, task_moved). Upserts the task into the local store so the
 * board/list reflects the change instantly without a refetch.
 *
 * Note: this does NOT refetch from the server. The task object is the
 * authoritative state broadcast by the backend. If the task belongs to a
 * project other than the currently-loaded one it is ignored (should not
 * happen since the broadcast is project-room-scoped).
 */
export function applyWsTaskEvent(task: DtoTaskResponse): void {
  const current = projectDetail.value;
  if (!current.project || current.project.id !== task.project_id) {
    return;
  }
  const existing = current.tasks.some((t) => t.id === task.id);
  const tasks = existing
    ? current.tasks.map((t) => (t.id === task.id ? { ...t, ...task } : t))
    : [...current.tasks, task];
  projectDetail.value = { ...current, tasks };
}

/**
 * Remove a task from the local store after a task_deleted WebSocket broadcast.
 */
export function removeWsTask(taskId: string): void {
  const current = projectDetail.value;
  projectDetail.value = {
    ...current,
    tasks: current.tasks.filter((t) => t.id !== taskId),
  };
}

/** Update a project (settings: enable cycles, cycle config, etc.). */
export async function updateProject(
  projectId: string,
  patch: DtoUpdateProjectRequest,
): Promise<DtoProjectResponse | null> {
  try {
    const { data } = await putProjectsById({
      path: { id: projectId },
      body: patch,
      throwOnError: true,
    });
    const updated = data;
    projectDetail.value = {
      ...projectDetail.value,
      project: updated,
    };
    return updated;
  } catch (err) {
    logError("updateProject failed:", err);
    showToast(msg("Failed to update project"), { variant: "error" });
    return null;
  }
}

/** Re-fetch a project's statuses (after status CRUD / reorder). */
export async function refreshStatuses(
  projectId: string,
): Promise<DtoTaskStatusResponse[]> {
  try {
    const { data } = await getProjectsByIdStatuses({
      path: { id: projectId },
      throwOnError: true,
    });
    const statuses = data ?? [];
    projectDetail.value = { ...projectDetail.value, statuses };
    return statuses;
  } catch (err) {
    logError("refreshStatuses failed:", err);
    return projectDetail.value.statuses;
  }
}

/**
 * Optimistically set a project's statuses in the store (e.g. after a drag
 * reorder) so the UI updates instantly; pair with `refreshStatuses` to
 * reconcile once the server round-trip finishes.
 */
export function setProjectStatuses(
  statuses: DtoTaskStatusResponse[],
): void {
  projectDetail.value = { ...projectDetail.value, statuses };
}
