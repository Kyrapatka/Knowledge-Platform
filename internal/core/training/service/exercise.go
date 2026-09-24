package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type ExerciseRequest struct {
	Problem         string `json:"problem"`
	Answer          string `json:"answer"`
	Solution        string `json:"solution"`
	Hint            string `json:"hint"`
	ExpectedVersion int    `json:"expected_version"`
}

func (r ExerciseRequest) validate() error {
	for _, value := range []string{r.Problem, r.Answer, r.Solution} {
		if strings.TrimSpace(value) == "" || len(value) > 64000 {
			return fmt.Errorf("%w: problem, answer and solution must contain 1..64000 bytes", ErrInvalid)
		}
	}
	if len(r.Hint) > 16000 {
		return fmt.Errorf("%w: hint exceeds 16000 bytes", ErrInvalid)
	}
	return nil
}

func (s *Service) CreateExercise(ctx context.Context, user, materialID uuid.UUID, req ExerciseRequest) (model.FormulaExercise, error) {
	var out model.FormulaExercise
	if err := req.validate(); err != nil {
		return out, err
	}
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		now := s.now().UTC()
		out = model.FormulaExercise{ID: uuid.New(), MaterialID: materialID, Problem: req.Problem, Answer: req.Answer, Solution: req.Solution, Hint: req.Hint, Version: 1, CreatedAt: now, UpdatedAt: now}
		return tx.CreateExercise(out)
	})
	return out, err
}
func (s *Service) Exercises(ctx context.Context, user, materialID uuid.UUID, limit, offset int) ([]model.FormulaExercise, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	var out []model.FormulaExercise
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = tx.Exercises(materialID, limit, offset)
		return err
	})
	return out, err
}
func (s *Service) UpdateExercise(ctx context.Context, user, materialID, id uuid.UUID, req ExerciseRequest) (model.FormulaExercise, error) {
	var out model.FormulaExercise
	if err := req.validate(); err != nil {
		return out, err
	}
	if req.ExpectedVersion < 1 {
		return out, ErrInvalid
	}
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = tx.Exercise(materialID, id)
		if err != nil {
			return err
		}
		out.Problem, out.Answer, out.Solution, out.Hint = req.Problem, req.Answer, req.Solution, req.Hint
		out.UpdatedAt = s.now().UTC()
		out.Version = req.ExpectedVersion + 1
		return tx.UpdateExercise(out, req.ExpectedVersion)
	})
	return out, err
}
func (s *Service) DeleteExercise(ctx context.Context, user, materialID, id uuid.UUID, version int) error {
	if version < 1 {
		return ErrInvalid
	}
	return s.transact(ctx, user, func(tx repository.Tx) error { return tx.DeleteExercise(materialID, id, version) })
}
