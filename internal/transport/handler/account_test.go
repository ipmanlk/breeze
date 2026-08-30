package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func TestAccountHandler_UpdateProfile_Success(t *testing.T) {
	user := &domain.User{ID: "user-1", OrgID: "org-1", Name: "New Name", Email: "a@x.com"}
	svc := &mockUserService{
		getByIDFn: func(_ context.Context, _, _ string) (*domain.User, error) { return user, nil },
		updateProfileFn: func(_ context.Context, _, _, _ string, _ *string) (*domain.User, error) {
			return user, nil
		},
	}
	h := NewAccountHandler(svc, slog.Default())

	r := httptest.NewRequest("PATCH", "/api/account", strings.NewReader(`{"name":"New Name"}`)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.UpdateProfile(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "New Name" {
		t.Errorf("name = %v, want New Name", resp["name"])
	}
}

func TestAccountHandler_UpdateProfile_InvalidBody(t *testing.T) {
	svc := &mockUserService{
		getByIDFn: func(_ context.Context, _, _ string) (*domain.User, error) { return &domain.User{}, nil },
	}
	h := NewAccountHandler(svc, slog.Default())

	// Missing required "name".
	r := httptest.NewRequest("PATCH", "/api/account", strings.NewReader(`{}`)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.UpdateProfile(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAccountHandler_UpdateProfile_Unauthenticated(t *testing.T) {
	h := NewAccountHandler(&mockUserService{}, slog.Default())

	r := httptest.NewRequest("PATCH", "/api/account", strings.NewReader(`{"name":"x"}`))
	w := httptest.NewRecorder()
	h.UpdateProfile(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAccountHandler_ChangePassword_Success(t *testing.T) {
	called := false
	svc := &mockUserService{
		changePasswordFn: func(_ context.Context, _, _, _, _ string) error {
			called = true
			return nil
		},
	}
	h := NewAccountHandler(svc, slog.Default())

	body := `{"current_password":"old","new_password":"newpass123"}`
	r := httptest.NewRequest("POST", "/api/account/change-password", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	if !called {
		t.Error("ChangePassword was not called on the service")
	}
}

func TestAccountHandler_ChangePassword_WrongCurrent(t *testing.T) {
	svc := &mockUserService{
		changePasswordFn: func(_ context.Context, _, _, _, _ string) error {
			return apperr.ErrInvalidCreds
		},
	}
	h := NewAccountHandler(svc, slog.Default())

	body := `{"current_password":"wrong","new_password":"newpass123"}`
	r := httptest.NewRequest("POST", "/api/account/change-password", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (wrong current password → 401)", w.Code, http.StatusUnauthorized)
	}
}

func TestAccountHandler_ChangePassword_ShortNew(t *testing.T) {
	h := NewAccountHandler(&mockUserService{}, slog.Default())

	// New password shorter than min 8 → validation error.
	body := `{"current_password":"old","new_password":"short"}`
	r := httptest.NewRequest("POST", "/api/account/change-password", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (short new password → 400)", w.Code, http.StatusBadRequest)
	}
}

func TestAccountHandler_ChangePassword_Unauthenticated(t *testing.T) {
	h := NewAccountHandler(&mockUserService{}, slog.Default())

	r := httptest.NewRequest("POST", "/api/account/change-password", strings.NewReader(`{"current_password":"a","new_password":"newpass123"}`))
	w := httptest.NewRecorder()
	h.ChangePassword(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
