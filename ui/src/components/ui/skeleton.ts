import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

/**
 * A loading skeleton placeholder. Renders an animated shimmer block that
 * approximates the shape of the content that will load.
 *
 * @property variant - "text" | "card" | "avatar" | "rect"
 * @property width - CSS width (e.g. "100%", "12rem")
 * @property height - CSS height
 * @property count - number of skeleton lines/blocks to render
 */
@customElement("plume-skeleton")
export class PlumeSkeleton extends LitElement {
  @property()
  variant: "text" | "card" | "avatar" | "rect" = "text";
  @property()
  width = "100%";
  @property()
  height = "1rem";
  @property({ type: Number })
  count = 1;

  static styles = css`
    :host {
      display: block;
    }
    .skel-wrap {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
    }
    .skel {
      background: linear-gradient(
        90deg,
        var(--muted) 25%,
        var(--muted-foreground) 50%,
        var(--muted) 75%
      );
      background-size: 200% 100%;
      animation: skel-shimmer 1.5s ease-in-out infinite;
      border-radius: var(--radius-sm);
    }
    .skel-avatar {
      border-radius: 50%;
    }
    .skel-card {
      border-radius: var(--radius-md);
    }
    @keyframes skel-shimmer {
      0% {
        background-position: 200% 0;
      }
      100% {
        background-position: -200% 0;
      }
    }
  `;

  render() {
    const items = Array.from({ length: this.count }, (_, i) => i);
    const cls = this.variant === "avatar"
      ? "skel skel-avatar"
      : this.variant === "card"
      ? "skel skel-card"
      : "skel";
    return html`
      <div class="skel-wrap">
        ${items.map(
          () =>
            html`
              <div
                class="${cls}"
                style="width:${this.width};height:${this.height}"
              ></div>
            `,
        )}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-skeleton": PlumeSkeleton;
  }
}
