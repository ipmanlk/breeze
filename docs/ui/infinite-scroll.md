# Infinite Scroll & Paginated Lists in Lit

How to build a cursor-paginated, infinite-scrolling list (the inbox pattern).
Read this when porting any React page that uses `useInfiniteQuery` +
`IntersectionObserver`.

---

## Refs in Lit are not React ref callbacks

React lets you pass a callback: `ref={el => setupObserver(el)}`. **Lit has no
such inline callback ref.** Use `createRef` + the `ref` directive, and wire the
observer in `firstUpdated()`:

```ts
import { createRef, ref } from "lit/directives/ref.js";

@customElement("breeze-inbox-page")
export class BreezeInboxPage extends LitElement {
  #sentinelRef = createRef<HTMLDivElement>();
  #observer?: IntersectionObserver;

  protected firstUpdated(): void {
    const el = this.#sentinelRef.value;
    if (!el) return;
    this.#observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          const s = notifications.value;
          if (s.hasMore && !s.isFetchingMore) {
            fetchMoreNotifications(this._unreadOnly);
          }
        }
      },
      { threshold: 0.1 },
    );
    this.#observer.observe(el);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#observer?.disconnect(); // always clean up
  }

  protected render() {
    return html`
      <div class="list">…items…</div>
      <div class="sentinel" ${ref(this.#sentinelRef)}></div>
    `;
  }
}
```

### Why `firstUpdated()` and not `connectedCallback()`

The sentinel element isn't in the DOM during `connectedCallback()`; the
component hasn't rendered yet. `firstUpdated()` fires after the first render, so
`this.#sentinelRef.value` is populated. `updated()` would re-fire on every
update; use `firstUpdated()` for one-time observer setup.

## Cursor-pagination signal store

React used `useInfiniteQuery` with `getNextPageParam`. The Lit equivalent is a
signal store that accumulates pages. Pattern
(`features/notifications/store.ts`):

```ts
export interface NotificationState {
  items: DtoNotificationResponse[];
  isLoading: boolean;
  hasMore: boolean;
  nextCursor: string | undefined;
  isFetchingMore: boolean;   // guards against double-fetch
}

export const notifications = signal<NotificationState>({ …initial });

export async function fetchNotifications(unreadOnly?: boolean) {
  notifications.value = { …initial, isLoading: true };
  const { data } = await getNotifications({ query: { limit: "20", ...(unreadOnly && { unread_only: "true" }) } });
  notifications.value = { items: data.items ?? [], isLoading: false,
    hasMore: data.has_more ?? false, nextCursor: data.next_cursor, isFetchingMore: false };
}

export async function fetchMoreNotifications(unreadOnly?: boolean) {
  const s = notifications.value;
  if (!s.hasMore || s.isFetchingMore) return;          // ← guard
  notifications.value = { …s, isFetchingMore: true };
  const { data } = await getNotifications({ query: { limit: "20", ...(s.nextCursor && { cursor: s.nextCursor }), ...(unreadOnly && { unread_only: "true" }) } });
  notifications.value = { items: [...s.items, ...(data.items ?? [])], isLoading: false,
    hasMore: data.has_more ?? false, nextCursor: data.next_cursor, isFetchingMore: false };
}
```

Key fields: `hasMore` (stop observing when false), `nextCursor` (cursor param),
`isFetchingMore` (prevents the observer from firing twice before the first page
resolves). The observer checks `s.hasMore && !s.isFetchingMore` before calling
`fetchMore`.

## Optimistic mutations

Mark-as-read updates the list **locally** without refetching (matches React's
`queryClient.setQueryData`):

```ts
export async function markNotificationRead(id: string) {
  try {
    await patchNotificationsByIdRead({ path: { id } });
  } catch { /* optimistic */ }
  const s = notifications.value;
  notifications.value = {
    ...s,
    items: s.items.map((n) => n.id === id ? { ...n, is_read: true } : n),
  };
  unreadCount.value = Math.max(0, unreadCount.value - 1);
}
```

## Loading states

Three visual states, mirroring React's `Skeleton` + `isFetchingNextPage`:

```ts
${s.isLoading
  ? html`<div class="loading">${Array.from({ length: 5 }, () => html`<div class="skeleton"></div>`)}</div>`
  : displayed.length === 0
  ? html`<div class="empty">…bell icon + "All caught up"…</div>`
  : html`${displayed.map(n => html`<breeze-notification-item …></breeze-notification-item>`)}`}

${s.isFetchingMore
  ? html`<div class="loading-more"><div class="skeleton"></div><div class="skeleton"></div></div>`
  : nothing}
```

The skeleton uses a `pulse` keyframe; animation durations are an accepted
literal (no token); see `design-tokens.md`.

## Don't re-style the scrollbar

`.list { overflow-y: auto; }` is enough; the global thin-scrollbar rules in
`index.css` already apply. Do not re-declare `::-webkit-scrollbar` per component
(it hardcodes `6px`/`3px`). See `design-tokens.md`.

## Filtering client-side vs re-fetching

For a toggle like "Unread only" that the backend supports via a query param,
**re-fetch** from page 1 (don't filter the already-loaded list, or pagination
breaks):

```ts
private _toggleUnread(e: CustomEvent) {
  this._unreadOnly = (e.detail as { checked: boolean }).checked;
  fetchNotifications(this._unreadOnly);   // resets to page 1 with the new filter
}
```
