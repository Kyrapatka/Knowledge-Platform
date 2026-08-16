package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type authServiceFake struct {
	registerFunc func(
		ctx context.Context,
		input service.RegisterInput,
	) (service.AuthResult, error)

	loginFunc func(
		ctx context.Context,
		input service.LoginInput,
	) (service.AuthResult, error)

	refreshFunc func(
		ctx context.Context,
		input service.RefreshInput,
	) (service.AuthResult, error)

	logoutFunc func(
		ctx context.Context,
		input service.LogoutInput,
	) error

	logoutAllFunc func(
		ctx context.Context,
		userID uuid.UUID,
	) error
}

func (f *authServiceFake) Register(
	ctx context.Context,
	input service.RegisterInput,
) (service.AuthResult, error) {
	return f.registerFunc(ctx, input)
}

func (f *authServiceFake) Login(
	ctx context.Context,
	input service.LoginInput,
) (service.AuthResult, error) {
	return f.loginFunc(ctx, input)
}

func (f *authServiceFake) Refresh(
	ctx context.Context,
	input service.RefreshInput,
) (service.AuthResult, error) {
	return f.refreshFunc(ctx, input)
}

func (f *authServiceFake) Logout(
	ctx context.Context,
	input service.LogoutInput,
) error {
	return f.logoutFunc(ctx, input)
}

func (f *authServiceFake) LogoutAll(
	ctx context.Context,
	userID uuid.UUID,
) error {
	return f.logoutAllFunc(ctx, userID)
}

func newTestRouter(
	h *Handler,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	authGroup := router.Group("/auth")

	h.RegisterPublicRoutes(authGroup)

	return router
}

func TestHandler_Refresh_Success(t *testing.T) {
	handler := New(
		&authServiceFake{
			refreshFunc: func(
				_ context.Context,
				input service.RefreshInput,
			) (service.AuthResult, error) {

				if input.RefreshToken != "refresh-token" {
					t.Fatalf(
						"unexpected refresh token: %s",
						input.RefreshToken,
					)
				}

				return service.AuthResult{
					User: model.User{
						ID:       uuid.New(),
						Nickname: "nikita",
					},
					AccessToken:  "new-access",
					RefreshToken: "refresh-token",
					ExpiresAt:    time.Now(),
				}, nil
			},
		},
	)

	router := newTestRouter(handler)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/refresh",
		strings.NewReader(`
		{
			"refresh_token":"refresh-token"
		}
		`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			recorder.Code,
		)
	}
}

func TestHandler_Refresh_InvalidToken(t *testing.T) {
	handler := New(
		&authServiceFake{
			refreshFunc: func(
				context.Context,
				service.RefreshInput,
			) (service.AuthResult, error) {
				return service.AuthResult{},
					service.ErrInvalidRefreshToken
			},
		},
	)

	router := newTestRouter(handler)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/refresh",
		strings.NewReader(`
		{
			"refresh_token":"wrong"
		}
		`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected 401, got %d",
			recorder.Code,
		)
	}
}

func TestHandler_Logout_Success(t *testing.T) {
	handler := New(
		&authServiceFake{
			logoutFunc: func(
				_ context.Context,
				input service.LogoutInput,
			) error {

				if input.RefreshToken != "refresh-token" {
					t.Fatalf(
						"unexpected token: %s",
						input.RefreshToken,
					)
				}

				return nil
			},
		},
	)

	router := newTestRouter(handler)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		strings.NewReader(`
		{
			"refresh_token":"refresh-token"
		}
		`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"expected 204, got %d",
			recorder.Code,
		)
	}
}

func TestHandler_Logout_Error(t *testing.T) {
	handler := New(
		&authServiceFake{
			logoutFunc: func(
				context.Context,
				service.LogoutInput,
			) error {
				return errors.New("logout error")
			},
		},
	)

	router := newTestRouter(handler)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout",
		strings.NewReader(`
		{
			"refresh_token":"refresh-token"
		}
		`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected 500, got %d",
			recorder.Code,
		)
	}
}

func TestHandler_LogoutAll_Success(t *testing.T) {
	userID := uuid.New()

	handler := New(
		&authServiceFake{
			logoutAllFunc: func(
				_ context.Context,
				id uuid.UUID,
			) error {
				if id != userID {
					t.Fatalf(
						"unexpected user ID: got %s, want %s",
						id,
						userID,
					)
				}

				return nil
			},
		},
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	authGroup := router.Group("/auth")

	authGroup.Use(func(c *gin.Context) {
		c.Set(userIDContextKey, userID)
		c.Next()
	})

	handler.RegisterProtectedRoutes(authGroup)

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/logout-all",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"expected 204, got %d",
			recorder.Code,
		)
	}
}
