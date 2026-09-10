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

func combinedIDs(view model.CombinedView) service.CombinedCurrentRequest {
	request := service.CombinedCurrentRequest{}
	for _, component := range view.Sessions {
		request.SessionIDs = append(request.SessionIDs, component.Session.ID)
	}
	return request
}

func (f *fixture) combined(t *testing.T, sources ...service.CombinedSource) model.CombinedView {
	t.Helper()
	return decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined", service.CombinedRequest{Sources: sources}), 200)
}

func (f *fixture) combinedCurrent(t *testing.T, view model.CombinedView) model.CombinedView {
	t.Helper()
	return decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/current", combinedIDs(view)), 200)
}

func TestCombinedMixedAlgorithmsTopicSelectionAndResume(t *testing.T) {
	f := newFixture(t)
	englishFolder := f.folder
	english := f.material(t, "Hello")
	excluded := f.material(t, "Not selected")
	f.exec(t, `UPDATE materials SET metadata='{"topic":"Greetings"}' WHERE id=?`, english)
	f.exec(t, `UPDATE materials SET metadata='{"topic":"Other"}' WHERE id=?`, excluded)
	interviewFolder := uuid.New()
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'Interview','interview_questions',config,'{"default_algorithm_key":"interview_long_term","pool_size":2}' FROM folders WHERE id=?`, interviewFolder, englishFolder)
	f.folder = interviewFolder
	interview := f.material(t, "What is an index?")
	f.exec(t, `UPDATE materials SET metadata='{"category":"SQL"}' WHERE id=?`, interview)
	view := f.combined(t, service.CombinedSource{FolderID: englishFolder, Topics: []string{"Greetings"}}, service.CombinedSource{FolderID: interviewFolder, Topics: []string{"SQL"}})
	if len(view.Sessions) != 2 || view.Current == nil {
		t.Fatal(view)
	}
	if view.Sessions[0].Plan.AlgorithmKey != "english_basic" || view.Sessions[1].Plan.AlgorithmKey != "interview_long_term" || view.Sessions[1].Plan.Config.HorizonDays != 150 {
		t.Fatal(view)
	}
	var n int64
	f.db.Table("user_material_progress").Where("material_id=?", excluded).Count(&n)
	if n != 0 {
		t.Fatal("filtered material admitted")
	}
	first := view.Current
	f.restart()
	retry := f.combinedCurrent(t, view)
	if !reflect.DeepEqual(first, retry.Current) {
		t.Fatal("refresh replaced unanswered snapshot", first, retry.Current)
	}
	var snapshots int64
	f.db.Table("training_session_items").Where("presentation IS NOT NULL").Count(&snapshots)
	if snapshots != 1 {
		t.Fatalf("snapshots prepared for hidden candidates: %d", snapshots)
	}
	req := actionFor(first.Presentation, algorithm.Correct)
	path := "/training/sessions/" + first.SessionID.String() + "/actions"
	answer := decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
	replayed := decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
	if !reflect.DeepEqual(answer, replayed) {
		t.Fatal("answer retry changed")
	}
	view = f.combinedCurrent(t, view)
	if view.Current == nil || view.Current.SessionID == first.SessionID {
		t.Fatal("combined queue did not alternate", view)
	}
	if view.Summary.Correct != 1 || view.Summary.MaterialsReviewed != 1 {
		t.Fatal(view.Summary)
	}
	var secondAction model.ActionResult
	for i := 0; i < 5 && view.Current != nil; i++ {
		secondAction = decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
		view = f.combinedCurrent(t, view)
	}
	if view.Current != nil || view.EmptyReason != "not_due" || view.NextReviewAt == nil {
		t.Fatal(view, secondAction)
	}
	for _, component := range view.Sessions {
		if component.Session.Status != model.StatusCompleted || component.Plan.Status != model.StatusActive {
			t.Fatal(component)
		}
	}
	before := combinedIDs(view)
	view = f.combinedCurrent(t, view)
	if !reflect.DeepEqual(before, combinedIDs(view)) {
		t.Fatal("empty refresh created sessions")
	}
	f.now = f.now.Add(24 * time.Hour)
	view = f.combinedCurrent(t, view)
	if view.Current == nil || reflect.DeepEqual(before, combinedIDs(view)) {
		t.Fatal("future due items not resumed", view)
	}
	if view.Summary.Correct != 0 {
		t.Fatal("new run must have its own summary")
	}
}

func TestCombinedSelectionReplacementOwnershipAndAtomicity(t *testing.T) {
	f := newFixture(t)
	sql := f.material(t, "SQL")
	goID := f.material(t, "Go")
	f.exec(t, `UPDATE materials SET metadata='{"topic":"SQL"}' WHERE id=?`, sql)
	f.exec(t, `UPDATE materials SET metadata='{"topic":"Go"}' WHERE id=?`, goID)
	view := f.combined(t, service.CombinedSource{FolderID: f.folder, Topics: []string{"SQL"}})
	if view.Current == nil || view.Current.Presentation.MaterialID != sql {
		t.Fatal(view)
	}
	replay := f.combined(t, service.CombinedSource{FolderID: f.folder, Topics: []string{"SQL"}})
	if !reflect.DeepEqual(view, replay) {
		t.Fatal("repeated launch differs")
	}
	f.now = f.now.Add(time.Second)
	next := f.combined(t, service.CombinedSource{FolderID: f.folder, Topics: []string{"Go"}})
	if next.Current == nil || next.Current.Presentation.MaterialID != goID || next.Sessions[0].Plan.ID != view.Sessions[0].Plan.ID {
		t.Fatal(next)
	}
	decode[map[string]any](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 409)
	decode[map[string]any](t, f.request(f.user, "POST", "/training/combined/current", combinedIDs(view)), 409)
	var plans int64
	f.db.Table("training_plans").Count(&plans)
	if plans != 1 {
		t.Fatal("selection created duplicate plans")
	}
	other := uuid.New()
	f.exec(t, `INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')`, other)
	decode[map[string]any](t, f.request(other, "POST", "/training/combined/current", combinedIDs(next)), 404)
	decode[map[string]any](t, f.request(other, "POST", "/training/combined", service.CombinedRequest{Sources: []service.CombinedSource{{FolderID: f.folder}}}), 404)
	decode[map[string]any](t, f.request(f.user, "POST", "/training/combined", service.CombinedRequest{Sources: []service.CombinedSource{{FolderID: f.folder}, {FolderID: uuid.New()}}}), 404)
	current := f.combinedCurrent(t, next)
	if current.Current.Presentation.ID != next.Current.Presentation.ID {
		t.Fatal("failed request changed existing session")
	}
}

func TestCombinedMainReviewPriorityAndSettingsPreserveRecovery(t *testing.T) {
	f := newFixture(t)
	a := f.material(t, "A")
	b := f.material(t, "B")
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	shown := view.Current.Presentation.MaterialID
	other := a
	if shown == a {
		other = b
	}
	future := f.now.AddDate(0, 0, 10)
	f.exec(t, `UPDATE user_material_progress SET stage_review_at=?,rehab_active=true,rehab_review_at=?,rehab_step=1,rehab_consecutive_correct=1,version=version+1 WHERE material_id=?`, future, f.now, shown)
	view = f.combinedCurrent(t, view)
	if view.Current.Presentation.MaterialID != other || view.Current.Presentation.Kind != model.ReviewStage {
		t.Fatal("stage did not supersede recovery", view)
	}
	plan := view.Sessions[0].Plan
	pool := 3
	req := service.ChangeRequest{CommandID: uuid.New(), ExpectedVersion: plan.Version, AlgorithmKey: plan.AlgorithmKey, Mode: algorithm.KeepStage, PoolSize: &pool, EndActiveSession: true}
	changed := decode[service.ChangeResult](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/algorithm", req), 200)
	if changed.Plan.Config.PoolSize != 3 || len(changed.Changes) != 0 {
		t.Fatal(changed)
	}
	var progress struct {
		RehabActive             bool
		RehabConsecutiveCorrect int
		StageReviewAt           time.Time
	}
	f.db.Table("user_material_progress").Where("material_id=?", shown).Take(&progress)
	if !progress.RehabActive || progress.RehabConsecutiveCorrect != 1 || !progress.StageReviewAt.Equal(future) {
		t.Fatal("pool change altered progress", progress)
	}
	decode[map[string]any](t, f.request(f.user, "POST", "/training/combined/current", combinedIDs(view)), 409)
	view = f.combined(t, service.CombinedSource{FolderID: f.folder})
	if view.Current == nil || view.Sessions[0].Plan.ID != plan.ID {
		t.Fatal(view)
	}
}
