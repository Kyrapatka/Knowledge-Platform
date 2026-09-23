package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) StartMock(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	var req service.StartGraphRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.StartGraph(c.Request.Context(), u, uuid.Nil, req)
	respond(c, 200, v, e)
}
func (h *Handler) ActiveMock(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	v, e := h.service.ActiveMock(c.Request.Context(), u)
	respond(c, 200, v, e)
}
func (h *Handler) PreviewMock(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	var req service.StartGraphRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.PreviewMock(c.Request.Context(), u, req)
	respond(c, 200, v, e)
}

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
