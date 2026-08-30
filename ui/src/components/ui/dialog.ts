import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import "./breeze-icon.ts";

/**
 * Breeze dialog: built on the native `<dialog>` element.
 *
 * Why native `<dialog>`:
 *  - Top-layer rendering: always above page content, no z-index management.
 *  - `::backdrop` pseudo-element for the overlay scrim.
 *  - Built-in focus trap and Escape-to-close.
 *  - `showModal()` / `close()` lifecycle.
 *
 * The host element uses `display: contents` so the `<dialog>` participates
 * in layout directly.  The dialog is always in the DOM; visibility is
 * toggled by calling `showModal()` / `close()` from `updated()`.
 *
 * Slots: `header`, (default = body), `footer`.
 * Events: `close`: dispatched when the dialog closes (Escape, backdrop, or
 * programmatic).
 *
 * Animations:
 *  - Enter: `.dialog-in` class triggers scale+fade keyframe
 *  - Exit: `.dialog-out` class triggers reverse keyframe, then close()
 *  - Backdrop fades via CSS transition on `::backdrop`
 *
 * Motion group: `--motion-overlay`
 */
@localized()
@customElement("breeze-dialog")
export class BreezeDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }
    dialog {
      width: var(--dialog-w, 36rem);
      max-width: calc(100vw - var(--space-8));
      height: fit-content;
      max-height: 85vh;
      min-height: auto;
      border: 1px solid var(--border);
      border-radius: var(--dialog-radius, var(--radius-lg));
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-lg);
      padding: 0;

      /* Enter/exit animations */
      opacity: 0;
      transform: scale(0.95) translateY(-16px);
      transition:
        opacity var(--dur-exit) var(--ease-3),
        transform var(--dur-exit) var(--ease-3),
        overlay var(--dur-exit) var(--ease-3) allow-discrete,
        display var(--dur-exit) var(--ease-3) allow-discrete;
    }
    dialog[open] {
      opacity: 1;
      /* transform: none (not scale(1) translateY(0)) so the open dialog
        does NOT establish a containing block for position: fixed
        descendants: otherwise dropdown panels (combobox/popover/select)
        inside the dialog position relative to the dialog, not the viewport.
        'none' still interpolates with the transform functions below. */
      transform: none;
      transition:
        opacity var(--dur-entrance) var(--ease-2),
        transform var(--dur-entrance) var(--ease-2);
    }
    /* Exit animation: applied while dialog is still [open] to animate out */
    dialog.dialog-out {
      opacity: 0;
      transform: scale(0.95) translateY(-16px);
      transition:
        opacity var(--dur-exit) var(--ease-3),
        transform var(--dur-exit) var(--ease-3);
    }
    /* Enter via @starting-style (progressive enhancement) */
    @starting-style {
      dialog[open] {
        opacity: 0;
        transform: scale(0.95) translateY(-16px);
      }
    }
    /* Backdrop: cross-fades in sync with the dialog.
      The scrim color stays constant; only opacity animates. Enter fades in
      via @starting-style, exit fades out via .dialog-out (while the dialog
      is still [open]), so the backdrop disappears with the dialog instead
      of vanishing instantly when close() removes it from the top layer. */
    dialog::backdrop {
      background: color-mix(in oklch, black 50%, transparent);
      opacity: 0;
      transition:
        opacity var(--dur-exit) var(--ease-3),
        overlay var(--dur-exit) var(--ease-3) allow-discrete,
        display var(--dur-exit) var(--ease-3) allow-discrete;
    }
    dialog[open]::backdrop {
      opacity: 1;
      transition: opacity var(--dur-entrance) var(--ease-2);
    }
    @starting-style {
      dialog[open]::backdrop {
        opacity: 0;
      }
    }
    /* Exit: backdrop fades out with the dialog */
    dialog.dialog-out::backdrop {
      opacity: 0;
      transition: opacity var(--dur-exit) var(--ease-3);
    }

    /* Top placement: used by the command palette (shadcn top-[20%]) */
    :host([placement="top"]) dialog {
      margin: var(--command-top) auto auto auto;
    }
    :host([size="wide"]) dialog {
      --dialog-w: 64rem;
      height: 90vh;
    }
    :host([size="full"]) dialog {
      --dialog-w: min(96vw, 1100px);
      height: 90vh;
      max-height: 90vh;
    }
    dialog[open] {
      display: flex;
      flex-direction: column;
      align-items: stretch;
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--space-4);
      padding: var(--space-4) var(--space-5);
      border-bottom: 1px solid var(--border);
      flex-shrink: 0;
      border-top-left-radius: var(--radius-lg);
      border-top-right-radius: var(--radius-lg);
    }
    .header h2 {
      margin: 0;
      font-size: var(--text-lg);
      font-weight: 600;
    }
    .close-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--control-h-sm);
      height: var(--control-h-sm);
      border: none;
      border-radius: var(--radius-md);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
      transition:
        var(--tr-fast),
        var(--tr-color);
    }
    .close-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .close-btn:active {
      transform: scale(0.92);
      transition: var(--tr-transform);
    }
    .body {
      flex: 1;
      min-height: 0;
      overflow: hidden;
      padding: var(--dialog-body-padding, var(--space-4) var(--space-5));
    }
    .footer {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-4) var(--space-5);
      border-top: 1px solid var(--border);
      flex-shrink: 0;
      border-bottom-left-radius: var(--radius-lg);
      border-bottom-right-radius: var(--radius-lg);
    }
  `;

  /** Controls dialog visibility. When set to true, `showModal()` is called. */
  @property({ type: Boolean })
  open = false;

  /** Dialog title (rendered in header). */
  @property()
  heading = "";

  /** Dialog size. `full` is the widest, used by the task detail dialog. */
  @property({ reflect: true })
  size: "default" | "wide" | "full" = "default";

  /** Vertical placement of the dialog. `top` anchors near the top (command palette). */
  @property({ reflect: true })
  placement: "center" | "top" = "center";

  /** Whether to show the default close button in the header. */
  @property({ type: Boolean })
  showCloseButton = true;

  /** Suppress the header entirely (useful when the content has its own title). */
  @property({ type: Boolean })
  noHeader = false;

  /** Suppress the footer entirely (useful when the content has no footer). */
  @property({ type: Boolean })
  noFooter = false;

  @query("dialog")
  private _dialog!: HTMLDialogElement;

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("open") && this._dialog) {
      if (this.open && !this._dialog.open) {
        this._dialog.showModal();
      } else if (!this.open && this._dialog.open) {
        // Animate out: add .dialog-out class, wait for the opacity transition,
        // then close the native <dialog> (which dismisses ::backdrop too).
        //
        // CAUTION: zero-duration transitions never fire `transitionend`
        // (per the CSS Transitions spec), and our motion-disable system
        // zeroes --dur-exit via --motion-scale when animations are off
        // (master switch, overlay group, or prefers-reduced-motion). Without
        // the guard below, the handler never runs, dialog.close() is never
        // called, and the <dialog> + ::backdrop stay stuck in the top layer
        //: Escape and backdrop clicks appear to "not close the overlay".
        // When there's nothing to animate, close immediately.
        this._dialog.classList.add("dialog-out");
        const duration = this._exitDurationMs();
        if (duration <= 0) {
          this._finishClose();
          return;
        }
        const onTransitionEnd = (e: TransitionEvent) => {
          if (e.target !== this._dialog) return;
          if (e.propertyName !== "opacity") return;
          this._dialog.removeEventListener("transitionend", onTransitionEnd);
          this._finishClose();
        };
        this._dialog.addEventListener("transitionend", onTransitionEnd);
        // Safety net: if `transitionend` is missed for any reason (tab
        // unfocus, throttling, etc.), still close after the expected time.
        setTimeout(() => {
          this._dialog.removeEventListener("transitionend", onTransitionEnd);
          if (this._dialog.classList.contains("dialog-out")) {
            this._finishClose();
          }
        }, duration + 50);
      }
    }
  }

  /** Resolve the live --dur-exit value (honors --motion-scale) to ms. */
  private _exitDurationMs(): number {
    const css = getComputedStyle(this._dialog).transitionDuration;
    const first = css.split(",")[0]?.trim() ?? "0s";
    if (first.endsWith("ms")) return parseFloat(first) || 0;
    if (first.endsWith("s")) return parseFloat(first) * 1000 || 0;
    return 0;
  }

  /** Remove exit class, close native dialog, emit `close`. Idempotent. */
  private _finishClose() {
    this._dialog.classList.remove("dialog-out");
    if (this._dialog.open) this._dialog.close();
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  /** Prevent native Escape close: we handle it ourselves with animation. */
  private _onCancel(e: Event) {
    e.preventDefault();
    this.open = false;
  }

  /** Click on the dialog itself (not its children) = backdrop click. */
  private _onDialogClick(e: MouseEvent) {
    if (e.target === this._dialog) {
      this.open = false;
    }
  }

  protected render() {
    return html`
      <dialog
        aria-labelledby="dlg-title"
        aria-modal="true"
        @cancel="${this._onCancel}"
        @click="${this._onDialogClick}"
      >
        ${this.noHeader ? nothing : html`
          <div class="header">
            <slot name="header">
              <h2 id="dlg-title">${this.heading}</h2>
            </slot>
            ${this.showCloseButton
              ? html`
                <button
                  class="close-btn"
                  aria-label="${msg("Close")}"
                  @click="${() => {
                    this.open = false;
                  }}"
                >
                  <breeze-icon name="x" size="16"></breeze-icon>
                </button>
              `
              : nothing}
          </div>
        `}
        <div class="body"><slot></slot></div>
        ${this.noFooter ? nothing : html`
          <div class="footer"><slot name="footer"></slot></div>
        `}
      </dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-dialog": BreezeDialog;
  }
}
