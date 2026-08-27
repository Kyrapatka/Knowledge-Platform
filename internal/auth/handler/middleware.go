package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const userIDContextKey = "user_id"

type AccessTokenParser interface {
	ParseAccessToken(
		ctx context.Context,
		token string,
	) (token.AccessTokenClaims, error)
}

func AuthMiddleware(
	parser AccessTokenParser,
) gin.HandlerFunc {
	if parser == nil {
		panic("auth middleware: token parser is nil")
	}

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")

		if header == "" {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "unauthorized",
				},
			)

			return
		}

		const prefix = "Bearer "

		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "unauthorized",
				},
			)

			return
		}

		tokenValue := strings.TrimSpace(
			strings.TrimPrefix(header, prefix),
		)

		if tokenValue == "" {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "unauthorized",
				},
			)

			return
		}

		claims, err := parser.ParseAccessToken(
			c.Request.Context(),
			tokenValue,
		)
		if err != nil {
			if errors.Is(err, token.ErrInvalidAccessToken) {
				c.AbortWithStatusJSON(
					http.StatusUnauthorized,
					errorResponse{
						Error: "unauthorized",
					},
				)

				return
			}

			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				errorResponse{
					Error: "internal_error",
				},
			)

			return
		}

		c.Set(
			userIDContextKey,
			claims.UserID,
		)

		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(userIDContextKey)
	if !exists {
		return uuid.Nil, false
	}

	userID, ok := value.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}

	return userID, true
}
