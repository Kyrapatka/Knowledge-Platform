package handler

import (
	"context"
	"errors"
	"net/http"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	workshop "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WorkshopService interface {
	UpdateConfig(
		ctx context.Context,
		ownerID uuid.UUID,
		folderID uuid.UUID,
		expectedVersion int64,
		config folderconfig.FolderConfig,
	) (foldermodel.Folder, error)
}

type Handler struct {
	service WorkshopService
}

func NewHandler(
	service WorkshopService,
) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) UpdateConfig(
	c *gin.Context,
) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok {
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"error": "unauthorized",
			},
		)
		return
	}

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
		return
	}

	var request UpdateConfigRequest

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

	updatedFolder, err := h.service.UpdateConfig(
		c.Request.Context(),
		ownerID,
		folderID,
		request.ExpectedVersion,
		request.Config,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			folder.ErrNotFound,
		):
			c.JSON(
				http.StatusNotFound,
				gin.H{
					"error": "folder_not_found",
				},
			)

		case errors.Is(
			err,
			workshop.ErrInvalidConfig,
		):
			c.JSON(
				http.StatusBadRequest,
				gin.H{
					"error":   "invalid_config",
					"message": err.Error(),
				},
			)

		case errors.Is(
			err,
			workshop.ErrConflict,
		):
			c.JSON(
				http.StatusConflict,
				gin.H{
					"error": "config_conflict",
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
		http.StatusOK,
		UpdateConfigResponse{
			Config:        updatedFolder.Config,
			ConfigVersion: updatedFolder.ConfigVersion,
		},
	)
}
