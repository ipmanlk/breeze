package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"log/slog"
)

func TestOrganizationHandler_Get_Success(t *testing.T) {
	org := &domain.Organization{ID: "org-1", Name: "My Org", Slug: "my-org", MessageEditWindowMinute: 15}
	svc := &mockOrgService{
		getByIDFn: func(_ context.Context, id string) (*domain.Organization, error) {
			if id != "org-1" {
				t.Errorf("GetByID called with %q, want org-1", id)
			}
			return org, nil
		},
	}
	h := NewOrganizationHandler(svc, slog.Default())

	r := httptest.NewRequest("GET", "/api/organization", nil).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "My Org" {
		t.Errorf("name = %v, want My Org", resp["name"])
	}
	if resp["slug"] != "my-org" {
		t.Errorf("slug = %v, want my-org", resp["slug"])
	}
	// message_edit_window_minutes must be snake_case + numeric.
	if resp["message_edit_window_minutes"] != float64(15) {
		t.Errorf("message_edit_window_minutes = %v, want 15", resp["message_edit_window_minutes"])
	}
}

func TestOrganizationHandler_Get_Unauthenticated(t *testing.T) {
	h := NewOrganizationHandler(&mockOrgService{}, slog.Default())

	r := httptest.NewRequest("GET", "/api/organization", nil)
	w := httptest.NewRecorder()
	h.Get(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationHandler_Update_Success(t *testing.T) {
	updated := &domain.Organization{ID: "org-1", Name: "Renamed", Slug: "renamed", MessageEditWindowMinute: 30}
	var gotName string
	var gotWindow int
	svc := &mockOrgService{
		updateFn: func(_ context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error) {
			gotName = name
			gotWindow = messageEditWindowMinute
			return updated, nil
		},
	}
	h := NewOrganizationHandler(svc, slog.Default())

	body := `{"name":"Renamed","message_edit_window_minutes":30}`
	r := httptest.NewRequest("PATCH", "/api/organization", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	if gotName != "Renamed" {
		t.Errorf("service received name = %q, want Renamed", gotName)
	}
	if gotWindow != 30 {
		t.Errorf("service received window = %d, want 30", gotWindow)
	}
}

func TestOrganizationHandler_Update_InvalidBody(t *testing.T) {
	h := NewOrganizationHandler(&mockOrgService{}, slog.Default())

	// name too short (min=2) → validation error.
	body := `{"name":"x","message_edit_window_minutes":0}`
	r := httptest.NewRequest("PATCH", "/api/organization", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestOrganizationHandler_Delete_Success(t *testing.T) {
	deleted := false
	svc := &mockOrgService{
		deleteFn: func(_ context.Context, orgID, confirmName string) error {
			deleted = true
			if orgID != "org-1" {
				t.Errorf("Delete called with %q, want org-1", orgID)
			}
			if confirmName != "Destroy Me" {
				t.Errorf("Delete called with confirmName %q, want 'Destroy Me'", confirmName)
			}
			return nil
		},
	}
	h := NewOrganizationHandler(svc, slog.Default())

	body := `{"confirm":"Destroy Me"}`
	r := httptest.NewRequest("DELETE", "/api/organization", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	if !deleted {
		t.Error("Delete was not called on the service")
	}
}

func TestOrganizationHandler_Delete_ConfirmMismatch(t *testing.T) {
	svc := &mockOrgService{
		deleteFn: func(_ context.Context, _, confirmName string) error {
			if confirmName != "Destroy Me" {
				return apperr.InvalidInput("type the org name to confirm")
			}
			return nil
		},
	}
	h := NewOrganizationHandler(svc, slog.Default())

	body := `{"confirm":"wrong name"}`
	r := httptest.NewRequest("DELETE", "/api/organization", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Delete(w, r)

	// Handler passes confirmName to service; service returns InvalidInput on mismatch → 400.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestOrganizationHandler_Delete_OrgNotFound(t *testing.T) {
	svc := &mockOrgService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apperr.ErrNotFound
		},
	}
	h := NewOrganizationHandler(svc, slog.Default())

	body := `{"confirm":"anything"}`
	r := httptest.NewRequest("DELETE", "/api/organization", strings.NewReader(body)).WithContext(authCtx())
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestOrganizationHandler_Delete_Unauthenticated(t *testing.T) {
	h := NewOrganizationHandler(&mockOrgService{}, slog.Default())

	r := httptest.NewRequest("DELETE", "/api/organization", strings.NewReader(`{"confirm":"x"}`))
	w := httptest.NewRecorder()
	h.Delete(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// keep context import used even if no direct call elsewhere in this file
var _ = context.Background
