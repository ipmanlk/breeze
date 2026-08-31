import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { settingsConvId, showChannelSettings } from "../store";
import { chatApi } from "../api";
import { activeConversation, conversationList } from "../store";
import { projects } from "@/store/projects";
import { getUsers } from "@/api";
import type { DtoUserResponse } from "@/api";
import type { Conversation, Member } from "../types";
import { navigate } from "@/routes/router";
import "@/components/ui/combobox.ts";
import "@/components/ui/button.ts";
import "@/components/ui/input.ts";
import "@/components/ui/plume-icon.ts";
import "@/components/ui/select.ts";
import "@/components/ui/avatar.ts";
import "@/components/ui/dialog.ts";
import { localized, msg } from "@lit/localize";

/**
 * Channel / category settings panel: right-side panel with collapsible
 * sections. Works for both `channel`/`voice` conversations and `category`
 * conversations (categories are conversations too, so the same permission,
 * project-link, and rename endpoints apply).
 *
 * Sections shown depend on the resolved permissions (my-permissions):
 *  - General (name + topic)       : requires can_manage
 *  - Linked Projects              : requires can_manage
 *  - Role Permissions (matrix)    : requires can_permissions
 *  - User Overrides               : requires can_permissions
 *  - Danger zone (delete)         : requires can_manage
 */
@localized()
@customElement("plume-channel-settings-panel")
export class PlumeChannelSettingsPanel extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    :host {
      width: var(--space-88);
      border-left: 1px solid var(--border);
      flex-shrink: 0;
      display: flex;
      flex-direction: column;
      background: var(--background);
      overflow-y: auto;
      animation: slide-in-right var(--dur-slow) var(--ease-2);
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--space-3);
      border-bottom: 1px solid var(--border);
      font-size: var(--text-xs);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted-foreground);
      position: sticky;
      top: 0;
      background: var(--background);
      z-index: 1;
    }
    .header button {
      background: none;
      border: none;
      color: var(--muted-foreground);
      cursor: pointer;
      padding: var(--space-1);
      border-radius: var(--radius-sm);
      display: inline-flex;
    }
    .header button:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .section {
      padding: var(--space-4) var(--space-3);
      border-bottom: 1px solid var(--border);
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
    }
    .section-title {
      font-size: var(--text-sm);
      font-weight: 600;
      color: var(--foreground);
    }
    .section-desc {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      line-height: 1.5;
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
    }
    .field label {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
    }
    .row-btns {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
    }
    .perm-table {
      display: grid;
      grid-template-columns: minmax(5rem, 1fr) repeat(4, 1.5fr);
      gap: 1px;
      background: var(--border);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      overflow: hidden;
      font-size: var(--text-xs);
    }
    .perm-cell {
      background: var(--background);
      padding: var(--space-1-5) var(--space-2);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: var(--space-8);
    }
    .perm-head {
      font-weight: 600;
      color: var(--muted-foreground);
      background: var(--muted);
    }
    .perm-role {
      justify-content: flex-start;
      font-weight: 500;
      color: var(--foreground);
      background: var(--muted);
    }
    .perm-toggle {
      cursor: pointer;
      gap: var(--space-1);
      transition: background var(--dur-fast) var(--ease-1);
      user-select: none;
    }
    .perm-toggle:hover {
      background: var(--accent);
    }
    .perm-toggle .dot {
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .perm-toggle.inherit .dot {
      background: var(--muted-foreground);
      opacity: 0.4;
    }
    .perm-toggle.inherit-allow .dot {
      background: var(--primary);
      opacity: 0.5;
    }
    .perm-toggle.inherit-allow .lbl {
      color: color-mix(in oklch, var(--primary) 50%, var(--muted-foreground));
    }
    .perm-toggle.inherit-deny .dot {
      background: var(--destructive);
      opacity: 0.5;
    }
    .perm-toggle.inherit-deny .lbl {
      color: color-mix(in oklch, var(--destructive) 50%, var(--muted-foreground));
    }
    .perm-toggle.allow .dot {
      background: var(--primary);
    }
    .perm-toggle.deny .dot {
      background: var(--destructive);
    }
    .perm-toggle .lbl {
      color: var(--muted-foreground);
    }
    .perm-toggle.allow .lbl {
      color: var(--primary);
      font-weight: 600;
    }
    .perm-toggle.deny .lbl {
      color: var(--destructive);
      font-weight: 600;
    }

    /* User overrides */
    .override-row {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-2);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
    }
    .override-info {
      flex: 1;
      min-width: 0;
    }
    .override-name {
      font-size: var(--text-sm);
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .override-perm {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .override-actions {
      display: flex;
      align-items: center;
      gap: var(--space-1);
    }
    .icon-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-7);
      height: var(--space-7);
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: var(--background);
      color: var(--muted-foreground);
      cursor: pointer;
    }
    .icon-btn:hover {
      background: var(--accent);
      color: var(--foreground);
    }
    .icon-btn.allow {
      color: var(--primary);
      border-color: color-mix(in oklch, var(--primary) 40%, var(--border));
    }
    .icon-btn.deny {
      color: var(--destructive);
      border-color: color-mix(in oklch, var(--destructive) 40%, var(--border));
    }
    .add-override {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
      padding: var(--space-2);
      border: 1px dashed var(--border);
      border-radius: var(--radius-md);
    }
    .add-override-row {
      display: flex;
      gap: var(--space-2);
      align-items: flex-end;
    }

    .danger-btn {
      width: 100%;
      justify-content: center;
    }
    .loading {
      padding: var(--space-8);
      text-align: center;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
    .saved-hint {
      font-size: var(--text-xs);
      color: var(--primary);
    }

    /* Members section */
    .member-list {
      display: flex;
      flex-direction: column;
      max-height: var(--space-48);
      overflow-y: auto;
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
    }
    .member-row {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-1-5) var(--space-2);
      font-size: var(--text-sm);
      border-bottom: 1px solid var(--border);
    }
    .member-row:last-child {
      border-bottom: none;
    }
    .member-name {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 500;
    }
    .member-role {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      flex-shrink: 0;
    }
    .member-remove {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--space-6);
      height: var(--space-6);
      border: none;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      flex-shrink: 0;
    }
    .member-remove:hover {
      background: var(--destructive);
      color: var(--destructive-foreground);
    }

    /* Add members sub-view */
    .add-member-search {
      width: 100%;
    }
    .add-member-list {
      display: flex;
      flex-direction: column;
      max-height: var(--space-40);
      overflow-y: auto;
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
    }
    .add-member-row {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: var(--space-1-5) var(--space-2);
      font-size: var(--text-sm);
      border-bottom: 1px solid var(--border);
      cursor: pointer;
      transition: background var(--dur-fast) var(--ease-1);
    }
    .add-member-row:last-child {
      border-bottom: none;
    }
    .add-member-row:hover {
      background: var(--accent);
    }
    .add-member-row.selected {
      background: color-mix(in oklch, var(--primary) 10%, transparent);
    }
    .add-member-check {
      display: flex;
      align-items: center;
      justify-content: center;
      width: var(--space-4);
      height: var(--space-4);
      border-radius: var(--radius-sm);
      border: 1px solid var(--border);
      flex-shrink: 0;
    }
    .add-member-row.selected .add-member-check {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
    }
    .add-member-info {
      flex: 1;
      min-width: 0;
    }
    .add-member-name {
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .add-member-email {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .add-member-actions {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
    }
  `;

  #signals = new SignalController(this);

  @property()
  conversationId = "";

  @state()
  private _loading = true;
  @state()
  private _canManage = false;
  @state()
  private _canPermissions = false;
  @state()
  private _conv: Conversation | null = null;
  @state()
  private _name = "";
  @state()
  private _topic = "";
  @state()
  private _projectIds: string[] = [];
  @state()
  private _rules: {
    role: string;
    permission: string;
    allow: boolean;
    explicit: boolean;
  }[] = [];
  @state()
  private _overrides: {
    user_id: string;
    permission: string;
    allow: boolean;
  }[] = [];
  @state()
  private _users: DtoUserResponse[] = [];
  @state()
  private _savingGeneral = false;
  @state()
  private _generalError = "";
  @state()
  private _generalSaved = false;
  @state()
  private _permError = "";
  @state()
  private _overrideError = "";
  @state()
  private _deleteOpen = false;
  @state()
  private _deleting = false;
  @state()
  private _newOverrideUser = "";
  @state()
  private _newOverridePerm = "channel:view";
  @state()
  private _newOverrideAllow = true;

  // Members section
  @state()
  private _members: Member[] = [];
  @state()
  private _membersLoading = false;
  @state()
  private _addMembersOpen = false;
  @state()
  private _addMemberEntries: {
    id: string;
    name: string;
    email: string;
    selected: boolean;
  }[] = [];
  @state()
  private _addMemberSearch = "";
  @state()
  private _addMemberSaving = false;
  @state()
  private _addMemberError = "";
  @state()
  private _memberError = "";

  #loadedId = "";

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(showChannelSettings, settingsConvId);
  }

  protected willUpdate(changed: Map<string, unknown>): void {
    if (
      changed.has("conversationId") && this.conversationId &&
      this.conversationId !== this.#loadedId
    ) {
      void this.#load();
    }
  }

  async #load() {
    if (!this.conversationId) return;
    this.#loadedId = this.conversationId;
    this._loading = true;
    this._generalSaved = false;
    try {
      const [perms, conv] = await Promise.all([
        chatApi.myPermissions(this.conversationId),
        chatApi.getConversation(this.conversationId),
      ]);
      this._canManage = perms.can_manage;
      this._canPermissions = perms.can_permissions;
      this._conv = conv;
      this._name = conv.name ?? "";
      this._topic = conv.topic ?? "";

      const tasks: Promise<unknown>[] = [];
      if (this._canPermissions) {
        tasks.push(
          chatApi.getPermissions(this.conversationId).then((r) =>
            this._rules = r
          ),
          chatApi.getUserOverrides(this.conversationId).then((r) =>
            this._overrides = r
          ),
          this.#loadUsers(),
        );
      }
      if (this._canManage) {
        tasks.push(
          chatApi.getProjectLinks(this.conversationId).then((l) =>
            this._projectIds = l.project_ids ?? []
          ),
          this.#loadMembers(),
        );
      }
      await Promise.all(tasks);
    } catch {
      // silently fail: panel may close
    } finally {
      this._loading = false;
    }
  }

  async #loadUsers() {
    try {
      const { data } = await getUsers({
        query: { limit: 100 },
        throwOnError: true,
      });
      this._users = data.items ?? [];
    } catch {
      this._users = [];
    }
  }

  /* Members */
  async #loadMembers() {
    this._membersLoading = true;
    try {
      this._members = await chatApi.listMembers(this.conversationId);
    } catch {
      this._members = [];
    }
    this._membersLoading = false;
  }

  async #removeMember(userId: string) {
    this._memberError = "";
    try {
      await chatApi.removeMember(this.conversationId, userId);
      this._members = this._members.filter((m) => m.id !== userId);
    } catch (err: unknown) {
      this._memberError = err instanceof Error
        ? err.message
        : msg("Failed to remove member");
    }
  }

  async #openAddMembers() {
    this._addMembersOpen = true;
    this._addMemberError = "";
    this._addMemberSearch = "";
    // Load users if not already loaded
    if (this._users.length === 0) {
      await this.#loadUsers();
    }
    const memberIds = new Set(this._members.map((m) => m.id));
    this._addMemberEntries = this._users
      .filter((u) => u.id && !memberIds.has(u.id))
      .map((u) => ({
        id: u.id!,
        name: u.name ?? u.email ?? u.id!,
        email: u.email ?? "",
        selected: false,
      }));
  }

  #toggleAddMember(id: string) {
    const entry = this._addMemberEntries.find((e) => e.id === id);
    if (!entry) return;
    entry.selected = !entry.selected;
    this.requestUpdate();
  }

  get #filteredAddMemberEntries() {
    if (!this._addMemberSearch) return this._addMemberEntries;
    const q = this._addMemberSearch.toLowerCase();
    return this._addMemberEntries.filter(
      (e) =>
        e.name.toLowerCase().includes(q) || e.email.toLowerCase().includes(q),
    );
  }

  get #selectedAddMemberIds(): string[] {
    return this._addMemberEntries.filter((e) => e.selected).map((e) => e.id);
  }

  async #addSelectedMembers() {
    const ids = this.#selectedAddMemberIds;
    if (ids.length === 0) return;
    this._addMemberSaving = true;
    this._addMemberError = "";
    try {
      await chatApi.addMembers(this.conversationId, ids);
      this._addMembersOpen = false;
      await this.#loadMembers();
    } catch (err: unknown) {
      this._addMemberError = err instanceof Error
        ? err.message
        : msg("Failed to add members");
    }
    this._addMemberSaving = false;
  }

  #close() {
    showChannelSettings.value = false;
    settingsConvId.value = null;
    // Remove ?settings=1 from URL
    const url = new URL(window.location.href);
    url.searchParams.delete("settings");
    window.history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  /* General */
  get #generalDirty() {
    return this._name !== (this._conv?.name ?? "") ||
      this._topic !== (this._conv?.topic ?? "");
  }

  async #saveGeneral() {
    if (!this.#generalDirty) return;
    this._savingGeneral = true;
    this._generalError = "";
    this._generalSaved = false;
    try {
      const updated = await chatApi.updateConversation(this.conversationId, {
        name: this._name,
        topic: this._topic,
      });
      this._conv = { ...this._conv, ...updated } as Conversation;
      this.#syncConversationList(updated);
      this._generalSaved = true;
      setTimeout(() => {
        this._generalSaved = false;
      }, 2000);
    } catch (err: unknown) {
      this._generalError = err instanceof Error
        ? err.message
        : msg("Failed to save.");
    } finally {
      this._savingGeneral = false;
    }
  }

  #syncConversationList(updated: Conversation) {
    conversationList.value = conversationList.value.map((c) =>
      c.id === this.conversationId ? { ...c, ...updated } : c
    );
    if (activeConversation.value?.id === this.conversationId) {
      activeConversation.value = {
        ...activeConversation.value,
        ...updated,
      } as Conversation;
    }
  }

  /* Linked projects */
  async #setProjectIds(ids: string[]) {
    this._projectIds = ids;
    try {
      await chatApi.setProjectLinks(this.conversationId, ids);
    } catch {
      void this.#load();
    }
  }

  /* Role permission matrix (3-state: inherit / allow / deny) */
  #ruleEffective(
    role: string,
    perm: string,
  ): { allow: boolean; explicit: boolean } {
    const r = this._rules.find((x) => x.role === role && x.permission === perm);
    if (!r) return { allow: false, explicit: false };
    return { allow: r.allow, explicit: r.explicit };
  }

  async #cyclePermission(role: string, perm: string) {
    const { allow, explicit } = this.#ruleEffective(role, perm);

    // Start with all existing explicit rules EXCEPT the one we're about to
    // toggle (if it was explicit we're either changing it or removing it).
    const explicitRules = this._rules
      .filter((r) => r.explicit && !(r.role === role && r.permission === perm))
      .map((r) => ({ role: r.role, permission: r.permission, allow: r.allow }));

    let addRule: { role: string; permission: string; allow: boolean } | null =
      null;

    if (!explicit) {
      // Inherited → create explicit allow override
      addRule = { role, permission: perm, allow: true };
    } else if (allow) {
      // Explicit allow → flip to deny
      addRule = { role, permission: perm, allow: false };
    } else {
      // Explicit deny → remove override (revert to inherited)
      // addRule stays null, so the rule is simply removed from the set
    }

    if (addRule) {
      explicitRules.push(addRule);
    }

    this._permError = "";
    try {
      await chatApi.setPermissions(this.conversationId, explicitRules);
      void this.#load();
    } catch (err: unknown) {
      this._permError = err instanceof Error
        ? err.message
        : msg("Failed to save.");
      void this.#load();
    }
  }

  /* User overrides */
  #userById(id: string): DtoUserResponse | undefined {
    return this._users.find((u) => u.id === id);
  }

  async #toggleOverride(userId: string, perm: string) {
    const existing = this._overrides.find((o) =>
      o.user_id === userId && o.permission === perm
    );
    if (!existing) return;
    const overrides = this._overrides.map((o) =>
      o.user_id === userId && o.permission === perm
        ? { ...o, allow: !o.allow }
        : o
    );
    this._overrides = overrides;
    try {
      await chatApi.setUserOverrides(this.conversationId, overrides);
    } catch {
      void this.#load();
    }
  }

  async #removeOverride(userId: string, perm: string) {
    const overrides = this._overrides.filter((o) =>
      !(o.user_id === userId && o.permission === perm)
    );
    this._overrides = overrides;
    try {
      await chatApi.setUserOverrides(this.conversationId, overrides);
    } catch {
      void this.#load();
    }
  }

  async #addOverride() {
    if (!this._newOverrideUser) {
      this._overrideError = "Select a user.";
      return;
    }
    if (
      this._overrides.some((o) =>
        o.user_id === this._newOverrideUser &&
        o.permission === this._newOverridePerm
      )
    ) {
      this._overrideError =
        "Override already exists for this user + permission.";
      return;
    }
    const overrides = [...this._overrides, {
      user_id: this._newOverrideUser,
      permission: this._newOverridePerm,
      allow: this._newOverrideAllow,
    }];
    this._overrides = overrides;
    this._newOverrideUser = "";
    this._overrideError = "";
    try {
      await chatApi.setUserOverrides(this.conversationId, overrides);
    } catch (err: unknown) {
      this._overrideError = err instanceof Error
        ? err.message
        : msg("Failed to add.");
      void this.#load();
    }
  }

  /* Delete */
  async #confirmDelete() {
    this._deleting = true;
    try {
      await chatApi.deleteConversation(this.conversationId);
      conversationList.value = conversationList.value.filter((c) =>
        c.id !== this.conversationId
      );
      if (activeConversation.value?.id === this.conversationId) {
        activeConversation.value = null;
        navigate("/chat");
      }
      this.#close();
    } catch {
      // ignore
    } finally {
      this._deleting = false;
      this._deleteOpen = false;
    }
  }

  /* Render */
  protected render() {
    const isOpen = showChannelSettings.value;
    if (!isOpen || !this.conversationId) return null;

    const isCategory = this._conv?.type === "category";
    const heading = isCategory ? "Category Settings" : "Channel Settings";

    return html`
      <div class="header">
        <span>${heading}</span>
        <button @click="${this.#close}" title="Close">
          <plume-icon name="x" size="16"></plume-icon>
        </button>
      </div>

      ${this._loading
        ? html`
          <div class="loading">Loading…</div>
        `
        : html`
          ${this._canManage ? this.#renderGeneral() : nothing} ${this._canManage
            ? (this._addMembersOpen
              ? this.#renderAddMembers()
              : this.#renderMembers())
            : nothing} ${this._canManage
            ? this.#renderProjects(isCategory)
            : nothing} ${this._canPermissions
            ? this.#renderRoleMatrix()
            : nothing} ${this._canPermissions
            ? this.#renderUserOverrides()
            : nothing} ${this._canManage ? this.#renderDanger() : nothing}
        `} ${this._deleteOpen ? this.#renderDeleteDialog() : nothing}
    `;
  }

  #renderMembers() {
    const isDM = this._conv?.type === "direct";
    return html`
      <div class="section">
        <div class="section-title">Members: ${this._members.length}</div>
        ${this._membersLoading
          ? html`
            <div class="section-desc">Loading…</div>
          `
          : this._members.length === 0
          ? html`
            <div class="section-desc">No members</div>
          `
          : html`
            <div class="member-list">
              ${this._members.map((m) => {
                const initial = (m.name ?? "?").charAt(0).toUpperCase();
                return html`
                  <div class="member-row">
                    <plume-avatar size="sm">${initial}</plume-avatar>
                    <span class="member-name">${m.name ?? m.email}</span>
                    <span class="member-role">${m.role ?? "member"}</span>
                    ${!isDM
                      ? html`
                        <button
                          class="member-remove"
                          title=${msg("Remove member")}
                          @click="${() => this.#removeMember(m.id)}"
                        >
                          <plume-icon name="x" size="12"></plume-icon>
                        </button>
                      `
                      : nothing}
                  </div>
                `;
              })}
            </div>
          `} ${this._memberError
          ? html`
            <p class="error">${this._memberError}</p>
          `
          : nothing} ${!isDM
          ? html`
            <plume-button
              size="sm"
              variant="outline"
              @click="${this.#openAddMembers}"
            >
              <plume-icon name="user-plus" size="14"></plume-icon>
              Add members
            </plume-button>
          `
          : nothing}
      </div>
    `;
  }

  #renderAddMembers() {
    return html`
      <div class="section">
        <div class="section-title">Add Members</div>
        <plume-input
          class="add-member-search"
          type="search"
          placeholder=${msg("Search users…")}
          .value="${this._addMemberSearch}"
          @input="${(e: Event) => {
            this._addMemberSearch = (e.target as HTMLInputElement).value;
          }}"
        ></plume-input>
        ${this.#filteredAddMemberEntries.length === 0
          ? html`
            <div class="section-desc">
              ${this._addMemberEntries.length === 0
                ? "All organization members are already in this conversation"
                : "No users match your search"}
            </div>
          `
          : html`
            <div class="add-member-list">
              ${this.#filteredAddMemberEntries.map((entry) =>
                html`
                  <div
                    class="add-member-row ${entry.selected ? "selected" : ""}"
                    @click="${() => this.#toggleAddMember(entry.id)}"
                  >
                    <span class="add-member-check">
                      ${entry.selected
                        ? html`
                          <plume-icon name="check" size="10"></plume-icon>
                        `
                        : nothing}
                    </span>
                    <div class="add-member-info">
                      <div class="add-member-name">${entry.name}</div>
                      <div class="add-member-email">${entry.email}</div>
                    </div>
                  </div>
                `
              )}
            </div>
          `} ${this._addMemberError
          ? html`
            <p class="error">${this._addMemberError}</p>
          `
          : nothing}
        <div class="add-member-actions">
          <plume-button
            size="sm"
            variant="ghost"
            @click="${() => {
              this._addMembersOpen = false;
            }}"
          >
            Cancel
          </plume-button>
          <plume-button
            size="sm"
            ?disabled="${this._addMemberSaving ||
              this.#selectedAddMemberIds.length === 0}"
            @click="${this.#addSelectedMembers}"
          >
            ${this._addMemberSaving
              ? "Adding…"
              : `Add (${this.#selectedAddMemberIds.length})`}
          </plume-button>
        </div>
      </div>
    `;
  }

  #renderGeneral() {
    return html`
      <div class="section">
        <div class="section-title">General</div>
        <div class="field">
          <label>Name</label>
          <plume-input
            placeholder=${msg("Name")}
            .value="${this._name}"
            @input="${(e: Event) => {
              this._name = (e.target as HTMLInputElement).value;
              this._generalSaved = false;
            }}"
          ></plume-input>
        </div>
        <div class="field">
          <label>Description</label>
          <plume-input
            placeholder=${msg("Description (optional)")}
            .value="${this._topic}"
            @input="${(e: Event) => {
              this._topic = (e.target as HTMLInputElement).value;
              this._generalSaved = false;
            }}"
          ></plume-input>
        </div>
        ${this._generalError
          ? html`
            <p class="error">${this._generalError}</p>
          `
          : nothing} ${this._generalSaved
          ? html`
            <span class="saved-hint">Saved</span>
          `
          : nothing}
        <div class="row-btns">
          <plume-button
            size="sm"
            ?disabled="${this._savingGeneral || !this.#generalDirty}"
            @click="${this.#saveGeneral}"
          >
            ${this._savingGeneral ? "Saving…" : "Save changes"}
          </plume-button>
        </div>
      </div>
    `;
  }

  #renderProjects(isCategory: boolean) {
    const allProjects = projects.value;
    const options = allProjects.projects
      .map((p) => ({ value: p.id || "", label: p.name || "" }))
      .filter((o) => o.value);
    return html`
      <div class="section">
        <div class="section-title">Linked Projects</div>
        <div class="section-desc">
          ${isCategory
            ? "Child channels inherit these project links. Members of these projects get implicit access."
            : "Only members of these projects get implicit access to this channel."}
        </div>
        <plume-combobox
          .options="${options}"
          .value="${this._projectIds}"
          placeholder=${msg("Select projects…")}
          @change="${(e: Event) =>
            this.#setProjectIds((e as CustomEvent).detail as string[])}"
        ></plume-combobox>
      </div>
    `;
  }

  #renderRoleMatrix() {
    const roles = ["everyone", "member", "viewer", "guest"];
    const perms = [
      { key: "channel:view", label: msg("View") },
      { key: "channel:send", label: msg("Send") },
      { key: "channel:manage", label: msg("Manage") },
      { key: "channel:permissions", label: msg("Perms") },
    ];
    return html`
      <div class="section">
        <div class="section-title">Role Permissions</div>
        <div class="section-desc">
          Effective permissions for each role. A checked cell means the permission is
          granted; unchecked means denied. <strong>Bold</strong>
          cells are explicitly set at this level: click to toggle. Muted cells are
          inherited from the parent category or org default: click to create an
          override.
        </div>
        <div class="perm-table">
          <div class="perm-cell perm-head"></div>
          ${perms.map((p) =>
            html`
              <div class="perm-cell perm-head">${p.label}</div>
            `
          )} ${roles.map((role) =>
            html`
              <div class="perm-cell perm-role">${role}</div>
              ${perms.map((p) => {
                const { allow, explicit } = this.#ruleEffective(role, p.key);
                const cls = explicit
                  ? (allow ? "allow" : "deny")
                  : (allow ? "inherited-allow" : "inherited-deny");
                return html`
                  <div
                    class="perm-cell perm-toggle ${cls}"
                    role="button"
                    tabindex="0"
                    title="${explicit
                      ? (allow
                        ? "Allowed (explicit): click to deny"
                        : "Denied (explicit): click to remove override")
                      : (allow
                        ? "Inherited allowed: click to override & deny"
                        : "Inherited denied: click to override & allow")}"
                    @click="${() => this.#cyclePermission(role, p.key)}"
                  >
                    <span class="dot"></span>
                    <span class="lbl">${allow ? "✓" : "✗"}</span>
                  </div>
                `;
              })}
            `
          )}
        </div>
        ${this._permError
          ? html`
            <p class="error">${this._permError}</p>
          `
          : nothing}
      </div>
    `;
  }

  #renderUserOverrides() {
    const permOpts = [
      { value: "channel:view", label: msg("View") },
      { value: "channel:send", label: msg("Send") },
      { value: "channel:manage", label: msg("Manage") },
      { value: "channel:permissions", label: msg("Perms") },
    ];
    const userOpts = this._users
      .filter((u) => u.id)
      .map((u) => ({ value: u.id!, label: u.name ?? u.email ?? u.id! }));

    return html`
      <div class="section">
        <div class="section-title">User Overrides</div>
        <div class="section-desc">
          Grant or revoke a specific permission for an individual user. Overrides take
          priority over role rules.
        </div>

        ${this._overrides.length === 0
          ? html`
            <div class="section-desc">No user overrides yet.</div>
          `
          : this._overrides.map((o) => {
            const u = this.#userById(o.user_id);
            const initial = (u?.name ?? "?").charAt(0).toUpperCase();
            return html`
              <div class="override-row">
                <plume-avatar size="sm">${initial}</plume-avatar>
                <div class="override-info">
                  <div class="override-name">${u?.name ?? o.user_id}</div>
                  <div class="override-perm">${o.permission.split(":")[1]}</div>
                </div>
                <div class="override-actions">
                  <button
                    class="icon-btn ${o.allow ? "allow" : "deny"}"
                    title="${o.allow
                      ? "Allowed: click to deny"
                      : "Denied: click to allow"}"
                    @click="${() =>
                      this.#toggleOverride(o.user_id, o.permission)}"
                  >
                    <plume-icon name="${o.allow
                      ? "check"
                      : "x"}" size="14"></plume-icon>
                  </button>
                  <button
                    class="icon-btn"
                    title=${msg("Remove override")}
                    @click="${() =>
                      this.#removeOverride(o.user_id, o.permission)}"
                  >
                    <plume-icon name="trash-2" size="14"></plume-icon>
                  </button>
                </div>
              </div>
            `;
          })}

        <div class="add-override">
          <div class="add-override-row">
            <div class="field" style="flex:1">
              <label>User</label>
              <plume-select
                .options="${userOpts}"
                .value="${this._newOverrideUser}"
                placeholder=${msg("Select user…")}
                @change="${(e: CustomEvent) => {
                  this._newOverrideUser = e.detail as string;
                }}"
              ></plume-select>
            </div>
            <div class="field" style="width:7rem">
              <label>Permission</label>
              <plume-select
                .options="${permOpts}"
                .value="${this._newOverridePerm}"
                @change="${(e: CustomEvent) => {
                  this._newOverridePerm = e.detail as string;
                }}"
              ></plume-select>
            </div>
          </div>
          <div class="add-override-row">
            <div class="field" style="flex:1">
              <label>Effect</label>
              <div style="display:flex;gap:var(--space-1)">
                <button
                  class="icon-btn ${this._newOverrideAllow ? "allow" : ""}"
                  style="flex:1;height:var(--control-h)"
                  @click="${() => {
                    this._newOverrideAllow = true;
                  }}"
                >
                  <plume-icon name="check" size="14"></plume-icon> Allow
                </button>
                <button
                  class="icon-btn ${!this._newOverrideAllow ? "deny" : ""}"
                  style="flex:1;height:var(--control-h)"
                  @click="${() => {
                    this._newOverrideAllow = false;
                  }}"
                >
                  <plume-icon name="x" size="14"></plume-icon> Deny
                </button>
              </div>
            </div>
            <plume-button size="sm" @click="${this.#addOverride}">
              <plume-icon name="plus" size="14"></plume-icon> Add
            </plume-button>
          </div>
          ${this._overrideError
            ? html`
              <p class="error">${this._overrideError}</p>
            `
            : nothing}
        </div>
      </div>
    `;
  }

  #renderDanger() {
    const isCategory = this._conv?.type === "category";
    return html`
      <div class="section">
        <div class="section-title">Danger Zone</div>
        <plume-button
          variant="destructive"
          class="danger-btn"
          @click="${() => {
            this._deleteOpen = true;
          }}"
        >
          <plume-icon name="trash-2" size="14"></plume-icon>
          Delete ${isCategory ? "category" : "channel"}
        </plume-button>
      </div>
    `;
  }

  #renderDeleteDialog() {
    const isCategory = this._conv?.type === "category";
    return html`
      <plume-dialog
        style="--dialog-w:24rem"
        .open="${true}"
        heading="Delete ${isCategory ? "category" : "channel"}"
        @close="${() => {
          this._deleteOpen = false;
        }}"
      >
        <p style="font-size:var(--text-sm);margin:0">
          Are you sure you want to delete
          <strong>${this._conv?.name ?? "this"}</strong>? ${isCategory
            ? "All channels inside this category will also be deleted."
            : "This action cannot be undone."}
        </p>
        <div
          slot="footer"
          style="display:flex;justify-content:flex-end;gap:var(--space-2);width:100%"
        >
          <plume-button variant="ghost" type="button" @click="${() => {
            this._deleteOpen = false;
          }}">
            ${msg("Cancel")}
          </plume-button>
          <plume-button
            variant="destructive"
            ?disabled="${this._deleting}"
            @click="${this.#confirmDelete}"
          >
            ${this._deleting ? "Deleting…" : "Delete"}
          </plume-button>
        </div>
      </plume-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-channel-settings-panel": PlumeChannelSettingsPanel;
  }
}
