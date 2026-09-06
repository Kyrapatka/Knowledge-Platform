package handler

import (
	"errors"
	"net/http"
	"strconv"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *service.Service }

func NewHandler(s *service.Service) *Handler { return &Handler{s} }

// RegisterRoutes expects a group protected by AuthMiddleware. Each handler
// also checks the authenticated context, including for accidental direct use.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/folders/:folderID/training-config", h.GetDefaults)
	api.PATCH("/folders/:folderID/training-config", h.UpdateDefaults)
	api.POST("/training/plans", h.CreatePlan)
	api.GET("/training/plans", h.ListPlans)
	api.GET("/training/plans/:planID", h.GetPlan)
	api.POST("/training/plans/:planID/cancel", h.CancelPlan)
	api.POST("/training/plans/:planID/sessions", h.StartSession)
	api.GET("/training/sessions/:sessionID", h.GetSession)
	api.GET("/training/sessions/:sessionID/current", h.GetSession)
	api.GET("/training/sessions/:sessionID/result", h.GetSession)
	api.POST("/training/sessions/:sessionID/finish", h.FinishSession)
	api.POST("/training/sessions/:sessionID/cancel", h.CancelSession)
	api.POST("/training/sessions/:sessionID/actions", h.Act)
	api.POST("/training/sessions/:sessionID/materials/:materialID/skip-rehab", h.SkipRecovery)
}

func identity(c *gin.Context, param string) (uuid.UUID, uuid.UUID, bool) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, uuid.Nil, false
	}
	if param == "" {
		return user, uuid.Nil, true
	}
	id, err := uuid.Parse(c.Param(param))
	if err != nil || id == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return uuid.Nil, uuid.Nil, false
	}
	return user, id, true
}

func respond(c *gin.Context, status int, value any, err error) {
	if err == nil {
		c.JSON(status, value)
		return
	}
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "training_not_found"})
	case errors.Is(err, repository.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "training_conflict"})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_training_request", "message": err.Error()})
	default:
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	}
}

func bind(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	if err := c.ShouldBindJSON(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return false
	}
	return true
}

func (h *Handler) GetDefaults(c *gin.Context) {
	u, id, ok := identity(c, "folderID")
	if !ok {
		return
	}
	v, e := h.service.FolderDefaults(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) UpdateDefaults(c *gin.Context) {
	u, id, ok := identity(c, "folderID")
	if !ok {
		return
	}
	var req service.FolderDefaults
	if !bind(c, &req) {
		return
	}
	v, e := h.service.UpdateDefaults(c.Request.Context(), u, id, req.Config, req.Version)
	respond(c, 200, v, e)
}
func (h *Handler) CreatePlan(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	var req service.CreatePlanRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.CreatePlan(c.Request.Context(), u, req)
	respond(c, 201, v, e)
}
func (h *Handler) ListPlans(c *gin.Context) {
	u, _, ok := identity(c, "")
	if !ok {
		return
	}
	limit, e1 := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, e2 := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e1 != nil || e2 != nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	v, e := h.service.ListPlans(c.Request.Context(), u, limit, offset)
	respond(c, 200, v, e)
}
func (h *Handler) GetPlan(c *gin.Context) {
	u, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	v, e := h.service.GetPlan(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) CancelPlan(c *gin.Context) {
	u, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	v, e := h.service.CancelPlan(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) StartSession(c *gin.Context) {
	u, id, ok := identity(c, "planID")
	if !ok {
		return
	}
	v, e := h.service.StartSession(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) GetSession(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	v, e := h.service.GetSession(c.Request.Context(), u, id)
	respond(c, 200, v, e)
}
func (h *Handler) FinishSession(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	v, e := h.service.FinishSession(c.Request.Context(), u, id, model.StatusCompleted)
	respond(c, 200, v, e)
}
func (h *Handler) CancelSession(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	v, e := h.service.FinishSession(c.Request.Context(), u, id, model.StatusCancelled)
	respond(c, 200, v, e)
}
func (h *Handler) Act(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	var req service.ActionRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.Act(c.Request.Context(), u, id, req)
	respond(c, 200, v, e)
}

func (h *Handler) SkipRecovery(c *gin.Context) {
	u, id, ok := identity(c, "sessionID")
	if !ok {
		return
	}
	materialID, err := uuid.Parse(c.Param("materialID"))
	if err != nil || materialID == uuid.Nil {
		respond(c, 0, nil, service.ErrInvalid)
		return
	}
	var req service.SkipRequest
	if !bind(c, &req) {
		return
	}
	v, e := h.service.SkipRecovery(c.Request.Context(), u, id, materialID, req)
	respond(c, 200, v, e)
}
