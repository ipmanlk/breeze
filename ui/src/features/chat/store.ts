import { signal } from "@preact/signals-core";
import type { Conversation, Message, UserPresence } from "./types";

export const activeConversation = signal<Conversation | null>(null);

export interface ChannelPermissions {
  can_view: boolean;
  can_send: boolean;
  can_manage: boolean;
  can_permissions: boolean;
}

export const channelPermissions = signal<ChannelPermissions | null>(null);
export const conversationList = signal<Conversation[]>([]);
export const typingUsers = signal<
  Record<string, { user_id: string; ts: number }[]>
>({});
export const replyToMessage = signal<Message | null>(null);
export const editMessage = signal<Message | null>(null);
export const presence = signal<Record<string, UserPresence>>({});
export const showMemberList = signal(true);

export const showChannelSettings = signal(false);
export const settingsConvId = signal<string | null>(null);

export const showCreateChannel = signal<
  { open: boolean; categoryId: string | null }
>({
  open: false,
  categoryId: null,
});
export const showCreateDm = signal(false);
export const showCreateCategory = signal(false);
export const showForwardDialog = signal<
  { open: boolean; message: Message | null }
>({
  open: false,
  message: null,
});
export const showChatSearch = signal(false);
export const highlightMessageId = signal<string | null>(null);

/**
 * Incoming WS message events for the active conversation.
 * `seq` is a process-wide monotonic counter so consumers can track what
 * they've processed even when the ring buffer trims old entries.
 */
export interface WsMessageEvent {
  seq: number;
  type: string;
  payload: Record<string, unknown>;
}

/** Ring-buffer cap: consumers only need recent events; unbounded growth
 * leaks memory in long-lived sessions. */
const WS_MESSAGE_EVENTS_MAX = 200;

let wsMessageSeq = 0;
export const wsMessageEvents = signal<WsMessageEvent[]>([]);

/** Append an event, trimming the buffer to the most recent entries. */
export function pushWsMessageEvent(
  type: string,
  payload: Record<string, unknown>,
): void {
  const next = [
    ...wsMessageEvents.value,
    { seq: ++wsMessageSeq, type, payload },
  ];
  wsMessageEvents.value = next.length > WS_MESSAGE_EVENTS_MAX
    ? next.slice(-WS_MESSAGE_EVENTS_MAX)
    : next;
}
