package handler_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
)

func TestEarlyReviewWindowRetryAndAnswer(t *testing.T) {
	f := newFixture(t)
	material := f.material(t, "Early card")
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	for i := 0; i < 3; i++ {
		decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
		view = f.combinedCurrent(t, view)
	}
	if view.Current != nil {
		t.Fatal("card should be cooling down")
	}
	due := f.now.Add(3 * time.Hour)
	f.exec(t, "UPDATE user_material_progress SET stage_review_at=? WHERE material_id=?", due, material)
	req := combinedIDs(view)
	req.ReviewEarly, req.CommandID = true, uuid.New()
	if got := f.request(f.user, "POST", "/training/combined/current", req); got.Code != 400 {
		t.Fatalf("exactly three hours must be rejected: %d %s", got.Code, got.Body.String())
	}
	f.now = f.now.Add(time.Second)
	ready := decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/current", req), 200)
	if ready.Current == nil || ready.Current.Presentation.MaterialID != material {
		t.Fatal("missing early presentation")
	}
	retry := decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/current", req), 200)
	if !reflect.DeepEqual(ready, retry) {
		t.Fatal("early command is not idempotent")
	}
	answer := decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+ready.Current.SessionID.String()+"/actions", actionFor(ready.Current.Presentation, algorithm.Correct)), 200)
	if answer.Event.Action != "correct" {
		t.Fatal(answer)
	}
	var events int64
	f.db.Table("training_events").Where("action='review_early'").Count(&events)
	if events != 1 {
		t.Fatal("retry duplicated early event", events)
	}
	if got := f.request(uuid.New(), "POST", "/training/combined/current", req); got.Code == 200 {
		t.Fatal("cross-owner early access")
	}
}

func TestEarlyRehabPreservesStageTimer(t *testing.T) {
	f := newFixture(t)
	material := f.material(t, "Recovery card")
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	stage, rehab := f.now.Add(10*24*time.Hour), f.now.Add(time.Hour)
	f.exec(t, "UPDATE user_material_progress SET stage_review_at=?, rehab_active=true, rehab_review_at=?, rehab_step=1 WHERE material_id=?", stage, rehab, material)
	view = f.combinedCurrent(t, view)
	req := combinedIDs(view)
	req.ReviewEarly, req.CommandID = true, uuid.New()
	ready := decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/current", req), 200)
	if ready.Current == nil || ready.Current.Presentation.Kind != model.ReviewRehab {
		t.Fatal("expected rehab", ready)
	}
	var row struct{ StageReviewAt time.Time }
	f.db.Table("user_material_progress").Where("material_id=?", material).Select("stage_review_at").Scan(&row)
	if !row.StageReviewAt.Equal(stage) {
		t.Fatal("rehab changed the main timer")
	}
}
