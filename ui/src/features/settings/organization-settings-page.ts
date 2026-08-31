import { css, html, LitElement, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import {
  deleteBackupRestorePending,
  deleteOrganization,
  getBackupDownload,
  getBackupRestorePending,
  getOrganization,
  patchOrganization,
  postBackupRestore,
} from "@/api";
import type { DtoOrganizationResponse } from "@/api";
import { auth, logout } from "@/store/auth";
import { showToast } from "@/components/ui/toast-store";
import { navigate } from "@/routes/router";
import { canManageOrg, isOrgElevatedRole } from "@/lib/permissions";
import { pageEnterStyles } from "@/styles/shared-animations";
import "../../components/ui/input.ts";
import "../../components/ui/button.ts";
import "../../components/ui/plume-icon.ts";
import "../../components/ui/spinner.ts";
import "../../layouts/app-layout.ts";
import { localized, msg, str } from "@lit/localize";

/**
 * Organization settings page. Any member can view the org; owners/admins can
 * rename it and set the message edit window; only owners see the danger zone
 * (delete org with type-to-confirm).
 */
@localized()
@customElement("plume-organization-settings-page")
export class PlumeOrganizationSettingsPage extends LitElement {
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
        flex-shrink: 0;
      }
      .page-head h1 {
        margin: 0;
        font-size: var(--text-lg);
        font-weight: 600;
        color: var(--foreground);
      }
      .page-head p {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }

      .page-content {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        padding: var(--space-6);
      }

      .sections {
        display: flex;
        flex-direction: column;
        gap: var(--space-6);
        max-width: var(--space-160);
      }

      .section {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
      }

      .section-header h2 {
        margin: 0;
        font-size: var(--text-base);
        font-weight: 600;
        color: var(--foreground);
      }
      .section-header p {
        margin: var(--space-1) 0 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }
      .audit-link {
        display: inline-flex;
        align-items: center;
        gap: var(--space-1);
        padding: var(--space-2) var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        cursor: pointer;
      }
      .audit-link:hover {
        background: var(--accent);
      }

      .field-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-4);
      }

      .field-label {
        display: flex;
        flex-direction: column;
        gap: var(--space-0-5);
      }
      .field-label .label {
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .field-label .description {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }

      .save-row {
        display: flex;
        align-items: center;
        gap: var(--space-3);
      }
      .danger-save-row {
        margin-top: var(--space-3);
      }

      .danger {
        border: 1px solid var(--destructive);
        border-radius: var(--radius-lg);
        padding: var(--space-4);
        background: color-mix(in oklch, var(--destructive) 8%, transparent);
      }
      .danger h3 {
        margin: 0 0 var(--space-1);
        font-size: var(--text-sm);
        font-weight: 600;
        color: var(--destructive);
      }
      .danger p {
        margin: 0 0 var(--space-3);
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }

      .pending-banner {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-3);
        padding: var(--space-3);
        border: 1px solid var(--accent);
        border-radius: var(--radius-lg);
        background: color-mix(in oklch, var(--accent) 8%, transparent);
        font-size: var(--text-sm);
        color: var(--foreground);
      }
      .restore-controls {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        flex-shrink: 0;
      }
      .restore-filename {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
        max-width: var(--space-48);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .restore-warning {
        margin: calc(-1 * var(--space-2)) 0 0;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }

      .loading {
        display: flex;
        align-items: center;
        justify-content: center;
        flex: 1;
        gap: var(--space-2);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
    `,
  ];

  /** When true, suppress <plume-app-layout> and .page-head. */
  @property({ type: Boolean })
  embedded = false;

  @state()
  private _org: DtoOrganizationResponse | null = null;
  @state()
  private _loading = true;
  @state()
  private _name = "";
  @state()
  private _editWindow = 0;
  @state()
  private _saving = false;
  @state()
  private _deleteConfirm = "";
  @state()
  private _deleting = false;
  @state()
  private _pendingRestore = false;
  @state()
  private _restoreFileName = "";
  @state()
  private _restoring = false;
  @state()
  private _downloading = false;

  connectedCallback(): void {
    super.connectedCallback();
    this.#load();
    this.#checkPendingRestore();
  }

  async #load(): Promise<void> {
    this._loading = true;
    try {
      const { data } = await getOrganization({ throwOnError: true });
      this._org = data;
      this._name = this._org?.name ?? "";
      this._editWindow = this._org?.message_edit_window_minutes ?? 0;
    } catch {
      showToast(msg("Failed to load organization"), { variant: "error" });
    } finally {
      this._loading = false;
    }
  }

  get #isOwner(): boolean {
    return auth.value.user?.role === "owner";
  }

  get #canViewAuditLog(): boolean {
    return isOrgElevatedRole(auth.value.user?.role);
  }

  get #canManageOrg(): boolean {
    return canManageOrg(auth.value.user?.role);
  }

  async #save(): Promise<void> {
    const name = this._name.trim();
    if (name.length < 2) {
      showToast(msg("Name must be at least 2 characters"), {
        variant: "error",
      });
      return;
    }
    this._saving = true;
    try {
      const { data } = await patchOrganization({
        body: {
          name,
          message_edit_window_minutes: this._editWindow,
        },
        throwOnError: true,
      });
      this._org = data;
      showToast(msg("Organization updated"));
    } catch {
      showToast(msg("Failed to update organization"), { variant: "error" });
    } finally {
      this._saving = false;
    }
  }

  async #checkPendingRestore(): Promise<void> {
    try {
      const { data } = await getBackupRestorePending({ throwOnError: true });
      this._pendingRestore = data.pending ?? false;
    } catch {
      // Non-critical: silently ignore.
    }
  }

  async #downloadBackup(): Promise<void> {
    this._downloading = true;
    try {
      // Use SDK with parseAs: 'blob' for binary download.
      // The response object provides Content-Disposition header for filename.
      const { data, response } = await getBackupDownload({
        parseAs: "blob",
        throwOnError: true,
      });
      const blob = data as Blob;
      const disposition = response.headers.get("Content-Disposition") ?? "";
      const match = disposition.match(/filename="(.+)"/);
      const filename = match ? match[1] : "plume-backup.db";
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);
      showToast(msg("Backup downloaded"));
    } catch {
      showToast(msg("Failed to download backup"), { variant: "error" });
    } finally {
      this._downloading = false;
    }
  }

  async #onRestoreFileSelect(e: Event): Promise<void> {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    this._restoreFileName = file.name;
    this._restoring = true;
    try {
      await postBackupRestore({
        body: { file },
        throwOnError: true,
      });
      showToast(msg("Backup staged. Restart the server to apply."));
    } catch (err) {
      const errMsg = err && typeof err === "object" && "message" in err
        ? (err as { message?: string }).message ??
          msg("Failed to stage restore")
        : msg("Failed to stage restore");
      showToast(errMsg, { variant: "error" });
    } finally {
      this._restoring = false;
    }
  }

  async #cancelPendingRestore(): Promise<void> {
    try {
      await deleteBackupRestorePending({ throwOnError: true });
      this._pendingRestore = false;
      showToast(msg("Pending restore cancelled"));
    } catch {
      showToast(msg("Failed to cancel restore"), { variant: "error" });
    }
  }

  async #deleteOrg(): Promise<void> {
    if (this._deleteConfirm !== (this._org?.name ?? "")) {
      showToast(msg("Type the organization name to confirm"), {
        variant: "error",
      });
      return;
    }
    this._deleting = true;
    try {
      await deleteOrganization({
        body: { confirm: this._deleteConfirm },
        throwOnError: true,
      });
      showToast(msg("Organization deleted"));
      await logout();
      navigate("/login");
    } catch {
      showToast(msg("Failed to delete organization"), { variant: "error" });
      this._deleting = false;
    }
  }

  protected render(): unknown {
    if (this._loading) {
      const spinner = html`
        <div
          class="loading"><plume-spinner></plume-spinner> ${msg(
            "Loading organization…",
          )}</div>
      `;
      if (this.embedded) return spinner;
      return html`<plume-app-layout>${spinner}</plume-app-layout>`;
    }

    const body = html`
      <div class="sections">
          <!-- General -->
          <div class="section">
            <div class="section-header">
              <h2>${msg("General")}</h2>
              <p>${msg("Organization name and message edit window")}</p>
            </div>

            <div class="field-row">
              <div class="field-label">
                <span class="label">${msg("Name")}</span>
                <span class="description">${msg(
                  "The display name of your workspace",
                )}</span>
              </div>
              <plume-input
                .value=${this._name}
                @input=${(
                  e: Event,
                ) => (this._name = (e.target as HTMLInputElement).value)}
              ></plume-input>
            </div>

            <div class="field-row">
              <div class="field-label">
                <span class="label">${msg(
                  "Message edit window (minutes)",
                )}</span>
                <span class="description">${msg(
                  "How long users can edit messages after sending. 0 = no limit.",
                )}</span>
              </div>
              <plume-input
                type="number"
                .value=${String(this._editWindow)}
                @input=${(e: Event) => (this._editWindow = parseInt(
                  (e.target as HTMLInputElement).value || "0",
                  10,
                ))}
              ></plume-input>
            </div>

            <div class="save-row">
              <plume-button
                ?disabled=${this._saving}
                @click=${() => this.#save()}
              >
                ${this._saving
                  ? html`<plume-spinner></plume-spinner><span>${
                    msg("Saving…")
                  }</span>`
                  : msg("Save changes")}
              </plume-button>
            </div>
          </div>

          <!-- Audit log link (owner/admin/member only) -->
          ${this.#canViewAuditLog
            ? html`
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Audit log")}</h2>
                  <p>${msg("Review admin actions taken in this workspace")}</p>
                </div>
                <button class="audit-link" type="button" @click="${() =>
                  navigate("/settings/audit-log")}">
                  ${msg("View audit log")}
                  <plume-icon name="chevron-right" size="14"></plume-icon>
                </button>
              </div>
            `
            : nothing}

          <!-- Database backup (owner/admin: PermOrgManage) -->
          ${this.#canManageOrg
            ? html`
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Database backup")}</h2>
                  <p>${msg("Download or restore a database backup")}</p>
                </div>

                <!-- Pending restore banner -->
                ${this._pendingRestore
                  ? html`
                    <div class="pending-banner">
                      <span>
                        ${msg(
                          "A restore is staged and will be applied on next server restart.",
                        )}
                      </span>
                      <plume-button
                        variant="outline"
                        size="sm"
                        @click=${this.#cancelPendingRestore}
                      >
                        ${msg("Cancel restore")}
                      </plume-button>
                    </div>
                  `
                  : nothing}

                <!-- Download -->
                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Download backup")}</span>
                    <span class="description">
                      ${msg(
                        "Download a snapshot of the current database. This may take a moment for large databases.",
                      )}
                    </span>
                  </div>
                  <div class="save-row">
                    <plume-button
                      ?disabled=${this._downloading}
                      @click=${this.#downloadBackup}
                    >
                      ${this._downloading
                        ? html`<plume-spinner></plume-spinner><span>${
                          msg("Downloading…")
                        }</span>`
                        : msg("Download backup")}
                    </plume-button>
                  </div>
                </div>

                <!-- Restore (owner only: PermOrgDelete) -->
                ${this.#isOwner
                  ? html`
                    <div class="field-row">
                      <div class="field-label">
                        <span class="label">${msg("Restore from backup")}</span>
                        <span class="description">
                          ${msg(
                            "Upload a .db backup file to restore the entire database.",
                          )}
                        </span>
                      </div>
                      <div class="restore-controls">
                        <input
                          type="file"
                          accept=".db,.sqlite,.sqlite3"
                          id="restore-file-input"
                          hidden
                          @change=${this.#onRestoreFileSelect}
                        />
                        <plume-button
                          variant="outline"
                          ?disabled=${this._restoring}
                          @click=${() =>
                            this.shadowRoot
                              ?.getElementById("restore-file-input")
                              ?.click()}
                        >
                          ${this._restoring
                            ? html`<plume-spinner></plume-spinner>`
                            : msg("Select backup file")}
                        </plume-button>
                        <span class="restore-filename">${this
                          ._restoreFileName}</span>
                      </div>
                    </div>
                    <p class="restore-warning">
                      ${msg(
                        "Restore replaces the entire database. The server must be restarted to apply. This action cannot be undone.",
                      )}
                    </p>
                  `
                  : nothing}
              </div>
            `
            : nothing}

          <!-- Danger Zone (owner only) -->
          ${this.#isOwner
            ? html`
              <div class="section">
                <div class="danger">
                  <h3>${msg("Delete organization")}</h3>
                  <p>
                    ${msg(
                      str`This permanently deletes "${
                        this._org?.name ?? ""
                      }". This action cannot be undone. Type the organization name to confirm.`,
                    )}
                  </p>
                  <plume-input
                    placeholder=${this._org?.name ?? ""}
                    .value=${this._deleteConfirm}
                    @input=${(e: Event) => (this._deleteConfirm = (
                      e.target as HTMLInputElement
                    ).value)}
                  ></plume-input>
                  <div class="save-row danger-save-row">
                    <plume-button
                      variant="destructive"
                      ?disabled=${this._deleting ||
                        this._deleteConfirm !== (this._org?.name ?? "")}
                      @click=${() => this.#deleteOrg()}
                    >
                      ${this._deleting
                        ? html`<plume-spinner></plume-spinner><span>${
                          msg("Deleting…")
                        }</span>`
                        : msg("Delete organization")}
                    </plume-button>
                  </div>
                </div>
              </div>
            `
            : null}
        </div>
    `;

    if (this.embedded) {
      return html`${body}`;
    }
    return html`
      <plume-app-layout>
        <div class="page-enter">
          <div class="page-head">
            <div>
              <h1>${msg("Organization")}</h1>
              <p>${msg("Manage your workspace settings")}</p>
            </div>
          </div>
          <div class="page-content">${body}</div>
        </div>
      </plume-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plume-organization-settings-page": PlumeOrganizationSettingsPage;
  }
}
