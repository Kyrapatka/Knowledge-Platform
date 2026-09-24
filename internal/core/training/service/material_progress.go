package service

import (
	"context"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"time"
)

type MaterialProgressView struct {
	MaterialID    uuid.UUID  `json:"material_id"`
	Version       int        `json:"version"`
	Stage         int        `json:"stage"`
	TargetAt      *time.Time `json:"target_at"`
	StageReviewAt *time.Time `json:"stage_review_at"`
	NextReviewAt  *time.Time `json:"next_review_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CanStartFinal bool       `json:"can_start_final"`
}

// Returns existing progress only; reading a material does not start its horizon.
func (s *Service) MaterialProgress(ctx context.Context, user, sessionID, materialID uuid.UUID) (MaterialProgressView, error) {
	var out MaterialProgressView
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		session, err := tx.Session(sessionID)
		if err != nil {
			return err
		}
		plan, err := tx.Plan(session.PlanID)
		if err != nil {
			return err
		}
		m, err := tx.Material(materialID)
		if err != nil {
			return err
		}
		if _, ok := plan.Config.Cards[m.FolderID.String()]; !ok {
			return repository.ErrNotFound
		}
		p, err := tx.Progress().Get(ctx, keyFor(plan, materialID))
		if err != nil {
			return err
		}
		out = MaterialProgressView{MaterialID: materialID, Version: p.Version, Stage: p.Stage, TargetAt: p.TargetAt, StageReviewAt: p.StageReviewAt, NextReviewAt: p.NextReviewAt(), CompletedAt: p.CompletedAt}
		a, err := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
		if err != nil {
			return err
		}
		if interview, ok := a.(algorithm.Interview); ok {
			schedule, err := interview.Schedule(p)
			if err != nil {
				return err
			}
			out.CanStartFinal = p.CompletedAt == nil && p.Stage == len(schedule) && plan.Status == model.StatusActive && session.Status == model.StatusActive
		}
		return nil
	})
	return out, err
}
