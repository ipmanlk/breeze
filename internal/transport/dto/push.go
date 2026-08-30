package dto

// PushSubscribeRequest is the body of POST /api/push/subscribe. The fields
// come directly from a PushSubscription object returned by the browser's
// pushManager.subscribe().
type PushSubscribeRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
	P256dh   string `json:"p256dh" validate:"required"`
	Auth     string `json:"auth" validate:"required"`
}

// PushPublicKeyResponse exposes the server's VAPID public key (base64url)
// so the browser can pass it to pushManager.subscribe({ applicationServerKey }).
type PushPublicKeyResponse struct {
	PublicKey string `json:"public_key"`
	// Enabled is false when VAPID is not configured; the UI hides the opt-in.
	Enabled bool `json:"enabled"`
}
