import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";
import { localized } from "@lit/localize";

const EMOJIS = [
  "👍",
  "❤️",
  "😂",
  "🎉",
  "🔥",
  "👀",
  "😢",
  "🙏",
  "🚀",
  "💯",
  "✅",
  "⭐",
];

const GRID_EMOJIS = [
  "👍",
  "❤️",
  "😂",
  "🎉",
  "🔥",
  "👀",
  "😢",
  "🙏",
  "🚀",
  "💯",
  "✅",
  "⭐",
  "😊",
  "🤔",
  "👋",
  "🎯",
  "💪",
  "🌟",
  "😎",
  "🥳",
  "🤝",
  "📌",
  "🔔",
  "💡",
];

@localized()
@customElement("plume-reaction-picker")
export class PlumeReactionPicker extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      border: 1px solid var(--border);
      background: var(--popover);
      box-shadow: var(--shadow-lg);
    }
    :host([layout="row"]) {
      display: flex;
    }
    :host([layout="row"]) {
      align-items: center;
      gap: 2px;
      padding: var(--space-1) var(--space-2);
      border-radius: var(--radius-full);
    }
    :host([layout="grid"]) {
      display: grid;
      grid-template-columns: repeat(8, 1fr);
      gap: var(--space-1);
      padding: var(--space-2);
      border-radius: var(--radius-md);
      width: var(--space-68);
    }
    button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border: none;
      border-radius: var(--radius-full);
      background: transparent;
      line-height: 1;
      cursor: pointer;
      transition: transform var(--dur-fast) var(--ease-1);
    }
    button:hover {
      transform: scale(1.25);
    }
    :host([layout="row"]) button {
      width: var(--space-7);
      height: var(--space-7);
      font-size: 1.15rem;
    }
    :host([layout="grid"]) button {
      width: var(--space-7);
      height: var(--space-7);
      font-size: 1.2rem;
    }
  `;

  @property()
  layout: "row" | "grid" = "row";

  private _onPick(emoji: string) {
    this.dispatchEvent(
      new CustomEvent("pick", {
        detail: { emoji },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    const list = this.layout === "grid" ? GRID_EMOJIS : EMOJIS;
    return html`
      ${list.map(
        (e) =>
          html`
            <button @click="${() => this._onPick(e)}">${e}</button>
          `,
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-reaction-picker": PlumeReactionPicker;
  }
}
