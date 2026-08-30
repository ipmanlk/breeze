package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type SetupHandler struct {
	org  port.OrganizationService
	auth port.AuthService
	log  *slog.Logger
}

func NewSetupHandler(org port.OrganizationService, auth port.AuthService, log *slog.Logger) *SetupHandler {
	return &SetupHandler{org: org, auth: auth, log: log}
}

// @Summary		Check setup status
// @Description	Returns whether the initial organization has been created
// @Tags			setup
// @Produce		json
// @Success		200	{object}	map[string]bool	"needs_setup flag"
// @Router			/setup [get]
func (h *SetupHandler) Check(w http.ResponseWriter, r *http.Request) {
	exists, err := h.org.Exists(r.Context())
	if err != nil {
		h.log.Error("check org exists", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	transport.JSON(w, r, http.StatusOK, map[string]bool{"needs_setup": !exists})
}

// @Summary		Complete initial setup
// @Description	Creates the first organization and admin user, returns auth cookie
// @Tags			setup
// @Accept			json
// @Produce		json
// @Param			body	body		dto.SetupRequest	true	"Setup details"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		409		{object}	transport.ErrorResponse	"setup already completed"
// @Router			/setup [post]
func (h *SetupHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req dto.SetupRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	_, _, err := h.org.Create(r.Context(), req.OrgName, req.Name, req.Email, req.Password)
	if err != nil {
		if err == apperr.ErrSetupComplete {
			transport.LocalizedErrorJSON(w, r, http.StatusConflict, "setup_complete", "ErrSetupComplete")
			return
		}
		h.log.Error("setup", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	_, _, token, err := h.auth.Login(r.Context(), domain.LoginParams{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: transport.UserAgent(r),
		IPAddress: transport.ClientIP(r),
	})
	if err != nil {
		h.log.Error("auto-login after setup", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	transport.SetAuthCookie(w, token, 7*24*60*60)
	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
