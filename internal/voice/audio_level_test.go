package voice

import (
	"testing"
	"time"

	"github.com/pion/rtp"
)

// makeAudioLevelPacket builds an RTP packet carrying a 1-byte RFC 6464
// audio level extension (extID) with the given dBov level (0 = loudest).
func makeAudioLevelPacket(extID uint8, level byte) *rtp.Packet {
	pkt := &rtp.Packet{Header: rtp.Header{}}
	// Set the V bit (0x80) so the level is treated as voice-active.
	pkt.Header.SetExtension(extID, []byte{0x80 | (level & 0x7F)})
	return pkt
}

func TestAudioLevelDetector_SpeakingOnLoud(t *testing.T) {
	var calls []bool
	d := newAudioLevelDetector("u1", 1, func(_ string, speaking bool) {
		calls = append(calls, speaking)
	})

	// Loud audio for longer than the on-debounce → should fire speaking=true.
	deadline := time.Now().Add(speakingOnDebounce + 50*time.Millisecond)
	for time.Now().Before(deadline) {
		d.processPacket(makeAudioLevelPacket(1, 10), false)
		time.Sleep(5 * time.Millisecond)
	}

	if len(calls) == 0 || !calls[len(calls)-1] {
		t.Fatalf("expected a speaking=true transition, got calls=%v", calls)
	}
}

func TestAudioLevelDetector_MutedSuppressesSpeaking(t *testing.T) {
	var calls []bool
	d := newAudioLevelDetector("u1", 1, func(_ string, speaking bool) {
		calls = append(calls, speaking)
	})

	// Loud audio long enough to start speaking (unmuted).
	deadline := time.Now().Add(speakingOnDebounce + 50*time.Millisecond)
	for time.Now().Before(deadline) {
		d.processPacket(makeAudioLevelPacket(1, 10), false)
		time.Sleep(5 * time.Millisecond)
	}
	if len(calls) == 0 || !calls[len(calls)-1] {
		t.Fatalf("expected speaking=true before muting, got calls=%v", calls)
	}

	// Now mute. Continued loud packets must NOT keep speaking true; the
	// first muted packet should immediately clear speaking to false.
	before := len(calls)
	d.processPacket(makeAudioLevelPacket(1, 10), true)
	if len(calls) != before+1 || calls[len(calls)-1] {
		t.Fatalf("expected muted to clear speaking=false, got calls=%v", calls)
	}

	// More muted loud packets must not re-trigger speaking=true.
	d.processPacket(makeAudioLevelPacket(1, 10), true)
	d.processPacket(makeAudioLevelPacket(1, 10), true)
	if calls[len(calls)-1] {
		t.Fatalf("expected no speaking=true while muted, got calls=%v", calls)
	}
}

func TestAudioLevelDetector_QuietDoesNotSpeak(t *testing.T) {
	var calls []bool
	d := newAudioLevelDetector("u1", 1, func(_ string, speaking bool) {
		calls = append(calls, speaking)
	})

	// Quiet audio (level 127 = silence) for a while → never speaks.
	deadline := time.Now().Add(speakingOnDebounce + 100*time.Millisecond)
	for time.Now().Before(deadline) {
		d.processPacket(makeAudioLevelPacket(1, 127), false)
		time.Sleep(5 * time.Millisecond)
	}

	if len(calls) != 0 {
		t.Fatalf("expected no speaking transitions for quiet audio, got calls=%v", calls)
	}
}
