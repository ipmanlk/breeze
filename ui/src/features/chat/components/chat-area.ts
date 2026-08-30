import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import {
  activeConversation,
  channelPermissions,
  conversationList,
  editMessage,
  presence,
  pushWsMessageEvent,
  replyToMessage,
  settingsConvId,
  showChannelSettings,
  showChatSearch,
  showMemberList,
  typingUsers,
} from "../store";
import { chatApi } from "../api";
import { auth } from "@/store/auth";
import type { Message } from "../types";
import { getConvDisplayName } from "../utils";
import "./message-list.ts";
import "./message-input.ts";
import "./typing-indicator.ts";
import "./context-banners.ts";
import "./pinned-messages-bar.ts";
import "../../voice/components/voice-channel-view.ts";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-chat-area")
export class BreezeChatArea extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-width: 0;
      overflow: hidden;
    }
    .empty {
      display: flex;
      flex: 1;
      align-items: center;
      justify-content: center;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: var(--topbar-h);
      padding: 0 var(--space-4);
      border-bottom: 1px solid var(--border);
      background: var(--background);
      flex-shrink: 0;
    }
    .header-left {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      min-width: 0;
    }
    .header-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      color: var(--muted-foreground);
      flex-shrink: 0;
    }
    .header-name {
      font-size: var(--text-sm);
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .header-topic {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      max-width: var(--space-48);
    }
    .header-divider {
      color: var(--muted-foreground);
      opacity: 0.5;
    }
    .header-member-count {
      font-size: var(--text-2xs);
      padding: 0 var(--space-1-5);
      height: var(--space-5);
      display: inline-flex;
      align-items: center;
      border-radius: var(--radius-full);
      background: var(--secondary);
      color: var(--secondary-foreground);
      white-space: nowrap;
      flex-shrink: 0;
    }
    .header-actions {
      display: flex;
      align-items: center;
      gap: var(--space-1);
      flex-shrink: 0;
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
      color: var(--muted-foreground);
      cursor: pointer;
    }
    .header-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .header-rename-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-6);
      height: var(--space-6);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
      opacity: 0;
      transition: opacity var(--dur-fast) var(--ease-1);
    }
    .header:hover .header-rename-btn {
      opacity: 1;
    }
    .header-rename-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    /* messages + input wrapper: fills the remaining height, scrolls internally */
    .content-wrap {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }
    /* messages scroll area: scrolls, keeps input pinned below */
    .messages-area {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }
    .typing-wrap {
      position: absolute;
      bottom: var(--space-3);
      left: 50%;
      z-index: 10;
      transform: translateX(-50%);
      pointer-events: none;
    }
    /* input area: fixed at the bottom, never shrinks */
    .input-area {
      flex-shrink: 0;
      padding: var(--space-1) var(--space-3) var(--space-3);
      border-top: 1px solid var(--border);
      background: var(--background);
    }
  `;

  #signals = new SignalController(this);

  @state()
  private _renameOpen = false;

  @state()
  private _renameValue = "";

  @state()
  private _renameSaving = false;

  @state()
  private _renameError = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(
      channelPermissions,
      activeConversation,
      editMessage,
      replyToMessage,
      presence,
      typingUsers,
      auth,
    );
  }

  private _onReplyTo(msg: Message) {
    replyToMessage.value = msg;
  }

  private _onEditMessage(msg: Message) {
    editMessage.value = msg;
  }

  private _onMessageEdited(e: CustomEvent) {
    const { message } = e.detail;
    if (!message) return;
    // Push into the event log so message-list updates immediately
    pushWsMessageEvent("message_updated", { message });
  }

  private _onDeleteMessage(message: Message) {
    if (!message) return;
    if (confirm(msg("Delete this message?"))) {
      chatApi.deleteMessage(message.conversation_id, message.id).catch(
        () => {},
      );
    }
  }

  private async _onPinMessage(msg: Message) {
    const wasPinned = msg.pinned;
    try {
      if (wasPinned) {
        await chatApi.unpinMessage(msg.conversation_id, msg.id);
      } else {
        await chatApi.pinMessage(msg.conversation_id, msg.id);
      }
      // Update message pinned state in the list + reload pinned bar
      pushWsMessageEvent(
        wasPinned ? "message_unpinned" : "message_pinned",
        {
          conversation_id: msg.conversation_id,
          message_id: msg.id,
          message: { ...msg, pinned: !wasPinned },
        },
      );
    } catch {
      // ignore
    }
  }

  private _onReaction(e: CustomEvent) {
    const { message, emoji } = e.detail;
    if (!message || !emoji) return;

    const existing = message.reactions?.find(
      (r: { emoji: string; mine: boolean }) => r.emoji === emoji && r.mine,
    );
    if (existing) {
      chatApi.removeReaction(message.conversation_id, message.id, emoji).catch(
        () => {},
      );
    } else {
      chatApi.addReaction(message.conversation_id, message.id, emoji).catch(
        () => {},
      );
    }
  }

  private _onToggleMembers() {
    showMemberList.value = !showMemberList.value;
  }

  private _onOpenSettings() {
    const conv = activeConversation.value;
    if (!conv) return;
    settingsConvId.value = conv.id;
    showChannelSettings.value = true;
    // Update URL to reflect settings panel is open
    const url = new URL(window.location.href);
    url.searchParams.set("settings", "1");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  private _onSearch() {
    showChatSearch.value = true;
  }

  private _onRenameClick() {
    const conv = activeConversation.value;
    if (!conv) return;
    this._renameValue = conv.name || "";
    this._renameError = "";
    this._renameOpen = true;
  }

  private async _onRenameSubmit(e: Event) {
    e.preventDefault();
    const conv = activeConversation.value;
    if (!conv) return;
    const name = this._renameValue.trim();
    if (!name) {
      this._renameError = "Name is required.";
      return;
    }
    this._renameSaving = true;
    this._renameError = "";
    try {
      const updated = await chatApi.updateConversation(conv.id, { name });
      conversationList.value = conversationList.value.map((c) =>
        c.id === conv.id ? { ...c, ...updated } : c
      );
      activeConversation.value = { ...conv, ...updated };
      this._renameOpen = false;
    } catch (err: unknown) {
      this._renameError = err instanceof Error
        ? err.message
        : msg("Failed to rename.");
    } finally {
      this._renameSaving = false;
    }
  }

  protected render() {
    const conv = activeConversation.value;
    if (!conv) {
      return html`
        <div class="empty">Select a channel to start chatting</div>
      `;
    }

    const typingData = typingUsers.value[conv.id] || [];
    const typingNames = typingData
      .map((t) => presence.value[t.user_id]?.user?.name || t.user_id)
      .slice(0, 3);
    const editMsg = editMessage.value;
    const replyMsg = replyToMessage.value;
    const user = auth.value.user;

    const perms = channelPermissions.value;
    const convIcon = conv.type === "voice" ? "volume-2" : "hash";

    return html`
      <div class="header">
        <div class="header-left">
          <span class="header-icon">
            <breeze-icon name="${convIcon}" size="16"></breeze-icon>
          </span>
          <span class="header-name">${getConvDisplayName(conv)}</span>
          ${(conv.type === "direct" || conv.type === "group") &&
              perms?.can_manage
            ? html`
              <button
                class="header-rename-btn"
                @click="${this._onRenameClick}"
                title="Rename conversation"
                aria-label=${msg("Rename conversation")}
              >
                <breeze-icon name="pencil" size="12"></breeze-icon>
              </button>
            `
            : nothing} ${conv.topic
            ? html`
              <span class="header-divider">·</span>
              <span class="header-topic">${conv.topic}</span>
            `
            : nothing}
          <span class="header-member-count">${conv
            .member_count} ${conv.member_count === 1
            ? msg("member")
            : msg("members")}</span>
        </div>
        <div class="header-actions">
          <button
            class="header-btn"
            @click="${this._onSearch}"
            title="Search messages"
            aria-label=${msg("Search messages")}
          >
            <breeze-icon name="search" size="16"></breeze-icon>
          </button>
          <button
            class="header-btn"
            @click="${this._onToggleMembers}"
            title="Members"
            aria-label=${msg("Members")}
          >
            <breeze-icon name="users" size="16"></breeze-icon>
          </button>
          ${perms?.can_manage
            ? html`
              <button
                class="header-btn"
                @click="${this._onOpenSettings}"
                title="Channel settings"
                aria-label=${msg("Channel settings")}
              >
                <breeze-icon name="settings" size="16"></breeze-icon>
              </button>
            `
            : nothing}
        </div>
      </div>

      <breeze-pinned-messages-bar
        conversationId="${conv.id}"
        conversationType="${conv.type}"
        currentUserId="${user?.id || ""}"
        @pin="${(e: CustomEvent) => this._onPinMessage(e.detail)}"
        @edit="${(e: CustomEvent) => this._onEditMessage(e.detail)}"
        @delete="${(e: CustomEvent) => this._onDeleteMessage(e.detail)}"
        @reply="${(e: CustomEvent) => this._onReplyTo(e.detail)}"
        @reaction="${this._onReaction}"
      ></breeze-pinned-messages-bar>

      ${conv.type === "voice" && perms?.can_send !== false
        ? html`
          <breeze-voice-channel-view
            conversationId="${conv.id}"
            conversationName="${conv.name}"
          ></breeze-voice-channel-view>
        `
        : nothing}

      <div class="content-wrap">
        <div class="messages-area">
          <breeze-message-list
            conversationId="${conv.id}"
            currentUserId="${user?.id || ""}"
            @reaction="${this._onReaction}"
            @reply="${(e: CustomEvent) => this._onReplyTo(e.detail)}"
            @edit="${(e: CustomEvent) => this._onEditMessage(e.detail)}"
            @delete="${(e: CustomEvent) => this._onDeleteMessage(e.detail)}"
            @pin="${(e: CustomEvent) => this._onPinMessage(e.detail)}"
          ></breeze-message-list>

          ${typingNames.length > 0
            ? html`
              <div class="typing-wrap">
                <breeze-typing-indicator
                  .names="${typingNames}"
                ></breeze-typing-indicator>
              </div>
            `
            : nothing}
        </div>

        <div class="input-area">
          ${editMsg
            ? html`
              <breeze-edit-banner
                @cancel="${() => {
                  editMessage.value = null;
                }}"
              ></breeze-edit-banner>
            `
            : nothing} ${replyMsg
            ? html`
              <breeze-reply-banner
                .message="${replyMsg}"
                @cancel="${() => {
                  replyToMessage.value = null;
                }}"
              ></breeze-reply-banner>
            `
            : nothing} ${perms?.can_send !== false
            ? html`
              <breeze-message-input
                .conversation="${conv}"
                @edited="${this._onMessageEdited}"
              ></breeze-message-input>
            `
            : html`
              <div
                style="padding:var(--space-3);text-align:center;font-size:var(--text-sm);color:var(--muted-foreground)"
              >
                You don't have permission to send messages in this channel.
              </div>
            `}
        </div>
      </div>

      ${this._renameOpen
        ? html`
          <breeze-dialog
            style="--dialog-w:24rem"
            .open="${true}"
            heading="Rename conversation"
            @close="${() => {
              this._renameOpen = false;
            }}"
          >
            <div style="display:flex;flex-direction:column;gap:var(--space-2)">
              <breeze-input
                placeholder=${msg("Conversation name")}
                .value="${this._renameValue}"
                maxlength="100"
                autofocus
                @input="${(e: Event) => {
                  this._renameValue = (e.target as HTMLInputElement).value;
                }}"
              ></breeze-input>
              ${this._renameError
                ? html`
                  <p style="font-size:var(--text-xs);font-weight:500;color:var(--destructive)">
                    ${this._renameError}
                  </p>
                `
                : nothing}
            </div>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <breeze-button variant="ghost" type="button" @click="${() => {
                this._renameOpen = false;
              }}">
                Cancel
              </breeze-button>
              <breeze-button ?disabled="${this._renameSaving ||
                !this._renameValue.trim()}" @click="${this._onRenameSubmit}">
                ${this._renameSaving ? "Saving..." : "Save"}
              </breeze-button>
            </div>
          </breeze-dialog>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-chat-area": BreezeChatArea;
  }
}
