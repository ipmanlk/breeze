/**
 * Async utilities for animation-aware loading.
 */

/**
 * Wraps an async load function with a minimum display time for the
 * loading state. This prevents the "flash" that occurs when data
 * loads faster than the user can perceive (e.g., < 200ms).
 *
 * Usage:
 *   async #loadMembers() {
 *     await loadWithMinTime(
 *       async () => { this._members = await api.list(); },
 *       (l) => { this._membersLoading = l; },
 *       200,
 *     );
 *   }
 *
 * @param loadFn - Async function that performs the data load.
 * @param onLoading - Callback to set the loading state boolean.
 * @param minTime - Minimum time (ms) to keep loading true (default 200).
 */
export async function loadWithMinTime<T>(
  loadFn: () => Promise<T>,
  onLoading: (loading: boolean) => void,
  minTime = 200,
): Promise<T> {
  const start = performance.now();
  onLoading(true);
  try {
    return await loadFn();
  } finally {
    const elapsed = performance.now() - start;
    const remaining = Math.max(0, minTime - elapsed);
    if (remaining > 0) {
      await new Promise((r) => setTimeout(r, remaining));
    }
    onLoading(false);
  }
}
