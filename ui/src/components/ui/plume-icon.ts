import { css, html, LitElement } from "lit";
import { unsafeSVG } from "lit/directives/unsafe-svg.js";
import { customElement, property } from "lit/decorators.js";
import { getIcon } from "@/lib/icons";

/**
 * Plume icon: renders lucide SVG directly from offline icon data.
 * Usage: <plume-icon name="house" size="16"></plume-icon>
 */
@customElement("plume-icon")
export class PlumeIcon extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      line-height: 0;
      color: inherit;
      flex-shrink: 0;
    }
    svg {
      display: block;
      color: inherit;
    }
  `;

  @property()
  name = "";

  @property({ type: Number })
  size = 20;

  protected render() {
    if (!this.name) {
      return html`
      `;
    }
    const body = getIcon(this.name);
    if (!body) {
      return html`
      `;
    }
    const s = String(this.size);
    return html`
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="${s}"
        height="${s}"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        ${unsafeSVG(body)}
      </svg>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-icon": PlumeIcon;
  }
}
