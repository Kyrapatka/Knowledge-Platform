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

func undoLatest(t *testing.T, f *fixture, view model.CombinedView) model.CombinedView {
	t.Helper()
	if len(view.UndoActions) == 0 {
		t.Fatal("expected an available undo")
	}
	req := service.UndoRequest{CommandID: uuid.New(), EventID: view.UndoActions[0], SessionIDs: combinedIDs(view).SessionIDs}
	return decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/undo", req), 200)
}

func TestUndoAcrossCombinedSessionsRestoresAnsweredCard(t *testing.T) {
	f := newFixture(t)
	firstFolder := f.folder
	f.material(t, "First folder card")
	secondFolder := uuid.New()
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'Second',template_key,config,training_config FROM folders WHERE id=?`, secondFolder, firstFolder)
	f.folder = secondFolder
	f.material(t, "Second folder card")
	view := f.combined(t, service.CombinedSource{FolderID: firstFolder}, service.CombinedSource{FolderID: secondFolder})
	var answered []*model.CombinedCurrent
	for _, action := range []algorithm.Action{algorithm.Correct, algorithm.Wrong, algorithm.Correct} {
		answered = append(answered, view.Current)
		decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, action)), 200)
		view = f.combinedCurrent(t, view)
	}
	if answered[0].SessionID == answered[1].SessionID {
		t.Fatal("fixture did not exercise different sessions")
	}
	for i := len(answered) - 1; i >= 0; i-- {
		view = undoLatest(t, f, view)
		if view.Current == nil || view.Current.SessionID != answered[i].SessionID || view.Current.Presentation.MaterialID != answered[i].Presentation.MaterialID {
			t.Fatal("undo did not return to the answered card", view.Current, answered[i])
		}
		if view.Current.Presentation.ID == answered[i].Presentation.ID || view.Current.Presentation.ProgressVersion <= answered[i].Presentation.ProgressVersion {
			t.Fatal("restored card must have a fresh command identity")
		}
		if again := f.combinedCurrent(t, view); !reflect.DeepEqual(view.Current, again.Current) {
			t.Fatal("reload changed restored card")
		}
	}
	if view.Summary.Correct != 0 || view.Summary.Wrong != 0 || view.Summary.MaterialsReviewed != 0 {
		t.Fatal("undone mixed-session answers still counted", view.Summary)
	}
}

func TestUndoCompletedFinalRestoresProgressAndReopensPlan(t *testing.T) {
	f := newFixture(t)
	id := f.material(t, "Final card")
	f.exec(t, "UPDATE materials SET difficulty='easy' WHERE id=?", id)
	plan := f.interviewPlan(t, "interview_cram", 1)
	view := f.combined(t, service.CombinedSource{FolderID: f.folder, PlanID: &plan.ID})
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	view = f.combinedCurrent(t, view)
	f.now = f.now.Add(24 * time.Hour)
	view = f.combinedCurrent(t, view)
	if view.Current == nil || !view.Current.Presentation.FinalReview {
		t.Fatal("expected final review", view.Current)
	}
	var before model.UserMaterialProgress
	if err := f.db.Table("user_material_progress").Where("material_id=? AND plan_id=?", id, plan.ID).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	view = f.combinedCurrent(t, view)
	if view.Current != nil || view.Sessions[0].Plan.Status != model.StatusCompleted || view.Sessions[0].Session.Status != model.StatusCompleted {
		t.Fatal("final review did not complete plan", view)
	}
	view = undoLatest(t, f, view)
	if view.Current == nil || !view.Current.Presentation.FinalReview || view.Sessions[0].Plan.Status != model.StatusActive || view.Sessions[0].Session.Status != model.StatusActive {
		t.Fatal("undo did not reopen completed final review", view)
	}
	var after model.UserMaterialProgress
	if err := f.db.Table("user_material_progress").Where("material_id=? AND plan_id=?", id, plan.ID).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Version <= before.Version {
		t.Fatal("undo must preserve monotonic versions")
	}
	after.Version, after.UpdatedAt = before.Version, before.UpdatedAt
	if !reflect.DeepEqual(before, after) {
		t.Fatal("undo changed counters, mastery, or the original schedule", before, after)
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	view = f.combinedCurrent(t, view)
	if view.Sessions[0].Plan.Status != model.StatusCompleted || view.Summary.Correct != 1 {
		t.Fatal("restored final could not complete again", view)
	}
}

func TestUndoEnglishPreservesDirectionAndIgnoresOldAnswerRetry(t *testing.T) {
	f := newFixture(t)
	id := f.material(t, "Word")
	f.exec(t, `UPDATE folders SET config='{"schema":{"fields":[{"key":"foreign","label":"Foreign","active":true},{"key":"native","label":"Native","active":true}]},"metadata_schema":{"fields":[]},"card":{"question_fields":["foreign"],"answer_fields":["native"]}}' WHERE id=?`, f.folder)
	f.exec(t, `UPDATE materials SET values='{"foreign":"hello","native":"привет"}' WHERE id=?`, id)
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	first := view.Current.Presentation
	req := actionFor(first, algorithm.Wrong)
	path := "/training/sessions/" + view.Current.SessionID.String() + "/actions"
	decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
	view = f.combinedCurrent(t, view)
	view = undoLatest(t, f, view)
	if view.Current.Presentation.Direction != first.Direction || !reflect.DeepEqual(view.Current.Presentation.Question, first.Question) {
		t.Fatal("undo must preserve the original side")
	}
	// A delayed network retry returns its receipt without reapplying the answer.
	decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
	view = f.combinedCurrent(t, view)
	if view.Summary.Wrong != 0 || view.Current.Presentation.Direction != first.Direction {
		t.Fatal("retry resurrected the undone answer", view)
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", path, actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	view = f.combinedCurrent(t, view)
	if view.Current.Presentation.Direction == first.Direction || view.Summary.Correct != 1 || view.Summary.Wrong != 0 {
		t.Fatal("re-answer must alternate exactly once", view)
	}
}
