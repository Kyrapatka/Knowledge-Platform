package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"reflect"
	"sort"
	"strings"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

type CombinedSource struct {
	FolderID     uuid.UUID  `json:"folder_id"`
	Topics       []string   `json:"topics,omitempty"`
	PlanID       *uuid.UUID `json:"plan_id,omitempty"`
	AlgorithmKey *string    `json:"algorithm_key,omitempty"`
	HorizonDays  *int       `json:"horizon_days,omitempty"`
	PoolSize     *int       `json:"pool_size,omitempty"`
}

type CombinedRequest struct {
	SelectionStrategy model.SelectionStrategy `json:"selection_strategy,omitempty"`
	Sources           []CombinedSource        `json:"sources"`
}
type CombinedCurrentRequest struct {
	SessionIDs  []uuid.UUID `json:"session_ids"`
	ReviewEarly bool        `json:"review_early,omitempty"`
	CommandID   uuid.UUID   `json:"command_id,omitempty"`
}

func selectedMaterial(m material.Material, sources []model.SessionSource) bool {
	if len(sources) == 0 {
		return true
	}
	topic := ""
	for _, key := range []string{"topic", "category"} {
		if value := m.Metadata[key]; value != nil && strings.TrimSpace(*value) != "" {
			topic = strings.TrimSpace(*value)
			break
		}
	}
	if topic == "" {
		topic = "__none__"
	}
	for _, source := range sources {
		if source.FolderID != m.FolderID {
			continue
		}
		if len(source.Topics) == 0 {
			return true
		}
		for _, allowed := range source.Topics {
			if allowed == topic {
				return true
			}
		}
	}
	return false
}

func reviewRank(kind model.ReviewKind) int {
	switch kind {
	case model.ReviewStage:
		return 0
	case model.ReviewRehab:
		return 1
	default:
		return 2
	}
}

func containsFolder(plan model.TrainingPlan, id uuid.UUID) bool {
	for _, source := range plan.SourceFolderIDs {
		if source == id {
			return true
		}
	}
	return false
}

func (s *Service) StartCombined(ctx context.Context, user uuid.UUID, req CombinedRequest) (model.CombinedView, error) {
	if req.SelectionStrategy != "" && req.SelectionStrategy != model.SelectionRandom {
		return model.CombinedView{}, fmt.Errorf("%w: graph selection is not supported in combined training", ErrInvalid)
	}
	var result model.CombinedView
	if len(req.Sources) == 0 || len(req.Sources) > 100 {
		return result, ErrInvalid
	}
	seen := map[uuid.UUID]bool{}
	for i := range req.Sources {
		source := &req.Sources[i]
		if source.FolderID == uuid.Nil || seen[source.FolderID] || len(source.Topics) > 100 {
			return result, ErrInvalid
		}
		seen[source.FolderID] = true
		topics := map[string]bool{}
		for _, topic := range source.Topics {
			topic = strings.TrimSpace(topic)
			if topic == "" || len(topic) > 200 {
				return result, ErrInvalid
			}
			topics[topic] = true
		}
		source.Topics = nil
		for topic := range topics {
			source.Topics = append(source.Topics, topic)
		}
		sort.Strings(source.Topics)
	}
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var plans []model.TrainingPlan
		for offset := 0; ; offset += 100 {
			page, err := tx.Plans(100, offset)
			if err != nil {
				return err
			}
			plans = append(plans, page...)
			if len(page) < 100 {
				break
			}
		}
		// Resolve every source in this transaction, so invalid later sources cannot
		// leave behind a partial set of plans or admitted materials.
		resolved := map[uuid.UUID]model.TrainingPlan{}
		selections := map[uuid.UUID][]model.SessionSource{}
		var order []uuid.UUID
		for _, source := range req.Sources {
			folder, err := tx.Folder(source.FolderID)
			if err != nil {
				return err
			}
			var plan model.TrainingPlan
			if source.PlanID != nil {
				plan, err = tx.Plan(*source.PlanID)
				if err != nil {
					return err
				}
				if !containsFolder(plan, source.FolderID) || plan.Status == model.StatusCancelled {
					return ErrInvalid
				}
			} else {
				// The newest matching active plan is the folder's current context.
				// Completed CRAM is retained; quick start must not silently reset it.
				for _, candidate := range plans {
					if candidate.Status != model.StatusActive || !containsFolder(candidate, source.FolderID) {
						continue
					}
					if source.AlgorithmKey != nil && candidate.AlgorithmKey != *source.AlgorithmKey {
						continue
					}
					plan = candidate
					break
				}
				if plan.ID == uuid.Nil {
					for _, candidate := range plans {
						if candidate.Status != model.StatusCompleted || candidate.Track != model.ProgressTrackCram || !containsFolder(candidate, source.FolderID) {
							continue
						}
						if source.AlgorithmKey != nil && candidate.AlgorithmKey != *source.AlgorithmKey {
							continue
						}
						plan = candidate
						break
					}
				}
				if plan.ID == uuid.Nil {
					defaults := folder.TrainingConfig
					if defaults.DefaultAlgorithmKey == "" {
						defaults = folderconfig.DefaultTrainingConfig(folder.TemplateKey)
					}
					key := defaults.DefaultAlgorithmKey
					if source.AlgorithmKey != nil {
						key = *source.AlgorithmKey
					}
					horizon := 0
					if key == "interview_long_term" {
						horizon = 150
					}
					if key == "interview_cram" {
						horizon = 5
					}
					if source.HorizonDays != nil {
						horizon = *source.HorizonDays
					}
					plan, err = s.createPlan(tx, user, CreatePlanRequest{SourceFolderIDs: []uuid.UUID{source.FolderID}, AlgorithmKey: &key, PoolSize: source.PoolSize, HorizonDays: horizon})
					if err != nil {
						return err
					}
					plans = append([]model.TrainingPlan{plan}, plans...)
				}
			}
			if source.AlgorithmKey != nil && plan.AlgorithmKey != *source.AlgorithmKey || source.HorizonDays != nil && plan.Config.HorizonDays != *source.HorizonDays || source.PoolSize != nil && plan.Config.PoolSize != *source.PoolSize {
				return fmt.Errorf("%w: change the existing plan settings explicitly before starting", ErrInvalid)
			}
			if _, ok := resolved[plan.ID]; !ok {
				order = append(order, plan.ID)
			}
			resolved[plan.ID] = plan
			selections[plan.ID] = append(selections[plan.ID], model.SessionSource{FolderID: source.FolderID, Topics: source.Topics})
		}
		var sessions []model.TrainingSession
		for _, id := range order {
			selection := selections[id]
			sort.Slice(selection, func(i, j int) bool { return selection[i].FolderID.String() < selection[j].FolderID.String() })
			session, err := s.scopedSession(ctx, tx, resolved[id], selection, true)
			if err != nil {
				return err
			}
			sessions = append(sessions, session)
		}
		var err error
		result, err = s.combinedView(ctx, tx, sessions)
		return err
	})
	return result, err
}

// scopedSession preserves a matching active run. A changed selection replaces
// only its internal run; persistent material progress and dates remain untouched.
func (s *Service) scopedSession(ctx context.Context, tx repository.Tx, plan model.TrainingPlan, selection []model.SessionSource, replace bool) (model.TrainingSession, error) {
	var configErr error
	plan, configErr = currentCardConfig(tx, plan)
	if configErr != nil {
		return model.TrainingSession{}, configErr
	}
	active, err := tx.ActiveSession(plan.ID)
	if err == nil {
		if active.SelectionStrategy == model.SelectionInterviewGraphV1 {
			return active, fmt.Errorf("%w: end the active mock interview before using combined training", repository.ErrConflict)
		}
		if active.Combined && reflect.DeepEqual(active.Selection, selection) {
			return active, nil
		}
		if !replace {
			return active, repository.ErrConflict
		}
		if err = tx.FinishSession(active.ID, model.StatusCancelled, s.now().UTC()); err != nil {
			return active, err
		}
		sessionEvent(tx, analytics.TrainingAbandoned, plan, active, s.now().UTC())
	} else if !errors.Is(err, repository.ErrNotFound) {
		return active, err
	}
	previous, err := tx.LatestSession(plan.ID)
	if err == nil && previous.Combined && previous.Status == model.StatusCompleted && reflect.DeepEqual(previous.Selection, selection) {
		if plan.Status != model.StatusActive {
			return previous, nil
		}
		available, err := tx.Candidates(plan, previous.ID, s.now().UTC(), 1)
		if err != nil {
			return previous, err
		}
		if len(available) == 0 {
			return previous, nil
		}
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return previous, err
	}
	now := s.now().UTC()
	session := model.TrainingSession{ID: uuid.New(), PlanID: plan.ID, UserID: plan.UserID, Combined: true, Selection: selection, Status: model.StatusActive, StartedAt: now, CreatedAt: now}
	if plan.Status != model.StatusActive {
		session.Status = model.StatusCompleted
		session.FinishedAt = &now
	}
	if err := tx.CreateSession(session); err != nil {
		return session, err
	}
	sessionEvent(tx, analytics.TrainingStarted, plan, session, now)
	if session.Status == model.StatusCompleted {
		sessionEvent(tx, analytics.TrainingCompleted, plan, session, now)
	}
	// Return the persisted representation (including PostgreSQL time precision
	// and timezone), just as a repeated launch does.
	return tx.Session(session.ID)
}

func (s *Service) CurrentCombined(ctx context.Context, user uuid.UUID, req CombinedCurrentRequest) (model.CombinedView, error) {
	if req.ReviewEarly {
		return s.reviewEarly(ctx, user, req)
	}
	var result model.CombinedView
	if len(req.SessionIDs) == 0 || len(req.SessionIDs) > 100 {
		return result, ErrInvalid
	}
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var sessions []model.TrainingSession
		seen := map[uuid.UUID]bool{}
		for _, id := range req.SessionIDs {
			session, err := tx.Session(id)
			if err != nil {
				return err
			}
			if !session.Combined || seen[session.PlanID] {
				return ErrInvalid
			}
			seen[session.PlanID] = true
			if session.Status == model.StatusCancelled {
				return repository.ErrConflict
			}
			if session.Status == model.StatusCompleted {
				plan, err := tx.Plan(session.PlanID)
				if err != nil {
					return err
				}
				if plan.Status == model.StatusCancelled {
					return repository.ErrConflict
				}
				session, err = s.scopedSession(ctx, tx, plan, session.Selection, false)
				if err != nil {
					return err
				}
			}
			sessions = append(sessions, session)
		}
		var err error
		result, err = s.combinedView(ctx, tx, sessions)
		return err
	})
	return result, err
}

func (s *Service) combinedView(ctx context.Context, tx repository.Tx, sessions []model.TrainingSession) (model.CombinedView, error) {
	result := model.CombinedView{Sessions: make([]model.CombinedComponent, 0, len(sessions))}
	type candidate struct {
		component int
		rank      int
		attempts  int
		shown     bool
	}
	var candidates []candidate
	var ids []uuid.UUID
	var total, pending int64
	for _, session := range sessions {
		plan, err := tx.Plan(session.PlanID)
		if err != nil {
			return result, err
		}
		plan, err = currentCardConfig(tx, plan)
		if err != nil {
			return result, err
		}
		var items []model.SessionItem
		if session.Status == model.StatusActive && plan.Status == model.StatusActive {
			items, err = s.preparePool(ctx, tx, plan, session, false)
			if err != nil {
				return result, err
			}
			if len(items) == 0 {
				complete, err := tx.CompletePlanIfReady(plan, s.now().UTC())
				if err != nil {
					return result, err
				}
				if !complete {
					if err = tx.FinishSession(session.ID, model.StatusCompleted, s.now().UTC()); err != nil {
						return result, err
					}
				}
				session, err = tx.Session(session.ID)
				if err != nil {
					return result, err
				}
				if session.Status == model.StatusCompleted {
					sessionEvent(tx, analytics.TrainingCompleted, plan, session, s.now().UTC())
				}
				plan, err = tx.Plan(plan.ID)
				if err != nil {
					return result, err
				}
			}
		}
		summary, err := tx.Summary(session.ID)
		if err != nil {
			return result, err
		}
		if len(items) > 0 {
			progress, err := tx.Progress().Get(ctx, keyFor(plan, items[0].MaterialID))
			if err != nil {
				return result, err
			}
			kind, _ := progress.DueReview(s.now().UTC())
			candidates = append(candidates, candidate{len(result.Sessions), reviewRank(kind), summary.Correct + summary.Wrong + summary.Advance + summary.Rollback + summary.SkipRehab, items[0].Presentation != nil})
		}
		availability, err := tx.Availability(plan, session.Selection)
		if err != nil {
			return result, err
		}
		total += availability.Total
		pending += availability.Pending
		if next := availability.NextReviewAt; next != nil && (result.NextReviewAt == nil || next.Before(*result.NextReviewAt)) {
			result.NextReviewAt = next
		}
		result.Sessions = append(result.Sessions, model.CombinedComponent{Session: session, Plan: plan, Summary: summary, PoolSize: len(items)})
		ids = append(ids, session.ID)
	}
	var err error
	result.Summary, err = tx.CombinedSummary(ids)
	if err != nil {
		return result, err
	}
	result.UndoActions, err = availableUndos(ctx, tx, sessions)
	if err != nil {
		return result, err
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		if candidates[i].shown != candidates[j].shown {
			return candidates[i].shown
		}
		return candidates[i].attempts < candidates[j].attempts
	})
	if len(candidates) > 0 {
		component := result.Sessions[candidates[0].component]
		items, err := s.preparePool(ctx, tx, component.Plan, component.Session, true)
		if err != nil {
			return result, err
		}
		if len(items) > 0 {
			result.Current = &model.CombinedCurrent{SessionID: component.Session.ID, PlanID: component.Plan.ID, AlgorithmKey: component.Plan.AlgorithmKey, Presentation: items[0].Presentation}
		}
	}
	if result.Current == nil {
		switch {
		case total == 0:
			result.EmptyReason = "no_matching_materials"
		case pending == 0:
			result.EmptyReason = "completed"
		default:
			result.EmptyReason = "not_due"
		}
	}
	return result, nil
}
