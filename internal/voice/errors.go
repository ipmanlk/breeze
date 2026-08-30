package voice

import "errors"

// ErrParticipantExists is returned by CreatePublisher when a participant is
// already in the room. The service layer treats this as a takeover signal:
// the existing session is torn down and the new connection takes over.
var ErrParticipantExists = errors.New("participant already exists in room")
