package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type GraphUndoRequest struct {
	CommandID uuid.UUID `json:"command_id"`
	EventID   uuid.UUID `json:"event_id"`
}

func (s *Service) UndoGraph(ctx context.Context, user, sessionID uuid.UUID, req GraphUndoRequest) (model.SessionView, error) {
	var out model.SessionView
	if req.CommandID == uuid.Nil || req.EventID == uuid.Nil {
		return out, ErrInvalid
	}
	hash := graphHash("graph-undo:", struct {
		SessionID uuid.UUID
		Request   GraphUndoRequest
	}{sessionID, req})
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
		if session.SelectionStrategy != model.SelectionInterviewGraphV1 || session.Combined || session.Status == model.StatusCancelled {
			return repository.ErrConflict
		}
		history, err := tx.UndoHistory()
		if err != nil {
			return err
		}
		if len(history) == 0 {
			return repository.ErrConflict
		}
		snapshot := history[0]
		if snapshot.EventID != req.EventID || snapshot.SessionID != sessionID || snapshot.GraphStateBefore == nil {
			return repository.ErrConflict
		}
		p, err := tx.Plan(session.PlanID)
		if err != nil {
			return err
		}
		latest, err := tx.LatestSession(p.ID)
		if err != nil {
			return err
		}
		if p.Status != model.StatusActive || p.Version != snapshot.PlanVersion || latest.ID != sessionID {
			return repository.ErrConflict
		}
		if _, err = tx.Material(snapshot.MaterialID); err != nil {
			return err
		}
		progress, err := tx.Progress().Get(ctx, keyFor(p, snapshot.MaterialID))
		if err != nil {
			return err
		}
		if progress.Version != snapshot.ExpectedVersion {
			return repository.ErrConflict
		}
		now := s.now().UTC()
		version := progress.Version
		if !snapshot.GraphProbe {
			before := snapshot.Before
			before.Version = version
			before.UpdatedAt = now
			restored, err := tx.Progress().Update(ctx, before, version)
			if err != nil {
				return err
			}
			version = restored.Version
		}
		gr := tx.InterviewGraph()
		currentState, err := gr.State(sessionID)
		if err != nil {
			return err
		}
		if _, err = gr.SaveState(sessionID, *snapshot.GraphStateBefore, currentState.Version, now); err != nil {
			return err
		}
		if snapshot.GraphSelectionEventID != nil {
			if err = gr.UndoSelection(sessionID, *snapshot.GraphSelectionEventID, now); err != nil {
				return err
			}
		}
		if err = tx.RestoreUndo(snapshot, version, now); err != nil {
			return err
		}
		session, err = tx.Session(sessionID)
		if err != nil {
			return err
		}
		out, err = s.graphView(ctx, tx, p, session)
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
