package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type ChangeRequest struct {
	EndActiveSession bool                 `json:"end_active_session,omitempty"`
	PoolSize         *int                 `json:"pool_size,omitempty"`
	CommandID        uuid.UUID            `json:"command_id"`
	ExpectedVersion  int                  `json:"expected_version"`
	AlgorithmKey     string               `json:"algorithm_key"`
	AlgorithmVersion int                  `json:"algorithm_version"`
	Mode             algorithm.ChangeMode `json:"mode"`
	Stage            int                  `json:"stage"`
	HorizonDays      *int                 `json:"horizon_days,omitempty"`
}
type ProgressChange struct {
	MaterialID    uuid.UUID  `json:"material_id"`
	StageBefore   int        `json:"stage_before"`
	StageAfter    int        `json:"stage_after"`
	VersionBefore int        `json:"version_before"`
	VersionAfter  int        `json:"version_after"`
	NextReviewAt  *time.Time `json:"next_review_at"`
}
type ChangeResult struct {
	Plan    model.TrainingPlan `json:"plan"`
	Changes []ProgressChange   `json:"changes"`
}

func (s *Service) ChangeAlgorithm(ctx context.Context, user, planID uuid.UUID, req ChangeRequest) (ChangeResult, error) {
	var out ChangeResult
	if req.CommandID == uuid.Nil || req.ExpectedVersion < 1 {
		return out, ErrInvalid
	}
	if req.PoolSize != nil && (*req.PoolSize < 1 || *req.PoolSize > 50) {
		return out, ErrInvalid
	}
	if req.Mode != algorithm.KeepStage && req.Mode != algorithm.ResetStage && req.Mode != algorithm.SetStage {
		return out, ErrInvalid
	}
	if (req.Mode == algorithm.SetStage && req.Stage < 1) || (req.Mode != algorithm.SetStage && req.Stage != 0) {
		return out, ErrInvalid
	}
	if req.AlgorithmVersion == 0 {
		req.AlgorithmVersion = 1
	}
	a, err := s.registry.Get(req.AlgorithmKey, req.AlgorithmVersion)
	if err != nil {
		return out, errors.Join(ErrInvalid, err)
	}
	raw, _ := json.Marshal(struct {
		Operation string
		PlanID    uuid.UUID
		Request   ChangeRequest
	}{"change_algorithm", planID, req})
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	err = s.store.Transact(ctx, user, func(tx repository.Tx) error {
		plan, err := tx.Plan(planID)
		if err != nil {
			return err
		}
		receipt, err := tx.Receipt(req.CommandID)
		if err == nil {
			if receipt.RequestHash != hash {
				return repository.ErrConflict
			}
			return json.Unmarshal(receipt.Response, &out)
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if plan.Status != model.StatusActive || plan.Version != req.ExpectedVersion {
			return repository.ErrConflict
		}
		if active, activeErr := tx.ActiveSession(planID); activeErr == nil {
			if !req.EndActiveSession {
				return fmt.Errorf("%w: finish or cancel the active session first", repository.ErrConflict)
			}
			if err = tx.FinishSession(active.ID, model.StatusCancelled, s.now().UTC()); err != nil {
				return err
			}
		} else if !errors.Is(activeErr, repository.ErrNotFound) {
			return activeErr
		}
		if algorithm.TrackFor(a) != plan.Track {
			return fmt.Errorf("%w: changing tracks requires a new plan", ErrInvalid)
		}
		horizon := plan.Config.HorizonDays
		if req.HorizonDays != nil {
			horizon = *req.HorizonDays
		}
		if (plan.Track == model.ProgressTrackDefault && horizon != 0) || (plan.Track == model.ProgressTrackCram && (horizon < 1 || horizon > 7)) || (plan.Track == model.ProgressTrackLongTerm && (horizon < 7 || horizon > 365)) {
			return ErrInvalid
		}
		// Validate SET even when no source material has been admitted yet.
		probe, err := model.NewProgress(model.NewProgressParams{UserID: user, MaterialID: uuid.New(), Track: plan.Track, PlanID: keyFor(plan, uuid.Nil).PlanID, AlgorithmKey: a.Key(), AlgorithmVersion: a.Version(), HorizonDays: horizon, Now: s.now().UTC()})
		if err != nil {
			return errors.Join(ErrInvalid, err)
		}
		if req.Mode == algorithm.SetStage {
			probe.Stage = req.Stage
		}
		if _, err = algorithm.ChangeInterval(a, probe); err != nil {
			return errors.Join(ErrInvalid, err)
		}
		progresses, err := tx.PlanProgress(plan)
		if err != nil {
			return err
		}
		out.Changes = make([]ProgressChange, 0, len(progresses))
		now := s.now().UTC()
		for _, before := range progresses {
			// Pool-only changes retain recovery and partial mastery as well as dates.
			if req.Mode == algorithm.KeepStage && a.Key() == plan.AlgorithmKey && a.Version() == plan.AlgorithmVersion && horizon == plan.Config.HorizonDays {
				continue
			}
			if before.AlgorithmKey != plan.AlgorithmKey || before.AlgorithmVersion != plan.AlgorithmVersion {
				return repository.ErrConflict
			}
			after, err := algorithm.ChangeProgress(before, a, req.Mode, req.Stage, req.HorizonDays, now)
			if err != nil {
				return errors.Join(ErrInvalid, err)
			}
			after, err = tx.Progress().Update(ctx, after, before.Version)
			if err != nil {
				return err
			}
			out.Changes = append(out.Changes, ProgressChange{before.MaterialID, before.Stage, after.Stage, before.Version, after.Version, after.NextReviewAt()})
		}
		previousKey, previousVersion := plan.AlgorithmKey, plan.AlgorithmVersion
		plan.AlgorithmKey, plan.AlgorithmVersion = a.Key(), a.Version()
		plan.Config.HorizonDays = horizon
		if req.PoolSize != nil {
			plan.Config.PoolSize = *req.PoolSize
		}
		plan.Version++
		plan.UpdatedAt = now
		if err = tx.UpdatePlan(plan, req.ExpectedVersion); err != nil {
			return err
		}
		out.Plan = plan
		details, err := json.Marshal(struct {
			Request         ChangeRequest `json:"request"`
			PreviousKey     string        `json:"previous_algorithm_key"`
			PreviousVersion int           `json:"previous_algorithm_version"`
			Result          ChangeResult  `json:"result"`
		}{req, previousKey, previousVersion, out})
		if err != nil {
			return err
		}
		if err = tx.SavePlanChange(model.PlanChange{ID: uuid.New(), UserID: user, PlanID: planID, CommandID: req.CommandID, Details: details, CreatedAt: now}); err != nil {
			return err
		}
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.SaveReceipt(model.CommandReceipt{UserID: user, CommandID: req.CommandID, RequestHash: hash, Response: response})
	})
	return out, err
}
func (s *Service) PlanChanges(ctx context.Context, user, planID uuid.UUID, limit, offset int) ([]model.PlanChange, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	var out []model.PlanChange
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = tx.PlanChanges(planID, limit, offset)
		return err
	})
	return out, err
}
