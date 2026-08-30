package voice

import (
	"sync"
)

// Room represents a voice channel with multiple participants.
type Room struct {
	convID       string
	orgID        string
	participants map[string]*Participant // userID -> Participant
	mu           sync.RWMutex
}

// newRoom creates a new voice room.
func newRoom(convID, orgID string) *Room {
	return &Room{
		convID:       convID,
		orgID:        orgID,
		participants: make(map[string]*Participant),
	}
}

// addParticipant adds a participant to the room.
func (r *Room) addParticipant(p *Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participants[p.userID] = p
}

// removeParticipant removes a participant from the room.
// Also cleans up the leaving participant's subscriber PCs from remaining
// participants.
func (r *Room) removeParticipant(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.participants[userID]
	if !ok {
		return
	}

	p.close()

	delete(r.participants, userID)

	// Remove the leaving participant as a publisher from all remaining participants
	for _, remaining := range r.participants {
		remaining.removeSubscriberPC(userID)
	}
}

// getParticipant returns a participant by user ID.
func (r *Room) getParticipant(userID string) (*Participant, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.participants[userID]
	return p, ok
}

// getParticipants returns all participants in the room.
func (r *Room) getParticipants() map[string]*Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Participant, len(r.participants))
	for k, v := range r.participants {
		result[k] = v
	}
	return result
}

// participantCount returns the number of participants without allocating a map copy.
func (r *Room) participantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.participants)
}

// subscriberForwarders returns a snapshot of all forwarders that carry the
// given publisher's audio (i.e. one forwarder per participant who is
// subscribed to publisherID. Each such forwarder lives on the *subscriber's*
// participant, keyed by publisherID (see CreateSubscriber: subscriber.addForwarder(publisherID, fwd)).
// The publisher's own track reader dispatches to these so audio reaches every
// subscriber.
func (r *Room) subscriberForwarders(publisherID string) []*rtpForwarder {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*rtpForwarder, 0, len(r.participants))
	for _, p := range r.participants {
		p.mu.RLock()
		if fwd, ok := p.forwarders[publisherID]; ok {
			result = append(result, fwd)
		}
		p.mu.RUnlock()
	}
	return result
}

// close shuts down the room and all participants.
func (r *Room) close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.participants {
		p.close()
	}
	r.participants = make(map[string]*Participant)
}
