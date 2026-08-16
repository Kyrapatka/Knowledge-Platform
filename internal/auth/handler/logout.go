package handler

import (
	"errors"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
)

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Logout(c *gin.Context) {
	var request logoutRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid_request",
			},
		)

		return
	}

	err := h.service.Logout(
		c.Request.Context(),
		service.LogoutInput{
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

	c.Status(http.StatusNoContent)
}
