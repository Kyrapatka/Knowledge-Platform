package handler_test

import (
	"reflect"
	"testing"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
)

func TestEnglishDirectionAlternatesAfterAnswersAndSurvivesReload(t *testing.T) {
	f := newFixture(t)
	id := f.material(t, "Word")
	f.exec(t, `UPDATE folders SET config='{"schema":{"fields":[{"key":"foreign","label":"Foreign","active":true},{"key":"native","label":"Native","active":true},{"key":"example","label":"Example","active":true}]},"metadata_schema":{"fields":[]},"card":{"question_fields":["foreign"],"answer_fields":["native","example"]}}' WHERE id=?`, f.folder)
	f.exec(t, `UPDATE materials SET values='{"foreign":"hello","native":"привет","example":"Say hello."}' WHERE id=?`, id)
	view := f.combined(t, service.CombinedSource{FolderID: f.folder})
	first := view.Current.Presentation
	if first.Direction != "foreign" && first.Direction != "native" {
		t.Fatal("missing randomized direction", first)
	}
	if first.Example != "Say hello." || first.ForeignWord != "hello" {
		t.Fatal("missing example snapshot", first)
	}
	f.restart()
	if again := f.combinedCurrent(t, view); !reflect.DeepEqual(first, again.Current.Presentation) {
		t.Fatal("reload changed direction")
	}
	for _, action := range []algorithm.Action{algorithm.Wrong, algorithm.Correct} {
		before := view.Current.Presentation
		req := actionFor(before, action)
		path := "/training/sessions/" + view.Current.SessionID.String() + "/actions"
		decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
		decode[model.ActionResult](t, f.request(f.user, "POST", path, req), 200)
		view = f.combinedCurrent(t, view)
		after := view.Current.Presentation
		if after.Direction == before.Direction || after.Question[0].Value != before.Answer[0].Value {
			t.Fatal("must alternate exactly once after an answer", before, after)
		}
	}
}
