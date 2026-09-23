package handler_test

import (
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func mockSnapshot(t *testing.T, f *fixture) string {
	t.Helper()
	var row struct{ Value string }
	err := f.db.Raw(`SELECT jsonb_build_object('plans',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM training_plans p),'sources',(SELECT jsonb_agg(to_jsonb(s)) FROM training_plan_sources s),'progress',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.material_id) FROM user_material_progress p))::text AS value`).Scan(&row).Error
	if err != nil {
		t.Fatal(err)
	}
	return row.Value
}
func TestMockMultiFolderPracticeOnlyAllModes(t *testing.T) {
	for _, mode := range []string{"real", "balanced", "custom", "deep"} {
		t.Run(mode, func(t *testing.T) {
			f, p, ids := graphFixture(t)
			graphDue(t, f, p, ids[0], true)
			graphDue(t, f, p, ids[1], false)
			second := uuid.New()
			f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'SQL',template_key,config,training_config FROM folders WHERE id=?`, second, f.folder)
			id := uuid.New()
			f.exec(t, `INSERT INTO materials(id,folder_id,values,metadata,difficulty) VALUES(?,?,'{"question":"SQL transactions?","answer":"Atomicity"}','{"topic":"SQL","category":"transactions"}','medium')`, id, second)
			// A second independent long-term plan must not prevent the combined mock.
			decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{second}, HorizonDays: 150}), 201)
			before := mockSnapshot(t, f)
			cfg := graph.DefaultConfig()
			cfg.InterviewMode = mode
			cfg.CustomWeights = map[string]float64{"go": 5, "sql": 3}
			cfg.DepthLevel = 3
			req := service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}, {FolderID: second}}, Config: &cfg}
			preview := decode[graph.InterviewPlan](t, f.request(f.user, "POST", "/training/mock-interviews/preview", req), 200)
			if len(preview.Topics) != 2 {
				t.Fatal(preview)
			}
			first := f.request(f.user, "POST", "/training/mock-interviews", req)
			v := decode[model.SessionView](t, first, 200)
			if v.Current == nil || v.Session.PlanID != uuid.Nil || !v.Graph.State.PracticeOnly || v.Current.InterviewGraph.ReviewCredit || v.Current.ProgressVersion != 0 {
				t.Fatalf("not a practice session: %+v", v)
			}
			if retry := f.request(f.user, "POST", "/training/mock-interviews", req); retry.Body.String() != first.Body.String() {
				t.Fatal("start not idempotent")
			}
			path := "/training/sessions/" + v.Session.ID.String()
			seen := map[uuid.UUID]bool{}
			for i := 0; i < 20; i++ {
				seen[v.Current.FolderID] = true
				old := v
				result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, "next_route")), 200)
				v = result.Session
				if v.Current == nil || v.Session.Status != model.StatusActive || v.Graph.State.AnsweredQuestions != 0 || len(v.Graph.State.CompletedRootIDs) != 0 || v.Graph.State.CurrentRoot != 1 {
					t.Fatal("skip consumed slot")
				}
				if i == 0 {
					restored := decode[model.SessionView](t, f.request(f.user, "POST", path+"/undo", service.GraphUndoRequest{CommandID: uuid.New(), EventID: result.Event.ID}), 200)
					if restored.Current.MaterialID != old.Current.MaterialID || !reflect.DeepEqual(restored.Graph.State.Plan, old.Graph.State.Plan) {
						t.Fatal("undo history")
					}
					v = restored
				}
			}
			if len(seen) != 2 {
				t.Fatal("not all sources participated")
			}
			f.restart()
			resumed := decode[model.SessionView](t, f.request(f.user, "GET", "/training/mock-interviews/active", nil), 200)
			if resumed.Current.ID != v.Current.ID {
				t.Fatal("resume changed question")
			}
			for i := 0; i < 4 && v.Current != nil; i++ {
				action := algorithm.Correct
				if i%2 == 1 {
					action = algorithm.Wrong
				}
				result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, action)), 200)
				v = result.Session
				if result.Event.ReviewCredit || result.Event.ProgressVersionAfter != 0 {
					t.Fatal("SRS event")
				}
			}
			if before != mockSnapshot(t, f) {
				t.Fatal("mock changed plans/progress")
			}
			if v.Graph.Statistics.Correct != v.Summary.Correct || v.Graph.Statistics.Wrong != v.Summary.Wrong {
				t.Fatal("repeat presentations multiplied statistics")
			}
			if len(v.UndoActions) > 0 {
				v = decode[model.SessionView](t, f.request(f.user, "POST", path+"/undo", service.GraphUndoRequest{CommandID: uuid.New(), EventID: v.UndoActions[0]}), 200)
			}
			if before != mockSnapshot(t, f) {
				t.Fatal("undo changed plans/progress")
			}
			decode[model.SessionView](t, f.request(f.user, "POST", path+"/finish", nil), 200)
		})
	}
}
func TestMockValidationIsolationAndActiveConflict(t *testing.T) {
	f, _, _ := graphFixture(t)
	cfg := graph.DefaultConfig()
	req := service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}}, Config: &cfg}
	for _, mode := range []string{"unknown", "custom"} {
		cfg.InterviewMode = mode
		decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", req), 400)
	}
	cfg = graph.DefaultConfig()
	cfg.InterviewMode = "deep"
	cfg.DepthLevel = 0
	decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", req), 400)
	cfg = graph.DefaultConfig()
	other := uuid.New()
	f.exec(t, `INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')`, other)
	decode[map[string]any](t, f.request(other, "POST", "/training/mock-interviews", req), 404)
	foreign := uuid.New()
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'English','english_words',config,training_config FROM folders WHERE id=?`, foreign, f.folder)
	req.Sources = append(req.Sources, model.SessionSource{FolderID: foreign})
	decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", req), 400)
	req.Sources = req.Sources[:1]
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", req), 200)
	cfg.InterviewMode = "balanced"
	req.CommandID = uuid.New()
	conflict := decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", req), 409)
	if fmt.Sprint(conflict) == "" {
		t.Fatal("missing conflict")
	}
	decode[map[string]any](t, f.request(other, "GET", "/training/sessions/"+v.Session.ID.String(), nil), 404)
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/sessions/"+v.Session.ID.String()+"/finish", nil), 200)
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", req), 200)
}
