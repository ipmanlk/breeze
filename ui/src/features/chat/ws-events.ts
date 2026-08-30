import type { DtoMessageResponse, DtoNotificationResponse } from "@/api";

export type ChatWsServerEvent =
  | {
    type: "message_new";
    payload: { message: DtoMessageResponse; conversation_id: string };
  }
  | {
    type: "message_updated";
    payload: { message: DtoMessageResponse; conversation_id: string };
  }
  | {
    type: "message_deleted";
    payload: { message_id: string; conversation_id: string };
  }
  | {
    type: "message_pinned";
    payload: { message: DtoMessageResponse; conversation_id: string };
  }
  | {
    type: "message_unpinned";
    payload: { message_id: string; conversation_id: string };
  }
  | {
    type: "message_reaction_added";
    payload: {
      message_id: string;
      conversation_id: string;
      user_id: string;
      emoji: string;
    };
  }
  | {
    type: "message_reaction_removed";
    payload: {
      message_id: string;
      conversation_id: string;
      user_id: string;
      emoji: string;
    };
  }
  | {
    type: "typing";
    payload: { conversation_id: string; user_id: string; is_typing: boolean };
  }
  | {
    type: "presence_updated";
    payload: { user_id: string; status: "online" | "away" | "offline" | "dnd" };
  }
  | {
    type: "notification_new";
    payload: DtoNotificationResponse;
  };

export const WS_EVENTS = {
  MESSAGE_NEW: "message_new",
  MESSAGE_UPDATED: "message_updated",
  MESSAGE_DELETED: "message_deleted",
  MESSAGE_PINNED: "message_pinned",
  MESSAGE_UNPINNED: "message_unpinned",
  MESSAGE_REACTION_ADDED: "message_reaction_added",
  MESSAGE_REACTION_REMOVED: "message_reaction_removed",
  TYPING: "typing",
  PRESENCE_UPDATED: "presence_updated",
  NOTIFICATION_NEW: "notification_new",
} as const;
