package handler

import (
	"errors"
	"net/http"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

func (h *Handler) Register(c *gin.Context) {
	var request registerRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid_request",
			},
		)

		return
	}

	result, err := h.service.Register(
		c.Request.Context(),
		service.RegisterInput{
			Nickname:  request.Nickname,
			Password:  request.Password,
			UserAgent: c.Request.UserAgent(),
			IPAddress: clientIPAddress(c),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidNickname):
			c.JSON(
				http.StatusBadRequest,
				errorResponse{
					Error: "invalid_nickname",
				},
			)

		case errors.Is(err, service.ErrInvalidPassword):
			c.JSON(
				http.StatusBadRequest,
				errorResponse{
					Error: "invalid_password",
				},
			)

		case errors.Is(err, service.ErrNicknameTaken):
			c.JSON(
				http.StatusConflict,
				errorResponse{
					Error: "nickname_taken",
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
		http.StatusCreated,
		toAuthResponse(result),
	)
}
