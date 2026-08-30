import { signal } from "@preact/signals-core";
import { showToast } from "@/components/ui/toast-store";
import { msg } from "@lit/localize";
import {
  getProjects,
  postProjects,
  postProjectsByIdArchive,
  postProjectsByIdUnarchive,
} from "@/api";
import type { DtoCreateProjectRequest, DtoProjectResponse } from "@/api";

export interface ProjectsState {
  projects: DtoProjectResponse[];
  isLoading: boolean;
}

export const projects = signal<ProjectsState>({
  projects: [],
  isLoading: true,
});

export async function fetchProjects(): Promise<void> {
  projects.value = { ...projects.value, isLoading: true };
  try {
    const { data } = await getProjects({ throwOnError: true });
    projects.value = {
      projects: data,
      isLoading: false,
    };
  } catch {
    projects.value = { projects: [], isLoading: false };
    showToast(msg("Failed to load projects"), { variant: "error" });
  }
}

/**
 * Fetch projects including archived ones. Used by the "Show archived" toggle
 * on the projects page so users can discover and restore archived projects
 * (which are otherwise hidden from the default list).
 */
export async function fetchProjectsIncludingArchived(): Promise<void> {
  projects.value = { ...projects.value, isLoading: true };
  try {
    const { data } = await getProjects({
      query: { archived: true },
      throwOnError: true,
    });
    projects.value = {
      projects: data,
      isLoading: false,
    };
  } catch {
    projects.value = { projects: [], isLoading: false };
    showToast(msg("Failed to load archived projects"), { variant: "error" });
  }
}

export async function createProject(
  body: DtoCreateProjectRequest,
): Promise<DtoProjectResponse | null> {
  try {
    const { data } = await postProjects({ body, throwOnError: true });
    await fetchProjects();
    return data;
  } catch {
    return null;
  }
}

/** Archive a project (hides it from the default list, makes it read-only). */
export async function archiveProject(
  projectId: string,
): Promise<boolean> {
  try {
    await postProjectsByIdArchive({
      path: { id: projectId },
      throwOnError: true,
    });
    await fetchProjects();
    showToast(msg("Project archived"), { variant: "success" });
    return true;
  } catch {
    showToast(msg("Failed to archive project"), { variant: "error" });
    return false;
  }
}

/** Restore an archived project to the active list. */
export async function unarchiveProject(
  projectId: string,
): Promise<boolean> {
  try {
    await postProjectsByIdUnarchive({
      path: { id: projectId },
      throwOnError: true,
    });
    showToast(msg("Project restored"), { variant: "success" });
    // Refresh whichever list is currently shown (archived-inclusive view).
    await fetchProjectsIncludingArchived();
    return true;
  } catch {
    showToast(msg("Failed to restore project"), { variant: "error" });
    return false;
  }
}
