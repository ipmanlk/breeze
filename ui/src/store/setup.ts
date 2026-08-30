import { signal } from "@preact/signals-core";
import { getSetup } from "@/api";

/** null = unknown, true = needs setup, false = configured */
export const setupRequired = signal<boolean | null>(null);

export async function checkSetup(): Promise<void> {
  try {
    const { data } = await getSetup({ throwOnError: true });
    setupRequired.value =
      (data as Record<string, boolean> | undefined)?.needs_setup ?? false;
  } catch {
    setupRequired.value = true;
  }
}
