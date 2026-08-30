package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/transport/middleware"
)

type mockAttachmentService struct {
	created *domain.Attachment
	// download fields let a test drive the Download return values
	dlReader io.ReadCloser
	dlCT     string
	dlProj   string
	dlName   string
}

func (m *mockAttachmentService) List(ctx context.Context, orgID, taskID, projectID string) ([]*domain.Attachment, error) {
	return nil, nil
}
func (m *mockAttachmentService) Create(ctx context.Context, params domain.CreateAttachmentParams) (*domain.Attachment, error) {
	return m.created, nil
}
func (m *mockAttachmentService) Delete(ctx context.Context, userID, orgID, id, taskID, projectID string) error {
	return nil
}
func (m *mockAttachmentService) Get(ctx context.Context, id string) (*domain.Attachment, error) {
	return nil, nil
}
func (m *mockAttachmentService) Download(ctx context.Context, orgID, id string) (io.ReadCloser, string, string, string, error) {
	return m.dlReader, m.dlCT, m.dlProj, m.dlName, nil
}

func TestAttachmentHandler_Upload_RejectsOversizedFile(t *testing.T) {
	svc := &mockAttachmentService{created: &domain.Attachment{ID: "att-1"}}
	h := NewAttachmentHandler(svc, &mockAccessService{}, avatarTestLogger)

	r := chi.NewRouter()
	// The production route wraps uploads in LimitRequestBody(MAX_UPLOAD_SIZE).
	// Replicate that here so the handler is tested with the same enforcement.
	r.With(middleware.LimitRequestBody(50<<20)).
		Post("/api/projects/{id}/tasks/{taskId}/attachments", h.Upload)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("file", "big.bin")
	fw.Write(make([]byte, 51<<20)) // 51 MB
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-1/tasks/task-1/attachments", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := req.Context()
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxEffectiveRole, domain.RoleMember)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestAttachmentHandler_Download_ForcesAttachmentHeaders verifies that task
// attachment downloads are always served as a download (Content-Disposition:
// attachment) with nosniff, preventing stored XSS from user-uploaded HTML.
func TestAttachmentHandler_Download_ForcesAttachmentHeaders(t *testing.T) {
	svc := &mockAttachmentService{
		dlReader: io.NopCloser(bytes.NewReader([]byte("<script>alert(1)</script>"))),
		dlCT:     "text/html",
		dlProj:   "proj-1",
		dlName:   "report.html",
	}
	h := NewAttachmentHandler(svc, &mockAccessService{}, avatarTestLogger)

	r := chi.NewRouter()
	r.Get("/api/attachments/{attachmentId}/download", h.Download)

	req := httptest.NewRequest(http.MethodGet, "/api/attachments/att-1/download", nil)
	ctx := context.WithValue(req.Context(), domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxEffectiveRole, domain.RoleMember)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Must force a download: never inline on the origin.
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Error("missing Content-Disposition header (stored XSS risk)")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// Scriptable type must be downgraded so a misconfigured browser cannot
	// render it as HTML.
	if got := rec.Header().Get("Content-Type"); got == "text/html" {
		t.Error("scriptable content type served inline; expected downgrade to octet-stream")
	}
}

// TestAttachmentHandler_Upload_RejectsBlockedType verifies that uploading a
// scriptable file type (text/html) is rejected at upload time.
func TestAttachmentHandler_Upload_RejectsBlockedType(t *testing.T) {
	svc := &mockAttachmentService{created: &domain.Attachment{ID: "att-1"}}
	h := NewAttachmentHandler(svc, &mockAccessService{}, avatarTestLogger)

	r := chi.NewRouter()
	r.Post("/api/projects/{id}/tasks/{taskId}/attachments", h.Upload)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{`form-data; name="file"; filename="evil.html"`}
	hdr["Content-Type"] = []string{"text/html"}
	fw, _ := w.CreatePart(hdr)
	fw.Write([]byte("<script>alert(1)</script>"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-1/tasks/task-1/attachments", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := context.WithValue(req.Context(), domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxEffectiveRole, domain.RoleMember)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (blocked file type)", rec.Code)
	}
}
