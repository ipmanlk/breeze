package handler

import (
	"encoding/csv"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

// sanitizeCSVCell neutralizes CSV/ spreadsheet formula injection. Spreadsheet
// apps (Excel, LibreOffice, Google Sheets) treat a leading =, +, -, @, tab, or
// carriage return as the start of a formula. Prefixing such cells with a single
// quote forces them to be treated as text. See OWASP CSV injection guidance.
func sanitizeCSVCell(s string) string {
	if len(s) > 0 && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

type ExportHandler struct {
	taskSvc      port.TaskService
	timeEntrySvc port.TimeEntryService
	accessSvc    port.AccessService

	log *slog.Logger
}

func NewExportHandler(
	taskSvc port.TaskService,
	timeEntrySvc port.TimeEntryService,
	accessSvc port.AccessService,
	log *slog.Logger,
) *ExportHandler {
	return &ExportHandler{
		taskSvc:      taskSvc,
		timeEntrySvc: timeEntrySvc,
		accessSvc:    accessSvc,
		log:          log,
	}
}

// @Summary		Export project tasks
// @Description	Exports all tasks in a project as CSV or JSON
// @Tags			export
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			format	query	string	false	"Export format: csv or json (default json)"
// @Success		200
// @Router			/projects/{id}/tasks/export [get]
func (h *ExportHandler) ExportTasks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	tasks, err := h.taskSvc.List(r.Context(), orgID, projectID, domain.TaskFilter{})
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	rows := make([][]string, len(tasks))
	for i, t := range tasks {
		assignees := ""
		for j, a := range t.Assignees {
			if j > 0 {
				assignees += ", "
			}
			assignees += a.ID
		}
		estimate := ""
		if t.Estimate != nil {
			estimate = strconv.Itoa(*t.Estimate)
		}
		startedAt := ""
		if t.StartedAt != nil {
			startedAt = t.StartedAt.Format("2006-01-02 15:04:05")
		}
		dueAt := ""
		if t.DueAt != nil {
			dueAt = t.DueAt.Format("2006-01-02 15:04:05")
		}
		completedAt := ""
		if t.CompletedAt != nil {
			completedAt = t.CompletedAt.Format("2006-01-02 15:04:05")
		}
		rows[i] = []string{
			sanitizeCSVCell(t.ID), sanitizeCSVCell(t.Title), sanitizeCSVCell(t.Description), sanitizeCSVCell(t.StatusID), sanitizeCSVCell(t.Priority),
			startedAt, dueAt, completedAt,
			estimate,
			sanitizeCSVCell(assignees),
			t.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="tasks.csv"`)
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "title", "description", "status_id", "priority", "started_at", "due_at", "completed_at", "estimate", "assignees", "created_at"})
		for _, row := range rows {
			cw.Write(row)
		}
		cw.Flush()
	case "json":
		resp := make([]*dto.TaskResponse, len(tasks))
		for i, t := range tasks {
			resp[i] = dto.NewTaskResponse(t)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="tasks.json"`)
		transport.JSON(w, r, http.StatusOK, resp)
	default:
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid_format", "ErrInvalidExportFormat")
	}
}

// @Summary		Export time entries
// @Description	Exports time entries for a project's tasks as CSV or JSON
// @Tags			export
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			format	query	string	false	"Export format: csv or json (default json)"
// @Success		200
// @Router			/projects/{id}/time-entries/export [get]
func (h *ExportHandler) ExportTimeEntries(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	// List all tasks, then collect time entries for each
	tasks, err := h.taskSvc.List(r.Context(), orgID, projectID, domain.TaskFilter{})
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	var allEntries []TimeEntryExportRow
	for _, t := range tasks {
		entries, err := h.timeEntrySvc.List(r.Context(), orgID, t.ID, projectID)
		if err != nil {
			h.log.Error("export time entries: skipping task", "task_id", t.ID, "error", err)
			continue
		}
		for _, e := range entries {
			row := TimeEntryExportRow{
				TaskID:      e.TaskID,
				TaskTitle:   t.Title,
				UserID:      e.UserID,
				Description: e.Description,
				StartedAt:   e.StartedAt,
			}
			if e.EndedAt != nil {
				row.EndedAt = e.EndedAt.Format("2006-01-02 15:04:05")
			}
			if e.DurationMinutes != nil {
				row.DurationMinutes = *e.DurationMinutes
			}
			allEntries = append(allEntries, row)
		}
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="time-entries.csv"`)
		cw := csv.NewWriter(w)
		cw.Write([]string{"task_id", "task_title", "user_id", "description", "started_at", "ended_at", "duration_minutes"})
		for _, e := range allEntries {
			cw.Write([]string{
				sanitizeCSVCell(e.TaskID), sanitizeCSVCell(e.TaskTitle), sanitizeCSVCell(e.UserID), sanitizeCSVCell(e.Description),
				e.StartedAt.Format("2006-01-02 15:04:05"),
				e.EndedAt,
				strconv.Itoa(e.DurationMinutes),
			})
		}
		cw.Flush()
	case "json":
		w.Header().Set("Content-Disposition", `attachment; filename="time-entries.json"`)
		transport.JSON(w, r, http.StatusOK, allEntries)
	default:
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid_format", "ErrInvalidExportFormat")
	}
}

type TimeEntryExportRow struct {
	TaskID          string    `json:"task_id"`
	TaskTitle       string    `json:"task_title"`
	UserID          string    `json:"user_id"`
	Description     string    `json:"description"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         string    `json:"ended_at,omitempty"`
	DurationMinutes int       `json:"duration_minutes,omitempty"`
}
