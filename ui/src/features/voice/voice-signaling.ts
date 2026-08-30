import { logError } from "@/lib/log";
import {
  activeVoiceConvId,
  voiceConnectionState,
  voiceICEServers,
  voiceIsDeafened,
  voiceIsMuted,
  voiceKickCounter,
  voiceKickReason,
  voiceMaxParticipants,
  voiceParticipants,
  voiceSDPOffer,
  voiceSpeaking,
} from "./store";
import type { VoiceParticipant } from "./types";
import { auth } from "@/store/auth";

let activeConnection: {
  handleSignal(payload: {
    type: string;
    sdp?: string;
    candidate?: string;
    target_user_id?: string;
  }): void;
  handleJoinResult(sdpOffer: string): void;
} | null = null;

export function setActiveConnection(
  conn: typeof activeConnection,
): void {
  activeConnection = conn;
}

/**
 * Process voice-related WebSocket events and update signals.
 * Called from the chat-layout WS handler for voice_* events.
 */
export function handleVoiceWsMessage(data: Record<string, unknown>): void {
  const activeConv = activeVoiceConvId.value;

  // voice_signal events carry WebRTC signaling data: route to
  // the active peer connection (ICE candidates, subscriber offers).
  if (data.type === "voice_signal") {
    if (activeConnection) {
      const p = data.payload as {
        type: string;
        sdp?: string;
        candidate?: string;
        target_user_id?: string;
      };
      activeConnection.handleSignal(p);
    }
    return;
  }

  switch (data.type as string) {
    case "voice_join_result": {
      const p = data.payload as {
        participants?: VoiceParticipant[];
        ice_servers?: Array<{
          urls: string[];
          username?: string;
          credential?: string;
        }>;
        sdp_offer?: string;
        max_participants?: number;
      };
      voiceParticipants.value = p.participants || [];
      voiceICEServers.value = p.ice_servers || [];
      voiceMaxParticipants.value = p.max_participants ?? 0;
      if (p.sdp_offer) activeConnection?.handleJoinResult(p.sdp_offer);
      voiceConnectionState.value = "connected";
      break;
    }

    case "voice_state_update": {
      const p = data.payload as {
        conversation_id: string;
        participants?: VoiceParticipant[];
      };
      if (p.conversation_id !== activeConv) break;
      voiceParticipants.value = p.participants || [];
      break;
    }

    case "voice_speaking": {
      const p = data.payload as {
        conversation_id: string;
        user_id: string;
        speaking: boolean;
      };
      if (p.conversation_id !== activeConv) break;
      const next = new Set(voiceSpeaking.value);
      if (p.speaking) next.add(p.user_id);
      else next.delete(p.user_id);
      voiceSpeaking.value = next;
      break;
    }

    case "voice_mute": {
      const p = data.payload as {
        conversation_id: string;
        user_id: string;
        muted: boolean;
      };
      if (p.conversation_id !== activeConv) break;
      voiceParticipants.value = voiceParticipants.value.map((part) =>
        part.user_id === p.user_id ? { ...part, muted: p.muted } : part
      );
      // Sync local mute signal when the server reports our own mute state
      // (e.g. deafen implies mute: the server broadcasts voice_mute with
      // the effective muted state, which may differ from the optimistic
      // local toggle on undeafen).
      if (p.user_id === auth.value.user?.id) {
        voiceIsMuted.value = p.muted;
      }
      break;
    }

    case "voice_deafen": {
      const p = data.payload as {
        conversation_id: string;
        user_id: string;
        deafened: boolean;
      };
      if (p.conversation_id !== activeConv) break;
      voiceParticipants.value = voiceParticipants.value.map((part) =>
        part.user_id === p.user_id ? { ...part, deafened: p.deafened } : part
      );
      // Sync local deafen signal for the current user (e.g. admin-initiated
      // deafen, or server-confirmed state after our own toggle).
      if (p.user_id === auth.value.user?.id) {
        voiceIsDeafened.value = p.deafened;
      }
      break;
    }

    case "voice_kick": {
      const p = data.payload as { conversation_id?: string; reason?: string };
      if (p.conversation_id && p.conversation_id !== activeConv) break;
      // Bump so the active view tears down its local peer connections + mic.
      voiceKickReason.value = p.reason ?? null;
      voiceKickCounter.value++;
      resetVoiceState();
      break;
    }

    case "voice_error": {
      const p = data.payload as {
        conversation_id?: string;
        code: string;
        message: string;
      };
      logError("Voice error:", p.code, p.message);
      voiceConnectionState.value = "error";
      break;
    }

    case "error": {
      const p = data.payload as {
        code: string;
        message: string;
        req_type?: string;
      };
      if (
        p.req_type === "voice_join" ||
        p.req_type === "voice_leave" ||
        p.req_type === "voice_signal"
      ) {
        logError("Voice server error:", p.code, p.message);
        voiceConnectionState.value = "error";
      }
      break;
    }
  }
}

export function resetVoiceState(): void {
  voiceConnectionState.value = "idle";
  activeVoiceConvId.value = null;
  voiceParticipants.value = [];
  voiceSpeaking.value = new Set();
  voiceSDPOffer.value = null;
  voiceICEServers.value = [];
  voiceIsMuted.value = false;
  voiceIsDeafened.value = false;
  voiceMaxParticipants.value = 0;
  // Note: voiceKickReason is intentionally NOT reset here. The view reads it
  // in willUpdate (driven by voiceKickCounter) to show a toast, then clears it.
}
