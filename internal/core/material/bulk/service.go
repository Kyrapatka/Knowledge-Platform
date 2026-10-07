package bulk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	trainingmodel "github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var ErrInvalid = errors.New("invalid bulk material request")
var ErrConflict = errors.New("this command was already used with different changes")

type ProfilePatch struct {
	Frequency *int    `json:"frequency,omitempty"`
	Subtopic  *string `json:"subtopic,omitempty"`
	LevelMin  *int    `json:"level_min,omitempty"`
	LevelMax  *int    `json:"level_max,omitempty"`
}
type Request struct {
	CommandID      uuid.UUID                 `json:"command_id"`
	Action         string                    `json:"action"`
	MaterialIDs    []uuid.UUID               `json:"material_ids"`
	TargetFolderID uuid.UUID                 `json:"target_folder_id,omitempty"`
	AllowDuplicate bool                      `json:"allow_duplicate,omitempty"`
	Metadata       map[string]*string        `json:"metadata,omitempty"`
	Difficulty     *materialmodel.Difficulty `json:"difficulty,omitempty"`
	Profile        *ProfilePatch             `json:"profile,omitempty"`
}
type Result struct {
	MaterialIDs []uuid.UUID `json:"material_ids"`
	FolderID    uuid.UUID   `json:"folder_id"`
	Affected    int         `json:"affected"`
}
type Service struct {
	analytics.Emitter
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }
func (r Request) validate() error {
	if r.CommandID == uuid.Nil || len(r.MaterialIDs) < 1 || len(r.MaterialIDs) > 100 {
		return ErrInvalid
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range r.MaterialIDs {
		if id == uuid.Nil || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	if r.Action != "copy" && r.Action != "delete" && r.Action != "metadata" {
		return ErrInvalid
	}
	if r.Action == "copy" && r.TargetFolderID == uuid.Nil {
		return ErrInvalid
	}
	if r.Action != "copy" && (r.TargetFolderID != uuid.Nil || r.AllowDuplicate) {
		return ErrInvalid
	}
	if r.Action != "metadata" && (r.Metadata != nil || r.Difficulty != nil || r.Profile != nil) {
		return ErrInvalid
	}
	if r.Difficulty != nil && !r.Difficulty.Valid() {
		return ErrInvalid
	}
	for k, v := range r.Metadata {
		if k != "topic" && k != "subtopic" && k != "company" && k != "level" {
			return fmt.Errorf("%w: field %s cannot be edited in bulk", ErrInvalid, k)
		}
		if v != nil && len(*v) > 200 {
			return ErrInvalid
		}
	}
	if r.Profile != nil {
		p := r.Profile
		if p.Frequency != nil && (*p.Frequency < 1 || *p.Frequency > 10) || p.LevelMin != nil && (*p.LevelMin < 1 || *p.LevelMin > 5) || p.LevelMax != nil && (*p.LevelMax < 1 || *p.LevelMax > 5) || p.Subtopic != nil && len(*p.Subtopic) > 200 {
			return ErrInvalid
		}
	}
	if r.Action == "metadata" && len(r.Metadata) == 0 && r.Difficulty == nil && r.Profile == nil {
		return ErrInvalid
	}
	return nil
}
func (s *Service) Apply(ctx context.Context, user, folder uuid.UUID, req Request) (Result, error) {
	out := Result{MaterialIDs: []uuid.UUID{}, FolderID: folder}
	if err := req.validate(); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(struct {
		Folder  uuid.UUID
		Request Request
	}{folder, req})
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	events := []analytics.Event{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var identity struct{ ID uuid.UUID }
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", user).Take(&identity).Error; err != nil {
			return err
		}
		var receipt struct {
			RequestHash string
			Response    []byte
		}
		err := tx.Table("material_bulk_commands").Where("user_id=? AND command_id=?", user, req.CommandID).Take(&receipt).Error
		if err == nil {
			if receipt.RequestHash != hash {
				return ErrConflict
			}
			return json.Unmarshal(receipt.Response, &out)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		folders := folderpg.NewRepository(tx)
		source, err := folders.GetByID(ctx, folder)
		if err != nil {
			return err
		}
		if source.OwnerID != user {
			return gorm.ErrRecordNotFound
		}
		repo := materialpg.NewRepository(tx)
		materials := materialservice.NewService(repo, folders)
		profiles := interview.NewService(interview.NewTransactionStore(tx))
		if req.Action == "copy" {
			target, err := folders.GetByID(ctx, req.TargetFolderID)
			if err != nil {
				return err
			}
			if target.OwnerID != user {
				return gorm.ErrRecordNotFound
			}
			if target.TemplateKey != source.TemplateKey || target.ID == source.ID {
				return fmt.Errorf("%w: choose a different folder of the same material type", ErrInvalid)
			}
			out.FolderID = target.ID
		}
		for _, id := range req.MaterialIDs {
			m, err := materials.GetByID(ctx, user, folder, id)
			if err != nil {
				return err
			}
			resultID := id
			switch req.Action {
			case "copy":
				if source.TemplateKey == "interview_questions" {
					result, err := profiles.CopyQuestion(ctx, user, folder, id, interview.CopyRequest{TargetFolderID: req.TargetFolderID, AllowDuplicate: req.AllowDuplicate})
					if err != nil {
						return err
					}
					resultID = result.MaterialID
				} else {
					copy, err := materials.CreateWithDifficulty(ctx, user, req.TargetFolderID, m.Values, m.Metadata, m.Difficulty)
					if err != nil {
						return err
					}
					resultID = copy.ID
					if source.TemplateKey == "formulas" {
						var exercises []trainingmodel.FormulaExercise
						if err := tx.Table("formula_exercises").Where("material_id=?", id).Find(&exercises).Error; err != nil {
							return err
						}
						for _, ex := range exercises {
							ex.ID = uuid.New()
							ex.MaterialID = resultID
							ex.Version = 1
							ex.CreatedAt = time.Now().UTC()
							ex.UpdatedAt = ex.CreatedAt
							if err := tx.Table("formula_exercises").Create(&ex).Error; err != nil {
								return err
							}
						}
					}
				}
				event := analytics.New(analytics.MaterialCreated, user)
				event.MaterialID = resultID.String()
				event.FolderID = req.TargetFolderID.String()
				event.Template = source.TemplateKey
				event.Difficulty = analytics.Ptr(string(m.Difficulty))
				events = append(events, event)
			case "delete":
				if err := materials.Delete(ctx, user, folder, id); err != nil {
					return err
				}
				event := analytics.New(analytics.MaterialDeleted, user)
				event.MaterialID = id.String()
				event.FolderID = folder.String()
				events = append(events, event)
			case "metadata":
				if _, err := materials.UpdateWithDifficulty(ctx, user, folder, id, nil, req.Metadata, req.Difficulty); err != nil {
					return err
				}
				if req.Profile != nil && source.TemplateKey != "interview_questions" {
					return ErrInvalid
				}
				if source.TemplateKey == "interview_questions" {
					profile, err := profiles.Profile(ctx, user, folder, id)
					if err != nil {
						return err
					}
					if req.Profile != nil && profile.ProfileVersion == 0 {
						return fmt.Errorf("%w: set up question profiles before editing profile fields in bulk", ErrInvalid)
					}
					if profile.ProfileVersion > 0 {
						changed := req.Profile != nil
						if v, ok := req.Metadata["topic"]; ok {
							profile.Topic = ""
							if v != nil {
								profile.Topic = *v
							}
							changed = true
						}
						if v, ok := req.Metadata["subtopic"]; ok {
							profile.Subtopic = ""
							if v != nil {
								profile.Subtopic = *v
							}
							changed = true
						}
						if req.Profile != nil {
							p := req.Profile
							if p.Frequency != nil {
								profile.Frequency = *p.Frequency
							}
							if p.Subtopic != nil {
								profile.Subtopic = *p.Subtopic
							}
							if p.LevelMin != nil {
								profile.LevelMin = *p.LevelMin
							}
							if p.LevelMax != nil {
								profile.LevelMax = *p.LevelMax
							}
						}
						if changed {
							_, err = profiles.SaveProfile(ctx, user, folder, id, interview.ProfileRequest{Profile: profile, ExpectedVersion: profile.ProfileVersion})
							if err != nil {
								return err
							}
						}
					}
				}
			}
			out.MaterialIDs = append(out.MaterialIDs, resultID)
		}
		out.Affected = len(out.MaterialIDs)
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Table("material_bulk_commands").Create(map[string]any{"user_id": user, "command_id": req.CommandID, "request_hash": hash, "response": response, "created_at": time.Now().UTC()}).Error
	})
	if err != nil {
		return Result{}, err
	}
	for _, event := range events {
		s.Publish(ctx, event)
	}
	return out, nil
}
func IsInvalid(err error) bool {
	return errors.Is(err, ErrInvalid) || errors.Is(err, material.ErrInvalidValues) || errors.Is(err, material.ErrInvalidMetadata) || errors.Is(err, material.ErrInvalidDifficulty) || errors.Is(err, interview.ErrInvalid)
}
