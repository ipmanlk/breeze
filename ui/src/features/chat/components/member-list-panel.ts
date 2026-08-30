import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import type { Member, UserPresence } from "../types";
import { chatApi } from "../api";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-member-list-panel")
export class BreezeMemberListPanel extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      width: var(--space-60);
      border-left: 1px solid var(--border);
      background: var(--background);
      flex-shrink: 0;
      overflow: hidden;
      animation: slide-in-right var(--dur-slow) var(--ease-2);
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: var(--topbar-h);
      padding: 0 var(--space-4);
      border-bottom: 1px solid var(--border);
      font-size: var(--text-xs);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted-foreground);
      flex-shrink: 0;
    }
    .close-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-6);
      height: var(--space-6);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
    }
    .close-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .scroll {
      flex: 1;
      overflow-y: auto;
      padding: var(--space-2);
    }
    .group-label {
      padding: var(--space-1) var(--space-2);
      font-size: var(--text-2xs, 0.6875rem);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted-foreground);
    }
    .member {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-1-5) var(--space-2);
      border-radius: var(--radius-md);
      transition: background var(--dur-fast) var(--ease-1);
    }
    .member:hover {
      background: color-mix(in oklch, var(--accent) 40%, transparent);
    }
    .avatar-wrap {
      position: relative;
      flex-shrink: 0;
    }
    .avatar {
      width: var(--space-6);
      height: var(--space-6);
      border-radius: var(--radius-full);
      background: var(--muted);
      color: var(--muted-foreground);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-xs);
      font-weight: 600;
    }
    .presence-dot {
      position: absolute;
      bottom: -1px;
      right: -1px;
      width: var(--space-2-5);
      height: var(--space-2-5);
      border-radius: 50%;
      border: 2px solid var(--background);
    }
    .presence-dot.online {
      background: oklch(0.72 0.17 149);
    }
    .presence-dot.away {
      background: oklch(0.75 0.16 80);
    }
    .presence-dot.dnd {
      background: oklch(0.58 0.2 25);
    }
    .presence-dot.offline {
      background: var(--muted-foreground);
      opacity: 0.4;
    }
    .member-name {
      flex: 1;
      font-size: var(--text-sm);
      color: var(--foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  `;

  @property()
  conversationId = "";

  @property({ type: Object, attribute: false })
  presence: Record<string, UserPresence> = {};

  @state()
  private _members: Member[] = [];

  @state()
  private _loading = true;

  protected willUpdate(changed: Map<string, unknown>) {
    if (changed.has("conversationId") && this.conversationId) {
      this._loading = true;
      this.#loadMembers();
    }
  }

  async #loadMembers() {
    try {
      const members = await chatApi.listMembers(this.conversationId);
      this._members = members;
    } catch {
      this._members = [];
    }
    this._loading = false;
  }

  private _onClose() {
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  private _getStatus(userId: string): string {
    return this.presence[userId]?.status || "offline";
  }

  protected render() {
    const online = this._members.filter((m) => {
      const s = this._getStatus(m.id);
      return s === "online" || s === "away";
    });
    const offline = this._members.filter((m) => {
      const s = this._getStatus(m.id);
      return s !== "online" && s !== "away";
    });

    return html`
      <div class="header">
        <span>Members: ${this._members.length}</span>
        <button
          class="close-btn"
          @click="${this._onClose}"
          aria-label=${msg("Close members panel")}
        >
          <breeze-icon name="x" size="14"></breeze-icon>
        </button>
      </div>
      <div class="scroll">
        ${this._loading
          ? html`
            <div class="group-label">Loading...</div>
          `
          : nothing} ${!this._loading && online.length > 0
          ? html`
            <div class="group-label">Online: ${online.length}</div>
            ${online.map((m) => this.#renderMember(m))}
          `
          : nothing} ${!this._loading && offline.length > 0
          ? html`
            <div class="group-label">Offline: ${offline.length}</div>
            ${offline.map((m) => this.#renderMember(m))}
          `
          : nothing}
      </div>
    `;
  }

  #renderMember(m: Member) {
    const status = this._getStatus(m.id);
    const dotClass = status === "online"
      ? "online"
      : status === "away"
      ? "away"
      : status === "dnd"
      ? "dnd"
      : "offline";

    return html`
      <div class="member">
        <div class="avatar-wrap">
          <div class="avatar">${m.name.charAt(0).toUpperCase()}</div>
          <span class="presence-dot ${dotClass}"></span>
        </div>
        <span class="member-name">${m.name}</span>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-member-list-panel": BreezeMemberListPanel;
  }
}
