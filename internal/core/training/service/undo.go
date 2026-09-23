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

type UndoRequest struct {
	CommandID  uuid.UUID   `json:"command_id"`
	EventID    uuid.UUID   `json:"event_id"`
	SessionIDs []uuid.UUID `json:"session_ids"`
}

func (s *Service) Undo(ctx context.Context, user uuid.UUID, req UndoRequest) (model.CombinedView, error) {
	var out model.CombinedView
	if req.CommandID == uuid.Nil || req.EventID == uuid.Nil || len(req.SessionIDs) == 0 || len(req.SessionIDs) > 100 {
		return out, ErrInvalid
	}
	raw, _ := json.Marshal(req)
	sum := sha256.Sum256(append([]byte("undo:"), raw...))
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
		history, err := tx.UndoHistory()
		if err != nil {
			return err
		}
		if len(history) == 0 || history[0].EventID != req.EventID {
			return repository.ErrConflict
		}
		snapshot := history[0]
		sessions := make([]model.TrainingSession, 0, len(req.SessionIDs))
		seen := map[uuid.UUID]bool{}
		found := false
		for _, id := range req.SessionIDs {
			session, err := tx.Session(id)
			if err != nil {
				return err
			}
			if !session.Combined || session.Status == model.StatusCancelled || seen[session.PlanID] {
				return repository.ErrConflict
			}
			seen[session.PlanID] = true
			if id == snapshot.SessionID {
				found = true
			}
			sessions = append(sessions, session)
		}
		if !found {
			return repository.ErrConflict
		}
		plan, err := tx.Plan(snapshot.PlanID)
		if err != nil {
			return err
		}
		latest, err := tx.LatestSession(plan.ID)
		if err != nil {
			return err
		}
		if plan.Status == model.StatusCancelled || plan.Version != snapshot.PlanVersion || latest.ID != snapshot.SessionID {
			return repository.ErrConflict
		}
		if _, err = tx.Material(snapshot.MaterialID); err != nil {
			return err
		}
		current, err := tx.Progress().Get(ctx, keyFor(plan, snapshot.MaterialID))
		if err != nil {
			return err
		}
		if current.Version != snapshot.ExpectedVersion {
			return repository.ErrConflict
		}
		before := snapshot.Before
		before.Version = current.Version
		before.UpdatedAt = s.now().UTC()
		restored, err := tx.Progress().Update(ctx, before, current.Version)
		if err != nil {
			return err
		}
		if err = tx.RestoreUndo(snapshot, restored.Version, s.now().UTC()); err != nil {
			return err
		}
		for i, session := range sessions {
			sessions[i], err = tx.Session(session.ID)
			if err != nil {
				return err
			}
			if session.ID != snapshot.SessionID {
				items, err := tx.Items(session.ID)
				if err != nil {
					return err
				}
				for _, item := range items {
					if item.Presentation != nil {
						item.Presentation = nil
						if err = tx.SaveItem(item); err != nil {
							return err
						}
					}
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

func availableUndos(ctx context.Context, tx repository.Tx, sessions []model.TrainingSession) ([]uuid.UUID, error) {
	result := []uuid.UUID{}
	history, err := tx.UndoHistory()
	if err != nil {
		return nil, err
	}
	allowed := map[uuid.UUID]bool{}
	for _, s := range sessions {
		allowed[s.ID] = true
	}
	for i, entry := range history {
		if _, err := tx.Material(entry.MaterialID); errors.Is(err, repository.ErrNotFound) {
			break
		} else if err != nil {
			return nil, err
		}
		if !allowed[entry.SessionID] {
			break
		}
		if entry.PlanID == uuid.Nil && entry.GraphStateBefore != nil && entry.GraphStateBefore.PracticeOnly {
			latest, err := tx.LatestSession(uuid.Nil)
			if err != nil {
				return nil, err
			}
			if latest.ID != entry.SessionID || latest.Status == model.StatusCancelled {
				break
			}
			result = append(result, entry.EventID)
			continue
		}
		p, err := tx.Plan(entry.PlanID)
		if err != nil {
			return nil, err
		}
		if p.Status == model.StatusCancelled || p.Version != entry.PlanVersion {
			break
		}
		latest, err := tx.LatestSession(p.ID)
		if err != nil {
			return nil, err
		}
		if latest.ID != entry.SessionID {
			break
		}
		if i == 0 {
			progress, err := tx.Progress().Get(ctx, keyFor(p, entry.MaterialID))
			if errors.Is(err, repository.ErrNotFound) && entry.GraphProbe && entry.ExpectedVersion == 0 {
				err = nil
			}
			if err != nil {
				return nil, err
			}
			if progress.Version != entry.ExpectedVersion {
				break
			}
		}
		result = append(result, entry.EventID)
	}
	return result, nil
}
