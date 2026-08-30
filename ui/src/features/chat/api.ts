import type {
  Conversation,
  Member,
  MentionResult,
  Message,
  UserPresence,
} from "./types";
import { identify } from "@/lib/sdk-helpers";
import {
  deleteConversationsById,
  deleteConversationsByIdMembersByUserId,
  deleteConversationsByIdMessagesByMsgId,
  deleteConversationsByIdMessagesByMsgIdPin,
  deleteConversationsByIdMessagesByMsgIdReactionsByEmoji,
  getChatPresence,
  getConversations,
  getConversationsById,
  getConversationsByIdAccess,
  getConversationsByIdMembers,
  getConversationsByIdMessages,
  getConversationsByIdMessagesByMsgIdReplies,
  getConversationsByIdMyPermissions,
  getConversationsByIdPermissions,
  getConversationsByIdPinned,
  getConversationsByIdProjects,
  getConversationsByIdUserOverrides,
  getConversationsSearch,
  getMentionsSearch,
  patchConversationsById,
  patchConversationsByIdMessagesByMsgId,
  patchConversationsByIdMute,
  patchConversationsByIdNotificationLevel,
  patchConversationsByIdPosition,
  postConversations,
  postConversationsByIdAttachments,
  postConversationsByIdMembers,
  postConversationsByIdMessages,
  postConversationsByIdMessagesByMsgIdPin,
  postConversationsByIdMessagesByMsgIdReactions,
  postConversationsByIdRead,
  putChatPresenceMe,
  putConversationsByIdPermissions,
  putConversationsByIdProjects,
  putConversationsByIdUserOverrides,
} from "@/api";

function toError(err: unknown): Error {
  if (err instanceof Error) return err;
  const obj = err as Record<string, unknown>;
  if (obj?.error && typeof obj.error === "object") {
    const e = obj.error as Record<string, unknown>;
    if (typeof e.message === "string") return new Error(e.message);
  }
  if (typeof obj?.message === "string") return new Error(obj.message);
  return new Error("Request failed");
}

export const chatApi = {
  async listPresence(): Promise<{ items: UserPresence[] }> {
    try {
      const { data } = await getChatPresence({ throwOnError: true });
      return identify<{ items: UserPresence[] }>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async setMyPresence(status: "online" | "away" | "offline"): Promise<void> {
    try {
      await putChatPresenceMe({ body: { status }, throwOnError: true });
    } catch (err) {
      throw toError(err);
    }
  },

  async listConversations(
    params?: { scope?: string; cursor?: string; limit?: number },
  ): Promise<
    { items: Conversation[]; next_cursor: string; has_more: boolean }
  > {
    try {
      const { data } = await getConversations({
        query: {
          scope: params?.scope,
          cursor: params?.cursor,
          limit: params?.limit,
        },
        throwOnError: true,
      });
      return identify<
        { items: Conversation[]; next_cursor: string; has_more: boolean }
      >(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async createConversation(body: {
    type: "direct" | "group" | "channel" | "voice" | "category";
    name?: string;
    topic?: string;
    project_ids?: string[];
    parent_id?: string;
    member_ids?: string[];
    target_id?: string;
  }): Promise<Conversation> {
    try {
      const { data } = await postConversations({
        body: {
          type: body.type,
          name: body.name,
          topic: body.topic,
          project_ids: body.project_ids,
          parent_id: body.parent_id,
          member_ids: body.member_ids,
          target_id: body.target_id,
        },
        throwOnError: true,
      });
      return identify<Conversation>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async getConversation(id: string): Promise<Conversation> {
    try {
      const { data } = await getConversationsById({
        path: { id },
        throwOnError: true,
      });
      return identify<Conversation>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async updateConversation(
    id: string,
    body: { name?: string; topic?: string },
  ): Promise<Conversation> {
    try {
      const { data } = await patchConversationsById({
        path: { id },
        body: { name: body.name, topic: body.topic },
        throwOnError: true,
      });
      return identify<Conversation>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async deleteConversation(id: string): Promise<void> {
    try {
      await deleteConversationsById({ path: { id }, throwOnError: true });
    } catch (err) {
      throw toError(err);
    }
  },

  async updatePosition(
    id: string,
    body: { parent_id?: string; position_key: string },
  ): Promise<void> {
    try {
      await patchConversationsByIdPosition({
        path: { id },
        body: { parent_id: body.parent_id, position_key: body.position_key },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async setNotificationLevel(
    id: string,
    level: "all" | "mentions" | "nothing",
  ): Promise<void> {
    try {
      await patchConversationsByIdNotificationLevel({
        path: { id },
        body: { level },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async listMembers(id: string): Promise<Member[]> {
    try {
      const { data } = await getConversationsByIdMembers({
        path: { id },
        throwOnError: true,
      });
      return identify<Member[]>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async addMembers(id: string, userIds: string[]): Promise<void> {
    try {
      await postConversationsByIdMembers({
        path: { id },
        body: { user_ids: userIds },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async removeMember(id: string, userId: string): Promise<void> {
    try {
      await deleteConversationsByIdMembersByUserId({
        path: { id, user_id: userId },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async markRead(id: string): Promise<void> {
    try {
      await postConversationsByIdRead({ path: { id }, throwOnError: true });
    } catch (err) {
      throw toError(err);
    }
  },

  async setMuted(id: string, muted: boolean): Promise<void> {
    try {
      await patchConversationsByIdMute({
        path: { id },
        body: { muted },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async listPinned(id: string): Promise<{ items: Message[] }> {
    try {
      const { data } = await getConversationsByIdPinned({
        path: { id },
        throwOnError: true,
      });
      return identify<{ items: Message[] }>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async listMessages(
    id: string,
    params?: { before?: string; limit?: number },
  ): Promise<{ items: Message[]; next_cursor: string; has_more: boolean }> {
    try {
      const { data } = await getConversationsByIdMessages({
        path: { id },
        query: { before: params?.before, limit: params?.limit },
        throwOnError: true,
      });
      return identify<
        { items: Message[]; next_cursor: string; has_more: boolean }
      >(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async sendMessage(
    id: string,
    body: {
      content: string;
      parent_id?: string;
      forwarded_message_id?: string;
      attachment_ids?: string[];
    },
  ): Promise<Message> {
    try {
      const { data } = await postConversationsByIdMessages({
        path: { id },
        body: {
          content: body.content,
          parent_id: body.parent_id,
          forwarded_message_id: body.forwarded_message_id,
          attachment_ids: body.attachment_ids,
        },
        throwOnError: true,
      });
      return identify<Message>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async editMessage(
    id: string,
    msgId: string,
    content: string,
  ): Promise<Message> {
    try {
      const { data } = await patchConversationsByIdMessagesByMsgId({
        path: { id, msg_id: msgId },
        body: { content },
        throwOnError: true,
      });
      return identify<Message>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async deleteMessage(id: string, msgId: string): Promise<void> {
    try {
      await deleteConversationsByIdMessagesByMsgId({
        path: { id, msg_id: msgId },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async listReplies(
    id: string,
    msgId: string,
    params?: { before?: string; limit?: number },
  ): Promise<{ items: Message[]; next_cursor: string; has_more: boolean }> {
    try {
      const { data } = await getConversationsByIdMessagesByMsgIdReplies({
        path: { id, msg_id: msgId },
        query: { before: params?.before, limit: params?.limit },
        throwOnError: true,
      });
      return identify<
        { items: Message[]; next_cursor: string; has_more: boolean }
      >(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async pinMessage(id: string, msgId: string): Promise<void> {
    try {
      await postConversationsByIdMessagesByMsgIdPin({
        path: { id, msg_id: msgId },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async unpinMessage(id: string, msgId: string): Promise<void> {
    try {
      await deleteConversationsByIdMessagesByMsgIdPin({
        path: { id, msg_id: msgId },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async addReaction(id: string, msgId: string, emoji: string): Promise<void> {
    try {
      await postConversationsByIdMessagesByMsgIdReactions({
        path: { id, msg_id: msgId },
        body: { emoji },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async removeReaction(
    id: string,
    msgId: string,
    emoji: string,
  ): Promise<void> {
    try {
      await deleteConversationsByIdMessagesByMsgIdReactionsByEmoji({
        path: { id, msg_id: msgId, emoji },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async uploadAttachment(id: string, file: File): Promise<{
    id: string;
    file_name: string;
    file_size: number;
    content_type: string;
    url: string;
  }> {
    try {
      const { data } = await postConversationsByIdAttachments({
        path: { id },
        body: { file },
        throwOnError: true,
      });
      return data as {
        id: string;
        file_name: string;
        file_size: number;
        content_type: string;
        url: string;
      };
    } catch (err) {
      throw toError(err);
    }
  },

  async searchMessages(params: {
    q: string;
    scope?: string;
    conversation_id?: string;
    sender_id?: string;
    has_attachment?: boolean;
    has_link?: boolean;
    is_pinned?: boolean;
    after?: string;
    before?: string;
    cursor?: string;
    limit?: number;
  }): Promise<{ items: Message[]; next_cursor: string; has_more: boolean }> {
    try {
      const { data } = await getConversationsSearch({
        query: {
          q: params.q,
          scope: params.scope,
          conversation_id: params.conversation_id,
          sender_id: params.sender_id,
          has_attachment: params.has_attachment,
          has_link: params.has_link,
          is_pinned: params.is_pinned,
          after: params.after,
          before: params.before,
          cursor: params.cursor,
          limit: params.limit,
        },
        throwOnError: true,
      });
      return identify<
        { items: Message[]; next_cursor: string; has_more: boolean }
      >(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async searchMentions(
    q: string,
    types?: string[],
    limit?: number,
  ): Promise<{ results: MentionResult[] }> {
    try {
      const { data } = await getMentionsSearch({
        query: { q, types: types?.length ? types.join(",") : undefined, limit },
        throwOnError: true,
      });
      return identify<{ results: MentionResult[] }>(data);
    } catch (err) {
      throw toError(err);
    }
  },

  async myPermissions(id: string): Promise<{
    can_view: boolean;
    can_send: boolean;
    can_manage: boolean;
    can_permissions: boolean;
  }> {
    try {
      const { data } = await getConversationsByIdMyPermissions({
        path: { id },
        throwOnError: true,
      });
      return data as {
        can_view: boolean;
        can_send: boolean;
        can_manage: boolean;
        can_permissions: boolean;
      };
    } catch (err) {
      throw toError(err);
    }
  },

  async getPermissions(id: string): Promise<{
    role: string;
    permission: string;
    allow: boolean;
    explicit: boolean;
  }[]> {
    try {
      const { data } = await getConversationsByIdPermissions({
        path: { id },
        throwOnError: true,
      });
      return data as {
        role: string;
        permission: string;
        allow: boolean;
        explicit: boolean;
      }[];
    } catch (err) {
      throw toError(err);
    }
  },

  async setPermissions(
    id: string,
    rules: { role: string; permission: string; allow: boolean }[],
  ): Promise<void> {
    try {
      await putConversationsByIdPermissions({
        path: { id },
        body: { rules: rules as never[] },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async getUserOverrides(id: string): Promise<{
    user_id: string;
    permission: string;
    allow: boolean;
  }[]> {
    try {
      const { data } = await getConversationsByIdUserOverrides({
        path: { id },
        throwOnError: true,
      });
      return data as { user_id: string; permission: string; allow: boolean }[];
    } catch (err) {
      throw toError(err);
    }
  },

  async setUserOverrides(
    id: string,
    overrides: { user_id: string; permission: string; allow: boolean }[],
  ): Promise<void> {
    try {
      await putConversationsByIdUserOverrides({
        path: { id },
        body: { overrides: overrides as never[] },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async getProjectLinks(id: string): Promise<{ project_ids: string[] }> {
    try {
      const { data } = await getConversationsByIdProjects({
        path: { id },
        throwOnError: true,
      });
      return data as { project_ids: string[] };
    } catch (err) {
      throw toError(err);
    }
  },

  async setProjectLinks(id: string, projectIds: string[]): Promise<void> {
    try {
      await putConversationsByIdProjects({
        path: { id },
        body: { project_ids: projectIds },
        throwOnError: true,
      });
    } catch (err) {
      throw toError(err);
    }
  },

  async listAccess(id: string): Promise<{
    user: { id: string; name: string; email: string };
    source: string;
  }[]> {
    try {
      const { data } = await getConversationsByIdAccess({
        path: { id },
        throwOnError: true,
      });
      return data as {
        user: { id: string; name: string; email: string };
        source: string;
      }[];
    } catch (err) {
      throw toError(err);
    }
  },
};
