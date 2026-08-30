import { css, html, LitElement } from "lit";
import { customElement, property } from "lit/decorators.js";

/**
 * Breeze stepper: visual step indicator.
 */
@customElement("breeze-stepper")
export class BreezeStepper extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: flex;
      gap: var(--space-2);
    }
    .step {
      flex: 1;
      height: var(--space-1);
      border-radius: var(--radius-full);
      background: var(--border);
      transition: background var(--dur-normal) var(--ease-1);
    }
    .step.active {
      background: var(--primary);
    }
  `;

  @property({ type: Number })
  steps = 2;
  @property({ type: Number })
  current = 0;

  #clamped() {
    return Math.max(0, Math.min(this.current, this.steps - 1));
  }

  protected render() {
    const active = this.#clamped();
    return html`
      ${Array.from({ length: this.steps }).map(
        (_, i) =>
          html`
            <div
              class="step ${i <= active ? "active" : ""}"
              role="presentation"
              aria-current="${i === active ? "step" : undefined}"
            ></div>
          `,
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-stepper": BreezeStepper;
  }
}
