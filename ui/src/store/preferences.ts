import { signal } from "@preact/signals-core";
import { getSettingsPreferences } from "@/api";
import type { DtoUserPreferencesResponse } from "@/api";
import { setLocale } from "@/i18n";

/**
 * User preferences signal.
 *
 * The backend stores the full preferences object; this signal mirrors just
 * the fields the app needs to read *outside* the settings page (currently:
 * `desktop_notifications`, consumed by app-shell to fire browser
 * notifications on `notification_new` WS events; and `language`, applied to
 * `setLocale()` here).
 *
 * The settings page remains the source of truth for editing: it calls
 * `setPreferences` after every save so this signal stays in sync without a
 * refetch.
 */
export interface PreferencesState {
  /** Full prefs object (may be partial until first load completes). */
  prefs: Partial<DtoUserPreferencesResponse>;
  loaded: boolean;
}

export const preferences = signal<PreferencesState>({
  prefs: {},
  loaded: false,
});

const DEFAULTS: { desktop_notifications: boolean } = {
  desktop_notifications: true,
};

/** Load preferences from the server. Safe to call once after auth. */
export async function loadPreferences(): Promise<void> {
  try {
    const { data } = await getSettingsPreferences({ throwOnError: true });
    preferences.value = {
      prefs: data ?? {},
      loaded: true,
    };
    // Apply the user's saved language to the i18n runtime. This overrides the
    // browser-based detectLocale() guess from app-shell for authenticated
    // users. Safe to call with any value: setLocale normalizes + falls back
    // to "en". Fire-and-forget: the promise resolves when the locale chunk
    // loads, but UI already renders in the source locale meanwhile.
    void setLocale(data?.language ?? "en");
  } catch {
    // Non-fatal: defaults will be used.
    preferences.value = { prefs: {}, loaded: true };
  }
}

/** Merge a partial update into the cached preferences (call after a save). */
export function setPreferences(
  patch: Partial<DtoUserPreferencesResponse>,
): void {
  preferences.value = {
    ...preferences.value,
    prefs: { ...preferences.value.prefs, ...patch },
  };
  // If the language changed, apply it immediately so the UI switches without
  // a reload. Optimistic: the settings page has already persisted via the API.
  if (patch.language !== undefined) {
    void setLocale(patch.language);
  }
}

/** True if desktop notifications are enabled (default: true). */
export function desktopNotificationsEnabled(): boolean {
  const v = preferences.value.prefs.desktop_notifications;
  return v === undefined ? DEFAULTS.desktop_notifications : !!v;
}
