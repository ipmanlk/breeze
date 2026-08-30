package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (f *fakePinger) PingContext(_ context.Context) error {
	return f.err
}

func TestHealthHandler_Check_Healthy(t *testing.T) {
	h := NewHealthHandler(&fakePinger{err: nil}, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Check(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf(`status = %q, want "ok"`, resp["status"])
	}
	if resp["db"] != "ok" {
		t.Errorf(`db = %q, want "ok"`, resp["db"])
	}
}

func TestHealthHandler_Check_DBDown(t *testing.T) {
	h := NewHealthHandler(&fakePinger{err: errors.New("connection refused")}, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Check(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "degraded" {
		t.Errorf(`status = %q, want "degraded"`, resp["status"])
	}
	if resp["db"] != "down" {
		t.Errorf(`db = %q, want "down"`, resp["db"])
	}
}
