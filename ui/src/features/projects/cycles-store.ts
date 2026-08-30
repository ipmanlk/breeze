import { signal } from "@preact/signals-core";
import {
  deleteProjectsByIdCyclesByCycleId,
  getProjectsByIdCycles,
  postProjectsByIdCycles,
  postProjectsByIdCyclesByCycleIdActivate,
  postProjectsByIdCyclesByCycleIdComplete,
  putProjectsByIdCyclesByCycleId,
} from "@/api";
import type {
  DtoCompleteCycleRequest,
  DtoCreateCycleRequest,
  DtoCycleResponse,
  DtoUpdateCycleRequest,
} from "@/api";

/**
 * Cycles store: project-scoped server-state cache for a project's cycles.
 *
 * Kept separate from `store/project-detail.ts` because cycles are only
 * consumed by the Settings/Cycles views and should not be fetched on every
 * project-detail load. `fetchCycles` is a no-op when `projectId` matches the
 * already-loaded project (avoid refetches on tab switches); pass
 * `force: true` to reload (after create/update/complete/delete).
 */
export interface CyclesState {
  projectId: string;
  cycles: DtoCycleResponse[];
  isLoading: boolean;
}

export const cycles = signal<CyclesState>({
  projectId: "",
  cycles: [],
  isLoading: false,
});

export async function fetchCycles(
  projectId: string,
  force = false,
): Promise<DtoCycleResponse[]> {
  if (!projectId) return [];
  if (
    !force &&
    cycles.value.projectId === projectId &&
    !cycles.value.isLoading
  ) {
    return cycles.value.cycles;
  }
  cycles.value = { projectId, cycles: [], isLoading: true };
  try {
    const { data } = await getProjectsByIdCycles({
      path: { id: projectId },
      throwOnError: true,
    });
    const list = data ?? [];
    cycles.value = { projectId, cycles: list, isLoading: false };
    return list;
  } catch {
    cycles.value = { projectId, cycles: [], isLoading: false };
    return [];
  }
}

export async function createCycle(
  projectId: string,
  body: DtoCreateCycleRequest,
): Promise<DtoCycleResponse | null> {
  try {
    await postProjectsByIdCycles({
      path: { id: projectId },
      body,
      throwOnError: true,
    });
    await fetchCycles(projectId, true);
    return cycles.value.cycles.at(-1) ?? null;
  } catch {
    return null;
  }
}

export async function updateCycle(
  projectId: string,
  cycleId: string,
  body: DtoUpdateCycleRequest,
): Promise<void> {
  try {
    await putProjectsByIdCyclesByCycleId({
      path: { id: projectId, cycleId },
      body,
      throwOnError: true,
    });
    await fetchCycles(projectId, true);
  } catch {
    await fetchCycles(projectId, true);
  }
}

export async function activateCycle(
  projectId: string,
  cycleId: string,
): Promise<void> {
  try {
    await postProjectsByIdCyclesByCycleIdActivate({
      path: { id: projectId, cycleId },
      throwOnError: true,
    });
    await fetchCycles(projectId, true);
  } catch {
    await fetchCycles(projectId, true);
  }
}

export async function completeCycle(
  projectId: string,
  cycleId: string,
  body?: DtoCompleteCycleRequest,
): Promise<void> {
  try {
    await postProjectsByIdCyclesByCycleIdComplete({
      path: { id: projectId, cycleId },
      body,
      throwOnError: true,
    });
    await fetchCycles(projectId, true);
  } catch {
    await fetchCycles(projectId, true);
  }
}

export async function deleteCycle(
  projectId: string,
  cycleId: string,
): Promise<void> {
  try {
    await deleteProjectsByIdCyclesByCycleId({
      path: { id: projectId, cycleId },
      throwOnError: true,
    });
    await fetchCycles(projectId, true);
  } catch {
    await fetchCycles(projectId, true);
  }
}
