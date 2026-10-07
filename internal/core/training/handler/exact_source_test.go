package handler_test

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestExactMockHTTPPreviewTraversalResumeAndOwnership(t *testing.T) {
	for _, mode := range []string{"real", "deep"} {
		t.Run(mode, func(t *testing.T) {
			f, _, ids := graphFixture(t)
			cfg := graph.DefaultConfig()
			cfg.InterviewMode = mode
			cfg.DepthLevel = 3
			selected := []uuid.UUID{ids[0], ids[2]}
			req := service.StartGraphRequest{CommandID: uuid.New(), Config: &cfg, Sources: []model.SessionSource{{FolderID: f.folder, MaterialIDs: selected}}}
			preview := decode[graph.InterviewPlan](t, f.request(f.user, "POST", "/training/mock-interviews/preview", req), 200)
			total := 0
			for _, topic := range preview.Topics {
				total += topic.Available
			}
			if total != 2 {
				t.Fatal("preview widened selection", total)
			}
			v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", req), 200)
			for n := 0; n < 2; n++ {
				v = decode[model.SessionView](t, f.request(f.user, "GET", "/training/sessions/"+v.Session.ID.String(), nil), 200)
				if !reflect.DeepEqual(v.Session.Selection[0].MaterialIDs, selected) || v.Current == nil || v.Current.MaterialID == ids[1] {
					t.Fatal("exact source lost on resume")
				}
				v = decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+v.Session.ID.String()+"/actions", actionFor(v.Current, algorithm.Correct)), 200).Session
			}
			if v.Current != nil || v.Graph.State.StopReason != "no_remaining_questions" {
				t.Fatal("escaped selected set")
			}
			for _, bad := range [][]uuid.UUID{{}, {uuid.Nil}, {ids[0], ids[0]}} {
				body := map[string]any{"command_id": uuid.New(), "sources": []any{map[string]any{"folder_id": f.folder, "material_ids": bad}}}
				decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", body), 400)
			}
			req.CommandID = uuid.New()
			req.Sources[0].MaterialIDs = []uuid.UUID{uuid.New()}
			decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", req), 404)
		})
	}
}
