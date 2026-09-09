package service

import (
	"context"
	"errors"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"time"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

func keyFor(p model.TrainingPlan, materialID uuid.UUID) repository.ProgressKey {
	key := repository.ProgressKey{UserID: p.UserID, MaterialID: materialID, Track: p.Track}
	if p.Track == model.ProgressTrackCram {
		key.PlanID = &p.ID
	}
	return key
}

func (s *Service) ensureProgress(ctx context.Context, tx repository.Tx, p model.TrainingPlan, m material.Material, now time.Time) (model.UserMaterialProgress, error) {
	key := keyFor(p, m.ID)
	progress, err := tx.Progress().Get(ctx, key)
	if errors.Is(err, repository.ErrNotFound) {
		progress, err = model.NewProgress(model.NewProgressParams{UserID: p.UserID, MaterialID: m.ID, Track: p.Track, PlanID: key.PlanID,
			AlgorithmKey: p.AlgorithmKey, AlgorithmVersion: p.AlgorithmVersion, HorizonDays: p.Config.HorizonDays, Now: now})
		if err != nil {
			return progress, err
		}
		err = tx.Progress().Create(ctx, progress)
	}
	if err != nil {
		return progress, err
	}
	if progress.AlgorithmKey != p.AlgorithmKey || progress.AlgorithmVersion != p.AlgorithmVersion {
		return progress, repository.ErrConflict
	}
	return progress, nil
}

func (s *Service) sessionView(ctx context.Context, tx repository.Tx, p model.TrainingPlan, session model.TrainingSession) (model.SessionView, error) {
	out := model.SessionView{Session: session}
	if session.Status == model.StatusActive && p.Status == model.StatusActive {
		items, err := s.fillPool(ctx, tx, p, session)
		if err != nil {
			return out, err
		}
		if len(items) == 0 {
			completed, err := tx.CompletePlanIfReady(p, s.now().UTC())
			if err != nil {
				return out, err
			}
			if completed {
				out.Session, err = tx.Session(session.ID)
				if err != nil {
					return out, err
				}
			}
		}
		out.PoolSize = len(items)
		if len(items) > 0 {
			out.Current = items[0].Presentation
		}
	}
	var err error
	out.Summary, err = tx.Summary(session.ID)
	return out, err
}

func (s *Service) fillPool(ctx context.Context, tx repository.Tx, p model.TrainingPlan, session model.TrainingSession) ([]model.SessionItem, error) {
	now := s.now().UTC()
	items, err := tx.Items(session.ID)
	if err != nil {
		return nil, err
	}
	active := make([]model.SessionItem, 0, p.Config.PoolSize)
	var position int64
	for _, i := range items {
		if i.Position >= position {
			position = i.Position + 1
		}
		if _, err := tx.Material(i.MaterialID); err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return nil, err
			}
			i.State, i.Presentation = "completed", nil
			if err = tx.SaveItem(i); err != nil {
				return nil, err
			}
			continue
		}
		progress, err := tx.Progress().Get(ctx, keyFor(p, i.MaterialID))
		if err != nil {
			return nil, err
		}
		kind, due := progress.DueReview(now)
		if i.Presentation != nil && p.Track != model.ProgressTrackDefault {
			a, _ := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
			if interview, ok := a.(algorithm.Interview); ok && i.Presentation.FinalReview != (kind == model.ReviewStage && interview.IsFinal(progress, now)) {
				i.Presentation = nil
				if err = tx.SaveItem(i); err != nil {
					return nil, err
				}
			}
		}
		if !due {
			i.State = "completed"
			i.Presentation = nil
			if err = tx.SaveItem(i); err != nil {
				return nil, err
			}
			continue
		}
		if i.Presentation != nil && (i.Presentation.ProgressVersion != progress.Version || i.Presentation.Kind != kind) {
			i.Presentation = nil
			if err = tx.SaveItem(i); err != nil {
				return nil, err
			}
		}
		if i.Presentation == nil {
			if p.AlgorithmKey == "formula_adaptive" {
				exercises, err := tx.Exercises(i.MaterialID, 1, 0)
				if err != nil {
					return nil, err
				}
				if len(exercises) == 0 {
					i.State = "completed"
					if err = tx.SaveItem(i); err != nil {
						return nil, err
					}
					continue
				}
			}
			m, err := tx.Material(i.MaterialID)
			if err != nil {
				return nil, err
			}
			c, ok := p.Config.Cards[m.FolderID.String()]
			if !ok || len(fields(c.Card.QuestionFields, c, m.Values)) == 0 || len(fields(c.Card.AnswerFields, c, m.Values)) == 0 {
				i.State = "completed"
				if err = tx.SaveItem(i); err != nil {
					return nil, err
				}
				continue
			}
		}
		active = append(active, i)
	}
	if slots := p.Config.PoolSize - len(active); slots > 0 {
		materials, err := tx.Candidates(p, session.ID, now, slots)
		if err != nil {
			return nil, err
		}
		for _, m := range materials {
			if _, err = s.ensureProgress(ctx, tx, p, m, now); err != nil {
				return nil, err
			}
			i := model.SessionItem{SessionID: session.ID, MaterialID: m.ID, State: "active", Position: position}
			position++
			if err = tx.SaveItem(i); err != nil {
				return nil, err
			}
			active = append(active, i)
		}
	}
	// Only the head is a presented task. Other pool items receive fresh content
	// snapshots when they actually reach the head, not when the pool is filled.
	for len(active) > 0 && active[0].Presentation == nil {
		i := &active[0]
		m, err := tx.Material(i.MaterialID)
		if err != nil {
			return nil, err
		}
		progress, err := tx.Progress().Get(ctx, keyFor(p, m.ID))
		if err != nil {
			return nil, err
		}
		kind, due := progress.DueReview(now)
		if !due {
			return nil, repository.ErrConflict
		}
		c, ok := p.Config.Cards[m.FolderID.String()]
		if !ok {
			return nil, repository.ErrConflict
		}
		question, answer := fields(c.Card.QuestionFields, c, m.Values), fields(c.Card.AnswerFields, c, m.Values)
		if len(question) == 0 || len(answer) == 0 {
			i.State = "completed"
			if err = tx.SaveItem(*i); err != nil {
				return nil, err
			}
			active = active[1:]
			continue
		}
		difficulty := progress.EffectiveDifficulty(m.Difficulty)
		a, err := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
		if err != nil {
			return nil, err
		}
		required, err := a.RequiredCorrect(difficulty)
		if err != nil {
			return nil, err
		}
		final := false
		displayStage, displayCorrect := progress.Stage, progress.ConsecutiveCorrect
		if interview, ok := a.(algorithm.Interview); ok {
			final = kind == model.ReviewStage && interview.IsFinal(progress, now)
			if final {
				schedule, err := interview.Schedule(progress)
				if err != nil {
					return nil, err
				}
				displayStage = len(schedule)
				if displayStage != progress.Stage {
					displayCorrect = 0
				}
			}
		}
		if kind == model.ReviewExtra {
			required = 1
		}
		i.Presentation = &model.Presentation{FinalReview: final, ID: uuid.New(), MaterialID: m.ID, FolderID: m.FolderID, Kind: kind,
			ProgressVersion: progress.Version, Stage: displayStage, ConsecutiveCorrect: displayCorrect,
			RehabConsecutiveCorrect: progress.RehabConsecutiveCorrect, RequiredCorrect: required,
			Difficulty: difficulty, Question: question, Answer: answer, CreatedAt: now}
		if formula, ok := a.(algorithm.Formula); ok {
			exercise, err := tx.PickExercise(m.ID, p.ID)
			if err != nil {
				return nil, err
			}
			mode := formula.Mode(progress, kind)
			i.Presentation.ExerciseID = &exercise.ID
			i.Presentation.ExerciseVersion = exercise.Version
			i.Presentation.PracticeMode = mode
			i.Presentation.Question, i.Presentation.Answer = formulaCard(mode, exercise, c, m.Values, question, answer)
		}
		if err = tx.SaveItem(*i); err != nil {
			return nil, err
		}
	}
	return active, nil
}
