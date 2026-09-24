package importer

import (
	"context"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"strings"

	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	materialpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvalidImportError struct {
	Preview Preview
}

func (e *InvalidImportError) Error() string { return "invalid folder import" }

type Result struct {
	FolderID     uuid.UUID `json:"folder_id"`
	FolderName   string    `json:"folder_name"`
	Template     string    `json:"template"`
	ItemsCreated int       `json:"items_created"`
	Warnings     []string  `json:"warnings"`
}

type Service struct {
	analytics.Emitter
	db       *gorm.DB
	registry *foldertemplate.Registry
}

func NewService(db *gorm.DB, registry *foldertemplate.Registry) *Service {
	return &Service{db: db, registry: registry}
}

func (s *Service) Validate(data []byte) Preview {
	_, preview := Parse(data)
	return preview
}

func (s *Service) Import(ctx context.Context, ownerID uuid.UUID, data []byte) (Result, error) {
	plan, preview := Parse(data)
	if !preview.Valid {
		return Result{}, &InvalidImportError{Preview: preview}
	}
	result := Result{Template: plan.Template, Warnings: preview.Warnings}
	var events []analytics.Event
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user struct{ ID uuid.UUID }
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", ownerID).Take(&user).Error; err != nil {
			return err
		}

		folderRepository := folderpostgres.NewRepository(tx)
		folderService := folderservice.NewService(folderRepository, s.registry)
		folders, err := folderService.List(ctx, ownerID)
		if err != nil {
			return err
		}
		name := availableFolderName(plan.FolderName, folders)
		createdFolder, err := folderService.Create(ctx, ownerID, name, plan.Description, plan.Template)
		if err != nil {
			return err
		}

		inputs := make([]materialservice.CreateInput, 0, len(plan.Items))
		for _, item := range plan.Items {
			inputs = append(inputs, materialservice.CreateInput{Values: item.Values, Metadata: item.Metadata, Difficulty: item.Difficulty})
		}
		materialService := materialservice.NewService(materialpostgres.NewRepository(tx), folderRepository)
		materials, err := materialService.CreateMany(ctx, ownerID, createdFolder.ID, inputs)
		if err != nil {
			return err
		}
		if plan.Template == TemplateInterviewQuestions {
			profiles := interview.NewStore(tx)
			for index, materialEntity := range materials {
				if plan.Items[index].Profile == nil {
					return errors.New("interview import plan has no profile")
				}
				if _, err := profiles.SaveProfileTx(ctx, ownerID, createdFolder.ID, materialEntity.ID, *plan.Items[index].Profile); err != nil {
					return fmt.Errorf("create interview profile for item %d: %w", index+1, err)
				}
			}
		}
		result.FolderID = createdFolder.ID
		result.FolderName = createdFolder.Title
		result.ItemsCreated = len(materials)
		event := analytics.New(analytics.FolderImported, ownerID)
		event.FolderID, event.Template = createdFolder.ID.String(), plan.Template
		events = append(events, event)
		for _, m := range materials {
			e := analytics.New(analytics.MaterialCreated, ownerID)
			e.FolderID, e.MaterialID, e.Template = createdFolder.ID.String(), m.ID.String(), plan.Template
			e.Difficulty = analytics.Ptr(string(m.Difficulty))
			events = append(events, e)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	// Nested services intentionally retain their no-op publishers until commit.
	for _, e := range events {
		s.Publish(ctx, e)
	}
	return result, nil
}

func availableFolderName(name string, folders []foldermodel.Folder) string {
	used := make(map[string]bool, len(folders))
	for _, folder := range folders {
		used[strings.ToLower(strings.TrimSpace(folder.Title))] = true
	}
	if !used[strings.ToLower(name)] {
		return name
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s (%d)", name, suffix)
		if !used[strings.ToLower(candidate)] {
			return candidate
		}
	}
}
