import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { keyed } from "lit/directives/keyed.js";
import { pageEnterStyles, tabContentStyles } from "@/styles/shared-animations";
import { SignalController } from "@/lib/signal-controller";
import { loadWithMinTime } from "@/lib/async";
import { canManageOrg } from "@/lib/permissions";
import { auth } from "@/store/auth";
import { inviteDialogOpen } from "./store";
import { membersApi } from "./api";
import type { DtoUserResponse } from "@/api";
import type { InviteItem } from "./api";
import "./components/role-badge.ts";
import "./components/invite-dialog.ts";
import "./components/member-projects-dialog.ts";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/select.ts";
import "@/components/ui/spinner.ts";
import "@/components/ui/skeleton.ts";
import "@/components/ui/tabs.ts";
import "@/components/ui/avatar.ts";
import "@/components/ui/breeze-icon.ts";
import "@/layouts/app-layout.ts";

/** @see getROLE_OPTIONS */
function getROLE_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "owner", label: msg("Owner") },
    { value: "admin", label: msg("Admin") },
    { value: "member", label: msg("Member") },
    { value: "viewer", label: msg("Viewer") },
    { value: "guest", label: msg("Guest") },
  ];
}

function getROLE_FILTER_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "", label: msg("All roles") },
    ...getROLE_OPTIONS(),
  ];
}

function getTABS(): { id: string; label: string }[] {
  return [
    { id: "active", label: msg("Active") },
    { id: "invites", label: msg("Invites") },
    { id: "inactive", label: msg("Inactive") },
  ];
}

type TabId = "active" | "invites" | "inactive";

@localized()
@customElement("breeze-members-page")
export class BreezeMembersPage extends LitElement {
  static styles = [
    pageEnterStyles,
    tabContentStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: contents;
      }
      .page {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      .header {
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
      }
      .header-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .header-title {
        font-size: var(--text-lg);
        font-weight: 600;
        font-family: var(--font-heading, inherit);
        margin: 0;
      }
      .header-sub {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        margin: var(--space-1) 0 0;
      }
      .tab-bar {
        padding: var(--space-2) var(--space-6) 0;
      }
      .content {
        flex: 1;
        padding: var(--space-6);
        overflow: auto;
        min-height: 0;
      }
      /* Toolbar */
      .toolbar {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        margin-bottom: var(--space-4);
      }
      .search-wrap {
        flex: 1;
        max-width: var(--space-80);
      }
      .role-filter {
        width: var(--space-36);
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
      /* Column layouts */
      .cols-member {
        grid-template-columns: 1fr 6rem 11rem;
      }
      .cols-invite {
        grid-template-columns: 1fr 4rem 7rem 6rem;
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
      /* Inline role select */
      .role-select {
        width: var(--space-28);
      }
      /* Empty state */
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
      /* Skeleton */
      .skeleton-row {
        display: grid;
        gap: var(--space-2);
        align-items: center;
        padding: var(--space-2-5) var(--space-4);
        border-bottom: 1px solid var(--border);
      }
      .skeleton-cell {
        height: var(--space-4);
        border-radius: var(--radius-sm);
        background: linear-gradient(
          90deg,
          var(--muted) 0%,
          color-mix(in oklch, var(--muted) 60%, var(--foreground) 5%) 40%,
          var(--muted) 80%
        );
        background-size: 200% 100%;
        animation: shimmer var(--dur-slow) var(--ease-1) infinite;
      }
      /* Invite role cell */
      .invite-role-cell {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        min-width: 0;
      }
      .invite-email {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .text-xs-muted {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
    `,
  ];

  #signals = new SignalController(this);

  @state()
  private _tab: TabId = "active";

  @state()
  private _search = "";

  @state()
  private _roleFilter = "";

  // Active members
  @state()
  private _members: DtoUserResponse[] = [];

  @state()
  private _membersLoading = true;

  /** Set when the active-members request fails so an error state can show. */
  @state()
  private _membersError = false;

  // Invites
  @state()
  private _invites: InviteItem[] = [];

  @state()
  private _invitesLoading = false;

  // Inactive members
  @state()
  private _inactive: DtoUserResponse[] = [];

  @state()
  private _inactiveLoading = false;

  /** Set when the inactive-members request fails so an error state can show. */
  @state()
  private _inactiveError = false;

  // Editing state
  @state()
  private _editingRole: string | null = null;

  // Project management dialog
  @state()
  private _projectsTarget: { id: string; name: string } | null = null;

  // Debounce
  private _debounceTimer = 0;

  // Track which user is being acted on
  @state()
  private _actingOn: string | null = null;

  // Confirmation dialog state
  @state()
  private _confirmAction: {
    type: "deactivate" | "reactivate" | "revoke" | "remove";
    id: string;
    label: string;
  } | null = null;

  // Pagination
  @state()
  private _cursor: string | undefined = undefined;

  @state()
  private _hasMore = false;

  @state()
  private _loadingMore = false;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(auth);
    this.#loadMembers();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._debounceTimer) clearTimeout(this._debounceTimer);
  }

  protected updated(changed: Map<string, unknown>): void {
    if (changed.has("_search") || changed.has("_roleFilter")) {
      if (this._debounceTimer) clearTimeout(this._debounceTimer);
      this._debounceTimer = window.setTimeout(() => {
        this._debounceTimer = 0;
        this.#loadMembers();
      }, 200);
    }
  }

  async #loadMembers(append = false) {
    await loadWithMinTime(async () => {
      if (append) {
        this._loadingMore = true;
      } else {
        this._membersLoading = true;
        this._membersError = false;
        this._cursor = undefined;
      }
      try {
        const result = await membersApi.list({
          cursor: append ? this._cursor : undefined,
          search: this._search || undefined,
          role: this._roleFilter || undefined,
          limit: 50,
        });
        const filtered = (result.items ?? []).filter((u) => u.is_active);
        if (append) {
          this._members = [...this._members, ...filtered];
        } else {
          this._members = filtered;
        }
        this._hasMore = result.has_more ?? false;
        this._cursor = result.next_cursor || undefined;
      } catch {
        this._membersError = true;
        if (!append) this._members = [];
        this._hasMore = false;
      }
    }, (l) => {
      if (append) {
        this._loadingMore = !l;
      } else {
        this._membersLoading = l;
      }
    }, 200);
  }

  async #loadInvites() {
    await loadWithMinTime(async () => {
      this._invites = await membersApi.listInvites();
    }, (l) => {
      this._invitesLoading = l;
    }, 200);
  }

  async #loadInactive(append = false) {
    await loadWithMinTime(async () => {
      if (append) {
        this._loadingMore = true;
      } else {
        this._inactiveLoading = true;
        this._inactiveError = false;
        this._cursor = undefined;
      }
      try {
        const result = await membersApi.list({
          cursor: append ? this._cursor : undefined,
          include_inactive: true,
          limit: 50,
        });
        const filtered = (result.items ?? []).filter((u) => !u.is_active);
        if (append) {
          this._inactive = [...this._inactive, ...filtered];
        } else {
          this._inactive = filtered;
        }
        this._hasMore = result.has_more ?? false;
        this._cursor = result.next_cursor || undefined;
      } catch {
        this._inactiveError = true;
        if (!append) this._inactive = [];
        this._hasMore = false;
      }
    }, (l) => {
      if (append) {
        this._loadingMore = !l;
      } else {
        this._inactiveLoading = l;
      }
    }, 200);
  }

  #onTabChange(e: CustomEvent) {
    const id = e.detail as TabId;
    this._tab = id;
    if (id === "invites" && this._invites.length === 0) this.#loadInvites();
    if (id === "inactive" && this._inactive.length === 0) this.#loadInactive();
  }

  async #updateRole(userId: string, role: string) {
    this._actingOn = userId;
    try {
      await membersApi.updateRole(
        userId,
        role as "owner" | "admin" | "member" | "viewer",
      );
      this._editingRole = null;
      await this.#loadMembers();
      if (this._tab === "inactive") await this.#loadInactive();
    } catch {
      // Keep editing state so user can retry
    }
    this._actingOn = null;
  }

  #confirmToggleActive(userId: string, active: boolean, label: string) {
    this._confirmAction = {
      type: active ? "reactivate" : "deactivate",
      id: userId,
      label,
    };
  }

  #confirmRevokeInvite(inviteId: string, label: string) {
    this._confirmAction = {
      type: "revoke",
      id: inviteId,
      label,
    };
  }

  async #executeConfirmedAction() {
    const action = this._confirmAction;
    if (!action) return;
    this._confirmAction = null;
    this._actingOn = action.id;
    try {
      switch (action.type) {
        case "deactivate":
          await membersApi.updateActive(action.id, false);
          await this.#loadMembers();
          await this.#loadInactive();
          break;
        case "reactivate":
          await membersApi.updateActive(action.id, true);
          await this.#loadMembers();
          await this.#loadInactive();
          break;
        case "revoke":
          await membersApi.revokeInvite(action.id);
          await this.#loadInvites();
          break;
      }
    } catch {
      // silent
    }
    this._actingOn = null;
  }

  #cancelConfirm() {
    this._confirmAction = null;
  }

  async #copyInviteLink(inviteId: string) {
    const url = `${window.location.origin}/join?token=${inviteId}`;
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      // silent
    }
  }

  #canManage(): boolean {
    return canManageOrg(auth.value.user?.role);
  }

  #canInvite(): boolean {
    return this.#canManage();
  }

  #renderSkeleton(cols: string, rows: number) {
    return Array.from({ length: rows }).map(() =>
      html`
        <div class="skeleton-row" style="grid-template-columns:${cols}">
          <div class="skeleton-cell"></div>
          <div class="skeleton-cell"></div>
          <div class="skeleton-cell"></div>
        </div>
      `
    );
  }

  #renderMemberRow(user: DtoUserResponse) {
    const name = user.name ?? msg("Unknown");
    const email = user.email ?? "";
    const role = user.role ?? "member";
    const isEditing = this._editingRole === user.id;
    const isBusy = this._actingOn === user.id || false;

    return html`
      <div class="table-row cols-member">
        <div class="member-cell">
          <breeze-avatar size="sm">
            ${name.charAt(0).toUpperCase()}
          </breeze-avatar>
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
          ${isEditing && this.#canManage()
            ? html`
              <breeze-select
                class="role-select"
                .options="${getROLE_OPTIONS()}"
                .value="${role}"
                @change="${(e: CustomEvent) => {
                  this.#updateRole(user.id!, e.detail as string);
                }}"
              ></breeze-select>
            `
            : html`
              <breeze-role-badge .role="${role}"></breeze-role-badge>
            `}
        </div>
        <div class="actions cols-actions-right">
          ${this.#canManage()
            ? html`
              ${!isEditing
                ? html`
                  <button
                    class="icon-btn"
                    aria-label="${msg("Change role")}"
                    title="${msg("Change role")}"
                    ?disabled="${isBusy}"
                    @click="${() => {
                      this._editingRole = user.id!;
                    }}"
                  >
                    <breeze-icon name="pencil" size="14"></breeze-icon>
                  </button>
                `
                : nothing}
              <button
                class="icon-btn"
                aria-label="${msg("Manage projects")}"
                title="${msg("Manage projects")}"
                ?disabled="${isBusy}"
                @click="${() => {
                  this._projectsTarget = { id: user.id!, name };
                }}"
              >
                <breeze-icon name="folder" size="14"></breeze-icon>
              </button>
              <button
                class="icon-btn destructive"
                aria-label="${user.is_active
                  ? msg("Deactivate")
                  : msg("Reactivate")}"
                title="${user.is_active
                  ? msg("Deactivate")
                  : msg("Reactivate")}"
                ?disabled="${isBusy}"
                @click="${() =>
                  this.#confirmToggleActive(user.id!, !user.is_active, name)}"
              >
                ${user.is_active
                  ? html`
                    <breeze-icon name="x" size="14"></breeze-icon>
                  `
                  : html`
                    <breeze-icon name="reply" size="14"></breeze-icon>
                  `}
              </button>
            `
            : nothing}
        </div>
      </div>
    `;
  }

  #renderInviteRow(invite: InviteItem) {
    const isBusy = this._actingOn === invite.id || false;

    return html`
      <div class="table-row cols-invite">
        <div class="invite-role-cell">
          <breeze-role-badge .role="${invite.role}"></breeze-role-badge>
          ${invite.email
            ? html`
              <span class="invite-email">&rarr; ${invite.email}</span>
            `
            : nothing}
        </div>
        <span class="text-xs-muted">${invite.use_count}</span>
        <span
          class="text-xs-muted"
          style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap"
        >
          ${invite.invited_by_name}
        </span>
        <div class="actions">
          <button
            class="icon-btn"
            aria-label=${msg("Copy invite link")}
            title=${msg("Copy link")}
            @click="${() => this.#copyInviteLink(invite.id)}"
          >
            <breeze-icon name="link" size="14"></breeze-icon>
          </button>
          <button
            class="icon-btn destructive"
            aria-label=${msg("Revoke invite")}
            title=${msg("Revoke")}
            ?disabled="${isBusy}"
            @click="${() => this.#confirmRevokeInvite(invite.id, invite.role)}"
          >
            <breeze-icon name="trash-2" size="14"></breeze-icon>
          </button>
        </div>
      </div>
    `;
  }

  #renderActiveTab() {
    return html`
      <div class="toolbar">
        <breeze-input
          class="search-wrap"
          type="search"
          placeholder="${msg("Search members...")}"
          .value="${this._search}"
          @input="${(e: Event) => {
            this._search = (e.target as HTMLInputElement).value;
          }}"
        ></breeze-input>
        <breeze-select
          class="role-filter"
          .options="${getROLE_FILTER_OPTIONS()}"
          .value="${this._roleFilter}"
          placeholder=${msg("All roles")}
          @change="${(e: CustomEvent) => {
            this._roleFilter = e.detail as string;
          }}"
        ></breeze-select>
        ${this._membersLoading
          ? html`
            <breeze-skeleton
              variant="text"
              count="5"
              height="2.5rem"
            ></breeze-skeleton>
          `
          : nothing}
      </div>

      <div class="table">
        <div class="table-header cols-member">
          <span>${msg("Member")}</span>
          <span>${msg("Role")}</span>
          <span class="cols-actions-right">${msg("Actions")}</span>
        </div>
        <div class="table-body">
          ${this._membersLoading && this._members.length === 0
            ? this.#renderSkeleton("1fr 6rem 9rem", 5)
            : this._membersError && this._members.length === 0
            ? html`
              <div class="empty">
                <breeze-icon
                  class="empty-icon"
                  name="alert-circle"
                  size="32"
                ></breeze-icon>
                <span class="empty-text">${msg(
                  "Unable to load members.",
                )}</span>
              </div>
            `
            : this._members.length === 0
            ? html`
              <div class="empty">
                <breeze-icon class="empty-icon" name="users" size="32"></breeze-icon>
                <span class="empty-text">
                  ${this._search || this._roleFilter
                    ? msg("No members match your filters.")
                    : msg("No active members.")}
                </span>
              </div>
            `
            : this._members.map((u) => this.#renderMemberRow(u))}
        </div>
        ${this._hasMore
          ? html`
            <div style="display:flex;justify-content:center;padding:var(--space-3)">
              <breeze-button
                variant="ghost"
                size="sm"
                ?disabled="${this._loadingMore || this._membersLoading}"
                @click="${() => this.#loadMembers(true)}"
              >
                ${this._loadingMore ? msg("Loading...") : msg("Load more")}
              </breeze-button>
            </div>
          `
          : nothing}
      </div>
    `;
  }

  #renderInvitesTab() {
    return html`
      <div class="table">
        <div class="table-header cols-invite">
          <span>${msg("Role")}</span>
          <span>${msg("Uses")}</span>
          <span>${msg("Invited by")}</span>
          <span class="cols-actions-right">${msg("Actions")}</span>
        </div>
        <div class="table-body">
          ${this._invitesLoading && this._invites.length === 0
            ? this.#renderSkeleton("1fr 4rem 7rem 6rem", 3)
            : this._invites.length === 0
            ? html`
              <div class="empty">
                <breeze-icon class="empty-icon" name="mail" size="32"></breeze-icon>
                <span class="empty-text">${msg("No pending invites.")}</span>
                <breeze-button
                  variant="ghost"
                  size="sm"
                  type="button"
                  @click="${() => {
                    inviteDialogOpen.value = true;
                  }}"
                >
                  Create invite
                </breeze-button>
              </div>
            `
            : this._invites.map((inv) => this.#renderInviteRow(inv))}
        </div>
      </div>
    `;
  }

  #renderInactiveTab() {
    return html`
      <div class="table">
        <div class="table-header cols-member">
          <span>${msg("Member")}</span>
          <span>${msg("Role")}</span>
          <span class="cols-actions-right">${msg("Actions")}</span>
        </div>
        <div class="table-body">
          ${this._inactiveLoading && this._inactive.length === 0
            ? this.#renderSkeleton("1fr 6rem 9rem", 3)
            : this._inactiveError && this._inactive.length === 0
            ? html`
              <div class="empty">
                <breeze-icon
                  class="empty-icon"
                  name="alert-circle"
                  size="32"
                ></breeze-icon>
                <span class="empty-text">${msg(
                  "Unable to load members.",
                )}</span>
              </div>
            `
            : this._inactive.length === 0
            ? html`
              <div class="empty">
                <breeze-icon class="empty-icon" name="circle-check" size="32"></breeze-icon>
                <span class="empty-text">${msg(
                  "No deactivated members.",
                )}</span>
              </div>
            `
            : this._inactive.map((u) => this.#renderMemberRow(u))}
        </div>
        ${this._hasMore
          ? html`
            <div style="display:flex;justify-content:center;padding:var(--space-3)">
              <breeze-button
                variant="ghost"
                size="sm"
                ?disabled="${this._loadingMore || this._inactiveLoading}"
                @click="${() => this.#loadInactive(true)}"
              >
                ${this._loadingMore ? msg("Loading...") : msg("Load more")}
              </breeze-button>
            </div>
          `
          : nothing}
      </div>
    `;
  }

  protected render() {
    const canInvite = this.#canInvite();

    return html`
      <breeze-app-layout>
        <div class="page page-enter">
          <div class="header">
            <div class="header-row">
              <div>
                <h1 class="header-title">Members</h1>
                <p class="header-sub">Manage workspace members and invites</p>
              </div>
              ${canInvite
                ? html`
                  <breeze-button size="sm" @click="${() => {
                    inviteDialogOpen.value = true;
                  }}">
                    Invite member
                  </breeze-button>
                `
                : nothing}
            </div>
          </div>

          <div class="tab-bar">
            <breeze-tabs
              .tabs="${getTABS()}"
              .value="${this._tab}"
              @change="${this.#onTabChange}"
            ></breeze-tabs>
          </div>

          <div class="content">
            ${keyed(
              this._tab,
              html`
                <div class="tab-content" role="tabpanel" aria-labelledby="tab-${this
                  ._tab}">
                  ${this._tab === "active"
                    ? this.#renderActiveTab()
                    : this._tab === "invites"
                    ? this.#renderInvitesTab()
                    : this.#renderInactiveTab()}
                </div>
              `,
            )}
          </div>
        </div>
      </breeze-app-layout>

      <breeze-invite-dialog></breeze-invite-dialog>

      ${this._projectsTarget
        ? html`
          <breeze-member-projects-dialog
            .open="${true}"
            .userId="${this._projectsTarget.id}"
            .userName="${this._projectsTarget.name}"
            @close="${() => {
              this._projectsTarget = null;
            }}"
          ></breeze-member-projects-dialog>
        `
        : nothing} ${this._confirmAction
        ? html`
          <breeze-dialog
            style="--dialog-w:24rem"
            .open="${true}"
            heading="${this._confirmAction.type === "deactivate"
              ? msg("Deactivate user")
              : this._confirmAction.type === "reactivate"
              ? msg("Reactivate user")
              : msg("Revoke invite")}"
            @close="${this.#cancelConfirm}"
          >
            <p style="font-size:var(--text-sm);margin:0">
              ${this._confirmAction.type === "deactivate"
                ? html`
                  Are you sure you want to deactivate <strong>${this
                    ._confirmAction.label}</strong>? They will lose
                  access to the workspace.
                `
                : this._confirmAction.type === "reactivate"
                ? html`
                  Are you sure you want to reactivate <strong>${this
                    ._confirmAction.label}</strong>?
                `
                : html`
                  Are you sure you want to revoke this <strong>${this
                    ._confirmAction.label}</strong> invite?
                `}
            </p>
            <div
              slot="footer"
              style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
            >
              <breeze-button variant="ghost" type="button" @click="${this
                .#cancelConfirm}">
                ${msg("Cancel")}
              </breeze-button>
              <breeze-button
                variant="destructive"
                @click="${this.#executeConfirmedAction}"
              >
                ${this._confirmAction.type === "reactivate"
                  ? msg("Reactivate")
                  : msg("Deactivate")}
              </breeze-button>
            </div>
          </breeze-dialog>
        `
        : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-members-page": BreezeMembersPage;
  }
}
