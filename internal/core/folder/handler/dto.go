package handler

import (
	"time"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"

	"github.com/google/uuid"
)

type CreateFolderRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TemplateKey string `json:"template_key"`
}

type UpdateFolderRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type FolderResponse struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	TemplateKey string    `json:"template_key"`

	Config        folderconfig.FolderConfig `json:"config"`
	ConfigVersion int64                     `json:"config_version"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
