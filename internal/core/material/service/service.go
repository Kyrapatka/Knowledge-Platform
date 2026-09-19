package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderrepository "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialrepository "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository"

	"github.com/google/uuid"
)

type Service struct {
	materialRepository materialrepository.Repository
	folderRepository   folderrepository.Repository
}

// CreateInput is the regular material creation contract used by both the HTTP
// create flow and transactional bulk operations such as folder import.
type CreateInput struct {
	Values     map[string]*string
	Metadata   map[string]*string
	Difficulty materialmodel.Difficulty
}

type batchRepository interface {
	CreateBatch(ctx context.Context, materials []materialmodel.Material) error
}

func NewService(
	materialRepository materialrepository.Repository,
	folderRepository folderrepository.Repository,
) *Service {
	return &Service{
		materialRepository: materialRepository,
		folderRepository:   folderRepository,
	}
}

func (s *Service) Create(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	values map[string]*string,
	metadata map[string]*string,
) (materialmodel.Material, error) {
	return s.CreateWithDifficulty(ctx, ownerID, folderID, values, metadata, materialmodel.DifficultyMedium)
}

func (s *Service) CreateWithDifficulty(
	ctx context.Context, ownerID, folderID uuid.UUID,
	values, metadata map[string]*string, difficulty materialmodel.Difficulty,
) (materialmodel.Material, error) {
	if !difficulty.Valid() {
		return materialmodel.Material{}, material.ErrInvalidDifficulty
	}
	folderEntity, err := s.getOwnedFolder(
		ctx,
		ownerID,
		folderID,
	)
	if err != nil {
		return materialmodel.Material{}, err
	}

	if values == nil {
		values = make(map[string]*string)
	}

	if metadata == nil {
		metadata = make(map[string]*string)
	}

	if err := validateCreateFields(
		folderEntity.Config.Schema.Fields,
		values,
		material.ErrInvalidValues,
		false,
	); err != nil {
		return materialmodel.Material{}, err
	}

	if err := validateCreateFields(
		folderEntity.Config.MetadataSchema.Fields,
		metadata,
		material.ErrInvalidMetadata,
		true,
	); err != nil {
		return materialmodel.Material{}, err
	}

	now := time.Now().UTC()

	m := materialmodel.Material{
		ID:         uuid.New(),
		FolderID:   folderID,
		Values:     values,
		Metadata:   metadata,
		Difficulty: difficulty,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.materialRepository.Create(
		ctx,
		m,
	); err != nil {
		return materialmodel.Material{}, fmt.Errorf(
			"create material: %w",
			err,
		)
	}

	return m, nil
}

// CreateMany applies the same folder ownership and schema validation as
// CreateWithDifficulty, but persists all materials as one repository batch.
// The caller can wrap this operation together with folder creation in a wider
// database transaction.
func (s *Service) CreateMany(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	inputs []CreateInput,
) ([]materialmodel.Material, error) {
	folderEntity, err := s.getOwnedFolder(ctx, ownerID, folderID)
	if err != nil {
		return nil, err
	}

	materials := make([]materialmodel.Material, 0, len(inputs))
	now := time.Now().UTC()
	for _, input := range inputs {
		if !input.Difficulty.Valid() {
			return nil, material.ErrInvalidDifficulty
		}
		values := input.Values
		if values == nil {
			values = make(map[string]*string)
		}
		metadata := input.Metadata
		if metadata == nil {
			metadata = make(map[string]*string)
		}
		if err := validateCreateFields(folderEntity.Config.Schema.Fields, values, material.ErrInvalidValues, false); err != nil {
			return nil, err
		}
		if err := validateCreateFields(folderEntity.Config.MetadataSchema.Fields, metadata, material.ErrInvalidMetadata, true); err != nil {
			return nil, err
		}
		materials = append(materials, materialmodel.Material{
			ID:         uuid.New(),
			FolderID:   folderID,
			Values:     values,
			Metadata:   metadata,
			Difficulty: input.Difficulty,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if repository, ok := s.materialRepository.(batchRepository); ok {
		if err := repository.CreateBatch(ctx, materials); err != nil {
			return nil, fmt.Errorf("create materials: %w", err)
		}
		return materials, nil
	}
	for _, item := range materials {
		if err := s.materialRepository.Create(ctx, item); err != nil {
			return nil, fmt.Errorf("create material: %w", err)
		}
	}
	return materials, nil
}

func (s *Service) List(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) ([]materialmodel.Material, error) {
	if _, err := s.getOwnedFolder(
		ctx,
		ownerID,
		folderID,
	); err != nil {
		return nil, err
	}

	materials, err := s.materialRepository.ListByFolder(
		ctx,
		folderID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list materials: %w",
			err,
		)
	}

	return materials, nil
}

func (s *Service) GetByID(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	materialID uuid.UUID,
) (materialmodel.Material, error) {
	if _, err := s.getOwnedFolder(
		ctx,
		ownerID,
		folderID,
	); err != nil {
		return materialmodel.Material{}, err
	}

	m, err := s.materialRepository.GetByID(
		ctx,
		materialID,
	)
	if err != nil {
		if errors.Is(
			err,
			material.ErrNotFound,
		) {
			return materialmodel.Material{},
				material.ErrNotFound
		}

		return materialmodel.Material{}, fmt.Errorf(
			"get material: %w",
			err,
		)
	}

	if m.FolderID != folderID {
		return materialmodel.Material{},
			material.ErrNotFound
	}

	return m, nil
}

func (s *Service) Update(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	materialID uuid.UUID,
	valuesPatch map[string]*string,
	metadataPatch map[string]*string,
) (materialmodel.Material, error) {
	return s.UpdateWithDifficulty(ctx, ownerID, folderID, materialID, valuesPatch, metadataPatch, nil)
}

func (s *Service) UpdateWithDifficulty(
	ctx context.Context, ownerID, folderID, materialID uuid.UUID,
	valuesPatch, metadataPatch map[string]*string, difficulty *materialmodel.Difficulty,
) (materialmodel.Material, error) {
	if difficulty != nil && !difficulty.Valid() {
		return materialmodel.Material{}, material.ErrInvalidDifficulty
	}
	folderEntity, err := s.getOwnedFolder(
		ctx,
		ownerID,
		folderID,
	)
	if err != nil {
		return materialmodel.Material{}, err
	}

	m, err := s.materialRepository.GetByID(
		ctx,
		materialID,
	)
	if err != nil {
		if errors.Is(
			err,
			material.ErrNotFound,
		) {
			return materialmodel.Material{},
				material.ErrNotFound
		}

		return materialmodel.Material{}, fmt.Errorf(
			"get material: %w",
			err,
		)
	}

	if m.FolderID != folderID {
		return materialmodel.Material{},
			material.ErrNotFound
	}

	if valuesPatch == nil &&
		metadataPatch == nil && difficulty == nil {

		return m, nil
	}
	if difficulty != nil {
		m.Difficulty = *difficulty
	}

	if valuesPatch != nil {
		if err := validatePatchFields(
			folderEntity.Config.Schema.Fields,
			valuesPatch,
			material.ErrInvalidValues,
			false,
		); err != nil {
			return materialmodel.Material{}, err
		}

		if m.Values == nil {
			m.Values = make(
				map[string]*string,
			)
		}

		applyPatch(
			m.Values,
			valuesPatch,
		)
	}

	if metadataPatch != nil {
		if err := validatePatchFields(
			folderEntity.Config.MetadataSchema.Fields,
			metadataPatch,
			material.ErrInvalidMetadata,
			true,
		); err != nil {
			return materialmodel.Material{}, err
		}

		if m.Metadata == nil {
			m.Metadata = make(
				map[string]*string,
			)
		}

		applyPatch(
			m.Metadata,
			metadataPatch,
		)
	}

	m.UpdatedAt = time.Now().UTC()

	if err := s.materialRepository.Update(
		ctx,
		m,
	); err != nil {
		if errors.Is(
			err,
			material.ErrNotFound,
		) {
			return materialmodel.Material{},
				material.ErrNotFound
		}

		return materialmodel.Material{}, fmt.Errorf(
			"update material: %w",
			err,
		)
	}

	return m, nil
}

func (s *Service) Delete(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	materialID uuid.UUID,
) error {
	if _, err := s.getOwnedFolder(
		ctx,
		ownerID,
		folderID,
	); err != nil {
		return err
	}

	m, err := s.materialRepository.GetByID(
		ctx,
		materialID,
	)
	if err != nil {
		if errors.Is(
			err,
			material.ErrNotFound,
		) {
			return material.ErrNotFound
		}

		return fmt.Errorf(
			"get material: %w",
			err,
		)
	}

	if m.FolderID != folderID {
		return material.ErrNotFound
	}

	if err := s.materialRepository.Delete(
		ctx,
		materialID,
	); err != nil {
		if errors.Is(
			err,
			material.ErrNotFound,
		) {
			return material.ErrNotFound
		}

		return fmt.Errorf(
			"delete material: %w",
			err,
		)
	}

	return nil
}

func (s *Service) getOwnedFolder(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) (foldermodel.Folder, error) {
	folderEntity, err := s.folderRepository.GetByID(
		ctx,
		folderID,
	)
	if err != nil {
		if errors.Is(
			err,
			folder.ErrNotFound,
		) {
			return foldermodel.Folder{},
				material.ErrFolderNotFound
		}

		return foldermodel.Folder{},
			fmt.Errorf(
				"get folder: %w",
				err,
			)
	}

	if folderEntity.OwnerID != ownerID {
		return foldermodel.Folder{},
			material.ErrFolderNotFound
	}

	return folderEntity, nil
}

func validateCreateFields(
	fields []folderconfig.FieldDefinition,
	values map[string]*string,
	baseErr error,
	metadata bool,
) error {
	allowed := make(
		map[string]folderconfig.FieldDefinition,
		len(fields),
	)

	for _, field := range fields {
		allowed[field.Key] = field
	}

	// Любой существующий key разрешён,
	// даже если Active=false.
	//
	// Active управляет отображением,
	// а не правом доступа к данным.
	for key := range values {
		if _, exists := allowed[key]; !exists {
			if metadata {
				return fmt.Errorf(
					"%w: unknown metadata field %q",
					baseErr,
					key,
				)
			}

			return fmt.Errorf(
				"%w: unknown field %q",
				baseErr,
				key,
			)
		}
	}

	// Required применяется только к активным полям.
	//
	// Иначе hidden required field сделал бы
	// создание Material невозможным через обычный UI.
	for _, field := range fields {
		if !field.Active {
			continue
		}

		if !field.Required {
			continue
		}

		value, exists := values[field.Key]

		if !exists ||
			value == nil ||
			*value == "" {

			if metadata {
				return fmt.Errorf(
					"%w: required metadata field %q",
					baseErr,
					field.Key,
				)
			}

			return fmt.Errorf(
				"%w: required field %q",
				baseErr,
				field.Key,
			)
		}
	}

	return nil
}

func validatePatchFields(
	fields []folderconfig.FieldDefinition,
	patch map[string]*string,
	baseErr error,
	metadata bool,
) error {
	allowed := make(
		map[string]folderconfig.FieldDefinition,
		len(fields),
	)

	for _, field := range fields {
		allowed[field.Key] = field
	}

	for key, value := range patch {
		field, exists := allowed[key]

		if !exists {
			if metadata {
				return fmt.Errorf(
					"%w: unknown metadata field %q",
					baseErr,
					key,
				)
			}

			return fmt.Errorf(
				"%w: unknown field %q",
				baseErr,
				key,
			)
		}

		// Inactive поле можно менять и удалять.
		//
		// Required начинает ограничивать PATCH
		// только когда поле активно.
		if field.Active &&
			field.Required &&
			(value == nil || *value == "") {

			if metadata {
				return fmt.Errorf(
					"%w: required metadata field %q cannot be empty",
					baseErr,
					key,
				)
			}

			return fmt.Errorf(
				"%w: required field %q cannot be empty",
				baseErr,
				key,
			)
		}
	}

	return nil
}

func applyPatch(
	target map[string]*string,
	patch map[string]*string,
) {
	for key, value := range patch {
		if value == nil {
			delete(
				target,
				key,
			)

			continue
		}

		target[key] = value
	}
}
