package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

// WorkspaceHandler exposes the multi-workspace endpoints: list the caller's
// workspaces, create a new one, and switch the active workspace. See
// docs/api/workspaces.md.
type WorkspaceHandler struct {
	orgSvc   port.OrganizationService
	userRepo port.UserRepository
	log      *slog.Logger
}

func NewWorkspaceHandler(orgSvc port.OrganizationService, userRepo port.UserRepository, log *slog.Logger) *WorkspaceHandler {
	return &WorkspaceHandler{orgSvc: orgSvc, userRepo: userRepo, log: log}
}

// resolveAccountID returns the account ID for the authenticated caller by
// looking up their current membership (users.id == session.UserID) and reading
// its account_id. Localized to the workspace endpoints so we don't have to
// thread account_id through the session/JWT/context (which would touch the WS
// handler, presence, and every context consumer).
func (h *WorkspaceHandler) resolveAccountID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNoUserInContext")
		return "", false
	}
	orgID, _ := transport.OrgIDFromContext(r.Context())
	user, err := h.userRepo.GetByID(r.Context(), orgID, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return "", false
	}
	return user.AccountID, true
}

// @Summary		List workspaces
// @Description	Returns the workspaces (organizations) the caller belongs to
// @Tags			workspaces
// @Produce		json
// @Success		200	{array}	dto.WorkspaceResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.resolveAccountID(w, r)
	if !ok {
		return
	}

	workspaces, err := h.orgSvc.ListForAccount(r.Context(), accountID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.WorkspaceResponse, len(workspaces))
	for i, ws := range workspaces {
		resp[i] = dto.NewWorkspaceResponse(
			ws.Organization.ID,
			ws.Organization.Name,
			ws.Organization.Slug,
			string(ws.Role),
			ws.IsOwner,
			ws.Organization.CreatedAt,
		)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a workspace
// @Description	Creates a new organization with the caller as owner
// @Tags			workspaces
// @Accept			json
// @Produce		json
// @Param			body	body		dto.CreateWorkspaceRequest	true	"Workspace name"
// @Success		201		{object}	dto.WorkspaceResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/workspaces [post]
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.resolveAccountID(w, r)
	if !ok {
		return
	}

	var req dto.CreateWorkspaceRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Carry the caller's display identity into the new owner membership.
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())
	caller, err := h.userRepo.GetByID(r.Context(), orgID, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	org, _, err := h.orgSvc.CreateWorkspace(r.Context(), accountID, req.Name, caller.Name, caller.Email, caller.AvatarURL)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewWorkspaceResponse(
		org.ID, org.Name, org.Slug, "owner", true, org.CreatedAt,
	))
}

// @Summary		Switch active workspace
// @Description	Switches the caller's active workspace, issuing a new session cookie
// @Tags			workspaces
// @Produce		json
// @Param			id	path		string	true	"Organization ID to switch to"
// @Success		200	{object}	dto.UserResponse
// @Failure		403	{object}	transport.ErrorResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/workspaces/{id}/switch [post]
func (h *WorkspaceHandler) Switch(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.resolveAccountID(w, r)
	if !ok {
		return
	}

	targetOrgID := chi.URLParam(r, "id")
	if targetOrgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}

	currentSessionID, _ := transport.SessionIDFromContext(r.Context())
	session, token, err := h.orgSvc.SwitchWorkspace(r.Context(), accountID, targetOrgID, currentSessionID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.SetAuthCookie(w, token, 7*24*60*60)

	// Return the refreshed user context (the new active workspace). We mirror
	// the /auth/me response shape so the frontend can update auth state in
	// place after a switch.
	user, err := h.userRepo.GetByID(r.Context(), session.OrgID, session.UserID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := dto.NewUserResponse(user)
	org, err := h.orgSvc.GetByID(r.Context(), session.OrgID)
	if err == nil && org != nil {
		resp.Org = &dto.OrgSummaryResponse{
			ID:   org.ID,
			Name: org.Name,
			Slug: org.Slug,
		}
	}
	resp.ActiveOrgID = session.OrgID

	// Include the refreshed workspace list so the client updates the switcher.
	workspaces, err := h.orgSvc.ListForAccount(r.Context(), accountID)
	if err == nil {
		resp.Workspaces = make([]*dto.WorkspaceResponse, len(workspaces))
		for i, ws := range workspaces {
			resp.Workspaces[i] = dto.NewWorkspaceResponse(
				ws.Organization.ID, ws.Organization.Name, ws.Organization.Slug,
				string(ws.Role), ws.IsOwner, ws.Organization.CreatedAt,
			)
		}
	}

	transport.JSON(w, r, http.StatusOK, resp)
}
