package middleware

import (
	"net/http"
	"strings"

	"ipmanlk/plume/internal/config"
)

// SecurityHeaders sets baseline browser security headers on all responses.
// They defend in depth against XSS, clickjacking, MIME-sniffing, and TLS
// downgrade attacks. CSP is conservative: scripts and styles only from self,
// images from self/data/blob, WebSocket from self/wss, and no framing.
//
// Because the SPA is served same-origin and assets are fingerprinted, a strict
// CSP does not break the app. The 'unsafe-inline' allowance on styles is needed
// for Lit's scoped style injection (component styles are hashed at build time
// but some dynamic style attributes remain inline).
func SecurityHeaders(appEnv string) func(http.Handler) http.Handler {
	isDev := appEnv == config.EnvDevelopment
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self' data:",
		"connect-src 'self' ws: wss:",
		"media-src 'self' blob:",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			// HSTS only makes sense over HTTPS and in non-dev environments.
			// Dev servers run over plain HTTP (mkcert/local), so HSTS would
			// break local development by pinning the browser to https.
			if !isDev {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			h.Set("Permissions-Policy", "geolocation=(), microphone=(self), camera=(self)")
			next.ServeHTTP(w, r)
		})
	}
}
