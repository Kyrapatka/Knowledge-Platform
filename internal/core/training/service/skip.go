package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type SkipRequest struct {
	CommandID       uuid.UUID `json:"command_id"`
	ExpectedVersion int       `json:"expected_version"`
}

// SkipRecovery also works between rehab days or before the extra review is due.
// It is a manual progress command, so it has no answered presentation.
func (s *Service) SkipRecovery(ctx context.Context, user, sessionID, materialID uuid.UUID, req SkipRequest) (model.ActionResult, error) {
	var out model.ActionResult
	if req.CommandID == uuid.Nil || req.ExpectedVersion < 1 {
		return out, ErrInvalid
	}
	b, _ := json.Marshal(struct {
		Operation             string
		SessionID, MaterialID uuid.UUID
		Request               SkipRequest
	}{"skip_recovery", sessionID, materialID, req})
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	err := s.transact(ctx, user, func(tx repository.Tx) error {
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
		m, err := tx.Material(materialID)
		if err != nil {
			return err
		}
		if _, ok := p.Config.Cards[m.FolderID.String()]; !ok {
			return repository.ErrNotFound
		}
		before, err := tx.Progress().Get(ctx, keyFor(p, materialID))
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if before.Version != req.ExpectedVersion || (!before.RehabActive && before.ExtraReviewAt == nil) {
			return repository.ErrConflict
		}
		if kind, due := before.DueReview(now); due && kind == model.ReviewStage {
			return repository.ErrConflict
		}
		kind := model.ReviewExtra
		if before.RehabActive {
			kind = model.ReviewRehab
		}
		next := before
		next.CancelRecovery()
		next.UpdatedAt = now
		next, err = tx.Progress().Update(ctx, next, before.Version)
		if err != nil {
			return err
		}
		e := model.TrainingEvent{ID: uuid.New(), CommandID: req.CommandID, UserID: user, PlanID: p.ID, SessionID: sessionID, MaterialID: materialID,
			Action: "skip_rehab", Kind: kind, AlgorithmKey: p.AlgorithmKey, AlgorithmVersion: p.AlgorithmVersion, StageBefore: before.Stage, StageAfter: next.Stage,
			ProgressVersionBefore: before.Version, ProgressVersionAfter: next.Version, CreatedAt: now}
		if err = tx.SaveEvent(e); err != nil {
			return err
		}
		view, err := s.sessionView(ctx, tx, p, session)
		if err != nil {
			return err
		}
		out = model.ActionResult{Event: e, NextReviewAt: next.NextReviewAt(), Session: view}
		queueAnswer(tx, answerEvent(e, p, session, m, nil, &before, &next), false)
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.SaveReceipt(model.CommandReceipt{UserID: user, CommandID: req.CommandID, RequestHash: hash, Response: response})
	})
	return out, err
}
