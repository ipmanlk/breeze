package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtection_SafeMethodsPassThrough(t *testing.T) {
	tests := []struct {
		method string
		name   string
	}{
		{http.MethodGet, "GET"},
		{http.MethodHead, "HEAD"},
		{http.MethodOptions, "OPTIONS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRFProtection(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "http://example.com/api/resource", nil)
			req.Header.Set("Origin", "http://evil.com") // would fail if checked
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			res := w.Result()
			if res.StatusCode != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", tt.name, res.StatusCode)
			}
		})
	}
}

func TestCSRFProtection_SameOriginPOST_Allowed(t *testing.T) {
	handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://app.breeze.local/api/resource", nil)
	req.Header.Set("Origin", "http://app.breeze.local")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for same-origin POST, got %d", res.StatusCode)
	}
}

func TestCSRFProtection_CrossOriginPOST_Rejected(t *testing.T) {
	handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://app.breeze.local/api/resource", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin POST, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if len(body) == 0 {
		t.Error("expected error body")
	}
}

func TestCSRFProtection_MissingOriginOnPOST_Allowed(t *testing.T) {
	handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No Origin, no Referer: programmatic client (curl, test runner).
	req := httptest.NewRequest(http.MethodPost, "http://app.breeze.local/api/resource", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for POST without Origin/Referer (programmatic client), got %d", res.StatusCode)
	}
}

func TestCSRFProtection_NullOriginRejected(t *testing.T) {
	handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// `Origin: null` is sent by sandboxed iframes, data: URIs, and file:
	// origins: not a verifiable same-origin source. Must be rejected.
	req := httptest.NewRequest(http.MethodPost, "http://app.breeze.local/api/resource", nil)
	req.Header.Set("Origin", "null")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for POST with Origin: null, got %d", res.StatusCode)
	}
}

func TestCSRFProtection_OriginMatchesHost_Allowed(t *testing.T) {
	handler := CSRFProtection(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://myserver:8080/api/resource", nil)
	req.Header.Set("Origin", "http://myserver:8080")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 when Origin matches Host, got %d", res.StatusCode)
	}
}

func TestCSRFProtection_RefererFallback_Allowed(t *testing.T) {
	tests := []struct {
		referer string
		host    string
		name    string
	}{
		{"http://app.breeze.local/some/page", "app.breeze.local", "same-origin Referer"},
		{"https://myserver:8080/other", "myserver:8080", "same-origin Referer with port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "http://"+tt.host+"/api/resource", nil)
			req.Header.Set("Referer", tt.referer)
			req.Host = tt.host
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			res := w.Result()
			if res.StatusCode != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", tt.name, res.StatusCode)
			}
		})
	}
}

func TestCSRFProtection_RefererFallback_Rejected(t *testing.T) {
	handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "http://app.breeze.local/api/resource", nil)
	req.Header.Set("Referer", "http://evil.com/trick")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin Referer, got %d", res.StatusCode)
	}
}

func TestCSRFProtection_PUTandPATCHandDELETE_Checked(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			handler := CSRFProtection([]string{"http://app.breeze.local"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			// Cross-origin should be rejected.
			req := httptest.NewRequest(method, "http://app.breeze.local/api/resource", nil)
			req.Header.Set("Origin", "http://evil.com")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			res := w.Result()
			if res.StatusCode != http.StatusForbidden {
				t.Errorf("expected 403 for cross-origin %s, got %d", method, res.StatusCode)
			}

			// Same-origin should be allowed.
			req2 := httptest.NewRequest(method, "http://app.breeze.local/api/resource", nil)
			req2.Header.Set("Origin", "http://app.breeze.local")
			w2 := httptest.NewRecorder()
			handler.ServeHTTP(w2, req2)

			res2 := w2.Result()
			if res2.StatusCode != http.StatusOK {
				t.Errorf("expected 200 for same-origin %s, got %d", method, res2.StatusCode)
			}
		})
	}
}

func TestCSRFProtection_AllowedOriginList(t *testing.T) {
	// CORS origin list includes multiple possible deployment origins.
	handler := CSRFProtection([]string{
		"https://app.breeze.io",
		"https://staging.breeze.io",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST from a production origin should be allowed even though it
	// doesn't match the request Host.
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/resource", nil)
	req.Header.Set("Origin", "https://app.breeze.io")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for allowed origin in list, got %d", res.StatusCode)
	}
}

func TestCSRFProtection_PUT_without_Origin_Allowed(t *testing.T) {
	handler := CSRFProtection(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// PUT without Origin or Referer: programmatic client.
	req := httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/resource", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for PUT without Origin (programmatic client), got %d", res.StatusCode)
	}
}
