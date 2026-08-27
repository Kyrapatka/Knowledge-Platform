package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"

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
	m materialmodel.Material,
) error {
	model, err := toModel(m)
	if err != nil {
		return err
	}

	if err := r.db.
		WithContext(ctx).
		Create(&model).
		Error; err != nil {

		return fmt.Errorf(
			"create material: %w",
			err,
		)
	}

	return nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (materialmodel.Material, error) {
	var model materialModel

	err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		First(&model).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return materialmodel.Material{}, material.ErrNotFound
		}

		return materialmodel.Material{}, fmt.Errorf(
			"get material by id: %w",
			err,
		)
	}

	m, err := toDomain(model)
	if err != nil {
		return materialmodel.Material{}, err
	}

	return m, nil
}

func (r *Repository) ListByFolder(
	ctx context.Context,
	folderID uuid.UUID,
) ([]materialmodel.Material, error) {
	var models []materialModel

	if err := r.db.
		WithContext(ctx).
		Where("folder_id = ?", folderID).
		Order("created_at DESC").
		Find(&models).
		Error; err != nil {

		return nil, fmt.Errorf(
			"list materials by folder: %w",
			err,
		)
	}

	materials := make(
		[]materialmodel.Material,
		0,
		len(models),
	)

	for _, model := range models {
		m, err := toDomain(model)
		if err != nil {
			return nil, err
		}

		materials = append(
			materials,
			m,
		)
	}

	return materials, nil
}

func (r *Repository) Update(
	ctx context.Context,
	m materialmodel.Material,
) error {
	valuesJSON, err := json.Marshal(m.Values)
	if err != nil {
		return fmt.Errorf(
			"marshal material values: %w",
			err,
		)
	}

	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return fmt.Errorf(
			"marshal material metadata: %w",
			err,
		)
	}

	result := r.db.
		WithContext(ctx).
		Model(&materialModel{}).
		Where("id = ?", m.ID).
		Updates(map[string]any{
			"values":     valuesJSON,
			"metadata":   metadataJSON,
			"updated_at": m.UpdatedAt,
		})

	if result.Error != nil {
		return fmt.Errorf(
			"update material: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return material.ErrNotFound
	}

	return nil
}

func (r *Repository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	result := r.db.
		WithContext(ctx).
		Delete(
			&materialModel{},
			"id = ?",
			id,
		)

	if result.Error != nil {
		return fmt.Errorf(
			"delete material: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return material.ErrNotFound
	}

	return nil
}

func toModel(
	m materialmodel.Material,
) (materialModel, error) {
	valuesJSON, err := json.Marshal(m.Values)
	if err != nil {
		return materialModel{}, fmt.Errorf(
			"marshal material values: %w",
			err,
		)
	}

	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return materialModel{}, fmt.Errorf(
			"marshal material metadata: %w",
			err,
		)
	}

	return materialModel{
		ID:        m.ID,
		FolderID:  m.FolderID,
		Values:    valuesJSON,
		Metadata:  metadataJSON,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func toDomain(
	model materialModel,
) (materialmodel.Material, error) {
	var values map[string]*string

	if err := json.Unmarshal(
		model.Values,
		&values,
	); err != nil {
		return materialmodel.Material{}, fmt.Errorf(
			"unmarshal material values: %w",
			err,
		)
	}

	var metadata map[string]*string

	if err := json.Unmarshal(
		model.Metadata,
		&metadata,
	); err != nil {
		return materialmodel.Material{}, fmt.Errorf(
			"unmarshal material metadata: %w",
			err,
		)
	}

	if values == nil {
		values = make(map[string]*string)
	}

	if metadata == nil {
		metadata = make(map[string]*string)
	}

	return materialmodel.Material{
		ID:        model.ID,
		FolderID:  model.FolderID,
		Values:    values,
		Metadata:  metadata,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}, nil
}
