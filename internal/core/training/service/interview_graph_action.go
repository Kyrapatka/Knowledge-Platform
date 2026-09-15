package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

// Called inside Act's existing transaction and after its command receipt check.
func (s *Service) graphAction(ctx context.Context, tx repository.Tx, p model.TrainingPlan, session model.TrainingSession, req ActionRequest, hash string) (model.ActionResult, error) {
	var out model.ActionResult
	if req.Action != algorithm.Correct && req.Action != algorithm.Wrong && req.Action != "next_route" {
		return out, ErrInvalid
	}
	if session.Combined || session.Status != model.StatusActive || p.Status != model.StatusActive {
		return out, repository.ErrConflict
	}
	items, err := tx.Items(session.ID)
	if err != nil {
		return out, err
	}
	if len(items) != 1 || items[0].Presentation == nil {
		return out, repository.ErrConflict
	}
	item := items[0]
	shown := item.Presentation
	if shown.InterviewGraph == nil || shown.ID != req.PresentationID || shown.ProgressVersion != req.ExpectedVersion {
		return out, repository.ErrConflict
	}
	if _, err = tx.Material(item.MaterialID); err != nil {
		return out, repository.ErrConflict
	}
	gr := tx.InterviewGraph()
	state, err := gr.State(session.ID)
	if err != nil {
		return out, err
	}
	selection, err := gr.Selection(session.ID, shown.InterviewGraph.SelectionEventID)
	if err != nil {
		return out, err
	}
	progress, err := tx.Progress().Get(ctx, keyFor(p, item.MaterialID))
	if errors.Is(err, repository.ErrNotFound) && req.ExpectedVersion == 0 && !shown.InterviewGraph.ReviewCredit {
		progress = model.UserMaterialProgress{MaterialID: item.MaterialID, Stage: shown.Stage}
		err = nil
	}
	if err != nil {
		return out, err
	}
	if progress.Version != req.ExpectedVersion {
		return out, repository.ErrConflict
	}
	now := s.now().UTC()
	next := progress
	credit := shown.InterviewGraph.ReviewCredit && !state.Config.IncludeDraft && req.Action != "next_route"
	if credit {
		kind, due := progress.DueReview(now)
		if !due || kind != shown.Kind {
			return out, repository.ErrConflict
		}
		a, err := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
		if err != nil {
			return out, err
		}
		next, err = a.Apply(algorithm.Input{Progress: progress, Difficulty: shown.Difficulty, Kind: shown.Kind, Action: req.Action, Now: now})
		if err != nil {
			return out, err
		}
		next, err = tx.Progress().Update(ctx, next, progress.Version)
		if err != nil {
			return out, err
		}
	}
	mode := "scheduled"
	if !credit {
		mode = "graph_probe"
	}
	if req.Action == "next_route" {
		mode = "graph_navigation"
	}
	event := model.TrainingEvent{ID: uuid.New(), CommandID: req.CommandID, UserID: p.UserID, PlanID: p.ID, SessionID: session.ID, MaterialID: item.MaterialID, PresentationID: &shown.ID, Action: string(req.Action), Kind: shown.Kind, AlgorithmKey: p.AlgorithmKey, AlgorithmVersion: p.AlgorithmVersion, StageBefore: progress.Stage, StageAfter: next.Stage, ProgressVersionBefore: progress.Version, ProgressVersionAfter: next.Version, ReviewCredit: credit, EventMode: mode, CreatedAt: now}
	if err = tx.SaveEvent(event); err != nil {
		return out, err
	}
	snapshot := model.UndoSnapshot{EventID: event.ID, PlanID: p.ID, SessionID: session.ID, MaterialID: item.MaterialID, ExpectedVersion: next.Version, PlanVersion: p.Version, Before: progress, Items: items, GraphStateBefore: &state, GraphProbe: !credit}
	candidates, err := gr.FollowUpCandidates(p, session, now)
	if err != nil {
		return out, err
	}
	catalog, err := gr.Catalog()
	if err != nil {
		return out, err
	}
	selected := graph.SelectMetadata(state, selection.Snapshot, string(req.Action), candidates, catalog)
	item.State = "completed"
	item.Presentation = nil
	if err = tx.SaveItem(item); err != nil {
		return out, err
	}
	snapshot.GraphSelectionEventID, err = s.persistGraphSelection(ctx, tx, p, &session, state, selected, &item.MaterialID, "")
	if err != nil {
		return out, err
	}
	if err = tx.SaveUndo(snapshot); err != nil {
		return out, err
	}
	view, err := s.graphView(ctx, tx, p, session)
	if err != nil {
		return out, err
	}
	out = model.ActionResult{Event: event, NextReviewAt: next.NextReviewAt(), Session: view}
	response, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	err = tx.SaveReceipt(model.CommandReceipt{UserID: p.UserID, CommandID: req.CommandID, RequestHash: hash, Response: response})
	return out, err
}
