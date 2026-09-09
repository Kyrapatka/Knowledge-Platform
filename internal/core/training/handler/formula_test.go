package handler_test

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
)

func (f *fixture) exercise(t *testing.T, materialID uuid.UUID, problem string) model.FormulaExercise {
	t.Helper()
	return decode[model.FormulaExercise](t, f.request(f.user, "POST", "/materials/"+materialID.String()+"/exercises", service.ExerciseRequest{Problem: problem, Answer: "50 km/h", Solution: "100 km / 2 h = 50 km/h", Hint: "Distance divided by time"}), 201)
}
func (f *fixture) formulaPlan(t *testing.T) model.TrainingPlan {
	key := "formula_adaptive"
	return decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &key}), 201)
}
func containsField(fields []model.CardField, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

func TestFormulaHTTPModesAndExerciseRotation(t *testing.T) {
	f := newFixture(t)
	materialID := f.material(t, "Velocity")
	start := f.now
	f.exercise(t, materialID, "Travel 100 km in 2 h")
	f.exercise(t, materialID, "Travel 150 km in 3 h")
	plan := f.formulaPlan(t)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + view.Session.ID.String()
	if view.Current.PracticeMode != model.PracticeWorked || !containsField(view.Current.Question, "exercise_solution") {
		t.Fatal(view)
	}
	first := *view.Current.ExerciseID
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
	if result.Session.Current.ExerciseID == nil || *result.Session.Current.ExerciseID == first || result.Event.ExerciseID == nil || *result.Event.ExerciseID != first {
		t.Fatal("exercise must rotate and be recorded", result)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(result.Session.Current, algorithm.Correct)), 200)
	if result.NextReviewAt.Sub(start) != 24*time.Hour {
		t.Fatal(result)
	}
	f.now = *result.NextReviewAt
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current.PracticeMode != model.PracticeFaded || !containsField(view.Current.Question, "exercise_hint") || containsField(view.Current.Question, "exercise_solution") || containsField(view.Current.Question, "back") {
		t.Fatal("wrong faded content", view)
	}
	for i := 0; i < 2; i++ {
		result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
		view = result.Session
	}
	f.now = *result.NextReviewAt
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current.PracticeMode != model.PracticeIndependent || len(view.Current.Question) != 1 || view.Current.Question[0].Key != "exercise_problem" {
		t.Fatal("independent task leaks support", view)
	}
	for i := 0; i < 2; i++ {
		result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
		view = result.Session
	}
	if result.NextReviewAt.Sub(start) != 7*24*time.Hour {
		t.Fatal(result)
	}
	f.now = *result.NextReviewAt
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current.PracticeMode != model.PracticeMixed || len(view.Current.Question) != 1 {
		t.Fatal(view)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Wrong)), 200)
	if result.Event.StageAfter != 4 || result.Session.Current.PracticeMode != model.PracticeFaded {
		t.Fatal(result)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(result.Session.Current, algorithm.Correct)), 200)
	if result.Session.Current.PracticeMode != model.PracticeMixed || result.Event.StageAfter != 4 {
		t.Fatal(result)
	}
}

func TestFormulaExerciseCRUDSnapshotsAndEmptyPool(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "Velocity")
	plan := f.formulaPlan(t)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + view.Session.ID.String()
	base := "/materials/" + m.String() + "/exercises"
	if view.Current != nil || view.Session.Status != model.StatusActive {
		t.Fatal(view)
	}
	e := f.exercise(t, m, "Original problem")
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	original := view.Current
	req := service.ExerciseRequest{Problem: "Edited problem", Answer: "new answer", Solution: "new solution", Hint: "", ExpectedVersion: 1}
	updated := decode[model.FormulaExercise](t, f.request(f.user, "PUT", base+"/"+e.ID.String(), req), 200)
	if updated.Version != 2 || updated.Hint != "" {
		t.Fatal(updated)
	}
	decode[map[string]any](t, f.request(f.user, "PUT", base+"/"+e.ID.String(), req), 409)
	f.restart()
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if !reflect.DeepEqual(original, view.Current) {
		t.Fatal("exercise edit changed active snapshot")
	}
	if r := f.request(f.user, "DELETE", base+"/"+e.ID.String()+"?expected_version=1", nil); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := f.request(f.user, "DELETE", base+"/"+e.ID.String()+"?expected_version=2", nil); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	answer := actionFor(original, algorithm.Correct)
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", answer), 200)
	if result.Event.ExerciseID == nil || *result.Event.ExerciseID != e.ID || result.Session.Current != nil {
		t.Fatal(result)
	}
	retry := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", answer), 200)
	if !reflect.DeepEqual(result, retry) {
		t.Fatal("answer retry changed")
	}
	list := decode[[]model.FormulaExercise](t, f.request(f.user, "GET", base, nil), 200)
	if len(list) != 0 {
		t.Fatal(list)
	}
	f.exercise(t, m, "Replacement")
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current == nil || view.Current.ConsecutiveCorrect != 1 || view.Current.ExerciseVersion != 1 {
		t.Fatal("adding exercise must resume partial mastery", view)
	}
}

func TestFormulaExercisesOwnershipAndValidation(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "Velocity")
	other := uuid.New()
	f.exec(t, "INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')", other)
	base := "/materials/" + m.String() + "/exercises"
	e := f.exercise(t, m, "Problem")
	valid := service.ExerciseRequest{Problem: "Problem", Answer: "Answer", Solution: "Solution", ExpectedVersion: 1}
	for _, tc := range []struct {
		method, path string
		body         any
	}{{"GET", base, nil}, {"POST", base, valid}, {"PUT", base + "/" + e.ID.String(), valid}, {"DELETE", base + "/" + e.ID.String() + "?expected_version=1", nil}} {
		decode[map[string]any](t, f.request(other, tc.method, tc.path, tc.body), 404)
		decode[map[string]any](t, f.request(uuid.Nil, tc.method, tc.path, tc.body), 401)
	}
	valid.Solution = " "
	decode[map[string]any](t, f.request(f.user, "POST", base, valid), 400)
	otherMaterial := f.material(t, "Another")
	decode[map[string]any](t, f.request(f.user, "DELETE", "/materials/"+otherMaterial.String()+"/exercises/"+e.ID.String()+"?expected_version="+strconv.Itoa(e.Version), nil), 404)
}

func TestFormulaLateRecoveryHTTP(t *testing.T) {
	f := newFixture(t)
	m := f.material(t, "Velocity")
	f.exercise(t, m, "Problem")
	plan := f.formulaPlan(t)
	start := f.now
	f.exec(t, "UPDATE materials SET difficulty='easy' WHERE id=?", m)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + view.Session.ID.String()
	f.exec(t, "UPDATE user_material_progress SET stage=8,version=version+1 WHERE user_id=? AND material_id=?", f.user, m)
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Wrong)), 200)
	if result.Session.Current.Kind != model.ReviewRehab || result.Session.Current.PracticeMode != model.PracticeWorked {
		t.Fatal(result)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(result.Session.Current, algorithm.Correct)), 200)
	if result.NextReviewAt.Sub(start) != 48*time.Hour {
		t.Fatal(result)
	}
	f.now = *result.NextReviewAt
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current.PracticeMode != model.PracticeIndependent || len(view.Current.Question) != 1 {
		t.Fatal(view)
	}
	result = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Wrong)), 200)
	if result.Event.StageAfter != 7 || result.Session.Current.PracticeMode != model.PracticeWorked {
		t.Fatal(result)
	}
	progress := decode[service.MaterialProgressView](t, f.request(f.user, "GET", path+"/materials/"+m.String()+"/progress", nil), 200)
	if progress.StageReviewAt.Sub(f.now) != 30*24*time.Hour {
		t.Fatal(progress)
	}
}
