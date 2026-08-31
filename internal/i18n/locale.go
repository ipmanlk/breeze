// Package i18n provides Plume's internationalization runtime for the Go
// backend: an embedded message bundle (go-i18n + TOML), locale resolution
// (user preference → Accept-Language → source), and a per-request Localizer
// factory.
//
// The frontend has its own i18n runtime (@lit/localize); this package handles
// server-generated strings: emails, push notifications, in-app notification
// content, and user-facing error messages.
//
// Locale model:
//   - Source locale: "en" (matches the frontend + the DB default).
//   - Target locales: currently "fr". Additive; see docs/i18n/adding-a-language.md.
//   - Tags are BCP 47 language subtags (neutral; regions added only when a
//     regional variant is genuinely needed).
//
// Resolution chain (ResolveLocale):
//
//	user preference → Accept-Language header → source locale ("en").
//
// Region fallback is supported: "fr-CA" → "fr" if "fr" is supported, else "en".
package i18n

import (
	"net/http"
	"strings"

	"golang.org/x/text/language"
)

// SourceLocale is the locale the source strings are written in. It is also the
// final fallback when no translation is available.
const SourceLocale = "en"

// supportedLocales is the set of locales Plume ships message files for.
// SourceLocale is always first. Keep in sync with ui/lit-localize.json's
// sourceLocale + targetLocales, and with the .toml files under messages/.
var supportedLocales = []string{SourceLocale, "fr"}

// matcher matches an arbitrary tag against the supported set, with region
// fallback (e.g. "fr-CA" → "fr"). Built once; reused across requests.
var matcher = language.NewMatcher(supportedTags())

func supportedTags() []language.Tag {
	tags := make([]language.Tag, len(supportedLocales))
	for i, l := range supportedLocales {
		tags[i] = language.Make(l)
	}
	return tags
}

// SupportedLocales returns the locales Plume has message catalogs for
// (including the source). Used by validation and the Settings dropdown.
func SupportedLocales() []string {
	out := make([]string, len(supportedLocales))
	copy(out, supportedLocales)
	return out
}

// IsSupported reports whether the given tag (exact, lowercased) is a supported
// locale. Use Normalize for arbitrary input that may need region fallback.
func IsSupported(tag string) bool {
	tag = strings.ToLower(tag)
	for _, l := range supportedLocales {
		if l == tag {
			return true
		}
	}
	return false
}

// Normalize resolves an arbitrary locale tag to a supported one, applying
// region→language fallback. Returns SourceLocale when nothing matches.
//
//	"fr"     → "fr"
//	"fr-CA"  → "fr"
//	"pt-BR"  → "en"   (pt not supported)
//	""       → "en"
func Normalize(tag string) string {
	if tag == "" {
		return SourceLocale
	}
	t, _, err := language.ParseAcceptLanguage(tag)
	if err != nil || len(t) == 0 {
		// Not a valid Accept-Language list; try parsing as a single tag.
		parsed, err := language.Parse(tag)
		if err != nil {
			return SourceLocale
		}
		t = []language.Tag{parsed}
	}
	_, idx, _ := matcher.Match(t...)
	return supportedLocales[idx]
}

// ResolveLocale applies the resolution chain:
//
//	userPref (if non-empty) → Accept-Language header → SourceLocale.
//
// Pass userPref = "" when there is no stored preference (unauthenticated
// requests, or a user who never set one). Normalize handles region fallback
// and always returns a supported locale (defaulting to SourceLocale).
func ResolveLocale(userPref string, r *http.Request) string {
	if userPref != "" {
		return Normalize(userPref)
	}
	if r != nil {
		return Normalize(r.Header.Get("Accept-Language"))
	}
	return SourceLocale
}
