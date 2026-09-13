package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
)

func (h *Handler) StartGraph(c *gin.Context) {
	u, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	var req service.StartGraphRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.StartGraph(c.Request.Context(), u, id, req)
	respond(c, 200, v, e)
}
func (h *Handler) UndoGraph(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	var req service.GraphUndoRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.UndoGraph(c.Request.Context(), u, id, req)
	respond(c, 200, v, e)
}
func (h *Handler) GraphConfig(c *gin.Context) {
	if _, _, ok := identity(c, ""); !ok {
		return
	}
	c.JSON(200, graph.DefaultConfig())
}
