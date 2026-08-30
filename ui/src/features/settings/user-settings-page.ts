import { css, html, LitElement } from "lit";
import { customElement, state } from "lit/decorators.js";
import {
  deleteAuthSessionsById,
  getAuthSessions,
  getSettingsNotifications,
  getSettingsPreferences,
  patchAccount,
  patchSettingsNotificationsByType,
  patchSettingsPreferences,
  postAccountAvatar,
  postAccountChangePassword,
} from "@/api";
import type {
  DtoNotificationPreferenceResponse,
  DtoSessionResponse,
  DtoUserPreferencesResponse,
} from "@/api";
import { getNotificationLabel } from "../notifications/notification-item";
import { applyPreset, currentPreset, THEME_PRESETS } from "@/store/theme";
import { auth, fetchMe, logout } from "@/store/auth";
import { setPreferences } from "@/store/preferences";
import { showToast } from "@/components/ui/toast-store";
import { pageEnterStyles } from "@/styles/shared-animations";
import { navigate } from "@/routes/router";
import "../../components/ui/switch.ts";
import "../../components/ui/spinner.ts";
import "../../components/ui/breeze-icon.ts";
import "../../components/ui/field.ts";
import "../../components/ui/input.ts";
import "../../components/ui/select.ts";
import "../../components/ui/button.ts";
import "../../components/ui/avatar.ts";
import "../../components/motion-settings.ts";
import "../../layouts/app-layout.ts";

const TIMEZONES = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Anchorage",
  "Pacific/Honolulu",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Moscow",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Seoul",
  "Australia/Sydney",
  "Pacific/Auckland",
];

function getLanguages() {
  return [
    { value: "en", label: msg("English") },
    { value: "fr", label: msg("Français") },
  ];
}

/**
 * Turn a raw User-Agent string into a short "Browser on OS" label for the
 * sessions list. Falls back to "Unknown device" when empty or unparseable.
 */
function summarizeUserAgent(ua?: string): string {
  if (!ua) return msg("Unknown device");
  const browser = (() => {
    if (/edg/i.test(ua)) return "Edge";
    if (/opr\/|opera/i.test(ua)) return "Opera";
    if (/chrome|chromium|crios/i.test(ua)) return "Chrome";
    if (/firefox|fxios/i.test(ua)) return "Firefox";
    if (/safari/i.test(ua)) return "Safari";
    return "Browser";
  })();
  const os = (() => {
    if (/windows/i.test(ua)) return "Windows";
    if (/mac os x|macintosh/i.test(ua)) return "macOS";
    if (/android/i.test(ua)) return "Android";
    if (/iphone|ipad|ipod/i.test(ua)) return "iOS";
    if (/linux/i.test(ua)) return "Linux";
    return "";
  })();
  return os ? `${browser} on ${os}` : browser;
}

/** Pick a lucide icon name matching the detected device class. */
function sessionDeviceIcon(ua?: string): string {
  if (!ua) return "monitor";
  if (/android|iphone|ipad|ipod|mobile/i.test(ua)) return "smartphone";
  return "monitor";
}

import { timeAgoShort } from "@/lib/format/time-ago";
import { localized, msg } from "@lit/localize";

@localized()
@customElement("breeze-user-settings-page")
export class BreezeUserSettingsPage extends LitElement {
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
        font-family: var(--font-heading, inherit);
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

      .section-header {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
      }
      .section-header h2 {
        margin: 0;
        font-size: var(--text-base);
        font-weight: 600;
        color: var(--foreground);
      }
      .section-header p {
        margin: 0;
        font-size: var(--text-sm);
        color: var(--muted-foreground);
      }

      .field-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-4);
        padding: var(--space-3) 0;
        border-bottom: 1px solid var(--border);
      }
      .field-row:last-child {
        border-bottom: none;
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
        display: inline-flex;
        align-items: center;
        gap: var(--space-2);
      }
      .field-label .description {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .form-error {
        font-size: var(--text-xs);
        font-weight: 500;
        color: var(--destructive);
      }

      .sessions-loading,
      .sessions-empty {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding: var(--space-4);
        color: var(--muted-foreground);
        font-size: var(--text-sm);
      }
      .session-list {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
      }
      .session-row {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--card);
      }
      .session-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: var(--control-h);
        height: var(--control-h);
        border-radius: var(--radius-md);
        background: var(--accent);
        color: var(--muted-foreground);
        flex-shrink: 0;
      }
      .session-info {
        flex: 1;
        min-width: 0;
      }
      .session-title {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        font-size: var(--text-sm);
        font-weight: 500;
        color: var(--foreground);
      }
      .session-meta {
        display: flex;
        align-items: center;
        gap: var(--space-1);
        margin-top: 2px;
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .session-current-badge {
        display: inline-block;
        padding: 0 var(--space-1-5);
        border-radius: var(--radius-full);
        background: color-mix(in oklch, var(--primary) 16%, transparent);
        color: var(--primary);
        font-size: var(--text-2xs, 0.6875rem);
        font-weight: 500;
        line-height: 1.4;
      }
      .session-revoked-badge {
        display: inline-block;
        padding: 0 var(--space-1-5);
        border-radius: var(--radius-full);
        background: color-mix(in oklch, var(--muted-foreground) 16%, transparent);
        color: var(--muted-foreground);
        font-size: var(--text-2xs, 0.6875rem);
        font-weight: 500;
        line-height: 1.4;
      }

      .field-control {
        flex-shrink: 0;
      }
      .field-control-row {
        display: flex;
        align-items: center;
        gap: var(--space-2);
      }
      .field-control-row-lg {
        display: flex;
        align-items: center;
        gap: var(--space-3);
      }
      .security-form {
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
        max-width: var(--space-96);
      }

      .save-status {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        padding-top: var(--space-4);
        border-top: 1px solid var(--border);
        font-size: var(--text-sm);
        color: var(--muted-foreground);
        min-height: var(--space-6);
      }
      .save-status .dot {
        width: var(--space-2);
        height: var(--space-2);
        border-radius: 50%;
        background: var(--muted-foreground);
        flex-shrink: 0;
      }
      .save-status.saving .dot {
        background: var(--ring);
        animation: pulse-dot 1s var(--ease-1) infinite;
      }
      .save-status.saved .dot {
        background: oklch(0.65 0.18 160);
      }
      .save-status.error {
        color: var(--destructive);
      }
      .save-status.error .dot {
        background: var(--destructive);
      }
      @keyframes pulse-dot {
        0%,
        100% {
          opacity: 1;
        }
        50% {
          opacity: 0.3;
        }
      }

      .loading {
        display: flex;
        align-items: center;
        justify-content: center;
        flex: 1;
        color: var(--muted-foreground);
        font-size: var(--text-sm);
        gap: var(--space-2);
      }

      /* Theme preset grid */
      .theme-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
        gap: var(--space-2);
      }
      .theme-card {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        gap: var(--space-1-5);
        padding: var(--space-3);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        background: var(--card);
        cursor: pointer;
        transition:
          border-color var(--dur-fast) var(--ease-1),
          box-shadow var(--dur-fast) var(--ease-1);
      }
      .theme-card:hover {
        border-color: var(--ring);
      }
      .theme-card.selected {
        border-color: var(--primary);
        box-shadow: 0 0 0 1px var(--primary);
      }
      .theme-card-preview {
        width: 100%;
        height: var(--space-10);
        border-radius: var(--radius-md);
        border: 1px solid var(--border);
        display: flex;
        align-items: center;
        gap: var(--space-1);
        padding: 0 var(--space-2);
        overflow: hidden;
      }
      .theme-card-preview .bar {
        width: var(--space-2);
        height: var(--space-5);
        border-radius: var(--radius-sm);
        flex-shrink: 0;
      }
      .theme-card-name {
        font-size: var(--text-sm);
        font-weight: 500;
      }
      .theme-card-desc {
        font-size: var(--text-xs);
        color: var(--muted-foreground);
      }
      .theme-section-divider {
        border: none;
        border-top: 1px solid var(--border);
        margin: 0;
      }
      .avatar-upload {
        display: inline-flex;
        align-items: center;
        gap: var(--space-2);
        padding: 0 var(--space-3);
        height: var(--control-h-sm);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        background: var(--background);
        color: var(--foreground);
        font-size: var(--text-sm);
        cursor: pointer;
      }
      .avatar-upload:hover {
        border-color: var(--ring);
      }
      .avatar-upload input[type="file"] {
        position: absolute;
        width: 1px;
        height: 1px;
        opacity: 0;
        overflow: hidden;
      }
    `,
  ];

  @state()
  private _prefs: DtoUserPreferencesResponse | null = null;
  @state()
  private _loading = true;
  @state()
  private _saveState: "idle" | "saving" | "saved" | "error" = "idle";

  // Account section state
  @state()
  private _profileName = "";
  @state()
  private _profileSaving = false;
  @state()
  private _avatarUploading = false;

  // Security section state
  @state()
  private _currentPassword = "";
  @state()
  private _newPassword = "";
  @state()
  private _confirmPassword = "";
  @state()
  private _passwordSaving = false;
  @state()
  private _passwordError = "";

  // Sessions section state
  @state()
  private _sessions: DtoSessionResponse[] = [];
  @state()
  private _sessionsLoading = false;
  @state()
  private _revokingId: string | null = null;
  @state()
  private _notifPrefs: DtoNotificationPreferenceResponse[] = [];
  @state()
  private _notifPrefsLoading = true;

  private _saveTimer: ReturnType<typeof setTimeout> | null = null;
  private _savedTimer: ReturnType<typeof setTimeout> | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.#load();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._saveTimer) clearTimeout(this._saveTimer);
    if (this._savedTimer) clearTimeout(this._savedTimer);
  }

  async #load(): Promise<void> {
    this._loading = true;
    try {
      const { data } = await getSettingsPreferences({ throwOnError: true });
      this._prefs = data;
    } catch {
      // ignore
    } finally {
      this._loading = false;
    }
    this._profileName = auth.value.user?.name ?? "";
    this.#loadSessions();
    this.#loadNotifPrefs();
  }

  // Account section handlers
  async #saveProfile(): Promise<void> {
    const name = this._profileName.trim();
    if (!name) return;
    this._profileSaving = true;
    try {
      await patchAccount({ body: { name }, throwOnError: true });
      await fetchMe();
      showToast(msg("Profile updated"));
    } catch {
      showToast(msg("Failed to update profile"), { variant: "error" });
    } finally {
      this._profileSaving = false;
    }
  }

  async #onAvatarChange(e: Event): Promise<void> {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    this._avatarUploading = true;
    try {
      await postAccountAvatar({ body: { file }, throwOnError: true });
      await fetchMe();
      showToast(msg("Avatar updated"));
    } catch {
      showToast(msg("Failed to upload avatar"), { variant: "error" });
    } finally {
      this._avatarUploading = false;
      input.value = ""; // allow re-selecting the same file
    }
  }

  // Security section handlers
  async #changePassword(): Promise<void> {
    this._passwordError = "";
    if (this._newPassword !== this._confirmPassword) {
      this._passwordError = msg("New passwords do not match");
      return;
    }
    if (this._newPassword.length < 8) {
      this._passwordError = msg("New password must be at least 8 characters");
      return;
    }
    this._passwordSaving = true;
    try {
      await postAccountChangePassword({
        body: {
          current_password: this._currentPassword,
          new_password: this._newPassword,
        },
        throwOnError: true,
      });
      showToast(msg("Password changed — please log in again"));
      await logout();
      navigate("/login");
    } catch (err) {
      this._passwordError = err instanceof Error
        ? err.message
        : msg("Failed to change password");
    } finally {
      this._passwordSaving = false;
    }
  }

  // Sessions section handlers
  async #loadSessions(): Promise<void> {
    this._sessionsLoading = true;
    try {
      const { data } = await getAuthSessions({ throwOnError: true });
      this._sessions = data ?? [];
    } catch {
      this._sessions = [];
    } finally {
      this._sessionsLoading = false;
    }
  }

  async #revokeSession(id: string): Promise<void> {
    this._revokingId = id;
    try {
      await deleteAuthSessionsById({ path: { id }, throwOnError: true });
      this._sessions = this._sessions.filter((s) => s.id !== id);
      showToast(msg("Session revoked"), { variant: "success" });
    } catch {
      showToast(msg("Failed to revoke session"), { variant: "error" });
    } finally {
      this._revokingId = null;
    }
  }

  async #loadNotifPrefs(): Promise<void> {
    this._notifPrefsLoading = true;
    try {
      const { data: notifData } = await getSettingsNotifications({
        throwOnError: true,
      });
      this._notifPrefs = notifData ?? [];
    } catch {
      this._notifPrefs = [];
    } finally {
      this._notifPrefsLoading = false;
    }
  }

  async #toggleNotifType(type: string, enabled: boolean): Promise<void> {
    // Optimistic update
    this._notifPrefs = this._notifPrefs.map((p) =>
      p.type === type ? { ...p, enabled } : p
    );
    this.requestUpdate();
    try {
      await patchSettingsNotificationsByType({
        path: { type },
        body: { enabled },
        throwOnError: true,
      });
    } catch {
      // Revert on failure
      this._notifPrefs = this._notifPrefs.map((p) =>
        p.type === type ? { ...p, enabled: !enabled } : p
      );
      this.requestUpdate();
      showToast(msg("Failed to update notification preference"), {
        variant: "error",
      });
    }
  }

  // #initials returns up to 2 uppercase initials for the avatar fallback.
  #initials(): string {
    const name = auth.value.user?.name ?? "";
    return name
      .trim()
      .split(/\s+/)
      .map((w) => w[0])
      .join("")
      .slice(0, 2)
      .toUpperCase();
  }

  #current<K extends keyof DtoUserPreferencesResponse>(
    key: K,
  ): DtoUserPreferencesResponse[K] {
    if (this._prefs?.[key] !== undefined) return this._prefs[key]!;
    const defaults: DtoUserPreferencesResponse = {
      theme: "dark",
      language: "en",
      timezone: "UTC",
      email_notifications: true,
      desktop_notifications: true,
      notification_sounds: true,
      sidebar_collapsed: false,
    };
    return defaults[key];
  }

  /**
   * Apply a change locally and persist it to the server. Every setting on
   * this page saves on change: there is no explicit save button. Rapid
   * changes are debounced into a single PATCH.
   */
  #set<K extends keyof DtoUserPreferencesResponse>(
    key: K,
    value: DtoUserPreferencesResponse[K],
  ): void {
    if (!this._prefs) this._prefs = {} as DtoUserPreferencesResponse;
    this._prefs = { ...this._prefs, [key]: value };
    this.requestUpdate();
    this.#scheduleSave({ [key]: value } as Partial<DtoUserPreferencesResponse>);
  }

  #selectTheme(presetId: string): void {
    applyPreset(presetId);
    this.#set("theme", presetId);
  }

  #scheduleSave(patch: Partial<DtoUserPreferencesResponse>): void {
    // Merge into the pending patch so a burst of changes becomes one request.
    this._pendingPatch = { ...this._pendingPatch, ...patch };
    if (this._saveTimer) clearTimeout(this._saveTimer);
    this._saveState = "saving";
    this._saveTimer = setTimeout(() => this.#flushSave(), 400);
  }

  private _pendingPatch: Partial<DtoUserPreferencesResponse> = {};

  async #flushSave(): Promise<void> {
    const patch = this._pendingPatch;
    if (Object.keys(patch).length === 0) return;
    this._pendingPatch = {};
    this._saveState = "saving";
    try {
      const { data } = await patchSettingsPreferences({
        body: patch,
        throwOnError: true,
      });
      this._prefs = data;
      // Keep the global preferences signal in sync so app-shell can react
      // to desktop-notification toggles without a refetch.
      setPreferences(this._prefs);
      this._saveState = "saved";
      if (this._savedTimer) clearTimeout(this._savedTimer);
      this._savedTimer = setTimeout(() => {
        if (this._saveState === "saved") this._saveState = "idle";
      }, 2000);
    } catch {
      this._saveState = "error";
    }
  }

  protected render(): unknown {
    if (this._loading) {
      return html`
        <breeze-app-layout>
          <div class="loading">
            <breeze-spinner></breeze-spinner>
            ${msg("Loading preferences…")}
          </div>
        </breeze-app-layout>
      `;
    }

    return html`
      <breeze-app-layout>
        <div class="page-enter">
          <div class="page-head">
            <div>
              <h1>${msg("Preferences")}</h1>
              <p>${msg("Manage your account preferences")}</p>
            </div>
          </div>

          <div class="page-content">
            <div class="sections">
              <!-- Account -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Account")}</h2>
                  <p>${msg("Your display name and avatar")}</p>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Avatar")}</span>
                    <span class="description">${msg(
                      "Shown across all workspaces",
                    )}</span>
                  </div>
                  <div class="field-control field-control-row-lg">
                    <breeze-avatar size="lg"
                      src="${auth.value.user?.avatar_url ?? ""}">${this
                        .#initials()}</breeze-avatar>
                    <label class="avatar-upload">
                      <input
                        type="file"
                        accept="image/*"
                        ?disabled="${this._avatarUploading}"
                        @change="${(e: Event) => this.#onAvatarChange(e)}"
                      />
                      ${this._avatarUploading
                        ? html`<breeze-spinner></breeze-spinner><span>${
                          msg("Uploading…")
                        }</span>`
                        : msg("Upload")}
                    </label>
                  </div>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Display name")}</span>
                    <span class="description">${msg(
                      "Synced across all your workspaces",
                    )}</span>
                  </div>
                  <div class="field-control field-control-row">
                    <breeze-input
                      .value="${this._profileName}"
                      @input="${(
                        e: Event,
                      ) => (this._profileName =
                        (e.target as HTMLInputElement).value)}"
                    ></breeze-input>
                    <breeze-button
                      ?disabled="${this._profileSaving ||
                        !this._profileName.trim()}"
                      @click="${() => this.#saveProfile()}"
                    >${this._profileSaving
                      ? html`<breeze-spinner></breeze-spinner>`
                      : msg("Save")}</breeze-button>
                  </div>
                </div>
              </div>

              <hr class="theme-section-divider" />

              <!-- Security -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Security")}</h2>
                  <p>${msg("Change your password")}</p>
                </div>

                <div class="security-form">
                  <breeze-field label=${msg("Current password")}>
                    <breeze-input
                      type="password"
                      .value="${this._currentPassword}"
                      @input="${(
                        e: Event,
                      ) => (this._currentPassword =
                        (e.target as HTMLInputElement).value)}"
                    ></breeze-input>
                  </breeze-field>
                  <breeze-field label=${msg("New password")}>
                    <breeze-input
                      type="password"
                      .value="${this._newPassword}"
                      @input="${(
                        e: Event,
                      ) => (this._newPassword =
                        (e.target as HTMLInputElement).value)}"
                    ></breeze-input>
                  </breeze-field>
                  <breeze-field label=${msg("Confirm new password")}>
                    <breeze-input
                      type="password"
                      .value="${this._confirmPassword}"
                      @input="${(
                        e: Event,
                      ) => (this._confirmPassword =
                        (e.target as HTMLInputElement).value)}"
                    ></breeze-input>
                  </breeze-field>
                  ${this._passwordError
                    ? html`<div class="form-error">${this._passwordError}</div>`
                    : null}
                  <div>
                    <breeze-button
                      ?disabled="${this._passwordSaving ||
                        !this._currentPassword || !this._newPassword ||
                        !this._confirmPassword}"
                      @click="${() => this.#changePassword()}"
                    >${this._passwordSaving
                      ? html`<breeze-spinner></breeze-spinner><span>${
                        msg("Saving…")
                      }</span>`
                      : msg("Change password")}</breeze-button>
                  </div>
                </div>
              </div>

              <hr class="theme-section-divider" />

              <!-- Sessions -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Sessions")}</h2>
                  <p>${msg("Devices currently signed in to your account")}</p>
                </div>

                ${this._sessionsLoading
                  ? html`
                    <div class="sessions-loading">
                      <breeze-spinner></breeze-spinner> ${msg(
                        "Loading sessions…",
                      )}
                    </div>
                  `
                  : this._sessions.length === 0
                  ? html`
                    <div class="sessions-empty">${msg(
                      "No active sessions.",
                    )}</div>
                  `
                  : html`
                    <div class="session-list">
                      ${this._sessions.map(
                        (s) =>
                          html`
                            <div class="session-row">
                              <div class="session-icon">
                                <breeze-icon
                                  name="${sessionDeviceIcon(s.user_agent)}"
                                  size="18"
                                ></breeze-icon>
                              </div>
                              <div class="session-info">
                                <div class="session-title">
                                  ${summarizeUserAgent(s.user_agent)}
                                  ${s.is_current
                                    ? html`
                                      <span class="session-current-badge">${msg(
                                        "This device",
                                      )}</span>
                                    `
                                    : null}
                                  ${s.revoked_at
                                    ? html`
                                      <span class="session-revoked-badge">${msg(
                                        "Revoked",
                                      )}</span>
                                    `
                                    : null}
                                </div>
                                <div class="session-meta">
                                  ${s.ip_address
                                    ? html`<span>${s.ip_address}</span>`
                                    : null}
                                  ${s.ip_address && s.created_at
                                    ? html`<span aria-hidden="true">·</span>`
                                    : null}
                                  ${s.created_at
                                    ? html`<span>${
                                      timeAgoShort(s.created_at)
                                    }</span>`
                                    : null}
                                </div>
                              </div>
                              ${!s.is_current && !s.revoked_at
                                ? html`
                                  <breeze-button
                                    variant="ghost"
                                    ?disabled="${this._revokingId === s.id}"
                                    @click="${() =>
                                      this.#revokeSession(s.id ?? "")}">
                                    ${this._revokingId === s.id
                                      ? html`<breeze-spinner></breeze-spinner>`
                                      : msg("Revoke")}
                                  </breeze-button>
                                `
                                : null}
                            </div>
                          `,
                      )}
                    </div>
                  `}
              </div>

              <hr class="theme-section-divider" />

              <!-- Theme Presets -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Theme")}</h2>
                  <p>${msg(
                    "Choose your preferred color scheme and palette",
                  )}</p>
                </div>

                <div class="theme-grid">
                  ${THEME_PRESETS.map(
                    (preset) =>
                      html`
                        <div
                          class="theme-card${currentPreset.value === preset.id
                            ? " selected"
                            : ""}"
                          @click="${() => this.#selectTheme(preset.id)}"
                        >
                          <div class="theme-card-preview">
                            <span
                              class="bar"
                              style="background:${preset.colorHex}"
                            ></span>
                            <span
                              class="bar"
                              style="background:${preset.mode === "dark"
                                ? "oklch(0.21 0.006 285.885)"
                                : "oklch(0.985 0 0)"}"
                            ></span>
                          </div>
                          <div class="theme-card-name">${preset.label}</div>
                          <div class="theme-card-desc">${preset
                            .description}</div>
                        </div>
                      `,
                  )}
                </div>
              </div>

              <hr class="theme-section-divider" />

              <!-- Motion & Animation -->
              <div class="section">
                <breeze-motion-settings></breeze-motion-settings>
              </div>

              <hr class="theme-section-divider" />

              <!-- General Preferences -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("General")}</h2>
                  <p>${msg("Language, timezone, and region settings")}</p>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Language")}</span>
                    <span class="description">${msg(
                      "Select your preferred language",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-select
                      searchable
                      .options=${getLanguages()}
                      .value=${this.#current("language")}
                      placeholder=${msg("Select language")}
                      @change=${(e: CustomEvent) =>
                        this.#set("language", (e.detail as string) ?? "")}
                    ></breeze-select>
                  </div>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Timezone")}</span>
                    <span class="description">${msg(
                      "Used for due dates and activity timestamps",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-select
                      searchable
                      .options=${TIMEZONES.map((tz) => ({
                        value: tz,
                        label: tz,
                      }))}
                      .value=${this.#current("timezone")}
                      placeholder=${msg("Select timezone")}
                      @change=${(e: CustomEvent) =>
                        this.#set("timezone", (e.detail as string) ?? "")}
                    ></breeze-select>
                  </div>
                </div>
              </div>

              <!-- Notifications -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Notification channels")}</h2>
                  <p>${msg("Choose how notifications are delivered")}</p>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Email notifications")}</span>
                    <span class="description">${msg(
                      "Receive email for mentions and assignments",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-switch
                      .checked="${this.#current("email_notifications")}"
                      @change="${(e: CustomEvent) =>
                        this.#set(
                          "email_notifications",
                          (e.detail as { checked: boolean }).checked,
                        )}"
                    ></breeze-switch>
                  </div>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Desktop notifications")}</span>
                    <span class="description">${msg(
                      "Show browser notifications for new messages",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-switch
                      .checked="${this.#current("desktop_notifications")}"
                      @change="${(e: CustomEvent) =>
                        this.#set(
                          "desktop_notifications",
                          (e.detail as { checked: boolean }).checked,
                        )}"
                    ></breeze-switch>
                  </div>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Notification sounds")}</span>
                    <span class="description">${msg(
                      "Play a sound when receiving notifications",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-switch
                      .checked="${this.#current("notification_sounds")}"
                      @change="${(e: CustomEvent) =>
                        this.#set(
                          "notification_sounds",
                          (e.detail as { checked: boolean }).checked,
                        )}"
                    ></breeze-switch>
                  </div>
                </div>
              </div>

              <!-- Notification Types (per-type) -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Notification types")}</h2>
                  <p>${msg("Choose which events trigger notifications")}</p>
                </div>

                ${this._notifPrefsLoading
                  ? html`
                    <div class="sessions-loading">
                      <breeze-spinner></breeze-spinner> ${msg(
                        "Loading preferences…",
                      )}
                    </div>
                  `
                  : this._notifPrefs.map(
                    (pref) =>
                      html`
                        <div class="field-row">
                          <div class="field-label">
                            <span class="label">${getNotificationLabel(
                              pref.type ?? "",
                            )}</span>
                          </div>
                          <div class="field-control">
                            <breeze-switch
                              .checked="${pref.enabled}"
                              @change="${(e: CustomEvent) =>
                                this.#toggleNotifType(
                                  pref.type ?? "",
                                  (e.detail as { checked: boolean }).checked,
                                )}"
                            ></breeze-switch>
                          </div>
                        </div>
                      `,
                  )}
              </div>

              <hr class="theme-section-divider" />

              <!-- Sidebar -->
              <div class="section">
                <div class="section-header">
                  <h2>${msg("Sidebar")}</h2>
                  <p>${msg("Configure sidebar behavior")}</p>
                </div>

                <div class="field-row">
                  <div class="field-label">
                    <span class="label">${msg("Collapsed by default")}</span>
                    <span class="description">${msg(
                      "Start with the sidebar collapsed on new sessions",
                    )}</span>
                  </div>
                  <div class="field-control">
                    <breeze-switch
                      .checked="${this.#current("sidebar_collapsed")}"
                      @change="${(e: CustomEvent) =>
                        this.#set(
                          "sidebar_collapsed",
                          (e.detail as { checked: boolean }).checked,
                        )}"
                    ></breeze-switch>
                  </div>
                </div>
              </div>

              <!-- Auto-save status (no save button: changes persist on change) -->
              <div class="save-status${this._saveState !== "idle"
                ? ` ${this._saveState}`
                : ""}">
                <span class="dot"></span>
                ${this._saveState === "saving"
                  ? msg("Saving…")
                  : this._saveState === "saved"
                  ? msg("All changes saved")
                  : this._saveState === "error"
                  ? msg("Failed to save — will retry on next change")
                  : msg("Changes save automatically")}
              </div>
            </div>
          </div>
        </div>
      </breeze-app-layout>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "breeze-user-settings-page": BreezeUserSettingsPage;
  }
}
