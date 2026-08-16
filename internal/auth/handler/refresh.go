package handler

import (
	"errors"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(c *gin.Context) {
	var request refreshRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid_request",
			},
		)

		return
	}

	result, err := h.service.Refresh(
		c.Request.Context(),
		service.RefreshInput{
			RefreshToken: request.RefreshToken,
		},
	)

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRefreshToken):
			c.JSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "invalid_refresh_token",
				},
			)

		case errors.Is(err, service.ErrSessionExpired):
			c.JSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "session_expired",
				},
			)

		case errors.Is(err, service.ErrUserBlocked):
			c.JSON(
				http.StatusForbidden,
				errorResponse{
					Error: "user_blocked",
				},
			)

		default:
			c.JSON(
				http.StatusInternalServerError,
				errorResponse{
					Error: "internal_error",
				},
			)
		}

		return
	}

	c.JSON(
		http.StatusOK,
		toAuthResponse(result),
	)
}
