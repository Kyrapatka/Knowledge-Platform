package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) LogoutAll(c *gin.Context) {
	value, exists := c.Get(userIDContextKey)

	if !exists {
		c.JSON(
			http.StatusUnauthorized,
			errorResponse{
				Error: "unauthorized",
			},
		)

		return
	}

	userID, ok := value.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		c.JSON(
			http.StatusUnauthorized,
			errorResponse{
				Error: "unauthorized",
			},
		)

		return
	}

	err := h.service.LogoutAll(
		c.Request.Context(),
		userID,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			errorResponse{
				Error: "internal_error",
			},
		)

		return
	}

	c.Status(http.StatusNoContent)
}
