package voice

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pion/webrtc/v4"
)

// Participant represents a user in a voice channel with their peer connections.
type Participant struct {
	userID      string
	connID      string                            // WS connection that owns this session
	pubPC       *webrtc.PeerConnection            // publisher: receives audio from this participant
	subPCs      map[string]*webrtc.PeerConnection // subscribers: send audio to this participant (keyed by publisher userID)
	inbound     *webrtc.TrackRemote               // the incoming audio track
	forwarders  map[string]*rtpForwarder          // forwarders from each publisher
	detector    *audioLevelDetector               // audio level detector for this participant's inbound track
	trackReader *trackReader                      // single reader goroutine for the inbound track
	muted       atomic.Bool                       // hot-path read on every RTP packet; atomic avoids lock contention
	mu          sync.RWMutex
	closed      bool

	// ctx is cancelled in close() so background goroutines spawned by
	// handleIncomingTrack (subscriber creation) can bail out cleanly instead
	// of racing with RemoveParticipant.
	ctx    context.Context
	cancel context.CancelFunc
}

// newParticipant creates a new participant in a room.
func newParticipant(userID, connID string, pubPC *webrtc.PeerConnection) *Participant {
	ctx, cancel := context.WithCancel(context.Background())
	return &Participant{
		userID:     userID,
		connID:     connID,
		pubPC:      pubPC,
		subPCs:     make(map[string]*webrtc.PeerConnection),
		forwarders: make(map[string]*rtpForwarder),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// connIDFor returns the owning connection ID for signaling routing.
func (p *Participant) connIDFor() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connID
}

// addSubscriberPC adds a subscriber peer connection for a specific publisher.
func (p *Participant) addSubscriberPC(publisherID string, pc *webrtc.PeerConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subPCs[publisherID] = pc
}

// removeSubscriberPC removes a subscriber peer connection.
func (p *Participant) removeSubscriberPC(publisherID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pc, ok := p.subPCs[publisherID]; ok {
		pc.Close()
		delete(p.subPCs, publisherID)
	}

	if fwd, ok := p.forwarders[publisherID]; ok {
		fwd.stop()
		delete(p.forwarders, publisherID)
	}
}

// addForwarder adds an RTP forwarder for a publisher.
func (p *Participant) addForwarder(publisherID string, fwd *rtpForwarder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forwarders[publisherID] = fwd
}

// getInboundTrack returns the inbound audio track and its negotiated codec
// safely (under the participant lock). The inbound field is written in
// handleIncomingTrack under the lock, so callers must not read it directly.
func (p *Participant) getInboundTrack() (*webrtc.TrackRemote, webrtc.RTPCodecCapability, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.inbound == nil {
		return nil, webrtc.RTPCodecCapability{}, false
	}
	return p.inbound, p.inbound.Codec().RTPCodecCapability, true
}

// getSubscriberPC returns the subscriber PC for a specific publisher.
func (p *Participant) getSubscriberPC(publisherID string) (*webrtc.PeerConnection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pc, ok := p.subPCs[publisherID]
	return pc, ok
}

// setMuted sets the muted state.
func (p *Participant) setMuted(muted bool) {
	p.muted.Store(muted)
}

// isMuted returns the muted state. This is called on the RTP hot path
// (~50 packets/sec per track), so it reads the atomic directly without
// acquiring the participant lock.
func (p *Participant) isMuted() bool {
	return p.muted.Load()
}

// close shuts down all peer connections and forwarders.
func (p *Participant) close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true

	// Cancel the participant context first so any in-flight subscriber
	// goroutines from handleIncomingTrack bail out before we tear down PCs.
	if p.cancel != nil {
		p.cancel()
	}

	if p.trackReader != nil {
		p.trackReader.stop()
	}

	if p.pubPC != nil {
		p.pubPC.Close()
	}

	for _, pc := range p.subPCs {
		pc.Close()
	}

	for _, fwd := range p.forwarders {
		fwd.stop()
	}

	p.subPCs = make(map[string]*webrtc.PeerConnection)
	p.forwarders = make(map[string]*rtpForwarder)
	p.mu.Unlock()
}
