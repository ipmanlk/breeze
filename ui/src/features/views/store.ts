import { msg } from "@lit/localize";
import { logError } from "@/lib/log";
import { sdkCall } from "@/lib/sdk-helpers";
import { signal } from "@preact/signals-core";
import { showToast } from "@/components/ui/toast-store";
import {
  deleteViewsById,
  deleteViewsByIdPin,
  getProjectsByIdViews,
  getViews,
  getViewsById,
  getViewsPins,
  patchViewsById,
  postViews,
  postViewsByIdPin,
} from "@/api";
import type { DtoViewResponse } from "@/api";
import type { View, ViewFilters, ViewLayout } from "./types";

export interface ViewsState {
  globalViews: View[];
  pinnedViews: View[];
  isLoading: boolean;
}

export const views = signal<ViewsState>({
  globalViews: [],
  pinnedViews: [],
  isLoading: true,
});

function unwrapView(data: DtoViewResponse): View {
  return {
    id: data.id ?? "",
    project_id: data.project_id,
    project_slug: data.project_slug,
    project_name: data.project_name,
    created_by: data.created_by ?? "",
    name: data.name ?? "",
    layout: (data.layout as ViewLayout) ?? "board",
    filters: (data.filters as ViewFilters) ?? {},
    created_at: data.created_at ?? "",
    updated_at: data.updated_at ?? "",
  };
}

export async function fetchGlobalViews(): Promise<void> {
  try {
    const { data } = await getViews({ throwOnError: true });
    const items = (data as DtoViewResponse[]) ?? [];
    views.value = {
      ...views.value,
      globalViews: items.map(unwrapView),
      isLoading: false,
    };
  } catch (err) {
    views.value = { ...views.value, isLoading: false };
    logError("fetchGlobalViews failed:", err);
  }
}

export async function fetchPinnedViews(): Promise<void> {
  try {
    const { data } = await getViewsPins({ throwOnError: true });
    const items = (data as DtoViewResponse[]) ?? [];
    views.value = {
      ...views.value,
      pinnedViews: items.map(unwrapView),
    };
  } catch (err) {
    logError("fetchPinnedViews failed:", err);
  }
}

export async function fetchProjectViews(projectId: string): Promise<View[]> {
  return sdkCall("fetchProjectViews failed:", async () => {
    const { data } = await getProjectsByIdViews({
      path: { id: projectId },
      throwOnError: true,
    });
    const items = (data as DtoViewResponse[]) ?? [];
    return items.map(unwrapView);
  }, []);
}

export async function fetchView(id: string): Promise<View | null> {
  return sdkCall("fetchView failed:", async () => {
    const { data } = await getViewsById({ path: { id }, throwOnError: true });
    if (!data) return null;
    return unwrapView(data as DtoViewResponse);
  }, null);
}

export async function pinView(id: string): Promise<void> {
  try {
    await postViewsByIdPin({ path: { id }, throwOnError: true });
    await fetchPinnedViews();
  } catch (err) {
    logError("pinView failed:", err);
    showToast(msg("Failed to pin view"), { variant: "error" });
  }
}

export async function unpinView(id: string): Promise<void> {
  try {
    await deleteViewsByIdPin({ path: { id }, throwOnError: true });
    await fetchPinnedViews();
  } catch (err) {
    logError("unpinView failed:", err);
    showToast(msg("Failed to unpin view"), { variant: "error" });
  }
}

export async function createView(
  name: string,
  layout: ViewLayout,
  filters: ViewFilters,
  projectId?: string,
): Promise<View | null> {
  try {
    const body: Record<string, unknown> = {
      name,
      layout,
      filters,
    };
    if (projectId) body.project_id = projectId;
    const { data } = await postViews({
      body: body as Parameters<typeof postViews>[0]["body"],
      throwOnError: true,
    });
    return unwrapView(data as DtoViewResponse);
  } catch (err) {
    logError("createView failed:", err);
    showToast(msg("Failed to create view"), { variant: "error" });
    return null;
  }
}

export async function updateView(
  id: string,
  patch: { name?: string; layout?: ViewLayout; filters?: ViewFilters },
): Promise<View | null> {
  try {
    const { data } = await patchViewsById({
      path: { id },
      body: patch as Parameters<typeof patchViewsById>[0]["body"],
      throwOnError: true,
    });
    return unwrapView(data as DtoViewResponse);
  } catch (err) {
    logError("updateView failed:", err);
    showToast(msg("Failed to update view"), { variant: "error" });
    return null;
  }
}

export async function deleteView(id: string): Promise<boolean> {
  try {
    await deleteViewsById({ path: { id }, throwOnError: true });
    return true;
  } catch (err) {
    logError("deleteView failed:", err);
    showToast(msg("Failed to delete view"), { variant: "error" });
    return false;
  }
}
