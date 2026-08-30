import { css, html, LitElement, nothing } from "lit";
import { customElement, property, query, state } from "lit/decorators.js";
import { localized, msg } from "@lit/localize";
import { OutsideClickController } from "@/lib/outside-click-controller";
import "./breeze-icon.ts";

const MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

/** Format an ISO timestamp compactly for the trigger.
 *  - Midnight -> date only: "Jul 18" (or "Jul 18, 2027" if not this year)
 *  - With time -> "Jul 18, 2:30 PM" (year appended only if not this year)
 *  Returns "" for falsy/invalid input. */
function fmtDateTime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const now = new Date();
  const sameYear = d.getFullYear() === now.getFullYear();
  const yearSuffix = sameYear ? "" : `, ${d.getFullYear()}`;
  const datePart = `${MONTHS[d.getMonth()]} ${d.getDate()}${yearSuffix}`;
  // Midnight -> show date only (date-only deadlines read cleaner).
  if (d.getHours() === 0 && d.getMinutes() === 0) return datePart;
  let hours = d.getHours();
  const ampm = hours >= 12 ? "PM" : "AM";
  hours = hours % 12 || 12;
  const mins = String(d.getMinutes()).padStart(2, "0");
  return `${datePart}, ${hours}:${mins} ${ampm}`;
}

/** Convert an ISO timestamp to a `datetime-local` input value
 *  (yyyy-MM-ddThh:mm) in local time. Returns "" for falsy/invalid input. */
function isoToDatetimeLocal(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}`
  );
}

/**
 * Breeze date field: date/time picker that matches the rest of the form
 * controls (breeze-select / breeze-combobox).
 *
 * Renders a clean trigger button showing a formatted date/time or a
 * placeholder. Opens a `position: fixed` popover containing a native
 * `<input type="datetime-local">` for the actual picking: preserving
 * platform-native UX, accessibility, and keyboard support. A hover clear
 * (x) button on the trigger replaces the old separate "Clear" button.
 *
 * Properties:
 *   `value`     : ISO 8601 timestamp (or "" for no date).
 *   `placeholder` (trigger placeholder when empty).
 *
 * Events:
 *   `change`    : detail is the new ISO string (or null when cleared).
 *
 * Uses `position: fixed` for the panel so it is never clipped by overflow
 * ancestors (dialogs, scroll containers, etc.): mirroring breeze-select.
 */
@localized()
@customElement("breeze-date-field")
export class BreezeDateField extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      position: relative;
    }
    .trigger {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border: 1px solid var(--input);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      white-space: nowrap;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .trigger:hover {
      background: var(--accent);
    }
    .trigger:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .trigger-label {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .trigger-label .text {
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .placeholder {
      color: var(--muted-foreground);
    }
    .clear {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      margin: 0 calc(var(--space-0-5) * -1);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
      transition:
        background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .clear:hover {
      background: var(--accent-foreground);
      color: var(--foreground);
    }
    .panel {
      position: fixed;
      z-index: var(--z-dropdown);
      width: min(20rem, calc(100vw - var(--space-4)));
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--popover);
      color: var(--popover-foreground);
      box-shadow: var(--shadow-md);
      padding: var(--space-3);
      opacity: 0;
      visibility: hidden;
      transform: translateY(-4px);
      transition:
        opacity var(--dur-fast) var(--ease-1),
        transform var(--dur-fast) var(--ease-1),
        visibility var(--dur-fast);
    }
    .panel.open {
      opacity: 1;
      visibility: visible;
      transform: translateY(0);
    }
    .panel-label {
      display: block;
      font-size: var(--text-xs);
      font-weight: 600;
      color: var(--muted-foreground);
      margin-bottom: var(--space-2);
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .panel input[type="datetime-local"] {
      display: block;
      width: 100%;
      height: var(--control-h);
      padding: 0 var(--space-3);
      border-radius: var(--radius-md);
      border: 1px solid var(--input);
      background: var(--background);
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      box-sizing: border-box;
      outline: none;
      transition:
        border-color var(--dur-fast) var(--ease-1),
        box-shadow var(--dur-fast) var(--ease-1);
    }
    .panel input[type="datetime-local"]:focus {
      border-color: var(--ring);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 30%, transparent);
    }
    .panel-actions {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
      margin-top: var(--space-3);
    }
    .panel-btn {
      padding: 0 var(--space-3);
      height: var(--control-h-sm);
      border-radius: var(--radius-md);
      border: 1px solid var(--border);
      background: transparent;
      color: var(--foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .panel-btn:hover {
      background: var(--accent);
    }
    .panel-btn.primary {
      background: var(--primary);
      color: var(--primary-foreground);
      border-color: var(--primary);
    }
    .panel-btn.primary:hover {
      opacity: 0.9;
    }
    .panel-btn.danger {
      color: var(--destructive);
      border-color: var(--border);
    }
    .panel-btn.danger:hover {
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
    }
  `;

  /** ISO 8601 timestamp, or "" for no date. */
  @property()
  value = "";

  @property()
  placeholder = msg("Set date");

  @state()
  private _open = false;

  /** Working value inside the popover (datetime-local string): committed
   *  to the host value only on "Done" / clearing, never on each keystroke,
   *  so opening the picker doesn't fire intermediate change events. */
  @state()
  private _draft = "";

  @query(".trigger")
  private _trigger!: HTMLElement;

  @query(".panel")
  private _panel!: HTMLElement;

  private _outsideClick = new OutsideClickController(this, () => {
    this._open = false;
  });

  private _onDocKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape" && this._open) {
      this._open = false;
      e.preventDefault();
      e.stopPropagation();
    }
  };

  private _onScroll = () => {
    if (this._open) this._positionPanel();
  };

  private _onResize = () => {
    if (this._open) this._positionPanel();
  };

  private _positionPanel() {
    if (!this._panel || !this._trigger) return;
    const rect = this._trigger.getBoundingClientRect();
    const panelW = this._panel.offsetWidth;
    const panelH = this._panel.offsetHeight;
    // Match the trigger width (but keep the max from CSS) so the panel
    // lines up with the field like breeze-select.
    this._panel.style.minWidth = `${rect.width}px`;
    let left = rect.left;
    // Clamp into viewport.
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
    this._panel.style.top = `${top}px`;
    this._panel.style.left = `${left}px`;
  }

  protected updated(changedProps: Map<string, unknown>) {
    if (changedProps.has("_open")) {
      if (this._open) {
        this._draft = isoToDatetimeLocal(this.value);
        this._outsideClick.connect();
        document.addEventListener("keydown", this._onDocKeydown);
        window.addEventListener("scroll", this._onScroll, true);
        window.addEventListener("resize", this._onResize);
        requestAnimationFrame(() => {
          this._positionPanel();
          this._panel?.querySelector("input")?.focus();
        });
      } else {
        this._outsideClick.disconnect();
        document.removeEventListener("keydown", this._onDocKeydown);
        window.removeEventListener("scroll", this._onScroll, true);
        window.removeEventListener("resize", this._onResize);
      }
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._onDocKeydown);
    window.removeEventListener("scroll", this._onScroll, true);
    window.removeEventListener("resize", this._onResize);
  }

  private _toggle() {
    this._open = !this._open;
  }

  private _commit() {
    const iso = this._draft ? new Date(this._draft).toISOString() : "";
    this.value = iso;
    this._open = false;
    this.dispatchEvent(
      new CustomEvent("change", {
        // Empty string (not null) so the backend clears the field: its
        // update contract treats `started_at: ""` as clear, `null` as
        // "leave unchanged".
        detail: iso,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _clear(e: Event) {
    e.stopPropagation();
    this.value = "";
    this._draft = "";
    this._open = false;
    this.dispatchEvent(
      new CustomEvent("change", {
        detail: "",
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected render() {
    const hasValue = !!this.value;
    return html`
      <button
        class="trigger"
        type="button"
        aria-haspopup="dialog"
        aria-expanded="${this._open}"
        ?data-open="${this._open}"
        @click="${this._toggle}"
      >
        <span class="trigger-label">
          <breeze-icon name="calendar-days" size="14"></breeze-icon>
          <span class="${hasValue ? "text" : "placeholder text"}">
            ${hasValue ? fmtDateTime(this.value) : this.placeholder}
          </span>
        </span>
        ${hasValue
          ? html`
            <span
              class="clear"
              role="button"
              tabindex="0"
              aria-label="${msg("Clear date")}"
              @click="${this._clear}"
              @keydown="${(e: KeyboardEvent) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  this._clear(e);
                }
              }}"
            >
              <breeze-icon name="x" size="14"></breeze-icon>
            </span>
          `
          : nothing}
      </button>
      <div class="panel ${this._open ? "open" : ""}">
        <span class="panel-label">${msg("Date & time")}</span>
        <input
          type="datetime-local"
          .value="${this._draft}"
          @input="${(e: Event) => {
            this._draft = (e.target as HTMLInputElement).value;
          }}"
          @keydown="${(e: KeyboardEvent) => {
            if (e.key === "Enter") {
              e.preventDefault();
              this._commit();
            }
          }}"
        />
        <div class="panel-actions">
          ${this._draft
            ? html`
              <button
                class="panel-btn danger"
                type="button"
                @click="${() => {
                  this._draft = "";
                }}"
              >
                ${msg("Clear")}
              </button>
            `
            : nothing}
          <button class="panel-btn primary" type="button" @click="${this
            ._commit}">
            ${msg("Done")}
          </button>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-date-field": BreezeDateField;
  }
}
