package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type mockMessageSVC struct {
	listMessagesFn   func(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	listRepliesFn    func(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	searchMessagesFn func(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error)
	sendMessageFn    func(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error)
	editMessageFn    func(ctx context.Context, params domain.EditMessageParams) (*domain.Message, error)
	deleteMessageFn  func(ctx context.Context, orgID, msgID, convID, deleterID string) error
	pinMessageFn     func(ctx context.Context, orgID, msgID, convID, pinnerID string) error
	unpinMessageFn   func(ctx context.Context, orgID, msgID, convID string) error
	addReactionFn    func(ctx context.Context, params domain.AddReactionParams) error
	removeReactionFn func(ctx context.Context, params domain.RemoveReactionParams) error
}

func (m *mockMessageSVC) ListMessages(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	return m.listMessagesFn(ctx, orgID, convID, filter)
}
func (m *mockMessageSVC) ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	return m.listRepliesFn(ctx, orgID, convID, parentID, filter)
}
func (m *mockMessageSVC) SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error) {
	return m.searchMessagesFn(ctx, orgID, userID, filter)
}
func (m *mockMessageSVC) SendMessage(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error) {
	return m.sendMessageFn(ctx, params)
}
func (m *mockMessageSVC) EditMessage(ctx context.Context, params domain.EditMessageParams) (*domain.Message, error) {
	return m.editMessageFn(ctx, params)
}
func (m *mockMessageSVC) DeleteMessage(ctx context.Context, orgID, msgID, convID, deleterID string) error {
	return m.deleteMessageFn(ctx, orgID, msgID, convID, deleterID)
}
func (m *mockMessageSVC) PinMessage(ctx context.Context, orgID, msgID, convID, pinnerID string) error {
	return m.pinMessageFn(ctx, orgID, msgID, convID, pinnerID)
}
func (m *mockMessageSVC) UnpinMessage(ctx context.Context, orgID, msgID, convID string) error {
	return m.unpinMessageFn(ctx, orgID, msgID, convID)
}
func (m *mockMessageSVC) AddReaction(ctx context.Context, params domain.AddReactionParams) error {
	return m.addReactionFn(ctx, params)
}
func (m *mockMessageSVC) RemoveReaction(ctx context.Context, params domain.RemoveReactionParams) error {
	return m.removeReactionFn(ctx, params)
}

var _ port.MessageService = (*mockMessageSVC)(nil)

type mockMentionSVC struct {
	searchFn func(ctx context.Context, orgID, userID, userRole, query string, types []domain.MentionType, limit int) ([]*domain.MentionResult, error)
}

func (m *mockMentionSVC) Search(ctx context.Context, orgID, userID, userRole, query string, types []domain.MentionType, limit int) ([]*domain.MentionResult, error) {
	return m.searchFn(ctx, orgID, userID, userRole, query, types, limit)
}

var _ port.MentionService = (*mockMentionSVC)(nil)

type mockConvRepo struct {
	getByIDFn  func(ctx context.Context, orgID, id string) (*domain.Conversation, error)
	isMemberFn func(ctx context.Context, convID, userID string) (bool, error)
}

func (m *mockConvRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Conversation, error) {
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockConvRepo) IsMember(ctx context.Context, convID, userID string) (bool, error) {
	return m.isMemberFn(ctx, convID, userID)
}
func (m *mockConvRepo) ListByUser(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error) {
	return nil, nil
}
func (m *mockConvRepo) ListByParent(ctx context.Context, orgID, parentID, userID string, includeProjectLinked bool) ([]*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvRepo) GetByIDWithMember(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvRepo) GetDMByUsers(ctx context.Context, orgID, requesterID, recipientID string) (*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvRepo) Create(ctx context.Context, conv *domain.Conversation) error { return nil }
func (m *mockConvRepo) Update(ctx context.Context, conv *domain.Conversation) error { return nil }
func (m *mockConvRepo) UpdateParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error {
	return nil
}
func (m *mockConvRepo) UpdatePositionKey(ctx context.Context, orgID, id string, positionKey string) error {
	return nil
}
func (m *mockConvRepo) ListCategories(ctx context.Context, orgID string) ([]*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvRepo) ListSiblingPositionKeys(ctx context.Context, orgID string, parentID *string) ([]string, error) {
	return nil, nil
}
func (m *mockConvRepo) Delete(ctx context.Context, orgID, id string) error { return nil }
func (m *mockConvRepo) SoftDeleteByParent(ctx context.Context, orgID, parentID string) error {
	return nil
}
func (m *mockConvRepo) AddMember(ctx context.Context, orgID, convID, userID string) error { return nil }
func (m *mockConvRepo) RemoveMember(ctx context.Context, convID, userID string) error     { return nil }
func (m *mockConvRepo) GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error) {
	return nil, nil
}
func (m *mockConvRepo) UpdateReadState(ctx context.Context, convID, userID string) error { return nil }
func (m *mockConvRepo) UnreadCount(ctx context.Context, convID, userID string) (int, error) {
	return 0, nil
}
func (m *mockConvRepo) UnreadCounts(ctx context.Context, userID string, convIDs []string) (map[string]int, error) {
	return map[string]int{}, nil
}
func (m *mockConvRepo) GetLastMessage(ctx context.Context, convID string) (*domain.Message, error) {
	return nil, nil
}
func (m *mockConvRepo) ListPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *mockConvRepo) CountMembers(ctx context.Context, convID string) (int, error) { return 0, nil }
func (m *mockConvRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Conversation, error) {
	return nil, nil
}

func (m *mockConvRepo) CreateWithMembers(_ context.Context, _ *domain.Conversation, _ []string) error {
	return nil
}

var _ port.ConversationRepository = (*mockConvRepo)(nil)

type mockMsgAttRepo struct {
	getByIDAndConversationFn func(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error)
}

func (m *mockMsgAttRepo) Create(ctx context.Context, att *domain.MessageAttachment) error { return nil }
func (m *mockMsgAttRepo) ListByMessage(ctx context.Context, messageID string) ([]*domain.MessageAttachment, error) {
	return nil, nil
}
func (m *mockMsgAttRepo) ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*domain.MessageAttachment, error) {
	return nil, nil
}
func (m *mockMsgAttRepo) GetByID(ctx context.Context, id string) (*domain.MessageAttachment, error) {
	return nil, nil
}
func (m *mockMsgAttRepo) GetByIDAndConversation(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error) {
	if m.getByIDAndConversationFn != nil {
		return m.getByIDAndConversationFn(ctx, id, conversationID)
	}
	return nil, nil
}
func (m *mockMsgAttRepo) Delete(ctx context.Context, id string) error                     { return nil }
func (m *mockMsgAttRepo) UpdateMessageID(ctx context.Context, id, messageID string) error { return nil }

type mockPendingAttRepo struct{}

func (m *mockPendingAttRepo) Create(ctx context.Context, att *domain.PendingAttachment) error {
	return nil
}
func (m *mockPendingAttRepo) GetByID(ctx context.Context, id, uploadedBy string) (*domain.PendingAttachment, error) {
	return nil, nil
}
func (m *mockPendingAttRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockPendingAttRepo) DeleteOlderThan(ctx context.Context, before time.Time) ([]*domain.PendingAttachment, error) {
	return nil, nil
}

type mockReactionRepoHandler struct{}

func (m *mockReactionRepoHandler) Add(ctx context.Context, orgID, messageID, userID, emoji string) error {
	return nil
}
func (m *mockReactionRepoHandler) Remove(ctx context.Context, messageID, userID, emoji string) (bool, error) {
	return true, nil
}
func (m *mockReactionRepoHandler) ListForMessages(ctx context.Context, messageIDs []string) ([]*domain.Reaction, error) {
	return nil, nil
}

type mockStore struct {
	getFn func(ctx context.Context, path string) (io.ReadCloser, error)
}

func (m *mockStore) Save(ctx context.Context, path string, reader io.Reader) error { return nil }
func (m *mockStore) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if m.getFn != nil {
		return m.getFn(ctx, path)
	}
	return nil, nil
}
func (m *mockStore) Delete(ctx context.Context, path string) error        { return nil }
func (m *mockStore) URL(ctx context.Context, path string) (string, error) { return "", nil }

func newMockConvRepo() *mockConvRepo {
	return &mockConvRepo{
		getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Conversation, error) {
			return &domain.Conversation{ID: id, OrgID: orgID, Name: "general", Type: domain.ConvChannel}, nil
		},
		isMemberFn: func(ctx context.Context, convID, userID string) (bool, error) {
			return true, nil
		},
	}
}

func newMessageHandler(svc port.MessageService) *MessageHandler {
	return NewMessageHandler(MessageHandlerDeps{
		SVC:            svc,
		MentionSvc:     &mockMentionSVC{},
		AttRepo:        &mockMsgAttRepo{},
		PendingAttRepo: &mockPendingAttRepo{},
		ReactionRepo:   &mockReactionRepoHandler{},
		AccessSvc:      &mockAccessService{},
		StoreBack:      &mockStore{},
		Log:            slog.Default(),
	})
}

func addAuthCtx(r *http.Request, userID, orgID string) *http.Request {
	ctx := context.WithValue(r.Context(), domain.CtxUserID, userID)
	ctx = context.WithValue(ctx, domain.CtxOrgID, orgID)
	return r.WithContext(ctx)
}

func addChiURLParam(r *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx)
	return r.WithContext(ctx)
}

func TestMessageHandler_SendMessage_Success(t *testing.T) {
	svc := &mockMessageSVC{
		sendMessageFn: func(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error) {
			return &domain.Message{
				ID:             "msg-1",
				ConversationID: "conv-1",
				OrgID:          "org-1",
				SenderID:       "user-1",
				Content:        "hello",
			}, nil
		},
	}
	h := newMessageHandler(svc)

	body := dto.SendMessageRequest{Content: "hello"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/conversations/conv-1/messages", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	w := httptest.NewRecorder()
	h.SendMessage(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.MessageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Errorf("expected 'hello', got '%s'", resp.Content)
	}
}

func TestMessageHandler_EditMessage_Success(t *testing.T) {
	editedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	svc := &mockMessageSVC{
		editMessageFn: func(ctx context.Context, params domain.EditMessageParams) (*domain.Message, error) {
			return &domain.Message{
				ID:             "msg-1",
				ConversationID: "conv-1",
				OrgID:          "org-1",
				SenderID:       "user-1",
				Content:        "updated",
				EditedAt:       &editedAt,
			}, nil
		},
	}
	h := newMessageHandler(svc)

	body := dto.EditMessageRequest{Content: "updated"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("PATCH", "/api/conversations/conv-1/messages/msg-1", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.EditMessage(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.MessageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Content != "updated" {
		t.Errorf("expected 'updated', got '%s'", resp.Content)
	}
}

func TestMessageHandler_DeleteMessage_Success(t *testing.T) {
	svc := &mockMessageSVC{
		deleteMessageFn: func(ctx context.Context, orgID, msgID, convID, deleterID string) error {
			return nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("DELETE", "/api/conversations/conv-1/messages/msg-1", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.DeleteMessage(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestMessageHandler_PinMessage_Success(t *testing.T) {
	called := false
	svc := &mockMessageSVC{
		pinMessageFn: func(ctx context.Context, orgID, msgID, convID, pinnerID string) error {
			called = true
			return nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("POST", "/api/conversations/conv-1/messages/msg-1/pin", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.PinMessage(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !called {
		t.Error("PinMessage was not called")
	}
}

func TestMessageHandler_UnpinMessage_Success(t *testing.T) {
	called := false
	svc := &mockMessageSVC{
		unpinMessageFn: func(ctx context.Context, orgID, msgID, convID string) error {
			called = true
			return nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("DELETE", "/api/conversations/conv-1/messages/msg-1/pin", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.UnpinMessage(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !called {
		t.Error("UnpinMessage was not called")
	}
}

func TestMessageHandler_ListMessages_Success(t *testing.T) {
	svc := &mockMessageSVC{
		listMessagesFn: func(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
			return &domain.MessageListResult{
				Items: []*domain.Message{
					{ID: "msg-1", ConversationID: convID, Content: "hello", SenderID: "user-1", Sender: &domain.User{ID: "user-1", Name: "Alice"}},
					{ID: "msg-2", ConversationID: convID, Content: "world", SenderID: "user-1", Sender: &domain.User{ID: "user-1", Name: "Alice"}},
				},
				HasMore: false,
			}, nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("GET", "/api/conversations/conv-1/messages", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	w := httptest.NewRecorder()
	h.ListMessages(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.MessageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].Content != "hello" {
		t.Errorf("expected 'hello', got '%s'", resp.Items[0].Content)
	}
}

func TestMessageHandler_ListReplies_Success(t *testing.T) {
	svc := &mockMessageSVC{
		listRepliesFn: func(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
			return &domain.MessageListResult{
				Items: []*domain.Message{
					{ID: "reply-1", ConversationID: convID, Content: "a reply", ParentID: &parentID, SenderID: "user-1", Sender: &domain.User{ID: "user-1", Name: "Alice"}},
				},
				HasMore: false,
			}, nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("GET", "/api/conversations/conv-1/messages/parent-1/replies", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "parent-1")
	w := httptest.NewRecorder()
	h.ListReplies(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.MessageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(resp.Items))
	}
	if resp.Items[0].Content != "a reply" {
		t.Errorf("expected 'a reply', got '%s'", resp.Items[0].Content)
	}
}

func TestMessageHandler_AddReaction_Success(t *testing.T) {
	svc := &mockMessageSVC{
		addReactionFn: func(ctx context.Context, params domain.AddReactionParams) error {
			return nil
		},
	}
	h := newMessageHandler(svc)

	body := dto.ReactionRequest{Emoji: "👍"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/conversations/conv-1/messages/msg-1/reactions", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.AddReaction(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestMessageHandler_RemoveReaction_Success(t *testing.T) {
	svc := &mockMessageSVC{
		removeReactionFn: func(ctx context.Context, params domain.RemoveReactionParams) error {
			return nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("DELETE", "/api/conversations/conv-1/messages/msg-1/reactions/%F0%9F%91%8D", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	r = addChiURLParam(r, "emoji", "👍")
	w := httptest.NewRecorder()
	h.RemoveReaction(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestMessageHandler_AddReaction_ValidationError(t *testing.T) {
	h := newMessageHandler(&mockMessageSVC{})

	body := dto.ReactionRequest{Emoji: ""}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/conversations/conv-1/messages/msg-1/reactions", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "msg-1")
	w := httptest.NewRecorder()
	h.AddReaction(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMessageHandler_SearchMessages_Success(t *testing.T) {
	svc := &mockMessageSVC{
		searchMessagesFn: func(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error) {
			return &domain.MessageSearchListResult{
				Items: []*domain.MessageSearchResult{
					{
						Message:          &domain.Message{ID: "msg-1", Content: "hello world", SenderID: "user-1", Sender: &domain.User{ID: "user-1", Name: "Alice"}},
						Rank:             0.5,
						Snippet:          "hello <mark>world</mark>",
						ConversationName: "general",
						ConversationType: domain.ConvChannel,
					},
				},
				HasMore: false,
			}, nil
		},
	}
	h := newMessageHandler(svc)

	r := httptest.NewRequest("GET", "/api/conversations/search?q=hello", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	w := httptest.NewRecorder()
	h.SearchMessages(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp dto.MessageSearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].ConversationName != "general" {
		t.Errorf("expected 'general', got '%s'", resp.Items[0].ConversationName)
	}
}

func TestMessageHandler_SearchMessages_EmptyQuery(t *testing.T) {
	h := newMessageHandler(&mockMessageSVC{})

	r := httptest.NewRequest("GET", "/api/conversations/search?q=", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	w := httptest.NewRecorder()
	h.SearchMessages(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestMessageHandler_DownloadAttachment_ForcesXSSHeaders verifies that message
// attachment downloads always serve as a download (Content-Disposition:
// attachment) with nosniff, and that blocked scriptable content types are
// downgraded to application/octet-stream to prevent stored XSS.
func TestMessageHandler_DownloadAttachment_ForcesXSSHeaders(t *testing.T) {
	attRepo := &mockMsgAttRepo{
		getByIDAndConversationFn: func(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error) {
			return &domain.MessageAttachment{
				ID:          "att-1",
				FileName:    "report.html",
				FileSize:    42,
				ContentType: "text/html",
				StoragePath: "uploads/att-1",
			}, nil
		},
	}
	storeBack := &mockStore{
		getFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("<script>alert(1)</script>"))), nil
		},
	}
	h := NewMessageHandler(MessageHandlerDeps{
		SVC:            &mockMessageSVC{},
		MentionSvc:     &mockMentionSVC{},
		AttRepo:        attRepo,
		PendingAttRepo: &mockPendingAttRepo{},
		ReactionRepo:   &mockReactionRepoHandler{},
		AccessSvc:      &mockAccessService{},
		StoreBack:      storeBack,
		Log:            slog.Default(),
	})

	r := httptest.NewRequest("GET", "/api/conversations/conv-1/attachments/att-1/download", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "att_id", "att-1")
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Must force a download: never inline on the origin.
	if cd := w.Header().Get("Content-Disposition"); cd == "" {
		t.Error("missing Content-Disposition header (stored XSS risk)")
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// Scriptable type must be downgraded so a misconfigured browser cannot
	// render it as HTML.
	if got := w.Header().Get("Content-Type"); got == "text/html" {
		t.Error("scriptable content type served inline; expected downgrade to octet-stream")
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := w.Header().Get("Content-Length"); got != "42" {
		t.Errorf("Content-Length = %q, want 42", got)
	}
}

// TestMessageHandler_DownloadAttachment_SafeTypeNotDowngraded verifies that
// safe content types (e.g. image/png) are kept as-is while still getting the
// nosniff header.
func TestMessageHandler_DownloadAttachment_SafeTypeNotDowngraded(t *testing.T) {
	attRepo := &mockMsgAttRepo{
		getByIDAndConversationFn: func(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error) {
			return &domain.MessageAttachment{
				ID:          "att-2",
				FileName:    "screenshot.png",
				FileSize:    1234,
				ContentType: "image/png",
				StoragePath: "uploads/att-2",
			}, nil
		},
	}
	storeBack := &mockStore{
		getFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("fake-png-bytes"))), nil
		},
	}
	h := NewMessageHandler(MessageHandlerDeps{
		SVC:            &mockMessageSVC{},
		MentionSvc:     &mockMentionSVC{},
		AttRepo:        attRepo,
		PendingAttRepo: &mockPendingAttRepo{},
		ReactionRepo:   &mockReactionRepoHandler{},
		AccessSvc:      &mockAccessService{},
		StoreBack:      storeBack,
		Log:            slog.Default(),
	})

	r := httptest.NewRequest("GET", "/api/conversations/conv-1/attachments/att-2/download", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "att_id", "att-2")
	w := httptest.NewRecorder()

	h.DownloadAttachment(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png (safe type must not be downgraded)", got)
	}
}
