package voice

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// rtpForwarder is a sink that receives RTP packets from a trackReader and
// writes them to a destination track. It does NOT read from the source
// track directly; the trackReader goroutine reads once and dispatches to
// all active forwarders, ensuring a single reader per TrackRemote.
//
// Each forwarder owns a writer goroutine with a small drop-oldest-free
// bounded queue: a slow subscriber (congested link) must not stall the
// publisher's read loop or the other subscribers' delivery (head-of-line
// blocking). Realtime audio favors dropping over queueing.
type rtpForwarder struct {
	dstTrack *webrtc.TrackLocalStaticRTP
	paused   atomic.Bool
	packets  chan *rtp.Packet
	done     chan struct{}
	stopOnce sync.Once
}

// forwarderQueueSize bounds the per-forwarder packet queue. At typical Opus
// pacing (20ms frames), 100 packets ≈ 2s of buffering; beyond that the
// subscriber is too far behind for audio and packets are dropped.
const forwarderQueueSize = 100

// newRTPForwarder creates a new RTP forwarder and starts its writer loop.
func newRTPForwarder(dstTrack *webrtc.TrackLocalStaticRTP) *rtpForwarder {
	f := &rtpForwarder{
		dstTrack: dstTrack,
		packets:  make(chan *rtp.Packet, forwarderQueueSize),
		done:     make(chan struct{}),
	}
	go f.writeLoop()
	return f
}

func (f *rtpForwarder) writeLoop() {
	defer close(f.done)
	for pkt := range f.packets {
		if f.paused.Load() {
			continue
		}
		if err := f.dstTrack.WriteRTP(pkt); err != nil && err != io.ErrClosedPipe {
			// Transient write errors are expected when peer connections
			// degrade; the paused/stop paths handle teardown.
		}
	}
}

// write enqueues an RTP packet for the writer goroutine. Never blocks:
// if the subscriber can't keep up, the oldest backlog is shed first.
func (f *rtpForwarder) write(packet *rtp.Packet) {
	select {
	case f.packets <- packet:
		return
	default:
	}
	// Queue full; drop the oldest queued packet and retry once.
	select {
	case <-f.packets:
	default:
	}
	select {
	case f.packets <- packet:
	default:
	}
}

// setPaused pauses or resumes forwarding without stopping.
func (f *rtpForwarder) setPaused(paused bool) {
	f.paused.Store(paused)
}

// stop shuts down the writer goroutine. Safe to call multiple times.
func (f *rtpForwarder) stop() {
	f.stopOnce.Do(func() {
		close(f.packets)
	})
}

// trackReader is a single goroutine that reads RTP packets from a TrackRemote,
// runs the audio level detector, and dispatches packets to all active forwarders.
// This ensures only one goroutine calls track.Read() and that audio level
// detection works even when there are no subscribers (no forwarders).
type trackReader struct {
	track    *webrtc.TrackRemote
	detector *audioLevelDetector
	isMuted  func() bool // returns the participant's current muted state
	stopCh   chan struct{}
	stopped  bool
	mu       sync.Mutex
	log      *slog.Logger
}

func newTrackReader(track *webrtc.TrackRemote, detector *audioLevelDetector, isMuted func() bool, log *slog.Logger) *trackReader {
	return &trackReader{
		track:    track,
		detector: detector,
		isMuted:  isMuted,
		stopCh:   make(chan struct{}),
		log:      log,
	}
}

// start begins reading RTP packets in a goroutine. The forwarders parameter
// should be a function that returns the current set of active forwarders that
// this reader should dispatch to (queried fresh on each packet so subscribers
// added after the reader starts are picked up).
func (r *trackReader) start(getForwarders func() []*rtpForwarder) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.log.Error("track reader panic recovered", "panic", rec)
			}
		}()
		buf := make([]byte, 1500)
		for {
			select {
			case <-r.stopCh:
				return
			default:
			}

			n, _, err := r.track.Read(buf)
			if err != nil {
				if err == io.EOF || err == io.ErrClosedPipe {
					return
				}
				continue
			}

			packet := &rtp.Packet{}
			if err := packet.Unmarshal(buf[:n]); err != nil {
				continue
			}

			// rtp.Packet.Unmarshal aliases Payload to buf, but packets are
			// queued here and marshalled later by forwarder writer goroutines,
			// while this loop immediately reuses buf for the next Read. Own
			// the payload so the queued packet never shares memory with buf.
			packet.Payload = append([]byte(nil), packet.Payload...)

			// Audio level detection (muted participants never report speaking)
			if r.detector != nil {
				r.detector.processPacket(packet, r.isMuted())
			}

			// Dispatch to all active forwarders
			for _, fwd := range getForwarders() {
				fwd.write(packet)
			}
		}
	}()
}

func (r *trackReader) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		close(r.stopCh)
		r.stopped = true
	}
}
