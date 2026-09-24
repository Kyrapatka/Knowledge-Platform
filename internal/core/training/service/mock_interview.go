package service

import (
	"context"
	"errors"
	"fmt"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"strings"
)

var ErrMockActive = errors.New("An interview is already active. Resume it or end it before changing its settings.")

// Adapter for the existing graph/presentation interfaces, never a persisted
// TrainingPlan. A NULL session plan_id is the database-level mock boundary.
func mockContext(tx repository.Tx, user uuid.UUID, sources []model.SessionSource) (model.TrainingPlan, error) {
	p := mockPlan(user, nil)
	if len(sources) == 0 || len(sources) > 100 {
		return p, fmt.Errorf("%w: select between 1 and 100 interview folders", ErrInvalid)
	}
	seen := map[uuid.UUID]bool{}
	for _, source := range sources {
		if source.FolderID == uuid.Nil || seen[source.FolderID] || len(source.Topics) > 100 {
			return p, fmt.Errorf("%w: duplicate or invalid interview source", ErrInvalid)
		}
		seen[source.FolderID] = true
		f, err := tx.Folder(source.FolderID)
		if err != nil {
			return p, err
		}
		if f.TemplateKey != "interview_questions" {
			return p, fmt.Errorf("%w: only interview question folders can be selected", ErrInvalid)
		}
		topics := map[string]bool{}
		for _, topic := range source.Topics {
			if strings.TrimSpace(topic) == "" || len(topic) > 200 || topics[topic] {
				return p, ErrInvalid
			}
			topics[topic] = true
		}
		p.SourceFolderIDs = append(p.SourceFolderIDs, source.FolderID)
		p.Config.Cards[source.FolderID.String()] = f.Config
	}
	return p, nil
}
func sessionPlan(tx repository.Tx, s model.TrainingSession) (model.TrainingPlan, error) {
	if s.PlanID != uuid.Nil {
		return tx.Plan(s.PlanID)
	}
	if s.SelectionStrategy != model.SelectionInterviewGraphV1 || s.Combined {
		return model.TrainingPlan{}, repository.ErrConflict
	}
	// Sources were validated at creation. Removed folders must not prevent ending
	// or resuming a session; candidate/material queries still enforce ownership.
	return mockPlan(s.UserID, s.Selection), nil
}

func mockPlan(user uuid.UUID, sources []model.SessionSource) model.TrainingPlan {
	p := model.TrainingPlan{UserID: user, Status: model.StatusActive, AlgorithmKey: "interview_long_term", AlgorithmVersion: 1, Config: model.PlanConfig{Cards: map[string]folderconfig.FolderConfig{}}}
	for _, source := range sources {
		p.SourceFolderIDs = append(p.SourceFolderIDs, source.FolderID)
	}
	return p
}

func (s *Service) ActiveMock(ctx context.Context, user uuid.UUID) (model.SessionView, error) {
	var out model.SessionView
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		session, err := tx.ActiveSession(uuid.Nil)
		if err != nil {
			return err
		}
		p, err := sessionPlan(tx, session)
		if err != nil {
			return err
		}
		out, err = s.graphView(ctx, tx, p, session)
		return err
	})
	return out, err
}
func (s *Service) PreviewMock(ctx context.Context, user uuid.UUID, req StartGraphRequest) (graph.InterviewPlan, error) {
	var out graph.InterviewPlan
	config := graph.DefaultConfig()
	if req.Config != nil {
		config = *req.Config
	}
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		p, err := mockContext(tx, user, req.Sources)
		if err != nil {
			return err
		}
		session := model.TrainingSession{UserID: user, Selection: req.Sources}
		candidates, err := tx.InterviewGraph().RootCandidates(p, session, s.now().UTC())
		if err != nil {
			return err
		}
		state := graph.State{Config: config, RandomSeed: 1}
		for _, source := range req.Sources {
			state.Sources = append(state.Sources, graph.Source{FolderID: source.FolderID, Topics: source.Topics})
		}
		out, err = graph.BuildPlan(state, candidates)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalid, err)
		}
		return nil
	})
	return out, err
}
