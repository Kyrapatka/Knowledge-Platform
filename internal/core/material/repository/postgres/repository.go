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
	"gorm.io/gorm/clause"
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

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID uuid.UUID }
		if err := tx.Table("folders").Select("owner_id AS id").Where("id=? AND deleted_at IS NULL", m.FolderID).Take(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return material.ErrFolderNotFound
			}
			return err
		}
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", owner.ID).Take(&owner).Error; err != nil {
			return err
		}
		// Recheck after locking: folder deletion may have committed while we waited.
		var count int64
		if err := tx.Table("folders").Where("id=? AND deleted_at IS NULL", m.FolderID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return material.ErrFolderNotFound
		}
		return tx.Create(&model).Error
	})
}

func (r *Repository) CreateBatch(
	ctx context.Context,
	materials []materialmodel.Material,
) error {
	if len(materials) == 0 {
		return nil
	}
	models := make([]materialModel, 0, len(materials))
	for _, materialEntity := range materials {
		model, err := toModel(materialEntity)
		if err != nil {
			return err
		}
		models = append(models, model)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID uuid.UUID }
		if err := tx.Table("folders").Select("owner_id AS id").Where("id=? AND deleted_at IS NULL", materials[0].FolderID).Take(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return material.ErrFolderNotFound
			}
			return err
		}
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", owner.ID).Take(&owner).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("folders").Where("id=? AND deleted_at IS NULL", materials[0].FolderID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return material.ErrFolderNotFound
		}
		return tx.CreateInBatches(&models, 250).Error
	})
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
			"difficulty": m.Difficulty,
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID uuid.UUID }
		if err := tx.Table("folders f").Select("f.owner_id AS id").Joins("JOIN materials m ON m.folder_id=f.id").Where("m.id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", id).Take(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return material.ErrNotFound
			}
			return err
		}
		// Use the same user lock as answer/plan commands. A deletion and an
		// answer therefore commit in a definite order without losing history.
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", owner.ID).Take(&owner).Error; err != nil {
			return err
		}
		result := tx.Delete(&materialModel{}, "id=?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return material.ErrNotFound
		}
		return nil
	})
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
		ID:         m.ID,
		FolderID:   m.FolderID,
		Values:     valuesJSON,
		Metadata:   metadataJSON,
		Difficulty: string(m.Difficulty),
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
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
		ID:         model.ID,
		FolderID:   model.FolderID,
		Values:     values,
		Metadata:   metadata,
		Difficulty: materialmodel.Difficulty(model.Difficulty),
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}, nil
}
