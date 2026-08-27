package service

import (
	"context"
	"errors"
	"fmt"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderrepository "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository"
	workshop "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop"

	"github.com/google/uuid"
)

type Service struct {
	folderRepository folderrepository.ConfigRepository
}

func NewService(
	folderRepository folderrepository.ConfigRepository,
) *Service {
	return &Service{
		folderRepository: folderRepository,
	}
}

func (s *Service) UpdateConfig(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	expectedVersion int64,
	newConfig folderconfig.FolderConfig,
) (foldermodel.Folder, error) {
	if expectedVersion <= 0 {
		return foldermodel.Folder{},
			fmt.Errorf(
				"%w: expected_version must be greater than zero",
				workshop.ErrInvalidConfig,
			)
	}

	currentFolder, err := s.folderRepository.GetByID(
		ctx,
		folderID,
	)
	if err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			return foldermodel.Folder{},
				folder.ErrNotFound
		}

		return foldermodel.Folder{},
			fmt.Errorf(
				"get folder for workshop: %w",
				err,
			)
	}

	if currentFolder.OwnerID != ownerID {
		return foldermodel.Folder{},
			folder.ErrNotFound
	}

	// Эта проверка быстро ловит запрос,
	// который уже устарел к моменту чтения Folder.
	//
	// Она НЕ заменяет optimistic locking в PostgreSQL,
	// потому что config может измениться между
	// GetByID и UpdateConfig.
	if currentFolder.ConfigVersion != expectedVersion {
		return foldermodel.Folder{},
			workshop.ErrConflict
	}

	if err := workshop.ValidateConfig(
		currentFolder.Config,
		newConfig,
	); err != nil {
		return foldermodel.Folder{}, err
	}

	newVersion, err := s.folderRepository.UpdateConfig(
		ctx,
		folderID,
		newConfig,
		expectedVersion,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			folder.ErrNotFound,
		):
			return foldermodel.Folder{},
				folder.ErrNotFound

		case errors.Is(
			err,
			folder.ErrConfigConflict,
		):
			return foldermodel.Folder{},
				workshop.ErrConflict

		default:
			return foldermodel.Folder{},
				fmt.Errorf(
					"update workshop config: %w",
					err,
				)
		}
	}

	currentFolder.Config = newConfig
	currentFolder.ConfigVersion = newVersion

	return currentFolder, nil
}
