package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/storage"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MessageHandlerDeps contains all dependencies for MessageHandler.
// Use this struct to avoid long parameter lists when constructing MessageHandler.
type MessageHandlerDeps struct {
	SVC            port.MessageService
	MentionSvc     port.MentionService
	AttRepo        port.MessageAttachmentRepository
	PendingAttRepo port.PendingAttachmentRepository
	ReactionRepo   port.ReactionRepository
	AccessSvc      port.AccessService
	StoreBack      storage.Storage
	Log            *slog.Logger
}

type MessageHandler struct {
	svc            port.MessageService
	mentionSvc     port.MentionService
	attRepo        port.MessageAttachmentRepository
	pendingAttRepo port.PendingAttachmentRepository
	reactionRepo   port.ReactionRepository
	accessSvc      port.AccessService
	storeBack      storage.Storage
	log            *slog.Logger
}

// NewMessageHandler creates a new MessageHandler with the provided dependencies.
func NewMessageHandler(deps MessageHandlerDeps) *MessageHandler {
	return &MessageHandler{
		svc:            deps.SVC,
		mentionSvc:     deps.MentionSvc,
		attRepo:        deps.AttRepo,
		pendingAttRepo: deps.PendingAttRepo,
		reactionRepo:   deps.ReactionRepo,
		accessSvc:      deps.AccessSvc,
		storeBack:      deps.StoreBack,
		log:            deps.Log,
	}
}

// @Summary		List messages
// @Description	Cursor-paginated messages in a conversation
// @Tags			messages
// @Param			id		path		string	true	"Conversation ID"
// @Param			before	query		string	false	"Cursor for pagination"
// @Param			limit	query		int		false	"Page size (max 100)"
// @Success		200		{object}	dto.MessageListResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages [get]
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, convID) {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	result, err := h.svc.ListMessages(r.Context(), orgID, convID, domain.MessageFilter{
		Before: r.URL.Query().Get("before"),
		Limit:  limit,
	})
	if err != nil {
		h.log.Error("list messages", "error", err)
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrConversationNotFound")
		return
	}

	// Hydrate reactions with current user context
	h.hydrateMessageReactions(r.Context(), result.Items, userID)

	resp := &dto.MessageListResponse{
		Items:      make([]*dto.MessageResponse, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, m := range result.Items {
		resp.Items[i] = dto.NewMessageResponse(m)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Send message
// @Description	Send a message in a conversation
// @Tags			messages
// @Param			id		path		string					true	"Conversation ID"
// @Param			body	body		dto.SendMessageRequest	true	"Message content"
// @Success		201		{object}	dto.MessageResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages [post]
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req dto.SendMessageRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")

	if !h.requireConvSendAccess(w, r, convID) {
		return
	}

	msg, err := h.svc.SendMessage(r.Context(), domain.CreateMessageParams{
		ConversationID:     convID,
		OrgID:              orgID,
		SenderID:           userID,
		Content:            req.Content,
		ParentID:           req.ParentID,
		ForwardedMessageID: req.ForwardedMessageID,
		AttachmentIDs:      req.AttachmentIDs,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewMessageResponse(msg))
}

// @Summary		Edit message
// @Description	Edit a message
// @Tags			messages
// @Param			id		path		string					true	"Conversation ID"
// @Param			msg_id	path		string					true	"Message ID"
// @Param			body	body		dto.EditMessageRequest	true	"New content"
// @Success		200		{object}	dto.MessageResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages/{msg_id} [patch]
func (h *MessageHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	var req dto.EditMessageRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvSendAccess(w, r, convID) {
		return
	}

	msg, err := h.svc.EditMessage(r.Context(), domain.EditMessageParams{
		MsgID:    msgID,
		ConvID:   convID,
		OrgID:    orgID,
		EditorID: userID,
		Content:  req.Content,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewMessageResponse(msg))
}

// @Summary		Delete message
// @Description	Soft-delete a message
// @Tags			messages
// @Param			id		path	string	true	"Conversation ID"
// @Param			msg_id	path	string	true	"Message ID"
// @Success		204
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages/{msg_id} [delete]
func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvSendAccess(w, r, convID) {
		return
	}

	if err := h.svc.DeleteMessage(r.Context(), orgID, msgID, convID, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		List replies
// @Description	List replies to a message
// @Tags			messages
// @Param			id		path		string	true	"Conversation ID"
// @Param			msg_id	path		string	true	"Parent message ID"
// @Param			before	query		string	false	"Cursor for pagination"
// @Param			limit	query		int		false	"Page size (max 100)"
// @Success		200		{object}	dto.MessageListResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages/{msg_id}/replies [get]
func (h *MessageHandler) ListReplies(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvAccess(w, r, convID) {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	result, err := h.svc.ListReplies(r.Context(), orgID, convID, msgID, domain.MessageFilter{
		Before: r.URL.Query().Get("before"),
		Limit:  limit,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	// Hydrate reactions with current user context
	h.hydrateMessageReactions(r.Context(), result.Items, userID)

	resp := &dto.MessageListResponse{
		Items:      make([]*dto.MessageResponse, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, m := range result.Items {
		resp.Items[i] = dto.NewMessageResponse(m)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Pin message
// @Description	Pin a message to the conversation
// @Tags			messages
// @Param			id		path	string	true	"Conversation ID"
// @Param			msg_id	path	string	true	"Message ID"
// @Success		200
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages/{msg_id}/pin [post]
func (h *MessageHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvManageAccess(w, r, convID) {
		return
	}

	if err := h.svc.PinMessage(r.Context(), orgID, msgID, convID, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// @Summary		Unpin message
// @Description	Unpin a message from the conversation
// @Tags			messages
// @Param			id		path	string	true	"Conversation ID"
// @Param			msg_id	path	string	true	"Message ID"
// @Success		200
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/messages/{msg_id}/pin [delete]
func (h *MessageHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvManageAccess(w, r, convID) {
		return
	}

	if err := h.svc.UnpinMessage(r.Context(), orgID, msgID, convID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *MessageHandler) requireConvAccess(w http.ResponseWriter, r *http.Request, convID string) bool {
	return EnsureConversationAccess(w, r, h.accessSvc, convID)
}

// requireConvSendAccess enforces the channel-level "send" permission
// (channel:send), honoring per-user overrides + role/everyone rules up the
// parent chain. Use for send/edit/delete/react/upload; anything that creates
// or mutates message content. Mirrors the WS CanSendInConversation check so
// the HTTP and WS layers agree.
func (h *MessageHandler) requireConvSendAccess(w http.ResponseWriter, r *http.Request, convID string) bool {
	return EnsureConversationSendAccess(w, r, h.accessSvc, convID)
}

// requireConvManageAccess enforces the channel-level "manage" permission
// (channel:manage). Use for moderation actions like pin/unpin.
func (h *MessageHandler) requireConvManageAccess(w http.ResponseWriter, r *http.Request, convID string) bool {
	return EnsureConversationManageAccess(w, r, h.accessSvc, convID)
}

// @Summary		Upload attachment
// @Description	Upload a file attachment for a message
// @Tags			messages
// @Accept			mpfd
// @Produce		json
// @Param			id		path		string	true	"Conversation ID"
// @Param			file	formData	file	true	"File to upload"
// @Success		201		{object}	dto.UploadAttachmentResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/attachments [post]
func (h *MessageHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if !h.requireConvSendAccess(w, r, convID) {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid", "ErrFileTooLarge")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid", "ErrFileRequired")
		return
	}
	defer file.Close()

	attID := uuid.New().String()
	storagePath := fmt.Sprintf("chat/%s/%s", convID, attID)

	if err := h.storeBack.Save(r.Context(), storagePath, file); err != nil {
		h.log.Error("save attachment", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to save file")
		return
	}

	fileURL, _ := h.storeBack.URL(r.Context(), storagePath)

	userID, _ := transport.UserIDFromContext(r.Context())
	if err := h.pendingAttRepo.Create(r.Context(), &domain.PendingAttachment{
		ID:             attID,
		ConversationID: convID,
		FileName:       header.Filename,
		FileSize:       header.Size,
		ContentType:    header.Header.Get("Content-Type"),
		StoragePath:    storagePath,
		UploadedBy:     userID,
	}); err != nil {
		h.log.Error("persist pending attachment", "error", err)
		h.storeBack.Delete(r.Context(), storagePath)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to save attachment metadata")
		return
	}

	resp := &dto.UploadAttachmentResponse{
		ID:          attID,
		FileName:    header.Filename,
		FileSize:    header.Size,
		ContentType: header.Header.Get("Content-Type"),
		URL:         fileURL,
	}
	transport.JSON(w, r, http.StatusCreated, resp)
}

// @Summary		Search mentions
// @Description	Search for users, projects, tasks, channels for @mention autocomplete
// @Tags			mentions
// @Param			q		query		string	true	"Search query"
// @Param			types	query		string	false	"Comma-separated types: user,project,task,channel,everyone"
// @Param			limit	query		int		false	"Max results (default 10, max 20)"
// @Success		200		{object}	dto.MentionSearchResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/mentions/search [get]
func (h *MessageHandler) SearchMentions(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	userRole, _ := transport.RoleFromContext(r.Context())
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	var typeFilters []domain.MentionType
	typesStr := r.URL.Query().Get("types")
	if typesStr != "" {
		for _, t := range strings.Split(typesStr, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			typeFilters = append(typeFilters, domain.MentionType(t))
		}
	}

	results, err := h.mentionSvc.Search(r.Context(), orgID, userID, string(userRole), query, typeFilters, limit)
	if err != nil {
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	resp := make([]*dto.MentionResultResponse, len(results))
	for i, r := range results {
		resp[i] = dto.NewMentionResultResponse(r)
	}
	transport.JSON(w, r, http.StatusOK, &dto.MentionSearchResponse{Results: resp})
}

// @Summary		Search messages
// @Description	Full-text search across workspace chat, project chat, and DMs
// @Tags			messages
// @Param			q				query	string	true	"Search query"
// @Param			scope			query	string	false	"all|workspace|project|dm (default all)"
// @Param			conversation_id	query	string	false	"Limit to a specific conversation"
// @Param			sender_id		query	string	false	"Filter by sender user ID"
// @Param			has_attachment	query	bool	false	"Only messages with attachments"
// @Param			has_link		query	bool	false	"Only messages containing URLs"
// @Param			is_pinned		query	bool	false	"Only pinned messages"
// @Param			after			query	string	false	"ISO date lower bound"
// @Param			before			query	string	false	"ISO date upper bound"
// @Param			cursor			query	string	false	"Pagination cursor"
// @Param			limit			query	int		false	"Page size (max 100)"
// @Success		200				{object}	dto.MessageSearchResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/search [get]
func (h *MessageHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	query := r.URL.Query().Get("q")
	if query == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrQueryRequired")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	roleStr, _ := transport.RoleFromContext(r.Context())
	includeProjectLinked := roleStr == "owner" || roleStr == "admin" || roleStr == "member"

	filter := domain.MessageSearchFilter{
		Query:                query,
		Scope:                domain.MessageSearchScope(r.URL.Query().Get("scope")),
		ConversationID:       r.URL.Query().Get("conversation_id"),
		SenderID:             r.URL.Query().Get("sender_id"),
		After:                r.URL.Query().Get("after"),
		Before:               r.URL.Query().Get("before"),
		Cursor:               r.URL.Query().Get("cursor"),
		Limit:                limit,
		IncludeProjectLinked: includeProjectLinked,
	}
	if filter.Scope == "" {
		filter.Scope = domain.MessageSearchScopeAll
	}
	if r.URL.Query().Get("has_attachment") == "true" {
		filter.HasAttachment = true
	}
	if r.URL.Query().Get("has_link") == "true" {
		filter.HasLink = true
	}
	if r.URL.Query().Get("is_pinned") == "true" {
		filter.IsPinned = true
	}

	result, err := h.svc.SearchMessages(r.Context(), orgID, userID, filter)
	if err != nil {
		if errors.Is(err, apperr.ErrInvalidInput) {
			transport.RespondWithError(w, r, h.log, err)
			return
		}
		h.log.Error("search messages", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	// The SQL pre-filter (membership / project-linked) is coarse: it cannot
	// evaluate per-channel view rules or user overrides. Post-filter every
	// hit through the same access layer the message-list endpoint uses so a
	// channel:view deny can't be bypassed via search. Result sets are bounded
	// by the query limit, so the per-row check stays cheap.
	role := domain.Role(roleStr)
	items := make([]*domain.MessageSearchResult, 0, len(result.Items))
	for _, item := range result.Items {
		if err := h.accessSvc.EnsureConversationAccess(r.Context(), orgID, userID, role, item.Message.ConversationID); err != nil {
			continue
		}
		items = append(items, item)
	}
	result.Items = items

	resp := dto.NewMessageSearchResponse(result)
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Download attachment
// @Description	Download a message attachment
// @Tags			messages
// @Param			id		path	string	true	"Conversation ID"
// @Param			att_id	path	string	true	"Attachment ID"
// @Success		200
// @Failure		404		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/attachments/{att_id}/download [get]
func (h *MessageHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if !h.requireConvAccess(w, r, convID) {
		return
	}

	attID := chi.URLParam(r, "att_id")

	// Fetch the attachment scoped to the conversation in the URL path. This
	// prevents a cross-conversation IDOR where a caller with access to convA
	// requests an attachment from convB by passing convA in the path and the
	// foreign attachment ID. The join in GetByIDAndConversation returns
	// ErrNotFound when the attachment doesn't belong to this conversation.
	att, err := h.attRepo.GetByIDAndConversation(r.Context(), attID, convID)
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAttachmentNotFound")
		return
	}

	reader, err := h.storeBack.Get(r.Context(), att.StoragePath)
	if err != nil {
		h.log.Error("get attachment from storage", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to read attachment")
		return
	}
	defer reader.Close()

	// Force a download and prevent inline rendering: serving user-uploaded
	// files inline on the Breeze origin would allow stored XSS (e.g. an
	// uploaded HTML file executing with the victim's cookies). nosniff stops
	// browsers from interpreting the bytes as a different, dangerous type.
	contentType := att.ContentType
	if isBlockedAttachmentType(contentType) {
		contentType = "application/octet-stream"
	}
	dispFilename := att.FileName
	if dispFilename == "" {
		dispFilename = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": dispFilename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.FileSize))
	io.Copy(w, reader)
}

// @Summary	Add reaction to a message
// @Tags		messages
// @Accept		json
// @Produce	json
// @Param		id		path	string				true	"Conversation ID"
// @Param		msg_id	path	string				true	"Message ID"
// @Param		body	body	dto.ReactionRequest	true	"Reaction emoji"
// @Success	204
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/messages/{msg_id}/reactions [post]
func (h *MessageHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvSendAccess(w, r, convID) {
		return
	}
	var req dto.ReactionRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.AddReaction(r.Context(), domain.AddReactionParams{
		MsgID:  msgID,
		ConvID: convID,
		UserID: userID,
		OrgID:  orgID,
		Emoji:  req.Emoji,
	}); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary	Remove reaction from a message
// @Tags		messages
// @Param		id		path	string	true	"Conversation ID"
// @Param		msg_id	path	string	true	"Message ID"
// @Param		emoji	path	string	true	"Emoji"
// @Success	204
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/messages/{msg_id}/reactions/{emoji} [delete]
func (h *MessageHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())
	convID := chi.URLParam(r, "id")
	msgID := chi.URLParam(r, "msg_id")

	if !h.requireConvSendAccess(w, r, convID) {
		return
	}
	emoji := chi.URLParam(r, "emoji")
	if err := h.svc.RemoveReaction(r.Context(), domain.RemoveReactionParams{
		MsgID:  msgID,
		ConvID: convID,
		UserID: userID,
		OrgID:  orgID,
		Emoji:  emoji,
	}); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hydrateMessageReactions hydrates reactions for messages with proper Mine flag
func (h *MessageHandler) hydrateMessageReactions(ctx context.Context, messages []*domain.Message, currentUserID string) {
	if len(messages) == 0 {
		return
	}
	msgIDs := make([]string, 0, len(messages))
	for _, m := range messages {
		msgIDs = append(msgIDs, m.ID)
	}
	reactions, err := h.reactionRepo.ListForMessages(ctx, msgIDs)
	if err != nil {
		h.log.Error("failed to hydrate reactions", "error", err)
		return
	}
	// Group reactions by message ID and emoji
	msgReactions := make(map[string][]domain.ReactionGroup)
	for _, r := range reactions {
		groups := msgReactions[r.MessageID]
		found := false
		for i := range groups {
			if groups[i].Emoji == r.Emoji {
				groups[i].Count++
				groups[i].UserIDs = append(groups[i].UserIDs, r.UserID)
				if r.UserID == currentUserID {
					groups[i].Mine = true
				}
				found = true
				break
			}
		}
		if !found {
			group := domain.ReactionGroup{
				Emoji:   r.Emoji,
				Count:   1,
				UserIDs: []string{r.UserID},
				Mine:    r.UserID == currentUserID,
			}
			msgReactions[r.MessageID] = append(groups, group)
		}
	}
	// Attach reactions to messages
	for _, m := range messages {
		m.Reactions = msgReactions[m.ID]
	}
}
