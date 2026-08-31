package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type ConversationHandler struct {
	svc       port.ConversationService
	accessSvc port.AccessService
	log       *slog.Logger
}

func NewConversationHandler(svc port.ConversationService, accessSvc port.AccessService, log *slog.Logger) *ConversationHandler {
	return &ConversationHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// requireConvAccess writes an error response and returns false when the
// authenticated user has no access to the conversation in the {id} route
// param. Shared with the message handler via EnsureConversationAccess.
func (h *ConversationHandler) requireConvAccess(w http.ResponseWriter, r *http.Request, convID string) bool {
	return EnsureConversationAccess(w, r, h.accessSvc, convID)
}

// requireConvManageAccess is the channel-level "manage" variant of
// requireConvAccess. Management endpoints must key off the caller's resolved
// channel:manage permission, not merely view access.
func (h *ConversationHandler) requireConvManageAccess(w http.ResponseWriter, r *http.Request, convID string) bool {
	return EnsureConversationManageAccess(w, r, h.accessSvc, convID)
}

// @Summary		List conversations
// @Description	List conversations for current user with optional scope filter
// @Tags			conversations
// @Param			scope	query		string	false	"Filter scope: workspace, dms, project:{id}"
// @Param			cursor	query		string	false	"Cursor for pagination"
// @Param			limit	query		int		false	"Page size (max 50)"
// @Success		200		{object}	dto.ConversationListResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations [get]
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	scope := r.URL.Query().Get("scope")
	var scopePtr *string
	if scope != "" {
		scopePtr = &scope
	}

	roleStr, _ := transport.RoleFromContext(r.Context())
	includeProjectLinked := roleStr == "owner" || roleStr == "admin" || roleStr == "member"

	result, err := h.svc.ListMyConversations(r.Context(), orgID, userID, domain.ConversationFilter{
		Cursor:               r.URL.Query().Get("cursor"),
		Limit:                limit,
		Scope:                scopePtr,
		IncludeProjectLinked: includeProjectLinked,
	})
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := &dto.ConversationListResponse{
		Items:      make([]*dto.ConversationResponse, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, c := range result.Items {
		resp.Items[i] = dto.NewConversationResponse(c)
		if c.LastMessage != nil {
			resp.Items[i].LastMessage = dto.NewMessageResponse(c.LastMessage)
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create conversation
// @Description	Create a channel, DM, or group DM
// @Tags			conversations
// @Param			body	body		dto.CreateConversationRequest	true	"Conversation details"
// @Success		201		{object}	dto.ConversationResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations [post]
func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConversationRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	var cv *domain.Conversation
	var err error

	switch domain.ConversationType(req.Type) {
	case domain.ConvChannel, domain.ConvVoice, domain.ConvCategory:
		cv, err = h.svc.CreateChannel(r.Context(), domain.CreateConversationParams{
			OrgID:      orgID,
			ParentID:   req.ParentID,
			Name:       req.Name,
			Topic:      req.Topic,
			Type:       domain.ConversationType(req.Type),
			CreatedBy:  userID,
			ProjectIDs: req.ProjectIDs,
		})
	case domain.ConvDirect:
		if req.TargetID == nil {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrTargetIDRequired")
			return
		}
		cv, err = h.svc.CreateDM(r.Context(), orgID, userID, *req.TargetID)
	case domain.ConvGroup:
		cv, err = h.svc.CreateGroupDM(r.Context(), orgID, userID, req.MemberIDs)
	default:
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidType")
		return
	}

	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewConversationResponse(cv))
}

// @Summary		Get conversation
// @Description	Get a single conversation by ID
// @Tags			conversations
// @Param			id	path		string	true	"Conversation ID"
// @Success		200	{object}	dto.ConversationResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/conversations/{id} [get]
func (h *ConversationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	cv, err := h.svc.GetByID(r.Context(), orgID, id, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewConversationResponse(cv))
}

// @Summary		Update conversation
// @Description	Update conversation name
// @Tags			conversations
// @Param			id		path		string							true	"Conversation ID"
// @Param			body	body		dto.UpdateConversationRequest	true	"Update details"
// @Success		200		{object}	dto.ConversationResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/conversations/{id} [patch]
func (h *ConversationHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateConversationRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvManageAccess(w, r, id) {
		return
	}

	cv, err := h.svc.GetByID(r.Context(), orgID, id, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	if req.Name == "" && req.Topic == nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrNameOrTopicRequired")
		return
	}

	if req.Name != "" {
		cv.Name = req.Name
	}
	if req.Topic != nil {
		if len(*req.Topic) > 500 {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrTopicTooLong")
			return
		}
		cv.Topic = *req.Topic
	}
	if err := h.svc.UpdateConversation(r.Context(), cv); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewConversationResponse(cv))
}

// @Summary		Delete conversation
// @Description	Delete a conversation
// @Tags			conversations
// @Param			id	path	string	true	"Conversation ID"
// @Success		204
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id} [delete]
func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvManageAccess(w, r, id) {
		return
	}

	if err := h.svc.DeleteConversation(r.Context(), orgID, id, userID, domain.Role(roleStr)); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		List members
// @Description	List members of a conversation
// @Tags			conversations
// @Param			id	path	string	true	"Conversation ID"
// @Success		200	{array}	dto.ConversationMemberResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/members [get]
func (h *ConversationHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	members, err := h.svc.GetMembers(r.Context(), id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.ConversationMemberResponse, len(members))
	for i, m := range members {
		resp[i] = dto.NewConversationMemberResponse(m)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Add members
// @Description	Add members to a conversation
// @Tags			conversations
// @Param			id		path	string					true	"Conversation ID"
// @Param			body	body	dto.AddMemberRequest	true	"Member IDs"
// @Success		200
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/members [post]
func (h *ConversationHandler) AddMembers(w http.ResponseWriter, r *http.Request) {
	var req dto.AddMemberRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvManageAccess(w, r, id) {
		return
	}

	if err := h.svc.AddMembers(r.Context(), orgID, id, userID, req.UserIDs); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// @Summary		Remove member
// @Description	Remove a member from a conversation
// @Tags			conversations
// @Param			id		path	string	true	"Conversation ID"
// @Param			user_id	path	string	true	"User ID to remove"
// @Success		204
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/members/{user_id} [delete]
func (h *ConversationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	targetID := chi.URLParam(r, "user_id")

	if !h.requireConvManageAccess(w, r, id) {
		return
	}

	if err := h.svc.RemoveMember(r.Context(), orgID, id, userID, targetID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Mark read
// @Description	Mark conversation as read
// @Tags			conversations
// @Param			id	path	string	true	"Conversation ID"
// @Success		200
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/read [post]
func (h *ConversationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	if err := h.svc.MarkRead(r.Context(), id, userID); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// @Summary		Toggle mute
// @Description	Toggle mute for a conversation
// @Tags			conversations
// @Param			id		path	string			true	"Conversation ID"
// @Param			body	body	dto.MuteRequest	true	"Mute state"
// @Success		200
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/mute [patch]
func (h *ConversationHandler) SetMuted(w http.ResponseWriter, r *http.Request) {
	var req dto.MuteRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	if err := h.svc.SetMuted(r.Context(), orgID, id, userID, req.Muted); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// @Summary		Pinned messages
// @Description	List pinned messages in a conversation
// @Tags			conversations
// @Param			id	path		string	true	"Conversation ID"
// @Success		200	{object}	dto.PinnedMessagesResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/pinned [get]
func (h *ConversationHandler) PinnedMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	pinned, err := h.svc.GetPinnedMessages(r.Context(), id, 10)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := &dto.PinnedMessagesResponse{
		Items: make([]*dto.MessageResponse, len(pinned)),
	}
	for i, m := range pinned {
		resp.Items[i] = dto.NewMessageResponse(m)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary	Set conversation parent/position
// @Tags		conversations
// @Accept		json
// @Produce	json
// @Param		id		path	string								true	"Conversation ID"
// @Param		body	body	dto.UpdateChannelPositionRequest	true	"New parent/position"
// @Success	200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/position [patch]
func (h *ConversationHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if !h.requireConvManageAccess(w, r, id) {
		return
	}
	var req dto.UpdateChannelPositionRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.UpdateChannelParent(r.Context(), orgID, id, req.ParentID, req.PositionKey); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary	Set per-conversation notification level
// @Tags		conversations
// @Accept		json
// @Produce	json
// @Param		id		path	string							true	"Conversation ID"
// @Param		body	body	dto.NotificationLevelRequest	true	"New notification level"
// @Success	200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/notification-level [patch]
func (h *ConversationHandler) SetNotificationLevel(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if !h.requireConvAccess(w, r, id) {
		return
	}
	var req dto.NotificationLevelRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.SetNotificationLevel(r.Context(), orgID, id, userID, domain.NotificationLevel(req.Level)); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		List conversations by parent
// @Description	List all channels under a specific parent (category)
// @Tags			conversations
// @Param			parent_id	query		string	true	"Parent ID"
// @Success		200			{array}		dto.ConversationResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/by-parent [get]
func (h *ConversationHandler) ListByParent(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	parentID := r.URL.Query().Get("parent_id")

	if parentID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrParentIDRequired")
		return
	}

	// The parent is itself a conversation; verify the caller can access it
	// before listing its children. This prevents a user from enumerating
	// channels under a category they have no access to.
	if !h.requireConvAccess(w, r, parentID) {
		return
	}

	roleStr, _ := transport.RoleFromContext(r.Context())
	includeProjectLinked := roleStr == "owner" || roleStr == "admin" || roleStr == "member"
	convs, err := h.svc.ListByParent(r.Context(), orgID, parentID, userID, domain.Role(roleStr), includeProjectLinked)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.ConversationResponse, len(convs))
	for i, c := range convs {
		resp[i] = dto.NewConversationResponse(c)
		if c.LastMessage != nil {
			resp[i].LastMessage = dto.NewMessageResponse(c.LastMessage)
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Get channel project links
// @Description	Get the projects linked to a channel
// @Tags			conversations
// @Param			id	path		string	true	"Conversation ID"
// @Success		200	{object}	map[string][]string
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/projects [get]
func (h *ConversationHandler) GetProjectLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	projectIDs, err := h.svc.GetChannelProjectLinks(r.Context(), id)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, map[string][]string{"project_ids": projectIDs})
}

// @Summary		Set channel project links
// @Description	Set the projects linked to a channel
// @Tags			conversations
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Conversation ID"
// @Param			body	body	dto.SetProjectLinksRequest	true	"Project IDs"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/projects [put]
func (h *ConversationHandler) SetProjectLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !h.requireConvManageAccess(w, r, id) {
		return
	}

	var req dto.SetProjectLinksRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.SetChannelProjectLinks(r.Context(), id, req.ProjectIDs); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		List channel access
// @Description	List all users who have access to a channel and how they got it
// @Tags			conversations
// @Param			id	path		string	true	"Conversation ID"
// @Success		200	{array}	dto.ChannelAccessEntryResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/conversations/{id}/access [get]
func (h *ConversationHandler) ListAccess(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if !h.requireConvAccess(w, r, id) {
		return
	}

	entries, err := h.svc.ListAccess(r.Context(), orgID, id)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.ChannelAccessEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = &dto.ChannelAccessEntryResponse{
			User:   dto.NewUserResponse(e.User),
			Source: e.Source,
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}
