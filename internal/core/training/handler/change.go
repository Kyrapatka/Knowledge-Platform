package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *Handler) ChangeAlgorithm(c *gin.Context) {
	user, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	var req service.ChangeRequest
	if !bind(c, &req) {
		return
	}
	out, err := h.service.ChangeAlgorithm(c.Request.Context(), user, id, req)
	respond(c, 200, out, err)
}
func (h *Handler) PlanChanges(c *gin.Context) {
	user, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	limit, e1 := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, e2 := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e1 != nil || e2 != nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	out, err := h.service.PlanChanges(c.Request.Context(), user, id, limit, offset)
	respond(c, 200, out, err)
}
