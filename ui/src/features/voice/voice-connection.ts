import { logError } from "@/lib/log";
import {
  activeVoiceConvId,
  voiceConnectionState,
  voiceICEServers,
} from "./store";

type SendFn = (msg: { type: string; payload: Record<string, unknown> }) => void;

/**
 * Manages the WebRTC lifecycle for a voice channel:
 * publisher PC (sends local audio), subscriber PCs (receive remote audio),
 * and mute/deafen toggling.
 */
export class VoiceConnection {
  #send: SendFn;
  #convId: string | null = null;
  #pubPC: RTCPeerConnection | null = null;
  #localStream: MediaStream | null = null;
  #subPCs = new Map<string, RTCPeerConnection>();
  #pendingOffer: string | null = null;
  #destroyed = false;

  /** userId → { stream } for remote audio playback. */
  #onRemoteStream?: (userId: string, stream: MediaStream) => void;
  #onRemoteStreamRemoved?: (userId: string) => void;

  constructor(send: SendFn) {
    this.#send = send;
  }

  set onRemoteStream(cb: (userId: string, stream: MediaStream) => void) {
    this.#onRemoteStream = cb;
  }
  set onRemoteStreamRemoved(cb: (userId: string) => void) {
    this.#onRemoteStreamRemoved = cb;
  }

  // Public API

  async join(convId: string): Promise<void> {
    this.#convId = convId;
    this.#destroyed = false;

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: false,
      });
      if (this.#destroyed) {
        stream.getTracks().forEach((t) => t.stop());
        return;
      }
      this.#localStream = stream;
      // The publisher PC is NOT built here. It is built in handleJoinResult
      // once the server's join result has arrived: that result carries the
      // TURN/ICE server config, and building the PC before it lands would
      // freeze a STUN-only (NAT-broken) configuration in place.

      // Apply offer if it arrived before getUserMedia resolved
      if (this.#pendingOffer) {
        this.#buildPublisherPC();
        await this.#handleOffer(this.#pendingOffer, this.#pubPC!);
        this.#pendingOffer = null;
      }
    } catch {
      voiceConnectionState.value = "error";
      activeVoiceConvId.value = null;
      voiceICEServers.value = [];
      this.#send({
        type: "voice_leave",
        payload: { conversation_id: convId },
      });
    }
  }

  leave(): void {
    this.#destroyed = true;
    this.#cleanup();
  }

  setMuted(muted: boolean): void {
    this.#localStream?.getAudioTracks().forEach((track) => {
      track.enabled = !muted;
    });
  }

  // Called by signaling handler when voice_join_result arrives

  handleJoinResult(sdpOffer: string): void {
    if (!this.#pubPC) {
      if (!this.#localStream || this.#destroyed) {
        // Mic not ready yet: queue; join() applies it after getUserMedia.
        this.#pendingOffer = sdpOffer;
        return;
      }
      this.#buildPublisherPC();
    }
    this.#handleOffer(sdpOffer, this.#pubPC!);
  }

  #buildPublisherPC(): void {
    if (this.#pubPC || !this.#localStream) return;
    const convId = this.#convId!;
    const pc = new RTCPeerConnection(this.#iceConfig());
    this.#pubPC = pc;

    this.#localStream.getTracks().forEach((track) =>
      pc.addTrack(track, this.#localStream!)
    );

    pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.#send({
          type: "voice_signal",
          payload: {
            conversation_id: convId,
            type: "ice",
            candidate: JSON.stringify(event.candidate),
          },
        });
      }
    };

    // Detect dead peer connections instead of showing an endless
    // "connecting" state.
    pc.onconnectionstatechange = () => {
      if (
        pc.connectionState === "failed" &&
        this.#convId &&
        !this.#destroyed
      ) {
        voiceConnectionState.value = "error";
      }
    };
  }

  // Called by signaling handler for voice_signal events

  handleSignal(payload: {
    type: string;
    sdp?: string;
    candidate?: string;
    target_user_id?: string;
  }): void {
    const convId = this.#convId;
    if (!convId) return;

    // Incoming subscriber offer from another participant's publisher
    if (
      payload.type === "offer" &&
      payload.target_user_id &&
      payload.sdp
    ) {
      this.#createSubscriber(payload.target_user_id, payload.sdp);
    }

    // ICE candidate for publisher or subscriber PC
    if (payload.type === "ice" && payload.candidate) {
      try {
        const candidate = new RTCIceCandidate(
          JSON.parse(payload.candidate),
        );
        if (payload.target_user_id) {
          this.#subPCs
            .get(payload.target_user_id)
            ?.addIceCandidate(candidate)
            .catch(() => {});
        } else {
          this.#pubPC?.addIceCandidate(candidate).catch(() => {});
        }
      } catch { /* ignore parse errors */ }
    }
  }

  // Participant tracking: clean up subscriber PCs for users who left

  syncParticipants(participants: Array<{ user_id: string }>): void {
    const ids = new Set(participants.map((p) => p.user_id));
    for (const [userId, pc] of this.#subPCs) {
      if (!ids.has(userId)) {
        pc.close();
        this.#subPCs.delete(userId);
        this.#onRemoteStreamRemoved?.(userId);
      }
    }
  }

  // Internal

  #iceConfig(): RTCConfiguration {
    const servers = voiceICEServers.value;
    return {
      // Pass full ICE server entries (including TURN username/credential).
      // The previous implementation mapped to { urls: s.urls } only, which
      // silently dropped TURN auth and made TURN connections fail.
      iceServers: servers.length > 0
        ? servers.map((s) => ({
          urls: s.urls,
          ...(s.username ? { username: s.username } : {}),
          ...(s.credential ? { credential: s.credential } : {}),
        }))
        : [{ urls: "stun:stun.l.google.com:19302" }],
    };
  }

  async #handleOffer(
    sdp: string,
    pc: RTCPeerConnection,
  ): Promise<void> {
    try {
      await pc.setRemoteDescription({ type: "offer", sdp });
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);
      this.#send({
        type: "voice_signal",
        payload: {
          conversation_id: this.#convId!,
          type: "answer",
          sdp: answer.sdp,
        },
      });
    } catch (err) {
      logError("Failed to handle SDP offer:", err);
    }
  }

  async #createSubscriber(
    publisherUserId: string,
    sdp: string,
  ): Promise<void> {
    const existing = this.#subPCs.get(publisherUserId);
    if (existing) {
      existing.close();
      this.#subPCs.delete(publisherUserId);
    }

    try {
      const pc = new RTCPeerConnection(this.#iceConfig());

      pc.ontrack = (event) => {
        // The SFU sends a single track per publisher. Streams are
        // captured from the track's associated stream set.
        const stream = event.streams[0];
        if (stream) {
          this.#onRemoteStream?.(publisherUserId, stream);
        }
      };

      pc.onicecandidate = (event) => {
        if (event.candidate) {
          this.#send({
            type: "voice_signal",
            payload: {
              conversation_id: this.#convId!,
              type: "ice",
              candidate: JSON.stringify(event.candidate),
              target_user_id: publisherUserId,
            },
          });
        }
      };

      pc.onconnectionstatechange = () => {
        if (pc.connectionState === "failed") {
          // Drop the dead subscriber PC so syncParticipants/a re-offer can
          // rebuild it from scratch.
          pc.close();
          this.#subPCs.delete(publisherUserId);
          this.#onRemoteStreamRemoved?.(publisherUserId);
        }
      };

      await pc.setRemoteDescription({ type: "offer", sdp });
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);

      this.#send({
        type: "voice_signal",
        payload: {
          conversation_id: this.#convId!,
          type: "subscriber_answer",
          sdp: answer.sdp,
          target_user_id: publisherUserId,
        },
      });

      this.#subPCs.set(publisherUserId, pc);
    } catch (err) {
      logError("Failed to create subscriber PC:", err);
    }
  }

  #cleanup(): void {
    this.#localStream?.getTracks().forEach((t) => t.stop());
    this.#localStream = null;
    this.#pubPC?.close();
    this.#pubPC = null;
    this.#subPCs.forEach((pc) => pc.close());
    this.#subPCs.clear();
    this.#pendingOffer = null;
  }
}
