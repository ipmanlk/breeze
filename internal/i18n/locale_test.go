package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"en", "en"},
		{"EN", "en"},
		{"fr", "fr"},
		{"fr-CA", "fr"}, // region stripped, language matches
		{"fr-FR", "fr"},
		{"pt-BR", "en"}, // pt not supported → source
		{"de", "en"},    // not supported → source
		{"", "en"},      // empty → source
		{"zh-Hans", "en"},
		{"en-US", "en"},
		{"xx-XX", "en"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestResolveLocale(t *testing.T) {
	// Helper: build a request with an Accept-Language header.
	req := func(acceptLang string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if acceptLang != "" {
			r.Header.Set("Accept-Language", acceptLang)
		}
		return r
	}

	t.Run("user pref wins", func(t *testing.T) {
		r := req("en")
		if got := ResolveLocale("fr", r); got != "fr" {
			t.Errorf("ResolveLocale(pref=fr, Accept-Language=en) = %q, want fr", got)
		}
	})

	t.Run("user pref en wins over Accept-Language fr", func(t *testing.T) {
		// An explicit "en" preference should win over a browser requesting fr.
		r := req("fr")
		if got := ResolveLocale("en", r); got != "en" {
			t.Errorf("ResolveLocale(pref=en, Accept-Language=fr) = %q, want en", got)
		}
	})

	t.Run("empty pref falls back to Accept-Language", func(t *testing.T) {
		r := req("fr-FR,fr;q=0.9")
		if got := ResolveLocale("", r); got != "fr" {
			t.Errorf("ResolveLocale(pref='', Accept-Language=fr-FR) = %q, want fr", got)
		}
	})

	t.Run("empty pref + unsupported Accept-Language falls back to en", func(t *testing.T) {
		r := req("de,pt-BR;q=0.8")
		if got := ResolveLocale("", r); got != "en" {
			t.Errorf("ResolveLocale(pref='', Accept-Language=de) = %q, want en", got)
		}
	})

	t.Run("region fallback via Accept-Language", func(t *testing.T) {
		r := req("fr-CA")
		if got := ResolveLocale("", r); got != "fr" {
			t.Errorf("ResolveLocale(pref='', Accept-Language=fr-CA) = %q, want fr", got)
		}
	})

	t.Run("nil request", func(t *testing.T) {
		if got := ResolveLocale("fr", nil); got != "fr" {
			t.Errorf("ResolveLocale(pref=fr, nil) = %q, want fr", got)
		}
		if got := ResolveLocale("", nil); got != SourceLocale {
			t.Errorf("ResolveLocale(pref='', nil) = %q, want %q", got, SourceLocale)
		}
	})
}

func TestIsSupported(t *testing.T) {
	if !IsSupported("en") {
		t.Error("IsSupported(en) = false, want true")
	}
	if !IsSupported("fr") {
		t.Error("IsSupported(fr) = false, want true")
	}
	if !IsSupported("FR") {
		t.Error("IsSupported(FR) = false, want true (case-insensitive)")
	}
	if IsSupported("de") {
		t.Error("IsSupported(de) = true, want false")
	}
}

func TestSupportedLocales(t *testing.T) {
	locs := SupportedLocales()
	if len(locs) < 2 {
		t.Fatalf("SupportedLocales() = %v, want at least en + one target", locs)
	}
	// Source locale is always first.
	if locs[0] != SourceLocale {
		t.Errorf("SupportedLocales()[0] = %q, want %q (source first)", locs[0], SourceLocale)
	}
	// Must contain fr (the launch target locale).
	found := false
	for _, l := range locs {
		if l == "fr" {
			found = true
		}
	}
	if !found {
		t.Errorf("SupportedLocales() = %v, want fr included", locs)
	}
}

func TestLocaleFromContext(t *testing.T) {
	t.Run("unset returns source", func(t *testing.T) {
		ctx := t.Context()
		if got := LocaleFromContext(ctx); got != SourceLocale {
			t.Errorf("LocaleFromContext(unset) = %q, want %q", got, SourceLocale)
		}
	})
	t.Run("set returns value", func(t *testing.T) {
		ctx := WithLocale(t.Context(), "fr-CA")
		if got := LocaleFromContext(ctx); got != "fr" {
			t.Errorf("LocaleFromContext(fr-CA) = %q, want fr (normalized)", got)
		}
	})
}

func TestLocaleMiddleware(t *testing.T) {
	bundle := NewBundle()

	t.Run("uses user pref from context", func(t *testing.T) {
		called := false
		mw := bundle.LocaleMiddleware(func(ctx context.Context) string {
			return "fr"
		})
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			if got := LocaleFromContext(r.Context()); got != "fr" {
				t.Errorf("locale in handler = %q, want fr", got)
			}
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		if !called {
			t.Error("handler not called")
		}
	})

	t.Run("falls back to Accept-Language when pref empty", func(t *testing.T) {
		mw := bundle.LocaleMiddleware(func(ctx context.Context) string { return "" })
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Language", "fr-FR")
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := LocaleFromContext(r.Context()); got != "fr" {
				t.Errorf("locale = %q, want fr (from Accept-Language)", got)
			}
		}))
		handler.ServeHTTP(httptest.NewRecorder(), r)
	})

	t.Run("falls back to en when nothing matches", func(t *testing.T) {
		mw := bundle.LocaleMiddleware(nil)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Language", "de")
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := LocaleFromContext(r.Context()); got != SourceLocale {
				t.Errorf("locale = %q, want en (fallback)", got)
			}
		}))
		handler.ServeHTTP(httptest.NewRecorder(), r)
	})
}
