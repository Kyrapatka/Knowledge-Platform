package materialimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Service struct {
	analytics.Emitter
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }
func load(ctx context.Context, db *gorm.DB, user, folder uuid.UUID) (foldermodel.Folder, []materialmodel.Material, error) {
	f, err := folderpg.NewRepository(db).GetByID(ctx, folder)
	if err != nil {
		return f, nil, err
	}
	if f.OwnerID != user {
		return f, nil, gorm.ErrRecordNotFound
	}
	m, err := materialpg.NewRepository(db).ListByFolder(ctx, folder)
	return f, m, err
}
func (s *Service) Validate(ctx context.Context, user, folder uuid.UUID, req Request) (Result, error) {
	f, existing, err := load(ctx, s.db, user, folder)
	if err != nil {
		return Result{}, err
	}
	return validate(f, req, existing), nil
}
func requestHash(folder uuid.UUID, req Request) string {
	b, _ := json.Marshal(struct {
		Operation string
		Folder    uuid.UUID
		Request   Request
	}{"material_import_v1", folder, req})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Service) Import(ctx context.Context, user, folder uuid.UUID, req Request) (Result, error) {
	out := Result{}
	hash := requestHash(folder, req)
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
		err := tx.Table("material_import_commands").Where("user_id=? AND command_id=?", user, req.CommandID).Take(&receipt).Error
		if err == nil {
			if receipt.RequestHash != hash {
				return ErrConflict
			}
			return json.Unmarshal(receipt.Response, &out)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Schema edits lock the folder row, independently of the owner lock.
		// Keep its active configuration stable through validation and commit.
		var lockedFolder struct{ ID uuid.UUID }
		if err := tx.Table("folders").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_id=? AND deleted_at IS NULL", folder, user).Take(&lockedFolder).Error; err != nil {
			return err
		}
		f, existing, err := load(ctx, tx, user, folder)
		if err != nil {
			return err
		}
		out = validate(f, req, existing)
		if !out.Valid {
			return &ValidationError{out}
		}
		inputs := []materialservice.CreateInput{}
		indexes := []int{}
		for i, r := range out.Items {
			if r.Status != "create" {
				continue
			}
			item := normalizeItem(req.Items[i])
			inputs = append(inputs, materialservice.CreateInput{Values: item.Values, Metadata: item.Metadata, Difficulty: item.Difficulty})
			indexes = append(indexes, i)
		}
		materials, err := materialservice.NewService(materialpg.NewRepository(tx), folderpg.NewRepository(tx)).CreateMany(ctx, user, folder, inputs)
		if err != nil {
			return err
		}
		profiles := interview.NewService(interview.NewTransactionStore(tx))
		for i, m := range materials {
			index := indexes[i]
			if f.TemplateKey == "interview_questions" {
				p, err := buildProfile(normalizeItem(req.Items[index]))
				if err != nil {
					return err
				}
				if _, err = profiles.SaveProfile(ctx, user, folder, m.ID, p); err != nil {
					return err
				}
			}
			out.Items[index].Status = "created"
			out.Items[index].MaterialID = m.ID
			e := analytics.New(analytics.MaterialCreated, user)
			e.FolderID = folder.String()
			e.MaterialID = m.ID.String()
			e.Template = f.TemplateKey
			e.Difficulty = analytics.Ptr(string(m.Difficulty))
			events = append(events, e)
		}
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Table("material_import_commands").Create(map[string]any{"user_id": user, "command_id": req.CommandID, "request_hash": hash, "response": response, "created_at": time.Now().UTC()}).Error
	})
	if err != nil {
		return Result{}, err
	}
	for _, event := range events {
		s.Publish(ctx, event)
	}
	return out, nil
}
