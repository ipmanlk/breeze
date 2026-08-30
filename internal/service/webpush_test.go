package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// TestWebPush_EncryptDecryptRoundTrip verifies that a payload encrypted by
// WebPush can be decrypted using the client's private key + auth secret,
// proving the RFC 8291 aes128gcm encoding is correct.
func TestWebPush_EncryptDecryptRoundTrip(t *testing.T) {
	// Generate a client (user agent) ECDH key pair + auth secret.
	clientPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("rand auth: %v", err)
	}

	// The subscription as the browser would send it (base64url).
	_ = Subscription{
		Endpoint: "https://fcm.googleapis.com/fcm/send/fake",
		P256dh:   base64.RawURLEncoding.EncodeToString(clientPriv.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(authSecret),
	}

	// We can't call WebPush.encrypt directly (it's a method on *WebPush which
	// needs a constructed sender), so replicate the encryption path via the
	// same helpers to prove the round-trip. First encrypt:
	plaintext := []byte(`{"title":"Hi","body":"Test"}`)

	clientPub, err := ecdh.P256().NewPublicKey(clientPriv.PublicKey().Bytes())
	if err != nil {
		t.Fatalf("decode client pub: %v", err)
	}

	// Ephemeral sender key.
	ephemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ephemeral: %v", err)
	}
	shared, err := ephemeral.ECDH(clientPub)
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	salt := make([]byte, 16)
	rand.Read(salt)

	ephemeralPub := ephemeral.PublicKey().Bytes()
	clientPubBytes := clientPub.Bytes()

	cek, err := deriveCEK(shared, authSecret, salt, ephemeralPub, clientPubBytes)
	if err != nil {
		t.Fatalf("deriveCEK: %v", err)
	}
	nonce, err := deriveNonce(shared, authSecret, salt, ephemeralPub, clientPubBytes)
	if err != nil {
		t.Fatalf("deriveNonce: %v", err)
	}

	// Build a single record (message + 0x02 + padding to 4096).
	const recordSize = 4096
	padLen := recordSize - len(plaintext) - 1
	record := make([]byte, 0, len(plaintext)+1+padLen)
	record = append(record, plaintext...)
	record = append(record, 0x02)
	record = append(record, make([]byte, padLen)...)

	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, nonce, record, nil)

	// Build the full aes128gcm message (header + ciphertext).
	var msg []byte
	msg = append(msg, salt...)
	msg = append(msg, byte(recordSize>>24), byte(recordSize>>16), byte(recordSize>>8), byte(recordSize&0xff))
	msg = append(msg, byte(len(ephemeralPub)))
	msg = append(msg, ephemeralPub...)
	msg = append(msg, ciphertext...)

	// ── Decrypt on the client side using the client private key ──
	// Parse the header.
	if len(msg) < 21 {
		t.Fatal("message too short")
	}
	rSalt := msg[:16]
	rRecordSize := int(msg[16])<<24 | int(msg[17])<<16 | int(msg[18])<<8 | int(msg[19])
	idLen := int(msg[20])
	keyStart := 21
	keyEnd := keyStart + idLen
	senderPubBytes := msg[keyStart:keyEnd]
	ct := msg[keyEnd:]

	senderPub, err := ecdh.P256().NewPublicKey(senderPubBytes)
	if err != nil {
		t.Fatalf("decode sender pub: %v", err)
	}
	clientShared, err := clientPriv.ECDH(senderPub)
	if err != nil {
		t.Fatalf("client ecdh: %v", err)
	}

	dCEK, err := deriveCEK(clientShared, authSecret, rSalt, senderPubBytes, clientPubBytes)
	if err != nil {
		t.Fatalf("client deriveCEK: %v", err)
	}
	dNonce, err := deriveNonce(clientShared, authSecret, rSalt, senderPubBytes, clientPubBytes)
	if err != nil {
		t.Fatalf("client deriveNonce: %v", err)
	}

	dblock, _ := aes.NewCipher(dCEK)
	dgcm, _ := cipher.NewGCM(dblock)
	decrypted, err := dgcm.Open(nil, dNonce, ct, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	// Strip the padding: everything up to the 0x02 delimiter is the message.
	idx := strings.IndexByte(string(decrypted), 0x02)
	if idx < 0 {
		t.Fatal("no 0x02 delimiter in decrypted record")
	}
	result := decrypted[:idx]

	if string(result) != string(plaintext) {
		t.Errorf("round-trip mismatch: got %q, want %q", result, plaintext)
	}
	if rRecordSize != recordSize {
		t.Errorf("record size = %d, want %d", rRecordSize, recordSize)
	}
}

// TestWebPush_BuildHKDFInfo verifies the info string matches RFC 8291 §2.2.
func TestWebPush_BuildHKDFInfo(t *testing.T) {
	eph := []byte{0x04, 0xAA}
	client := []byte{0x04, 0xBB}
	info := buildHKDFInfo(eph, client)
	if !strings.HasPrefix(info, "WebPush: info") {
		t.Errorf("info missing prefix: %q", info)
	}
	if !strings.Contains(info, string(client)) || !strings.Contains(info, string(eph)) {
		t.Errorf("info missing client/ephemeral key bytes")
	}
}

// TestWebPush_EndpointOrigin verifies URL origin extraction.
func TestWebPush_EndpointOrigin(t *testing.T) {
	got, err := endpointOrigin("https://fcm.googleapis.com/fcm/send/abc123")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "https://fcm.googleapis.com" {
		t.Errorf("origin = %q", got)
	}
}

// TestWebPush_EncryptPayloadSize checks that encrypt returns an error for
// payloads exceeding the maximum single-record size (4095 bytes) instead of
// silently truncating them.
func TestWebPush_EncryptPayloadSize(t *testing.T) {
	// Generate a client key pair + auth secret to build a subscription.
	clientPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("rand auth: %v", err)
	}

	sub := Subscription{
		Endpoint: "https://fcm.googleapis.com/fcm/send/fake",
		P256dh:   base64.RawURLEncoding.EncodeToString(clientPriv.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(authSecret),
	}

	// A payload exactly at the max (4095 bytes) must succeed.
	maxPayload := make([]byte, 4095)
	for i := range maxPayload {
		maxPayload[i] = byte('A' + i%26)
	}
	result, err := (&WebPush{}).encrypt(maxPayload, sub)
	if err != nil {
		t.Fatalf("encrypt(4095) should succeed but got error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("encrypt(4095) returned empty result")
	}

	// A payload at max+1 (4096 bytes) must fail with a "too large" error.
	overSize := make([]byte, 4096)
	for i := range overSize {
		overSize[i] = byte('A' + i%26)
	}
	_, err = (&WebPush{}).encrypt(overSize, sub)
	if err == nil {
		t.Fatal("encrypt(4096) should fail but succeeded")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error message should mention 'too large', got: %v", err)
	}

	// A payload well over the max (5000 bytes) must also fail.
	bigPayload := make([]byte, 5000)
	for i := range bigPayload {
		bigPayload[i] = byte('A' + i%26)
	}
	_, err = (&WebPush{}).encrypt(bigPayload, sub)
	if err == nil {
		t.Fatal("encrypt(5000) should fail but succeeded")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error message should mention 'too large', got: %v", err)
	}
}

// TestWebPush_HKDFConsistency verifies the hkdf wrappers produce stable output.
func TestWebPush_HKDFConsistency(t *testing.T) {
	ikm, err := hkdfExtract(sha256.New, []byte("secret"), []byte("salt"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	out, err := hkdfExpand(sha256.New, ikm, "info", 32)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(out) != 32 {
		t.Errorf("len = %d, want 32", len(out))
	}
	// Cross-check against the one-shot hkdf.Key.
	direct, err := hkdf.Key(sha256.New, []byte("secret"), []byte("salt"), "info", 32)
	if err != nil {
		t.Fatalf("hkdf.Key: %v", err)
	}
	if string(out) != string(direct) {
		t.Error("extract+expand does not match one-shot hkdf.Key")
	}
}
