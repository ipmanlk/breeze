import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { pageEnterStyles } from "@/styles/shared-animations";
import { localized, msg } from "@lit/localize";
import "../components/ui/button.ts";

@localized()
@customElement("plume-not-found-page")
export class PlumeNotFoundPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      :host {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        min-height: 60vh;
        gap: var(--space-4);
        text-align: center;
      }
      .nf-code {
        font-size: var(--text-5xl);
        font-weight: 700;
        color: var(--muted-foreground);
        line-height: 1;
      }
      .nf-title {
        font-size: var(--text-xl);
        font-weight: 600;
        color: var(--foreground);
      }
      .nf-desc {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        max-width: var(--space-96);
      }
    `,
  ];

  render() {
    return html`
      <div class="page-enter">
        <div class="nf-code">404</div>
        <div class="nf-title">${msg("Page not found")}</div>
        <p class="nf-desc">
          ${msg(
            "The page you're looking for doesn't exist or may have been moved.",
          )}
        </p>
        <plume-button variant="default" @click="${() => this.#goHome()}">
          ${msg("Go to dashboard")}
        </plume-button>
      </div>
    `;
  }

  #goHome() {
    window.history.pushState({}, "", "/");
    window.dispatchEvent(new PopStateEvent("popstate"));
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-not-found-page": PlumeNotFoundPage;
  }
}
