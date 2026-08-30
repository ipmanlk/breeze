import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";

import {
  getProjects,
  getUsersByIdProjectMemberships,
  putUsersByIdProjectMemberships,
} from "@/api";
import type { DtoProjectAssignment } from "@/api";
import "@/components/ui/button.ts";
import "@/components/ui/breeze-icon.ts";
import "@/components/ui/dialog.ts";
import "@/components/ui/input.ts";
import "@/components/ui/select.ts";
import "@/components/ui/spinner.ts";
import "@/components/ui/avatar.ts";

function getPROJECT_ROLE_OPTIONS(): { value: string; label: string }[] {
  return [
    { value: "admin", label: msg("Admin") },
    { value: "member", label: msg("Member") },
    { value: "viewer", label: msg("Viewer") },
    { value: "guest", label: msg("Guest") },
  ];
}

interface ProjectEntry {
  id: string;
  name: string;
  color: string;
  icon: string;
  assigned: boolean;
  role: string;
}

/**
 * Dialog for managing a member's project assignments.
 *
 * Shows all org projects with toggle checkboxes and role selectors.
 * Follows the same table + inline edit pattern as project-members-view.
 */
@localized()
@customElement("breeze-member-projects-dialog")
export class BreezeMemberProjectsDialog extends LitElement {
  static styles = css`
    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }
    .body {
      display: flex;
      flex-direction: column;
      gap: var(--space-4);
      min-height: var(--space-48);
    }
    .search-wrap {
      width: 100%;
    }
    .project-list {
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      max-height: var(--space-96);
      overflow-y: auto;
    }
    .project-row {
      display: grid;
      grid-template-columns: auto 1fr 8rem;
      gap: var(--space-3);
      align-items: center;
      padding: var(--space-2-5) var(--space-3);
      font-size: var(--text-sm);
      border-bottom: 1px solid var(--border);
      transition: background var(--dur-fast) var(--ease-1);
    }
    .project-row:last-child {
      border-bottom: none;
    }
    .project-row:hover {
      background: var(--accent);
    }
    .project-info {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      min-width: 0;
    }
    .project-color {
      width: var(--space-2-5);
      height: var(--space-2-5);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .project-name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 500;
    }
    .check-box {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 1.125rem;
      height: 1.125rem;
      border-radius: var(--radius-sm);
      border: 1px solid var(--border);
      flex-shrink: 0;
      cursor: pointer;
      transition:
        background var(--dur-fast) var(--ease-1),
        border-color var(--dur-fast) var(--ease-1);
    }
    .check-box.checked {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
    }
    .check-box.disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
    .role-select {
      width: 100%;
    }
    .empty {
      text-align: center;
      padding: var(--space-8);
      color: var(--muted-foreground);
      font-size: var(--text-sm);
    }
    .loading {
      display: flex;
      align-items: center;
      justify-content: center;
      padding: var(--space-8);
    }
    .footer {
      display: flex;
      justify-content: flex-end;
      gap: var(--space-2);
      width: 100%;
    }
    .error {
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--destructive);
    }
    .selection-info {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
  `;

  @property({ type: Boolean })
  open = false;

  @property({ type: String })
  userId = "";

  @property({ type: String })
  userName = "";

  @state()
  private _entries: ProjectEntry[] = [];

  @state()
  private _search = "";

  @state()
  private _loading = true;

  @state()
  private _saving = false;

  @state()
  private _error = "";

  protected willUpdate(changed: Map<string, unknown>): void {
    if (changed.has("open") && this.open) {
      void this.#load();
    }
  }

  async #load() {
    this._loading = true;
    this._error = "";
    try {
      const [allProjects, memberships] = await Promise.all([
        getProjects({ throwOnError: true }).then(
          (r) => r.data,
        ),
        getUsersByIdProjectMemberships({
          path: { id: this.userId },
          throwOnError: true,
        }).then(
          (r) => r.data,
        ),
      ]);

      const membershipMap = new Map<string, string>();
      for (const m of memberships) {
        membershipMap.set(m.project_id!, m.role!);
      }

      this._entries = (allProjects ?? []).map((p) => ({
        id: p.id ?? "",
        name: p.name ?? "",
        color: p.color ?? "",
        icon: p.icon ?? "",
        assigned: membershipMap.has(p.id ?? ""),
        role: membershipMap.get(p.id ?? "") ?? "member",
      }));
    } catch {
      this._error = msg("Failed to load projects.");
      this._entries = [];
    }
    this._loading = false;
  }

  #toggleProject(id: string) {
    const entry = this._entries.find((e) => e.id === id);
    if (!entry) return;
    entry.assigned = !entry.assigned;
    this.requestUpdate();
  }

  #setRole(id: string, role: string) {
    const entry = this._entries.find((e) => e.id === id);
    if (!entry) return;
    entry.role = role;
    this.requestUpdate();
  }

  get #filteredEntries(): ProjectEntry[] {
    if (!this._search) return this._entries;
    const q = this._search.toLowerCase();
    return this._entries.filter((e) => e.name.toLowerCase().includes(q));
  }

  get #assignedCount(): number {
    return this._entries.filter((e) => e.assigned).length;
  }

  async #save() {
    this._saving = true;
    this._error = "";

    const assignments: DtoProjectAssignment[] = [];
    for (const e of this._entries) {
      if (e.assigned) {
        assignments.push({
          project_id: e.id,
          role: e.role as "admin" | "member" | "viewer" | "guest",
        });
      }
    }

    try {
      await putUsersByIdProjectMemberships({
        path: { id: this.userId },
        body: { assignments },
        throwOnError: true,
      });
      this.dispatchEvent(
        new CustomEvent("close", { bubbles: true, composed: true }),
      );
    } catch {
      this._error = msg("Failed to save project assignments.");
    }
    this._saving = false;
  }

  #onClose() {
    this.dispatchEvent(
      new CustomEvent("close", { bubbles: true, composed: true }),
    );
  }

  protected render() {
    return html`
      <breeze-dialog
        .open="${this.open}"
        heading="${msg("Project assignments")} — ${this.userName}"
        style="--dialog-w:32rem"
        @close="${this.#onClose}"
      >
        <div class="body">
          ${this._loading
            ? html`
              <div class="loading">
                <breeze-spinner size="20"></breeze-spinner>
              </div>
            `
            : this._error && this._entries.length === 0
            ? html`
              <div class="empty">${this._error}</div>
            `
            : html`
              <breeze-input
                class="search-wrap"
                type="search"
                placeholder="${msg("Search projects")}"
                .value="${this._search}"
                @input="${(e: Event) => {
                  this._search = (e.target as HTMLInputElement).value;
                }}"
              ></breeze-input>

              <div class="project-list">
                ${this.#filteredEntries.length === 0
                  ? html`
                    <div class="empty">${msg(
                      "No projects match your search.",
                    )}</div>
                  `
                  : this.#filteredEntries.map((entry) =>
                    html`
                      <div class="project-row">
                        <span
                          class="check-box ${entry.assigned ? "checked" : ""}"
                          @click="${() => this.#toggleProject(entry.id)}"
                          role="checkbox"
                          aria-checked="${entry.assigned}"
                          tabindex="0"
                          @keydown="${(e: KeyboardEvent) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              this.#toggleProject(entry.id);
                            }
                          }}"
                        >
                          ${entry.assigned
                            ? html`
                              <breeze-icon name="check" size="10"></breeze-icon>
                            `
                            : nothing}
                        </span>
                        <div class="project-info">
                          <span
                            class="project-color"
                            style="background:${entry.color ||
                              "var(--muted-foreground)"}"
                          ></span>
                          <span class="project-name">${entry.name}</span>
                        </div>
                        ${entry.assigned
                          ? html`
                            <breeze-select
                              class="role-select"
                              .options="${getPROJECT_ROLE_OPTIONS()}"
                              .value="${entry.role}"
                              @change="${(e: CustomEvent) => {
                                this.#setRole(entry.id, e.detail as string);
                              }}"
                            ></breeze-select>
                          `
                          : html`
                            <span style="font-size:var(--text-xs);color:var(--muted-foreground)">
                              Not assigned
                            </span>
                          `}
                      </div>
                    `
                  )}
              </div>

              <span class="selection-info">
                ${this.#assignedCount} of ${this._entries.length} projects
              </span>
              ${this._error
                ? html`
                  <p class="error">${this._error}</p>
                `
                : nothing}
            `}
        </div>

        <div slot="footer" class="footer">
          <breeze-button
            variant="ghost"
            type="button"
            @click="${this.#onClose}"
          >
            ${msg("Cancel")}
          </breeze-button>
          <breeze-button
            type="button"
            ?disabled="${this._loading || this._saving}"
            @click="${this.#save}"
          >
            ${this._saving ? msg("Saving...") : msg("Save")}
          </breeze-button>
        </div>
      </breeze-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-member-projects-dialog": BreezeMemberProjectsDialog;
  }
}
