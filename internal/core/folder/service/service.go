package service

import (
	"context"
	"errors"
	"fmt"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"time"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderrepository "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"

	"github.com/google/uuid"
)

type Service struct {
	analytics.Emitter
	repository       folderrepository.Repository
	templateRegistry *foldertemplate.Registry
}

func NewService(
	repository folderrepository.Repository,
	templateRegistry *foldertemplate.Registry,
) *Service {
	return &Service{
		repository:       repository,
		templateRegistry: templateRegistry,
	}
}

func (s *Service) Create(
	ctx context.Context,
	ownerID uuid.UUID,
	title string,
	description string,
	templateKey string,
) (foldermodel.Folder, error) {
	template, err := s.templateRegistry.Get(
		templateKey,
	)
	if err != nil {
		if errors.Is(
			err,
			foldertemplate.ErrNotFound,
		) {
			return foldermodel.Folder{},
				folder.ErrInvalidTemplate
		}

		return foldermodel.Folder{},
			fmt.Errorf(
				"get template: %w",
				err,
			)
	}

	now := time.Now().UTC()

	f := foldermodel.Folder{
		ID:                    uuid.New(),
		OwnerID:               ownerID,
		Title:                 title,
		Description:           description,
		TemplateKey:           template.Key,
		Config:                template.Config,
		ConfigVersion:         1,
		TrainingConfig:        folderconfig.DefaultTrainingConfig(template.Key),
		TrainingConfigVersion: 1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := s.repository.Create(
		ctx,
		f,
	); err != nil {
		return foldermodel.Folder{},
			fmt.Errorf(
				"create folder: %w",
				err,
			)
	}

	event := analytics.New(analytics.FolderCreated, ownerID)
	event.FolderID, event.Template = f.ID.String(), f.TemplateKey
	s.Publish(ctx, event)
	return f, nil
}

func (s *Service) GetByID(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) (foldermodel.Folder, error) {
	f, err := s.repository.GetByID(
		ctx,
		folderID,
	)
	if err != nil {
		return foldermodel.Folder{}, err
	}

	if f.OwnerID != ownerID {
		return foldermodel.Folder{},
			folder.ErrNotFound
	}

	return f, nil
}

func (s *Service) List(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]foldermodel.Folder, error) {
	folders, err := s.repository.ListByOwner(
		ctx,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list folders: %w",
			err,
		)
	}

	return folders, nil
}

func (s *Service) Update(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	title string,
	description string,
) (foldermodel.Folder, error) {
	f, err := s.repository.GetByID(
		ctx,
		folderID,
	)
	if err != nil {
		return foldermodel.Folder{}, err
	}

	if f.OwnerID != ownerID {
		return foldermodel.Folder{},
			folder.ErrNotFound
	}

	f.Title = title
	f.Description = description
	f.UpdatedAt = time.Now().UTC()

	if err := s.repository.Update(
		ctx,
		f,
	); err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			return foldermodel.Folder{},
				folder.ErrNotFound
		}

		return foldermodel.Folder{},
			fmt.Errorf(
				"update folder: %w",
				err,
			)
	}

	return f, nil
}

func (s *Service) Delete(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) error {
	f, err := s.repository.GetByID(
		ctx,
		folderID,
	)
	if err != nil {
		return err
	}

	if f.OwnerID != ownerID {
		return folder.ErrNotFound
	}

	if err := s.repository.Delete(
		ctx,
		folderID,
	); err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			return folder.ErrNotFound
		}

		return fmt.Errorf(
			"delete folder: %w",
			err,
		)
	}

	event := analytics.New(analytics.FolderDeleted, ownerID)
	event.FolderID, event.Template = folderID.String(), f.TemplateKey
	s.Publish(ctx, event)
	return nil
}
