package handler_test

import (
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"testing"
)

func TestMockFallbackHTTPCompletionAndIndependentProgress(t *testing.T) {
	for _, mode := range []string{"real", "deep"} {
		for _, limit := range []int{2, 24} {
			for _, onlyFallback := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s-limit%d-onlyFallback%t", mode, limit, onlyFallback), func(t *testing.T) {
					f, _, ids := graphFixture(t)
					if onlyFallback {
						f.exec(t, `UPDATE interview_question_profiles SET root_weight=0 WHERE folder_id=?`, f.folder)
					}
					profileSnapshot := func() string {
						var value string
						if err := f.db.Raw(`SELECT jsonb_agg(to_jsonb(p) ORDER BY material_id)::text FROM interview_question_profiles p`).Scan(&value).Error; err != nil {
							t.Fatal(err)
						}
						return value
					}
					profiles, before := profileSnapshot(), mockSnapshot(t, f)
					cfg := graph.DefaultConfig()
					cfg.InterviewMode = mode
					cfg.DepthLevel = 3
					cfg.QuestionLimit = limit
					req := service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}}, Config: &cfg}
					decode[graph.InterviewPlan](t, f.request(f.user, "POST", "/training/mock-interviews/preview", req), 200)
					// Preview must not consume candidates or persist session state.
					var count int64
					f.db.Table("interview_graph_session_state").Count(&count)
					if count != 0 {
						t.Fatal("preview created state")
					}
					view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", req), 200)
					if view.Current == nil {
						t.Fatal("could not start with eligible questions")
					}
					if onlyFallback && view.Graph.Selection.SelectionReason != "fallback_root" {
						t.Fatal("missing fallback event")
					}
					seen := map[uuid.UUID]bool{}
					fallback := onlyFallback
					for n := 0; n < 10 && view.Current != nil; n++ {
						if seen[view.Current.MaterialID] {
							t.Fatal("automatic repeat")
						}
						seen[view.Current.MaterialID] = true
						if view.Graph.Selection.SelectionReason == "fallback_root" {
							fallback = true
						}
						path := "/training/sessions/" + view.Session.ID.String() + "/actions"
						view = decode[model.ActionResult](t, f.request(f.user, "POST", path, actionFor(view.Current, algorithm.Correct)), 200).Session
					}
					want := min(limit, len(ids))
					reason := "no_remaining_questions"
					if limit <= len(ids) {
						reason = "question_limit"
					}
					if view.Current != nil || view.Session.Status != model.StatusCompleted || view.Graph.State.AnsweredQuestions != want || view.Graph.State.StopReason != reason || len(seen) != want {
						t.Fatalf("wrong completion %+v", view)
					}
					if limit > len(ids) && !fallback {
						t.Fatal("unconnected question was never a fallback")
					}
					if profiles != profileSnapshot() || before != mockSnapshot(t, f) {
						t.Fatal("Mock modified profile metadata or normal training")
					}
				})
			}
		}
	}
}
