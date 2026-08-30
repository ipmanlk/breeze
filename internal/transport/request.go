package transport

import (
	"net"
	"net/http"
	"strings"
)

// TrustedProxyCIDRs controls which proxy IPs are allowed to supply
// X-Forwarded-For / X-Real-IP headers. When empty (the default), proxy
// headers are ignored and only r.RemoteAddr is used. Set this to the CIDR
// of your reverse proxy (e.g. "10.0.0.0/8", "172.16.0.0/12") if you deploy
// behind a trusted proxy like nginx or a load balancer.
var TrustedProxyCIDRs []*net.IPNet

func isTrustedProxy(ipStr string) bool {
	if len(TrustedProxyCIDRs) == 0 {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, cidr := range TrustedProxyCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIP extracts the caller's IP from a request.
//
// By default (no trusted proxies configured) it returns r.RemoteAddr and
// never consults proxy headers, so a client cannot influence the result.
//
// When TrustedProxyCIDRs is configured and the direct peer matches one of
// those CIDRs, the X-Forwarded-For chain is walked right-to-left: hops that
// are themselves trusted proxies are skipped, and the first non-trusted
// address is returned. This means a client cannot place a spoofed address
// at the front of the chain to control the result; only addresses appended
// by trusted proxies are considered authoritative. X-Real-IP is honored as a
// fallback for proxies that don't set X-Forwarded-For.
//
// The result is used to record session origin metadata and to key rate-limit
// buckets.
func ClientIP(r *http.Request) string {
	host := remoteHost(r)
	if !isTrustedProxy(host) {
		return host
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" && net.ParseIP(realIP) != nil {
			return realIP
		}
		return host
	}

	hops := strings.Split(xff, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(hops[i])
		// A malformed or empty hop means the chain can't be trusted past
		// this point; fall back to the address we actually see.
		if candidate == "" || net.ParseIP(candidate) == nil {
			return host
		}
		if !isTrustedProxy(candidate) {
			return candidate
		}
	}
	// Every hop is a trusted proxy (e.g. chained internal LBs); the client
	// is directly connected infrastructure we already trust, so use the peer.
	return host
}

// UserAgent returns the request's User-Agent header (empty if absent).
func UserAgent(r *http.Request) string {
	return r.Header.Get("User-Agent")
}
