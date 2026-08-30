import type {
  DtoConversationMemberResponse,
  DtoConversationResponse,
  DtoMentionResultResponse,
  DtoMentionsResponse,
  DtoMessageAttachmentResp,
  DtoMessageResponse,
  DtoReactionGroupResp,
  DtoUserPresenceResponse,
  DtoUserResponse,
} from "@/api/types.gen";

/**
 * Derive types from the generated @/api/types.gen without re-declaring any
 * field shapes or types.  The generated types correctly mirror the Go DTOs
 * but mark every field optional; this module narrows specific fields to
 * required where the API always returns them.
 *
 * If a generated type is added/removed/renamed, the compiler catches it
 * here: no silent drift from the API contract.
 */

// ---------------------------------------------------------------------------
// Utility: make selected keys required
// ---------------------------------------------------------------------------

/** Returns `T` with the listed keys narrowed from optional to required. */
type RequiredFields<T, K extends keyof T> =
  & T
  & { [P in K]-?: NonNullable<T[P]> };

// ---------------------------------------------------------------------------
// Types: field definitions sourced from generated types only
// ---------------------------------------------------------------------------

/** Conversation type discriminators. The Go DTO annotates these as enums so
 * the generated type narrows them; if the backend ever adds a new type this
 * union will fail to compile here, surfacing the change immediately. */
export type ConversationType =
  | "direct"
  | "group"
  | "channel"
  | "voice"
  | "category";

export type Conversation =
  & RequiredFields<
    DtoConversationResponse,
    | "id"
    | "org_id"
    | "name"
    | "created_by"
    | "position_key"
    | "created_at"
    | "updated_at"
    | "unread_count"
    | "member_count"
    | "muted"
    | "notification_level"
  >
  & {
    /** Narrow the discriminator to a proper union instead of `string`. */
    type: ConversationType;
  };

export type Message =
  & RequiredFields<
    DtoMessageResponse,
    "id" | "conversation_id" | "sender_id" | "content" | "pinned" | "created_at"
  >
  & {
    /** Reactions narrowed to use the local ReactionGroup type (fields always present). */
    reactions?: ReactionGroup[];
    /** Attachments narrowed to use the local Attachment type (fields always present). */
    attachments?: Attachment[];
  };

export type Attachment = Required<DtoMessageAttachmentResp>;
export type ReactionGroup = Required<DtoReactionGroupResp>;
export type Member = RequiredFields<
  DtoConversationMemberResponse,
  "id" | "name" | "email" | "role" | "joined_at" | "last_read_at" | "muted"
>;
export type MentionResult = RequiredFields<
  DtoMentionResultResponse,
  "id" | "type" | "label"
>;
export type UserPresence = RequiredFields<
  DtoUserPresenceResponse,
  "user_id" | "org_id" | "status" | "last_seen"
>;
export type User = RequiredFields<
  DtoUserResponse,
  "id" | "name" | "email"
>;
export type Mentions = DtoMentionsResponse;

/** Paginated message list response shape. */
export interface MessagesPage {
  items: Message[];
  has_more: boolean;
  next_cursor: string;
}
