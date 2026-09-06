package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type ActionRequest struct {
	CommandID       uuid.UUID        `json:"command_id"`
	PresentationID  uuid.UUID        `json:"presentation_id"`
	ExpectedVersion int              `json:"expected_version"`
	Action          algorithm.Action `json:"action"`
}

func (s *Service) Act(ctx context.Context, user, sessionID uuid.UUID, req ActionRequest) (model.ActionResult, error) {
	var out model.ActionResult
	if req.CommandID == uuid.Nil || req.PresentationID == uuid.Nil || req.ExpectedVersion < 1 {
		return out, ErrInvalid
	}
	switch req.Action {
	case algorithm.Correct, algorithm.Wrong, algorithm.Advance, algorithm.Rollback, algorithm.SkipRehab:
	default:
		return out, ErrInvalid
	}
	b, _ := json.Marshal(struct {
		SessionID uuid.UUID
		Request   ActionRequest
	}{sessionID, req})
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		session, err := tx.Session(sessionID)
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
		p, err := tx.Plan(session.PlanID)
		if err != nil {
			return err
		}
		if session.Status != model.StatusActive || p.Status != model.StatusActive {
			return repository.ErrConflict
		}
		items, err := tx.Items(session.ID)
		if err != nil {
			return err
		}
		if len(items) == 0 || items[0].Presentation == nil {
			return repository.ErrConflict
		}
		i := items[0]
		shown := i.Presentation
		if shown.ID != req.PresentationID || shown.ProgressVersion != req.ExpectedVersion {
			return repository.ErrConflict
		}
		progress, err := tx.Progress().Get(ctx, keyFor(p, i.MaterialID))
		if err != nil {
			return err
		}
		now := s.now().UTC()
		kind, due := progress.DueReview(now)
		if progress.Version != req.ExpectedVersion || !due || kind != shown.Kind {
			return repository.ErrConflict
		}
		if req.Action == algorithm.SkipRehab && kind == model.ReviewStage {
			return ErrInvalid
		}
		a, err := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
		if err != nil {
			return err
		}
		next, err := a.Apply(algorithm.Input{Progress: progress, Difficulty: shown.Difficulty, Kind: kind, Action: req.Action, Now: now})
		if err != nil {
			return errors.Join(ErrInvalid, err)
		}
		next, err = tx.Progress().Update(ctx, next, progress.Version)
		if err != nil {
			return err
		}
		e := model.TrainingEvent{ID: uuid.New(), CommandID: req.CommandID, UserID: user, PlanID: p.ID, SessionID: session.ID, MaterialID: i.MaterialID,
			PresentationID: &shown.ID, Action: string(req.Action), Kind: kind, AlgorithmKey: a.Key(), AlgorithmVersion: a.Version(),
			StageBefore: progress.Stage, StageAfter: next.Stage, ProgressVersionBefore: progress.Version, ProgressVersionAfter: next.Version, CreatedAt: now}
		if err = tx.SaveEvent(e); err != nil {
			return err
		}
		i.Presentation = nil
		i.Position = items[len(items)-1].Position + 1
		if _, due := next.DueReview(now); !due {
			i.State = "completed"
		}
		if err = tx.SaveItem(i); err != nil {
			return err
		}
		view, err := s.sessionView(ctx, tx, p, session)
		if err != nil {
			return err
		}
		out = model.ActionResult{Event: e, NextReviewAt: next.NextReviewAt(), Session: view}
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.SaveReceipt(model.CommandReceipt{UserID: user, CommandID: req.CommandID, RequestHash: hash, Response: response})
	})
	return out, err
}
