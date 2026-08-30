import { signal } from "@preact/signals-core";

/**
 * Toast severity: drives icon + accent color.
 */
export type ToastVariant = "default" | "success" | "error";

export interface ToastItem {
  id: number;
  message: string;
  variant: ToastVariant;
  /** Auto-dismiss after this many ms. 0 = sticky (manual dismiss). */
  duration: number;
}

export interface ToastOptions {
  variant?: ToastVariant;
  duration?: number;
}

export const toasts = signal<ToastItem[]>([]);

let nextId = 1;

/**
 * Show a transient toast notification. Returns the toast id (so callers can
 * dismiss it programmatically). Toasts render in a top-level host appended
 * to <body> by <breeze-toast-host>, so they survive navigation and are not
 * scoped to a single view's lifecycle.
 */
export function showToast(message: string, opts: ToastOptions = {}): number {
  const id = nextId++;
  const duration = opts.duration ?? 4000;
  const item: ToastItem = {
    id,
    message,
    variant: opts.variant ?? "default",
    duration,
  };
  toasts.value = [...toasts.value, item];
  if (duration > 0) {
    setTimeout(() => dismissToast(id), duration);
  }
  return id;
}

/** Dismiss a toast by id (no-op if already gone). */
export function dismissToast(id: number): void {
  const next = toasts.value.filter((t) => t.id !== id);
  if (next.length !== toasts.value.length) {
    toasts.value = next;
  }
}

/** Dismiss all toasts. */
export function clearToasts(): void {
  if (toasts.value.length > 0) toasts.value = [];
}
