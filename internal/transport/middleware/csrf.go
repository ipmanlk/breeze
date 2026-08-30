package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// CSRFProtection validates the Origin (or Referer as fallback) header on
// state-changing HTTP methods to provide defense-in-depth CSRF protection
// beyond SameSite=Lax on the auth cookie.
//
// Safe methods (GET, HEAD, OPTIONS) pass through without checking.
//
// For unsafe methods (POST, PUT, PATCH, DELETE), the middleware reads the
// Origin header (preferred) and validates it against an allowlist built from:
//   - All entries from the CORSOrigins config (the SPA's origins).
//   - The request's own Host header (http://<host> and https://<host>) for
//     same-origin requests when the SPA and API share a domain.
//
// If Origin is absent (e.g. programmatic HTTP clients, curl, tests), the
// Referer header is checked as a fallback. If Referer is also absent, the
// request is allowed through; browsers always send Origin on cross-site
// POST requests, so its absence implies a non-browser client.
//
// Browser behaviour relied upon:
//   - All modern browsers send Origin on cross-origin POST/PUT/PATCH/DELETE
//     requests (Fetch spec).
//   - Same-origin POST requests from JavaScript (fetch, XHR) also include
//     Origin. Same-origin form submissions may omit Origin but include Referer.
//   - A cross-origin attacker cannot forge the Origin or Referer headers from
//     a browser-based attack (same-origin policy).
func CSRFProtection(allowedOrigins []string) func(http.Handler) http.Handler {
	baseAllowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o != "" {
			baseAllowed[strings.TrimRight(o, "/")] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if originAllowed(r, baseAllowed) {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, `{"error":{"code":"forbidden","message":"CSRF check failed: cross-origin request rejected"}}`, http.StatusForbidden)
		})
	}
}

// originAllowed reports whether the request's Origin (or Referer as fallback)
// matches an allowed origin.
func originAllowed(r *http.Request, baseAllowed map[string]bool) bool {
	source := r.Header.Get("Origin")
	fromReferer := false
	if source == "" {
		source = r.Header.Get("Referer")
		fromReferer = true
	}
	if source == "" {
		// No Origin and no Referer; likely a non-browser client. Allow
		// through; browsers always send Origin on cross-site requests.
		return true
	}
	if source == "null" {
		// Browsers send `Origin: null` for sandboxed iframes, data: URIs, and
		// file: origins. These are not trusted same-origin sources and cannot
		// be verified; reject.
		return false
	}

	srcURL, err := url.Parse(source)
	if err != nil || srcURL.Host == "" {
		// With a Referer that doesn't parse, be strict: reject.
		if fromReferer {
			return false
		}
		// For an unparseable Origin, allow defensively.
		return true
	}

	// Build the check set: base allowed origins + request Host (both schemes).
	allowed := make(map[string]bool, len(baseAllowed)+2)
	for k := range baseAllowed {
		allowed[k] = true
	}
	if reqHost := r.Host; reqHost != "" {
		allowed["http://"+reqHost] = true
		allowed["https://"+reqHost] = true
	}

	// For Origin headers, the value is just scheme://host, so we can check
	// the raw value. For Referer, the value is a full URL with path, so we
	// must compare only the scheme+host part.
	if fromReferer {
		refOrigin := srcURL.Scheme + "://" + srcURL.Host
		return allowed[strings.TrimRight(refOrigin, "/")]
	}

	srcNorm := strings.TrimRight(source, "/")
	if allowed[srcNorm] {
		return true
	}

	// Also check the bare host; some clients send Origin as just a hostname.
	return allowed[srcURL.Host]
}
