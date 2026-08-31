package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

// OrganizationHandler exposes the org settings endpoints: view the active
// org, rename it / set the message edit window, and delete it (danger zone).
// Viewing is open to any authenticated member; update requires
// PermOrgManage (owner/admin) and delete requires PermOrgDelete (owner only).
type OrganizationHandler struct {
	orgSvc port.OrganizationService
	log    *slog.Logger
}

func NewOrganizationHandler(orgSvc port.OrganizationService, log *slog.Logger) *OrganizationHandler {
	return &OrganizationHandler{orgSvc: orgSvc, log: log}
}

// @Summary		Get organization
// @Description	Returns the caller's active organization
// @Tags			organization
// @Produce		json
// @Success		200	{object}	dto.OrganizationResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/organization [get]
func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	org, err := h.orgSvc.GetByID(r.Context(), orgID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewOrganizationResponse(org))
}

// @Summary		Update organization
// @Description	Renames the organization and sets the message edit window
// @Tags			organization
// @Accept			json
// @Produce		json
// @Param			body	body		dto.UpdateOrganizationRequest	true	"Organization updates"
// @Success		200		{object}	dto.OrganizationResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/organization [patch]
func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	var req dto.UpdateOrganizationRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	org, err := h.orgSvc.Update(r.Context(), orgID, req.Name, req.MessageEditWindowMinute)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewOrganizationResponse(org))
}

// @Summary		Delete organization
// @Description	Permanently deletes the organization (type-to-confirm the org name)
// @Tags			organization
// @Accept			json
// @Produce		json
// @Param			body	body		dto.DeleteOrganizationRequest	true	"Confirmation"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/organization [delete]
func (h *OrganizationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	var req dto.DeleteOrganizationRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.orgSvc.Delete(r.Context(), orgID, req.Confirm); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}
