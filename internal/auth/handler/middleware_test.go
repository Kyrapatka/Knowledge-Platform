package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errParseAccessToken = errors.New("parse access token error")

type accessTokenParserFake struct {
	parseFunc func(
		ctx context.Context,
		tokenValue string,
	) (token.AccessTokenClaims, error)
}

func (f *accessTokenParserFake) ParseAccessToken(
	ctx context.Context,
	tokenValue string,
) (token.AccessTokenClaims, error) {
	return f.parseFunc(ctx, tokenValue)
}

func TestAuthMiddleware_Success(t *testing.T) {
	userID := uuid.New()

	parser := &accessTokenParserFake{
		parseFunc: func(
			_ context.Context,
			tokenValue string,
		) (token.AccessTokenClaims, error) {
			if tokenValue != "valid-token" {
				t.Fatalf(
					"unexpected token: %q",
					tokenValue,
				)
			}

			return token.AccessTokenClaims{
				UserID: userID,
			}, nil
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			value, exists := c.Get(userIDContextKey)
			if !exists {
				t.Fatal("user ID was not added to context")
			}

			contextUserID, ok := value.(uuid.UUID)
			if !ok {
				t.Fatalf(
					"unexpected user ID type: %T",
					value,
				)
			}

			if contextUserID != userID {
				t.Fatalf(
					"unexpected user ID: got %s, want %s",
					contextUserID,
					userID,
				)
			}

			c.Status(http.StatusNoContent)
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer valid-token",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status 204, got %d",
			recorder.Code,
		)
	}
}

func TestAuthMiddleware_MissingAuthorizationHeader(
	t *testing.T,
) {
	parser := &accessTokenParserFake{
		parseFunc: func(
			context.Context,
			string,
		) (token.AccessTokenClaims, error) {
			t.Fatal("parser must not be called")

			return token.AccessTokenClaims{}, nil
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			t.Fatal("protected handler must not be called")
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status 401, got %d",
			recorder.Code,
		)
	}
}

func TestAuthMiddleware_InvalidAuthorizationScheme(
	t *testing.T,
) {
	parser := &accessTokenParserFake{
		parseFunc: func(
			context.Context,
			string,
		) (token.AccessTokenClaims, error) {
			t.Fatal("parser must not be called")

			return token.AccessTokenClaims{}, nil
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			t.Fatal("protected handler must not be called")
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Basic some-token",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status 401, got %d",
			recorder.Code,
		)
	}
}

func TestAuthMiddleware_EmptyBearerToken(
	t *testing.T,
) {
	parser := &accessTokenParserFake{
		parseFunc: func(
			context.Context,
			string,
		) (token.AccessTokenClaims, error) {
			t.Fatal("parser must not be called")

			return token.AccessTokenClaims{}, nil
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			t.Fatal("protected handler must not be called")
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer ",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status 401, got %d",
			recorder.Code,
		)
	}
}

func TestAuthMiddleware_InvalidAccessToken(
	t *testing.T,
) {
	parser := &accessTokenParserFake{
		parseFunc: func(
			context.Context,
			string,
		) (token.AccessTokenClaims, error) {
			return token.AccessTokenClaims{},
				token.ErrInvalidAccessToken
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			t.Fatal("protected handler must not be called")
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer invalid-token",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status 401, got %d",
			recorder.Code,
		)
	}
}

func TestAuthMiddleware_InternalParserError(
	t *testing.T,
) {
	parser := &accessTokenParserFake{
		parseFunc: func(
			context.Context,
			string,
		) (token.AccessTokenClaims, error) {
			return token.AccessTokenClaims{},
				errParseAccessToken
		},
	}

	router := newMiddlewareTestRouter(
		parser,
		func(c *gin.Context) {
			t.Fatal("protected handler must not be called")
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer token",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status 500, got %d",
			recorder.Code,
		)
	}
}

func newMiddlewareTestRouter(
	parser AccessTokenParser,
	protectedHandler gin.HandlerFunc,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.GET(
		"/protected",
		AuthMiddleware(parser),
		protectedHandler,
	)

	return router
}
