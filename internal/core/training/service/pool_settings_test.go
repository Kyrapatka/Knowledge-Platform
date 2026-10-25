package service

import (
	"context"
	"errors"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"testing"
	"time"
)

type settingsStore struct {
	repository.Tx
	folder foldermodel.Folder
	plan   model.TrainingPlan
}

func (m *settingsStore) Transact(_ context.Context, _ uuid.UUID, fn func(repository.Tx) error) error {
	return fn(m)
}
func (m *settingsStore) Folder(uuid.UUID) (foldermodel.Folder, error) { return m.folder, nil }
func (m *settingsStore) HasPlanOverlap([]uuid.UUID, model.ProgressTrack) (bool, error) {
	return false, nil
}
func (m *settingsStore) HasIncompatibleProgress([]uuid.UUID, model.ProgressTrack, string, int) (bool, error) {
	return false, nil
}
func (m *settingsStore) CreatePlan(p model.TrainingPlan) error { m.plan = p; return nil }
func (m *settingsStore) SaveDefaults(_ uuid.UUID, c folderconfig.TrainingConfig, v int64, _ time.Time) error {
	m.folder.TrainingConfig = c
	m.folder.TrainingConfigVersion = v + 1
	return nil
}
func TestPoolSettingsServiceValidationPersistence(t *testing.T) {
	for _, tc := range []struct {
		size, threshold int
		valid           bool
	}{{1, 0, true}, {5, 0, true}, {5, 3, true}, {5, 4, true}, {8, 5, true}, {12, 4, true}, {100, 99, true}, {0, 0, false}, {-1, 0, false}, {101, 5, false}, {5, -1, false}, {5, 5, false}, {5, 6, false}} {
		store := &settingsStore{folder: foldermodel.Folder{ID: uuid.New(), TrainingConfig: folderconfig.DefaultTrainingConfig("english_words"), TrainingConfigVersion: 1}}
		store.folder.Config.Schema.Fields = []folderconfig.FieldDefinition{{Key: "q", Active: true}, {Key: "a", Active: true}}
		store.folder.Config.Card.QuestionFields = []string{"q"}
		store.folder.Config.Card.AnswerFields = []string{"a"}
		svc := NewService(store)
		ctx := context.Background()
		user := uuid.New()
		_, err := svc.UpdateDefaults(ctx, user, store.folder.ID, folderconfig.TrainingConfig{DefaultAlgorithmKey: "english_basic", PoolSize: tc.size, RefillThreshold: &tc.threshold}, 1)
		if (err == nil) != tc.valid || err != nil && !errors.Is(err, ErrInvalid) {
			t.Fatalf("defaults %+v: %v", tc, err)
		}
		if tc.valid {
			loaded, e := svc.FolderDefaults(ctx, user, store.folder.ID)
			if e != nil || loaded.Config.PoolSize != tc.size || *loaded.Config.RefillThreshold != tc.threshold {
				t.Fatal("defaults lost", loaded, e)
			}
		}
		p, err := svc.CreatePlan(ctx, user, CreatePlanRequest{SourceFolderIDs: []uuid.UUID{store.folder.ID}, PoolSize: &tc.size, RefillThreshold: &tc.threshold})
		if (err == nil) != tc.valid || err != nil && !errors.Is(err, ErrInvalid) {
			t.Fatalf("plan %+v: %v", tc, err)
		}
		if tc.valid && (p.PoolPolicy() != (model.PoolPolicy{PoolSize: tc.size, RefillThreshold: tc.threshold}) || store.plan.PoolPolicy() != p.PoolPolicy()) {
			t.Fatal("plan settings changed")
		}
	}
}

func (m *settingsStore) Plan(uuid.UUID) (model.TrainingPlan, error) { return m.plan, nil }
func (m *settingsStore) Receipt(uuid.UUID) (model.CommandReceipt, error) {
	return model.CommandReceipt{}, repository.ErrNotFound
}
func (m *settingsStore) ActiveSession(uuid.UUID) (model.TrainingSession, error) {
	return model.TrainingSession{}, repository.ErrNotFound
}
func (m *settingsStore) PlanProgress(model.TrainingPlan) ([]model.UserMaterialProgress, error) {
	return nil, nil
}
func (m *settingsStore) UpdatePlan(p model.TrainingPlan, _ int) error { m.plan = p; return nil }
func (m *settingsStore) SavePlanChange(model.PlanChange) error        { return nil }
func (m *settingsStore) SaveReceipt(model.CommandReceipt) error       { return nil }
func TestChangingPoolSizePreservesThresholdAndRejectsInvalidCombination(t *testing.T) {
	threshold := 4
	size := 12
	m := &settingsStore{plan: model.TrainingPlan{ID: uuid.New(), UserID: uuid.New(), Version: 1, AlgorithmKey: "english_basic", AlgorithmVersion: 1, Track: model.ProgressTrackDefault, Status: model.StatusActive, Config: model.PlanConfig{PoolSize: size, RefillThreshold: &threshold}}}
	s := NewService(m)
	change := func(size int, threshold *int) (ChangeResult, error) {
		return s.ChangeAlgorithm(context.Background(), m.plan.UserID, m.plan.ID, ChangeRequest{CommandID: uuid.New(), ExpectedVersion: m.plan.Version, AlgorithmKey: "english_basic", Mode: algorithm.KeepStage, PoolSize: &size, RefillThreshold: threshold})
	}
	result, err := change(5, nil)
	if err != nil || result.Plan.Config.PoolSize != 5 || *result.Plan.Config.RefillThreshold != 4 {
		t.Fatal(result, err)
	}
	_, err = change(4, nil)
	if !errors.Is(err, ErrInvalid) || m.plan.Config.PoolSize != 5 || *m.plan.Config.RefillThreshold != 4 {
		t.Fatal("invalid edit changed settings", err, m.plan.Config)
	}
	zero := 0
	result, err = change(1, &zero)
	if err != nil || result.Plan.Config.PoolSize != 1 || *result.Plan.Config.RefillThreshold != 0 {
		t.Fatal(result, err)
	}
}
