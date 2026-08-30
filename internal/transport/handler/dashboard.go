package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type DashboardHandler struct {
	svc port.DashboardService
	log *slog.Logger
}

func NewDashboardHandler(svc port.DashboardService, log *slog.Logger) *DashboardHandler {
	return &DashboardHandler{svc: svc, log: log}
}

// @Summary      Get dashboard data
// @Description  Returns personalized dashboard sections (my tasks, due soon, activity, stats, projects) respecting user layout preferences.
// @Tags         dashboard
// @Produce      json
// @Success      200  {object}  dto.DashboardResponse
// @Failure      500  {object}  transport.ErrorResponse
// @Router       /dashboard [get]
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())

	data, err := h.svc.GetDashboard(r.Context(), domain.GetDashboardParams{
		OrgID:  orgID,
		UserID: userID,
		Role:   domain.Role(roleStr),
	})
	if err != nil {
		h.log.Error("get dashboard", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to load dashboard")
		return
	}

	resp, err := dto.NewDashboardResponse(data)
	if err != nil {
		h.log.Error("serialize dashboard", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to serialize dashboard")
		return
	}

	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary      Update dashboard layout
// @Description  Sets the visible sections and their order for the current user's dashboard.
// @Tags         dashboard
// @Accept       json
// @Produce      json
// @Param        body  body  dto.VisibilityRequest  true  "Ordered list of section types"
// @Success      200   {object}  dto.VisibilityResponse
// @Failure      400   {object}  transport.ErrorResponse
// @Failure      500   {object}  transport.ErrorResponse
// @Router       /dashboard/visibility [patch]
func (h *DashboardHandler) UpdateVisibility(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	var req dto.VisibilityRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidRequestBody")
		return
	}
	if len(req.Sections) == 0 {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrSectionsEmpty")
		return
	}

	sections := make([]domain.SectionType, len(req.Sections))
	for i, s := range req.Sections {
		sections[i] = domain.SectionType(s)
	}

	result, err := h.svc.SetVisibility(r.Context(), domain.SetVisibilityParams{
		UserID:   userID,
		OrgID:    orgID,
		Sections: sections,
	})
	if err != nil {
		h.log.Error("set dashboard visibility", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to update dashboard layout")
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewVisibilityResponse(result))
}
