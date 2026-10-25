package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ListMockPresets(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	v, e := h.service.MockPresets(c.Request.Context(), u)
	respond(c, 200, v, e)
}
func (h *Handler) GetMockPreset(c *gin.Context) {
	u, id, ok := identity(c, "presetID")
	if !ok {
		return
	}
	v, e := h.service.MockPreset(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) SaveMockPreset(c *gin.Context) {
	param := ""
	status := 201
	if c.Request.Method == "PUT" {
		param = "presetID"
		status = 200
	}
	u, id, ok := identity(c, param)
	if !ok {
		return
	}
	if param == "" {
		id = uuid.Nil
	}
	var req service.SaveMockPresetRequest
	if c.ShouldBindJSON(&req) != nil {
		respond(c, 400, nil, service.ErrInvalid)
		return
	}
	v, e := h.service.SaveMockPreset(c.Request.Context(), u, id, req)
	respond(c, status, v, e)
}
func (h *Handler) DeleteMockPreset(c *gin.Context) {
	u, id, ok := identity(c, "presetID")
	if !ok {
		return
	}
	e := h.service.DeleteMockPreset(c.Request.Context(), u, id)
	respond(c, 200, gin.H{"deleted": e == nil}, e)
}
func (h *Handler) DuplicateMockPreset(c *gin.Context) {
	u, id, ok := identity(c, "presetID")
	if !ok {
		return
	}
	v, e := h.service.DuplicateMockPreset(c.Request.Context(), u, id)
	respond(c, 201, v, e)
}
