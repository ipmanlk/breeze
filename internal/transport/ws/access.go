package ws

import (
	"context"

	"ipmanlk/breeze/internal/domain"
)

// RoomAccessChecker verifies whether a user may subscribe to / broadcast into
// a given conversation or project room. The WS client consults it before
// honoring client-driven subscribe messages and before relaying typing
// indicators, so a user cannot eavesdrop on rooms they have no access to.
//
// The implementation lives in the transport layer (it composes the channel
// permission service + project access resolver) and is injected into the hub
// at wiring time. Defining the interface here keeps the ws package free of
// transport/service imports.
type RoomAccessChecker interface {
	// CanAccessConversation reports whether userID (with orgRole) may view
	// conversationID within orgID. Returns false on any error (fail closed).
	CanAccessConversation(ctx context.Context, orgID, conversationID, userID string, orgRole domain.Role) bool

	// CanAccessProject reports whether userID (with orgRole) may view
	// projectID within orgID. Returns false on any error (fail closed).
	CanAccessProject(ctx context.Context, orgID, projectID, userID string, orgRole domain.Role) bool

	// CanSendInConversation reports whether userID (with orgRole) has send
	// permission in conversationID. Used to gate typing indicators. Returns
	// false on any error (fail closed).
	CanSendInConversation(ctx context.Context, orgID, conversationID, userID string, orgRole domain.Role) bool
}
