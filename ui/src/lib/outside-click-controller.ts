/**
 * OutsideClickController: shared helper for the "click outside to close"
 * pattern used by select, combobox, popover, theme-switcher, nav-user,
 * workspace-switcher, channel-item, dm-sidebar, message-input, and
 * message-item.
 *
 * Usage:
 *   private _outsideClick = new OutsideClickController(this, () => this._open = false);
 *   // On open:  this._outsideClick.connect();
 *   // On close: this._outsideClick.disconnect();
 *
 * For components that need mousedown instead of click (e.g. emoji pickers
 * that must close before focus moves):
 *   new OutsideClickController(this, () => ..., "mousedown")
 *
 * Uses composedPath() so it works correctly across shadow DOM boundaries
 * (per ui/AGENTS.md rule 6: always use composedPath(), not contains()).
 */
export class OutsideClickController {
  private host: EventTarget;
  private handler: () => void;
  private boundOnClick: (e: Event) => void;
  private eventType: string;
  private active = false;

  constructor(
    host: EventTarget,
    handler: () => void,
    eventType: string = "click",
  ) {
    this.host = host;
    this.handler = handler;
    this.eventType = eventType;
    this.boundOnClick = this.onClick.bind(this);
  }

  connect() {
    if (this.active) return;
    document.addEventListener(this.eventType, this.boundOnClick);
    this.active = true;
  }

  disconnect() {
    if (!this.active) return;
    document.removeEventListener(this.eventType, this.boundOnClick);
    this.active = false;
  }

  get isActive() {
    return this.active;
  }

  private onClick(e: Event) {
    if (!e.composedPath().includes(this.host)) {
      this.handler();
    }
  }
}
