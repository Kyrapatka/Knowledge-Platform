package handler

import (
	"errors"
	"net/http"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *materialservice.Service
}

func NewHandler(
	service *materialservice.Service,
) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	folderID, ok := parseUUIDParam(
		c,
		"folderID",
		"invalid_folder_id",
	)
	if !ok {
		return
	}

	var request CreateMaterialRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid_request",
			},
		)
		return
	}

	difficulty := materialmodel.DifficultyMedium
	if request.Difficulty != nil {
		difficulty = *request.Difficulty
	}
	createdMaterial, err := h.service.CreateWithDifficulty(
		c.Request.Context(),
		ownerID,
		folderID,
		request.Values,
		request.Metadata,
		difficulty,
	)
	if err != nil {
		writeServiceError(
			c,
			err,
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		toResponse(createdMaterial),
	)
}

func (h *Handler) List(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	folderID, ok := parseUUIDParam(
		c,
		"folderID",
		"invalid_folder_id",
	)
	if !ok {
		return
	}

	materials, err := h.service.List(
		c.Request.Context(),
		ownerID,
		folderID,
	)
	if err != nil {
		writeServiceError(
			c,
			err,
		)
		return
	}

	response := make(
		[]MaterialResponse,
		0,
		len(materials),
	)

	for _, m := range materials {
		response = append(
			response,
			toResponse(m),
		)
	}

	c.JSON(
		http.StatusOK,
		response,
	)
}

func (h *Handler) GetByID(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	folderID, ok := parseUUIDParam(
		c,
		"folderID",
		"invalid_folder_id",
	)
	if !ok {
		return
	}

	materialID, ok := parseUUIDParam(
		c,
		"materialID",
		"invalid_material_id",
	)
	if !ok {
		return
	}

	m, err := h.service.GetByID(
		c.Request.Context(),
		ownerID,
		folderID,
		materialID,
	)
	if err != nil {
		writeServiceError(
			c,
			err,
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toResponse(m),
	)
}

func (h *Handler) Update(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	folderID, ok := parseUUIDParam(
		c,
		"folderID",
		"invalid_folder_id",
	)
	if !ok {
		return
	}

	materialID, ok := parseUUIDParam(
		c,
		"materialID",
		"invalid_material_id",
	)
	if !ok {
		return
	}

	var request UpdateMaterialRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid_request",
			},
		)
		return
	}

	updatedMaterial, err := h.service.UpdateWithDifficulty(
		c.Request.Context(),
		ownerID,
		folderID,
		materialID,
		request.Values,
		request.Metadata,
		request.Difficulty,
	)
	if err != nil {
		writeServiceError(
			c,
			err,
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toResponse(updatedMaterial),
	)
}

func (h *Handler) Delete(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	folderID, ok := parseUUIDParam(
		c,
		"folderID",
		"invalid_folder_id",
	)
	if !ok {
		return
	}

	materialID, ok := parseUUIDParam(
		c,
		"materialID",
		"invalid_material_id",
	)
	if !ok {
		return
	}

	if err := h.service.Delete(
		c.Request.Context(),
		ownerID,
		folderID,
		materialID,
	); err != nil {
		writeServiceError(
			c,
			err,
		)
		return
	}

	c.Status(
		http.StatusNoContent,
	)
}

func parseUUIDParam(
	c *gin.Context,
	param string,
	errorCode string,
) (uuid.UUID, bool) {
	id, err := uuid.Parse(
		c.Param(param),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": errorCode,
			},
		)

		return uuid.Nil, false
	}

	return id, true
}

func writeUnauthorized(
	c *gin.Context,
) {
	c.JSON(
		http.StatusUnauthorized,
		gin.H{
			"error": "unauthorized",
		},
	)
}

func writeServiceError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, material.ErrInvalidDifficulty):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_difficulty"})
	case errors.Is(
		err,
		material.ErrFolderNotFound,
	):
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"error": "folder_not_found",
			},
		)

	case errors.Is(
		err,
		material.ErrNotFound,
	):
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"error": "material_not_found",
			},
		)

	case errors.Is(
		err,
		material.ErrInvalidValues,
	):
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)

	case errors.Is(
		err,
		material.ErrInvalidMetadata,
	):
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)

	default:
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal_error",
			},
		)
	}
}

func toResponse(
	m materialmodel.Material,
) MaterialResponse {
	return MaterialResponse{
		ID:         m.ID,
		FolderID:   m.FolderID,
		Values:     m.Values,
		Metadata:   m.Metadata,
		Difficulty: m.Difficulty,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
