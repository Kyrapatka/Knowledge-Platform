package bulk

import (
	"encoding/json"
	"errors"
	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/folders/:folderID/materials/bulk", h.apply)
}
func (h *Handler) apply(c *gin.Context) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("folderID"))
	if err != nil || id == uuid.Nil {
		c.JSON(400, gin.H{"error": "invalid_folder_id"})
		return
	}
	var req Request
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}
	result, err := h.service.Apply(c.Request.Context(), user, id, req)
	var duplicate *interview.DuplicateQuestion
	switch {
	case err == nil:
		c.JSON(200, result)
	case errors.As(err, &duplicate):
		c.JSON(409, gin.H{"error": "duplicate_question", "existing_material_id": duplicate.MaterialID, "folder_id": duplicate.FolderID})
	case IsInvalid(err):
		c.JSON(400, gin.H{"error": "invalid_bulk_materials", "message": err.Error()})
	case errors.Is(err, ErrConflict) || errors.Is(err, interview.ErrConflict):
		c.JSON(409, gin.H{"error": "bulk_conflict", "message": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, material.ErrNotFound) || errors.Is(err, material.ErrFolderNotFound) || errors.Is(err, folder.ErrNotFound):
		c.JSON(404, gin.H{"error": "material_not_found"})
	default:
		c.Error(err)
		c.JSON(500, gin.H{"error": "internal_error"})
	}
}
