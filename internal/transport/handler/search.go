package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

type SearchHandler struct {
	svc port.SearchService
	log *slog.Logger
}

func NewSearchHandler(svc port.SearchService, log *slog.Logger) *SearchHandler {
	return &SearchHandler{svc: svc, log: log}
}

// @Summary      Global search
// @Description  Search across projects, tasks, channels, DMs, and members. Empty q returns recent projects/tasks.
// @Tags         search
// @Produce      json
// @Param        q      query    string  false  "Search query (empty returns recent items)"
// @Param        types  query    string  false  "Comma-separated result types (project,task,channel,direct_message,member). Defaults to project,task."
// @Param        limit  query    int     false  "Max results (1-50, default 20)"
// @Param        project_id query string  false  "Narrow task search to one project"
// @Success      200    {object}  dto.SearchResponse
// @Failure      400    {object}  transport.ErrorResponse
// @Router       /search [get]
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())

	q := strings.TrimSpace(r.URL.Query().Get("q"))

	typesStr := r.URL.Query().Get("types")
	var types []domain.SearchType
	if typesStr != "" {
		parts := strings.Split(typesStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			switch p {
			case "project":
				types = append(types, domain.SearchTypeProject)
			case "task":
				types = append(types, domain.SearchTypeTask)
			case "channel":
				types = append(types, domain.SearchTypeChannel)
			case "direct_message":
				types = append(types, domain.SearchTypeDirectMessage)
			case "member":
				types = append(types, domain.SearchTypeMember)
			}
		}
	}

	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidLimit")
			return
		}
	}

	results, err := h.svc.Search(r.Context(), domain.SearchParams{
		OrgID:     orgID,
		UserID:    userID,
		Role:      domain.Role(roleStr),
		Query:     q,
		Limit:     limit,
		Types:     types,
		ProjectID: strings.TrimSpace(r.URL.Query().Get("project_id")),
	})
	if err != nil {
		h.log.Error("search", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "search failed")
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewSearchResponse(results))
}
