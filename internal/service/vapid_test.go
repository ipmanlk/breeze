package service

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"

	"ipmanlk/plume/internal/config"
)

// generateVAPIDKeyPair generates a P-256 VAPID key pair and returns a config
// with base64url-encoded keys. Used by tests; production keys come from env.
func generateVAPIDKeyPair() (config.VAPIDConfig, error) {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return config.VAPIDConfig{}, err
	}
	return config.VAPIDConfig{
		PublicKey:  base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
		PrivateKey: base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		Subject:    "mailto:test@plume.local",
	}, nil
}
