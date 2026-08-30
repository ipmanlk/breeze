import { signal } from "@preact/signals-core";
import type { VoiceParticipant } from "./types";

export type VoiceConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "error";

export const voiceParticipants = signal<VoiceParticipant[]>([]);
export const voiceIsMuted = signal(false);
export const voiceIsDeafened = signal(false);
export const voiceSpeaking = signal<Set<string>>(new Set());
export const voiceConnectionState = signal<VoiceConnectionState>("idle");
export const activeVoiceConvId = signal<string | null>(null);
export const voiceSDPOffer = signal<string | null>(null);
export const voiceICEServers = signal<
  Array<{ urls: string[]; username?: string; credential?: string }>
>([]);
// Bumped each time a voice_kick is received (takeover or admin kick). The
// voice-channel-view watches this to tear down its local peer connections +
// mic stream: signals alone don't close RTCPeerConnections.
export const voiceKickCounter = signal(0);
// Reason carried by the last voice_kick ("taken_over" | undefined). The view
// surfaces a user-visible toast distinguishing takeover from admin kick.
export const voiceKickReason = signal<string | null>(null);
// Per-channel participant cap (from join result). 0 = unlimited.
export const voiceMaxParticipants = signal(0);
