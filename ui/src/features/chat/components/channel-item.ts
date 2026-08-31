import { html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";
import type { Conversation } from "../types";
import { chatApi } from "../api";
import {
  activeConversation,
  conversationList,
  settingsConvId,
  showChannelSettings,
} from "../store";
import { setupChannelDraggable } from "../conversation-dnd";
import { OutsideClickController } from "@/lib/outside-click-controller";
import { SignalController } from "@/lib/signal-controller";
import { canManageOrg } from "@/lib/permissions";
import { auth } from "@/store/auth";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import { localized, msg } from "@lit/localize";

const CI_STYLES = `
plume-channel-item { display: block; position: relative; }
.ci-row {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  border-radius: var(--radius-md);
}
.ci-row[data-active] {
  background: var(--sidebar-accent);
  color: var(--sidebar-accent-foreground);
}
.ci-row[data-dragging] { opacity: 0.4; }
.ci-grip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-4);
  height: var(--space-4);
  flex-shrink: 0;
  color: var(--muted-foreground);
  cursor: grab;
  touch-action: none;
  opacity: 0;
}
.ci-row:hover .ci-grip,
.ci-row[data-dragging] .ci-grip {
  opacity: 1;
}
.ci-main-btn {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 1;
  min-width: 0;
  height: var(--control-h-sm);
  padding: 0 var(--space-2);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: inherit;
  font-size: var(--text-sm);
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  transition: background var(--dur-fast) var(--ease-1);
}
.ci-main-btn:active {
  transform: scale(0.97);
  transition: var(--tr-transform);
}
.ci-main-btn:hover { background: var(--sidebar-accent); }
.ci-row[data-active] .ci-main-btn:hover { background: transparent; }
.ci-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-4);
  height: var(--space-4);
  flex-shrink: 0;
  color: var(--muted-foreground);
}
.ci-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ci-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: var(--space-5);
  height: var(--space-5);
  padding: 0 var(--space-1-5);
  border-radius: var(--radius-full);
  background: var(--primary);
  color: var(--primary-foreground);
  font-size: var(--text-2xs);
  font-weight: 600;
  flex-shrink: 0;
}
.ci-more-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--space-5);
  height: var(--space-5);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted-foreground);
  cursor: pointer;
  opacity: 0;
  flex-shrink: 0;
}
.ci-row:hover .ci-more-btn,
.ci-more-btn.open { opacity: 1; }
.ci-more-btn:hover {
  background: var(--sidebar-accent);
  color: var(--sidebar-foreground);
}
.ci-menu {
  position: absolute;
  right: 0;
  top: calc(100% + var(--space-0-5));
  z-index: var(--z-dropdown, 50);
  min-width: var(--space-40);
  padding: var(--space-1);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--popover);
  color: var(--popover-foreground);
  box-shadow: var(--shadow-md);
  animation: menu-in var(--dur-fast) var(--ease-2);
  transform-origin: top right;
}
.ci-menu-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-1-5) var(--space-2);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--foreground);
  font-size: var(--text-sm);
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  transition: background var(--dur-fast) var(--ease-1);
}
.ci-menu-item:active {
  transform: scale(0.97);
  transition: var(--tr-transform);
}
.ci-menu-item:hover { background: var(--accent); }
.ci-menu-item.destructive { color: var(--destructive); }
.ci-menu-item.destructive:hover {
  background: color-mix(in oklch, var(--destructive) 12%, transparent);
}
.ci-menu-divider {
  height: 1px;
  background: var(--border);
  margin: var(--space-1) 0;
}
.ci-dlg-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
.ci-error {
  font-size: var(--text-xs);
  font-weight: 500;
  color: var(--destructive);
}
.ci-del-desc {
  font-size: var(--text-sm);
  color: var(--muted-foreground);
  margin: 0;
}
`;

/**
 * Channel item: light DOM for @atlaskit DnD compatibility.
 *
 * Features:
 *  - Draggable (grip icon on hover)
 *  - Click to navigate
 *  - Context menu: Rename, Delete
 *  - Inline rename and delete confirmation dialogs
 */
@localized()
@customElement("plume-channel-item")
export class PlumeChannelItem extends LitElement {
  createRenderRoot() {
    return this;
  }

  @property({ type: Object, attribute: false })
  conv!: Conversation;

  @property({ type: Boolean })
  isActive = false;

  @state()
  private _menuOpen = false;

  @state()
  private _renameOpen = false;

  @state()
  private _deleteOpen = false;

  @state()
  private _renameValue = "";

  @state()
  private _saving = false;

  @state()
  private _deleting = false;

  @state()
  private _error = "";

  /**
   * This channel's resolved manage permission, fetched lazily when the
   * context menu opens. Null until known: the menu is fail-closed.
   * Org owner/admins bypass this via `canManageOrg`.
   */
  @state()
  private _canManage: boolean | null = null;

  /** Conversation id that `_canManage` was resolved for. */
  #permsConvId = "";

  #signals = new SignalController(this);

  #rowRef = createRef<HTMLDivElement>();
  #dragCleanup?: () => void;

  private _outsideClick = new OutsideClickController(this, () => {
    if (this._menuOpen) this._menuOpen = false;
  });

  private _keydownHandler = (e: KeyboardEvent) => {
    if (e.key === "Escape" && this._menuOpen) {
      this._menuOpen = false;
    }
  };

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(auth);
    this._outsideClick.connect();
    document.addEventListener("keydown", this._keydownHandler);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#dragCleanup?.();
    this._outsideClick.disconnect();
    document.removeEventListener("keydown", this._keydownHandler);
  }

  protected updated(changed: Map<string, unknown>) {
    if (changed.has("conv")) {
      this.#setupDrag();
      // Invalidate a cached permission when the row is reused for a
      // different conversation.
      if (this.conv.id !== this.#permsConvId) this._canManage = null;
    }
  }

  /** True when the current user may manage THIS channel. Org owner/admin
   * always qualify; everyone else needs an explicit can_manage from the
   * channel permission resolution (fail-closed while unknown). */
  get #canManageChannel(): boolean {
    return canManageOrg(auth.value.user?.role) || this._canManage === true;
  }

  /** Lazily resolve this conversation's manage permission so the context
   * menu only appears for users who can actually use it. */
  async #ensureCanManage() {
    const convId = this.conv.id;
    if (
      !convId ||
      (this.#permsConvId === convId && this._canManage !== null)
    ) {
      return;
    }
    this.#permsConvId = convId;
    try {
      const perms = await chatApi.myPermissions(convId);
      if (this.conv.id === convId) this._canManage = perms.can_manage === true;
    } catch {
      // Fail-closed on fetch errors.
      if (this.conv.id === convId) this._canManage = false;
    }
  }

  #setupDrag() {
    this.#dragCleanup?.();
    const el = this.#rowRef.value;
    if (!el || !this.conv.id) return;
    this.#dragCleanup = setupChannelDraggable(el, this.conv);
  }

  private _onClick() {
    this.dispatchEvent(
      new CustomEvent("select", {
        detail: this.conv,
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _toggleMenu(e: Event) {
    e.stopPropagation();
    this._menuOpen = !this._menuOpen;
    // Resolve manage permission on first open: the menu stays hidden
    // until the user is known to be allowed.
    if (this._menuOpen) void this.#ensureCanManage();
  }

  private _openSettings() {
    this._menuOpen = false;
    settingsConvId.value = this.conv.id;
    showChannelSettings.value = true;
    const url = new URL(window.location.href);
    url.searchParams.set("settings", "1");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  private _openRename() {
    this._menuOpen = false;
    this._renameValue = this.conv.name;
    this._error = "";
    this._renameOpen = true;
  }

  private _openDelete() {
    this._menuOpen = false;
    this._error = "";
    this._deleteOpen = true;
  }

  private async _onRenameSubmit(e: Event) {
    e.preventDefault();
    const name = this._renameValue.trim();
    if (!name) {
      this._error = "Name is required.";
      return;
    }
    this._saving = true;
    this._error = "";
    try {
      const updated = await chatApi.updateConversation(this.conv.id, { name });
      conversationList.value = conversationList.value.map((c) =>
        c.id === this.conv.id ? { ...c, ...updated } : c
      );
      if (activeConversation.value?.id === this.conv.id) {
        activeConversation.value = { ...activeConversation.value, ...updated };
      }
      this._renameOpen = false;
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to rename.");
    } finally {
      this._saving = false;
    }
  }

  private async _onDeleteConfirm() {
    this._deleting = true;
    try {
      await chatApi.deleteConversation(this.conv.id);
      conversationList.value = conversationList.value.filter(
        (c) => c.id !== this.conv.id,
      );
      if (activeConversation.value?.id === this.conv.id) {
        activeConversation.value = null;
      }
      this._deleteOpen = false;
    } catch {
      // ignore
    } finally {
      this._deleting = false;
    }
  }

  protected render() {
    const { conv, isActive } = this;
    const iconName = conv.type === "voice" ? "volume-2" : "hash";

    return html`
      <style>
      ${CI_STYLES}
      </style>

      <div
        ${ref(this.#rowRef)}
        class="ci-row"
        data-channel-id="${conv.id}"
        ?data-active="${isActive}"
      >
        <span class="ci-grip">
          <plume-icon name="grip-vertical" size="12"></plume-icon>
        </span>
        <button type="button" class="ci-main-btn" @click="${this._onClick}">
          <span class="ci-icon">
            <plume-icon name="${iconName}" size="14"></plume-icon>
          </span>
          <span class="ci-name">${conv.name}</span>
          ${conv.unread_count > 0
            ? html`
              <span class="ci-badge">${conv.unread_count > 99
                ? "99+"
                : conv.unread_count}</span>
            `
            : ""}
        </button>

        ${conv.type === "channel" && this.#canManageChannel
          ? html`
            <button
              type="button"
              class="ci-more-btn ${this._menuOpen ? "open" : ""}"
              aria-label=${msg("Channel actions")}
              @click="${this._toggleMenu}"
            >
              <plume-icon name="more-vertical" size="14"></plume-icon>
            </button>
            ${this._menuOpen
              ? html`
                <div class="ci-menu">
                  <button class="ci-menu-item" @click="${this._openSettings}">
                    <plume-icon name="settings" size="14"></plume-icon>
                    ${msg("Settings")}
                  </button>
                  <button class="ci-menu-item" @click="${this._openRename}">
                    <plume-icon name="pencil" size="14"></plume-icon>
                    ${msg("Rename")}
                  </button>
                  <div class="ci-menu-divider"></div>
                  <button class="ci-menu-item destructive" @click="${this
                    ._openDelete}">
                    <plume-icon name="trash-2" size="14"></plume-icon>
                    ${msg("Delete")}
                  </button>
                </div>
              `
              : nothing}
          `
          : nothing}
      </div>

      <!-- Rename dialog -->
      ${this._renameOpen
        ? html`
          <plume-dialog
            style="--dialog-w:24rem"
            .open="${true}"
            heading="Rename channel"
            @close="${() => {
              this._renameOpen = false;
            }}"
          >
            <div class="ci-dlg-body">
              <plume-input
                placeholder=${msg("channel-name")}
                .value="${this._renameValue}"
                maxlength="100"
                autofocus
                @input="${(e: Event) => {
                  this._renameValue = (e.target as HTMLInputElement).value;
                }}"
              ></plume-input>
              ${this._error
                ? html`
                  <p class="ci-error">${this._error}</p>
                `
                : ""}
            </div>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <plume-button variant="ghost" type="button" @click="${() => {
                this._renameOpen = false;
              }}">
                Cancel
              </plume-button>
              <plume-button ?disabled="${this._saving ||
                !this._renameValue.trim()}" @click="${this._onRenameSubmit}">
                ${this._saving ? "Saving..." : "Save"}
              </plume-button>
            </div>
          </plume-dialog>
        `
        : nothing}

      <!-- Delete confirmation dialog -->
      ${this._deleteOpen
        ? html`
          <plume-dialog
            style="--dialog-w:28rem"
            .open="${true}"
            heading="Delete channel"
            @close="${() => {
              this._deleteOpen = false;
            }}"
          >
            <p class="ci-del-desc">
              This will permanently delete #${this.conv
                .name} and all its messages. This cannot
              be undone.
            </p>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <plume-button variant="ghost" type="button" @click="${() => {
                this._deleteOpen = false;
              }}">
                Cancel
              </plume-button>
              <plume-button
                variant="destructive"
                ?disabled="${this._deleting}"
                @click="${this._onDeleteConfirm}"
              >
                ${this._deleting ? "Deleting..." : "Delete channel"}
              </plume-button>
            </div>
          </plume-dialog>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-channel-item": PlumeChannelItem;
  }
}
