package handler

import (
	"errors"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var request loginRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid_request",
			},
		)

		return
	}

	result, err := h.service.Login(
		c.Request.Context(),
		service.LoginInput{
			Nickname:  request.Nickname,
			Password:  request.Password,
			UserAgent: c.Request.UserAgent(),
			IPAddress: clientIPAddress(c),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			c.JSON(
				http.StatusUnauthorized,
				errorResponse{
					Error: "invalid_credentials",
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
