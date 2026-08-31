# i18n Architecture

> **How Plume's internationalization fits together.** Reference for developers.
> For *how to add a language*, see [adding-a-language.md](./adding-a-language.md).

## Component map

```
┌─ FRONTEND (Lit 3 + Vite + Deno) ──────────────────────────────────────┐
│                                                                        │
│  user preferences (signal) ──▶ setLocale(lang) ──▶ @lit/localize        │
│         ▲                                    │                         │
│         │                                    ▼                         │
│  GET /api/preferences              lazy-import locales/<lang>.js        │
│         │                                    │                         │
│         │                                    ▼                         │
│  <html lang="<lang>">             msg("Save") → localized string        │
│  (set on locale change)                        │                        │
│                                                ▼                        │
│  Intl.DateTimeFormat / RelativeTimeFormat / NumberFormat  ◀── getLocale() │
│  (in src/i18n/format.ts; replaces lib/format/ hardcoded English)        │
└────────────────────────────────────────────────────────────────────────┘
                         │
                         │  user_preferences.language (BCP-47, e.g. "fr")
                         ▼
┌─ BACKEND (Go, single binary) ─────────────────────────────────────────┐
│                                                                        │
│  request ──▶ middleware (auth) ──▶ ResolveLocale(userPref, Accept-L)   │
│                                          │                             │
│                                          ▼                             │
│                            i18n.NewLocalizer(bundle, lang, "en")        │
│                                          │                             │
│            ┌─────────────────────────────┼──────────────────────┐       │
│            ▼                             ▼                      ▼       │
│   MailService                  PushService            RespondWithError │
│   (mail_templates.go)          (push_service.go)     (transport)       │
│   localizer.Localize(          localizer.Localize(   localizer.Localize│
│     {MessageID, TemplateData,    {MessageID, …})        {MessageID, …})│
│      PluralCount})                                                    │
│            │                             │                      │       │
│            ▼                             ▼                      ▼       │
│   i18n.Bundle (one, embedded via go:embed)                            │
│   internal/i18n/messages/{mail,push,errors}/active.{en,fr}.toml        │
└────────────────────────────────────────────────────────────────────────┘
```

## Frontend pieces

| File | Role |
|---|---|
| `ui/lit-localize.json` | `@lit/localize` config: `sourceLocale`, `targetLocales`, runtime mode, XLIFF interchange dir |
| `ui/src/i18n/index.ts` | `configureLocalization()` → `{ getLocale, setLocale }`; locale-list helpers, `detectLocale`, `normalizeLocale`. **Does NOT re-export `msg`/`localized`/`str`** (see constraint below) |
| `ui/src/i18n/locale-codes.ts` | **generated**; `sourceLocale`, `targetLocales`, `allLocales` arrays |
| `ui/src/i18n/locales/<lang>.js` | **generated**; runtime module per locale, lazy-`import()`ed by `setLocale` |
| `ui/src/i18n/messages/<lang>.xlf` | **translator-edited**; XLIFF 1.2 source of truth for translations |
| `ui/src/i18n/format.ts` | locale-aware `Intl.*` wrappers (date/time/number/relative); replaces `lib/format/` |
| `ui/src/features/settings/user-settings-page.ts` | `LANGUAGES` dropdown → `setLocale()` + `PUT /api/preferences` |
| `ui/src/app-shell.ts` | on boot: `detectLocale()` for unauthenticated; `loadPreferences()` → `setLocale(prefs.language)` |

> **⚠ Critical extraction constraint:** Components MUST import `msg`, `localized`,
> and `str` **directly from `@lit/localize`**, not from `@/i18n`. The
> `lit-localize extract` CLI uses TypeScript's type-checker to find the
> `_LIT_LOCALIZE_MSG_` brand symbol on each `msg` call; re-exporting `msg`
> through a wrapper module loses that brand, and extraction silently misses
> those strings. `@/i18n` is only for runtime config (`getLocale`/`setLocale`)
> and locale-list helpers. This is a hard constraint of the toolchain.

**Reactivity:** `@localized()` decorator (or `updateWhenLocaleChanges()`) makes a Lit element
re-render whenever `setLocale()` is called. No manual re-render wiring.

## Backend pieces

| File | Role |
|---|---|
| `internal/i18n/i18n.go` | `*i18n.Bundle` (go:embed of `messages/*/active.*.toml`), `NewLocalizer(locale)` factory |
| `internal/i18n/locale.go` | supported-locales list, `ResolveLocale(userPref, r)` fallback chain, `IsValidLocale()` |
| `internal/i18n/messages/mail/active.<lang>.toml` | email template strings |
| `internal/i18n/messages/push/active.<lang>.toml` | push notification strings |
| `internal/i18n/messages/errors/active.<lang>.toml` | user-facing error / apperr strings |

**Resolution chain:** `ResolveLocale(userPref, r)` returns the best BCP-47 tag:
`user_preferences.language` → `Accept-Language` header → `"en"` (default). The matcher uses
`golang.org/x/text/language` (CLDR), so `pt-BR` falls back to `pt` if `pt-BR` isn't present,
and an unsupported tag falls back to `en`.

**Per-operation localizer:** `i18n.NewLocalizer(bundle, lang, "en")` is cheap (struct alloc).
Services build one per send, not a global. Missing message → falls back to `en`.

## Locale lifecycle

```
BOOT (authenticated)
  1. GET /api/preferences → { language: "fr", … }
  2. setLocale("fr") → lazy import locales/fr.js → @localized components render French
  3. document.documentElement.lang = "fr"

BOOT (unauthenticated; login/setup)
  1. detect: localStorage('plume.locale') → navigator.language → "en"
  2. setLocale(detected)

USER CHANGES LANGUAGE in Settings
  1. setLocale("fr") immediately (optimistic; UI updates before the save)
  2. PUT /api/preferences { language: "fr" }
  3. on failure: revert setLocale() + show toast

BACKEND SENDS AN EMAIL / PUSH
  1. ResolveLocale(userPref, r) → "fr"
  2. NewLocalizer(bundle, "fr", "en")
  3. localizer.Localize({ MessageID, TemplateData, PluralCount }) → French string
  4. send
```

## The persistence that already exists

The `user_preferences.language` column, domain field, sqlc query, service
update, and API DTO **all already exist** end-to-end. The i18n work does **not**
touch the persistence layer; it wires the stored value into rendering.

## File-format summary

| Layer | Format | Why |
|---|---|---|
| Frontend interchange | XLIFF 1.2 (`.xlf`) | Only format `@lit/localize` CLI supports; industry standard for CAT tools |
| Backend messages | TOML | go-i18n convention; developer-edited (not translator-edited), so CAT-tool ecosystem doesn't apply |
| Locale tags | BCP 47 (`en`, `fr`) | Neutral language subtag; `Intl` resolves bare `en` to US-English formatting; extensible |

