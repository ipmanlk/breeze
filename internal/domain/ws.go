package domain

import "fmt"

type WsMessageType string

const (
	WsTypePing                    WsMessageType = "ping"
	WsTypePong                    WsMessageType = "pong"
	WsTypeConnected               WsMessageType = "connected"
	WsTypeError                   WsMessageType = "error"
	WsTypeConversationSubscribe   WsMessageType = "conversation_subscribe"
	WsTypeConversationUnsubscribe WsMessageType = "conversation_unsubscribe"
	WsTypeProjectSubscribe        WsMessageType = "project_subscribe"
	WsTypeProjectUnsubscribe      WsMessageType = "project_unsubscribe"
	WsTypeTypingStart             WsMessageType = "typing_start"
	WsTypeTypingStop              WsMessageType = "typing_stop"
	WsTypeMessageNew              WsMessageType = "message_new"
	WsTypeMessageUpdated          WsMessageType = "message_updated"
	WsTypeMessageDeleted          WsMessageType = "message_deleted"
	WsTypeMessagePinned           WsMessageType = "message_pinned"
	WsTypeMessageUnpinned         WsMessageType = "message_unpinned"
	WsTypeMessageReactionAdded    WsMessageType = "message_reaction_added"
	WsTypeMessageReactionRemoved  WsMessageType = "message_reaction_removed"
	WsTypeTyping                  WsMessageType = "typing"
	WsTypePresenceUpdated         WsMessageType = "presence_updated"
	WsTypeNotificationNew         WsMessageType = "notification_new"

	// Comment lifecycle events (broadcast to the project room)
	WsTypeCommentNew     WsMessageType = "comment_new"
	WsTypeCommentUpdated WsMessageType = "comment_updated"
	WsTypeCommentDeleted WsMessageType = "comment_deleted"

	// Task lifecycle events (broadcast to the project room)
	WsTypeTaskCreated          WsMessageType = "task_created"
	WsTypeTaskUpdated          WsMessageType = "task_updated"
	WsTypeTaskMoved            WsMessageType = "task_moved"
	WsTypeTaskDeleted          WsMessageType = "task_deleted"
	WsTypeTaskActivityRecorded WsMessageType = "task_activity_recorded"

	// Voice channel events
	WsTypeVoiceJoin        WsMessageType = "voice_join"
	WsTypeVoiceLeave       WsMessageType = "voice_leave"
	WsTypeVoiceSignal      WsMessageType = "voice_signal"
	WsTypeVoiceStateUpdate WsMessageType = "voice_state_update"
	WsTypeVoiceSpeaking    WsMessageType = "voice_speaking"
	WsTypeVoiceMute        WsMessageType = "voice_mute"
	WsTypeVoiceDeafen      WsMessageType = "voice_deafen"
	WsTypeVoiceKick        WsMessageType = "voice_kick"
	WsTypeVoiceError       WsMessageType = "voice_error"
	WsTypeVoiceJoinResult  WsMessageType = "voice_join_result"
)

func RoomKeyOrg(orgID string) string {
	return fmt.Sprintf("org:%s", orgID)
}

func RoomKeyUser(orgID, userID string) string {
	return fmt.Sprintf("org:%s:user:%s", orgID, userID)
}

func RoomKeyConversation(orgID, convID string) string {
	return fmt.Sprintf("org:%s:conversation:%s", orgID, convID)
}

func RoomKeyConnection(orgID, connID string) string {
	return fmt.Sprintf("org:%s:conn:%s", orgID, connID)
}

func RoomKeyProject(orgID, projectID string) string {
	return fmt.Sprintf("org:%s:project:%s", orgID, projectID)
}
