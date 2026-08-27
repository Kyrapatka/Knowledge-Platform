package repository

import (
	"context"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"

	"github.com/google/uuid"
)

// Repository содержит обычные CRUD-операции Folder.
//
// FolderService и MaterialService не должны знать,
// что существует специальная Workshop-операция UpdateConfig.
type Repository interface {
	Create(
		ctx context.Context,
		folder foldermodel.Folder,
	) error

	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (foldermodel.Folder, error)

	ListByOwner(
		ctx context.Context,
		ownerID uuid.UUID,
	) ([]foldermodel.Folder, error)

	Update(
		ctx context.Context,
		folder foldermodel.Folder,
	) error

	Delete(
		ctx context.Context,
		id uuid.UUID,
	) error
}

// ConfigRepository содержит только те операции,
// которые нужны Workshop.
//
// PostgreSQL Repository реализует одновременно
// Repository и ConfigRepository.
type ConfigRepository interface {
	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (foldermodel.Folder, error)

	UpdateConfig(
		ctx context.Context,
		folderID uuid.UUID,
		config folderconfig.FolderConfig,
		expectedVersion int64,
	) (int64, error)
}
