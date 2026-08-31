import { css, html, LitElement, nothing } from "lit";
import { customElement } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { voiceParticipants, voiceSpeaking } from "../store";
import "./voice-participant.ts";

@customElement("plume-voice-panel")
export class PlumeVoicePanel extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: flex-start;
      gap: var(--space-3);
      overflow-x: auto;
      padding: var(--space-3) var(--space-4);
    }
  `;

  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(voiceParticipants, voiceSpeaking);
  }

  protected render() {
    const participants = voiceParticipants.value;
    if (participants.length === 0) return nothing;
    const speaking = voiceSpeaking.value;

    return html`
      ${participants.map(
        (p) =>
          html`
            <plume-voice-participant
              .participant="${p}"
              .isSpeaking="${speaking.has(p.user_id)}"
            ></plume-voice-participant>
          `,
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-voice-panel": PlumeVoicePanel;
  }
}
