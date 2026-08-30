import { localized, msg } from "@lit/localize";
import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import "../../../components/ui/breeze-icon.ts";

@localized()
@customElement("breeze-voice-controls")
export class BreezeVoiceControls extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-4);
      border-top: 1px solid var(--border);
      background: color-mix(in oklch, var(--background) 50%, transparent);
    }
    .ctrl-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-8);
      height: var(--space-8);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .ctrl-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .ctrl-btn[data-active] {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      color: var(--destructive);
    }
    .ctrl-btn[data-active]:hover {
      background: color-mix(in oklch, var(--destructive) 18%, transparent);
    }
    .divider {
      width: 1px;
      height: var(--space-5);
      background: var(--border);
      margin: 0 var(--space-1);
    }
    .leave-btn {
      background: var(--destructive);
      color: var(--destructive-foreground);
    }
    .leave-btn:hover {
      opacity: 0.9;
    }
  `;

  @property({ type: Boolean })
  isMuted = false;

  @property({ type: Boolean })
  isDeafened = false;

  protected render() {
    return html`
      <button
        class="ctrl-btn"
        ?data-active="${this.isMuted}"
        @click="${this.#onMute}"
        aria-label="${this.isMuted ? msg("Unmute") : msg("Mute")}"
      >
        <breeze-icon
          name="${this.isMuted ? "mic-off" : "mic"}"
          size="16"
        ></breeze-icon>
      </button>

      <button
        class="ctrl-btn"
        ?data-active="${this.isDeafened}"
        @click="${this.#onDeafen}"
        aria-label="${this.isDeafened ? msg("Undeafen") : msg("Deafen")}"
      >
        <breeze-icon
          name="${this.isDeafened ? "volume-x" : "headphones"}"
          size="16"
        ></breeze-icon>
      </button>

      <span class="divider"></span>

      <button
        class="ctrl-btn leave-btn"
        @click="${this.#onLeave}"
        aria-label="${msg("Leave voice channel")}"
      >
        <breeze-icon name="phone-off" size="16"></breeze-icon>
      </button>
    `;
  }

  #onMute() {
    this.dispatchEvent(
      new CustomEvent("mute", { bubbles: true, composed: true }),
    );
  }
  #onDeafen() {
    this.dispatchEvent(
      new CustomEvent("deafen", { bubbles: true, composed: true }),
    );
  }
  #onLeave() {
    this.dispatchEvent(
      new CustomEvent("leave", { bubbles: true, composed: true }),
    );
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-voice-controls": BreezeVoiceControls;
  }
}
