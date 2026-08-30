import { localized, msg } from "@lit/localize";
import { css, html, LitElement, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { SignalController } from "@/lib/signal-controller";
import { inviteDialogOpen } from "../store";
import { membersApi } from "../api";
import { getProjects } from "@/api";
import type { DtoProjectAssignment } from "@/api";
import type { InviteCreated } from "../api";
import "../components/role-badge.ts";
import "@/components/ui/select.ts";
import "@/components/ui/input.ts";
import "@/components/ui/button.ts";
import "@/components/ui/dialog.ts";

interface ProjectEntry {
  id: string;
  name: string;
  color: string;
  assigned: boolean;
  role: string;
}

/**
 * Invite creation dialog.
 *
 * Two-phase:
 *  1. Form: select role + optional email restriction + optional project
 *     assignments → "Generate link"
 *  2. Result: show invite URL with copy button + expiry info → "Done"
 *
 * When projects are assigned at invite time, the user is automatically added
 * to those projects upon accepting the invite.
 */
@localized()
@customElement("breeze-invite-dialog")
export class BreezeInviteDialog extends LitElement {
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
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: var(--space-1-5);
    }
    .field-label {
      font-size: var(--text-sm);
      font-weight: 500;
      color: var(--foreground);
    }
    .field-hint {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
    }
    .field-error {
      font-size: var(--text-xs);
      color: var(--destructive);
    }

    /* Collapsible project section */
    .section-toggle {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      padding: 0;
      border: none;
      background: none;
      color: var(--muted-foreground);
      font-size: var(--text-sm);
      font-family: inherit;
      cursor: pointer;
      transition: color var(--dur-fast) var(--ease-1);
    }
    .section-toggle:hover {
      color: var(--foreground);
    }
    .section-toggle breeze-icon {
      transition: transform var(--dur-fast) var(--ease-1);
    }
    .section-toggle.open breeze-icon {
      transform: rotate(90deg);
    }
    .project-section {
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      overflow: hidden;
    }
    .project-section-header {
      display: grid;
      grid-template-columns: 1fr 8rem;
      gap: var(--space-3);
      padding: var(--space-2) var(--space-3);
      font-size: var(--text-xs);
      font-weight: 500;
      color: var(--muted-foreground);
      background: color-mix(in oklch, var(--muted) 30%, transparent);
      border-bottom: 1px solid var(--border);
    }
    .project-list {
      max-height: var(--space-56);
      overflow-y: auto;
    }
    .project-row {
      display: grid;
      grid-template-columns: auto 1fr 8rem;
      gap: var(--space-3);
      align-items: center;
      padding: var(--space-2) var(--space-3);
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
      width: var(--space-2);
      height: var(--space-2);
      border-radius: var(--radius-full);
      flex-shrink: 0;
    }
    .project-name {
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
    .role-select {
      width: 100%;
    }
    .loading-projects {
      padding: var(--space-4);
      text-align: center;
      font-size: var(--text-sm);
      color: var(--muted-foreground);
    }
    .project-hint {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      padding: var(--space-2) var(--space-3);
    }

    /* Result */
    .result-box {
      border: 1px solid var(--border);
      border-radius: var(--radius-md);
      background: color-mix(in oklch, var(--muted) 40%, transparent);
      padding: var(--space-4);
    }
    .result-label {
      font-size: var(--text-sm);
      font-weight: 500;
      margin-bottom: var(--space-2);
    }
    .result-link-row {
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .result-link {
      flex: 1;
      background: var(--background);
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      padding: var(--space-1) var(--space-2);
      font-size: var(--text-xs);
      font-family: monospace;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .result-expiry {
      font-size: var(--text-xs);
      color: var(--muted-foreground);
      margin-top: var(--space-2);
    }
    .result-projects {
      margin-top: var(--space-3);
      padding-top: var(--space-3);
      border-top: 1px solid var(--border);
      font-size: var(--text-xs);
      color: var(--muted-foreground);
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
  private _error = "";

  @state()
  private _role: "admin" | "member" | "viewer" | "guest" = "member";

  @state()
  private _email = "";

  @state()
  private _busy = false;

  @state()
  private _generated: InviteCreated | null = null;

  @state()
  private _copied = false;

  // Project selection state
  @state()
  private _projectEntries: ProjectEntry[] = [];

  @state()
  private _projectsLoading = false;

  @state()
  private _projectSectionOpen = false;

  private _timer = 0;

  connectedCallback(): void {
    super.connectedCallback();
    this.#signals.watch(inviteDialogOpen);
  }

  #reset() {
    this._error = "";
    this._role = "member";
    this._email = "";
    this._busy = false;
    this._generated = null;
    this._copied = false;
    this._projectEntries = [];
    this._projectSectionOpen = false;
    if (this._timer) {
      clearTimeout(this._timer);
      this._timer = 0;
    }
  }

  private async _loadProjects() {
    if (this._projectEntries.length > 0) return;
    this._projectsLoading = true;
    try {
      const { data } = await getProjects({ throwOnError: true });
      const projects = data;
      this._projectEntries = (projects ?? []).map((p) => ({
        id: p.id ?? "",
        name: p.name ?? "",
        color: p.color ?? "",
        assigned: false,
        role: "member",
      }));
    } catch {
      this._projectEntries = [];
    }
    this._projectsLoading = false;
  }

  private _onClose() {
    inviteDialogOpen.value = false;
    this.#reset();
  }

  private async _onGenerate(e: Event) {
    e.preventDefault();
    this._error = "";

    // Validate email if provided
    if (this._email.trim() && !this._email.includes("@")) {
      this._error = "Please enter a valid email address.";
      return;
    }

    this._busy = true;
    try {
      const body: {
        role: "admin" | "member" | "viewer" | "guest";
        email?: string;
        project_assignments?: DtoProjectAssignment[];
      } = {
        role: this._role,
      };

      if (this._email.trim()) {
        body.email = this._email.trim();
      }

      // Collect project assignments
      const assignments: DtoProjectAssignment[] = [];
      for (const entry of this._projectEntries) {
        if (entry.assigned) {
          assignments.push({
            project_id: entry.id,
            role: entry.role as "admin" | "member" | "viewer" | "guest",
          });
        }
      }
      if (assignments.length > 0) {
        body.project_assignments = assignments;
      }

      const result = await membersApi.createInvite(body);
      this._generated = result;
    } catch (err: unknown) {
      const errMsg = err instanceof Error
        ? err.message
        : msg("Failed to create invite.");
      this._error = errMsg;
    } finally {
      this._busy = false;
    }
  }

  private async _copyLink() {
    if (!this._generated) return;
    const fullUrl = `${window.location.origin}${this._generated.url}`;
    try {
      await navigator.clipboard.writeText(fullUrl);
      this._copied = true;
      this._timer = window.setTimeout(() => {
        this._copied = false;
        this._timer = 0;
      }, 2000);
    } catch {
      // Silently ignore clipboard failures
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._timer) clearTimeout(this._timer);
  }

  #toggleProject(id: string) {
    const entry = this._projectEntries.find((e) => e.id === id);
    if (!entry) return;
    entry.assigned = !entry.assigned;
    this.requestUpdate();
  }

  #setProjectRole(id: string, role: string) {
    const entry = this._projectEntries.find((e) => e.id === id);
    if (!entry) return;
    entry.role = role;
    this.requestUpdate();
  }

  get #assignedProjectCount(): number {
    return this._projectEntries.filter((e) => e.assigned).length;
  }

  private get _roleOptions() {
    return [
      { value: "admin", label: msg("Admin") },
      { value: "member", label: msg("Member") },
      { value: "viewer", label: msg("Viewer") },
      { value: "guest", label: msg("Guest") },
    ];
  }

  protected render() {
    const showForm = !this._generated;

    return html`
      <breeze-dialog
        .open="${inviteDialogOpen.value}"
        heading="${msg("Invite people")}"
        style="--dialog-w:32rem"
        @close="${this._onClose}"
      >
        ${showForm
          ? html`
            <form @submit="${this._onGenerate}" class="body">
              <div class="field">
                <label class="field-label">${msg("Role")}</label>
                <breeze-select
                  .options="${this._roleOptions}"
                  .value="${this._role}"
                  @change="${(e: CustomEvent) => {
                    this._role = e.detail as typeof this._role;
                  }}"
                ></breeze-select>
              </div>

              <div class="field">
                <label class="field-label" for="invite-email">
                  ${msg("Email restriction")}
                </label>
                <span class="field-hint">
                  ${msg("Optional — leave blank to allow any email")}
                </span>
                <breeze-input
                  id="invite-email"
                  type="email"
                  placeholder="${msg("Leave blank for any email")}"
                  .value="${this._email}"
                  @input="${(e: Event) => {
                    this._email = (e.target as HTMLInputElement).value;
                  }}"
                ></breeze-input>
              </div>

              <!-- Optional project assignments -->
              <button
                type="button"
                class="section-toggle ${this._projectSectionOpen ? "open" : ""}"
                @click="${() => {
                  this._projectSectionOpen = !this._projectSectionOpen;
                  if (this._projectSectionOpen) this._loadProjects();
                }}"
              >
                <breeze-icon name="chevron-right" size="14"></breeze-icon>
                ${msg("Assign to projects")} ${this.#assignedProjectCount > 0
                  ? html`
                    (${this.#assignedProjectCount})
                  `
                  : nothing}
              </button>

              ${this._projectSectionOpen
                ? html`
                  <div class="project-section">
                    <div class="project-section-header">
                      <span>${msg("Project")}</span>
                      <span>${msg("Role")}</span>
                    </div>
                    ${this._projectsLoading
                      ? html`
                        <div class="loading-projects">Loading projects...</div>
                      `
                      : this._projectEntries.length === 0
                      ? html`
                        <div class="project-hint">
                          No projects found. Create a project first.
                        </div>
                      `
                      : html`
                        <div class="project-list">
                          ${this._projectEntries.map((entry) =>
                            html`
                              <div class="project-row">
                                <span
                                  class="check-box ${entry.assigned
                                    ? "checked"
                                    : ""}"
                                  @click="${() =>
                                    this.#toggleProject(entry.id)}"
                                  role="checkbox"
                                  aria-checked="${entry.assigned}"
                                  tabindex="0"
                                  @keydown="${(e: KeyboardEvent) => {
                                    if (
                                      e.key === "Enter" || e.key === " "
                                    ) {
                                      e.preventDefault();
                                      this.#toggleProject(entry.id);
                                    }
                                  }}"
                                >
                                  ${entry.assigned
                                    ? html`
                                      <breeze-icon
                                        name="check"
                                        size="10"
                                      ></breeze-icon>
                                    `
                                    : nothing}
                                </span>
                                <div class="project-info">
                                  <span
                                    class="project-color"
                                    style="background:${entry.color ||
                                      "var(--muted-foreground)"}"
                                  ></span>
                                  <span class="project-name">${entry
                                    .name}</span>
                                </div>
                                ${entry.assigned
                                  ? html`
                                    <breeze-select
                                      class="role-select"
                                      .options="${this._roleOptions}"
                                      .value="${entry.role}"
                                      @change="${(e: CustomEvent) => {
                                        this.#setProjectRole(
                                          entry.id,
                                          e.detail as string,
                                        );
                                      }}"
                                    ></breeze-select>
                                  `
                                  : html`
                                    <span
                                      style="font-size:var(--text-xs);color:var(--muted-foreground)"
                                    >
                                      —
                                    </span>
                                  `}
                              </div>
                            `
                          )}
                        </div>
                      `}
                  </div>
                `
                : nothing} ${this._error
                ? html`
                  <div class="field-error">${this._error}</div>
                `
                : nothing}
            </form>

            <div slot="footer" class="footer">
              <breeze-button
                variant="ghost"
                type="button"
                @click="${this._onClose}"
              >
                ${msg("Cancel")}
              </breeze-button>
              <breeze-button
                ?disabled="${this._busy}"
                type="submit"
                @click="${this._onGenerate}"
              >
                ${this._busy ? msg("Generating...") : msg("Generate link")}
              </breeze-button>
            </div>
          `
          : html`
            <div class="result-box">
              <div class="result-label">${msg("Invite link")}</div>
              <div class="result-link-row">
                <code class="result-link">
                  ${window.location.origin}${this._generated!.url}
                </code>
                <breeze-button
                  variant="outline"
                  size="sm"
                  type="button"
                  @click="${this._copyLink}"
                >
                  ${this._copied ? msg("Copied") : msg("Copy")}
                </breeze-button>
              </div>
              <div class="result-expiry">
                ${msg("Expires")} ${new Date(this._generated!.expires_at)
                  .toLocaleDateString()}
              </div>
              ${this.#assignedProjectCount > 0
                ? html`
                  <div class="result-projects">
                    ${msg("Will be added to")} ${this
                      .#assignedProjectCount} ${this.#assignedProjectCount === 1
                      ? msg("project")
                      : msg("projects")} ${msg("upon acceptance.")}
                  </div>
                `
                : nothing}
            </div>

            <div slot="footer" class="footer">
              <breeze-button
                variant="ghost"
                type="button"
                @click="${this._onClose}"
              >
                ${msg("Done")}
              </breeze-button>
            </div>
          `}
      </breeze-dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-invite-dialog": BreezeInviteDialog;
  }
}
