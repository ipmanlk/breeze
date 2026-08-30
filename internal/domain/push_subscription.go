package domain

import "time"

// PushPayload is the JSON sent to the service worker's push event.data.json().
type PushPayload struct {
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Link  string            `json:"link,omitempty"`
	Tag   string            `json:"tag,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
}

// PushSubscription is a Web Push API subscription endpoint registered by a
// browser. The server stores the per-subscription keys needed to encrypt
// payloads per RFC 8291 and POSTs notifications to the endpoint.
type PushSubscription struct {
	ID       string
	UserID   string
	OrgID    string
	Endpoint string
	// P256dh is the client ECDH public key (base64url).
	P256dh string
	// Auth is the client auth secret (base64url).
	Auth      string
	CreatedAt time.Time
}
