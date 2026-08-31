package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/version"
)

// Pinger is the minimal DB-connectivity contract the health check depends on.
// *sql.DB satisfies it via PingContext, so the handler never imports
// database/sql directly.
type Pinger interface {
	PingContext(ctx context.Context) error
}

type HealthHandler struct {
	db  Pinger
	log *slog.Logger
}

func NewHealthHandler(db Pinger, log *slog.Logger) *HealthHandler {
	return &HealthHandler{db: db, log: log}
}

// @Summary		Health check
// @Description	Returns service + DB health
// @Tags			health
// @Produce		json
// @Success		200	{object}	map[string]string	"service and DB healthy"
// @Failure		503	{object}	map[string]string	"DB unreachable"
// @Router			/healthz [get]
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		h.log.Error("health check db ping failed", "error", err)
		transport.JSON(w, r, http.StatusServiceUnavailable, map[string]string{
			"status": "degraded",
			"db":     "down",
		})
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]string{
		"status": "ok",
		"db":     "ok",
	})
}

// @Summary		Version
// @Description	Returns the running build version
// @Tags			health
// @Produce		json
// @Success		200	{object}	map[string]string	"build version"
// @Router			/api/version [get]
func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	transport.JSON(w, r, http.StatusOK, map[string]string{
		"version": version.Version,
	})
}
