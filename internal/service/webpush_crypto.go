package service

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"hash"
)

// hkdfExtract is a thin wrapper over crypto/hkdf.Extract so callers don't
// have to pass the generic type parameter. Per RFC 5869, secret is the input
// keying material and salt is the (possibly empty) salt.
func hkdfExtract(h func() hash.Hash, secret, salt []byte) ([]byte, error) {
	return hkdf.Extract(h, secret, salt)
}

// hkdfExpand is a thin wrapper over crypto/hkdf.Expand.
func hkdfExpand(h func() hash.Hash, pseudorandomKey []byte, info string, keyLength int) ([]byte, error) {
	return hkdf.Expand(h, pseudorandomKey, info, keyLength)
}

// ecdhToECDSA converts a crypto/ecdh P-256 private key into a crypto/ecdsa
// private key, which is required for signing (ecdh keys can't sign).
func ecdhToECDSA(k *ecdh.PrivateKey) (*ecdsa.PrivateKey, error) {
	pkcs8, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, fmt.Errorf("marshal ecdh key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8: %w", err)
	}
	ecdsaKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected *ecdsa.PrivateKey, got %T", parsed)
	}
	return ecdsaKey, nil
}

// ecdsaSignRaw signs input with ES256 (ECDSA P-256 + SHA-256) and returns
// the raw r||s signature (64 bytes) as required by the JWT JWS spec for
// the "ES256" algorithm (RFC 7518 §3.4).
func ecdsaSignRaw(priv *ecdsa.PrivateKey, input []byte) ([]byte, error) {
	h := sha256.Sum256(input)
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	if err != nil {
		return nil, fmt.Errorf("ecdsa sign: %w", err)
	}
	// r and s are big-endian, fixed-width 32 bytes each (curve order is 256
	// bits). Pad to 32 bytes.
	rb := r.Bytes()
	sb := s.Bytes()
	sig := make([]byte, 64)
	copy(sig[32-len(rb):], rb)
	copy(sig[64-len(sb):], sb)
	return sig, nil
}
