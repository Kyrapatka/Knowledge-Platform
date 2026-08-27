package handler

import (
	"context"
	"errors"
	"net/http"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FolderService interface {
	Create(
		ctx context.Context,
		ownerID uuid.UUID,
		title string,
		description string,
		templateKey string,
	) (foldermodel.Folder, error)

	GetByID(
		ctx context.Context,
		ownerID uuid.UUID,
		folderID uuid.UUID,
	) (foldermodel.Folder, error)

	List(
		ctx context.Context,
		ownerID uuid.UUID,
	) ([]foldermodel.Folder, error)

	Update(
		ctx context.Context,
		ownerID uuid.UUID,
		folderID uuid.UUID,
		title string,
		description string,
	) (foldermodel.Folder, error)

	Delete(
		ctx context.Context,
		ownerID uuid.UUID,
		folderID uuid.UUID,
	) error
}

type Handler struct {
	service FolderService
}

func NewHandler(
	service FolderService,
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

	var request CreateFolderRequest

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid_request",
			},
		)
		return
	}

	createdFolder, err := h.service.Create(
		c.Request.Context(),
		ownerID,
		request.Title,
		request.Description,
		request.TemplateKey,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			folder.ErrInvalidTemplate,
		):
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"error": "invalid_template",
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

		return
	}

	c.JSON(
		http.StatusCreated,
		toResponse(createdFolder),
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

	folderID, ok := parseFolderID(c)
	if !ok {
		return
	}

	result, err := h.service.GetByID(
		c.Request.Context(),
		ownerID,
		folderID,
	)
	if err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			c.JSON(
				http.StatusNotFound,
				gin.H{
					"error": "folder_not_found",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal_error",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toResponse(result),
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

	folders, err := h.service.List(
		c.Request.Context(),
		ownerID,
	)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal_error",
			},
		)
		return
	}

	response := make(
		[]FolderResponse,
		0,
		len(folders),
	)

	for _, f := range folders {
		response = append(
			response,
			toResponse(f),
		)
	}

	c.JSON(
		http.StatusOK,
		response,
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

	folderID, ok := parseFolderID(c)
	if !ok {
		return
	}

	var request UpdateFolderRequest

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid_request",
			},
		)
		return
	}

	updatedFolder, err := h.service.Update(
		c.Request.Context(),
		ownerID,
		folderID,
		request.Title,
		request.Description,
	)
	if err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			c.JSON(
				http.StatusNotFound,
				gin.H{
					"error": "folder_not_found",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal_error",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toResponse(updatedFolder),
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

	folderID, ok := parseFolderID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(
		c.Request.Context(),
		ownerID,
		folderID,
	); err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			c.JSON(
				http.StatusNotFound,
				gin.H{
					"error": "folder_not_found",
				},
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal_error",
			},
		)
		return
	}

	c.Status(
		http.StatusNoContent,
	)
}

func parseFolderID(
	c *gin.Context,
) (uuid.UUID, bool) {
	folderID, err := uuid.Parse(
		c.Param("folderID"),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid_folder_id",
			},
		)

		return uuid.Nil, false
	}

	return folderID, true
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

func toResponse(
	f foldermodel.Folder,
) FolderResponse {
	return FolderResponse{
		ID:            f.ID,
		OwnerID:       f.OwnerID,
		Title:         f.Title,
		Description:   f.Description,
		TemplateKey:   f.TemplateKey,
		Config:        f.Config,
		ConfigVersion: f.ConfigVersion,
		CreatedAt:     f.CreatedAt,
		UpdatedAt:     f.UpdatedAt,
	}
}
