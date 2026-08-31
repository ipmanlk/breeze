import { css, html, LitElement, type PropertyValues } from "lit";
import { customElement, property, query } from "lit/decorators.js";
import { OutsideClickController } from "@/lib/outside-click-controller";

/**
 * Plume Popover: simple dropdown/popover.
 *
 * Trigger element goes in the "trigger" slot.
 * Content goes in the "content" slot.
 *
 * Opens on click, closes on outside click or Escape.
 *
 * Uses `position: fixed` for the content panel so it is never clipped by
 * overflow ancestors (dialogs, scroll containers, etc.): mirroring the
 * `plume-select` approach. The panel is repositioned on scroll/resize to
 * track its trigger.
 *
 * Set `closeOnSelect` to false for multi-select popovers that should stay
 * open after the user clicks an item inside the content area. The default
 * (`true`) closes the popover on any click inside the content: matching
 * single-select dropdowns (status pickers, action menus, etc.).
 */
@customElement("plume-popover")
export class PlumePopover extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: inline-block;
      position: relative;
    }
    .content {
      position: fixed;
      z-index: var(--z-dropdown, 50);
      /* min-width is set via JS (_positionPanel) to match the trigger, since
        min-width: 100% on a fixed element resolves to 100vw. */
      padding: var(--space-1);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      opacity: 0;
      visibility: hidden;
      transform: translateY(-4px);
      transition:
        opacity var(--dur-fast) var(--ease-1),
        transform var(--dur-fast) var(--ease-1),
        visibility var(--dur-fast);
    }
    .content.open {
      opacity: 1;
      visibility: visible;
      transform: translateY(0);
    }
    ::slotted([slot="trigger"]) {
      cursor: pointer;
    }
  `;

  @property({ type: Boolean })
  open = false;

  /**
   * When true (default), clicking inside the content area closes the
   * popover after the click: ideal for single-select dropdowns and action
   * menus. Set to false for multi-select popovers (label pickers, filter
   * bars) that should stay open so the user can toggle multiple items.
   */
  @property({ type: Boolean, attribute: "close-on-select" })
  closeOnSelect = true;

  @property()
  placement: "bottom-start" | "bottom-end" = "bottom-start";

  @query(".content")
  private _content!: HTMLDivElement;

  @query(".trigger-wrap")
  private _triggerWrap!: HTMLDivElement;

  private _outsideClick = new OutsideClickController(this, () => {
    this.open = false;
  });
  private _keydownHandler = this._handleKeydown.bind(this);

  connectedCallback(): void {
    super.connectedCallback();
    this._outsideClick.connect();
    document.addEventListener("keydown", this._keydownHandler);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._keydownHandler);
    this._removeScrollListeners();
  }

  private _handleKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape" && this.open) {
      this.open = false;
    }
  }

  private _toggle(e: Event): void {
    e.stopPropagation();
    this.open = !this.open;
  }

  private _handleContentClick(e: Event): void {
    // Only auto-close if the click originated from inside the content slot
    // (not the trigger) and closeOnSelect is enabled.
    if (this.closeOnSelect) {
      e.stopPropagation();
      this.open = false;
    }
  }

  protected updated(changedProps: PropertyValues): void {
    if (changedProps.has("open")) {
      if (this.open) {
        // Position on next frame so the panel has its measured dimensions.
        requestAnimationFrame(() => this._positionPanel());
        this._addScrollListeners();
      } else {
        this._removeScrollListeners();
      }
    }
    if (changedProps.has("placement") && this.open) {
      requestAnimationFrame(() => this._positionPanel());
    }
  }

  private _addScrollListeners(): void {
    document.addEventListener("scroll", this._onScroll, true);
    window.addEventListener("resize", this._onScroll);
  }

  private _removeScrollListeners(): void {
    document.removeEventListener("scroll", this._onScroll, true);
    window.removeEventListener("resize", this._onScroll);
  }

  private _onScroll = (): void => {
    if (this.open) this._positionPanel();
  };

  private _positionPanel(): void {
    if (!this._content || !this._triggerWrap) return;
    const rect = this._triggerWrap.getBoundingClientRect();
    const panelW = this._content.offsetWidth;
    const panelH = this._content.offsetHeight;
    // At least as wide as the trigger; content may make it wider.
    this._content.style.minWidth = `${rect.width}px`;
    let left: number;
    if (this.placement === "bottom-end") {
      left = rect.right - panelW;
    } else {
      left = rect.left;
    }
    // Keep within viewport.
    if (left + panelW > window.innerWidth - 8) {
      left = window.innerWidth - panelW - 8;
    }
    if (left < 8) left = 8;

    // Vertical: prefer below; flip above when there's no room below.
    const gap = 4;
    const roomBelow = window.innerHeight - rect.bottom - gap;
    const roomAbove = rect.top - gap;
    let top: number;
    if (roomBelow >= panelH || roomBelow >= roomAbove) {
      top = rect.bottom + gap;
    } else {
      top = Math.max(8, rect.top - panelH - gap);
    }
    this._content.style.top = `${top}px`;
    this._content.style.left = `${left}px`;
  }

  protected render() {
    return html`
      <div class="trigger-wrap" @click="${this._toggle}">
        <slot name="trigger"></slot>
      </div>
      <div
        class="content ${this.open ? "open" : ""}"
        @click="${this._handleContentClick}"
      ><slot name="content"></slot></div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-popover": PlumePopover;
  }
}
