package model

import (
	"time"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/google/uuid"
)

type Folder struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	Title         string
	Description   string
	TemplateKey   string
	Config        folderconfig.FolderConfig
	ConfigVersion int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
