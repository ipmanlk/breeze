import { logError } from "@/lib/log";

/**
 * Wraps an async SDK call in a try/catch that logs failures via logError
 * and returns a default value instead of throwing.
 *
 * Use this for fire-and-forget SDK calls where the caller doesn't need
 * to handle the error (e.g. background fetches that update signal state).
 * For calls that need to show a toast, set state on error, or rethrow,
 * keep the explicit try/catch: the wrapper adds indirection without
 * supporting those patterns.
 *
 * Example:
 *   const views = await sdkCall("fetchProjectViews failed:", () =>
 *     getProjectsByIdViews({ path: { id: projectId }, throwOnError: true }),
 *     []
 *   );
 */
/**
 * Narrow unknown data to a known type at the call site.
 *
 * Use this when you know the runtime type is correct but TypeScript cannot
 * prove it: e.g. narrowing from `Record<string, unknown>` (DnD source.data)
 * or from an all-optional DTO to a required-fields frontend type.
 *
 * This centralises the `as unknown as` pattern so it appears only here,
 * making it auditable and grep-able.
 */
export function identify<T>(data: unknown): T {
  return data as T;
}

export async function sdkCall<T>(
  label: string,
  fn: () => Promise<T>,
  defaultValue: T,
): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    logError(label, err);
    return defaultValue;
  }
}
