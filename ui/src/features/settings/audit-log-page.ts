import { css, html, LitElement } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { getAuditLog } from "@/api";
import type { DtoAuditEntryResponse } from "@/api";
import { pageEnterStyles } from "@/styles/shared-animations";
import { timeAgoShort } from "@/lib/format/time-ago";
import { membersApi } from "@/features/members/api";
import type { DtoUserResponse } from "@/api";
import "../../components/ui/spinner.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/select.ts";
import "../../layouts/app-layout.ts";
import { localized, msg, str } from "@lit/localize";

/** @see getACTION_META */
function getACTION_META(): Record<string, { label: string; icon: string }> {
  return {
    role_changed: { label: msg("Changed role"), icon: "shield" },
    user_activated: { label: msg("Activated user"), icon: "check-circle" },
    user_deactivated: { label: msg("Deactivated user"), icon: "circle" },
    member_removed: { label: msg("Removed member"), icon: "user-minus" },
    member_role_changed: { label: msg("Changed member role"), icon: "shield" },
    org_deleted: { label: msg("Deleted organization"), icon: "trash-2" },
    project_deleted: { label: msg("Deleted project"), icon: "trash-2" },
    invite_revoked: { label: msg("Revoked invite"), icon: "mail" },
    task_created: { label: msg("Created task"), icon: "plus" },
    task_deleted: { label: msg("Deleted task"), icon: "trash-2" },
  };
}

const PAGE_SIZE = 50;

/** @see getACTION_FILTER_OPTIONS */
function getACTION_FILTER_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "", label: msg("All actions") },
    { value: "role_changed", label: msg("Role changed") },
    { value: "user_activated", label: msg("User activated") },
    { value: "user_deactivated", label: msg("User deactivated") },
    { value: "member_removed", label: msg("Member removed") },
    { value: "member_role_changed", label: msg("Member role changed") },
    { value: "org_deleted", label: msg("Org deleted") },
    { value: "project_deleted", label: msg("Project deleted") },
    { value: "invite_revoked", label: msg("Invite revoked") },
    { value: "task_created", label: msg("Task created") },
    { value: "task_deleted", label: msg("Task deleted") },
  ];
}

@localized()
@customElement("plume-audit-log-page")
export class PlumeAuditLogPage extends LitElement {
  static styles = [
    pageEnterStyles,
    css`
      *,
      *::before,
      *::after {
        box-sizing: border-box;
      }
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      .page-head {
        display: flex;
        align-items: center;
        gap: var(--space-4);
        padding: var(--space-4) var(--space-6);
        border-bottom: 1px solid var(--border);
      }
      .page-head h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--foreground);
      }
      .page-head .subtitle {
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .page-content {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-6);
      }
      .filters {
        display: flex;
        gap: var(--space-3);
        flex-wrap: wrap;
        max-width: var(--container-xl);
        margin-bottom: var(--space-4);
      }
      .filter {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        width: var(--filter-w);
      }
      .entries {
        max-width: var(--container-xl);
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .entry {
        display: flex;
        align-items: flex-start;
        gap: var(--space-3);
        padding: var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--card);
      }
      .entry-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--space-8);
        height: var(--space-8);
        border-radius: var(--radius-md);
        background: var(--accent);
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .entry-body {
        flex: 1;
        min-width: 0;
      }
      .entry-title {
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .entry-meta {
        margin-top: 2px;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-1);
      }
      .entry-meta .sep {
        opacity: 0.5;
      }
      .entry-detail {
        margin-top: var(--space-1);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        font-family: var(--font-mono, monospace);
      }
      .loading,
      .empty {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-8);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      .pager {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: var(--space-4);
        gap: var(--space-2);
      }
      .pager button {
        height: var(--space-7);
        padding: 0 var(--space-3);
        border-radius: var(--radius-md);
        border: 1px solid var(--border);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-xs);
        cursor: pointer;
      }
      .pager button:disabled {
        opacity: 0.4;
        cursor: not-allowed;
      }
      .pager .count {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
    `,
  ];

  /** When true, suppress <plume-app-layout> and .page-head. */
  @property({ type: Boolean })
  embedded = false;

  @state()
  private _entries: DtoAuditEntryResponse[] = [];
  @state()
  private _total = 0;
  @state()
  private _offset = 0;
  @state()
  private _loading = true;
  @state()
  private _error: string | null = null;
  @state()
  private _actionFilter = "";
  @state()
  private _actorFilter = "";
  @state()
  private _members: DtoUserResponse[] = [];

  connectedCallback(): void {
    super.connectedCallback();
    this.#loadMembers();
    this.#load(0);
  }

  async #loadMembers(): Promise<void> {
    try {
      // Fetch all members via cursor pagination so the actor filter is
      // complete regardless of org size (the API caps per-page at 100).
      const all: DtoUserResponse[] = [];
      let cursor: string | undefined;
      do {
        const data = await membersApi.list({
          include_inactive: true,
          limit: 100,
          cursor,
        });
        all.push(...(data.items ?? []));
        cursor = data.has_more ? (data.next_cursor ?? undefined) : undefined;
      } while (cursor);
      this._members = all;
    } catch {
      // Non-critical: actor filter just falls back to the raw ID.
    }
  }

  async #load(offset: number): Promise<void> {
    this._loading = true;
    this._error = null;
    try {
      const { data } = await getAuditLog({
        query: {
          limit: PAGE_SIZE,
          offset,
          action: this._actionFilter || undefined,
          actor_id: this._actorFilter || undefined,
        },
        throwOnError: true,
      });
      this._entries = data.items ?? [];
      this._total = data.total ?? 0;
      this._offset = offset;
    } catch (err: unknown) {
      this._entries = [];
      this._total = 0;
      const obj = err as Record<string, unknown>;
      const errorBody = obj?.error as Record<string, unknown> | undefined;
      if (errorBody?.code === "forbidden") {
        this._error = "permission";
      } else {
        this._error = "load";
      }
    } finally {
      this._loading = false;
    }
  }

  #onActionFilter(e: CustomEvent): void {
    this._actionFilter = (e.detail as string) ?? "";
    this.#load(0);
  }

  #onActorFilter(e: CustomEvent): void {
    this._actorFilter = (e.detail as string) ?? "";
    this.#load(0);
  }

  #formatMetadata(raw?: string): string {
    if (!raw) return "";
    try {
      const obj = JSON.parse(raw) as Record<string, unknown>;
      return Object.entries(obj)
        .map(([k, v]) => `${k}: ${String(v)}`)
        .join(", ");
    } catch {
      return raw;
    }
  }

  protected render(): unknown {
    const auditBody = html`
      ${!this._loading && !this._error
        ? html`
          <div class="filters">
            <label class="filter">
              <span>${msg("Action")}</span>
              <plume-select
                searchable
                .options=${getACTION_FILTER_OPTIONS()}
                .value=${this._actionFilter}
                placeholder=${msg("All actions")}
                @change=${this.#onActionFilter}
              ></plume-select>
            </label>
            <label class="filter">
              <span>${msg("Actor")}</span>
              <plume-select
                searchable
                .options=${[
                  { value: "", label: msg("All actors") },
                  ...this._members.map((m) => ({
                    value: m.id ?? "",
                    label: m.name ?? m.email ?? msg("Unknown"),
                  })),
                ]}
                .value=${this._actorFilter}
                placeholder=${msg("All actors")}
                @change=${this.#onActorFilter}
              ></plume-select>
            </label>
          </div>
        `
        : null}
        ${this._loading
          ? html`<div class="loading">
              <plume-spinner></plume-spinner> ${msg("Loading…")}
            </div>`
          : this._error === "permission"
          ? html`<div class="empty">${
            msg("You don't have permission to view the audit log.")
          }</div>`
          : this._error === "load"
          ? html`<div class="empty">${msg("Failed to load audit log.")}</div>`
          : this._entries.length === 0
          ? html`<div class="empty">${msg("No audit entries yet.")}</div>`
          : html`<div class="entries">
              ${
            this._entries.map((e) => {
              const meta = getACTION_META()[e.action ?? ""] ??
                { label: e.action ?? msg("unknown"), icon: "info" };
              return html`
                <div class="entry">
                  <div class="entry-icon">
                    <plume-icon name="${meta
                      .icon}" size="16"></plume-icon>
                  </div>
                  <div class="entry-body">
                    <div class="entry-title">${meta.label}</div>
                    <div class="entry-meta">
                      <span>${e.actor_name ?? e.actor_email ??
                        e.actor_id}</span>
                      <span class="sep">·</span>
                      <span>${e.entity_type}${e.entity_id
                        ? ` ${e.entity_id.slice(0, 8)}`
                        : ""}</span>
                      <span class="sep">·</span>
                      <span>${timeAgoShort(e.created_at ?? "", 30)}</span>
                    </div>
                    ${e.metadata
                      ? html`<div class="entry-detail">
                            ${this.#formatMetadata(e.metadata)}
                          </div>`
                      : null}
                  </div>
                </div>
              `;
            })
          }
            </div>`}
        ${!this._loading && this._total > PAGE_SIZE
          ? html`
            <div class="pager">
              <button
                ?disabled=${this._offset === 0}
                @click=${() =>
                  this.#load(Math.max(0, this._offset - PAGE_SIZE))}
              >${msg("Previous")}</button>
              <span class="count">
                                    ${msg(
                                      str`${this._offset + 1}–${
                                        Math.min(
                                          this._offset + this._entries.length,
                                          this._total,
                                        )
                                      } of ${this._total}`,
                                    )}
                              </span>
              <button
                ?disabled=${this._offset + PAGE_SIZE >= this._total}
                @click=${() => this.#load(this._offset + PAGE_SIZE)}
              >${msg("Next")}</button>
            </div>
          `
          : null}
    `;

    if (this.embedded) {
      return html`${auditBody}`;
    }
    return html`
      <plume-app-layout>
        <div class="page-enter">
          <div class="page-head">
            <h1>${msg("Audit log")}</h1>
            <span class="subtitle">${msg(
              "Admin actions in this workspace",
            )}</span>
          </div>
          <div class="page-content">${auditBody}</div>
        </div>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-audit-log-page": PlumeAuditLogPage;
  }
}
