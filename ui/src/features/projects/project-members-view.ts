import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { createRef, ref } from "lit/directives/ref.js";

import {
  deleteProjectsByIdMembersByUserId,
  getProjectsByIdMembers,
  getUsers,
  postProjectsByIdMembers,
  putProjectsByIdMembersByUserId,
} from "@/api";
import type { DtoProjectMemberResponse, DtoUserResponse } from "@/api";
import type { PlumeInput } from "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/select.ts";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/avatar.ts";
import "@/components/ui/spinner.ts";
import "@/features/members/components/role-badge.ts";
import { localized, msg } from "@lit/localize";

function getROLE_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "admin", label: msg("Admin") },
    { value: "member", label: msg("Member") },
    { value: "viewer", label: msg("Viewer") },
    { value: "guest", label: msg("Guest") },
  ];
}

/**
 * Project members management view: rendered inside the project detail page's
 * "Members" tab. Follows the same table + click-to-edit role pattern as the
 * workspace members page. No redundant title (the tab already says "Members").
 *
 * Light DOM: used inside plume-project-detail-page.
 */
@localized()
@customElement("plume-project-members-view")
export class PlumeProjectMembersView extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      display: block;
      max-width: var(--container-lg);
    }

    /* Header row: member count on the left, add action on the right.
      Mirrors the settings view's section-header-with-action pattern. */
    .pmv-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: var(--space-4);
    }
    .pmv-count {
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }

    /* Table */
    .table {
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      overflow: hidden;
    }
    .table-header {
      display: grid;
      gap: var(--space-2);
      padding: var(--space-2) var(--space-4);
      background: color-mix(in oklch, var(--muted) 40%, transparent);
      border-bottom: 1px solid var(--border);
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
    }
    .table-body {
      max-height: calc(100vh - 20rem);
      overflow-y: auto;
    }
    .table-row {
      display: grid;
      gap: var(--space-2);
      align-items: center;
      padding: var(--space-2-5) var(--space-4);
      font-size: var(--text-sm);
      border-bottom: 1px solid var(--border);
    }
    .table-row:last-child {
      border-bottom: none;
    }
    .table-row:hover {
      background: var(--accent);
    }
    .cols-member {
      grid-template-columns: 1fr 7rem 9rem;
    }
    .cols-actions-right {
      text-align: right;
    }

    /* Member cell */
    .member-cell {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      min-width: 0;
    }
    .member-info {
      min-width: 0;
    }
    .member-name {
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .member-email {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    /* Actions */
    .actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: var(--space-1);
    }
    .icon-btn {
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
      transition:
        background var(--dur-fast) var(--ease-1),
        color var(--dur-fast) var(--ease-1);
    }
    .icon-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .icon-btn.destructive:hover {
      background: var(--destructive);
      color: var(--destructive-foreground);
    }
    .icon-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* Inline role select (only visible while editing). */
    .role-select {
      width: var(--space-28);
    }

    /* Empty / loading */
    .empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: var(--space-16) 0;
      gap: var(--space-2);
      color: var(--muted-foreground);
    }
    .empty-icon {
      color: var(--muted-foreground);
      margin-bottom: var(--space-2);
    }
    .empty-text {
      font-size: var(--text-sm);
    }
    .pmv-loading {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
      padding: var(--space-4);
    }
    .pmv-sk-row {
      display: flex;
      align-items: center;
      gap: var(--space-3);
      padding: var(--space-2) var(--space-3);
    }
    .pmv-sk-avatar {
      width: var(--space-8);
      height: var(--space-8);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .pmv-sk-name {
      height: var(--space-3-5);
      width: 40%;
      border-radius: var(--radius-sm);
    }
    .pmv-sk-role {
      height: var(--space-3);
      width: var(--space-20);
      border-radius: var(--radius-full);
      margin-left: auto;
    }

    /* Add-dialog user list */
    .add-body {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
    }
    .add-role-row {
      display: flex;
      align-items: center;
      gap: var(--space-3);
    }
    .add-role-label {
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
    .add-role-select {
      width: var(--space-32);
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
    .add-footer {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
      width: 100%;
    }
  `;

  @property({ type: String })
  projectId = "";

  /** Whether the current user can manage this project's members
   * (project:members.manage). Passed in from the project detail page, which
   * derives it from the backend `my-access` response. */
  @property({ type: Boolean })
  canManage = false;

  @state()
  private _members: DtoProjectMemberResponse[] = [];

  @state()
  private _loading = true;

  /* Role editing state (click-to-edit, like the workspace members page) */
  @state()
  private _editingRole: string | null = null;

  @state()
  private _actingOn: string | null = null;

  /* Add-dialog state */
  @state()
  private _addDialogOpen = false;

  @state()
  private _searchQuery = "";

  @state()
  private _searchResults: DtoUserResponse[] = [];

  @state()
  private _searchLoading = false;

  @state()
  private _selectedIds: string[] = [];

  @state()
  private _adding = false;

  @state()
  private _addError = "";

  /** Role assigned to newly added members (picked in the add dialog). */
  @state()
  private _addRole: "admin" | "member" | "viewer" | "guest" = "member";

  private _debounceTimer = 0;
  private _addSearchRef = createRef<PlumeInput>();

  /* Remove-dialog state */
  @state()
  private _removeConfirmId: string | null = null;

  @state()
  private _removeConfirmName = "";

  @state()
  private _removeError = "";

  protected willUpdate(changed: Map<string, unknown>): void {
    if (changed.has("projectId") && this.projectId) {
      void this.#load();
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
  }

  async #load() {
    this._loading = true;
    try {
      const { data } = await getProjectsByIdMembers({
        path: { id: this.projectId },
        throwOnError: true,
      });
      this._members = data.items ??
        [];
    } catch {
      this._members = [];
    } finally {
      this._loading = false;
    }
  }

  /* Add dialog */

  async #openAddDialog() {
    this._addDialogOpen = true;
    this._searchQuery = "";
    this._searchResults = [];
    this._selectedIds = [];
    this._addError = "";
    this._adding = false;
    this._addRole = "member";
    await this.#searchUsers("");
    await this.updateComplete;
    this._addSearchRef.value?.focus();
  }

  #onSearchInput(q: string) {
    this._searchQuery = q;
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
    this._debounceTimer = window.setTimeout(() => {
      this._debounceTimer = 0;
      this.#searchUsers(q);
    }, 150);
  }

  async #searchUsers(q: string) {
    this._searchLoading = true;
    try {
      if (!q || q.length < 1) {
        const { data } = await getUsers({
          query: { limit: 50 },
          throwOnError: true,
        });
        this._searchResults = data.items ?? [];
      } else {
        const { data } = await getUsers({
          query: { search: q, limit: 25 },
          throwOnError: true,
        });
        this._searchResults = data.items ?? [];
      }
    } catch {
      this._searchResults = [];
    }
    this._searchLoading = false;
  }

  /** Users not already in the project. */
  get #addableUsers(): DtoUserResponse[] {
    const memberIds = new Set(this._members.map((m) => m.id));
    return (this._searchResults ?? []).filter((u) => !memberIds.has(u.id));
  }

  #toggleUser(id: string) {
    if (this._selectedIds.includes(id)) {
      this._selectedIds = this._selectedIds.filter((x) => x !== id);
    } else {
      this._selectedIds = [...this._selectedIds, id];
    }
  }

  async #addSelected() {
    if (this._selectedIds.length === 0) {
      this._addError = "Select at least one user.";
      return;
    }
    this._adding = true;
    this._addError = "";

    const results = await Promise.allSettled(
      this._selectedIds.map((uid) =>
        postProjectsByIdMembers({
          path: { id: this.projectId },
          body: { user_id: uid, role: this._addRole },
          throwOnError: true,
        })
      ),
    );

    const failed = this._selectedIds.filter((_, i) =>
      results[i].status === "rejected"
    );
    const succeeded = this._selectedIds.filter((_, i) =>
      results[i].status === "fulfilled"
    );

    this._adding = false;

    if (succeeded.length > 0) {
      await this.#load();
    }

    if (failed.length > 0) {
      // Keep only the failed selections so the user can retry / inspect.
      this._selectedIds = failed;
      this._addError = succeeded.length > 0
        ? `Added ${succeeded.length}, but ${failed.length} failed. They may already be members.`
        : `Failed to add ${failed.length} member${
          failed.length > 1 ? "s" : ""
        }. They may already be members.`;
    } else {
      this._addDialogOpen = false;
      this._selectedIds = [];
      this._searchQuery = "";
    }
  }

  /* Remove dialog */

  #confirmRemoveMember(userId: string, name: string) {
    this._removeError = "";
    this._removeConfirmId = userId;
    this._removeConfirmName = name;
  }

  #cancelRemove() {
    this._removeConfirmId = null;
    this._removeError = "";
  }

  async #executeRemoveMember() {
    const userId = this._removeConfirmId;
    if (!userId) return;
    this._actingOn = userId;
    this._removeError = "";
    try {
      await deleteProjectsByIdMembersByUserId({
        path: { id: this.projectId, userId },
        throwOnError: true,
      });
      this._removeConfirmId = null;
      await this.#load();
    } catch (err) {
      this._removeError = err instanceof Error
        ? err.message
        : msg("Failed to remove member. Please try again.");
    } finally {
      this._actingOn = null;
    }
  }

  /* Role update */

  async #updateRole(userId: string, role: string) {
    this._actingOn = userId;
    try {
      await putProjectsByIdMembersByUserId({
        path: { id: this.projectId, userId },
        body: { role: role as "admin" | "member" | "viewer" | "guest" },
        throwOnError: true,
      });
      this._editingRole = null;
      await this.#load();
    } catch {
      // keep editing state so user can retry
    }
    this._actingOn = null;
  }

  /* Render */

  protected render() {
    if (this._loading) {
      return html`
        <div class="pmv-loading">
          ${[1, 2, 3, 4, 5].map(() =>
            html`
              <div class="pmv-sk-row">
                <div class="pmv-sk-avatar skeleton-shimmer"></div>
                <div class="pmv-sk-name skeleton-shimmer"></div>
                <div class="pmv-sk-role skeleton-shimmer"></div>
              </div>
            `
          )}
        </div>
      `;
    }

    const count = this._members.length;

    return html`
      <div class="pmv-head">
        <span class="pmv-count">
          ${count} member${count === 1 ? "" : "s"}
        </span>
        ${this.canManage
          ? html`
            <plume-button size="sm" type="button" @click="${this
              .#openAddDialog}">
              <plume-icon name="plus" size="14"></plume-icon>
              Add member
            </plume-button>
          `
          : nothing}
      </div>

      ${this._members.length === 0
        ? html`
          <div class="empty">
            <plume-icon class="empty-icon" name="users" size="32"></plume-icon>
            <span class="empty-text">${msg("No members found.")}</span>
          </div>
        `
        : html`
          <div class="table">
            <div class="table-header cols-member">
              <span>${msg("Member")}</span>
              <span>${msg("Role")}</span>
              <span class="cols-actions-right">${msg("Actions")}</span>
            </div>
            <div class="table-body">
              ${this._members.map((m) => this.#renderMemberRow(m))}
            </div>
          </div>
        `}

      <!-- Add-dialog -->
      <plume-dialog
        .open="${this._addDialogOpen}"
        heading=${msg("Add members")}
        @close="${() => {
          this._addDialogOpen = false;
        }}"
      >
        <div class="add-body">
          <plume-input
            type="search"
            placeholder=${msg("Search users…")}
            .value="${this._searchQuery}"
            ${ref(this._addSearchRef)}
            @input="${(e: Event) => {
              this.#onSearchInput((e.target as HTMLInputElement).value);
            }}"
          ></plume-input>

          <div class="add-role-row">
            <label class="add-role-label" for="pmv-add-role">${msg(
              "Project role",
            )}</label>
            <plume-select
              id="pmv-add-role"
              class="add-role-select"
              .options="${getROLE_OPTIONS()}"
              .value="${this._addRole}"
              @change="${(e: CustomEvent) => {
                this._addRole = e.detail as
                  | "admin"
                  | "member"
                  | "viewer"
                  | "guest";
              }}"
            ></plume-select>
          </div>

          <div class="user-list">
            <div class="user-list-inner">
              ${this._searchLoading
                ? html`
                  <div class="hint">
                    <plume-spinner size="16"></plume-spinner>
                  </div>
                `
                : this.#addableUsers.length === 0
                ? html`
                  <div class="hint">${msg("No users to add.")}</div>
                `
                : this.#addableUsers.map((u) => {
                  const uid = u.id ?? "";
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
                        <div class="user-name">
                          ${u.name ?? "Unknown"}
                        </div>
                        ${u.email
                          ? html`
                            <div class="user-email">${u.email}</div>
                          `
                          : nothing}
                      </div>
                      <span
                        class="check-box ${selected ? "checked" : ""}"
                      >
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

          ${this._selectedIds.length > 0
            ? html`
              <p class="selection-info">
                ${this._selectedIds.length} selected
              </p>
            `
            : nothing} ${this._addError
            ? html`
              <p class="error">${this._addError}</p>
            `
            : nothing}
        </div>
        <div slot="footer" class="add-footer">
          <plume-button
            variant="ghost"
            type="button"
            @click="${() => {
              this._addDialogOpen = false;
            }}"
          >
            Cancel
          </plume-button>
          <plume-button
            type="button"
            ?disabled="${this._adding || this._selectedIds.length === 0}"
            @click="${this.#addSelected}"
          >
            ${this._adding
              ? "Adding…"
              : `Add selected (${this._selectedIds.length})`}
          </plume-button>
        </div>
      </plume-dialog>

      <!-- Remove-dialog -->
      ${this._removeConfirmId
        ? html`
          <plume-dialog
            style="--dialog-w:24rem"
            .open="${true}"
            heading=${msg("Remove member")}
            @close="${this.#cancelRemove}"
          >
            <p style="font-size:var(--text-sm);margin:0">
              Are you sure you want to remove
              <strong>${this._removeConfirmName}</strong>
              from this project?
            </p>
            ${this._removeError
              ? html`
                <p class="error" style="margin-top:var(--space-2)">
                  ${this._removeError}
                </p>
              `
              : nothing}
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <plume-button
                variant="ghost"
                type="button"
                @click="${this.#cancelRemove}"
              >
                Cancel
              </plume-button>
              <plume-button
                variant="destructive"
                type="button"
                ?disabled="${this._actingOn === this._removeConfirmId}"
                @click="${this.#executeRemoveMember}"
              >
                ${this._actingOn === this._removeConfirmId
                  ? "Removing…"
                  : "Remove"}
              </plume-button>
            </div>
          </plume-dialog>
        `
        : nothing}
    `;
  }

  #renderMemberRow(m: DtoProjectMemberResponse) {
    const id = m.id ?? "";
    const name = m.name ?? "Unknown";
    const email = m.email ?? "";
    const role = m.role ?? "member";
    const isEditing = this.canManage && this._editingRole === id;
    const isBusy = this._actingOn === id;

    return html`
      <div class="table-row cols-member">
        <div class="member-cell">
          <plume-avatar size="sm">
            ${name.charAt(0).toUpperCase()}
          </plume-avatar>
          <div class="member-info">
            <div class="member-name">${name}</div>
            ${email
              ? html`
                <div class="member-email">${email}</div>
              `
              : nothing}
          </div>
        </div>
        <div>
          ${isEditing
            ? html`
              <plume-select
                class="role-select"
                .options="${getROLE_OPTIONS()}"
                .value="${role}"
                @change="${(e: CustomEvent) => {
                  this.#updateRole(id, e.detail as string);
                }}"
              ></plume-select>
            `
            : html`
              <plume-role-badge .role="${role}"></plume-role-badge>
            `}
        </div>
        <div class="actions">
          ${this.canManage && (m.role_overridable ?? false)
            ? html`
              ${!isEditing
                ? html`
                  <button
                    class="icon-btn"
                    type="button"
                    aria-label=${msg("Change role")}
                    title=${msg("Change role")}
                    ?disabled="${isBusy}"
                    @click="${() => {
                      this._editingRole = id;
                    }}"
                  >
                    <plume-icon name="pencil" size="14"></plume-icon>
                  </button>
                `
                : nothing}
              <button
                class="icon-btn destructive"
                type="button"
                aria-label=${msg("Remove member")}
                title=${msg("Remove member")}
                ?disabled="${isBusy}"
                @click="${() => this.#confirmRemoveMember(id, name)}"
              >
                <plume-icon name="x" size="14"></plume-icon>
              </button>
            `
            : nothing}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-project-members-view": PlumeProjectMembersView;
  }
}
