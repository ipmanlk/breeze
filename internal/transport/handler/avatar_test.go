package handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/storage"

	"github.com/go-chi/chi/v5"
)

var avatarTestLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockUserRepoWithAvatar struct {
	port.UserRepository
	runInTx func(fn func(port.UserRepository) error) error
	user    *domain.User
}

func (m *mockUserRepoWithAvatar) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	return m.user, nil
}

func TestAvatarHandler_Get_StreamsAvatar(t *testing.T) {
	tmp := t.TempDir()
	store := storage.NewLocal(tmp)
	avatarPath := "avatars/acct-1/avatar.png"
	if err := store.Save(context.Background(), avatarPath, bytes.NewReader([]byte("fake-image"))); err != nil {
		t.Fatalf("save avatar: %v", err)
	}

	avatarURL := "/uploads/" + avatarPath
	userRepo := &mockUserRepoWithAvatar{user: &domain.User{
		ID:        "user-1",
		OrgID:     "org-1",
		AccountID: "acct-1",
		AvatarURL: &avatarURL,
	}}
	h := NewAvatarHandler(userRepo, store, avatarTestLogger)

	r := chi.NewRouter()
	r.Get("/api/avatars/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/avatars/user-1", nil)
	ctx := context.WithValue(req.Context(), domain.CtxOrgID, "org-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "fake-image" {
		t.Errorf("body = %q, want %q", got, "fake-image")
	}
}

func TestAvatarHandler_Get_NoAvatarReturns404(t *testing.T) {
	userRepo := &mockUserRepoWithAvatar{user: &domain.User{
		ID:        "user-1",
		OrgID:     "org-1",
		AccountID: "acct-1",
		AvatarURL: nil,
	}}
	h := NewAvatarHandler(userRepo, storage.NewLocal(t.TempDir()), avatarTestLogger)
	r := chi.NewRouter()
	r.Get("/api/avatars/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/avatars/user-1", nil)
	ctx := context.WithValue(req.Context(), domain.CtxOrgID, "org-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func (m *mockUserRepoWithAvatar) RunInTransaction(ctx context.Context, fn func(port.UserRepository) error) error {
	if m.runInTx != nil {
		return m.runInTx(fn)
	}
	return fn(m)
}
