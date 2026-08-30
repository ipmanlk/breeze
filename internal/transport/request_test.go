package transport

import (
	"net"
	"net/http"
	"testing"
)

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse cidr %q: %v", cidr, err)
	}
	return network
}

func TestClientIPWithoutTrustedProxiesIgnoresHeaders(t *testing.T) {
	TrustedProxyCIDRs = nil
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("203.0.113.7:1234", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	if got := ClientIP(req); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want direct peer %q", got, "203.0.113.7")
	}
}

func TestClientIPSpoofedPrefixIgnoredBehindTrustedProxy(t *testing.T) {
	// Attacker sends X-Forwarded-For: 8.8.8.8 through the trusted proxy; the
	// proxy appends the real client IP. The rightmost untrusted hop must win.
	TrustedProxyCIDRs = []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("10.0.0.2:443", map[string]string{
		"X-Forwarded-For": "8.8.8.8, 198.51.100.23",
	})
	if got := ClientIP(req); got != "198.51.100.23" {
		t.Fatalf("ClientIP() = %q, want 198.51.100.23", got)
	}
}

func TestClientIPWalksPastChainedTrustedProxies(t *testing.T) {
	TrustedProxyCIDRs = []*net.IPNet{mustCIDR(t, "10.0.0.0/8"), mustCIDR(t, "192.168.0.0/16")}
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("10.0.0.2:443", map[string]string{
		"X-Forwarded-For": "203.0.113.9, 192.168.5.5",
	})
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q, want 203.0.113.9", got)
	}
}

func TestClientIPAllTrustedChainFallsBackToPeer(t *testing.T) {
	TrustedProxyCIDRs = []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("10.0.0.2:443", map[string]string{
		"X-Forwarded-For": "10.0.0.3, 10.0.0.4",
	})
	if got := ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP() = %q, want peer 10.0.0.2", got)
	}
}

func TestClientIPMalformedHopFallsBackToPeer(t *testing.T) {
	TrustedProxyCIDRs = []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("10.0.0.2:443", map[string]string{
		"X-Forwarded-For": "1.2.3.4, not-an-ip",
	})
	if got := ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP() = %q, want peer for malformed chain", got)
	}
}

func TestClientIPRealIPFallbackWithoutForwardedFor(t *testing.T) {
	TrustedProxyCIDRs = []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("10.0.0.2:443", map[string]string{"X-Real-IP": "198.51.100.77"})
	if got := ClientIP(req); got != "198.51.100.77" {
		t.Fatalf("ClientIP() = %q, want 198.51.100.77", got)
	}

	// A non-IP value in X-Real-IP must not be trusted as a rate-limit key.
	req = httptestRequest("10.0.0.2:443", map[string]string{"X-Real-IP": "garbage; drop table"})
	if got := ClientIP(req); got != "10.0.0.2" {
		t.Fatalf("ClientIP() = %q, want peer for malformed X-Real-IP", got)
	}
}

func TestClientIPv6Peer(t *testing.T) {
	TrustedProxyCIDRs = nil
	defer func() { TrustedProxyCIDRs = nil }()

	req := httptestRequest("[2001:db8::1]:443", nil)
	if got := ClientIP(req); got != "2001:db8::1" {
		t.Fatalf("ClientIP() = %q, want 2001:db8::1", got)
	}
}

func httptestRequest(remoteAddr string, headers map[string]string) *http.Request {
	req := &http.Request{RemoteAddr: remoteAddr, Header: http.Header{}}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}
