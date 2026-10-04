package interview

import (
	"context"
	"errors"
	"fmt"
	"strings"

	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	"github.com/google/uuid"
)

type CopyRequest struct {
	TargetFolderID uuid.UUID `json:"target_folder_id"`
	AllowDuplicate bool      `json:"allow_duplicate"`
}
type CopyResult struct {
	MaterialID uuid.UUID `json:"material_id"`
	FolderID   uuid.UUID `json:"folder_id"`
	Difficulty string    `json:"-"`
}
type DuplicateQuestion struct{ MaterialID, FolderID uuid.UUID }

func (*DuplicateQuestion) Error() string { return "duplicate_question" }

func (s *Service) CopyQuestion(ctx context.Context, user, folder, source uuid.UUID, req CopyRequest) (CopyResult, error) {
	var out CopyResult
	if req.TargetFolderID == uuid.Nil || req.TargetFolderID == folder {
		return out, ErrInvalid
	}
	err := s.repository.Transact(ctx, user, func(tx Transaction) error {
		if _, err := ownedValues(tx, folder, source); err != nil {
			return err
		}
		if err := ownedFolder(tx, req.TargetFolderID); err != nil {
			return err
		}
		profile, err := tx.Profile(source)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		hasProfile := err == nil
		out, err = tx.CopyContent(source, req.TargetFolderID, req.AllowDuplicate)
		if err != nil {
			return err
		}
		if hasProfile {
			// A copy has its own identity and revision. It must never participate in
			// seed reconciliation for the original question.
			profile.SeedKey = nil
			_, err = saveProfile(tx, user, req.TargetFolderID, out.MaterialID, ProfileRequest{Profile: profile})
		}
		return err
	})
	return out, err
}

func normalizedQuestion(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// The enclosing account transaction serializes duplicate lookup and insertion
// with other copy/create/delete requests. Only content tables are written.
func (t *storeTx) CopyContent(source, target uuid.UUID, allowDuplicate bool) (CopyResult, error) {
	ctx := t.db.Statement.Context
	repo := materialpg.NewRepository(t.db)
	m, err := repo.GetByID(ctx, source)
	if errors.Is(err, material.ErrNotFound) {
		return CopyResult{}, ErrNotFound
	}
	if err != nil {
		return CopyResult{}, err
	}
	if !allowDuplicate {
		question := ""
		if m.Values["question"] != nil {
			question = normalizedQuestion(*m.Values["question"])
		}
		var candidates []struct {
			ID       uuid.UUID
			Question string
		}
		if err = t.db.Table("materials").Select("id, COALESCE(values->>'question', '') AS question").Where("folder_id=? AND deleted_at IS NULL", target).Order("created_at,id").Scan(&candidates).Error; err != nil {
			return CopyResult{}, err
		}
		for _, candidate := range candidates {
			if normalizedQuestion(candidate.Question) == question {
				return CopyResult{}, &DuplicateQuestion{candidate.ID, target}
			}
		}
	}
	// Reuse the existing material schema/ownership validation. The emitter is
	// deliberately unset: the HTTP handler publishes only after this commit.
	created, err := materialservice.NewService(repo, folderpg.NewRepository(t.db)).CreateWithDifficulty(ctx, t.user, target, m.Values, m.Metadata, m.Difficulty)
	if errors.Is(err, material.ErrInvalidValues) || errors.Is(err, material.ErrInvalidMetadata) || errors.Is(err, material.ErrInvalidDifficulty) {
		return CopyResult{}, fmt.Errorf("%w: target folder fields are incompatible with this question", ErrInvalid)
	}
	if errors.Is(err, material.ErrFolderNotFound) {
		return CopyResult{}, ErrNotFound
	}
	return CopyResult{MaterialID: created.ID, FolderID: target, Difficulty: string(created.Difficulty)}, err
}
