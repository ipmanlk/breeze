package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ipmanlk/breeze/internal/config"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/i18n"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/service"
	"ipmanlk/breeze/internal/storage"
	"ipmanlk/breeze/internal/store"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"
	"ipmanlk/breeze/internal/transport"
	handler "ipmanlk/breeze/internal/transport/handler"
	"ipmanlk/breeze/internal/transport/middleware"
	"ipmanlk/breeze/internal/transport/ws"
	"ipmanlk/breeze/internal/voice"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "ipmanlk/breeze/api/swagger"
)

const (
	uploadDirPerm               = 0o755
	maxRequestBodyBytes         = 1 << 20 // 1 MiB; for multipart uploads
	jsonRequestBodyBytes        = 1 << 16 // 64 KiB; for JSON API endpoints (fits 500-item batch/reorder payloads)
	httpReadWriteTimeout        = 15 * time.Second
	httpIdleTimeout             = 60 * time.Second
	pendingCleanupInterval      = 30 * time.Minute
	pendingAttachmentTTL        = 1 * time.Hour
	sessionCleanupInterval      = 1 * time.Hour
	auditCleanupIntervalDefault = 6 * time.Hour

	loginRateLimit                 = 10
	setupRateLimit                 = 5
	passwordResetRateLimit         = 5
	resetAttemptsCleanupInterval   = 10 * time.Minute
	passwordResetConfirmTokenLimit = 3
	inviteAcceptRateLimit          = 10
	rateLimitWindow                = 10 * time.Minute
)

func cors(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			matched := false
			for _, o := range origins {
				if o == origin || (o == "*" && origin != "") {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					matched = true
					break
				}
			}
			// Only allow credentials when an origin matched. Setting
			// Allow-Credentials: true unconditionally (even for requests with
			// no matching origin) is misleading and can confuse browsers.
			if matched {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				// Expose headers the SPA reads from responses.
				w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type App struct {
	cfg               config.Config
	logger            *slog.Logger
	conn              *sql.DB
	http              *http.Server
	wsHub             *ws.Hub
	dueChecker        *service.DueChecker
	authService       *service.AuthService
	pendingRepo       port.PendingAttachmentRepository
	sessionRepo       port.SessionRepository
	auditRepo         port.AuditRepository
	passwordResetRepo port.PasswordResetRepository
	storageBackend    storage.Storage
}

func New(cfg config.Config) (*App, error) {
	// Structured JSON logging in production for parseable/aggregatable logs;
	// text in development for human readability.
	var logHandler slog.Handler
	if cfg.AppEnv == config.EnvProduction {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})
	}
	logger := slog.New(logHandler)

	transport.DefaultSameSite = cfg.AuthCookieSameSite()

	for _, cidr := range cfg.TrustedProxyCIDRs {
		c := strings.TrimSpace(cidr)
		if c == "" {
			continue
		}
		_, cidrNet, err := net.ParseCIDR(c)
		if err != nil {
			logger.Warn("invalid trusted proxy CIDR, skipping", "cidr", c, "error", err)
			continue
		}
		transport.TrustedProxyCIDRs = append(transport.TrustedProxyCIDRs, cidrNet)
	}

	if err := os.MkdirAll(cfg.UploadDir, uploadDirPerm); err != nil {
		return nil, fmt.Errorf("upload directory: %w", err)
	}

	conn, err := store.NewDB(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	if err := migration.RunMigrations(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	logger.Info("migrations complete")

	// Apply a staged database restore if one is pending (staged via
	// POST /api/backup/restore). The swap happens here, before any store uses
	// the connection, so all stores operate on the restored DB. The current
	// DB is backed up to <DBPath>.bak.
	if fi, statErr := os.Stat(cfg.DBPath + ".restore-pending"); statErr == nil && fi.Size() > 0 {
		logger.Info("pending database restore found, applying...", "size", fi.Size())
		conn.Close()
		bakPath := cfg.DBPath + ".bak"

		// Crash-recovery: if a previous swap was interrupted after renaming
		// the live DB to .bak but before moving the staged file into place,
		// the live DB is missing. Restore it from .bak before proceeding so
		// the swap is idempotent and never leaves the server without a DB.
		if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
			if _, bakErr := os.Stat(bakPath); bakErr == nil {
				logger.Warn("live db missing; recovering from .bak before restore", "backup_path", bakPath)
				if err := os.Rename(bakPath, cfg.DBPath); err != nil {
					return nil, fmt.Errorf("recover db from .bak for restore: %w", err)
				}
			}
		}

		os.Remove(bakPath)
		if err := os.Rename(cfg.DBPath, bakPath); err != nil {
			return nil, fmt.Errorf("back up current db for restore: %w", err)
		}
		if err := os.Rename(cfg.DBPath+".restore-pending", cfg.DBPath); err != nil {
			return nil, fmt.Errorf("apply staged restore: %w", err)
		}
		conn, err = store.NewDB(cfg.DBPath)
		if err != nil {
			return nil, fmt.Errorf("re-open db after restore: %w", err)
		}
		if err := migration.RunMigrations(conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("migrate restored db: %w", err)
		}
		logger.Info("restore applied successfully; old db backed up", "backup_path", bakPath)
	}

	queries := sqlc.New(conn)

	orgRepo := store.NewOrgStore(queries, conn)
	userRepo := store.NewUserStore(queries, conn)
	accountRepo := store.NewAccountStore(queries)
	sessionRepo := store.NewSessionStore(queries)
	projectRepo := store.NewProjectStore(queries, conn)
	statusRepo := store.NewTaskStatusStore(queries)
	taskRepo := store.NewTaskStore(queries, conn)
	cycleRepo := store.NewCycleStore(queries, conn)
	attachmentRepo := store.NewAttachmentStore(queries)
	timeEntryRepo := store.NewTimeEntryStore(queries, conn)
	pmRepo := store.NewProjectMemberStore(queries, conn)
	inviteRepo := store.NewInviteStore(queries, conn)

	notifRepo := store.NewNotificationStore(queries)
	notifPrefRepo := store.NewNotificationPreferenceStore(queries)
	userPrefsRepo := store.NewUserPreferencesStore(queries)
	searchRepo := store.NewSearchStore(queries)

	storeBackend := storage.NewLocal(cfg.UploadDir)

	mailer := service.NewMailer(cfg.SMTP, logger)
	webPush := service.NewWebPush(cfg.VAPID, logger)
	pushRepo := store.NewPushSubscriptionStore(queries)
	pushService := service.NewPushService(pushRepo, webPush, logger)

	// i18n: embedded message bundle (mail/push/errors). Constructed once;
	// NewLocalizer is cheap so per-operation localizers are built at send time.
	i18nBundle := i18n.NewBundle()
	transport.SetI18nBundle(i18nBundle)

	passwordResetRepo := store.NewPasswordResetStore(queries)
	authService := service.NewAuthService(accountRepo, userRepo, sessionRepo, passwordResetRepo, mailer, cfg.SMTP.AppURL, cfg.JWTSecret, i18nBundle)
	orgService := service.NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, cfg.JWTSecret)
	userService := service.NewUserService(userRepo, sessionRepo, accountRepo, storeBackend)
	inviteService := service.NewInviteService(userRepo, accountRepo, inviteRepo, pmRepo, orgRepo, projectRepo, mailer, cfg.SMTP.AppURL, i18nBundle)

	wsHub := ws.NewHubWithLimits(logger, cfg.WebSocket.MaxConnectionsPerUser, cfg.WebSocket.MaxConnectionsGlobal)
	go wsHub.Run()

	notifService := service.NewNotificationService(notifRepo, notifPrefRepo, userPrefsRepo, userRepo, wsHub, mailer, pushService, i18nBundle, logger)

	viewRepo := store.NewViewStore(queries)
	convRepo := store.NewConversationStore(queries, conn)

	accessChecker := service.NewAccessChecker(projectRepo, userRepo, pmRepo, taskRepo)

	viewService := service.NewViewService(viewRepo, accessChecker)
	projectService := service.NewProjectService(projectRepo, statusRepo, viewRepo)
	taskService := service.NewTaskServiceWithDeps(service.TaskServiceDeps{
		TaskRepo:     taskRepo,
		ProjRepo:     projectRepo,
		StatusRepo:   statusRepo,
		CycleRepo:    cycleRepo,
		NotifSvc:     notifService,
		UserRepo:     userRepo,
		ConvRepo:     convRepo,
		Broadcaster:  wsHub,
		ActivityRepo: store.NewTaskActivityStore(queries),
		Access:       accessChecker,
	})
	statusService := service.NewTaskStatusService(statusRepo, accessChecker)
	cycleService := service.NewCycleService(cycleRepo, projectRepo, taskRepo, accessChecker)
	activityStore := store.NewTaskActivityStore(queries)
	attachmentService := service.NewAttachmentService(attachmentRepo, taskRepo, storeBackend, accessChecker, activityStore, wsHub)
	timeEntryService := service.NewTimeEntryService(timeEntryRepo, taskRepo, accessChecker, activityStore, wsHub)
	pmService := service.NewProjectMemberService(pmRepo, userRepo)
	userPrefsService := service.NewUserPreferencesService(userPrefsRepo)

	msgRepo := store.NewMessageStore(queries)
	msgAttRepo := store.NewMessageAttachmentStore(queries)
	pendingAttRepo := store.NewPendingAttachmentStore(queries)
	reactionRepo := store.NewReactionStore(queries)
	presenceRepo := store.NewPresenceStore(queries)
	prefRepo := store.NewUserChannelPreferenceStore(queries)
	linkRepo := store.NewChannelProjectLinkStore(queries, conn)
	permRepo := store.NewChannelPermissionStore(queries)

	presenceService := service.NewPresenceService(presenceRepo, wsHub)
	permService := service.NewChannelPermissionService(permRepo, convRepo, linkRepo, userRepo)

	// AccessService consolidates authorization checks that were previously
	// scattered across handler-injected repositories.
	accessService := service.NewAccessService(pmRepo, projectRepo, permService, convRepo)

	conversationService := service.NewConversationService(service.ConversationServiceDeps{
		ConvRepo:       convRepo,
		UserRepo:       userRepo,
		MsgRepo:        msgRepo,
		PrefRepo:       prefRepo,
		LinkRepo:       linkRepo,
		PermRepo:       permRepo,
		NotifSvc:       notifService,
		ChannelPermSvc: permService,
		Broadcaster:    wsHub,
		Logger:         logger,
	})
	messageService := service.NewMessageService(service.MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            orgRepo,
		ProjectRepo:        projectRepo,
		TaskRepo:           taskRepo,
		UserRepo:           userRepo,
		AttRepo:            msgAttRepo,
		PendingAttRepo:     pendingAttRepo,
		ReactionRepo:       reactionRepo,
		PrefRepo:           prefRepo,
		NotifSvc:           notifService,
		ChannelPermService: permService,
		Broadcaster:        wsHub,
		UserPrefRepo:       userPrefsRepo,
		I18n:               i18nBundle,
		Log:                logger,
	})
	mentionSearchRepo := store.NewMentionSearchStore(queries)
	mentionService := service.NewMentionService(mentionSearchRepo, convRepo)
	searchService := service.NewSearchService(searchRepo)
	labelService := service.NewLabelService(store.NewLabelStore(queries, conn), accessChecker, store.NewTaskActivityStore(queries), taskRepo, wsHub)
	taskDepService := service.NewTaskDependencyService(store.NewTaskDependencyStore(queries), taskRepo, accessChecker, activityStore, wsHub)
	auditRepo := store.NewAuditStore(queries)
	auditService := service.NewAuditService(auditRepo, logger)
	commentService := service.NewCommentService(store.NewCommentStore(queries), taskRepo, projectRepo, convRepo, userRepo, notifService, wsHub, logger, accessChecker, activityStore)
	taskTemplateService := service.NewTaskTemplateService(store.NewTaskTemplateStore(queries, conn), taskRepo, statusRepo, projectRepo, userRepo, logger)
	taskTemplateService.SetTaskService(taskService)
	customFieldService := service.NewCustomFieldService(store.NewCustomFieldStore(queries), projectRepo, accessChecker)
	dashboardRepo := store.NewDashboardStore(queries)
	dashboardService := service.NewDashboardService(dashboardRepo)

	// Voice
	voiceCfg := voice.Config{
		STUNURLs:          cfg.Voice.STUNURLs,
		TurnEnabled:       cfg.Voice.TurnEnabled,
		TurnHost:          cfg.Voice.TurnHost,
		TurnPort:          cfg.Voice.TurnPort,
		TurnUser:          cfg.Voice.TurnUser,
		TurnPass:          cfg.Voice.TurnPass,
		TurnSecret:        cfg.Voice.TurnSecret,
		TurnCredentialTTL: cfg.Voice.TurnCredentialTTL,
		TurnURLs:          cfg.Voice.TurnURLs,
		MaxParticipants:   cfg.Voice.MaxParticipants,
	}
	voiceEngine, err := voice.NewEngine(voiceCfg, logger)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("voice engine: %w", err)
	}
	sfu := voice.NewSFU(voiceEngine, logger)
	voiceRepo := store.NewVoiceParticipantStore(queries)
	// Crash recovery: voice_participants rows describe live WebRTC sessions,
	// but after an unclean shutdown every connection is gone. Sweep the table
	// before any client can connect so ghost participants never appear.
	if n, err := voiceRepo.DeleteAll(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sweep stale voice participants: %w", err)
	} else if n > 0 {
		logger.Info("swept stale voice participants from previous run", "count", n)
	}
	voiceService := service.NewVoiceService(service.VoiceServiceDeps{
		ParticipantRepo: voiceRepo,
		ConvRepo:        convRepo,
		UserRepo:        userRepo,
		PermService:     permService,
		SFU:             sfu,
		Broadcaster:     wsHub,
		Log:             logger,
	})
	voiceHandler := handler.NewVoiceHandler(voiceService, accessService, logger)

	wsHandler := handler.NewWsHandler(wsHub, presenceService, presenceRepo, voiceService, cfg.CORSOrigins, logger)
	wsHandler.SetAccessChecker(handler.NewWSRoomAccessChecker(permService, convRepo, pmRepo, projectRepo, logger))
	wsHandler.SetSessionValidator(authService)
	wsHandler.SetTypingDebounce(cfg.WebSocket.TypingDebounce)

	authHandler := handler.NewAuthHandler(authService, userService, orgService, logger)
	setupHandler := handler.NewSetupHandler(orgService, authService, logger)
	healthHandler := handler.NewHealthHandler(conn, logger)
	workspaceHandler := handler.NewWorkspaceHandler(orgService, userRepo, logger)
	orgHandler := handler.NewOrganizationHandler(orgService, logger)
	accountHandler := handler.NewAccountHandler(userService, logger)
	avatarHandler := handler.NewAvatarHandler(userRepo, storeBackend, logger)
	projectHandler := handler.NewProjectHandler(projectService, accessService, auditService, logger)
	statusHandler := handler.NewTaskStatusHandler(statusService, accessService, logger)
	taskHandler := handler.NewTaskHandler(taskService, accessService, logger)
	taskHandler.SetTemplateService(taskTemplateService)
	taskHandler.SetAuditService(auditService)
	cycleHandler := handler.NewCycleHandler(cycleService, accessService, logger)
	attachmentHandler := handler.NewAttachmentHandler(attachmentService, accessService, logger)
	userHandler := handler.NewUserHandler(userService, pmService, auditService, logger)
	inviteHandler := handler.NewInviteHandler(inviteService, auditService, logger)
	timeHandler := handler.NewTimeEntryHandler(timeEntryService, accessService, logger)
	pmHandler := handler.NewProjectMemberHandler(pmService, accessService, auditService, logger)
	notifHandler := handler.NewNotificationHandler(notifService, logger)
	pushHandler := handler.NewPushHandler(pushService, logger)
	userPrefsHandler := handler.NewUserPreferencesHandler(userPrefsService, logger)
	searchHandler := handler.NewSearchHandler(searchService, logger)
	labelHandler := handler.NewLabelHandler(labelService, logger)
	taskDepHandler := handler.NewTaskDependencyHandler(taskDepService, logger)
	auditHandler := handler.NewAuditHandler(auditService, logger)

	backupService := service.NewBackupService(conn, cfg.DBPath, logger)
	backupHandler := handler.NewBackupHandler(backupService, logger)
	commentHandler := handler.NewCommentHandler(commentService, accessService, logger)
	dashboardHandler := handler.NewDashboardHandler(dashboardService, logger)
	taskTemplateHandler := handler.NewTaskTemplateHandler(taskTemplateService, logger)
	customFieldHandler := handler.NewCustomFieldHandler(customFieldService, logger)
	exportHandler := handler.NewExportHandler(taskService, timeEntryService, accessService, logger)
	viewHandler := handler.NewViewHandler(viewService, projectRepo, accessService, logger)

	convHandler := handler.NewConversationHandler(conversationService, accessService, logger)
	permHandler := handler.NewChannelPermissionHandler(permService, accessService, logger)
	msgHandler := handler.NewMessageHandler(handler.MessageHandlerDeps{
		SVC:            messageService,
		MentionSvc:     mentionService,
		AttRepo:        msgAttRepo,
		PendingAttRepo: pendingAttRepo,
		ReactionRepo:   reactionRepo,
		AccessSvc:      accessService,
		StoreBack:      storeBackend,
		Log:            logger,
	})
	presenceHandler := handler.NewPresenceHandler(presenceService, logger)

	dueChecker := service.NewDueChecker(notifService, logger, 10*time.Minute)

	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	// No chimw.RealIP here: it blindly trusts spoofable proxy headers. IP
	// extraction is handled by transport.ClientIP, which only honors those
	// headers from peers in TrustedProxyCIDRs.
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.CleanPath)
	r.Use(middleware.SecurityHeaders(cfg.AppEnv))

	// Apply CORS when origins are explicitly configured or in development.
	// In production with no CORS_ORIGINS, the server defaults to safe
	// same-origin behavior (no cross-origin headers emitted).
	if cfg.AppEnv == config.EnvDevelopment || os.Getenv("CORS_ORIGINS") != "" {
		for _, o := range cfg.CORSOrigins {
			if o == "*" {
				// A reflected wildcard origin combined with
				// Allow-Credentials: true lets ANY website make authenticated
				// cross-origin requests to this instance. Only acceptable on a
				// trusted network; say so loudly at startup.
				logger.Warn("CORS_ORIGINS contains '*': any origin may send credentialed requests. Restrict it to your SPA's origin in production.")
			}
		}
		r.Use(cors(cfg.CORSOrigins))
	}

	// Resolve the requester's locale from the Accept-Language header and stash
	// it in the request context. Applies to every route so error responses +
	// public endpoints localize too. Authenticated users' stored language
	// preference is resolved later in RequireAuth (which overrides the
	// Accept-Language default with the user's explicit choice).
	r.Use(i18nBundle.LocaleMiddleware(nil))

	r.Use(middleware.RequireSetup(orgRepo, logger))

	// Swagger UI is a developer aid that exposes the full API surface
	// (endpoints, schemas, params). Serve it only in development to avoid
	// information disclosure in production deployments.
	if cfg.AppEnv == config.EnvDevelopment {
		r.Get("/api/swagger/*", httpSwagger.Handler(
			httpSwagger.URL("/api/swagger/doc.json"),
		))
	}

	// Public endpoints get the same JSON body cap as the authenticated API.
	// Without it, io.ReadAll inside rate-limit peeking and JSON decoding
	// would buffer arbitrarily large request bodies into memory from
	// unauthenticated callers. They are also CSRF-checked like the rest of
	// the API: without this, a cross-site form post could log a visitor into
	// an attacker-controlled account (login CSRF) or trigger password-reset
	// emails.
	csrf := middleware.CSRFProtection(cfg.CORSOrigins)

	r.Get("/api/setup", setupHandler.Check)
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitLogin(setupRateLimit, rateLimitWindow)).Post("/api/setup", setupHandler.Setup)
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitLoginByEmail(loginRateLimit, rateLimitWindow)).Post("/api/auth/login", authHandler.Login)

	r.Get("/healthz", healthHandler.Check)
	r.Get("/api/version", healthHandler.Version)

	// Password reset: public (no auth). The request endpoint always returns
	// success to avoid leaking whether an email is registered; the reset link
	// is logged server-side (air-gapped fallback when no SMTP is configured).
	// Rate-limited: 5 requests / 10 min / IP to deter abuse.
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitLogin(passwordResetRateLimit, rateLimitWindow)).Post("/api/auth/password-reset/request", authHandler.RequestPasswordReset)
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitPasswordResetConfirm(passwordResetConfirmTokenLimit, rateLimitWindow)).Post("/api/auth/password-reset/confirm", authHandler.ConfirmPasswordReset)
	// GET for backward compatibility with reset email links; POST preferred
	// for API callers to avoid token leakage in URLs/access logs.
	r.With(middleware.RateLimitLogin(passwordResetRateLimit, rateLimitWindow)).Get("/api/auth/password-reset/validate", authHandler.ValidateResetToken)
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitLogin(passwordResetRateLimit, rateLimitWindow)).Post("/api/auth/password-reset/validate", authHandler.ValidateResetToken)

	// Rate-limited like the other public token endpoints: each hit is a DB
	// lookup keyed by an attacker-supplied token.
	r.With(middleware.RateLimitLogin(passwordResetRateLimit, rateLimitWindow)).Get("/api/invites/{token}/validate", inviteHandler.Validate)
	r.With(csrf, middleware.LimitRequestBody(jsonRequestBodyBytes), middleware.RateLimitLogin(inviteAcceptRateLimit, rateLimitWindow)).Post("/api/invites/{token}/accept", inviteHandler.Accept)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authService, userPrefsRepo, logger))
		r.Use(middleware.CSRFProtection(cfg.CORSOrigins))

		// Upload routes: preserve larger body limit for file uploads.
		// MAX_UPLOAD_SIZE drives the cap consistently across avatars and
		// attachments (default 50 MiB).
		r.Group(func(r chi.Router) {
			r.Use(middleware.LimitRequestBody(cfg.UploadSizeLimit()))

			// Avatar upload
			r.Post("/api/account/avatar", accountHandler.UploadAvatar)

			// Task attachment upload
			r.With(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermAttachmentCreate)).
				With(middleware.RejectArchivedProject(projectRepo)).
				Post("/api/projects/{id}/tasks/{taskId}/attachments", attachmentHandler.Upload)

			// Message attachment upload
			r.With(middleware.RequirePermission(domain.PermChatSend)).
				Post("/api/conversations/{id}/attachments", msgHandler.UploadAttachment)
		})

		// Backup restore needs a larger limit for SQLite database files
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgDelete))
			r.Use(middleware.LimitRequestBody(200 << 20))
			r.Post("/api/backup/restore", backupHandler.StageRestore)
		})

		// Default body limit for JSON API routes
		r.Use(middleware.LimitRequestBody(jsonRequestBodyBytes))

		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/me", authHandler.Me)
		r.Get("/api/auth/sessions", authHandler.ListSessions)
		r.Delete("/api/auth/sessions/{id}", authHandler.RevokeSession)

		// Browser push notifications (Web Push). Any authenticated user manages
		// their own subscriptions. The vapid-public-key endpoint is public to
		// the auth group so the UI can check availability before prompting.
		r.Get("/api/push/vapid-public-key", pushHandler.PublicKey)
		r.Post("/api/push/subscribe", pushHandler.Subscribe)
		r.Delete("/api/push/subscribe", pushHandler.Unsubscribe)

		// Account self-service: any authenticated user manages their own profile,
		// avatar, and password. No org permission required.
		r.Patch("/api/account", accountHandler.UpdateProfile)
		r.With(middleware.RateLimitLogin(5, 10*time.Minute)).Post("/api/account/change-password", accountHandler.ChangePassword)
		r.Get("/api/avatars/{id}", avatarHandler.Get)

		r.Get("/api/workspaces", workspaceHandler.List)
		r.Post("/api/workspaces", workspaceHandler.Create)
		r.Post("/api/workspaces/{id}/switch", workspaceHandler.Switch)

		// Organization settings. Any authenticated member can view their org;
		// rename/edit-window requires PermOrgManage (owner/admin); delete is
		// owner-only (PermOrgDelete).
		r.Get("/api/organization", orgHandler.Get)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgManage))
			r.Patch("/api/organization", orgHandler.Update)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgDelete))
			r.Delete("/api/organization", orgHandler.Delete)
		})

		// Database backup/restore. Download requires PermOrgManage
		// (owner/admin); staging a restore requires PermOrgDelete (owner
		// only; destructive). Checking/cancelling a pending restore
		// requires PermOrgManage.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgManage))
			r.Get("/api/backup/download", backupHandler.Download)
			r.Get("/api/backup/restore/pending", backupHandler.CheckPendingRestore)
			r.Delete("/api/backup/restore/pending", backupHandler.ClearPendingRestore)
		})

		r.Get("/api/ws", wsHandler.Upgrade)

		// Search is gated on PermProjectView so guests (who lack project access)
		// cannot discover task titles across the org. The service additionally
		// scopes task results to projects the caller can access.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectView))
			r.Get("/api/search", searchHandler.Search)
		})

		// Labels CRUD (org-scoped); reads require project/task view access,
		// writes require task:create (members and above).
		// Labels reuse the org-level project-edit permission for simplicity;
		// a dedicated label-management permission could be added in the future.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermTaskView))
			r.Get("/api/labels", labelHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermTaskCreate))
			r.Post("/api/labels", labelHandler.Create)
			r.Patch("/api/labels/{id}", labelHandler.Update)
			r.Delete("/api/labels/{id}", labelHandler.Delete)
		})

		// Dashboard and My Issues are intentionally open to all authenticated
		// users; every user has a personal dashboard and the /api/tasks endpoint
		// scopes results to the caller's user ID. No org-level permission check
		// is needed here; the service handles cross-org protection internally.
		r.Get("/api/dashboard", dashboardHandler.GetDashboard)
		r.Patch("/api/dashboard/visibility", dashboardHandler.UpdateVisibility)
		r.Get("/api/tasks", taskHandler.ListTasks)
		r.Get("/api/attachments/{attachmentId}/download", attachmentHandler.Download)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgMembersView))
			r.Get("/api/users", userHandler.List)
			r.Get("/api/users/{id}", userHandler.Get)
			r.Get("/api/users/{id}/project-memberships", userHandler.ListProjectMemberships)
			r.Get("/api/audit-log", auditHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgMembersManage))
			r.Put("/api/users/{id}/role", userHandler.UpdateRole)
			r.Put("/api/users/{id}/active", userHandler.UpdateActive)
			r.Put("/api/users/{id}/project-memberships", userHandler.UpdateProjectMemberships)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermOrgMembersInvite))
			r.Post("/api/invites", inviteHandler.Create)
			r.Get("/api/invites", inviteHandler.List)
			r.Delete("/api/invites/{id}", inviteHandler.Revoke)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectView))
			r.Get("/api/projects", projectHandler.List)
			r.Get("/api/projects/by-slug/{slug}", projectHandler.GetBySlug)
		})

		// Batch + reorder routes are registered BEFORE the tasks/{taskId} GET route
		// below so chi matches the literal "batch"/"reorder" segments as static
		// paths rather than a {taskId} param (which would 405 on POST). They need
		// PermTaskEdit + reject-archived, applied inline.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskEdit))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/tasks/batch", taskHandler.BatchUpdate)
			r.Post("/api/projects/{id}/tasks/reorder", taskHandler.Reorder)
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
			r.Get("/api/projects/{id}/templates", taskTemplateHandler.List)
			r.Get("/api/projects/{id}/custom-fields", customFieldHandler.List)
			r.Get("/api/projects/{id}/tasks/{taskId}/custom-fields", customFieldHandler.GetTaskValues)
			r.Get("/api/projects/{id}/tasks/export", exportHandler.ExportTasks)
			r.Get("/api/projects/{id}/time-entries/export", exportHandler.ExportTimeEntries)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectView))
			r.Post("/api/views", viewHandler.Create)
			r.Get("/api/views", viewHandler.ListGlobal)
			r.Get("/api/views/pins", viewHandler.ListPinned)
			r.Get("/api/views/{id}", viewHandler.Get)
			r.Patch("/api/views/{id}", viewHandler.Update)
			r.Delete("/api/views/{id}", viewHandler.Delete)
			r.Post("/api/views/{id}/pin", viewHandler.Pin)
			r.Delete("/api/views/{id}/pin", viewHandler.Unpin)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskCreate))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/tasks", taskHandler.Create)
			r.Post("/api/projects/{id}/tasks/{taskId}/duplicate", taskHandler.Duplicate)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskEdit))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Put("/api/projects/{id}/tasks/{taskId}", taskHandler.Update)
			r.Patch("/api/projects/{id}/tasks/{taskId}/position", taskHandler.Move)
			r.Post("/api/projects/{id}/tasks/{taskId}/move", taskHandler.MoveToProject)
			r.Get("/api/projects/{id}/tasks/{taskId}/activity", taskHandler.ListActivity)
			r.Get("/api/projects/{id}/tasks/{taskId}/subtasks", taskHandler.ListSubtasks)
			r.Post("/api/projects/{id}/tasks/{taskId}/subtasks/reorder", taskHandler.ReorderSubtasks)
			r.Post("/api/projects/{id}/tasks/{taskId}/dependencies", taskDepHandler.Add)
			r.Delete("/api/projects/{id}/tasks/{taskId}/dependencies/{blocksTaskId}", taskDepHandler.Remove)
			r.Put("/api/projects/{id}/tasks/{taskId}/labels", labelHandler.SetTaskLabels)
			r.Put("/api/projects/{id}/tasks/{taskId}/custom-fields/{fieldId}", customFieldHandler.SetValue)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskDelete))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Delete("/api/projects/{id}/tasks/{taskId}", taskHandler.Delete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectCreate))
			r.Post("/api/projects", projectHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectManage))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Put("/api/projects/{id}", projectHandler.Update)
			r.Post("/api/projects/{id}/templates", taskTemplateHandler.Create)
			r.Patch("/api/projects/{id}/templates/{templateId}", taskTemplateHandler.Update)
			r.Delete("/api/projects/{id}/templates/{templateId}", taskTemplateHandler.Delete)
			r.Post("/api/projects/{id}/templates/{templateId}/instantiate", taskTemplateHandler.Instantiate)
			r.Post("/api/projects/{id}/custom-fields", customFieldHandler.Create)
			r.Patch("/api/projects/{id}/custom-fields/{fieldId}", customFieldHandler.Update)
			r.Delete("/api/projects/{id}/custom-fields/{fieldId}", customFieldHandler.Delete)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermProjectDelete))
			r.Delete("/api/projects/{id}", projectHandler.Delete)
			r.Post("/api/projects/{id}/archive", projectHandler.Archive)
			r.Post("/api/projects/{id}/unarchive", projectHandler.Unarchive)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectStatusManage))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/statuses", statusHandler.Create)
			r.Put("/api/projects/{id}/statuses/{statusId}", statusHandler.Update)
			r.Delete("/api/projects/{id}/statuses/{statusId}", statusHandler.Delete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectCycleManage))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/cycles", cycleHandler.Create)
			r.Put("/api/projects/{id}/cycles/{cycleId}", cycleHandler.Update)
			r.Delete("/api/projects/{id}/cycles/{cycleId}", cycleHandler.Delete)
			r.Post("/api/projects/{id}/cycles/{cycleId}/activate", cycleHandler.Activate)
			r.Post("/api/projects/{id}/cycles/{cycleId}/complete", cycleHandler.Complete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectMembersManage))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Get("/api/projects/{id}/members", pmHandler.List)
			r.Post("/api/projects/{id}/members", pmHandler.Add)
			r.Put("/api/projects/{id}/members/{userId}", pmHandler.UpdateRole)
			r.Delete("/api/projects/{id}/members/{userId}", pmHandler.Remove)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskView))
			r.Get("/api/projects/{id}/tasks/{taskId}/comments", commentHandler.List)
			r.Get("/api/projects/{id}/tasks/{taskId}/dependencies/blocking", taskDepHandler.ListBlocking)
			r.Get("/api/projects/{id}/tasks/{taskId}/dependencies/blocked", taskDepHandler.ListBlocked)
			r.Get("/api/projects/{id}/tasks/{taskId}/labels", labelHandler.GetTaskLabels)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskCreate))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Post("/api/projects/{id}/tasks/{taskId}/comments", commentHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTaskEdit))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Patch("/api/projects/{id}/tasks/{taskId}/comments/{commentId}", commentHandler.Update)
			r.Delete("/api/projects/{id}/tasks/{taskId}/comments/{commentId}", commentHandler.Delete)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermAttachmentDelete))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Delete("/api/projects/{id}/tasks/{taskId}/attachments/{attachmentId}", attachmentHandler.Delete)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermProjectView))
			r.Get("/api/projects/{id}/tasks/{taskId}/attachments", attachmentHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTimeCreate))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Get("/api/projects/{id}/tasks/{taskId}/time-entries", timeHandler.List)
			r.Post("/api/projects/{id}/tasks/{taskId}/time-entries", timeHandler.Create)
			r.Post("/api/projects/{id}/tasks/{taskId}/time-entries/start", timeHandler.Start)
			r.Post("/api/projects/{id}/tasks/{taskId}/time-entries/stop", timeHandler.Stop)
			r.Put("/api/projects/{id}/tasks/{taskId}/time-entries/{entryId}", timeHandler.Update)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireProjectPermission(pmRepo, projectRepo, domain.PermTimeDelete))
			r.Use(middleware.RejectArchivedProject(projectRepo))
			r.Delete("/api/projects/{id}/tasks/{taskId}/time-entries/{entryId}", timeHandler.Delete)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermNotificationView))
			r.Get("/api/notifications", notifHandler.List)
			r.Get("/api/notifications/unread-count", notifHandler.CountUnread)
			r.Patch("/api/notifications/{id}/read", notifHandler.MarkRead)
			r.Patch("/api/notifications/read-all", notifHandler.MarkAllRead)
			r.Get("/api/settings/notifications", notifHandler.GetPreferences)
			r.Patch("/api/settings/notifications/{type}", notifHandler.SetPreference)
		})

		r.Group(func(r chi.Router) {
			r.Get("/api/settings/preferences", userPrefsHandler.Get)
			r.Patch("/api/settings/preferences", userPrefsHandler.Update)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatRead))
			r.Get("/api/conversations", convHandler.List)
			// Static conversation sub-paths must be registered alongside the
			// parametrised {id} routes. chi v5 prioritises static segments over
			// {id} params, so /conversations/search and /conversations/by-parent
			// resolve correctly even with /conversations/{id} present.
			r.Get("/api/conversations/search", msgHandler.SearchMessages)
			r.Get("/api/conversations/by-parent", convHandler.ListByParent)
			r.Get("/api/conversations/{id}", convHandler.GetByID)
			r.Get("/api/conversations/{id}/members", convHandler.ListMembers)
			r.Get("/api/conversations/{id}/projects", convHandler.GetProjectLinks)
			r.Get("/api/conversations/{id}/access", convHandler.ListAccess)
			r.Get("/api/conversations/{id}/my-permissions", permHandler.ResolvePermissions)
			r.Get("/api/conversations/{id}/permissions", permHandler.GetPermissions)
			r.Get("/api/conversations/{id}/user-overrides", permHandler.GetUserOverrides)
			r.Get("/api/conversations/{id}/voice/participants", voiceHandler.ListParticipants)
			r.Post("/api/conversations/{id}/read", convHandler.MarkRead)
			r.Patch("/api/conversations/{id}/mute", convHandler.SetMuted)
			r.Patch("/api/conversations/{id}/notification-level", convHandler.SetNotificationLevel)
			r.Get("/api/conversations/{id}/pinned", convHandler.PinnedMessages)
			r.Get("/api/conversations/{id}/messages", msgHandler.ListMessages)
			r.Get("/api/conversations/{id}/messages/{msg_id}/replies", msgHandler.ListReplies)
			// Attachment download needs only view access: the handler enforces
			// conversation access itself. Gating it on chat:send would lock
			// viewers/guests out of files they are allowed to see.
			r.Get("/api/conversations/{id}/attachments/{att_id}/download", msgHandler.DownloadAttachment)
			r.Get("/api/mentions/search", msgHandler.SearchMentions)
			r.Get("/api/chat/presence", presenceHandler.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatSend))
			r.Post("/api/conversations/{id}/messages", msgHandler.SendMessage)
			r.Patch("/api/conversations/{id}/messages/{msg_id}", msgHandler.EditMessage)
			r.Delete("/api/conversations/{id}/messages/{msg_id}", msgHandler.DeleteMessage)
			r.Post("/api/conversations/{id}/messages/{msg_id}/pin", msgHandler.PinMessage)
			r.Delete("/api/conversations/{id}/messages/{msg_id}/pin", msgHandler.UnpinMessage)
			r.Post("/api/conversations/{id}/messages/{msg_id}/reactions", msgHandler.AddReaction)
			r.Delete("/api/conversations/{id}/messages/{msg_id}/reactions/{emoji}", msgHandler.RemoveReaction)
			r.Put("/api/chat/presence/me", presenceHandler.SetMe)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatChannelCreate))
			r.Post("/api/conversations", convHandler.Create)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePermission(domain.PermChatChannelManage))
			r.Patch("/api/conversations/{id}", convHandler.Update)
			r.Delete("/api/conversations/{id}", convHandler.Delete)
			r.Patch("/api/conversations/{id}/position", convHandler.UpdatePosition)
			r.Put("/api/conversations/{id}/projects", convHandler.SetProjectLinks)
			r.Put("/api/conversations/{id}/permissions", permHandler.SetPermissions)
			r.Put("/api/conversations/{id}/user-overrides", permHandler.SetUserOverrides)
			r.Post("/api/conversations/{id}/members", convHandler.AddMembers)
			r.Delete("/api/conversations/{id}/members/{user_id}", convHandler.RemoveMember)
		})
	})

	if err := setupUIHandler(r, logger); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ui handler: %w", err)
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  httpReadWriteTimeout,
		WriteTimeout: httpReadWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}

	return &App{
		cfg:               cfg,
		logger:            logger,
		conn:              conn,
		http:              httpServer,
		wsHub:             wsHub,
		dueChecker:        dueChecker,
		authService:       authService,
		pendingRepo:       pendingAttRepo,
		sessionRepo:       sessionRepo,
		auditRepo:         auditRepo,
		passwordResetRepo: passwordResetRepo,
		storageBackend:    storeBackend,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		a.logger.Info("server starting", "port", a.http.Addr)
		if err := a.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("server error", "error", err)
			cancel()
		}
	}()

	go a.dueChecker.Run(ctx)

	// Periodic cleanup of orphaned pending attachments. Tied to the app
	// lifecycle context so it shuts down cleanly on SIGINT/SIGTERM.
	go func() {
		ticker := time.NewTicker(pendingCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stale, err := a.pendingRepo.DeleteOlderThan(ctx, time.Now().Add(-pendingAttachmentTTL))
				if err != nil {
					a.logger.Error("pending attachment cleanup", "error", err)
					continue
				}
				for _, att := range stale {
					if err := a.storageBackend.Delete(ctx, att.StoragePath); err != nil {
						a.logger.Error("delete stale attachment file", "error", err, "path", att.StoragePath)
					}
				}
				if len(stale) > 0 {
					a.logger.Info("cleaned up stale pending attachments", "count", len(stale))
				}
			}
		}
	}()

	// Periodic cleanup of expired sessions.
	go func() {
		ticker := time.NewTicker(sessionCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.sessionRepo.DeleteExpired(ctx); err != nil {
					a.logger.Error("session cleanup", "error", err)
				} else {
					a.logger.Debug("expired session cleanup completed")
				}
			}
		}
	}()

	// Periodic purge of old audit log entries. Only runs when
	// AUDIT_RETENTION is set (>0); otherwise audit entries are kept forever.
	if a.cfg.AuditRetention > 0 {
		interval := a.cfg.AuditCleanupInterval
		if interval <= 0 {
			interval = auditCleanupIntervalDefault
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					cutoff := time.Now().Add(-a.cfg.AuditRetention)
					if n, err := a.auditRepo.DeleteOlderThan(ctx, cutoff); err != nil {
						a.logger.Error("audit retention purge", "error", err)
					} else if n > 0 {
						a.logger.Info("audit retention purge completed", "deleted", n)
					}
				}
			}
		}()
	}

	// Periodic cleanup of password-reset attempt tracking to prevent unbounded
	// map growth under sustained token-guessing attacks.
	go func() {
		ticker := time.NewTicker(resetAttemptsCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.authService.ResetAttemptCleanup()
			}
		}
	}()

	// Periodic purge of consumed and expired password-reset tokens.
	go func() {
		ticker := time.NewTicker(sessionCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.passwordResetRepo.DeleteExpired(ctx); err != nil {
					a.logger.Error("password reset token purge", "error", err)
				} else {
					a.logger.Debug("password reset token purge completed")
				}
			}
		}
	}()

	<-ctx.Done()
	a.logger.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully close WebSocket connections with a "going away" status so
	// clients receive a close frame instead of an abrupt drop.
	// Do this before HTTP shutdown so in-flight WS upgrades finish.
	if a.wsHub != nil {
		a.wsHub.Shutdown()
		select {
		case <-a.wsHub.Done():
		case <-shutdownCtx.Done():
			a.logger.Warn("websocket hub shutdown timed out")
		}
	}

	if err := a.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}

	if err := a.conn.Close(); err != nil {
		return fmt.Errorf("db close: %w", err)
	}

	return nil
}
