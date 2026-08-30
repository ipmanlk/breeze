import { msg } from "@lit/localize";
import { signal } from "@preact/signals-core";
import { getDashboard, patchDashboardVisibility } from "@/api";
import { showToast } from "@/components/ui/toast-store";
import type { DtoDashboardSectionResponse } from "@/api";

export interface DashboardState {
  sections: DtoDashboardSectionResponse[];
  isLoading: boolean;
}

export const dashboard = signal<DashboardState>({
  sections: [],
  isLoading: true,
});

export async function fetchDashboard(): Promise<void> {
  dashboard.value = { ...dashboard.value, isLoading: true };
  try {
    const { data } = await getDashboard({ throwOnError: true });
    dashboard.value = {
      sections: data?.sections ?? [],
      isLoading: false,
    };
  } catch {
    dashboard.value = { sections: [], isLoading: false };
    showToast(msg("Failed to load dashboard"), { variant: "error" });
  }
}

export async function reorderSections(types: string[]): Promise<void> {
  try {
    await patchDashboardVisibility({
      body: { sections: types },
      throwOnError: true,
    });
    await fetchDashboard();
  } catch {
    // keep local state as-is
    showToast(msg("Failed to reorder sections"), { variant: "error" });
  }
}
