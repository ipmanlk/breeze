import { html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import {
  activeConversation,
  channelPermissions,
  conversationList,
  presence,
  pushWsMessageEvent,
  settingsConvId,
  showChannelSettings,
  showMemberList,
  typingUsers,
} from "../store";
import { chatApi } from "../api";
import { auth } from "@/store/auth";
import { connectionStatus, sendWsMessage, wsClient } from "@/store/ws";
import { handleVoiceWsMessage } from "../../voice/voice-signaling";
import type { Message, UserPresence } from "../types";
import {
  startConversationMonitor,
  stopConversationMonitor,
} from "../conversation-dnd";
import "./workspace-sidebar.ts";
import "./dm-sidebar.ts";
import "./chat-area.ts";
import "./member-list-panel.ts";
import "./channel-settings-panel.ts";
import "./create-category-dialog.ts";
import "./create-channel-dialog.ts";
import "./create-dm-dialog.ts";
import "./chat-search-dialog.ts";
import { localized } from "@lit/localize";

export type ChatMode = "workspace" | "dms";

const CL_STYLES = `
breeze-chat-layout {
  display: flex;
  flex: 1;
  min-height: 0;
  margin: calc(-1 * var(--space-4));
  overflow: hidden;
  background: var(--background);
}
@keyframes cl-main-in {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.cl-main {
  display: flex;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  animation: cl-main-in var(--dur-slow) var(--ease-2);
}
`;

/**
 * Three-column chat layout: sidebar | chat area | member/settings panel.
 *
 * **Light DOM**: required for @atlaskit/pragmatic-drag-and-drop in the
 * workspace sidebar. `breeze-app-layout` keeps its shadow DOM (content is
 * slotted into the light DOM tree), but this component and its parent
 * `breeze-chat-page` must be light DOM so the chain from the sidebar up to
 * the document is unbroken. Styles are global, prefixed `cl-`.
 *
 * Host styles fill the `.content` area of `breeze-app-layout` edge-to-edge:
 * - `flex: 1; min-height: 0` fills the remaining height in the flex column
 * - `margin: calc(-1 * var(--space-4))` cancels `.content`'s padding on all sides
 *
 * WebSocket is connected globally by app-shell (always-on for authenticated
 * users). This component subscribes/unsubscribes to conversation rooms and
 * listens for real-time chat events (message_new, typing, presence, etc).
 *
 * Settings panel state is driven by the `?settings=1` query parameter so it
 * survives refreshes and works with browser back/forward navigation.
 */
@localized()
@customElement("breeze-chat-layout")
export class BreezeChatLayout extends LitElement {
  createRenderRoot() {
    return this;
  }

  @property()
  mode: ChatMode = "workspace";

  @property()
  conversationId = "";

  #signals = new SignalController(this);
  #activeConvId = "";
  #prevPermissionsConvId = "";
  #loaded = false;
  #prevMode: ChatMode = "workspace";
  #prevWs: WebSocket | null = null;
  #wsMessageHandler: ((e: MessageEvent) => void) | null = null;
  #popstateHandler: (() => void) | null = null;
  #typingPruneTimer: ReturnType<typeof setInterval> | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    startConversationMonitor();
    this.#signals.watch(
      conversationList,
      activeConversation,
      showMemberList,
      showChannelSettings,
      settingsConvId,
      presence,
      typingUsers,
      wsClient,
      connectionStatus,
      auth,
    );

    // Sync settings panel state from URL on mount
    this.#syncSettingsFromUrl();

    // Listen for browser back/forward to sync settings state
    this.#popstateHandler = () => this.#syncSettingsFromUrl();
    window.addEventListener("popstate", this.#popstateHandler);

    // Prune stale typing indicators (stop event lost on disconnect)
    this.#typingPruneTimer = setInterval(
      () => this.#pruneTypingIndicators(),
      3000,
    );
  }

  /** Read ?settings=1 from the URL and update the signal. */
  #syncSettingsFromUrl() {
    const params = new URLSearchParams(window.location.search);
    const wantSettings = params.get("settings") === "1";
    if (wantSettings !== showChannelSettings.value) {
      showChannelSettings.value = wantSettings;
      if (wantSettings) {
        // If opening via URL and no explicit settingsConvId, use active conv
        if (!settingsConvId.value) {
          const conv = activeConversation.value;
          if (conv) settingsConvId.value = conv.id;
        }
      } else {
        settingsConvId.value = null;
      }
    }
  }

  /** Push ?settings=1 (or remove it) to the URL without a full navigation. */
  static setSettingsUrl(open: boolean) {
    const url = new URL(window.location.href);
    if (open) {
      url.searchParams.set("settings", "1");
    } else {
      url.searchParams.delete("settings");
    }
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  protected willUpdate(changed: Map<string, unknown>) {
    if (auth.value.isAuthenticated && !this.#loaded) {
      this.#loaded = true;
      this.#loadData();
    }

    // When mode switches (workspace ↔ dms), clear active conversation and reload
    if (changed.has("mode") && this.mode !== this.#prevMode) {
      this.#prevMode = this.mode;
      activeConversation.value = null;
      this.#activeConvId = "";
      this.#loadData();
    }

    this.#syncWsListener();
    this.#syncWsSubscription();
    this.#syncPermissions();
  }

  async #loadData() {
    const scope = this.mode === "dms" ? "dms" : "workspace";
    try {
      const res = await chatApi.listConversations({ scope, limit: 100 });
      conversationList.value = res.items;

      const active = activeConversation.value;
      if (active?.id) {
        const updated = res.items.find((c) => c.id === active.id);
        if (updated) activeConversation.value = updated;
      }
    } catch {
      // ignore
    }

    try {
      const res = await chatApi.listPresence();
      const map: Record<string, UserPresence> = {};
      res.items.forEach((p) => (map[p.user_id] = p));
      presence.value = { ...presence.value, ...map };
    } catch {
      // ignore
    }
  }

  /**
   * Attach/detach a message listener to the global WebSocket.
   * When WS reconnects (new instance), re-attach and re-subscribe.
   */
  #syncWsListener() {
    const ws = wsClient.value;
    if (ws === this.#prevWs) return;

    // Remove old listener
    if (this.#prevWs && this.#wsMessageHandler) {
      this.#prevWs.removeEventListener("message", this.#wsMessageHandler);
    }

    this.#prevWs = ws;

    // Add new listener
    if (ws) {
      this.#wsMessageHandler = (e: MessageEvent) => this.#onWsMessage(e);
      ws.addEventListener("message", this.#wsMessageHandler);

      // Re-subscribe to active conversation on reconnect
      const conv = activeConversation.value;
      if (conv?.id) {
        sendWsMessage({
          type: "conversation_subscribe",
          payload: { conversation_id: conv.id },
        });
        chatApi.markRead(conv.id).catch(() => {});
        this.#activeConvId = conv.id;
      }
    } else {
      this.#activeConvId = "";
    }
  }

  #pruneTypingIndicators() {
    const now = Date.now();
    const maxAge = 6000;
    let changed = false;
    const next: typeof typingUsers.value = {};
    for (const [convId, users] of Object.entries(typingUsers.value)) {
      const fresh = users.filter((u) => now - u.ts < maxAge);
      if (fresh.length !== users.length) changed = true;
      if (fresh.length > 0) next[convId] = fresh;
      else if (users.length > 0) changed = true;
    }
    if (changed) typingUsers.value = next;
  }

  #bumpUnread(convId: string) {
    conversationList.value = conversationList.value.map((c) =>
      c.id === convId ? { ...c, unread_count: (c.unread_count ?? 0) + 1 } : c
    );
  }

  #clearUnread(convId: string) {
    conversationList.value = conversationList.value.map((c) =>
      c.id === convId && (c.unread_count ?? 0) > 0
        ? { ...c, unread_count: 0 }
        : c
    );
  }

  #onWsMessage(e: MessageEvent) {
    try {
      const data = JSON.parse(e.data);
      if (!data || typeof data !== "object") return;
      if (typeof data.type === "string" && data.type.startsWith("voice_")) {
        handleVoiceWsMessage(data);
        return;
      }

      const activeId = activeConversation.value?.id ?? null;

      switch (data.type) {
        case "message_new": {
          const p = data.payload;
          if (!p?.message) return;
          const msg = p.message as Message;
          if (!msg.conversation_id) return;
          if (msg.conversation_id === activeId) {
            pushWsMessageEvent(data.type, p);
          } else {
            this.#bumpUnread(msg.conversation_id);
          }
          break;
        }
        case "message_updated": {
          const p = data.payload;
          if (!p?.message) return;
          // Only the active conversation's list is live; inactive updates
          // are picked up on next open via fetch.
          if (activeId && p.message.conversation_id === activeId) {
            pushWsMessageEvent(data.type, p);
          }
          break;
        }
        case "message_deleted": {
          const p = data.payload;
          if (!p?.conversation_id) return;
          if (activeId && p.conversation_id === activeId) {
            pushWsMessageEvent(data.type, p);
          }
          break;
        }
        case "message_reaction_added":
        case "message_reaction_removed": {
          const p = data.payload;
          if (!p?.conversation_id) return;
          if (activeId && p.conversation_id === activeId) {
            pushWsMessageEvent(data.type, p);
          }
          break;
        }
        case "message_pinned":
        case "message_unpinned": {
          const p = data.payload;
          if (!p?.conversation_id) return;
          if (activeId && p.conversation_id === activeId) {
            pushWsMessageEvent(data.type, p);
          }
          break;
        }
        case "typing": {
          const p = data.payload;
          if (!p?.conversation_id) return;
          // Typing is only shown for the active conversation
          if (activeId && p.conversation_id !== activeId) return;
          if (!activeId) return;
          if (p.user_id === auth.value.user?.id) return;
          const current = typingUsers.value[activeId] || [];
          if (p.is_typing) {
            if (!current.find((t) => t.user_id === p.user_id)) {
              typingUsers.value = {
                ...typingUsers.value,
                [activeId]: [...current, {
                  user_id: p.user_id,
                  ts: Date.now(),
                }],
              };
            } else {
              // Refresh timestamp so expiry is sliding
              typingUsers.value = {
                ...typingUsers.value,
                [activeId]: current.map((t) =>
                  t.user_id === p.user_id ? { ...t, ts: Date.now() } : t
                ),
              };
            }
          } else {
            typingUsers.value = {
              ...typingUsers.value,
              [activeId]: current.filter((t) => t.user_id !== p.user_id),
            };
          }
          break;
        }
        case "presence_updated": {
          const p = data.payload as { user_id?: string; status?: string };
          const uid = p.user_id;
          if (!uid) return;
          const existing = presence.value[uid];
          presence.value = {
            ...presence.value,
            [uid]: {
              user_id: uid,
              org_id: existing?.org_id ?? "",
              status: p.status ?? "offline",
              last_seen: new Date().toISOString(),
              user: existing?.user,
            },
          };
          break;
        }
      }
    } catch {
      // ignore malformed messages
    }
  }

  /**
   * Subscribe/unsubscribe to conversation rooms when active conversation changes.
   */
  #syncWsSubscription() {
    const conv = activeConversation.value;
    const ws = wsClient.value;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;

    if (conv?.id && conv.id !== this.#activeConvId) {
      if (this.#activeConvId) {
        sendWsMessage({
          type: "conversation_unsubscribe",
          payload: { conversation_id: this.#activeConvId },
        });
      }
      sendWsMessage({
        type: "conversation_subscribe",
        payload: { conversation_id: conv.id },
      });
      this.#clearUnread(conv.id);
      chatApi.markRead(conv.id).catch(() => {});
      this.#activeConvId = conv.id;
    }
  }

  #syncPermissions() {
    const conv = activeConversation.value;
    if (!conv?.id || conv.id === this.#prevPermissionsConvId) return;
    this.#prevPermissionsConvId = conv.id;
    chatApi.myPermissions(conv.id).then((perms) => {
      channelPermissions.value = perms;
    }).catch(() => {
      channelPermissions.value = null;
    });
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    stopConversationMonitor();
    // Remove WS message listener
    if (this.#prevWs && this.#wsMessageHandler) {
      this.#prevWs.removeEventListener("message", this.#wsMessageHandler);
    }
    this.#prevWs = null;
    this.#wsMessageHandler = null;

    if (this.#typingPruneTimer) {
      clearInterval(this.#typingPruneTimer);
      this.#typingPruneTimer = null;
    }

    // Remove popstate listener
    if (this.#popstateHandler) {
      window.removeEventListener("popstate", this.#popstateHandler);
      this.#popstateHandler = null;
    }

    // Unsubscribe from conversation room
    if (this.#activeConvId) {
      sendWsMessage({
        type: "conversation_unsubscribe",
        payload: { conversation_id: this.#activeConvId },
      });
    }
  }

  protected render() {
    const activeConv = activeConversation.value;
    const membersOpen = showMemberList.value;
    const settingsOpen = showChannelSettings.value;

    return html`
      <style>
      ${CL_STYLES}
      </style>
      ${this.mode === "workspace"
        ? html`
          <breeze-workspace-sidebar></breeze-workspace-sidebar>
        `
        : html`
          <breeze-dm-sidebar></breeze-dm-sidebar>
        `}
      <div class="cl-main">
        <breeze-chat-area></breeze-chat-area>
        ${settingsOpen && (settingsConvId.value || activeConv)
          ? html`
            <breeze-channel-settings-panel
              .conversationId="${settingsConvId.value ?? activeConv?.id ?? ""}"
            ></breeze-channel-settings-panel>
          `
          : membersOpen && activeConv
          ? html`
            <breeze-member-list-panel
              conversationId="${activeConv.id}"
              .presence="${presence.value}"
              @close="${() => {
                showMemberList.value = false;
              }}"
            ></breeze-member-list-panel>
          `
          : ""}
      </div>

      <breeze-create-category-dialog></breeze-create-category-dialog>
      <breeze-create-channel-dialog></breeze-create-channel-dialog>
      <breeze-create-dm-dialog></breeze-create-dm-dialog>
      <breeze-chat-search-dialog></breeze-chat-search-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-chat-layout": BreezeChatLayout;
  }
}
