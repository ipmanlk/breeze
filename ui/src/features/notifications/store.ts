import { signal } from "@preact/signals-core";
import {
  getNotifications,
  getNotificationsUnreadCount,
  patchNotificationsByIdRead,
  patchNotificationsReadAll,
} from "@/api";
import type { DtoNotificationResponse } from "@/api";

// State
export interface NotificationState {
  items: DtoNotificationResponse[];
  isLoading: boolean;
  hasMore: boolean;
  nextCursor: string | undefined;
  isFetchingMore: boolean;
}

const initial: NotificationState = {
  items: [],
  isLoading: false,
  hasMore: false,
  nextCursor: undefined,
  isFetchingMore: false,
};

export const notifications = signal<NotificationState>({ ...initial });
export const unreadCount = signal<number>(0);

// Actions
/** Fetch the first page of notifications. */
export async function fetchNotifications(
  unreadOnly?: boolean,
): Promise<void> {
  notifications.value = { ...initial, isLoading: true };
  try {
    const params: Record<string, string> = { limit: "20" };
    if (unreadOnly) params.unread_only = "true";
    const { data } = await getNotifications({ query: params });
    const result = data ?? { items: [], has_more: false };
    notifications.value = {
      items: result.items ?? [],
      isLoading: false,
      hasMore: result.has_more ?? false,
      nextCursor: result.next_cursor,
      isFetchingMore: false,
    };
  } catch {
    notifications.value = { ...initial };
  }
}

/** Fetch the next page (cursor-based). */
export async function fetchMoreNotifications(
  unreadOnly?: boolean,
): Promise<void> {
  const s = notifications.value;
  if (!s.hasMore || s.isFetchingMore) return;
  notifications.value = { ...s, isFetchingMore: true };
  try {
    const params: Record<string, string> = { limit: "20" };
    if (s.nextCursor) params.cursor = s.nextCursor;
    if (unreadOnly) params.unread_only = "true";
    const { data } = await getNotifications({ query: params });
    const result = data ?? { items: [], has_more: false };
    notifications.value = {
      items: [...s.items, ...(result.items ?? [])],
      isLoading: false,
      hasMore: result.has_more ?? false,
      nextCursor: result.next_cursor,
      isFetchingMore: false,
    };
  } catch {
    notifications.value = { ...s, isFetchingMore: false };
  }
}

/** Mark a single notification as read. */
export async function markNotificationRead(id: string): Promise<void> {
  try {
    await patchNotificationsByIdRead({ path: { id } });
  } catch { /* optimistically proceed */ }
  const s = notifications.value;
  notifications.value = {
    ...s,
    items: s.items.map((n) => n.id === id ? { ...n, is_read: true } : n),
  };
  unreadCount.value = Math.max(0, unreadCount.value - 1);
}

/** Mark all notifications as read. */
export async function markAllNotificationsRead(): Promise<void> {
  try {
    await patchNotificationsReadAll();
  } catch { /* optimistically proceed */ }
  const s = notifications.value;
  notifications.value = {
    ...s,
    items: s.items.map((n) => ({ ...n, is_read: true })),
  };
  unreadCount.value = 0;
}

/** Fetch the unread count (for badge display). */
export async function fetchUnreadCount(): Promise<void> {
  try {
    const { data } = await getNotificationsUnreadCount();
    unreadCount.value = (data as { count?: number })?.count ?? 0;
  } catch { /* ignore */ }
}

/**
 * Apply a real-time `notification_new` WebSocket event.
 *
 * Increments the unread badge and prepends the notification to the inbox
 * list (only if the inbox has already been loaded: otherwise the next
 * fetch will pick it up). This is the wiring that makes the bell react
 * live to mentions, assignments, and comments.
 */
export function handleNotificationEvent(
  notif: DtoNotificationResponse,
): void {
  // Bump the unread badge.
  if (!notif.is_read) {
    unreadCount.value = unreadCount.value + 1;
  }
  // Prepend to the inbox if it has been loaded (non-empty items or an
  // explicit empty state). Avoids inserting into a never-loaded list.
  const s = notifications.value;
  if (s.items.length > 0 || !s.isLoading) {
    if (!s.items.some((n) => n.id === notif.id)) {
      notifications.value = {
        ...s,
        items: [notif, ...s.items],
      };
    }
  }
}
