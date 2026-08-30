package voice

import (
	"sync"
	"time"

	"github.com/pion/rtp"
)

const (
	// audioLevelThreshold is the minimum dBov level considered "speaking".
	// -40 dBov is the industry-standard default (used by WebRTC reference
	// implementations). Background noise (fans, keyboards, AC) typically
	// registers at -50 to -45 dBov, so -40 filters it out while still
	// detecting normal speech (-10 to -30 dBov).
	audioLevelThreshold = -40.0

	// speakingOnDebounce is the minimum sustained loud duration before
	// transitioning to speaking. This filters out transient noise spikes.
	speakingOnDebounce = 150 * time.Millisecond

	// speakingOffDebounce is the minimum sustained silence before
	// transitioning away from speaking. Longer than on-debounce to avoid
	// flicker during natural speech pauses.
	speakingOffDebounce = 500 * time.Millisecond

	audioLevelExtURI = "urn:ietf:params:rtp-hdrext:ssrc-audio-level"
)

// audioLevelDetector monitors RTP packets for audio level changes and fires
// onSpeaking callbacks only on boolean transitions (not on every packet).
type audioLevelDetector struct {
	userID          string
	audioLevelExtID uint8
	speaking        bool
	lastChange      time.Time
	loudSince       time.Time // when sustained loud audio started (zero if not loud)
	mu              sync.Mutex
	onSpeaking      func(userID string, speaking bool)
}

func newAudioLevelDetector(userID string, audioLevelExtID uint8, onSpeaking func(userID string, speaking bool)) *audioLevelDetector {
	return &audioLevelDetector{
		userID:          userID,
		audioLevelExtID: audioLevelExtID,
		onSpeaking:      onSpeaking,
	}
}

// processPacket examines an RTP packet for the audio level extension and
// fires onSpeaking on threshold transitions (debounced with sustained-level
// requirement to filter out transient background noise).
//
// Muted participants never emit speaking-true: if the detector's participant
// is muted, any "speaking" transition is suppressed and an in-progress
// speaking state is cleared. This prevents stale speaking indicators when a
// browser keeps sending the audio level extension (V=1) after muting.
func (d *audioLevelDetector) processPacket(packet *rtp.Packet, muted bool) {
	if d.onSpeaking == nil {
		return
	}

	level := d.extractAudioLevel(packet)
	if level == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	isLoud := *level > audioLevelThreshold

	// If muted, force-clear any active speaking state and never start it.
	if muted {
		d.loudSince = time.Time{}
		if d.speaking {
			d.speaking = false
			d.lastChange = now
			d.onSpeaking(d.userID, false)
		}
		return
	}

	if isLoud {
		// Track when sustained loud audio started
		if d.loudSince.IsZero() {
			d.loudSince = now
		}

		if !d.speaking && now.Sub(d.loudSince) >= speakingOnDebounce {
			d.speaking = true
			d.lastChange = now
			d.onSpeaking(d.userID, true)
		}
	} else {
		// Reset loud tracking; audio dropped below threshold
		d.loudSince = time.Time{}

		if d.speaking && now.Sub(d.lastChange) >= speakingOffDebounce {
			d.speaking = false
			d.lastChange = now
			d.onSpeaking(d.userID, false)
		}
	}
}

// extractAudioLevel reads the RFC 6464 audio level header extension from an
// RTP packet. Returns nil if the extension is not present.
func (d *audioLevelDetector) extractAudioLevel(packet *rtp.Packet) *float64 {
	ext := packet.GetExtension(d.audioLevelExtID)
	if len(ext) == 0 {
		return nil
	}

	// RFC 6464: extension data is 1 byte (0EXT) or 2 bytes (1EXT).
	// The last byte: bit 7 = V (voice activity), bits 6-0 = level (0-127).
	// Level is in -dBov: 0 = max volume, 127 = silence.
	levelByte := ext[len(ext)-1]
	level := -float64(levelByte & 0x7F)
	return &level
}
