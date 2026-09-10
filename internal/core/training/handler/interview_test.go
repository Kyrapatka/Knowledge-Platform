package handler_test

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
	"time"
)

func (f *fixture) interviewPlan(t *testing.T, key string, horizon int) model.TrainingPlan {
	return decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &key, HorizonDays: horizon}), 201)
}
func TestCramHTTPFinalAndIsolation(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "Question")
	start := f.now
	f.exec(t, "UPDATE materials SET difficulty='easy' WHERE id=?", m)
	plan := f.interviewPlan(t, "interview_cram", 1)
	other := f.interviewPlan(t, "interview_cram", 1)
	session := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + session.Session.ID.String()
	finalPath := path + "/materials/" + m.String() + "/start-final"
	decode[map[string]any](t, f.request(f.user, "POST", finalPath, service.SkipRequest{CommandID: uuid.New(), ExpectedVersion: session.Current.ProgressVersion}), 400)
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Correct)), 200)
	if result.Session.Current != nil || result.NextReviewAt.Sub(start) != 8*time.Hour-30*time.Minute {
		t.Fatal(result)
	}
	f.now = start.Add(8 * time.Hour)
	f.restart()
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Correct)), 200)
	if result.Session.Session.Status != model.StatusActive || result.NextReviewAt.Sub(start) != 24*time.Hour-30*time.Minute {
		t.Fatal(result)
	}
	req := service.SkipRequest{CommandID: uuid.New(), ExpectedVersion: result.Event.ProgressVersionAfter}
	progress := decode[service.MaterialProgressView](t, f.request(f.user, "GET", path+"/materials/"+m.String()+"/progress", nil), 200)
	if !progress.CanStartFinal || progress.Version != req.ExpectedVersion || !progress.TargetAt.Equal(start.Add(24*time.Hour)) {
		t.Fatal(progress)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", finalPath, req), 200)
	retry := decode[model.ActionResult](t, f.request(f.user, "POST", finalPath, req), 200)
	if !reflect.DeepEqual(result, retry) || result.Session.Current == nil || !result.Session.Current.FinalReview {
		t.Fatal("final command not replayable", result, retry)
	}
	answer := actionFor(result.Session.Current, algorithm.Correct)
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", answer), 200)
	if result.Session.Session.Status != model.StatusCompleted || result.NextReviewAt != nil {
		t.Fatal(result)
	}
	retry = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", answer), 200)
	if !reflect.DeepEqual(result, retry) {
		t.Fatal("completion receipt changed")
	}
	completed := decode[model.TrainingPlan](t, f.request(f.user, "GET", "/training/plans/"+plan.ID.String(), nil), 200)
	if completed.Status != model.StatusCompleted {
		t.Fatal(completed)
	}
	session = decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+other.ID.String()+"/sessions", nil), 200)
	if session.Current == nil || session.Current.Stage != 1 {
		t.Fatal("CRAM progress leaked", session)
	}
}

func TestLongTermRecoveryHTTP(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "Question")
	start := f.now
	plan := f.interviewPlan(t, "interview_long_term", 365)
	session := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + session.Session.ID.String()
	f.exec(t, "UPDATE user_material_progress SET stage=11,version=version+1 WHERE user_id=? AND material_id=?", f.user, m)
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Wrong)), 200)
	if result.Event.StageAfter != 9 || result.Session.Current.Kind != model.ReviewRehab {
		t.Fatal(result)
	}
	for i := 0; i < 2; i++ {
		result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(result.Session.Current, algorithm.Correct)), 200)
	}
	if result.Session.Current != nil || result.NextReviewAt.Sub(start) != 48*time.Hour-30*time.Minute {
		t.Fatal(result)
	}
	f.now = start.AddDate(0, 0, 2)
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	for i := 0; i < 2; i++ {
		result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Correct)), 200)
		session = result.Session
	}
	if result.NextReviewAt.Sub(start) != 12*24*time.Hour-30*time.Minute {
		t.Fatal(result)
	}
	progress := decode[service.MaterialProgressView](t, f.request(f.user, "GET", path+"/materials/"+m.String()+"/progress", nil), 200)
	if progress.StageReviewAt.Sub(start) != 50*24*time.Hour-30*time.Minute {
		t.Fatal("rehab moved main schedule", progress)
	}
	f.now = start.AddDate(0, 0, 50)
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if session.Current.Kind != model.ReviewStage {
		t.Fatal(session)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Correct)), 200)
	if result.Event.StageAfter != 9 || result.Session.Current.ConsecutiveCorrect != 1 || result.Session.Current.RehabConsecutiveCorrect != 0 {
		t.Fatal("recovery affected stage mastery", result)
	}
}

func TestInterviewDynamicHorizonsAndOverdueFinal(t *testing.T) {
	f := newFixture(t)
	first := f.material(t, "First")
	start := f.now
	f.exec(t, "UPDATE materials SET difficulty='easy' WHERE id=?", first)
	plan := f.interviewPlan(t, "interview_long_term", 150)
	session := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + session.Session.ID.String()
	old := session.Current
	f.now = start.AddDate(0, 0, 100)
	second := f.material(t, "New")
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	var rows []struct {
		MaterialID                  uuid.UUID
		LearningStartedAt, TargetAt time.Time
	}
	if err := f.db.Table("user_material_progress").Where("user_id=? AND track='long_term'", f.user).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatal(rows)
	}
	for _, row := range rows {
		if row.TargetAt.Sub(row.LearningStartedAt) != 150*24*time.Hour {
			t.Fatal(row)
		}
		if row.MaterialID == second && !row.LearningStartedAt.Equal(f.now) {
			t.Fatal("late material shares old target")
		}
	}
	f.now = start.AddDate(0, 0, 150)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", actionFor(old, algorithm.Correct)), 409)
	session = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if session.Current.MaterialID != first || !session.Current.FinalReview {
		t.Fatal(session)
	}
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(session.Current, algorithm.Correct)), 200)
	if result.Session.Session.Status != model.StatusActive || result.NextReviewAt != nil {
		t.Fatal("new material must keep plan alive", result)
	}
	// A persistent track cannot use the same sources in a second active plan.
	key := "interview_long_term"
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &key, HorizonDays: 30}), 409)
}

func TestInterviewHorizonValidation(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		key  string
		days int
	}{{"interview_cram", 0}, {"interview_cram", 8}, {"interview_long_term", 6}, {"interview_long_term", 366}, {"english_basic", 1}} {
		decode[map[string]any](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &tc.key, HorizonDays: tc.days}), 400)
	}
}
