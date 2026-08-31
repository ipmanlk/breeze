import { localized, msg } from "@lit/localize";
import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { listItemEnterStyles } from "@/styles/shared-animations";
import { timeAgoShort } from "@/lib/format/time-ago";
import "../../components/ui/plume-icon.ts";

/** Map notification type → lucide icon name. */
const TYPE_ICON: Record<string, string> = {
  task_assigned: "user-plus",
  task_status_changed: "arrow-right-left",
  task_comment: "message-square",
  mention: "at-sign",
  chat_dm: "message-square",
  chat_mention: "at-sign",
  task_due_soon: "clock",
  task_overdue: "alert-triangle",
  project_invited: "user-plus",
  project_role_changed: "shield",
  cycle_started: "play-circle",
  cycle_ending: "alert-circle",
};

/**
 * Map notification type → localized human-readable label.
 * Called at render time (not module-level) so msg() re-evaluates on locale
 * change. Used for notification preference section headers, filter labels, etc.
 */
export function getNotificationLabel(type: string): string {
  switch (type) {
    case "task_assigned":
      return msg("Task assignments");
    case "task_status_changed":
      return msg("Task status changes");
    case "task_comment":
      return msg("Comments");
    case "task_due_soon":
      return msg("Tasks due soon");
    case "task_overdue":
      return msg("Overdue tasks");
    case "chat_dm":
      return msg("Direct messages");
    case "chat_mention":
      return msg("Chat mentions");
    default:
      return msg("Notifications");
  }
}

/** Types where the icon should use warning color instead of muted. */
const WARNING_TYPES = new Set(["task_overdue"]);

/**
 * Notification list item.
 *
 * Shows an unread dot, type icon, title, optional body, and relative time.
 */
@localized()
@customElement("plume-notification-item")
export class PlumeNotificationItem extends LitElement {
  static styles = [
    listItemEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: block;
      }
      .item {
        display: flex;
        align-items: flex-start;
        gap: var(--space-3);
        width: 100%;
        padding: var(--space-2) var(--space-3);
        padding-top: var(--space-2-5);
        padding-bottom: var(--space-2-5);
        border: none;
        background: transparent;
        color: var(--foreground);
        text-align: left;
        cursor: pointer;
        transition:
          background var(--dur-fast) var(--ease-1),
          opacity var(--dur-normal) var(--ease-1);
      }
      .item[data-read] {
        opacity: 0.6;
      }
      .item.list-item-enter {
        animation: list-item-in var(--dur-normal) var(--ease-2);
      }
      .item:hover {
        background: color-mix(in oklch, var(--accent) 50%, transparent);
      }
      .dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: var(--radius-full);
        flex-shrink: 0;
        margin-top: var(--space-1-5);
      }
      .dot.unread {
        background: var(--primary);
      }
      .dot.read {
        background: transparent;
      }
      .icon {
        margin-top: var(--space-0-5);
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .icon.warning {
        color: var(--warning);
      }
      .content {
        display: flex;
        flex-direction: column;
        gap: var(--space-0-5);
        min-width: 0;
        flex: 1;
      }
      .title {
        font-size: var(--text-sm);
        line-height: var(--leading-tight);
      }
      .title.unread {
        font-weight: 600;
      }
      .body {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        overflow: hidden;
        display: -webkit-box;
        -webkit-line-clamp: 2;
        -webkit-box-orient: vertical;
        line-height: var(--leading-normal);
      }
      .time {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
    `,
  ];

  @property()
  itemId = "";
  @property()
  type = "";
  @property()
  title = "";
  @property()
  body = "";
  @property()
  link = "";
  @property({ type: Boolean })
  isRead = false;
  @property()
  createdAt = "";

  private _onClick(): void {
    this.dispatchEvent(
      new CustomEvent("notification-click", {
        detail: { id: this.itemId, link: this.link },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    const iconName = TYPE_ICON[this.type] ?? "bell";
    const isWarning = WARNING_TYPES.has(this.type);

    return html`
      <button type="button" class="item" @click="${this._onClick}">
        <span class="dot ${this.isRead ? "read" : "unread"}"></span>
        <plume-icon
          class="icon ${isWarning ? "warning" : ""}"
          name="${iconName}"
          size="16"
        ></plume-icon>
        <div class="content">
          <span class="title ${this.isRead ? "" : "unread"}">
            ${this.title}
          </span>
          ${this.body
            ? html`
              <span class="body">${this.body}</span>
            `
            : null}
          <span class="time">${timeAgoShort(this.createdAt)}</span>
        </div>
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-notification-item": PlumeNotificationItem;
  }
}
