import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { navigate } from "@/routes/router";
import { auth } from "@/store/auth";
import { getUsers } from "@/api";
import { conversationList, showCreateDm } from "../store";
import { chatApi } from "../api";
import type { DtoUserResponse } from "@/api";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/avatar.ts";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/spinner.ts";
import { localized, msg } from "@lit/localize";

/**
 * Create DM / Group DM dialog.
 *
 *  - Immediate search on type (no form library overhead)
 *  - Multi-select with checkboxes in the user list
 *  - Explicit "Group DM" hint when 2+ users are selected
 *  - Clear error messages and loading states
 *  - Debounced search to avoid API thrashing
 *  - Resets all state on close
 */
@localized()
@customElement("plume-create-dm-dialog")
export class PlumeCreateDmDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: contents;
    }
    .body {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
    }
    .user-list {
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      max-height: var(--space-72);
      overflow-y: auto;
      overflow-x: hidden;
    }
    .user-list-inner {
      padding: var(--space-1);
    }
    .user-row {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      width: 100%;
      padding: var(--space-2) var(--space-3);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: inherit;
      font-family: inherit;
      font-size: var(--text-sm);
      text-align: left;
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .user-row:hover {
      background: var(--accent);
    }
    .user-row.selected {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
    }
    .user-info {
      flex: 1;
      min-width: 0;
    }
    .user-name {
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .user-email {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .check-box {
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-sm);
      border: 1px solid var(--border);
      flex-shrink: 0;
      transition:
        background var(--dur-fast) var(--ease-1),
        border-color var(--dur-fast) var(--ease-1);
    }
    .check-box.checked {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
    }
    .hint {
      padding: var(--space-4);
      text-align: center;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
    .selection-info {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
    .footer {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
      width: 100%;
    }
  `;

  #signals = new SignalController(this);

  @state()
  private _search = "";

  @state()
  private _users: DtoUserResponse[] = [];

  @state()
  private _loading = false;

  @state()
  private _selectedIds: string[] = [];

  @state()
  private _creating = false;

  @state()
  private _error = "";

  private _debounceTimer = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(showCreateDm, auth);
  }

  protected willUpdate(changed: Map<string, unknown>): void {
    if (changed.has("_search")) {
      if (this._debounceTimer) clearTimeout(this._debounceTimer);
      this._debounceTimer = window.setTimeout(() => {
        this._debounceTimer = 0;
        this.#search();
      }, 150);
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
  }

  #reset() {
    this._search = "";
    this._users = [];
    this._loading = false;
    this._selectedIds = [];
    this._creating = false;
    this._error = "";
  }

  #onClose() {
    showCreateDm.value = false;
    this.#reset();
  }

  async #search() {
    const q = this._search.trim();
    if (!q) {
      this._users = [];
      return;
    }
    this._loading = true;
    try {
      const { data } = await getUsers({
        query: { search: q, limit: 25 },
        throwOnError: true,
      });
      const items = data?.items ?? [];
      const self = auth.value.user;
      this._users = items.filter((u) => u.id !== self?.id);
    } catch {
      this._users = [];
    }
    this._loading = false;
  }

  #toggleUser(id: string) {
    if (this._selectedIds.includes(id)) {
      this._selectedIds = this._selectedIds.filter((x) => x !== id);
    } else {
      this._selectedIds = [...this._selectedIds, id];
    }
  }

  async #onCreate(e: Event) {
    e.preventDefault();

    if (this._selectedIds.length === 0) {
      this._error = "Select at least one person.";
      return;
    }

    const self = auth.value.user;
    if (!self?.id) {
      this._error = "Not authenticated.";
      return;
    }

    this._creating = true;
    this._error = "";

    try {
      let conv;
      if (this._selectedIds.length === 1) {
        conv = await chatApi.createConversation({
          type: "direct",
          target_id: this._selectedIds[0],
        });
        // The backend doesn't set name for direct DMs: inject partner name
        const partner = this._users.find((u) => u.id === this._selectedIds[0]);
        if (partner?.name && !conv.name) {
          conv = { ...conv, name: partner.name };
        }
      } else {
        conv = await chatApi.createConversation({
          type: "group",
          member_ids: [self.id, ...this._selectedIds],
        });
        // For group DMs without a server-set name, generate one from members
        if (!conv.name) {
          const names = this._users
            .filter((u) => this._selectedIds.includes(u.id!))
            .map((u) => u.name ?? "Unknown");
          conv = { ...conv, name: names.join(", ") };
        }
      }

      // Prepend to conversation list if not already present
      const current = conversationList.value;
      if (!current.find((c) => c.id === conv.id)) {
        conversationList.value = [conv, ...current];
      }

      navigate(`/messages/${conv.id}`);
      showCreateDm.value = false;
      this.#reset();
    } catch (err: unknown) {
      this._error = err instanceof Error
        ? err.message
        : msg("Failed to start chat.");
    } finally {
      this._creating = false;
    }
  }

  protected render() {
    const isOpen = showCreateDm.value;
    const hasSelection = this._selectedIds.length > 0;
    const isGroup = this._selectedIds.length > 1;

    return html`
      <plume-dialog
        style="--dialog-w:28rem"
        .open="${isOpen}"
        heading="New Message"
        @close="${this.#onClose}"
      >
        <div class="body">
          <plume-input
            type="search"
            placeholder=${msg("Search people...")}
            .value="${this._search}"
            autofocus
            @input="${(e: Event) => {
              this._search = (e.target as HTMLInputElement).value;
            }}"
          ></plume-input>

          <div class="user-list">
            <div class="user-list-inner">
              ${!this._search
                ? html`
                  <div class="hint">${msg("Type to search for people")}</div>
                `
                : this._loading
                ? html`
                  <div class="hint">
                    <plume-spinner size="16"></plume-spinner>
                  </div>
                `
                : this._users.length === 0
                ? html`
                  <div class="hint">${msg("No matches")}</div>
                `
                : this._users.map((u) => {
                  const uid = u.id!;
                  const selected = this._selectedIds.includes(uid);
                  const initial = (u.name ?? "?").charAt(0).toUpperCase();
                  return html`
                    <button
                      type="button"
                      class="user-row ${selected ? "selected" : ""}"
                      @click="${() => this.#toggleUser(uid)}"
                    >
                      <plume-avatar size="sm">${initial}</plume-avatar>
                      <div class="user-info">
                        <div class="user-name">${u.name ?? "Unknown"}</div>
                        ${u.email
                          ? html`
                            <div class="user-email">${u.email}</div>
                          `
                          : nothing}
                      </div>
                      <span class="check-box ${selected ? "checked" : ""}">
                        ${selected
                          ? html`
                            <plume-icon name="check" size="10"></plume-icon>
                          `
                          : nothing}
                      </span>
                    </button>
                  `;
                })}
            </div>
          </div>

          ${hasSelection
            ? html`
              <p class="selection-info">
                ${this._selectedIds.length} selected. ${isGroup
                  ? "Creates a group DM."
                  : "Creates a direct message."}
              </p>
            `
            : nothing} ${this._error
            ? html`
              <p class="error">${this._error}</p>
            `
            : nothing}
        </div>

        <div slot="footer" class="footer">
          <plume-button
            variant="ghost"
            type="button"
            @click="${this.#onClose}"
          >
            Cancel
          </plume-button>
          <plume-button
            type="submit"
            ?disabled="${this._creating || !hasSelection}"
            @click="${this.#onCreate}"
          >
            <plume-icon name="user-plus" size="16"></plume-icon>
            ${this._creating
              ? "Starting..."
              : isGroup
              ? "Start group DM"
              : "Start DM"}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-create-dm-dialog": PlumeCreateDmDialog;
  }
}
