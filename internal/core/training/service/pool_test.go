package service

import (
	"context"
	"fmt"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"sort"
	"testing"
	"time"
)

type poolProgress struct {
	repository.ProgressRepository
	values map[uuid.UUID]model.UserMaterialProgress
}

func (p *poolProgress) Get(_ context.Context, key repository.ProgressKey) (model.UserMaterialProgress, error) {
	v, ok := p.values[key.MaterialID]
	if !ok {
		return v, repository.ErrNotFound
	}
	return v, nil
}
func (p *poolProgress) Create(_ context.Context, v model.UserMaterialProgress) error {
	p.values[v.MaterialID] = v
	return nil
}

type poolTx struct {
	repository.Tx
	folder    foldermodel.Folder
	materials map[uuid.UUID]materialmodel.Material
	items     map[uuid.UUID]model.SessionItem
	queue     []materialmodel.Material
	progress  poolProgress
	requests  []int
}

func (tx *poolTx) Folder(uuid.UUID) (foldermodel.Folder, error) { return tx.folder, nil }
func (tx *poolTx) Material(id uuid.UUID) (materialmodel.Material, error) {
	return tx.materials[id], nil
}
func (tx *poolTx) Progress() repository.ProgressRepository { return &tx.progress }
func (tx *poolTx) Items(uuid.UUID) ([]model.SessionItem, error) {
	out := []model.SessionItem{}
	for _, i := range tx.items {
		if i.State == "active" {
			out = append(out, i)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}
func (tx *poolTx) SaveItem(i model.SessionItem) error { tx.items[i.MaterialID] = i; return nil }
func (tx *poolTx) Candidates(_ model.TrainingPlan, _ uuid.UUID, _ time.Time, n int) ([]materialmodel.Material, error) {
	tx.requests = append(tx.requests, n)
	n = min(n, len(tx.queue))
	out := append([]materialmodel.Material(nil), tx.queue[:n]...)
	tx.queue = tx.queue[n:]
	return out, nil
}
func TestPoolEngineOnlyQueriesQueueAtThreshold(t *testing.T) {
	for _, tc := range []struct {
		key                        string
		target, threshold, horizon int
		track                      model.ProgressTrack
	}{
		{"english_basic", 8, 5, 0, model.ProgressTrackDefault}, {"interview_long_term", 5, 3, 90, model.ProgressTrackLongTerm},
	} {
		for _, extra := range []int{1, 8} {
			t.Run(fmt.Sprintf("%s-%d", tc.key, extra), func(t *testing.T) {
				now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
				tx := &poolTx{materials: map[uuid.UUID]materialmodel.Material{}, items: map[uuid.UUID]model.SessionItem{}, progress: poolProgress{values: map[uuid.UUID]model.UserMaterialProgress{}}}
				cfg := folderconfig.FolderConfig{}
				cfg.Card.QuestionFields = []string{"front"}
				cfg.Card.AnswerFields = []string{"back"}
				// Fields are read through the existing card schema, not hardcoded by the pool.
				cfg.Schema.Fields = []folderconfig.FieldDefinition{{Key: "front", Active: true}, {Key: "back", Active: true}}
				tx.folder = foldermodel.Folder{ID: uuid.New(), Config: cfg}
				p := model.TrainingPlan{ID: uuid.New(), UserID: uuid.New(), AlgorithmKey: tc.key, AlgorithmVersion: 1, Track: tc.track, SourceFolderIDs: []uuid.UUID{tx.folder.ID}, Config: model.PlanConfig{PoolSize: tc.target, HorizonDays: tc.horizon, Cards: map[string]folderconfig.FolderConfig{tx.folder.ID.String(): cfg}}}
				for i := 0; i < tc.target+extra; i++ {
					q, a := fmt.Sprint(i), "answer"
					m := materialmodel.Material{ID: uuid.New(), FolderID: tx.folder.ID, Values: map[string]*string{"front": &q, "back": &a}, Difficulty: materialmodel.DifficultyMedium}
					tx.materials[m.ID] = m
					tx.queue = append(tx.queue, m)
				}
				session := model.TrainingSession{ID: uuid.New()}
				svc := NewServiceWithClock(nil, func() time.Time { return now })
				prepare := func() []model.SessionItem {
					t.Helper()
					items, err := svc.preparePool(context.Background(), tx, p, session, false)
					if err != nil {
						t.Fatal(err)
					}
					return items
				}
				items := prepare()
				if len(items) != tc.target || len(tx.requests) != 1 {
					t.Fatal("initial admission")
				}
				// A still-due material is recycled with the same identity and no queue read.
				items = prepare()
				if len(items) != tc.target || len(tx.requests) != 1 {
					t.Fatal("recycle refilled pool")
				}
				removed := map[uuid.UUID]bool{}
				for step := 1; step <= tc.target-tc.threshold; step++ {
					id := items[0].MaterialID
					removed[id] = true
					progress := tx.progress.values[id]
					future := now.Add(24 * time.Hour)
					progress.StageReviewAt = &future
					tx.progress.values[id] = progress
					items = prepare()
					want := tc.target - step
					if step == tc.target-tc.threshold {
						want += min(extra, tc.target-tc.threshold)
					}
					if len(items) != want {
						t.Fatalf("step %d: got %d want %d", step, len(items), want)
					}
					wantQueries := 1
					if step == tc.target-tc.threshold {
						wantQueries = 2
					}
					if len(tx.requests) != wantQueries {
						t.Fatal("queue queried above threshold", tx.requests)
					}
					seen := map[uuid.UUID]bool{}
					for _, item := range items {
						if seen[item.MaterialID] || removed[item.MaterialID] {
							t.Fatal("duplicate/not due re-admission")
						}
						seen[item.MaterialID] = true
					}
				}
				if tx.requests[1] != tc.target-tc.threshold {
					t.Fatal("wrong refill batch", tx.requests)
				}
			})
		}
	}
}
