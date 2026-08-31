import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";
import { navigate } from "@/routes/router";
import {
  listItemEnterStyles,
  pageEnterStyles,
} from "@/styles/shared-animations";
import {
  fetchMoreNotifications,
  fetchNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  notifications,
  unreadCount,
} from "./store";
import { SignalController } from "@/lib/signal-controller";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/switch.ts";
import "../../layouts/app-layout.ts";
import "./notification-item.ts";

/**
 * Inbox page: notifications list with unread-only toggle and mark-all-read.
 */
@localized()
@customElement("plume-inbox-page")
export class PlumeInboxPage extends LitElement {
  static styles = [
    pageEnterStyles,
    listItemEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }
      .page {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      /* Header */
      .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-3);
        border-bottom: 1px solid var(--border);
        padding: var(--space-4) var(--space-6);
        flex-shrink: 0;
      }
      .header h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        font-family: var(--font-heading, inherit);
        color: var(--foreground);
      }
      .header-sub {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .header-actions {
        display: flex;
        align-items: center;
        gap: var(--space-3);
      }
      .toggle-label {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-size: var(--text-xs);
        font-weight: 400;
        color: var(--muted-foreground);
        cursor: pointer;
        user-select: none;
      }
      .mark-all-btn {
        display: inline-flex;
        align-items: center;
        height: var(--control-h-sm);
        padding: 0 var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        cursor: pointer;
        transition: background var(--dur-fast) var(--ease-1);
      }
      .mark-all-btn:hover {
        background: var(--accent);
      }
      /* List: scrollbar styling comes from the global thin-scrollbar rules
        in styles/index.css (applied via the universal selector). */
      .list {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-6);
      }
      /* Empty state */
      .empty {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--space-2);
        padding: var(--space-20) 0;
        text-align: center;
      }
      .empty-icon {
        color: color-mix(in oklch, var(--muted-foreground) 30%, transparent);
      }
      .empty h3 {
        margin: 0;
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .empty p {
        margin: 0;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      /* Loading */
      .loading {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        padding: var(--space-4);
      }
      .skeleton {
        height: var(--space-12);
        background: var(--muted);
        border-radius: var(--radius-md);
        animation: pulse var(--dur-slow) ease-in-out infinite;
      }
      .skeleton.skeleton-shimmer {
        background: linear-gradient(
          90deg,
          var(--muted) 0%,
          color-mix(in oklch, var(--muted) 60%, var(--foreground) 5%) 40%,
          var(--muted) 80%
        );
        background-size: 200% 100%;
        animation: shimmer var(--dur-slow) var(--ease-1) infinite;
      }
      @keyframes pulse {
        0%,
        100% {
          opacity: 1;
        }
        50% {
          opacity: 0.4;
        }
      }
      /* Load more sentinel */
      .sentinel {
        height: var(--space-4);
      }
      .loading-more {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        padding: 0 var(--space-4) var(--space-4);
      }
    `,
  ];

  @state()
  private _unreadOnly = false;

  #signals = new SignalController(this);
  #fetched = false;
  #observer?: IntersectionObserver;
  #sentinelRef = createRef();

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(notifications, unreadCount);
    if (!this.#fetched) {
      this.#fetched = true;
      fetchNotifications(this._unreadOnly);
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#observer?.disconnect();
  }

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

  private _toggleUnread(e: CustomEvent): void {
    this._unreadOnly = (e.detail as { checked: boolean }).checked;
    fetchNotifications(this._unreadOnly);
  }

  private _markAllRead(): void {
    markAllNotificationsRead();
  }

  private _onNotificationClick(e: CustomEvent): void {
    const { id, link } = e.detail;
    markNotificationRead(id);
    if (link) navigate(link);
  }

  protected render() {
    const s = notifications.value;
    const displayed = this._unreadOnly
      ? s.items.filter((n) => !n.is_read)
      : s.items;

    return html`
      <plume-app-layout>
        <div class="page page-enter">
          <div class="header">
            <div>
              <h1>${msg("Inbox")}</h1>
              <p class="header-sub">${msg(
                "Stay up to date with your notifications.",
              )}</p>
            </div>
            <div class="header-actions">
              <label class="toggle-label">
                ${msg("Unread only")}
                <plume-switch
                  .checked="${this._unreadOnly}"
                  @change="${this._toggleUnread}"
                ></plume-switch>
              </label>
              <button
                class="mark-all-btn"
                type="button"
                @click="${this._markAllRead}"
              >
                ${msg("Mark all read")}
              </button>
            </div>
          </div>

          <div class="list">
            ${s.isLoading
              ? html`
                <div class="loading">
                  ${Array.from({ length: 5 }, (_, i) =>
                    html`
                      <div class="skeleton skeleton-shimmer" key="${i}"></div>
                    `)}
                </div>
              `
              : displayed.length === 0
              ? html`
                <div class="empty">
                  <plume-icon
                    class="empty-icon"
                    name="bell"
                    size="48"
                  ></plume-icon>
                  <h2>${msg("All caught up")}</h2>
                  <p>${msg("Notifications will appear here.")}</p>
                </div>
              `
              : html`
                ${displayed.map(
                  (n) =>
                    n && html`
                      <plume-notification-item
                        class="list-item-enter"
                        .itemId="${n.id ?? ""}"
                        .type="${n.type ?? ""}"
                        .title="${n.title ?? ""}"
                        .body="${n.body ?? ""}"
                        .link="${n.project_slug
                          ? `/projects/${n.project_slug}?task=${n.entity_id}`
                          : (n.link ?? "")}"
                        .isRead="${n.is_read ?? false}"
                        .createdAt="${n.created_at ?? ""}"
                        @notification-click="${this._onNotificationClick}"
                      ></plume-notification-item>
                    `,
                )}
              `}
            <div class="sentinel" ${ref(this.#sentinelRef)}></div>
            ${s.isFetchingMore
              ? html`
                <div class="loading-more">
                  <div class="skeleton skeleton-shimmer"></div>
                  <div class="skeleton skeleton-shimmer"></div>
                </div>
              `
              : nothing}
          </div>
        </div>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-inbox-page": PlumeInboxPage;
  }
}
