package voice

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

// Engine holds the WebRTC media engine and configuration.
type Engine struct {
	mediaEngine     *webrtc.MediaEngine
	interceptorReg  *interceptor.Registry
	settingEngine   webrtc.SettingEngine
	iceServers      []webrtc.ICEServer // static servers (STUN + legacy static TURN)
	turnAuth        *TurnAuth          // ephemeral credential generator (nil if no secret)
	turnURLs        []string           // TURN URI(s) to advertise (with ephemeral creds applied)
	maxParticipants int
	log             *slog.Logger
}

// NewEngine creates a new WebRTC engine with the given configuration.
func NewEngine(cfg Config, log *slog.Logger) (*Engine, error) {
	e := &Engine{
		log:             log,
		iceServers:      make([]webrtc.ICEServer, 0, len(cfg.STUNURLs)),
		maxParticipants: cfg.MaxParticipants,
	}

	// Build STUN ICE servers from config
	for _, url := range cfg.STUNURLs {
		e.iceServers = append(e.iceServers, webrtc.ICEServer{
			URLs: []string{url},
		})
	}

	// TURN: prefer ephemeral REST credentials when a secret is configured
	// (industry standard; see TurnAuth). Fall back to static long-term
	// credentials for compatibility with simple setups.
	if cfg.TurnEnabled {
		// Resolve the TURN URI list. An explicit TurnURLs list (supporting
		// TCP/TLS transports) takes precedence over the TurnHost/TurnPort-
		// derived plain UDP URL.
		if len(cfg.TurnURLs) > 0 {
			e.turnURLs = cfg.TurnURLs
		} else {
			e.turnURLs = []string{fmt.Sprintf("turn:%s:%d", cfg.TurnHost, cfg.TurnPort)}
		}
		if cfg.TurnSecret != "" {
			e.turnAuth = NewTurnAuth(cfg.TurnSecret, cfg.TurnCredentialTTL)
		} else {
			// Static long-term creds: apply to every TURN URL.
			for _, u := range e.turnURLs {
				iceServer := webrtc.ICEServer{URLs: []string{u}}
				if cfg.TurnUser != "" {
					iceServer.Username = cfg.TurnUser
					iceServer.Credential = cfg.TurnPass
				}
				e.iceServers = append(e.iceServers, iceServer)
			}
		}
	}

	// Setup media engine with Opus only
	if err := e.setupMediaEngine(); err != nil {
		return nil, fmt.Errorf("setup media engine: %w", err)
	}

	// Setup interceptors
	if err := e.setupInterceptors(); err != nil {
		return nil, fmt.Errorf("setup interceptors: %w", err)
	}

	return e, nil
}

// ICEServers returns the static ICE servers (STUN + legacy static TURN).
// Ephemeral TURN servers are generated per-join via ICEServersForUser so
// each participant gets credentials bound to them and time-limited.
func (e *Engine) ICEServers() []webrtc.ICEServer {
	return e.iceServers
}

// MaxParticipants returns the configured per-channel participant cap.
func (e *Engine) MaxParticipants() int {
	return e.maxParticipants
}

// ICEServersForUser returns the full ICE server list for a specific join,
// generating ephemeral TURN REST credentials when a TURN secret is configured.
// This is the RFC 5766 TURN REST API scheme (coturn `use-auth-secret`). The
// same per-user credential is applied to every TURN URL (UDP/TCP/TLS) so a
// client can fall back across transports.
func (e *Engine) ICEServersForUser(userID string) []webrtc.ICEServer {
	if e.turnAuth == nil {
		// No ephemeral TURN; return the static list (STUN + static TURN).
		return e.iceServers
	}

	servers := make([]webrtc.ICEServer, 0, len(e.iceServers)+1)
	// STUN + any static servers first
	servers = append(servers, e.iceServers...)

	username, password, _ := e.turnAuth.Generate(userID, time.Now())
	servers = append(servers, webrtc.ICEServer{
		URLs:       e.turnURLs,
		Username:   username,
		Credential: password,
	})
	return servers
}

// setupMediaEngine configures the media engine for voice-only (Opus).
func (e *Engine) setupMediaEngine() error {
	e.mediaEngine = &webrtc.MediaEngine{}

	opusCodec := webrtc.RTPCodecCapability{
		MimeType:    webrtc.MimeTypeOpus,
		ClockRate:   48000,
		Channels:    2,
		SDPFmtpLine: "minptime=10;useinbandfec=1",
	}

	if err := e.mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: opusCodec,
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return fmt.Errorf("register opus codec: %w", err)
	}

	if err := e.mediaEngine.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: "urn:ietf:params:rtp-hdrext:ssrc-audio-level"},
		webrtc.RTPCodecTypeAudio,
	); err != nil {
		return fmt.Errorf("register audio level extension: %w", err)
	}

	return nil
}

// setupInterceptors configures RTCP interceptors.
func (e *Engine) setupInterceptors() error {
	e.interceptorReg = &interceptor.Registry{}

	if err := webrtc.RegisterDefaultInterceptors(e.mediaEngine, e.interceptorReg); err != nil {
		return fmt.Errorf("register default interceptors: %w", err)
	}

	return nil
}

// newPeerConnection creates a new WebRTC peer connection for the SFU itself
// (publisher and subscriber PCs). When ephemeral TURN credentials are
// configured (the recommended coturn `use-auth-secret` mode), the static
// e.iceServers list contains only STUN; TURN is generated per-user for the
// client side. The SFU's own PCs must also be able to gather relay candidates
// when behind NAT, so we generate an ephemeral credential for a dedicated
// "sfu" identity and include the TURN server here.
func (e *Engine) newPeerConnection() (*webrtc.PeerConnection, error) {
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(e.mediaEngine),
		webrtc.WithSettingEngine(e.settingEngine),
		webrtc.WithInterceptorRegistry(e.interceptorReg),
	)

	iceServers := e.iceServers
	if e.turnAuth != nil {
		username, password, _ := e.turnAuth.Generate("sfu", time.Now())
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       e.turnURLs,
			Username:   username,
			Credential: password,
		})
	}

	config := webrtc.Configuration{
		ICEServers:   iceServers,
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	return api.NewPeerConnection(config)
}
