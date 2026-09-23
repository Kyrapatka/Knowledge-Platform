package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

var ErrInvalid = errors.New("invalid training request")

type Service struct {
	store    repository.RuntimeStore
	registry *algorithm.Registry
	now      func() time.Time
}

func NewService(store repository.RuntimeStore) *Service { return NewServiceWithClock(store, time.Now) }
func NewServiceWithClock(store repository.RuntimeStore, clock func() time.Time) *Service {
	r, err := algorithm.NewRegistry(algorithm.English{}, algorithm.English{Adaptive: true}, algorithm.Interview{Cram: true}, algorithm.Interview{}, algorithm.Formula{})
	if err != nil {
		panic(err)
	}
	return &Service{store, r, clock}
}

type CreatePlanRequest struct {
	HorizonDays     int         `json:"horizon_days"`
	SourceFolderIDs []uuid.UUID `json:"source_folder_ids"`
	AlgorithmKey    *string     `json:"algorithm_key"`
	PoolSize        *int        `json:"pool_size"`
}

type FolderDefaults struct {
	Config  folderconfig.TrainingConfig `json:"training_config"`
	Version int64                       `json:"version"`
}

func (s *Service) FolderDefaults(ctx context.Context, user, id uuid.UUID) (FolderDefaults, error) {
	var out FolderDefaults
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		f, err := tx.Folder(id)
		if err != nil {
			return err
		}
		out = FolderDefaults{f.TrainingConfig, f.TrainingConfigVersion}
		return nil
	})
	return out, err
}

func (s *Service) UpdateDefaults(ctx context.Context, user, id uuid.UUID, c folderconfig.TrainingConfig, version int64) (FolderDefaults, error) {
	if version < 1 || c.PoolSize < 1 || c.PoolSize > 50 {
		return FolderDefaults{}, ErrInvalid
	}
	if _, err := s.registry.Get(c.DefaultAlgorithmKey, 1); err != nil {
		return FolderDefaults{}, fmt.Errorf("%w: unsupported algorithm", ErrInvalid)
	}
	out := FolderDefaults{c, version + 1}
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		if _, err := tx.Folder(id); err != nil {
			return err
		}
		return tx.SaveDefaults(id, c, version, s.now().UTC())
	})
	return out, err
}

func (s *Service) CreatePlan(ctx context.Context, user uuid.UUID, req CreatePlanRequest) (model.TrainingPlan, error) {
	var out model.TrainingPlan
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = s.createPlan(tx, user, req)
		return err
	})
	return out, err
}

func (s *Service) createPlan(tx repository.Tx, user uuid.UUID, req CreatePlanRequest) (model.TrainingPlan, error) {
	var out model.TrainingPlan
	if len(req.SourceFolderIDs) == 0 || len(req.SourceFolderIDs) > 100 {
		return out, ErrInvalid
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range req.SourceFolderIDs {
		if id == uuid.Nil || seen[id] {
			return out, ErrInvalid
		}
		seen[id] = true
	}
	err := func() error {
		cards := make(map[string]folderconfig.FolderConfig, len(seen))
		var key string
		var pool int
		for index, id := range req.SourceFolderIDs {
			f, err := tx.Folder(id)
			if err != nil {
				return err
			}
			if err = validateCard(f.Config); err != nil {
				return err
			}
			cards[id.String()] = f.Config
			defaults := f.TrainingConfig
			if defaults.DefaultAlgorithmKey == "" {
				defaults = folderconfig.DefaultTrainingConfig(f.TemplateKey)
			}
			if index == 0 {
				key = defaults.DefaultAlgorithmKey
				pool = defaults.PoolSize
			}
			if req.AlgorithmKey == nil && key != defaults.DefaultAlgorithmKey {
				return fmt.Errorf("%w: choose algorithm explicitly for conflicting folder defaults", ErrInvalid)
			}
			if req.PoolSize == nil && pool != defaults.PoolSize {
				return fmt.Errorf("%w: choose pool_size explicitly for conflicting folder defaults", ErrInvalid)
			}
		}
		if req.AlgorithmKey != nil {
			key = *req.AlgorithmKey
		}
		if req.PoolSize != nil {
			pool = *req.PoolSize
		}
		if pool < 1 || pool > 50 {
			return fmt.Errorf("%w: pool_size must be 1..50", ErrInvalid)
		}
		if _, err := s.registry.Get(key, 1); err != nil {
			return fmt.Errorf("%w: unsupported algorithm", ErrInvalid)
		}
		track := model.ProgressTrackDefault
		switch key {
		case "interview_cram":
			track = model.ProgressTrackCram
			if req.HorizonDays < 1 || req.HorizonDays > 7 {
				return ErrInvalid
			}
		case "interview_long_term":
			track = model.ProgressTrackLongTerm
			if req.HorizonDays < 7 || req.HorizonDays > 365 {
				return ErrInvalid
			}
		default:
			if req.HorizonDays != 0 {
				return ErrInvalid
			}
		}
		overlap, err := tx.HasPlanOverlap(req.SourceFolderIDs, track)
		if err != nil {
			return err
		}
		if overlap && track != model.ProgressTrackCram {
			return repository.ErrConflict
		}
		incompatible, err := tx.HasIncompatibleProgress(req.SourceFolderIDs, track, key, 1)
		if err != nil {
			return err
		}
		if incompatible {
			return fmt.Errorf("%w: existing progress uses another algorithm", repository.ErrConflict)
		}
		now := s.now().UTC()
		out = model.TrainingPlan{Version: 1, ID: uuid.New(), UserID: user, Track: track, AlgorithmKey: key, AlgorithmVersion: 1, Status: model.StatusActive,
			Config: model.PlanConfig{PoolSize: pool, HorizonDays: req.HorizonDays, Cards: cards}, SourceFolderIDs: append([]uuid.UUID(nil), req.SourceFolderIDs...), StartedAt: now, CreatedAt: now, UpdatedAt: now}
		return tx.CreatePlan(out)
	}()
	return out, err
}

func validateCard(c folderconfig.FolderConfig) error {
	active := map[string]bool{}
	for _, f := range c.Schema.Fields {
		active[f.Key] = f.Active
	}
	for _, side := range [][]string{c.Card.QuestionFields, c.Card.AnswerFields} {
		if len(side) == 0 {
			return fmt.Errorf("%w: card question and answer must have fields", ErrInvalid)
		}
		seen := map[string]bool{}
		for _, key := range side {
			if !active[key] || seen[key] {
				return fmt.Errorf("%w: card references inactive, missing or duplicate fields", ErrInvalid)
			}
			seen[key] = true
		}
	}
	return nil
}

func (s *Service) GetPlan(ctx context.Context, user, id uuid.UUID) (model.TrainingPlan, error) {
	var out model.TrainingPlan
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error { var err error; out, err = tx.Plan(id); return err })
	return out, err
}
func (s *Service) ListPlans(ctx context.Context, user uuid.UUID, limit, offset int) ([]model.TrainingPlan, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	var out []model.TrainingPlan
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error { var err error; out, err = tx.Plans(limit, offset); return err })
	return out, err
}
func (s *Service) CancelPlan(ctx context.Context, user, id uuid.UUID) (model.TrainingPlan, error) {
	var out model.TrainingPlan
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		p, err := tx.Plan(id)
		if err != nil {
			return err
		}
		if p.Status == model.StatusCompleted {
			return repository.ErrConflict
		}
		if p.Status == model.StatusActive {
			if err = tx.CancelPlan(id, s.now().UTC()); err != nil {
				return err
			}
		}
		out, err = tx.Plan(id)
		return err
	})
	return out, err
}

func (s *Service) StartSession(ctx context.Context, user, planID uuid.UUID) (model.SessionView, error) {
	var out model.SessionView
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		p, err := tx.Plan(planID)
		if err != nil {
			return err
		}
		if p.Status != model.StatusActive {
			return repository.ErrConflict
		}
		session, err := tx.ActiveSession(p.ID)
		if errors.Is(err, repository.ErrNotFound) {
			now := s.now().UTC()
			session = model.TrainingSession{ID: uuid.New(), PlanID: p.ID, UserID: user, Status: model.StatusActive, StartedAt: now, CreatedAt: now}
			if err = tx.CreateSession(session); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if session.SelectionStrategy == model.SelectionInterviewGraphV1 {
			return fmt.Errorf("%w: resume or end the active mock interview first", repository.ErrConflict)
		}
		out, err = s.sessionView(ctx, tx, p, session)
		return err
	})
	return out, err
}

func (s *Service) GetSession(ctx context.Context, user, id uuid.UUID) (model.SessionView, error) {
	var out model.SessionView
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		session, err := tx.Session(id)
		if err != nil {
			return err
		}
		p, err := sessionPlan(tx, session)
		if err != nil {
			return err
		}
		out, err = s.sessionView(ctx, tx, p, session)
		return err
	})
	return out, err
}

func (s *Service) FinishSession(ctx context.Context, user, id uuid.UUID, status model.Status) (model.SessionView, error) {
	var out model.SessionView
	if status != model.StatusCompleted && status != model.StatusCancelled {
		return out, ErrInvalid
	}
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		session, err := tx.Session(id)
		if err != nil {
			return err
		}
		if session.Status != model.StatusActive && session.Status != status {
			return repository.ErrConflict
		}
		if session.Status == model.StatusActive {
			now := s.now().UTC()
			if session.SelectionStrategy == model.SelectionInterviewGraphV1 {
				state, err := tx.InterviewGraph().State(id)
				if err != nil {
					return err
				}
				state.StopReason = "user_finished"
				if _, err = tx.InterviewGraph().SaveState(id, state, state.Version, now); err != nil {
					return err
				}
			}
			if err = tx.FinishSession(id, status, now); err != nil {
				return err
			}
			session.Status = status
			session.FinishedAt = &now
		}
		p, err := sessionPlan(tx, session)
		if err != nil {
			return err
		}
		out, err = s.sessionView(ctx, tx, p, session)
		return err
	})
	return out, err
}

func fields(keys []string, c folderconfig.FolderConfig, values map[string]*string) []model.CardField {
	labels := map[string]string{}
	for _, f := range c.Schema.Fields {
		if f.Active {
			labels[f.Key] = f.Label
		}
	}
	out := make([]model.CardField, 0, len(keys))
	for _, key := range keys {
		label, ok := labels[key]
		value := values[key]
		if ok && value != nil && strings.TrimSpace(*value) != "" {
			out = append(out, model.CardField{Key: key, Label: label, Value: *value})
		}
	}
	return out
}
