package handler

import (
	"context"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthService interface {
	Register(
		ctx context.Context,
		input service.RegisterInput,
	) (service.AuthResult, error)

	Login(
		ctx context.Context,
		input service.LoginInput,
	) (service.AuthResult, error)

	Refresh(
		ctx context.Context,
		input service.RefreshInput,
	) (service.AuthResult, error)

	Logout(
		ctx context.Context,
		input service.LogoutInput,
	) error

	LogoutAll(
		ctx context.Context,
		userID uuid.UUID,
	) error
}

type Handler struct {
	service AuthService
}

func New(service AuthService) *Handler {
	if service == nil {
		panic("auth handler: service is nil")
	}

	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterPublicRoutes(
	router *gin.RouterGroup,
) {
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.POST("/refresh", h.Refresh)
	router.POST("/logout", h.Logout)
}

func (h *Handler) RegisterProtectedRoutes(
	router *gin.RouterGroup,
) {
	router.POST("/logout-all", h.LogoutAll)
}
