package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport/dto"
)

type mockVoiceService struct {
	listParticipantsFn func(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error)
}

var _ port.VoiceService = (*mockVoiceService)(nil)

func (m *mockVoiceService) Join(ctx context.Context, orgID, userID string, callerRole domain.Role, connID, convID string) (*domain.VoiceJoinResult, error) {
	return nil, nil
}
func (m *mockVoiceService) Leave(ctx context.Context, orgID, userID, convID string) error { return nil }
func (m *mockVoiceService) LeaveByConnection(ctx context.Context, orgID, userID, connID string) error {
	return nil
}
func (m *mockVoiceService) SetMute(ctx context.Context, orgID, userID, convID string, muted bool) error {
	return nil
}
func (m *mockVoiceService) SetDeafen(ctx context.Context, orgID, userID, convID string, deafened bool) error {
	return nil
}
func (m *mockVoiceService) Kick(ctx context.Context, orgID, callerUserID string, callerRole domain.Role, convID, targetUserID string) error {
	return nil
}
func (m *mockVoiceService) ListParticipants(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
	if m.listParticipantsFn != nil {
		return m.listParticipantsFn(ctx, orgID, convID)
	}
	return nil, nil
}
func (m *mockVoiceService) HandleSignal(ctx context.Context, orgID, userID, connID, convID string, msg domain.VoiceSignalMsg) error {
	return nil
}
func (m *mockVoiceService) SetSpeaking(ctx context.Context, userID, orgID, convID string, speaking bool) error {
	return nil
}

func TestVoiceHandler_ListParticipants_Returns200(t *testing.T) {
	svc := &mockVoiceService{
		listParticipantsFn: func(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
			return []domain.VoiceParticipantInfo{
				{ID: "p1", UserID: "u1", Name: "Alice", Muted: false, Deafened: false},
				{ID: "p2", UserID: "u2", Name: "Bob", Muted: true, Deafened: false},
			}, nil
		},
	}
	permSvc := &mockChannelPermissionService{}
	h := NewVoiceHandler(svc, accessSvcFromPerm(permSvc), slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/conversations/conv-1/voice/participants", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	h.ListParticipants(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var participants []*dto.VoiceParticipantResponse
	if err := json.NewDecoder(w.Body).Decode(&participants); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}
	if participants[0].Name != "Alice" {
		t.Errorf("expected first participant 'Alice', got '%s'", participants[0].Name)
	}
	if !participants[1].Muted {
		t.Error("expected second participant to be muted")
	}
}

func TestVoiceHandler_ListParticipants_ServiceError_Returns500(t *testing.T) {
	svc := &mockVoiceService{
		listParticipantsFn: func(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
			return nil, context.DeadlineExceeded
		},
	}
	permSvc := &mockChannelPermissionService{}
	h := NewVoiceHandler(svc, accessSvcFromPerm(permSvc), slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/conversations/conv-1/voice/participants", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	h.ListParticipants(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestVoiceHandler_ListParticipants_NoAuth_Returns400(t *testing.T) {
	svc := &mockVoiceService{}
	permSvc := &mockChannelPermissionService{}
	h := NewVoiceHandler(svc, accessSvcFromPerm(permSvc), slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/conversations/conv-1/voice/participants", nil)
	r = addChiURLParam(r, "id", "conv-1")
	h.ListParticipants(w, r)

	// EnsureConversationAccess returns 400 when auth context is missing
	// because the org context is absent. Previously the handler had its own
	// access check that returned 401 in this case, but the shared guard is
	// more accurate: missing context is a bad request, not an auth failure.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
