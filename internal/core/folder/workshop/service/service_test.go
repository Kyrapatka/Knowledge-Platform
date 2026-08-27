package service

import (
	"context"
	"errors"
	"testing"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	workshop "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop"

	"github.com/google/uuid"
)

type fakeConfigRepository struct {
	getByIDFunc func(
		ctx context.Context,
		id uuid.UUID,
	) (foldermodel.Folder, error)

	updateConfigFunc func(
		ctx context.Context,
		folderID uuid.UUID,
		config folderconfig.FolderConfig,
		expectedVersion int64,
	) (int64, error)
}

func (r *fakeConfigRepository) GetByID(
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

func (r *fakeConfigRepository) UpdateConfig(
	ctx context.Context,
	folderID uuid.UUID,
	config folderconfig.FolderConfig,
	expectedVersion int64,
) (int64, error) {
	if r.updateConfigFunc != nil {
		return r.updateConfigFunc(
			ctx,
			folderID,
			config,
			expectedVersion,
		)
	}

	return expectedVersion + 1, nil
}

func TestService_UpdateConfig(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	currentConfig := testConfig()

	repository := &fakeConfigRepository{
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

			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       ownerID,
				Config:        currentConfig,
				ConfigVersion: 3,
			}, nil
		},

		updateConfigFunc: func(
			ctx context.Context,
			id uuid.UUID,
			config folderconfig.FolderConfig,
			expectedVersion int64,
		) (int64, error) {
			if id != folderID {
				t.Fatalf(
					"expected folder ID %s, got %s",
					folderID,
					id,
				)
			}

			if expectedVersion != 3 {
				t.Fatalf(
					"expected version 3, got %d",
					expectedVersion,
				)
			}

			if config.Schema.Fields[0].Label != "Word" {
				t.Fatalf(
					"expected changed label, got %q",
					config.Schema.Fields[0].Label,
				)
			}

			return 4, nil
		},
	}

	service := NewService(
		repository,
	)

	newConfig := cloneTestConfig(
		currentConfig,
	)

	newConfig.Schema.Fields[0].Label = "Word"

	result, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		3,
		newConfig,
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if result.ConfigVersion != 4 {
		t.Fatalf(
			"expected config version 4, got %d",
			result.ConfigVersion,
		)
	}

	if result.Config.Schema.Fields[0].Label != "Word" {
		t.Fatalf(
			"expected label Word, got %q",
			result.Config.Schema.Fields[0].Label,
		)
	}
}

func TestService_UpdateConfig_InvalidExpectedVersion(t *testing.T) {
	service := NewService(
		&fakeConfigRepository{},
	)

	_, err := service.UpdateConfig(
		context.Background(),
		uuid.New(),
		uuid.New(),
		0,
		testConfig(),
	)

	if !errors.Is(
		err,
		workshop.ErrInvalidConfig,
	) {
		t.Fatalf(
			"expected ErrInvalidConfig, got %v",
			err,
		)
	}
}

func TestService_UpdateConfig_FolderNotFound(t *testing.T) {
	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{},
				folder.ErrNotFound
		},
	}

	service := NewService(
		repository,
	)

	_, err := service.UpdateConfig(
		context.Background(),
		uuid.New(),
		uuid.New(),
		1,
		testConfig(),
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

func TestService_UpdateConfig_ForeignFolder(t *testing.T) {
	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	folderID := uuid.New()

	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       otherOwnerID,
				Config:        testConfig(),
				ConfigVersion: 1,
			}, nil
		},
	}

	service := NewService(
		repository,
	)

	_, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		1,
		testConfig(),
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

func TestService_UpdateConfig_StaleVersionBeforeUpdate(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	updateCalled := false

	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       ownerID,
				Config:        testConfig(),
				ConfigVersion: 5,
			}, nil
		},

		updateConfigFunc: func(
			ctx context.Context,
			folderID uuid.UUID,
			config folderconfig.FolderConfig,
			expectedVersion int64,
		) (int64, error) {
			updateCalled = true
			return 0, nil
		},
	}

	service := NewService(
		repository,
	)

	_, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		4,
		testConfig(),
	)

	if !errors.Is(
		err,
		workshop.ErrConflict,
	) {
		t.Fatalf(
			"expected ErrConflict, got %v",
			err,
		)
	}

	if updateCalled {
		t.Fatal(
			"UpdateConfig must not be called for stale version",
		)
	}
}

func TestService_UpdateConfig_RaceConflict(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       ownerID,
				Config:        testConfig(),
				ConfigVersion: 3,
			}, nil
		},

		updateConfigFunc: func(
			ctx context.Context,
			folderID uuid.UUID,
			config folderconfig.FolderConfig,
			expectedVersion int64,
		) (int64, error) {
			return 0,
				folder.ErrConfigConflict
		},
	}

	service := NewService(
		repository,
	)

	_, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		3,
		testConfig(),
	)

	if !errors.Is(
		err,
		workshop.ErrConflict,
	) {
		t.Fatalf(
			"expected ErrConflict, got %v",
			err,
		)
	}
}

func TestService_UpdateConfig_InvalidConfig(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	updateCalled := false

	currentConfig := testConfig()

	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       ownerID,
				Config:        currentConfig,
				ConfigVersion: 1,
			}, nil
		},

		updateConfigFunc: func(
			ctx context.Context,
			folderID uuid.UUID,
			config folderconfig.FolderConfig,
			expectedVersion int64,
		) (int64, error) {
			updateCalled = true
			return 2, nil
		},
	}

	service := NewService(
		repository,
	)

	newConfig := cloneTestConfig(
		currentConfig,
	)

	newConfig.Schema.Fields =
		newConfig.Schema.Fields[1:]

	_, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		1,
		newConfig,
	)

	if !errors.Is(
		err,
		workshop.ErrInvalidConfig,
	) {
		t.Fatalf(
			"expected ErrInvalidConfig, got %v",
			err,
		)
	}

	if updateCalled {
		t.Fatal(
			"repository UpdateConfig must not be called for invalid config",
		)
	}
}

func TestService_UpdateConfig_RepositoryNotFound(t *testing.T) {
	ownerID := uuid.New()
	folderID := uuid.New()

	repository := &fakeConfigRepository{
		getByIDFunc: func(
			ctx context.Context,
			id uuid.UUID,
		) (foldermodel.Folder, error) {
			return foldermodel.Folder{
				ID:            folderID,
				OwnerID:       ownerID,
				Config:        testConfig(),
				ConfigVersion: 1,
			}, nil
		},

		updateConfigFunc: func(
			ctx context.Context,
			folderID uuid.UUID,
			config folderconfig.FolderConfig,
			expectedVersion int64,
		) (int64, error) {
			return 0,
				folder.ErrNotFound
		},
	}

	service := NewService(
		repository,
	)

	_, err := service.UpdateConfig(
		context.Background(),
		ownerID,
		folderID,
		1,
		testConfig(),
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

func testConfig() folderconfig.FolderConfig {
	return folderconfig.FolderConfig{
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
			},
		},

		MetadataSchema: folderconfig.MetadataSchema{
			Fields: []folderconfig.FieldDefinition{},
		},

		Card: folderconfig.CardConfig{
			QuestionFields: []string{
				"foreign",
			},

			AnswerFields: []string{
				"native",
				"example",
			},
		},
	}
}

func cloneTestConfig(
	config folderconfig.FolderConfig,
) folderconfig.FolderConfig {
	result := config

	result.Schema.Fields = append(
		[]folderconfig.FieldDefinition(nil),
		config.Schema.Fields...,
	)

	result.MetadataSchema.Fields = append(
		[]folderconfig.FieldDefinition(nil),
		config.MetadataSchema.Fields...,
	)

	result.Card.QuestionFields = append(
		[]string(nil),
		config.Card.QuestionFields...,
	)

	result.Card.AnswerFields = append(
		[]string(nil),
		config.Card.AnswerFields...,
	)

	return result
}
