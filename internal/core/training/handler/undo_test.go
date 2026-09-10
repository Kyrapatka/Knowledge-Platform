package handler_test

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestUndoThreeAnswersRestoresProgressAndStatistics(t *testing.T) {
	f := newFixture(t)
	material := f.material(t, "Undo card")
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	for _, action := range []algorithm.Action{algorithm.Wrong, algorithm.Correct, algorithm.Correct, algorithm.Correct} {
		decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, action)), 200)
		view = f.combinedCurrent(t, view)
	}
	if view.Current != nil || len(view.UndoActions) != 3 {
		t.Fatal("expected cooldown and three undo entries", view)
	}
	version := 0
	for n := 3; n > 0; n-- {
		req := service.UndoRequest{CommandID: uuid.New(), EventID: view.UndoActions[0], SessionIDs: combinedIDs(view).SessionIDs}
		view = decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/undo", req), 200)
		again := decode[model.CombinedView](t, f.request(f.user, "POST", "/training/combined/undo", req), 200)
		if !reflect.DeepEqual(view, again) {
			t.Fatal("undo must be idempotentent")
		}
		if view.Current == nil || view.Current.Presentation.MaterialID != material || view.Current.Presentation.Stage != 1 || view.Current.Presentation.ProgressVersion <= version || len(view.UndoActions) != n-1 {
			t.Fatal("invalid undo state", view)
		}
		version = view.Current.Presentation.ProgressVersion
	}
	if view.Summary.Correct != 0 || view.Summary.Wrong != 1 || view.Summary.StagePromotions != 0 {
		t.Fatal("undone answers still counted", view.Summary)
	}
	req := service.UndoRequest{CommandID: uuid.New(), EventID: uuid.New(), SessionIDs: combinedIDs(view).SessionIDs}
	if got := f.request(f.user, "POST", "/training/combined/undo", req); got.Code != 409 {
		t.Fatal("fourth undo accepted", got.Code)
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	view = f.combinedCurrent(t, view)
	if view.Summary.Correct != 1 || len(view.UndoActions) != 1 {
		t.Fatal("could not answer restored card", view)
	}
}

func TestUndoRejectsStaleOrForeignRequests(t *testing.T) {
	f := newFixture(t)
	f.material(t, "Owned card")
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Wrong)), 200)
	view = f.combinedCurrent(t, view)
	req := service.UndoRequest{CommandID: uuid.New(), EventID: view.UndoActions[0], SessionIDs: combinedIDs(view).SessionIDs}
	if got := f.request(uuid.New(), "POST", "/training/combined/undo", req); got.Code == 200 {
		t.Fatal("cross-user undo accepted")
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+view.Current.SessionID.String()+"/actions", actionFor(view.Current.Presentation, algorithm.Correct)), 200)
	if got := f.request(f.user, "POST", "/training/combined/undo", req); got.Code != 409 {
		t.Fatal("stale undo accepted", got.Code)
	}
}
