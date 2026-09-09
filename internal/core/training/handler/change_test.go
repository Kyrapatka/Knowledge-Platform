package handler_test

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestChangeAlgorithmHTTP(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "A")
	plan := f.plan(t)
	session := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/plans/" + plan.ID.String() + "/algorithm"
	req := service.ChangeRequest{CommandID: uuid.New(), ExpectedVersion: plan.Version, AlgorithmKey: "english_adaptive", Mode: algorithm.KeepStage}
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 409)
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/sessions/"+session.Session.ID.String()+"/finish", nil), 200)
	anchor := f.now.AddDate(0, 0, -5)
	f.exec(t, "UPDATE user_material_progress SET stage=5,stage_last_review_at=?,stage_review_at=?,correct_count=17,wrong_count=4,consecutive_correct=2 WHERE user_id=? AND material_id=?", anchor, anchor.AddDate(0, 0, 10), f.user, m)
	result := decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	if result.Plan.Version != 2 || len(result.Changes) != 1 || result.Changes[0].StageAfter != 5 || !result.Changes[0].NextReviewAt.Equal(anchor.AddDate(0, 0, 10)) {
		t.Fatal(result)
	}
	retry := decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	if !reflect.DeepEqual(result, retry) {
		t.Fatal("change replay differs")
	}
	req.Mode = algorithm.ResetStage
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 409)
	req.CommandID = uuid.New()
	req.ExpectedVersion = 2
	result = decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	if result.Plan.Version != 3 || result.Changes[0].StageAfter != 1 || !result.Changes[0].NextReviewAt.Equal(f.now) {
		t.Fatal(result)
	}
	var counts struct{ CorrectCount, WrongCount, ConsecutiveCorrect int }
	if err := f.db.Table("user_material_progress").Where("user_id=? AND material_id=?", f.user, m).Take(&counts).Error; err != nil {
		t.Fatal(err)
	}
	if counts.CorrectCount != 17 || counts.WrongCount != 4 || counts.ConsecutiveCorrect != 0 {
		t.Fatal(counts)
	}
	changes := decode[[]model.PlanChange](t, f.request(f.user, "GET", "/training/plans/"+plan.ID.String()+"/changes", nil), 200)
	if len(changes) != 2 {
		t.Fatal(changes)
	}
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	if view.Current == nil || view.Current.RequiredCorrect != 4 {
		t.Fatal(view)
	}
}

func TestChangeAlgorithmAtomicFailureAndBounds(t *testing.T) {
	f := newFixture(t)
	f.material(t, "A")
	f.material(t, "B")
	plan := f.plan(t)
	session := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/sessions/"+session.Session.ID.String()+"/finish", nil), 200)
	path := "/training/plans/" + plan.ID.String() + "/algorithm"
	f.exec(t, `CREATE FUNCTION reject_plan_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_plan_change BEFORE INSERT ON training_plan_changes FOR EACH ROW EXECUTE FUNCTION reject_plan_change()`)
	req := service.ChangeRequest{CommandID: uuid.New(), ExpectedVersion: 1, AlgorithmKey: "formula_adaptive", Mode: algorithm.SetStage, Stage: 5}
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 500)
	var changed int64
	f.db.Table("user_material_progress").Where("algorithm_key='formula_adaptive'").Count(&changed)
	if changed != 0 {
		t.Fatal("progress partially committed")
	}
	current := decode[model.TrainingPlan](t, f.request(f.user, "GET", "/training/plans/"+plan.ID.String(), nil), 200)
	if current.Version != 1 || current.AlgorithmKey != "english_basic" {
		t.Fatal(current)
	}
	f.exec(t, "DROP TRIGGER reject_plan_change ON training_plan_changes")
	result := decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	if len(result.Changes) != 2 {
		t.Fatal(result)
	}
	req.CommandID = uuid.New()
	req.ExpectedVersion = 2
	req.Stage = 11
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 400)
	req.AlgorithmKey = "interview_cram"
	req.Stage = 1
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 400)
}

func TestChangeHorizonKeepsIndividualStartAndOwnership(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "A")
	start := f.now
	plan := f.interviewPlan(t, "interview_long_term", 150)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/sessions/"+view.Session.ID.String()+"/finish", nil), 200)
	f.now = start.AddDate(0, 0, 100)
	horizon := 30
	req := service.ChangeRequest{CommandID: uuid.New(), ExpectedVersion: 1, AlgorithmKey: "interview_long_term", Mode: algorithm.KeepStage, HorizonDays: &horizon}
	path := "/training/plans/" + plan.ID.String() + "/algorithm"
	other := uuid.New()
	f.exec(t, "INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')", other)
	decode[map[string]any](t, f.request(other, "POST", path, req), 404)
	decode[map[string]any](t, f.request(other, "GET", "/training/plans/"+plan.ID.String()+"/changes", nil), 404)
	result := decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	if result.Plan.Config.HorizonDays != 30 {
		t.Fatal(result)
	}
	progressPath := "/training/sessions/" + view.Session.ID.String() + "/materials/" + m.String() + "/progress"
	progress := decode[service.MaterialProgressView](t, f.request(f.user, "GET", progressPath, nil), 200)
	if !progress.TargetAt.Equal(start.AddDate(0, 0, 30)) {
		t.Fatal("KEEP restarted horizon", progress)
	}
	req.CommandID = uuid.New()
	req.ExpectedVersion = 2
	req.Mode = algorithm.ResetStage
	decode[service.ChangeResult](t, f.request(f.user, "POST", path, req), 200)
	progress = decode[service.MaterialProgressView](t, f.request(f.user, "GET", progressPath, nil), 200)
	if !progress.TargetAt.Equal(f.now.AddDate(0, 0, 30)) || !progress.StageReviewAt.Equal(f.now) {
		t.Fatal("RESET must restart horizon", progress)
	}
}
