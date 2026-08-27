package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(
	db *gorm.DB,
) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Create(
	ctx context.Context,
	f foldermodel.Folder,
) error {
	model, err := toModel(f)
	if err != nil {
		return err
	}

	if err := r.db.
		WithContext(ctx).
		Create(&model).
		Error; err != nil {

		return fmt.Errorf(
			"create folder: %w",
			err,
		)
	}

	return nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (foldermodel.Folder, error) {
	var model folderModel

	err := r.db.
		WithContext(ctx).
		Where(
			"id = ?",
			id,
		).
		First(&model).
		Error

	if err != nil {
		if errors.Is(
			err,
			gorm.ErrRecordNotFound,
		) {
			return foldermodel.Folder{},
				folder.ErrNotFound
		}

		return foldermodel.Folder{},
			fmt.Errorf(
				"get folder by id: %w",
				err,
			)
	}

	f, err := toDomain(model)
	if err != nil {
		return foldermodel.Folder{}, err
	}

	return f, nil
}

func (r *Repository) ListByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]foldermodel.Folder, error) {
	var models []folderModel

	if err := r.db.
		WithContext(ctx).
		Where(
			"owner_id = ?",
			ownerID,
		).
		Order(
			"created_at DESC",
		).
		Find(&models).
		Error; err != nil {

		return nil, fmt.Errorf(
			"list folders by owner: %w",
			err,
		)
	}

	folders := make(
		[]foldermodel.Folder,
		0,
		len(models),
	)

	for _, model := range models {
		f, err := toDomain(model)
		if err != nil {
			return nil, err
		}

		folders = append(
			folders,
			f,
		)
	}

	return folders, nil
}

func (r *Repository) Update(
	ctx context.Context,
	f foldermodel.Folder,
) error {
	result := r.db.
		WithContext(ctx).
		Model(&folderModel{}).
		Where(
			"id = ?",
			f.ID,
		).
		Updates(map[string]any{
			"title":       f.Title,
			"description": f.Description,
			"updated_at":  f.UpdatedAt,
		})

	if result.Error != nil {
		return fmt.Errorf(
			"update folder: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return folder.ErrNotFound
	}

	return nil
}

func (r *Repository) UpdateConfig(
	ctx context.Context,
	folderID uuid.UUID,
	config folderconfig.FolderConfig,
	expectedVersion int64,
) (int64, error) {
	configJSON, err := json.Marshal(
		config,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"marshal folder config: %w",
			err,
		)
	}

	newVersion := expectedVersion + 1

	result := r.db.
		WithContext(ctx).
		Model(&folderModel{}).
		Where(
			"id = ? AND config_version = ?",
			folderID,
			expectedVersion,
		).
		Updates(map[string]any{
			"config":         configJSON,
			"config_version": newVersion,
			"updated_at":     gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return 0, fmt.Errorf(
			"update folder config: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 1 {
		return newVersion, nil
	}

	// UPDATE ничего не изменил.
	//
	// Возможны два основных случая:
	//
	// 1. Folder уже не существует.
	// 2. Folder существует, но config_version уже другая.
	var count int64

	if err := r.db.
		WithContext(ctx).
		Model(&folderModel{}).
		Where(
			"id = ?",
			folderID,
		).
		Count(&count).
		Error; err != nil {

		return 0, fmt.Errorf(
			"check folder after config conflict: %w",
			err,
		)
	}

	if count == 0 {
		return 0, folder.ErrNotFound
	}

	return 0, folder.ErrConfigConflict
}

func (r *Repository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	result := r.db.
		WithContext(ctx).
		Delete(
			&folderModel{},
			"id = ?",
			id,
		)

	if result.Error != nil {
		return fmt.Errorf(
			"delete folder: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return folder.ErrNotFound
	}

	return nil
}

func toModel(
	f foldermodel.Folder,
) (folderModel, error) {
	configJSON, err := json.Marshal(
		f.Config,
	)
	if err != nil {
		return folderModel{}, fmt.Errorf(
			"marshal folder config: %w",
			err,
		)
	}

	return folderModel{
		ID:            f.ID,
		OwnerID:       f.OwnerID,
		Title:         f.Title,
		Description:   f.Description,
		TemplateKey:   f.TemplateKey,
		Config:        configJSON,
		ConfigVersion: f.ConfigVersion,
		CreatedAt:     f.CreatedAt,
		UpdatedAt:     f.UpdatedAt,
	}, nil
}

func toDomain(
	model folderModel,
) (foldermodel.Folder, error) {
	var config folderconfig.FolderConfig

	if err := json.Unmarshal(
		model.Config,
		&config,
	); err != nil {
		return foldermodel.Folder{}, fmt.Errorf(
			"unmarshal folder config: %w",
			err,
		)
	}

	return foldermodel.Folder{
		ID:            model.ID,
		OwnerID:       model.OwnerID,
		Title:         model.Title,
		Description:   model.Description,
		TemplateKey:   model.TemplateKey,
		Config:        config,
		ConfigVersion: model.ConfigVersion,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}
