// Command vapidkeys generates a VAPID key pair for Web Push.
//
// Usage:
//
//	go run ./cmd/vapidkeys
//
// Print the base64url-encoded public + private keys. Set them as
// VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY in your environment to enable
// browser push notifications.
package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate key: %v\n", err)
		os.Exit(1)
	}
	pub := priv.PublicKey()

	privB64 := base64.RawURLEncoding.EncodeToString(priv.Bytes())
	pubB64 := base64.RawURLEncoding.EncodeToString(pub.Bytes())

	fmt.Println("# Web Push VAPID keys; set these in your environment:")
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", pubB64)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", privB64)
	fmt.Println("# Optional: VAPID_SUBJECT=mailto:you@example.com")
}
