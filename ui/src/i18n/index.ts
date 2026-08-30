/**
 * Breeze internationalization entry point.
 *
 * Wraps `@lit/localize`'s runtime configuration and exposes the functions the
 * rest of the app needs: `msg`, `str`, `@localized`, `getLocale`, and
 * `setLocale` (with locale persistence + `<html lang>` syncing built in).
 *
 * Re-exports the runtime config (`getLocale`, `setLocale`) plus locale-list
 * helpers used by the Settings dropdown and the format module. Components
 * import `msg`, `localized`, and `str` **directly from `@lit/localize`**, not
 * from here: because `lit-localize extract` resolves the `msg` brand
 * symbol (`_LIT_LOCALIZE_MSG_`) via TypeScript's type-checker, and that
 * brand is lost when re-exported through a wrapper module. This is a
 * hard constraint of the extraction toolchain.
 *
 * @lit/localize runtime mode:
 *   - `en` is the source locale; `msg("Save")` returns `"Save"` with no lookup.
 *   - Target locales (currently `fr`) are lazy-loaded via `loadLocale` when
 *     `setLocale` is called.
 *   - `@localized()` Lit elements re-render automatically on locale change.
 *
 * Locale selection + persistence:
 *   - Authenticated users: `user_preferences.language` (BCP-47) is the single
 *     source of truth; `setLocale` is called from `app-shell.ts` after prefs
 *     load, and from the Settings page when the user changes the dropdown.
 *   - Unauthenticated users (login/setup): `detectLocale()` falls back to
 *     `navigator.language`, then `"en"`.
 *
 * See docs/i18n/ for the full design (PLAN.md, ARCHITECTURE.md,
 * adding-a-language.md, translator-guide.md).
 */
import { configureLocalization } from "@lit/localize";

/**
 * Supported BCP-47 locale codes. `en` is the source locale.
 * NOTE: keep in sync with `lit-localize.json` + the generated `locale-codes.ts`.
 */
export const SOURCE_LOCALE = "en" as const;

/** Locales shipped with Breeze (excludes the source locale). */
export const TARGET_LOCALES = ["fr"] as const;

/** Every locale Breeze supports, including the source. */
export const ALL_LOCALES = [SOURCE_LOCALE, ...TARGET_LOCALES] as const;

export type LocaleCode = (typeof ALL_LOCALES)[number];

/** Human-readable labels for the Settings dropdown (rendered in their own language). */
export const LOCALE_LABELS: Record<LocaleCode, string> = {
  en: "English",
  fr: "Français",
};

/**
 * Returns `true` if the given tag is a locale Breeze supports. Performs only a
 * direct check: callers that receive an arbitrary user/`Accept-Language` tag
 * should use `normalizeLocale()` which falls back through region→language→en.
 */
export function isSupportedLocale(tag: string): tag is LocaleCode {
  return (ALL_LOCALES as readonly string[]).includes(tag);
}

/**
 * Normalize an arbitrary locale tag to one Breeze supports, applying the
 * fallback chain: exact match → language-only match → `SOURCE_LOCALE`.
 *
 *   "fr"        → "fr"
 *   "fr-CA"     → "fr"        (region stripped, language matches)
 *   "pt-BR"     → "en"        (language "pt" not supported → source)
 *   "" / "de"   → "en"
 */
export function normalizeLocale(tag: string | undefined | null): LocaleCode {
  if (!tag) return SOURCE_LOCALE;
  const lower = tag.toLowerCase();
  if (isSupportedLocale(lower)) return lower;
  const lang = lower.split("-")[0];
  if (isSupportedLocale(lang)) return lang;
  return SOURCE_LOCALE;
}

/** localStorage key caching the last selected locale (unauthenticated routes). */
const LOCALE_STORAGE_KEY = "breeze.locale";

/**
 * Detect the best locale for an unauthenticated user:
 * `localStorage` cache → `navigator.language`/`navigator.languages` → `en`.
 * Persists the result so the login page doesn't re-flash English on reload.
 */
export function detectLocale(): LocaleCode {
  try {
    const cached = localStorage.getItem(LOCALE_STORAGE_KEY);
    if (cached) {
      const normalized = normalizeLocale(cached);
      if (normalized !== cached) {
        localStorage.setItem(LOCALE_STORAGE_KEY, normalized);
      }
      return normalized;
    }
  } catch {
    // localStorage may be unavailable (private mode); fall through.
  }
  const langs = typeof navigator !== "undefined"
    ? [navigator.language, ...(navigator.languages ?? [])]
    : [];
  for (const l of langs) {
    const normalized = normalizeLocale(l);
    if (normalized !== SOURCE_LOCALE) {
      try {
        localStorage.setItem(LOCALE_STORAGE_KEY, normalized);
      } catch {
        // ignore
      }
      return normalized;
    }
  }
  return SOURCE_LOCALE;
}

// Lazy-load a target locale's runtime templates. Vite/Deno bundler splits
// each import into its own chunk so only the requested locale ships.
// The generated locale modules export `templates` (a map of message id →
// translated string). `@lit/localize` types this as `LocaleModule`; we use a
// structural cast to avoid importing the package's internal types path.
const loadLocale = (
  locale: string,
): Promise<{ templates: Record<string, string> }> =>
  import(`./locales/${locale}.js`);

const { getLocale, setLocale: setLocaleInternal } = configureLocalization({
  sourceLocale: SOURCE_LOCALE,
  targetLocales: [...TARGET_LOCALES],
  loadLocale,
});

/** Current active locale code. */
export { getLocale };

/** Sync `<html lang>` for accessibility, screen readers, and CSS `:lang()`. */
function syncHtmlLang(locale: string): void {
  if (typeof document !== "undefined") {
    document.documentElement.lang = locale;
  }
}

/**
 * Set the active locale. Wraps `@lit/localize`'s `setLocale` to also:
 *   - normalize the tag to a supported locale (safe to call with any input),
 *   - sync `<html lang>`,
 *   - cache the choice in `localStorage` (used by unauthenticated routes).
 *
 * For the source locale (`en`) this resolves immediately (no chunk to load).
 */
export async function setLocale(locale: string): Promise<void> {
  const normalized = normalizeLocale(locale);
  if (normalized === getLocale()) {
    syncHtmlLang(normalized);
    return;
  }
  // The source locale needs no loadLocale() call; @lit/localize handles it.
  // Target locales trigger a lazy import + @localized re-render.
  await setLocaleInternal(normalized);
  syncHtmlLang(normalized);
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, normalized);
  } catch {
    // localStorage unavailable; locale still applies for this session.
  }
}
