/**
 * Centralized logging utilities.
 *
 * All non-framework code should route user-facing diagnostics through these
 * helpers instead of calling `console.*` directly. Doing so keeps call sites
 * consistent and gives a single seam for future enhancements (structured
 * logging, remote telemetry, production stripping, etc.).
 *
 * The helpers currently forward verbatim to the matching `console` method so
 * existing behavior (stack traces, devtools formatting) is preserved.
 */

export function logError(...args: unknown[]): void {
  console.error(...args);
}

export function logWarn(...args: unknown[]): void {
  console.warn(...args);
}
