package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ipmanlk/plume/internal/config"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"
	"ipmanlk/plume/internal/service"
	"ipmanlk/plume/internal/storage"
	"ipmanlk/plume/internal/store"
	"ipmanlk/plume/internal/store/migration"
	"ipmanlk/plume/internal/store/sqlc"
	handler "ipmanlk/plume/internal/transport/handler"
	"ipmanlk/plume/internal/transport/middleware"
	"ipmanlk/plume/internal/transport/ws"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// --- wire-up ---

// e2eApp holds a fully wired HTTP test server backed by a real SQLite database.
type e2eApp struct {
	db     *sql.DB
	server *httptest.Server
	hub    *ws.Hub
}

func newE2EApp(t *testing.T) *e2eApp {
	t.Helper()

	t.Setenv("JWT_SECRET", "e2e-test-secret")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := os.MkdirAll(filepath.Join(tmpDir, "uploads"), 0o755); err != nil {
		t.Fatalf("create upload dir: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	conn, err := store.NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	q := sqlc.New(conn)

	orgRepo := store.NewOrgStore(q, conn)
	userRepo := store.NewUserStore(q, conn)
	sessionRepo := store.NewSessionStore(q)
	projectRepo := store.NewProjectStore(q, conn)
	statusRepo := store.NewTaskStatusStore(q)
	taskRepo := store.NewTaskStore(q, conn)
	cycleRepo := store.NewCycleStore(q, conn)
	notifRepo := store.NewNotificationStore(q)
	notifPrefRepo := store.NewNotificationPreferenceStore(q)
	searchRepo := store.NewSearchStore(q)

	convStore := store.NewConversationStore(q, conn)
	msgStore := store.NewMessageStore(q)
	msgAttachmentStore := store.NewMessageAttachmentStore(q)
	pendingAttachmentStore := store.NewPendingAttachmentStore(q)
	reactionStore := store.NewReactionStore(q)
	prefStore := store.NewUserChannelPreferenceStore(q)
	linkStore := store.NewChannelProjectLinkStore(q, conn)
	permStore := store.NewChannelPermissionStore(q)

	authSvc := service.NewAuthService(store.NewAccountStore(q), userRepo, sessionRepo, store.NewPasswordResetStore(q), service.NewMailer(config.SMTPConfig{}, logger), "", "e2e-test-secret", i18n.NewBundle())
	orgSvc := service.NewOrganizationService(orgRepo, userRepo, store.NewAccountStore(q), sessionRepo, "e2e-test-secret")
	userSvc := service.NewUserService(userRepo, sessionRepo, store.NewAccountStore(q), storage.NewLocal(filepath.Join(tmpDir, "uploads")))

	pmRepo := store.NewProjectMemberStore(q, conn)
	pmSvc := service.NewProjectMemberService(pmRepo, userRepo)
	auditSvc := service.NewAuditService(store.NewAuditStore(q), logger)

	permSvc := service.NewChannelPermissionService(permStore, convStore, linkStore, userRepo)

	// AccessService for handler authorization checks.
	accessService := service.NewAccessService(pmRepo, projectRepo, permSvc, convStore)

	pmHandler := handler.NewProjectMemberHandler(pmSvc, accessService, auditSvc, logger)

	inviteSvc := service.NewInviteService(userRepo, store.NewAccountStore(q), store.NewInviteStore(q, conn), pmRepo, orgRepo, projectRepo, service.NewMailer(config.SMTPConfig{}, logger), "", i18n.NewBundle())

	hub := ws.NewHub(logger)
	go hub.Run()

	notifSvc := service.NewNotificationService(notifRepo, notifPrefRepo, store.NewUserPreferencesStore(q), userRepo, hub, service.NewMailer(config.SMTPConfig{}, logger), service.NewPushService(store.NewPushSubscriptionStore(q), service.NewWebPush(config.VAPIDConfig{}, logger), logger), i18n.NewBundle(), logger)
	viewRepo := store.NewViewStore(q)
	projectSvc := service.NewProjectService(projectRepo, statusRepo, viewRepo)
	taskSvc := service.NewTaskService(taskRepo, projectRepo, statusRepo, cycleRepo, notifSvc, userRepo, convStore)
	statusSvc := service.NewTaskStatusService(statusRepo, nil)
	cycleSvc := service.NewCycleService(cycleRepo, projectRepo, taskRepo, nil)
	searchSvc := service.NewSearchService(searchRepo)
	dashRepo := store.NewDashboardStore(q)
	dashSvc := service.NewDashboardService(dashRepo)

	setupHandler := handler.NewSetupHandler(orgSvc, authSvc, logger)
	authHandler := handler.NewAuthHandler(authSvc, userSvc, orgSvc, logger)
	projectHandler := handler.NewProjectHandler(projectSvc, accessService, auditSvc, logger)
	statusHandler := handler.NewTaskStatusHandler(statusSvc, accessService, logger)
	taskHandler := handler.NewTaskHandler(taskSvc, accessService, logger)
	cycleHandler := handler.NewCycleHandler(cycleSvc, accessService, logger)
	userHandler := handler.NewUserHandler(userSvc, pmSvc, auditSvc, logger)
	inviteHandler := handler.NewInviteHandler(inviteSvc, auditSvc, logger)
	notifHandler := handler.NewNotificationHandler(notifSvc, logger)
	searchHandler := handler.NewSearchHandler(searchSvc, logger)
	viewSvc := service.NewViewService(viewRepo, nil)
	viewHandler := handler.NewViewHandler(viewSvc, projectRepo, accessService, logger)
	dashSvc = service.NewDashboardService(dashRepo)
	dashHandler := handler.NewDashboardHandler(dashSvc, logger)
	auditHandler := handler.NewAuditHandler(auditSvc, logger)

	conversationSvc := service.NewConversationService(service.ConversationServiceDeps{
		ConvRepo:    convStore,
		UserRepo:    userRepo,
		MsgRepo:     msgStore,
		PrefRepo:    prefStore,
		LinkRepo:    linkStore,
		PermRepo:    permStore,
		NotifSvc:    notifSvc,
		Broadcaster: hub,
		Logger:      logger,
	})
	mentionSvc := service.NewMentionService(store.NewMentionSearchStore(q), convStore)
	messageSvc := service.NewMessageService(service.MessageServiceDeps{
		MsgRepo:        msgStore,
		ConvRepo:       convStore,
		OrgRepo:        orgRepo,
		ProjectRepo:    projectRepo,
		TaskRepo:       taskRepo,
		UserRepo:       userRepo,
		AttRepo:        msgAttachmentStore,
		PendingAttRepo: pendingAttachmentStore,
		ReactionRepo:   reactionStore,
		PrefRepo:       prefStore,
		NotifSvc:       notifSvc,
		Broadcaster:    hub,
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            logger,
	})

	convHandler := handler.NewConversationHandler(conversationSvc, accessService, logger)
	permHandler := handler.NewChannelPermissionHandler(permSvc, accessService, logger)
	commentSvc := service.NewCommentService(store.NewCommentStore(q), taskRepo, projectRepo, convStore, userRepo, notifSvc, hub, logger, nil, nil)
	commentHandler := handler.NewCommentHandler(commentSvc, accessService, logger)
	msgHandler := handler.NewMessageHandler(handler.MessageHandlerDeps{
		SVC:            messageSvc,
		MentionSvc:     mentionSvc,
		AttRepo:        msgAttachmentStore,
		PendingAttRepo: pendingAttachmentStore,
		ReactionRepo:   reactionStore,
		AccessSvc:      accessService,
		StoreBack:      storage.NewLocal(tmpDir),
		Log:            logger,
	})
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.CleanPath)
	r.Use(middleware.RequireSetup(orgRepo, logger))

	r.Get("/api/setup", setupHandler.Check)
	r.Post("/api/setup", setupHandler.Setup)
	r.Post("/api/auth/login", authHandler.Login)

	r.Get("/api/invites/{token}/validate", inviteHandler.Validate)
	r.Post("/api/invites/{token}/accept", inviteHandler.Accept)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authSvc, store.NewUserPreferencesStore(q), logger))
		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/me", authHandler.Me)
		r.Get("/api/users", userHandler.List)
		r.Get("/api/users/{id}", userHandler.Get)
		r.Get("/api/audit-log", auditHandler.List)
		r.Put("/api/users/{id}/role", userHandler.UpdateRole)
		r.Put("/api/users/{id}/active", userHandler.UpdateActive)
		r.Post("/api/invites", inviteHandler.Create)
		r.Get("/api/invites", inviteHandler.List)
		r.Delete("/api/invites/{id}", inviteHandler.Revoke)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectView))
			r.Get("/api/projects", projectHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectView))
			r.Get("/api/projects/{id}", projectHandler.Get)
			r.Get("/api/projects/{id}/my-access", projectHandler.MyAccess)
			r.Get("/api/projects/{id}/tasks", taskHandler.List)
			r.Get("/api/projects/{id}/tasks/{taskId}", taskHandler.Get)
			r.Get("/api/projects/{id}/statuses", statusHandler.List)
			r.Get("/api/projects/{id}/cycles", cycleHandler.List)
			r.Get("/api/projects/{id}/cycles/active", cycleHandler.GetActive)
			r.Get("/api/projects/{id}/views", viewHandler.ListByProject)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskCreate))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/tasks", taskHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskEdit))
			// Literal sub-paths before {taskId} so chi matches them as static.
			r.Post("/api/projects/{id}/tasks/batch", taskHandler.BatchUpdate)
			r.Post("/api/projects/{id}/tasks/reorder", taskHandler.Reorder)
			r.Put("/api/projects/{id}/tasks/{taskId}", taskHandler.Update)
			r.Patch("/api/projects/{id}/tasks/{taskId}/position", taskHandler.Move)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskDelete))
			r.Delete("/api/projects/{id}/tasks/{taskId}", taskHandler.Delete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectCreate))
			r.Post("/api/projects", projectHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectManage))
			r.Put("/api/projects/{id}", projectHandler.Update)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectDelete))
			r.Delete("/api/projects/{id}", projectHandler.Delete)
			r.Post("/api/projects/{id}/archive", projectHandler.Archive)
			r.Post("/api/projects/{id}/unarchive", projectHandler.Unarchive)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectStatusManage))
			r.Post("/api/projects/{id}/statuses", statusHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectCycleManage))
			r.Post("/api/projects/{id}/cycles", cycleHandler.Create)
			r.Put("/api/projects/{id}/cycles/{cycleId}", cycleHandler.Update)
			r.Post("/api/projects/{id}/cycles/{cycleId}/activate", cycleHandler.Activate)
			r.Post("/api/projects/{id}/cycles/{cycleId}/complete", cycleHandler.Complete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectMembersManage))
			r.Get("/api/projects/{id}/members", pmHandler.List)
			r.Post("/api/projects/{id}/members", pmHandler.Add)
			r.Put("/api/projects/{id}/members/{userId}", pmHandler.UpdateRole)
			r.Delete("/api/projects/{id}/members/{userId}", pmHandler.Remove)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskView))
			r.Get("/api/projects/{id}/tasks/{taskId}/comments", commentHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskCreate))
			r.Post("/api/projects/{id}/tasks/{taskId}/comments", commentHandler.Create)
			r.Patch("/api/projects/{id}/tasks/{taskId}/comments/{commentId}", commentHandler.Update)
			r.Delete("/api/projects/{id}/tasks/{taskId}/comments/{commentId}", commentHandler.Delete)
		})

		r.Get("/api/notifications", notifHandler.List)
		r.Get("/api/notifications/unread-count", notifHandler.CountUnread)
		r.Patch("/api/notifications/{id}/read", notifHandler.MarkRead)
		r.Patch("/api/notifications/read-all", notifHandler.MarkAllRead)
		r.Get("/api/settings/notifications", notifHandler.GetPreferences)
		r.Patch("/api/settings/notifications/{type}", notifHandler.SetPreference)

		r.Get("/api/dashboard", dashHandler.GetDashboard)
		r.Patch("/api/dashboard/visibility", dashHandler.UpdateVisibility)

		r.Get("/api/tasks", taskHandler.ListTasks)

		r.Post("/api/views", viewHandler.Create)
		r.Get("/api/views", viewHandler.ListGlobal)
		r.Get("/api/views/pins", viewHandler.ListPinned)
		r.Get("/api/views/{id}", viewHandler.Get)
		r.Patch("/api/views/{id}", viewHandler.Update)
		r.Delete("/api/views/{id}", viewHandler.Delete)
		r.Post("/api/views/{id}/pin", viewHandler.Pin)
		r.Delete("/api/views/{id}/pin", viewHandler.Unpin)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatRead))
			r.Get("/api/conversations", convHandler.List)
			r.Get("/api/conversations/{id}", convHandler.GetByID)
			r.Get("/api/conversations/{id}/messages", msgHandler.ListMessages)
			r.Get("/api/conversations/{id}/my-permissions", permHandler.ResolvePermissions)
			r.Get("/api/conversations/{id}/members", convHandler.ListMembers)
			r.Get("/api/conversations/{id}/pinned", convHandler.PinnedMessages)
			r.Get("/api/conversations/{id}/access", convHandler.ListAccess)
			r.Post("/api/conversations/{id}/read", convHandler.MarkRead)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatSend))
			r.Post("/api/conversations/{id}/messages", msgHandler.SendMessage)
			r.Post("/api/conversations/{id}/attachments", msgHandler.UploadAttachment)
			r.Get("/api/conversations/{id}/attachments/{att_id}/download", msgHandler.DownloadAttachment)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatChannelCreate))
			r.Post("/api/conversations", convHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatChannelManage))
			r.Patch("/api/conversations/{id}", convHandler.Update)
			r.Delete("/api/conversations/{id}", convHandler.Delete)
			r.Put("/api/conversations/{id}/projects", convHandler.SetProjectLinks)
			r.Post("/api/conversations/{id}/members", convHandler.AddMembers)
		})

		// Search is gated on PermProjectView so guests can't discover task titles.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectView))
			r.Get("/api/search", searchHandler.Search)
		})
	})

	server := httptest.NewServer(r)
	t.Cleanup(func() { server.Close() })

	return &e2eApp{db: conn, server: server, hub: hub}
}

func (a *e2eApp) URL(path string) string { return a.server.URL + path }

// --- typed response structs matching server DTOs ---

type projectResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type taskResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	StatusID string `json:"status_id"`
}

type taskStatusResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cycleResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type unreadCountResponse struct {
	Count int `json:"count"`
}

type notifPrefResponse struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type setupCheckResponse struct {
	NeedsSetup bool `json:"needs_setup"`
}

type conversationResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	ProjectIDs []string `json:"project_ids,omitempty"`
}

type conversationListResponse struct {
	Items      []conversationResponse `json:"items"`
	NextCursor string                 `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}

type messageResponse struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	AuthorID string `json:"author_id"`
}

type messageListResponse struct {
	Items      []messageResponse `json:"items"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// --- helpers ---

func loginCookie(t *testing.T, app *e2eApp) string {
	t.Helper()
	return loginAs(t, app, "admin@test.com", "admin123")
}

func loginAs(t *testing.T, app *e2eApp, email, password string) string {
	t.Helper()
	resp := doJSON(t, http.MethodPost, app.URL("/api/auth/login"), map[string]string{
		"email": email, "password": password,
	}, "")
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "login")
	for _, c := range resp.Cookies() {
		if c.Name == "__Host-token" && c.Value != "" {
			return "__Host-token=" + c.Value
		}
	}
	t.Fatal("login returned no token cookie")
	return ""
}

func setupOrg(t *testing.T, app *e2eApp) {
	t.Helper()
	resp := doJSON(t, http.MethodPost, app.URL("/api/setup"), map[string]string{
		"org_name": "Test Org",
		"name":     "Admin",
		"email":    "admin@test.com",
		"password": "admin123",
	}, "")
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "setupOrg")
}

func doJSON(t *testing.T, method, url string, body any, cookie string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func readBodyStr(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func readBodyJSON(t *testing.T, resp *http.Response, into any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func requireStatus(t *testing.T, resp *http.Response, want int, context string) {
	t.Helper()
	if resp.StatusCode == want {
		return
	}
	b, _ := io.ReadAll(resp.Body)
	t.Fatalf("%s: expected %d, got %d (%s)", context, want, resp.StatusCode, string(b))
}
