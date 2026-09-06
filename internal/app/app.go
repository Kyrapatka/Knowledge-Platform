package app

import (
	"fmt"
	traininghandler "github.com/Kyrapatka/knowledge-platform/internal/core/training/handler"
	trainingpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	trainingservice "github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/config"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/password"
	authpostgres "github.com/Kyrapatka/knowledge-platform/internal/auth/repository/postgres"
	authservice "github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"

	folderhandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/handler"
	folderpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"

	workshophandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop/handler"
	workshopservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop/service"

	materialhandler "github.com/Kyrapatka/knowledge-platform/internal/core/material/handler"
	materialpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/database"

	"github.com/gin-gonic/gin"
)

type App struct {
	router *gin.Engine

	httpAddress string

	closeDatabase func() error
}

func New(
	cfg config.Config,
) (*App, error) {
	postgresDB, err := database.OpenPostgres(
		cfg.DatabaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open PostgreSQL: %w",
			err,
		)
	}

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
		_ = postgresDB.Close()

		return nil, fmt.Errorf(
			"create token manager: %w",
			err,
		)
	}

	authService := authservice.New(
		userRepository,
		sessionRepository,
		passwordHasher,
		tokenManager,
		cfg.RefreshTokenTTL,
	)

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

	folderHandler := folderhandler.NewHandler(
		folderService,
	)

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

	materialHandler := materialhandler.NewHandler(
		materialService,
	)

	// --------------------
	// HTTP
	// --------------------

	router := gin.New()

	router.Use(
		gin.Logger(),
		gin.Recovery(),
	)

	application := &App{
		router:        router,
		httpAddress:   cfg.HTTPAddress,
		closeDatabase: postgresDB.Close,
	}

	application.registerRoutes(
		authHandler,
		folderHandler,
		workshopHandler,
		materialHandler,
		tokenManager,
	)
	trainingAPI := router.Group("/api/v1")
	trainingAPI.Use(authhandler.AuthMiddleware(tokenManager))
	traininghandler.NewHandler(trainingservice.NewService(trainingpostgres.NewRuntimeStore(postgresDB.GORM))).RegisterRoutes(trainingAPI)

	return application, nil
}

func (a *App) Run() error {
	fmt.Printf(
		"HTTP server started on %s\n",
		a.httpAddress,
	)

	return a.router.Run(
		a.httpAddress,
	)
}

func (a *App) Close() error {
	if a.closeDatabase == nil {
		return nil
	}

	return a.closeDatabase()
}

func (a *App) registerRoutes(
	authHandler *authhandler.Handler,
	folderHandler *folderhandler.Handler,
	workshopHandler *workshophandler.Handler,
	materialHandler *materialhandler.Handler,
	tokenManager *token.JWTManager,
) {
	// --------------------
	// Health
	// --------------------

	a.router.GET(
		"/health",
		func(c *gin.Context) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "ok",
				},
			)
		},
	)

	api := a.router.Group(
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
