package service

import (
	"context"
	"errors"
	"testing"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"

	"github.com/google/uuid"
)

type fakeRepository struct {
	createFunc      func(ctx context.Context, folder foldermodel.Folder) error
	getByIDFunc     func(ctx context.Context, id uuid.UUID) (foldermodel.Folder, error)
	listByOwnerFunc func(ctx context.Context, ownerID uuid.UUID) ([]foldermodel.Folder, error)
	updateFunc      func(ctx context.Context, folder foldermodel.Folder) error
	deleteFunc      func(ctx context.Context, id uuid.UUID) error
}

func (r *fakeRepository) Create(
	ctx context.Context,
	folder foldermodel.Folder,
) error {
	if r.createFunc != nil {
		return r.createFunc(
			ctx,
			folder,
		)
	}

	return nil
}

func (r *fakeRepository) GetByID(
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

func (r *fakeRepository) ListByOwner(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]foldermodel.Folder, error) {
	if r.listByOwnerFunc != nil {
		return r.listByOwnerFunc(
			ctx,
			ownerID,
		)
	}

	return []foldermodel.Folder{}, nil
}

func (r *fakeRepository) Update(
	ctx context.Context,
	folder foldermodel.Folder,
) error {
	if r.updateFunc != nil {
		return r.updateFunc(
			ctx,
			folder,
		)
	}

	return nil
}

func (r *fakeRepository) Delete(
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

func newTestService(
	repository *fakeRepository,
) *Service {
	templateRegistry := foldertemplate.NewRegistry(
		foldertemplate.DefaultTemplates(),
	)

	return NewService(
		repository,
		templateRegistry,
	)
}

func TestService_Create(t *testing.T) {
	ownerID := uuid.New()

	var savedFolder foldermodel.Folder

	repository := &fakeRepository{
		createFunc: func(
			ctx context.Context,
			folder foldermodel.Folder,
		) error {
			savedFolder = folder
			return nil
		},
	}

	service := newTestService(repository)

	createdFolder, err := service.Create(
		context.Background(),
		ownerID,
		"English",
		"My English words",
		"english_words",
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if createdFolder.ID == uuid.Nil {
		t.Fatal("expected folder ID")
	}

	if createdFolder.OwnerID != ownerID {
		t.Fatalf(
			"expected owner %s, got %s",
			ownerID,
			createdFolder.OwnerID,
		)
	}

	if createdFolder.Title != "English" {
		t.Fatalf(
			"expected title %q, got %q",
			"English",
			createdFolder.Title,
		)
	}

	if createdFolder.Description != "My English words" {
		t.Fatalf(
			"expected description %q, got %q",
			"My English words",
			createdFolder.Description,
		)
	}

	if createdFolder.TemplateKey != "english_words" {
		t.Fatalf(
			"expected template key %q, got %q",
			"english_words",
			createdFolder.TemplateKey,
		)
	}

	if len(createdFolder.Config.Schema.Fields) == 0 {
		t.Fatal("expected folder config schema")
	}

	if len(createdFolder.Config.Card.QuestionFields) == 0 {
		t.Fatal("expected card question fields")
	}

	if savedFolder.ID != createdFolder.ID {
		t.Fatal("repository received different folder")
	}

	if createdFolder.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt")
	}

	if createdFolder.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt")
	}
}

func TestService_Create_InvalidTemplate(t *testing.T) {
	repository := &fakeRepository{}

	service := newTestService(repository)

	_, err := service.Create(
		context.Background(),
		uuid.New(),
		"Folder",
		"",
		"abracadabra",
	)

	if !errors.Is(
		err,
		folder.ErrInvalidTemplate,
	) {
		t.Fatalf(
			"expected ErrInvalidTemplate, got %v",
			err,
		)
	}
}

func TestService_Create_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")

	repository := &fakeRepository{
		createFunc: func(
			ctx context.Context,
			folder foldermodel.Folder,
		) error {
			return expectedErr
		},
	}

	service := newTestService(repository)

	_, err := service.Create(
		context.Background(),
		uuid.New(),
		"English",
		"",
		"english_words",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(
		err,
		expectedErr,
	) {
		t.Fatalf(
			"expected repository error, got %v",
			err,
		)
	}
}

func TestService_GetByID(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	expectedFolder := foldermodel.Folder{
		ID:          folderID,
		OwnerID:     ownerID,
		Title:       "English",
		Description: "Words",
		TemplateKey: "english_words",
	}

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			if id != folderID {
				t.Fatalf(
					"expected folder ID %s, got %s",
					folderID,
					id,
				)
			}

			return expectedFolder, nil
		},
	}

	service := newTestService(repository)

	result, err := service.GetByID(
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

	if result.ID != folderID {
		t.Fatalf(
			"expected folder ID %s, got %s",
			folderID,
			result.ID,
		)
	}
}

func TestService_GetByID_ForeignOwner(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	folderID := uuid.New()

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:      folderID,
				OwnerID: otherOwnerID,
			}, nil
		},
	}

	service := newTestService(repository)

	_, err := service.GetByID(
		context.Background(),
		ownerID,
		folderID,
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{}, folder.ErrNotFound
		},
	}

	service := newTestService(repository)

	_, err := service.GetByID(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestService_List(t *testing.T) {
	ownerID := uuid.New()

	expectedFolders := []foldermodel.Folder{
		{
			ID:      uuid.New(),
			OwnerID: ownerID,
			Title:   "English",
		},
		{
			ID:      uuid.New(),
			OwnerID: ownerID,
			Title:   "Go",
		},
	}

	repository := &fakeRepository{
		listByOwnerFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]foldermodel.Folder, error) {
			if id != ownerID {
				t.Fatalf(
					"expected owner ID %s, got %s",
					ownerID,
					id,
				)
			}

			return expectedFolders, nil
		},
	}

	service := newTestService(repository)

	result, err := service.List(
		context.Background(),
		ownerID,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if len(result) != 2 {
		t.Fatalf(
			"expected 2 folders, got %d",
			len(result),
		)
	}
}

func TestService_List_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")

	repository := &fakeRepository{
		listByOwnerFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]foldermodel.Folder, error) {
			return nil, expectedErr
		},
	}

	service := newTestService(repository)

	_, err := service.List(
		context.Background(),
		uuid.New(),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(
		err,
		expectedErr,
	) {
		t.Fatalf(
			"expected repository error, got %v",
			err,
		)
	}
}

func TestService_Update(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	existingFolder := foldermodel.Folder{
		ID:          folderID,
		OwnerID:     ownerID,
		Title:       "Old title",
		Description: "Old description",
		TemplateKey: "english_words",
	}

	var savedFolder foldermodel.Folder

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return existingFolder, nil
		},

		updateFunc: func(
			ctx context.Context,
			folder foldermodel.Folder,
		) error {
			savedFolder = folder
			return nil
		},
	}

	service := newTestService(repository)

	result, err := service.Update(
		context.Background(),
		ownerID,
		folderID,
		"New title",
		"New description",
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if result.Title != "New title" {
		t.Fatalf(
			"expected title %q, got %q",
			"New title",
			result.Title,
		)
	}

	if result.Description != "New description" {
		t.Fatalf(
			"expected description %q, got %q",
			"New description",
			result.Description,
		)
	}

	if savedFolder.Title != "New title" {
		t.Fatalf(
			"repository received title %q",
			savedFolder.Title,
		)
	}

	if savedFolder.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt")
	}
}

func TestService_Update_ForeignOwner(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:      id,
				OwnerID: otherOwnerID,
			}, nil
		},
	}

	service := newTestService(repository)

	_, err := service.Update(
		context.Background(),
		ownerID,
		uuid.New(),
		"Title",
		"Description",
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestService_Update_NotFound(t *testing.T) {
	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{}, folder.ErrNotFound
		},
	}

	service := newTestService(repository)

	_, err := service.Update(
		context.Background(),
		uuid.New(),
		uuid.New(),
		"Title",
		"Description",
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestService_Delete(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	deleteCalled := false

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:      folderID,
				OwnerID: ownerID,
			}, nil
		},

		deleteFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) error {
			if id != folderID {
				t.Fatalf(
					"expected folder ID %s, got %s",
					folderID,
					id,
				)
			}

			deleteCalled = true
			return nil
		},
	}

	service := newTestService(repository)

	err := service.Delete(
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

	if !deleteCalled {
		t.Fatal("expected repository Delete to be called")
	}
}

func TestService_Delete_ForeignOwner(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()

	deleteCalled := false

	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:      id,
				OwnerID: otherOwnerID,
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

	service := newTestService(repository)

	err := service.Delete(
		context.Background(),
		ownerID,
		uuid.New(),
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}

	if deleteCalled {
		t.Fatal("Delete must not be called for foreign folder")
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	repository := &fakeRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{}, folder.ErrNotFound
		},
	}

	service := newTestService(repository)

	err := service.Delete(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)

	if !errors.Is(
		err,
		folder.ErrNotFound,
	) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}
