import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { OutsideClickController } from "@/lib/outside-click-controller";
import {
  activeConversation,
  conversationList,
  presence,
  showCreateDm,
} from "../store";
import { type Conversation } from "../types";
import { chatApi } from "../api";
import { navigate } from "@/routes/router";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import { localized, msg } from "@lit/localize";

function getDmName(conv: Conversation): string {
  return conv.name || conv.partner_name || "";
}

const DS_STYLES = css`
  *,
  *::before,
  *::after {
    box-sizing: border-box;
  }
  :host {
    display: flex;
    flex-direction: column;
    width: var(--space-64);
    border-right: 1px solid var(--border);
    background: var(--sidebar);
    color: var(--sidebar-foreground);
    flex-shrink: 0;
    overflow: hidden;
  }
  .header {
    display: flex;
    align-items: center;
    height: var(--space-12);
    padding: 0 var(--space-3);
    border-bottom: 1px solid var(--sidebar-border);
  }
  .header-title {
    font-size: var(--text-sm);
    font-weight: 600;
    flex: 1;
  }
  .header-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--space-7);
    height: var(--space-7);
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--sidebar-foreground);
    cursor: pointer;
  }
  .header-btn:hover {
    background: var(--sidebar-accent);
  }
  .section-label {
    display: flex;
    align-items: center;
    height: var(--space-6);
    padding: 0 var(--space-3);
    font-size: var(--text-2xs, 0.6875rem);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted-foreground);
  }
  .scroll-area {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-1) var(--space-2);
  }
  .dm-row {
    position: relative;
  }
  .dm-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: var(--control-h-sm);
    padding: 0 var(--space-2);
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--sidebar-foreground);
    font-size: var(--text-sm);
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background var(--dur-fast) var(--ease-1);
  }
  .dm-item:hover {
    background: var(--sidebar-accent);
  }
  .dm-item[data-active] {
    background: var(--sidebar-accent);
  }
  .dm-more-btn {
    position: absolute;
    right: var(--space-1);
    top: 50%;
    transform: translateY(-50%);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--space-5);
    height: var(--space-5);
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--muted-foreground);
    cursor: pointer;
    opacity: 0;
    flex-shrink: 0;
  }
  .dm-row:hover .dm-more-btn,
  .dm-more-btn.open {
    opacity: 1;
  }
  .dm-more-btn:hover {
    background: var(--sidebar-accent);
    color: var(--sidebar-foreground);
  }
  .dm-menu {
    position: absolute;
    right: 0;
    top: calc(100% + var(--space-0-5));
    z-index: var(--z-dropdown, 50);
    min-width: var(--space-40);
    padding: var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--popover);
    color: var(--popover-foreground);
    box-shadow: var(--shadow-md);
  }
  .dm-menu-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-1-5) var(--space-2);
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--foreground);
    font-size: var(--text-sm);
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    transition: background var(--dur-fast) var(--ease-1);
  }
  .dm-menu-item:hover {
    background: var(--accent);
  }
  .presence-dot {
    width: var(--space-2);
    height: var(--space-2);
    border-radius: 50%;
    flex-shrink: 0;
  }
  .presence-dot.online {
    background: var(--primary);
  }
  .presence-dot.away {
    background: var(--warning);
  }
  .presence-dot.offline {
    background: var(--muted-foreground);
    opacity: 0.4;
  }
  .dm-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dm-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: var(--space-5);
    height: var(--space-5);
    padding: 0 var(--space-1-5);
    border-radius: var(--radius-full);
    background: var(--primary);
    color: var(--primary-foreground);
    font-size: var(--text-2xs);
    font-weight: 600;
    flex-shrink: 0;
  }
  .empty-hint {
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
    color: var(--muted-foreground);
  }
  .ds-dlg-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .ds-error {
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--destructive);
  }
`;

@localized()
@customElement("plume-dm-sidebar")
export class PlumeDmSidebar extends LitElement {
  static styles = DS_STYLES;

  #signals = new SignalController(this);

  @state()
  private _menuConvId: string | null = null;

  @state()
  private _renameConv: Conversation | null = null;

  @state()
  private _renameValue = "";

  @state()
  private _saving = false;

  @state()
  private _error = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(conversationList, activeConversation, presence);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onKeydown);
  }

  private _selectConv(conv: Conversation) {
    navigate(`/messages/${conv.id}`);
  }

  private _onCreateDm() {
    showCreateDm.value = true;
  }

  private _getStatus(userId: string): string {
    return presence.value[userId]?.status || "offline";
  }

  private _toggleMenu(e: Event, convId: string) {
    e.stopPropagation();
    const wasOpen = this._menuConvId === convId;
    this._menuConvId = wasOpen ? null : convId;
    if (!wasOpen) {
      this._outsideClick.connect();
      document.addEventListener("keydown", this._onKeydown);
    } else {
      this._outsideClick.disconnect();
      document.removeEventListener("keydown", this._onKeydown);
    }
  }

  private _outsideClick = new OutsideClickController(this, () => {
    this._menuConvId = null;
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onKeydown);
  });

  private _onKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      this._menuConvId = null;
      this._outsideClick.disconnect();
      document.removeEventListener("keydown", this._onKeydown);
    }
  };

  private _openRename(conv: Conversation) {
    this._menuConvId = null;
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onKeydown);
    this._renameConv = conv;
    this._renameValue = conv.name || "";
    this._error = "";
  }

  private _closeRename() {
    this._renameConv = null;
    this._renameValue = "";
    this._error = "";
  }

  private async _onRenameSubmit(e: Event) {
    e.preventDefault();
    const name = this._renameValue.trim();
    if (!name) {
      this._error = "Name is required.";
      return;
    }
    const conv = this._renameConv;
    if (!conv) return;
    this._saving = true;
    this._error = "";
    try {
      const updated = await chatApi.updateConversation(conv.id, { name });
      conversationList.value = conversationList.value.map((c) =>
        c.id === conv.id ? { ...c, ...updated } : c
      );
      if (activeConversation.value?.id === conv.id) {
        activeConversation.value = { ...activeConversation.value, ...updated };
      }
      this._closeRename();
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to rename.");
    } finally {
      this._saving = false;
    }
  }

  protected render() {
    const convs = conversationList.value;
    const activeId = activeConversation.value?.id;

    const dmConvs = convs.filter((c) =>
      c.type === "direct" || c.type === "group"
    )
      .sort((a, b) =>
        (b.last_message?.created_at || b.created_at).localeCompare(
          a.last_message?.created_at || a.created_at,
        )
      );

    return html`
      <div class="header">
        <span class="header-title">Direct Messages</span>
        <button
          class="header-btn"
          @click="${this._onCreateDm}"
          title="${msg("New message")}"
          aria-label=${msg("New message")}
        >
          <plume-icon name="plus" size="16"></plume-icon>
        </button>
      </div>

      <div class="scroll-area">
        <div class="section-label">Messages</div>
        ${dmConvs.length === 0
          ? html`
            <div class="empty-hint">No conversations yet. Start a new message!</div>
          `
          : dmConvs.map(
            (conv) =>
              html`
                <div class="dm-row">
                  <button
                    class="dm-item"
                    ?data-active="${activeId === conv.id}"
                    @click="${() => this._selectConv(conv)}"
                  >
                    <span class="presence-dot ${this._getStatus(
                      conv.partner_user_id || "",
                    )}"></span>
                    <span class="dm-name">${getDmName(conv)}</span>
                    ${conv.unread_count > 0
                      ? html`
                        <span class="dm-badge">${conv.unread_count > 99
                          ? "99+"
                          : conv.unread_count}</span>
                      `
                      : ""}
                  </button>
                  <button
                    type="button"
                    class="dm-more-btn ${this._menuConvId === conv.id
                      ? "open"
                      : ""}"
                    aria-label=${msg("Conversation actions")}
                    @click="${(e: Event) => this._toggleMenu(e, conv.id)}"
                  >
                    <plume-icon name="more-vertical" size="14"></plume-icon>
                  </button>
                  ${this._menuConvId === conv.id
                    ? html`
                      <div class="dm-menu">
                        <button class="dm-menu-item" @click="${() =>
                          this._openRename(conv)}">
                          <plume-icon name="pencil" size="14"></plume-icon>
                          Rename
                        </button>
                      </div>
                    `
                    : nothing}
                </div>
              `,
          )}
      </div>

      ${this._renameConv
        ? html`
          <plume-dialog
            style="--dialog-w:24rem"
            .open="${true}"
            heading="Rename conversation"
            @close="${this._closeRename}"
          >
            <div class="ds-dlg-body">
              <plume-input
                placeholder=${msg("Conversation name")}
                .value="${this._renameValue}"
                maxlength="100"
                autofocus
                @input="${(e: Event) => {
                  this._renameValue = (e.target as HTMLInputElement).value;
                }}"
              ></plume-input>
              ${this._error
                ? html`
                  <p class="ds-error">${this._error}</p>
                `
                : nothing}
            </div>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <plume-button variant="ghost" type="button" @click="${this
                ._closeRename}">
                Cancel
              </plume-button>
              <plume-button ?disabled="${this._saving ||
                !this._renameValue.trim()}" @click="${this._onRenameSubmit}">
                ${this._saving ? "Saving..." : "Save"}
              </plume-button>
            </div>
          </plume-dialog>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-dm-sidebar": PlumeDmSidebar;
  }
}
