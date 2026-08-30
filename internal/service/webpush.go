// Package service: webpush.go implements RFC 8291 (Message Encryption for
// Web Push) and RFC 8292 (VAPID) so Breeze can send encrypted push payloads to
// browser push subscriptions using only the Go standard library.
package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ipmanlk/breeze/internal/config"
)

// WebPush encrypts payloads and delivers them to push endpoints. It is safe
// for concurrent use. When VAPID keys are not configured, Enabled() returns
// false and Push is a no-op.
type WebPush struct {
	cfg    config.VAPIDConfig
	http   *http.Client
	logger *slog.Logger

	// private key parsed once at construction.
	priv *ecdh.PrivateKey
}

// NewWebPush builds a WebPush sender from VAPID config. If PublicKey or
// PrivateKey is empty the returned sender is disabled.
func NewWebPush(cfg config.VAPIDConfig, logger *slog.Logger) *WebPush {
	w := &WebPush{
		cfg: cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
		logger: logger,
	}
	if cfg.PublicKey == "" || cfg.PrivateKey == "" {
		logger.Info("VAPID not configured; browser push notifications disabled")
		return w
	}
	priv, err := decodeVAPIDPrivateKey(cfg.PrivateKey)
	if err != nil {
		logger.Error("invalid VAPID_PRIVATE_KEY; browser push disabled", "error", err)
		return w
	}
	pub, err := decodeVAPIDPublicKey(cfg.PublicKey)
	if err != nil {
		logger.Error("invalid VAPID_PUBLIC_KEY; browser push disabled", "error", err)
		return w
	}
	// Sanity check: the configured public key must match the private key.
	if !priv.PublicKey().Equal(pub) {
		logger.Error("VAPID public key does not match private key; browser push disabled")
		return w
	}
	w.priv = priv
	logger.Info("VAPID configured; browser push notifications enabled")
	return w
}

// Enabled reports whether VAPID keys are configured and valid.
func (w *WebPush) Enabled() bool {
	return w.priv != nil
}

// PublicKey returns the configured VAPID public key as base64url (no padding),
// suitable for passing to pushManager.subscribe({ applicationServerKey }).
// Returns "" when disabled.
func (w *WebPush) PublicKey() string {
	return w.cfg.PublicKey
}

// Subscription is a Web Push subscription endpoint + the per-subscription
// keys required to encrypt payloads per RFC 8291.
type Subscription struct {
	Endpoint string
	P256dh   string // base64url client public key
	Auth     string // base64url auth secret
}

// PushPayload is the JSON sent to the service worker's push event.data.json().
type PushPayload struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Link  string            `json:"link,omitempty"`
	Tag   string            `json:"tag,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
}

// Push encrypts the payload and POSTs it to the subscription endpoint. Returns
// nil on success or a non-nil error describing why delivery failed (caller
// decides whether to drop the subscription, e.g. on 410 Gone).
func (w *WebPush) Push(ctx context.Context, sub Subscription, payload PushPayload) error {
	if !w.Enabled() {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	encrypted, err := w.encrypt(body, sub)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	authHeader, err := w.vapidAuthHeader(sub.Endpoint)
	if err != nil {
		return fmt.Errorf("vapid: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(encrypted))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("TTL", "2419200") // 28 days max

	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	// 404/410 = subscription no longer valid; caller should delete it.
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		io.Copy(io.Discard, resp.Body)
		return ErrPushSubscriptionGone
	}
	// Read up to 1 KB of the error body for debugging push-service issues.
	// Logging helps diagnose 400-level errors from FCM/APNs without
	// exposing the full body in the error text.
	bodySnippet := ""
	limited := io.LimitReader(resp.Body, 1024)
	if snippet, readErr := io.ReadAll(limited); readErr == nil && len(snippet) > 0 {
		bodySnippet = string(snippet)
	}
	w.logger.Warn("push endpoint error", "status", resp.StatusCode, "body", bodySnippet)
	return fmt.Errorf("push endpoint returned %d", resp.StatusCode)
}

// ErrPushSubscriptionGone signals the subscription is no longer valid and
// should be removed.
var ErrPushSubscriptionGone = errors.New("push subscription no longer valid")

// ── RFC 8291: aes128gcm content encoding ──────────────────────────
//
// The aes128gcm encoding wraps the ciphertext in a binary header so the
// push service doesn't need a separate Content-Encoding header per key:
//
//   +-----------+--------+-----------+---------------+
//   | salt (16) | rs (4) | idlen (1) | keyid (idlen) |
//   +-----------+--------+-----------+---------------+
//   followed by the AEAD-encrypted record.
//
// The keyid is the sender's ephemeral ECDH public key (uncompressed, 65 bytes
// for P-256). The content-encryption key is derived via HKDF from the ECDH
// shared secret + the subscription auth secret.

func (w *WebPush) encrypt(plaintext []byte, sub Subscription) ([]byte, error) {
	// Decode the subscription's client public key + auth secret.
	clientPub, err := decodeClientPublicKey(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("decode p256dh: %w", err)
	}
	authSecret, err := base64URLDecode(sub.Auth)
	if err != nil {
		return nil, fmt.Errorf("decode auth: %w", err)
	}

	// Generate an ephemeral ECDH key pair for this message.
	ephemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ephemeral key: %w", err)
	}
	sharedSecret, err := ephemeral.ECDH(clientPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// Random 16-byte salt.
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}

	// Derive the content encryption key + nonce per RFC 8291 §2.
	ephemeralPubBytes := ephemeral.PublicKey().Bytes() // uncompressed 65 bytes
	clientPubBytes := clientPub.Bytes()

	cek, err := deriveCEK(sharedSecret, authSecret, salt, ephemeralPubBytes, clientPubBytes)
	if err != nil {
		return nil, err
	}
	nonce, err := deriveNonce(sharedSecret, authSecret, salt, ephemeralPubBytes, clientPubBytes)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// The plaintext record is the message padded with a single 0x02 byte
	// followed by 0x00 padding to the record size (4096 max). We use a single
	// record with the full message + 1 delimiter byte.
	const recordSize = 4096
	maxPlaintext := recordSize - 1 // reserve 1 byte for the 0x02 last-record delimiter
	if len(plaintext) > maxPlaintext {
		return nil, fmt.Errorf("webpush payload too large: %d bytes (max %d)", len(plaintext), maxPlaintext)
	}
	padLen := recordSize - len(plaintext) - 1
	record := make([]byte, 0, len(plaintext)+1+padLen)
	record = append(record, plaintext...)
	record = append(record, 0x02) // delimiter for the last record
	record = append(record, make([]byte, padLen)...)

	ciphertext := gcm.Seal(nil, nonce, record, nil)

	// Build the aes128gcm binary header + body.
	keyid := ephemeralPubBytes
	var header bytes.Buffer
	header.Write(salt)
	// 4-byte big-endian record size.
	header.WriteByte(byte(recordSize >> 24))
	header.WriteByte(byte(recordSize >> 16))
	header.WriteByte(byte(recordSize >> 8))
	header.WriteByte(byte(recordSize & 0xff))
	header.WriteByte(byte(len(keyid)))
	header.Write(keyid)
	header.Write(ciphertext)
	return header.Bytes(), nil
}

// deriveCEK derives the content encryption key (16 bytes) per RFC 8291.
func deriveCEK(sharedSecret, authSecret, salt, ephemeralPub, clientPub []byte) ([]byte, error) {
	info := buildHKDFInfo(ephemeralPub, clientPub)
	// IKM = ECDH shared secret, salt = auth secret (RFC 8291 §2.2).
	ikm, err := hkdfExtract(sha256.New, sharedSecret, authSecret)
	if err != nil {
		return nil, err
	}
	return hkdfExpand(sha256.New, ikm, info, 16)
}

// deriveNonce derives the 12-byte nonce per RFC 8291.
func deriveNonce(sharedSecret, authSecret, salt, ephemeralPub, clientPub []byte) ([]byte, error) {
	info := buildHKDFInfo(ephemeralPub, clientPub)
	ikm, err := hkdfExtract(sha256.New, sharedSecret, authSecret)
	if err != nil {
		return nil, err
	}
	return hkdfExpand(sha256.New, ikm, info, 12)
}

// buildHKDFInfo builds the info parameter per RFC 8291 §2.2:
//
//	"WebPush: info" || uak (client pub) || eph (ephemeral sender pub)
//
// where both public keys are the uncompressed SEC 1 encoding.
func buildHKDFInfo(ephemeralPub, clientPub []byte) string {
	var b strings.Builder
	b.WriteString("WebPush: info")
	b.Write(clientPub)
	b.Write(ephemeralPub)
	return b.String()
}

// vapidAuthHeader builds the VAPID JWT (RFC 8292) Authorization header for a
// push request. The header is a self-signed ES256 JWT identifying the
// application server; the push service verifies it to prevent abuse.
func (w *WebPush) vapidAuthHeader(endpoint string) (string, error) {
	origin, err := endpointOrigin(endpoint)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := map[string]any{
		"aud": origin,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": w.cfg.Subject,
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// JWT header: {"typ":"JWT","alg":"ES256"}.
	headerJSON := []byte(`{"typ":"JWT","alg":"ES256"}`)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + payloadB64

	sig, err := w.signES256([]byte(signingInput))
	if err != nil {
		return "", err
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	jwt := signingInput + "." + sigB64
	return "vapid t=" + jwt + ", k=" + w.cfg.PublicKey, nil
}

// signES256 signs the input with the VAPID private key using ECDSA P-256
// SHA-256, producing the raw r||s signature format required by JWT ES256.
func (w *WebPush) signES256(input []byte) ([]byte, error) {
	// crypto/ecdh doesn't expose signing; convert to crypto/ecdsa.
	ecdsaPriv, err := ecdhToECDSA(w.priv)
	if err != nil {
		return nil, err
	}
	return ecdsaSignRaw(ecdsaPriv, input)
}

// ── key encoding helpers ──────────────────────────────────────────

// decodeVAPIDPrivateKey decodes a base64url (no padding) P-256 private key
// (the raw 32-byte scalar) into an ecdh.PrivateKey.
func decodeVAPIDPrivateKey(b64 string) (*ecdh.PrivateKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64url decode: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(raw))
	}
	// ecdh.P256().NewPrivateKey expects the SEC 1 uncompressed point for
	// public keys; for private keys it expects the raw 32-byte scalar in
	// a specific form. We construct via ecdsa then convert.
	curve := ecdh.P256()
	// Build the full scalar (P-256 private key is a 32-byte big-endian int).
	// ecdh NewPrivateKey accepts the raw scalar for NIST curves.
	return curve.NewPrivateKey(raw)
}

// decodeVAPIDPublicKey decodes a base64url (no padding) P-256 public key in
// uncompressed form (0x04 || X || Y, 65 bytes) into an ecdh.PublicKey.
func decodeVAPIDPublicKey(b64 string) (*ecdh.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64url decode: %w", err)
	}
	return ecdh.P256().NewPublicKey(raw)
}

// decodeClientPublicKey decodes the subscription's p256dh key (base64url,
// possibly with padding) into an ecdh.PublicKey.
func decodeClientPublicKey(b64 string) (*ecdh.PublicKey, error) {
	raw, err := base64URLDecode(b64)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPublicKey(raw)
}

// base64URLDecode decodes a base64url string that may or may not have padding.
func base64URLDecode(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

// endpointOrigin extracts https://host from a push endpoint URL.
func endpointOrigin(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid endpoint URL: %s", rawURL)
	}
	return u.Scheme + "://" + u.Host, nil
}

// ── TLS dial helper (unused; kept for future implicit-TLS endpoints) ──
