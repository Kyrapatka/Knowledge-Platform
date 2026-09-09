package handler

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strconv"
)

func (h *Handler) CreateExercise(c *gin.Context) {
	user, materialID, ok := identity(c, "materialID")
	if !ok {
		return
	}
	var req service.ExerciseRequest
	if !bind(c, &req) {
		return
	}
	out, err := h.service.CreateExercise(c.Request.Context(), user, materialID, req)
	respond(c, 201, out, err)
}
func (h *Handler) Exercises(c *gin.Context) {
	user, materialID, ok := identity(c, "materialID")
	if !ok {
		return
	}
	limit, e1 := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, e2 := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e1 != nil || e2 != nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	out, err := h.service.Exercises(c.Request.Context(), user, materialID, limit, offset)
	respond(c, 200, out, err)
}
func (h *Handler) UpdateExercise(c *gin.Context) {
	user, materialID, ok := identity(c, "materialID")
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("exerciseID"))
	if err != nil || id == uuid.Nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	var req service.ExerciseRequest
	if !bind(c, &req) {
		return
	}
	out, err := h.service.UpdateExercise(c.Request.Context(), user, materialID, id, req)
	respond(c, 200, out, err)
}
func (h *Handler) DeleteExercise(c *gin.Context) {
	user, materialID, ok := identity(c, "materialID")
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("exerciseID"))
	if err != nil || id == uuid.Nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	version, err := strconv.Atoi(c.Query("expected_version"))
	if err != nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	err = h.service.DeleteExercise(c.Request.Context(), user, materialID, id, version)
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	c.Status(204)
}
