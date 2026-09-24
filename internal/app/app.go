package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/internal/core/dashboard"
	interview "github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	traininghandler "github.com/Kyrapatka/knowledge-platform/internal/core/training/handler"
	trainingpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	trainingservice "github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/Kyrapatka/knowledge-platform/internal/webui"

	"github.com/Kyrapatka/knowledge-platform/config"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/password"
	authpostgres "github.com/Kyrapatka/knowledge-platform/internal/auth/repository/postgres"
	authservice "github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"

	folderhandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/handler"
	folderimporter "github.com/Kyrapatka/knowledge-platform/internal/core/folder/importer"
	folderpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"

	workshophandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop/handler"
	workshopservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop/service"

	materialhandler "github.com/Kyrapatka/knowledge-platform/internal/core/material/handler"
	materialpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/database"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/httpmiddleware"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/metrics"

	"github.com/gin-gonic/gin"
)

type App struct {
	logger    *slog.Logger
	server    *http.Server
	readiness Readiness

	closeDatabase  func() error
	closeAnalytics func() error
}

func New(
	cfg config.Config,
	logger *slog.Logger,
) (*App, error) {
	if logger == nil {
		return nil, errors.New("application logger is required")
	}
	postgresDB, err := database.OpenPostgres(
		cfg.DatabaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open PostgreSQL: %w",
			err,
		)
	}
	logger.Info("database connected")

	// --------------------
	// Auth
	// --------------------

	userRepository := authpostgres.NewUserRepository(
		postgresDB.GORM,
	)

	sessionRepository := authpostgres.NewSessionRepository(
		postgresDB.GORM,
	)

	passwordHasher := password.NewArgon2id()

	tokenManager, err := token.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTIssuer,
		cfg.AccessTokenTTL,
	)
	if err != nil {
		startupErr := fmt.Errorf("create token manager: %w", err)
		if closeErr := postgresDB.Close(); closeErr != nil {
			return nil, errors.Join(startupErr, fmt.Errorf("close database after startup failure: %w", closeErr))
		}
		logger.Info("database closed")
		return nil, startupErr
	}

	observability := metrics.New()
	publisher, closeAnalytics, err := startAnalytics(cfg.Analytics, observability, logger)
	if err != nil {
		return nil, errors.Join(err, postgresDB.Close())
	}

	authService := authservice.New(
		userRepository,
		sessionRepository,
		passwordHasher,
		tokenManager,
		cfg.RefreshTokenTTL,
	)
	authService.SetPublisher(publisher)

	authHandler := authhandler.New(
		authService,
	)

	// --------------------
	// Folder
	// --------------------

	folderRepository := folderpostgres.NewRepository(
		postgresDB.GORM,
	)

	templateRegistry := foldertemplate.NewRegistry(
		foldertemplate.DefaultTemplates(),
	)

	folderService := folderservice.NewService(
		folderRepository,
		templateRegistry,
	)
	folderService.SetPublisher(publisher)

	folderHandler := folderhandler.NewHandler(
		folderService,
	)

	importService := folderimporter.NewService(postgresDB.GORM, templateRegistry)
	importService.SetPublisher(publisher)
	folderImportHandler := folderimporter.NewHandler(importService)

	// --------------------
	// Workshop
	// --------------------

	workshopService := workshopservice.NewService(
		folderRepository,
	)

	workshopHandler := workshophandler.NewHandler(
		workshopService,
	)

	// --------------------
	// Material
	// --------------------

	materialRepository := materialpostgres.NewRepository(
		postgresDB.GORM,
	)

	materialService := materialservice.NewService(
		materialRepository,
		folderRepository,
	)
	materialService.SetPublisher(publisher)

	materialHandler := materialhandler.NewHandler(
		materialService,
	)

	// --------------------
	// HTTP
	// --------------------

	router := gin.New()

	router.Use(
		httpmiddleware.RequestID(logger),
		httpmiddleware.AccessLog(logger),
		observability.Middleware(),
		httpmiddleware.Recovery(),
	)
	router.GET("/metrics", gin.WrapH(observability.Handler()))

	application := &App{
		logger:         logger,
		server:         newHTTPServer(cfg.HTTPAddress, router),
		closeDatabase:  postgresDB.Close,
		closeAnalytics: closeAnalytics,
	}

	application.registerRoutes(
		router,
		authHandler,
		folderHandler,
		folderImportHandler,
		workshopHandler,
		materialHandler,
		tokenManager,
	)
	trainingAPI := router.Group("/api/v1")
	trainingAPI.Use(authhandler.AuthMiddleware(tokenManager))
	trainingService := trainingservice.NewService(trainingpostgres.NewRuntimeStore(postgresDB.GORM))
	trainingService.SetPublisher(publisher)
	traininghandler.NewHandler(trainingService).RegisterRoutes(trainingAPI)
	dashboard.NewHandler(postgresDB.GORM).RegisterRoutes(trainingAPI)
	interviewHandler := interview.NewHandler(postgresDB.GORM)
	interviewHandler.SetPublisher(publisher)
	interviewHandler.RegisterRoutes(trainingAPI)
	webui.Register(router, "web/dist")

	// Publish readiness only after DB validation and all route/dependency wiring.
	// New has not exposed the App yet; no shutdown can race this sole enable point.
	application.readiness.SetReady(true)
	return application, nil
}

func (a *App) registerRoutes(
	router *gin.Engine,
	authHandler *authhandler.Handler,
	folderHandler *folderhandler.Handler,
	folderImportHandler *folderimporter.Handler,
	workshopHandler *workshophandler.Handler,
	materialHandler *materialhandler.Handler,
	tokenManager *token.JWTManager,
) {
	// --------------------
	// Health
	// --------------------

	a.registerProbes(router)

	api := router.Group(
		"/api/v1",
	)

	// --------------------
	// Auth
	// --------------------

	authGroup := api.Group(
		"/auth",
	)

	authHandler.RegisterPublicRoutes(
		authGroup,
	)

	protectedAuthGroup := api.Group(
		"/auth",
	)

	protectedAuthGroup.Use(
		authhandler.AuthMiddleware(
			tokenManager,
		),
	)

	authHandler.RegisterProtectedRoutes(
		protectedAuthGroup,
	)

	// --------------------
	// Folders
	// --------------------

	foldersGroup := api.Group(
		"/folders",
	)

	foldersGroup.Use(
		authhandler.AuthMiddleware(
			tokenManager,
		),
	)

	foldersGroup.POST(
		"",
		folderHandler.Create,
	)

	foldersGroup.POST(
		"/import/validate",
		folderImportHandler.Validate,
	)

	foldersGroup.POST(
		"/import",
		folderImportHandler.Import,
	)

	foldersGroup.GET(
		"",
		folderHandler.List,
	)

	foldersGroup.GET(
		"/:folderID",
		folderHandler.GetByID,
	)

	foldersGroup.PATCH(
		"/:folderID",
		folderHandler.Update,
	)

	foldersGroup.DELETE(
		"/:folderID",
		folderHandler.Delete,
	)

	// --------------------
	// Workshop
	// --------------------

	foldersGroup.PATCH(
		"/:folderID/workshop",
		workshopHandler.UpdateConfig,
	)

	// --------------------
	// Materials
	// --------------------

	foldersGroup.POST(
		"/:folderID/materials",
		materialHandler.Create,
	)

	foldersGroup.GET(
		"/:folderID/materials",
		materialHandler.List,
	)

	foldersGroup.GET(
		"/:folderID/materials/:materialID",
		materialHandler.GetByID,
	)

	foldersGroup.PATCH(
		"/:folderID/materials/:materialID",
		materialHandler.Update,
	)

	foldersGroup.DELETE(
		"/:folderID/materials/:materialID",
		materialHandler.Delete,
	)
}
