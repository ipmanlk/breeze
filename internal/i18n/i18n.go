package i18n

import (
	"context"
	"embed"
	"fmt"
	"net/http"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"ipmanlk/breeze/internal/domain"
)

//go:embed messages/*/active.*.toml
var messageFS embed.FS

// CtxLocale is the request-context key carrying the resolved locale tag.
// Defined in domain.context as a domain.ContextKey (matching the codebase
// convention for context keys), re-exported here for the i18n package's
// middleware. Set by the Locale middleware so handlers/services can read the
// requester's locale without re-resolving. For per-recipient localization
// (e.g. sending an email to a *different* user), services resolve the
// recipient's locale from their user_preferences directly, not from this
// context value.
var CtxLocale = domain.CtxLocale

// Bundle is the shared, embedded go-i18n message bundle. Constructed once at
// startup (NewBundle) and safe for concurrent reads. NewLocalizer is cheap,
// so build one per operation rather than caching per-locale.
type Bundle struct {
	b *goi18n.Bundle
}

// NewBundle loads all embedded TOML message files into a go-i18n bundle.
// Panics if a required source-locale file is missing or malformed (a
// build-time authoring error). Missing target-locale files are skipped
// gracefully; a locale with no translations falls back to source at runtime.
func NewBundle() *Bundle {
	b := goi18n.NewBundle(language.Make(SourceLocale))
	b.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, lang := range supportedLocales {
		for _, domain := range []string{"mail", "push", "errors"} {
			path := "messages/" + domain + "/active." + lang + ".toml"
			data, err := messageFS.ReadFile(path)
			if err != nil {
				// Only the source locale's files are required; target-locale
				// files are optional until translated.
				if lang == SourceLocale {
					panic(fmt.Sprintf("i18n: required source message file missing: %s: %v", path, err))
				}
				continue
			}
			if _, err := b.ParseMessageFileBytes(data, domain+"/active."+lang+".toml"); err != nil {
				panic(fmt.Sprintf("i18n: parse %s: %v", path, err))
			}
		}
	}
	return &Bundle{b: b}
}

// NewLocalizer builds a go-i18n Localizer for the given locale, with fallback
// to the source locale. Cheap to construct (struct + map lookup); build one
// per email/push/error rather than caching.
func (b *Bundle) NewLocalizer(locale string) *goi18n.Localizer {
	return goi18n.NewLocalizer(b.b, Normalize(locale), SourceLocale)
}

// LocalizeConfig mirrors go-i18n's LocalizeConfig: a MessageID plus optional
// template data and plural count. Callers that define DefaultMessage in code
// should use the go-i18n Localizer directly via NewLocalizer.
type LocalizeConfig = goi18n.LocalizeConfig

// Localize looks up a message by ID in the given locale, with fallback to the
// source locale. TemplateData fills {{.Name}} placeholders; PluralCount
// selects the plural category (one/other/…). Returns the source string when
// the ID is missing (go-i18n's default fallback behavior).
//
// For messages defined with a DefaultMessage in code, prefer NewLocalizer +
// loc.Localize(&LocalizeConfig{DefaultMessage: …}) directly.
func (b *Bundle) Localize(locale, messageID string, templateData map[string]any, pluralCount any) (string, error) {
	loc := b.NewLocalizer(locale)
	return loc.Localize(&goi18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
		PluralCount:  pluralCount,
	})
}

// MustLocalize is like Localize but returns the messageID on error (never
// panics); useful in mail/push paths where a missing translation should
// surface the ID rather than crash the send.
func (b *Bundle) MustLocalize(locale, messageID string, templateData map[string]any, pluralCount any) string {
	s, err := b.Localize(locale, messageID, templateData, pluralCount)
	if err != nil {
		return messageID
	}
	return s
}

// LocaleFromContext returns the locale stashed in ctx by the Locale middleware,
// or SourceLocale if unset (e.g. background jobs with no request context).
func LocaleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxLocale).(string); ok && v != "" {
		return v
	}
	return SourceLocale
}

// WithLocale stashes a resolved locale into ctx. Used by the Locale middleware.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, CtxLocale, Normalize(locale))
}

// LocaleMiddleware resolves the requester's locale (user preference if
// available on the context, else Accept-Language) and stashes it in the
// request context under CtxLocale. Handlers read it via LocaleFromContext.
//
// userPrefFromCtx, if non-nil, extracts the stored user language from the
// context (e.g. from an earlier auth middleware that loaded preferences).
// When nil or returning "", the middleware falls back to Accept-Language.
func (b *Bundle) LocaleMiddleware(userPrefFromCtx func(context.Context) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var pref string
			if userPrefFromCtx != nil {
				pref = userPrefFromCtx(r.Context())
			}
			locale := ResolveLocale(pref, r)
			ctx := WithLocale(r.Context(), locale)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
