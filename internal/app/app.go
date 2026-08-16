package app

import (
	"fmt"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/config"
	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/password"
	authpostgres "github.com/Kyrapatka/knowledge-platform/internal/auth/repository/postgres"
	authservice "github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/database"
	"github.com/gin-gonic/gin"
)

type App struct {
	router *gin.Engine

	httpAddress string

	closeDatabase func() error
}

func New(cfg config.Config) (*App, error) {
	postgresDB, err := database.OpenPostgres(
		cfg.DatabaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open PostgreSQL: %w",
			err,
		)
	}

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
		tokenManager,
	)

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
	tokenManager *token.JWTManager,
) {
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

	api := a.router.Group("/api/v1")

	authGroup := api.Group("/auth")

	authHandler.RegisterPublicRoutes(
		authGroup,
	)

	protectedAuthGroup := api.Group("/auth")

	protectedAuthGroup.Use(
		authhandler.AuthMiddleware(
			tokenManager,
		),
	)

	authHandler.RegisterProtectedRoutes(
		protectedAuthGroup,
	)
}
