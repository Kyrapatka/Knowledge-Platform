package handler_test

import (
	"context"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	trainingpg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestTrainingPoolThresholdAndEligibility(t *testing.T) {
	for _, tc := range []struct {
		key                        string
		target, threshold, horizon int
	}{
		{"interview_long_term", 5, 3, 90}, {"interview_cram", 5, 3, 3}, {"english_basic", 8, 5, 0}, {"english_adaptive", 8, 5, 0},
	} {
		for _, extra := range []int{1, 6} {
			t.Run(fmt.Sprintf("%s-extra%d", tc.key, extra), func(t *testing.T) {
				f := newFixture(t)
				admitted := map[uuid.UUID]bool{}
				for i := 0; i < tc.target+extra; i++ {
					id := f.material(t, fmt.Sprintf("eligible-%d", i))
					admitted[id] = true
				}
				completed, notDue := f.material(t, "completed"), f.material(t, "future")
				empty := f.material(t, "no answer")
				f.exec(t, `UPDATE materials SET values='{"front":"Question","back":""}' WHERE id=?`, empty)
				size := tc.target
				key := tc.key
				plan := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &key, PoolSize: &size, HorizonDays: tc.horizon}), 201)
				if plan.Config.RefillThreshold == nil || *plan.Config.RefillThreshold != tc.threshold {
					t.Fatal("threshold missing from plan", plan.Config)
				}
				for _, id := range []uuid.UUID{completed, notDue} {
					var planID *uuid.UUID
					if plan.Track == model.ProgressTrackCram {
						planID = &plan.ID
					}
					p, err := model.NewProgress(model.NewProgressParams{UserID: f.user, MaterialID: id, Track: plan.Track, PlanID: planID, AlgorithmKey: tc.key, AlgorithmVersion: 1, HorizonDays: tc.horizon, Now: f.now})
					if err != nil {
						t.Fatal(err)
					}
					if id == completed {
						p.CompletedAt = &f.now
						p.StageReviewAt = nil
					} else {
						next := f.now.Add(24 * time.Hour)
						p.StageReviewAt = &next
					}
					if err = trainingpg.NewProgressRepository(f.db).Create(context.Background(), p); err != nil {
						t.Fatal(err)
					}
				}
				view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
				path := "/training/sessions/" + view.Session.ID.String()
				active := func() map[uuid.UUID]bool {
					var ids []uuid.UUID
					if err := f.db.Table("training_session_items").Where("session_id=? AND state='active'", view.Session.ID).Pluck("material_id", &ids).Error; err != nil {
						t.Fatal(err)
					}
					out := map[uuid.UUID]bool{}
					for _, id := range ids {
						if out[id] || !admitted[id] {
							t.Fatal("duplicate/ineligible item admitted", id)
						}
						out[id] = true
					}
					return out
				}
				if len(active()) != tc.target {
					t.Fatal("initial pool", len(active()))
				}
				// Wrong recycles the current stage without admitting queue items.
				before := active()
				view = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Wrong)), 200).Session
				after := active()
				for id := range before {
					if !after[id] {
						t.Fatal("recycle removed active item")
					}
				}
				if len(after) != tc.target {
					t.Fatal("recycle changed size")
				}
				removed := map[uuid.UUID]bool{}
				for step := 1; step <= tc.target-tc.threshold; step++ {
					id := view.Current.MaterialID
					removed[id] = true
					view = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Advance)), 200).Session
					want := tc.target - step
					if want == tc.threshold {
						want += min(extra, tc.target-tc.threshold)
					}
					if view.PoolSize != want || len(active()) != want {
						t.Fatalf("after %d departures: %d want %d", step, view.PoolSize, want)
					}
					for gone := range removed {
						if active()[gone] {
							t.Fatal("not-due/completed-stage material re-added")
						}
					}
					// A refresh must not refill an above-threshold pool.
					view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
					if view.PoolSize != want {
						t.Fatal("refresh changed admission rule")
					}
				}
				for n := 0; n < 100 && view.Current != nil; n++ {
					removed[view.Current.MaterialID] = true
					view = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Advance)), 200).Session
					_ = active()
				}
				if view.Current != nil || len(removed) != len(admitted) {
					t.Fatal("pool did not drain eligible queue", len(removed), len(admitted))
				}
			})
		}
	}
}
