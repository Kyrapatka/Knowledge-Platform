package service

import (
	"context"
	"errors"
	"testing"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"

	"github.com/google/uuid"
)

type fakeMaterialRepository struct {
	createFunc       func(ctx context.Context, material materialmodel.Material) error
	getByIDFunc      func(ctx context.Context, id uuid.UUID) (materialmodel.Material, error)
	listByFolderFunc func(ctx context.Context, folderID uuid.UUID) ([]materialmodel.Material, error)
	updateFunc       func(ctx context.Context, material materialmodel.Material) error
	deleteFunc       func(ctx context.Context, id uuid.UUID) error
}

func (r *fakeMaterialRepository) Create(
	ctx context.Context,
	m materialmodel.Material,
) error {
	if r.createFunc != nil {
		return r.createFunc(
			ctx,
			m,
		)
	}

	return nil
}

func (r *fakeMaterialRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (materialmodel.Material, error) {
	if r.getByIDFunc != nil {
		return r.getByIDFunc(
			ctx,
			id,
		)
	}

	return materialmodel.Material{}, nil
}

func (r *fakeMaterialRepository) ListByFolder(
	ctx context.Context,
	folderID uuid.UUID,
) ([]materialmodel.Material, error) {
	if r.listByFolderFunc != nil {
		return r.listByFolderFunc(
			ctx,
			folderID,
		)
	}

	return []materialmodel.Material{}, nil
}

func (r *fakeMaterialRepository) Update(
	ctx context.Context,
	m materialmodel.Material,
) error {
	if r.updateFunc != nil {
		return r.updateFunc(
			ctx,
			m,
		)
	}

	return nil
}

func (r *fakeMaterialRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	if r.deleteFunc != nil {
		return r.deleteFunc(
			ctx,
			id,
		)
	}

	return nil
}

type fakeFolderRepository struct {
	getByIDFunc func(ctx context.Context, id uuid.UUID) (foldermodel.Folder, error)
}

func (r *fakeFolderRepository) Create(
	ctx context.Context,
	f foldermodel.Folder,
) error {
	return nil
}

func (r *fakeFolderRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (foldermodel.Folder, error) {
	if r.getByIDFunc != nil {
		return r.getByIDFunc(
			ctx,
			id,
		)
	}

	return foldermodel.Folder{}, nil
}

func (r *fakeFolderRepository) ListByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]foldermodel.Folder, error) {
	return []foldermodel.Folder{}, nil
}

func (r *fakeFolderRepository) Update(
	ctx context.Context,
	f foldermodel.Folder,
) error {
	return nil
}

func (r *fakeFolderRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	return nil
}

func stringPtr(
	value string,
) *string {
	return &value
}

func englishFolder(
	id uuid.UUID,
	ownerID uuid.UUID,
) foldermodel.Folder {
	return foldermodel.Folder{
		ID:      id,
		OwnerID: ownerID,

		TemplateKey: "english_words",

		Config: folderconfig.FolderConfig{
			Schema: folderconfig.MaterialSchema{
				Fields: []folderconfig.FieldDefinition{
					{
						Key:      "foreign",
						Label:    "Foreign",
						Required: true,
						Active:   true,
					},
					{
						Key:      "native",
						Label:    "Native",
						Required: true,
						Active:   true,
					},
					{
						Key:      "example",
						Label:    "Example",
						Required: false,
						Active:   true,
					},
					{
						Key:      "note",
						Label:    "Note",
						Required: false,
						Active:   true,
					},
				},
			},

			MetadataSchema: folderconfig.MetadataSchema{
				Fields: []folderconfig.FieldDefinition{},
			},
		},
	}
}

func interviewFolder(
	id uuid.UUID,
	ownerID uuid.UUID,
) foldermodel.Folder {
	return foldermodel.Folder{
		ID:      id,
		OwnerID: ownerID,

		TemplateKey: "interview_questions",

		Config: folderconfig.FolderConfig{
			Schema: folderconfig.MaterialSchema{
				Fields: []folderconfig.FieldDefinition{
					{
						Key:      "question",
						Label:    "Question",
						Required: true,
						Active:   true,
					},
					{
						Key:      "answer",
						Label:    "Answer",
						Required: true,
						Active:   true,
					},
				},
			},

			MetadataSchema: folderconfig.MetadataSchema{
				Fields: []folderconfig.FieldDefinition{
					{
						Key:      "company",
						Label:    "Company",
						Required: false,
						Active:   true,
					},
					{
						Key:      "level",
						Label:    "Level",
						Required: false,
						Active:   true,
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------
// CREATE
// ---------------------------------------------------------

func TestService_Create(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	var savedMaterial materialmodel.Material

	materialRepository := &fakeMaterialRepository{
		createFunc: func(
			ctx context.Context,
			m materialmodel.Material,
		) error {
			savedMaterial = m
			return nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	values := map[string]*string{
		"foreign": stringPtr("apple"),
		"native":  stringPtr("яблоко"),
		"example": stringPtr("I ate an apple"),
	}

	result, err := service.Create(
		context.Background(),
		ownerID,
		folderID,
		values,
		map[string]*string{},
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if result.ID == uuid.Nil {
		t.Fatal("expected material ID")
	}

	if result.FolderID != folderID {
		t.Fatalf(
			"expected folder ID %s, got %s",
			folderID,
			result.FolderID,
		)
	}

	if savedMaterial.ID != result.ID {
		t.Fatal("repository received different material")
	}

	if savedMaterial.Values["foreign"] == nil {
		t.Fatal("expected foreign value")
	}

	if *savedMaterial.Values["foreign"] != "apple" {
		t.Fatalf(
			"expected apple, got %q",
			*savedMaterial.Values["foreign"],
		)
	}

	if result.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt")
	}

	if result.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt")
	}
}

func TestService_Create_MissingRequiredField(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	service := NewService(
		&fakeMaterialRepository{},
		folderRepository,
	)

	_, err := service.Create(
		context.Background(),
		ownerID,
		folderID,
		map[string]*string{
			"foreign": stringPtr("apple"),
		},
		map[string]*string{},
	)

	if !errors.Is(
		err,
		material.ErrInvalidValues,
	) {
		t.Fatalf(
			"expected ErrInvalidValues, got %v",
			err,
		)
	}
}

func TestService_Create_UnknownField(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	service := NewService(
		&fakeMaterialRepository{},
		folderRepository,
	)

	_, err := service.Create(
		context.Background(),
		ownerID,
		folderID,
		map[string]*string{
			"foreign":   stringPtr("apple"),
			"native":    stringPtr("яблоко"),
			"wtf_field": stringPtr("123"),
		},
		map[string]*string{},
	)

	if !errors.Is(
		err,
		material.ErrInvalidValues,
	) {
		t.Fatalf(
			"expected ErrInvalidValues, got %v",
			err,
		)
	}
}

func TestService_Create_UnknownMetadata(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return interviewFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	service := NewService(
		&fakeMaterialRepository{},
		folderRepository,
	)

	_, err := service.Create(
		context.Background(),
		ownerID,
		folderID,
		map[string]*string{
			"question": stringPtr("What is context.Context?"),
			"answer":   stringPtr("Context carries cancellation."),
		},
		map[string]*string{
			"unknown": stringPtr("value"),
		},
	)

	if !errors.Is(
		err,
		material.ErrInvalidMetadata,
	) {
		t.Fatalf(
			"expected ErrInvalidMetadata, got %v",
			err,
		)
	}
}

func TestService_Create_FolderNotFound(t *testing.T) {
	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{}, folder.ErrNotFound
		},
	}

	service := NewService(
		&fakeMaterialRepository{},
		folderRepository,
	)

	_, err := service.Create(
		context.Background(),
		uuid.New(),
		uuid.New(),
		map[string]*string{},
		map[string]*string{},
	)

	if !errors.Is(
		err,
		material.ErrFolderNotFound,
	) {
		t.Fatalf(
			"expected ErrFolderNotFound, got %v",
			err,
		)
	}
}

func TestService_Create_ForeignFolder(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				otherOwnerID,
			), nil
		},
	}

	service := NewService(
		&fakeMaterialRepository{},
		folderRepository,
	)

	_, err := service.Create(
		context.Background(),
		ownerID,
		folderID,
		map[string]*string{},
		map[string]*string{},
	)

	if !errors.Is(
		err,
		material.ErrFolderNotFound,
	) {
		t.Fatalf(
			"expected ErrFolderNotFound, got %v",
			err,
		)
	}
}

// ---------------------------------------------------------
// LIST
// ---------------------------------------------------------

func TestService_List(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	expectedMaterials := []materialmodel.Material{
		{
			ID:       uuid.New(),
			FolderID: folderID,
		},
		{
			ID:       uuid.New(),
			FolderID: folderID,
		},
	}

	materialRepository := &fakeMaterialRepository{
		listByFolderFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]materialmodel.Material, error) {
			if id != folderID {
				t.Fatalf(
					"expected folder ID %s, got %s",
					folderID,
					id,
				)
			}

			return expectedMaterials, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	result, err := service.List(
		context.Background(),
		ownerID,
		folderID,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if len(result) != 2 {
		t.Fatalf(
			"expected 2 materials, got %d",
			len(result),
		)
	}
}

// ---------------------------------------------------------
// GET
// ---------------------------------------------------------

func TestService_GetByID(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       materialID,
				FolderID: folderID,
			}, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	result, err := service.GetByID(
		context.Background(),
		ownerID,
		folderID,
		materialID,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if result.ID != materialID {
		t.Fatalf(
			"expected material ID %s, got %s",
			materialID,
			result.ID,
		)
	}
}

func TestService_GetByID_WrongFolder(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       id,
				FolderID: uuid.New(),
			}, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	_, err := service.GetByID(
		context.Background(),
		ownerID,
		folderID,
		uuid.New(),
	)

	if !errors.Is(
		err,
		material.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

// ---------------------------------------------------------
// UPDATE
// ---------------------------------------------------------

func TestService_Update_PartialValues(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	existingMaterial := materialmodel.Material{
		ID:       materialID,
		FolderID: folderID,

		Values: map[string]*string{
			"foreign": stringPtr("apple"),
			"native":  stringPtr("яблоко"),
			"example": stringPtr("I eat an apple"),
		},

		Metadata: map[string]*string{},
	}

	var savedMaterial materialmodel.Material

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return existingMaterial, nil
		},

		updateFunc: func(
			ctx context.Context,
			m materialmodel.Material,
		) error {
			savedMaterial = m
			return nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	result, err := service.Update(
		context.Background(),
		ownerID,
		folderID,
		materialID,
		map[string]*string{
			"example": stringPtr("I ate an apple"),
		},
		nil,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if *result.Values["foreign"] != "apple" {
		t.Fatal("foreign field was unexpectedly changed")
	}

	if *result.Values["native"] != "яблоко" {
		t.Fatal("native field was unexpectedly changed")
	}

	if *result.Values["example"] != "I ate an apple" {
		t.Fatalf(
			"expected updated example, got %q",
			*result.Values["example"],
		)
	}

	if *savedMaterial.Values["example"] != "I ate an apple" {
		t.Fatal("repository received wrong material")
	}
}

func TestService_Update_DeleteOptionalField(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       materialID,
				FolderID: folderID,

				Values: map[string]*string{
					"foreign": stringPtr("apple"),
					"native":  stringPtr("яблоко"),
					"example": stringPtr("example"),
				},

				Metadata: map[string]*string{},
			}, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	result, err := service.Update(
		context.Background(),
		ownerID,
		folderID,
		materialID,
		map[string]*string{
			"example": nil,
		},
		nil,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if _, exists := result.Values["example"]; exists {
		t.Fatal("expected example field to be deleted")
	}

	if _, exists := result.Values["foreign"]; !exists {
		t.Fatal("foreign field must remain")
	}

	if _, exists := result.Values["native"]; !exists {
		t.Fatal("native field must remain")
	}
}

func TestService_Update_CannotDeleteRequiredField(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       materialID,
				FolderID: folderID,

				Values: map[string]*string{
					"foreign": stringPtr("apple"),
					"native":  stringPtr("яблоко"),
				},

				Metadata: map[string]*string{},
			}, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	_, err := service.Update(
		context.Background(),
		ownerID,
		folderID,
		materialID,
		map[string]*string{
			"foreign": nil,
		},
		nil,
	)

	if !errors.Is(
		err,
		material.ErrInvalidValues,
	) {
		t.Fatalf(
			"expected ErrInvalidValues, got %v",
			err,
		)
	}
}

func TestService_Update_UnknownField(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       materialID,
				FolderID: folderID,

				Values: map[string]*string{
					"foreign": stringPtr("apple"),
					"native":  stringPtr("яблоко"),
				},

				Metadata: map[string]*string{},
			}, nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	_, err := service.Update(
		context.Background(),
		ownerID,
		folderID,
		materialID,
		map[string]*string{
			"unknown": stringPtr("123"),
		},
		nil,
	)

	if !errors.Is(
		err,
		material.ErrInvalidValues,
	) {
		t.Fatalf(
			"expected ErrInvalidValues, got %v",
			err,
		)
	}
}

// ---------------------------------------------------------
// DELETE
// ---------------------------------------------------------

func TestService_Delete(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()
	materialID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	deleteCalled := false

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       materialID,
				FolderID: folderID,
			}, nil
		},

		deleteFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) error {
			if id != materialID {
				t.Fatalf(
					"expected material ID %s, got %s",
					materialID,
					id,
				)
			}

			deleteCalled = true
			return nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	err := service.Delete(
		context.Background(),
		ownerID,
		folderID,
		materialID,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if !deleteCalled {
		t.Fatal("expected repository Delete to be called")
	}
}

func TestService_Delete_WrongFolder(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	deleteCalled := false

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{
				ID:       id,
				FolderID: uuid.New(),
			}, nil
		},

		deleteFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) error {
			deleteCalled = true
			return nil
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	err := service.Delete(
		context.Background(),
		ownerID,
		folderID,
		uuid.New(),
	)

	if !errors.Is(
		err,
		material.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}

	if deleteCalled {
		t.Fatal("Delete must not be called")
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	folderRepository := &fakeFolderRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return englishFolder(
				folderID,
				ownerID,
			), nil
		},
	}

	materialRepository := &fakeMaterialRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (materialmodel.Material, error) {
			return materialmodel.Material{},
				material.ErrNotFound
		},
	}

	service := NewService(
		materialRepository,
		folderRepository,
	)

	err := service.Delete(
		context.Background(),
		ownerID,
		folderID,
		uuid.New(),
	)

	if !errors.Is(
		err,
		material.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}
