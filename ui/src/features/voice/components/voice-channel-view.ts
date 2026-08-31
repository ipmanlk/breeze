import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { repeat } from "lit/directives/repeat.js";
import { SignalController } from "@/lib/signal-controller";
import { sendWsMessage } from "@/store/ws";
import {
  activeVoiceConvId,
  voiceConnectionState,
  voiceIsDeafened,
  voiceIsMuted,
  voiceKickCounter,
  voiceKickReason,
  voiceMaxParticipants,
  voiceParticipants,
  voiceSDPOffer,
} from "../store";
import { VoiceConnection } from "../voice-connection";
import { resetVoiceState, setActiveConnection } from "../voice-signaling";
import { showToast } from "@/components/ui/toast-store";
import { getConversationsByIdVoiceParticipants } from "@/api";
import "./voice-panel.ts";
import "./voice-controls.ts";

@localized()
@customElement("plume-voice-channel-view")
export class PlumeVoiceChannelView extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-direction: column;
      flex-shrink: 0;
      border-bottom: 1px solid var(--border);
      background: color-mix(in oklch, var(--muted) 30%, transparent);
    }

    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--space-2) var(--space-4);
      flex-shrink: 0;
    }
    .header-left {
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .header-icon {
      color: var(--muted-foreground);
    }
    .header-name {
      font-size: var(--text-sm);
      font-weight: 600;
    }
    .header-right {
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .count {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      white-space: nowrap;
    }
    .connecting {
      display: flex;
      align-items: center;
      gap: var(--space-1-5);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }

    .join-row {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      padding: 0 var(--space-4) var(--space-3) var(--space-4);
    }
    .join-icon {
      display: flex;
      flex-shrink: 0;
      align-items: center;
      justify-content: center;
      width: var(--space-10);
      height: var(--space-10);
      border-radius: var(--radius-full);
      border: 1px solid color-mix(in oklch, var(--primary) 20%, transparent);
      background: color-mix(in oklch, var(--primary) 5%, transparent);
      color: color-mix(in oklch, var(--primary) 70%, transparent);
    }
    .join-icon[data-error] {
      border-color: color-mix(in oklch, var(--destructive) 30%, transparent);
      background: color-mix(in oklch, var(--destructive) 5%, transparent);
      color: color-mix(in oklch, var(--destructive) 70%, transparent);
    }
    .join-text {
      flex: 1;
      min-width: 0;
    }
    .join-title {
      font-size: var(--text-sm);
      font-weight: 500;
      margin: 0;
    }
    .join-sub {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      margin: 0;
    }
    .join-btn {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1-5);
      height: var(--space-8);
      padding: 0 var(--space-3);
      border: none;
      border-radius: var(--radius-md);
      background: var(--primary);
      color: var(--primary-foreground);
      font-size: var(--text-xs);
      font-weight: 500;
      cursor: pointer;
      flex-shrink: 0;
    }
    .join-btn[data-outline] {
      background: transparent;
      border: 1px solid var(--input);
      color: var(--foreground);
    }
    .join-btn:hover {
      opacity: 0.9;
    }
    audio {
      display: none;
    }
  `;

  @property()
  conversationId = "";

  @property()
  conversationName = "";

  @state()
  private _remoteStreams: Array<{ userId: string; stream: MediaStream }> = [];

  // Mirrors voiceKickCounter so willUpdate can detect changes (Lit only
  // diffs @state/@property, not plain signal reads in willUpdate).
  @state()
  private _voiceKickSeen = 0;

  // Pre-join participant count (Issue 3): fetched from the HTTP endpoint so
  // users see capacity before clicking "Join Voice". The live value (from
  // voice_state_update / join result) takes over once connected.
  @state()
  private _preJoinCount: number | null = null;

  #signals = new SignalController(this);
  #connection: VoiceConnection | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(
      voiceConnectionState,
      voiceParticipants,
      voiceSDPOffer,
      voiceIsMuted,
      voiceKickCounter,
      voiceKickReason,
      voiceMaxParticipants,
    );
    // Only react to kicks that arrive during this mount session. With the
    // disconnect teardown (below) sending voice_leave, a kick can't arrive
    // while disconnected: so syncing to the current counter avoids showing
    // a stale toast if the view remounts after a previous kick.
    this._voiceKickSeen = voiceKickCounter.value;
    this.#fetchPreJoinCount();
  }

  // Fetch the current participant count for this channel before joining so
  // users can see if it's full. Only relevant when not already connected to
  // this channel (the live signal drives the count once connected).
  async #fetchPreJoinCount(): Promise<void> {
    if (!this.conversationId) return;
    if (activeVoiceConvId.value === this.conversationId) return;
    try {
      const { data } = await getConversationsByIdVoiceParticipants({
        path: { id: this.conversationId },
      });
      // Stale-guard: a join may have completed during the fetch.
      if (activeVoiceConvId.value === this.conversationId) return;
      this._preJoinCount = Array.isArray(data) ? data.length : 0;
    } catch {
      // Permission errors / network failures are non-fatal: we just hide
      // the pre-join count.
      this._preJoinCount = null;
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    // If this view owns the active voice session for its conversation, tear
    // down the local peer connections + mic stream so they don't leak when
    // the user navigates away (the WS voice_leave is sent by #onLeave-style
    // teardown below). Sidebar collapse does NOT unmount this element;
    // app-layout toggles width via a CSS attribute only: so this only runs
    // on real navigation/unmount.
    if (
      this.#connection &&
      activeVoiceConvId.value === this.conversationId
    ) {
      if (this.conversationId) {
        sendWsMessage({
          type: "voice_leave",
          payload: { conversation_id: this.conversationId },
        });
      }
      this.#connection.leave();
      this.#connection = null;
      this._remoteStreams = [];
      resetVoiceState();
    }
    setActiveConnection(null);
  }

  // Sync subscriber PCs with participant changes
  protected willUpdate(changed: Map<string, unknown>): void {
    // Refetch the pre-join count when navigating to a different voice channel.
    if (changed.has("conversationId")) {
      this._preJoinCount = null;
      this.#fetchPreJoinCount();
    }

    this.#connection?.syncParticipants(voiceParticipants.value);
    // voice_kick received (takeover or admin kick): tear down local peer
    // connections + mic stream. Signals were already reset by resetVoiceState;
    // this closes the WebRTC resources that signals can't reach.
    const kick = voiceKickCounter.value;
    if (kick !== this._voiceKickSeen) {
      this._voiceKickSeen = kick;
      // Distinguish takeover from admin kick with a user-visible toast.
      const reason = voiceKickReason.value;
      voiceKickReason.value = null;
      if (reason === "taken_over") {
        showToast(msg("Voice session taken over by another tab"), {
          variant: "default",
          duration: 5000,
        });
      } else {
        showToast(msg("You were kicked from the voice channel"), {
          variant: "error",
          duration: 5000,
        });
      }
      if (this.#connection) {
        this.#connection.leave();
        this.#connection = null;
        this._remoteStreams = [];
        setActiveConnection(null);
      }
    }
  }

  // Actions

  #onJoin(): void {
    const convId = this.conversationId;

    voiceConnectionState.value = "connecting";
    activeVoiceConvId.value = convId;
    voiceSDPOffer.value = null;
    voiceIsMuted.value = false;
    voiceIsDeafened.value = false;

    this.#connection = new VoiceConnection(sendWsMessage);
    setActiveConnection(this.#connection);

    this.#connection.onRemoteStream = (userId, stream) => {
      this._remoteStreams = [
        ...this._remoteStreams.filter((r) => r.userId !== userId),
        { userId, stream },
      ];
    };
    this.#connection.onRemoteStreamRemoved = (userId) => {
      this._remoteStreams = this._remoteStreams.filter(
        (r) => r.userId !== userId,
      );
    };

    this.#connection.join(convId);
    sendWsMessage({
      type: "voice_join",
      payload: { conversation_id: convId },
    });
  }

  #onLeave(): void {
    const convId = activeVoiceConvId.value;
    if (convId) {
      sendWsMessage({
        type: "voice_leave",
        payload: { conversation_id: convId },
      });
    }
    this.#connection?.leave();
    this.#connection = null;
    this._remoteStreams = [];
    setActiveConnection(null);
    resetVoiceState();
  }

  #onMute(): void {
    const newMuted = !voiceIsMuted.value;
    voiceIsMuted.value = newMuted;
    this.#connection?.setMuted(newMuted);
    if (activeVoiceConvId.value) {
      sendWsMessage({
        type: "voice_mute",
        payload: {
          conversation_id: activeVoiceConvId.value,
          muted: newMuted,
        },
      });
    }
  }

  #onDeafen(): void {
    const newDeafened = !voiceIsDeafened.value;
    voiceIsDeafened.value = newDeafened;
    // Deafening always implies muting. On undeafen, don't assume unmute;
    // the server preserves the previous mute state and broadcasts
    // voice_mute with the effective value, which syncs voiceIsMuted.
    if (newDeafened) {
      voiceIsMuted.value = true;
      this.#connection?.setMuted(true);
    }
    if (activeVoiceConvId.value) {
      sendWsMessage({
        type: "voice_deafen",
        payload: {
          conversation_id: activeVoiceConvId.value,
          deafened: newDeafened,
        },
      });
    }
  }

  // Render helpers

  #renderParticipantCount(n: number) {
    const max = voiceMaxParticipants.value;
    // Show n/max (or just n when unlimited) so users see capacity.
    return html`
      <span class="count" title="${msg("Participants")}${max
        ? " (cap " + max + ")"
        : ""}">
        <plume-icon name="users" size="12"></plume-icon>
        ${max > 0 ? `${n}/${max}` : n}
      </span>
    `;
  }

  // Render

  protected render() {
    const state = voiceConnectionState.value;
    const isInChannel = activeVoiceConvId.value === this.conversationId;
    const isConnected = state === "connected" && isInChannel;
    const isConnecting = state === "connecting" && isInChannel;
    // Participant count for the header. When connected, use the live signal;
    // before joining, use the pre-join count fetched from the HTTP endpoint
    // so users see occupancy before clicking "Join Voice".
    const headerCount = isInChannel
      ? voiceParticipants.value.length
      : this._preJoinCount;
    const showCount = headerCount != null;

    return html`
      <div class="header">
        <div class="header-left">
          <span class="header-icon">
            <plume-icon name="volume-2" size="14"></plume-icon>
          </span>
          <span class="header-name">${this.conversationName}</span>
        </div>
        <div class="header-right">
          ${showCount ? this.#renderParticipantCount(headerCount) : nothing}
          ${isConnecting
            ? html`
              <span class="connecting">
                <plume-icon name="loader-2" size="14"></plume-icon>
                Connecting…
              </span>
            `
            : nothing}
        </div>
      </div>

      ${isConnected
        ? html`
          <plume-voice-panel></plume-voice-panel>
          <plume-voice-controls
            .isMuted="${voiceIsMuted.value}"
            .isDeafened="${voiceIsDeafened.value}"
            @mute="${this.#onMute}"
            @deafen="${this.#onDeafen}"
            @leave="${this.#onLeave}"
          ></plume-voice-controls>
        `
        : html`
          <div class="join-row">
            <div class="join-icon" ?data-error="${state === "error"}">
              <plume-icon
                name="${state === "error" ? "mic-off" : "phone"}"
                size="18"
              ></plume-icon>
            </div>
            <div class="join-text">
              <p class="join-title">
                ${state === "error"
                  ? msg("Microphone access denied")
                  : msg("No one's here yet")}
              </p>
              <p class="join-sub">
                ${state === "error"
                  ? msg("Plume needs mic permission to use voice.")
                  : msg("Join the voice channel to start talking.")}
              </p>
            </div>
            <button
              class="join-btn"
              ?data-outline="${state === "error"}"
              @click="${this.#onJoin}"
            >
              <plume-icon
                name="${state === "error" ? "mic-off" : "phone"}"
                size="14"
              ></plume-icon>
              ${state === "error" ? msg("Retry") : msg("Join Voice")}
            </button>
          </div>
        `} ${repeat(
          this._remoteStreams,
          (rs) => rs.userId,
          (rs) =>
            html`
              <audio
                .srcObject="${rs.stream}"
                autoplay
                ?muted="${voiceIsDeafened.value}"
              ></audio>
            `,
        )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-voice-channel-view": PlumeVoiceChannelView;
  }
}
