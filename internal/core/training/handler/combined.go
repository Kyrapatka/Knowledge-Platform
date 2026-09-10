package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) StartCombined(c *gin.Context) {
	user, _, ok := identity(c, "")
	if !ok {
		return
	}
	var req service.CombinedRequest
	if !bind(c, &req) {
		return
	}
	result, err := h.service.StartCombined(c.Request.Context(), user, req)
	respond(c, http.StatusOK, result, err)
}

func (h *Handler) CurrentCombined(c *gin.Context) {
	user, _, ok := identity(c, "")
	if !ok {
		return
	}
	var req service.CombinedCurrentRequest
	if !bind(c, &req) {
		return
	}
	result, err := h.service.CurrentCombined(c.Request.Context(), user, req)
	respond(c, http.StatusOK, result, err)
}
