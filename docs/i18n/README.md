# Plume i18n (Internationalization)

Plume is fully internationalizable. A user picks a language in Settings and
the entire product (UI, emails, push notifications, and errors) renders in
that language, with locale-aware date/time/number formatting.

This directory is the source of truth for the feature. Start here.

## Documents

| Doc | Audience | What it covers |
|---|---|---|
| **[ARCHITECTURE.md](./ARCHITECTURE.md)** | Developers | How the pieces fit together (frontend + backend), component map, locale lifecycle. |
| **[adding-a-language.md](./adding-a-language.md)** | Developers | Step-by-step runbook for adding a new locale, frontend + backend. |
| **[translator-guide.md](./translator-guide.md)** | Translators | How to edit XLIFF files; placeholder & plural rules. No code knowledge needed. |

- **Source locale:** `en` (US-English content, neutral BCP 47 tag for extensibility)
- **Launch locales:** `en` + `fr` (French chosen to exercise the plural pipeline: its `one` = {0,1} vs English's {1})
- **Frontend library:** `@lit/localize` v0.12.2 (runtime mode)
- **Backend library:** `nicksnyder/go-i18n/v2` v2.6.1 (TOML messages)
- **Date/time/number:** native `Intl.*` APIs (no date-fns/dayjs)
- **Persistence:** the `user_preferences.language` column already exists end-to-end; no schema work needed

## Adding a language

Follow **[adding-a-language.md](./adding-a-language.md)**. TL;DR:
1. Add the tag to `ui/lit-localize.json` `targetLocales` + `internal/i18n/locale.go`
2. `make i18n-extract` → translate `ui/src/i18n/messages/<lang>.xlf`
3. Copy + translate `internal/i18n/messages/*/active.<lang>.toml`
4. Add the locale to the Settings `LANGUAGES` dropdown
5. `make i18n-build && make build-ui && make build`
