/**
 * Motion preferences store.
 *
 * Persists animation preferences to localStorage (for instant apply on
 * page load) AND to the server via the preferences API (so they follow
 * the user across devices/browsers).
 *
 * Importing this module at app initialization (main.ts) is
 * sufficient to apply saved preferences from localStorage. After auth,
 * call loadMotionFromPreferences() to sync from the server.
 */

import { getSettingsPreferences, patchSettingsPreferences } from "@/api";

export interface MotionSettings {
  global: boolean;
  page: boolean;
  feedback: boolean;
  layout: boolean;
  overlay: boolean;
  list: boolean;
  loading: boolean;
  dnd: boolean;
  notify: boolean;
  chat: boolean;
  voice: boolean;
  scale: number;
}

const STORAGE_KEY = "plume-motion";

/** Reads the OS prefers-reduced-motion setting (SSR-safe). */
function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true;
}

const DEFAULTS: MotionSettings = {
  global: true,
  page: true,
  feedback: true,
  layout: true,
  overlay: true,
  list: true,
  loading: true,
  dnd: true,
  notify: true,
  chat: true,
  voice: true,
  scale: 1,
};

function load(): MotionSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...DEFAULTS, ...JSON.parse(raw) };
  } catch {
    // ignore corrupt data
  }
  // No saved preference yet: honor the OS reduced-motion setting as the
  // default. The user can still override via Settings; the override is
  // persisted so it sticks across reloads (handled by motionSettings.set).
  if (prefersReducedMotion()) return { ...DEFAULTS, global: false };
  return { ...DEFAULTS };
}

export function applyMotionSettings(settings: MotionSettings): void {
  const root = document.documentElement;
  // Master switch: when global animations are off, force --motion-scale to 0
  // so every --dur-* token (which multiplies by it) resolves to 0ms. Custom
  // properties inherit across shadow boundaries, so this reaches inside Lit
  // shadow-DOM components too: not just light-DOM elements. The speed slider
  // only takes effect when animations are enabled.
  //
  // NOTE: this MUST be set inline (not via the :root[data-motion="0"] rule in
  // motion-disable.css) because any inline --motion-scale value wins over
  // that stylesheet rule, and we always write one below for the speed slider.
  const effectiveScale = settings.global ? settings.scale : 0;
  root.dataset.motion = settings.global ? "1" : "0";
  root.dataset.motionPage = settings.page ? "1" : "0";
  root.dataset.motionFeedback = settings.feedback ? "1" : "0";
  root.dataset.motionLayout = settings.layout ? "1" : "0";
  root.dataset.motionOverlay = settings.overlay ? "1" : "0";
  root.dataset.motionList = settings.list ? "1" : "0";
  root.dataset.motionLoading = settings.loading ? "1" : "0";
  root.dataset.motionDnd = settings.dnd ? "1" : "0";
  root.dataset.motionNotify = settings.notify ? "1" : "0";
  root.dataset.motionChat = settings.chat ? "1" : "0";
  root.dataset.motionVoice = settings.voice ? "1" : "0";
  root.style.setProperty("--motion-scale", String(effectiveScale));

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // storage full or unavailable
  }
}

const initial = load();
applyMotionSettings(initial);

export const motionSettings: {
  value: MotionSettings;
  set: (s: MotionSettings) => void;
} = {
  value: { ...initial },
  set(settings: MotionSettings) {
    this.value = settings;
    applyMotionSettings(settings);
    scheduleSaveToServer(settings);
  },
};

// Server sync
let saveTimer: ReturnType<typeof setTimeout> | null = null;

/** Debounced save: batches rapid slider/toggle changes into one request. */
function scheduleSaveToServer(settings: MotionSettings): void {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    saveMotionToServer(settings);
  }, 500);
}

function saveMotionToServer(settings: MotionSettings): void {
  patchSettingsPreferences({
    body: { motion_settings: JSON.stringify(settings) },
    throwOnError: true,
  }).catch(() => {
    // Best-effort: preferences still work from localStorage.
  });
}

/**
 * Load motion settings from server preferences after auth.
 * Falls back to localStorage if the server has no saved settings.
 */
export async function loadMotionFromPreferences(): Promise<void> {
  try {
    const { data } = await getSettingsPreferences({ throwOnError: true });
    const raw = (data as Record<string, unknown>)?.motion_settings;
    if (typeof raw === "string" && raw) {
      const parsed = JSON.parse(raw);
      const merged = { ...DEFAULTS, ...parsed };
      motionSettings.value = merged;
      applyMotionSettings(merged);
    }
  } catch {
    // ignore: localStorage already applied on import
  }
}
