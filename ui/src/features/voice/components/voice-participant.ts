import { css, html, LitElement, nothing } from "lit";
import { customElement, property } from "lit/decorators.js";
import type { VoiceParticipant } from "../types";
import "../../../components/ui/breeze-icon.ts";

@customElement("breeze-voice-participant")
export class BreezeVoiceParticipant extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      flex-shrink: 0;
      flex-direction: column;
      align-items: center;
      gap: var(--space-1-5);
    }
    .avatar-wrap {
      position: relative;
      border-radius: var(--radius-full);
      transition: box-shadow var(--dur-fast) var(--ease-1);
      box-shadow: 0 0 0 2px transparent;
    }
    .avatar-wrap[data-speaking] {
      box-shadow:
        0 0 0 2px #22c55e,
        0 0 0 4px color-mix(in oklch, #22c55e 30%, transparent);
      animation: pulse-glow var(--dur-slow) var(--ease-1) infinite;
    }
    .avatar {
      width: var(--space-12);
      height: var(--space-12);
      border-radius: var(--radius-full);
      background: var(--muted);
      color: var(--muted-foreground);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: var(--text-xs);
      font-weight: 600;
      overflow: hidden;
    }
    .avatar img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
    .badge {
      position: absolute;
      bottom: -2px;
      right: -2px;
      background: var(--background);
      border-radius: var(--radius-full);
      padding: 2px;
      box-shadow: var(--shadow-sm);
      color: var(--muted-foreground);
    }
    .badge[data-destructive] {
      color: var(--destructive);
    }
    .badge breeze-icon {
      display: block;
    }
    .name {
      max-width: var(--space-20);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
    }
  `;

  @property({ type: Object, attribute: false })
  participant!: VoiceParticipant;

  @property({ type: Boolean })
  isSpeaking = false;

  #initials(name: string): string {
    return name
      .split(" ")
      .map((n) => n[0] ?? "")
      .join("")
      .toUpperCase()
      .slice(0, 2);
  }

  protected render() {
    const p = this.participant;
    return html`
      <div class="avatar-wrap" ?data-speaking="${this.isSpeaking}">
        <div class="avatar">
          ${p.avatar_url
            ? html`
              <img src="${p.avatar_url}" alt="${p.name}" />
            `
            : this.#initials(p.name)}
        </div>
        ${p.muted || p.deafened
          ? html`
            <span class="badge" ?data-destructive="${p.deafened}">
              <breeze-icon
                name="${p.deafened ? "volume-x" : "mic-off"}"
                size="14"
              ></breeze-icon>
            </span>
          `
          : nothing}
      </div>
      <span class="name">${p.name}</span>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-voice-participant": BreezeVoiceParticipant;
  }
}
