package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

// Explicit opt-in moves just the nearest scheduled review into the current pool.
// Other timers (in particular the Stage timer during recovery) stay unchanged.
func (s *Service) reviewEarly(ctx context.Context, user uuid.UUID, req CombinedCurrentRequest) (model.CombinedView, error) {
	var out model.CombinedView
	if req.CommandID == uuid.Nil || len(req.SessionIDs) == 0 || len(req.SessionIDs) > 100 {
		return out, ErrInvalid
	}
	raw, _ := json.Marshal(req)
	sum := sha256.Sum256(append([]byte("review_early:"), raw...))
	hash := hex.EncodeToString(sum[:])
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
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
		now := s.now().UTC()
		sessions := make([]model.TrainingSession, 0, len(req.SessionIDs))
		seen := map[uuid.UUID]bool{}
		var chosen *model.TrainingPlan
		var chosenSession model.TrainingSession
		var nearest *time.Time
		for _, id := range req.SessionIDs {
			session, err := tx.Session(id)
			if err != nil {
				return err
			}
			if !session.Combined || session.Status == model.StatusCancelled || seen[session.PlanID] {
				return repository.ErrConflict
			}
			seen[session.PlanID] = true
			plan, err := tx.Plan(session.PlanID)
			if err != nil {
				return err
			}
			plan, err = currentCardConfig(tx, plan)
			if err != nil {
				return err
			}
			if plan.Status == model.StatusCancelled {
				return repository.ErrConflict
			}
			// A newer run must not be displaced by an old browser tab.
			latest, err := tx.LatestSession(plan.ID)
			if err != nil {
				return err
			}
			if latest.ID != session.ID {
				return repository.ErrConflict
			}
			sessions = append(sessions, session)
			if plan.Status != model.StatusActive {
				continue
			}
			available, err := tx.Availability(plan, session.Selection)
			if err != nil {
				return err
			}
			if next := available.NextReviewAt; next != nil && (nearest == nil || next.Before(*nearest)) {
				nearest, chosen, chosenSession = next, &plan, session
			}
		}
		if chosen == nil || nearest == nil || !nearest.After(now) || !nearest.Before(now.Add(3*time.Hour)) {
			return ErrInvalid
		}
		// Ask for exactly the earliest eligible date, not the entire three-hour window.
		materials, err := tx.Candidates(*chosen, chosenSession.ID, *nearest, 1)
		if err != nil {
			return err
		}
		if len(materials) != 1 {
			return repository.ErrConflict
		}
		before, err := tx.Progress().Get(ctx, keyFor(*chosen, materials[0].ID))
		if err != nil {
			return err
		}
		due := before.NextReviewAt()
		if before.CompletedAt != nil || due == nil || !due.Equal(*nearest) {
			return repository.ErrConflict
		}
		kind, ok := before.DueReview(*due)
		if !ok {
			return repository.ErrConflict
		}
		next := before
		switch kind {
		case model.ReviewStage:
			next.StageReviewAt = &now
		case model.ReviewRehab:
			next.RehabReviewAt = &now
		case model.ReviewExtra:
			next.ExtraReviewAt = &now
		default:
			return ErrInvalid
		}
		next.UpdatedAt = now
		next, err = tx.Progress().Update(ctx, next, before.Version)
		if err != nil {
			return err
		}
		if err = tx.SaveEvent(model.TrainingEvent{ID: uuid.New(), CommandID: req.CommandID, UserID: user, PlanID: chosen.ID, SessionID: chosenSession.ID, MaterialID: materials[0].ID, Action: "review_early", Kind: kind, AlgorithmKey: chosen.AlgorithmKey, AlgorithmVersion: chosen.AlgorithmVersion, StageBefore: before.Stage, StageAfter: next.Stage, ProgressVersionBefore: before.Version, ProgressVersionAfter: next.Version, CreatedAt: now}); err != nil {
			return err
		}
		for i, session := range sessions {
			if session.ID == chosenSession.ID && session.Status == model.StatusCompleted {
				sessions[i], err = s.scopedSession(ctx, tx, *chosen, session.Selection, false)
				if err != nil {
					return err
				}
			}
		}
		out, err = s.combinedView(ctx, tx, sessions)
		if err != nil {
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
