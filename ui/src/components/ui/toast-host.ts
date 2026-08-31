import { css, html, LitElement } from "lit";
import { customElement } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { repeat } from "lit/directives/repeat.js";
import { SignalController } from "@/lib/signal-controller";
import { dismissToast, toasts } from "./toast-store";
import "./plume-icon.ts";

/**
 * Top-level toast host. Renders transient notifications in the viewport's
 * top-right corner, above all other content (z-index: var(--z-toast)).
 *
 * Mounting: appended to <body> once by the app shell (see app-shell.ts), so
 * toasts survive client-side navigation and are not tied to any view's
 * lifecycle. Show toasts via `showToast()` from `./toast-store`: never
 * instantiate this element manually.
 *
 * Motion group: --motion-overlay (enter/exit fade+slide).
 */
@localized()
@customElement("plume-toast-host")
export class PlumeToastHost extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      position: fixed;
      top: var(--space-4);
      right: var(--space-4);
      z-index: var(--z-toast);
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
      max-width: calc(100vw - var(--space-8));
      pointer-events: none;
    }
    .toast {
      pointer-events: auto;
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-3);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      font-size: var(--text-sm);
      animation: toast-in var(--dur-entrance) var(--ease-2);
    }
    .toast[data-variant="success"] .icon {
      color: color-mix(in oklch, var(--primary) 80%, var(--foreground));
    }
    .toast[data-variant="error"] .icon {
      color: var(--destructive);
    }
    .icon {
      display: flex;
      flex-shrink: 0;
      color: var(--muted-foreground);
    }
    .message {
      flex: 1;
      min-width: 0;
    }
    .close {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 1.25rem;
      height: 1.25rem;
      border: none;
      border-radius: var(--radius-sm, var(--radius-md));
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      padding: 0;
    }
    .close:hover {
      color: var(--foreground);
      background: var(--muted);
    }
    @keyframes toast-in {
      from {
        opacity: 0;
        transform: translateY(-8px) scale(0.98);
      }
      to {
        opacity: 1;
        transform: translateY(0) scale(1);
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .toast {
        animation: none;
      }
    }
  `;

  #signals = new SignalController(this);

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(toasts);
    // A5: aria-live region for screen reader announcements.
    this.setAttribute("role", "status");
    this.setAttribute("aria-live", "polite");
  }

  #iconFor(variant: string): string {
    switch (variant) {
      case "success":
        return "check-circle";
      case "error":
        return "alert-circle";
      default:
        return "info";
    }
  }

  protected render() {
    return html`
      ${repeat(
        toasts.value,
        (t) => t.id,
        (t) =>
          html`
            <div class="toast" data-variant="${t.variant}" role="status">
              <span class="icon">
                <plume-icon name="${this.#iconFor(
                  t.variant,
                )}" size="16"></plume-icon>
              </span>
              <span class="message">${t.message}</span>
              <button
                class="close"
                type="button"
                aria-label="${msg("Dismiss")}"
                @click="${() => dismissToast(t.id)}"
              >
                <plume-icon name="x" size="14"></plume-icon>
              </button>
            </div>
          `,
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-toast-host": PlumeToastHost;
  }
}
