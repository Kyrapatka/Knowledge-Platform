package handler_test

import (
	"context"
	"encoding/json"
	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	pg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func graphFixture(t *testing.T) (*fixture, model.TrainingPlan, []uuid.UUID) {
	t.Helper()
	f := newFixture(t)
	cfg := `{"schema":{"fields":[{"key":"question","label":"Question","active":true},{"key":"answer","label":"Answer","active":true}]},"metadata_schema":{"fields":[]},"card":{"question_fields":["question"],"answer_fields":["answer"]}}`
	f.exec(t, `UPDATE folders SET template_key='interview_questions',config=?,training_config='{"default_algorithm_key":"interview_long_term","pool_size":5}' WHERE id=?`, cfg, f.folder)
	ids := []uuid.UUID{}
	for i, slug := range []string{"goroutine", "context_switch", "stack"} {
		id, concept := uuid.New(), uuid.New()
		ids = append(ids, id)
		values, _ := json.Marshal(map[string]string{"question": "Explain " + slug, "answer": "Reference " + slug})
		f.exec(t, `INSERT INTO materials(id,folder_id,values,metadata,difficulty) VALUES(?,?,?,'{"topic":"Runtime"}','easy')`, id, f.folder, string(values))
		root := 0
		if i == 0 {
			root = 10
		}
		f.exec(t, `INSERT INTO interview_question_profiles(material_id,folder_id,owner_id,seed_key,domain,frequency,interview_difficulty,specificity,root_weight,status) VALUES(?,?,?,?,'go',8,2,2,?,'ready')`, id, f.folder, f.user, slug, root)
		f.exec(t, `INSERT INTO interview_concepts(id,owner_id,slug,display_name,domain,topic) VALUES(?,?,?,?,'go','Runtime')`, concept, f.user, slug, slug)
		for _, role := range []string{"primary", "tested"} {
			f.exec(t, `INSERT INTO interview_question_concepts(material_id,concept_id,role) VALUES(?,?,?)`, id, concept, role)
		}
		alias := slug
		if slug == "context_switch" {
			alias = "context switch"
		}
		f.exec(t, `INSERT INTO interview_concept_aliases(id,concept_id,alias,normalized_alias) VALUES(?,?,?,?)`, uuid.New(), concept, alias, alias)
	}
	f.exec(t, `INSERT INTO interview_question_concepts(material_id,concept_id,role) SELECT ?,id,'answer' FROM interview_concepts WHERE owner_id=? AND slug='context_switch'`, ids[0], f.user)
	plan := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, HorizonDays: 150}), 201)
	return f, plan, ids
}
func startGraph(t *testing.T, f *fixture, p model.TrainingPlan, limit int) model.SessionView {
	t.Helper()
	config := graph.DefaultConfig()
	config.QuestionLimit = limit
	return decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/interview-graph", service.StartGraphRequest{CommandID: uuid.New(), Config: &config}), 200)
}
func graphProgress(t *testing.T, f *fixture, id uuid.UUID) string {
	t.Helper()
	var row struct{ Value string }
	if err := f.db.Raw(`SELECT to_jsonb(p)::text AS value FROM user_material_progress p WHERE material_id=?`, id).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.Value
}
func graphDue(t *testing.T, f *fixture, p model.TrainingPlan, id uuid.UUID, due bool) {
	t.Helper()
	progress, err := model.NewProgress(model.NewProgressParams{UserID: f.user, MaterialID: id, Track: p.Track, AlgorithmKey: p.AlgorithmKey, AlgorithmVersion: 1, HorizonDays: 150, Now: f.now.Add(-24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	at := f.now.Add(5 * 24 * time.Hour)
	if due {
		at = f.now.Add(-time.Hour)
	}
	progress.StageReviewAt = &at
	if err = pg.NewProgressRepository(f.db).Create(context.Background(), progress); err != nil {
		t.Fatal(err)
	}
}
func TestGraphStartEndpointAndResume(t *testing.T) {
	f, p, ids := graphFixture(t)
	config := graph.DefaultConfig()
	req := service.StartGraphRequest{CommandID: uuid.New(), Config: &config}
	path := "/training/plans/" + p.ID.String() + "/interview-graph"
	first := f.request(f.user, "POST", path, req)
	v := decode[model.SessionView](t, first, 200)
	if v.Current == nil || v.Current.MaterialID != ids[0] || v.PoolSize != 1 || v.Session.SelectionStrategy != model.SelectionInterviewGraphV1 || v.Current.InterviewGraph.Probe {
		t.Fatalf("bad graph start: %+v", v)
	}
	if retry := f.request(f.user, "POST", path, req); retry.Body.String() != first.Body.String() {
		t.Fatal("start receipt changed")
	}
	config.QuestionLimit = 2
	req.Config = &config
	decode[map[string]any](t, f.request(f.user, "POST", path, req), 409)
	f.restart()
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{question}','"edited question"') WHERE id=?`, ids[0])
	sessionPath := "/training/sessions/" + v.Session.ID.String()
	again := decode[model.SessionView](t, f.request(f.user, "GET", sessionPath, nil), 200)
	if !reflect.DeepEqual(v.Current, again.Current) || !reflect.DeepEqual(v.Graph.State, again.Graph.State) {
		t.Fatal("snapshot or restart state changed")
	}
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 409)
	decode[map[string]any](t, f.request(f.user, "POST", "/training/combined", map[string]any{"selection_strategy": "interview_graph_v1", "sources": []map[string]any{{"folder_id": f.folder}}}), 400)
}
func TestGraphProbeUndoCompletionAndPrivacy(t *testing.T) {
	f, p, ids := graphFixture(t)
	graphDue(t, f, p, ids[1], false)
	before := graphProgress(t, f, ids[1])
	v := startGraph(t, f, p, 2)
	path := "/training/sessions/" + v.Session.ID.String()
	firstReq := actionFor(v.Current, algorithm.Correct)
	firstReq.AnswerText = "A context switch preserves execution state"
	firstReq.AnswerLanguage = "en"
	first := f.request(f.user, "POST", path+"/actions", firstReq)
	a := decode[model.ActionResult](t, first, 200)
	if a.Session.Current == nil || a.Session.Current.MaterialID != ids[1] || !a.Session.Current.InterviewGraph.Probe || a.Session.Graph.Selection.SelectionReason != "answer_concept" {
		t.Fatalf("missing semantic probe: %+v", a.Session)
	}
	if len(a.Session.Graph.Selection.DetectedConcepts) == 0 || len(a.Session.Graph.Selection.Candidates) == 0 {
		t.Fatal("missing debug")
	}
	if retry := f.request(f.user, "POST", path+"/actions", firstReq); retry.Body.String() != first.Body.String() {
		t.Fatal("answer replay changed")
	}
	firstReq.AnswerText = "stack"
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", firstReq), 409)
	var stored int64
	if err := f.db.Table("interview_graph_selection_events").Where("session_id=? AND raw_answer IS NOT NULL", v.Session.ID).Count(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("raw answer was retained")
	}
	var payloads []string
	if err := f.db.Raw("SELECT snapshot::text FROM interview_graph_selection_events WHERE session_id=?", v.Session.ID).Scan(&payloads).Error; err != nil {
		t.Fatal(err)
	}
	for _, x := range payloads {
		if strings.Contains(x, "preserves execution") {
			t.Fatal("raw answer in snapshot")
		}
	}
	lastReq := actionFor(a.Session.Current, algorithm.Wrong)
	last := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", lastReq), 200)
	if graphProgress(t, f, ids[1]) != before || last.Event.ReviewCredit || last.Event.ProgressVersionBefore != last.Event.ProgressVersionAfter {
		t.Fatal("probe mutated SRS")
	}
	if last.Session.Current != nil || last.Session.Session.Status != model.StatusCompleted || last.Session.Graph.State.StopReason != "question_limit" {
		t.Fatal("limit failed")
	}
	stats := last.Session.Graph.Statistics
	if stats.ScheduledReviews != 1 || stats.InterviewProbes != 1 || stats.Correct != 1 || stats.Wrong != 1 || len(stats.ConceptCoverage) != 2 {
		t.Fatalf("bad graph statistics %+v", stats)
	}
	plan := decode[model.TrainingPlan](t, f.request(f.user, "GET", "/training/plans/"+p.ID.String(), nil), 200)
	if plan.Status != model.StatusActive || plan.Config.PoolSize != 5 {
		t.Fatal("graph changed plan")
	}
	undo := service.GraphUndoRequest{CommandID: uuid.New(), EventID: last.Event.ID}
	restoredResponse := f.request(f.user, "POST", path+"/undo", undo)
	restored := decode[model.SessionView](t, restoredResponse, 200)
	if restored.Current == nil || restored.Current.MaterialID != ids[1] || restored.Current.ID == a.Session.Current.ID || restored.Session.Status != model.StatusActive || graphProgress(t, f, ids[1]) != before {
		t.Fatal("completed probe undo failed")
	}
	if repeat := f.request(f.user, "POST", path+"/undo", undo); repeat.Body.String() != restoredResponse.Body.String() {
		t.Fatal("undo replay changed")
	}
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", actionFor(a.Session.Current, algorithm.Correct)), 409)
	root := decode[model.SessionView](t, f.request(f.user, "POST", path+"/undo", service.GraphUndoRequest{CommandID: uuid.New(), EventID: a.Event.ID}), 200)
	if root.Current.MaterialID != ids[0] || root.Graph.State.RandomIndex != v.Graph.State.RandomIndex || !reflect.DeepEqual(root.Graph.State.AskedMaterialIDs, v.Graph.State.AskedMaterialIDs) || root.Summary.Correct != 0 {
		t.Fatal("root undo failed")
	}
	f.restart()
	reloaded := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if !reflect.DeepEqual(root.Current, reloaded.Current) {
		t.Fatal("undo reload changed question")
	}
}
func TestGraphDueFollowUpAndFrontierUndo(t *testing.T) {
	f, p, ids := graphFixture(t)
	f.exec(t, `INSERT INTO interview_question_concepts(material_id,concept_id,role,ordinal) SELECT ?,id,'answer',1 FROM interview_concepts WHERE owner_id=? AND slug='stack'`, ids[0], f.user)
	graphDue(t, f, p, ids[1], true)
	graphDue(t, f, p, ids[2], true)
	v := startGraph(t, f, p, 5)
	path := "/training/sessions/" + v.Session.ID.String()
	req := actionFor(v.Current, algorithm.Correct)
	req.AnswerText = "context switch and stack"
	a := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", req), 200)
	if len(a.Session.Graph.State.Frontier) != 1 || a.Session.Current.InterviewGraph.Probe {
		t.Fatal("frontier/due")
	}
	prior := graphProgress(t, f, a.Session.Current.MaterialID)
	b := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(a.Session.Current, algorithm.Correct)), 200)
	if graphProgress(t, f, a.Session.Current.MaterialID) == prior || !b.Event.ReviewCredit || b.Session.Graph.Selection.SelectionReason != "frontier" {
		t.Fatal("due follow-up not credited or frontier unused")
	}
	u := decode[model.SessionView](t, f.request(f.user, "POST", path+"/undo", service.GraphUndoRequest{CommandID: uuid.New(), EventID: b.Event.ID}), 200)
	if !reflect.DeepEqual(u.Graph.State.Frontier, a.Session.Graph.State.Frontier) || u.Graph.State.RandomIndex != a.Session.Graph.State.RandomIndex || u.Graph.State.CurrentDepth != a.Session.Graph.State.CurrentDepth {
		t.Fatal("undo did not restore graph")
	}
}
func TestGraphConcurrentActionsAndAtomicSelection(t *testing.T) {
	f, p, _ := graphFixture(t)
	v := startGraph(t, f, p, 3)
	path := "/training/sessions/" + v.Session.ID.String()
	req := actionFor(v.Current, algorithm.Correct)
	req.AnswerText = "context switch"
	f.exec(t, `CREATE FUNCTION reject_graph_selection() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test selection failure'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_graph BEFORE INSERT ON interview_graph_selection_events FOR EACH ROW EXECUTE FUNCTION reject_graph_selection()`)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", req), 500)
	same := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if !reflect.DeepEqual(same.Current, v.Current) || same.Summary.Correct != 0 || same.Graph.State.Version != v.Graph.State.Version {
		t.Fatal("partial transaction")
	}
	f.exec(t, `DROP TRIGGER reject_graph ON interview_graph_selection_events`)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := req
			q.CommandID = uuid.New()
			codes <- f.request(f.user, "POST", path+"/actions", q).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestGraphDeletedFolderAndSourceValidation(t *testing.T) {
	f, p, _ := graphFixture(t)
	bad := service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: uuid.New()}}}
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/interview-graph", bad), 400)
	v := startGraph(t, f, p, 5)
	path := "/training/sessions/" + v.Session.ID.String()
	f.exec(t, `UPDATE folders SET deleted_at=? WHERE id=?`, f.now, f.folder)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Correct)), 409)
	out := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if out.Current != nil || out.Session.Status != model.StatusCompleted {
		t.Fatal("deleted sources not handled")
	}
}
func TestSeedImportImportsEdgesAndProfileLifecycle(t *testing.T) {
	f := newFixture(t)
	group := f.router.Group("/api/v1")
	group.Use(auth.AuthMiddleware(testParser{}))
	interview.NewHandler(f.db).RegisterRoutes(group)
	imported := decode[interview.ImportResult](t, f.request(f.user, "POST", "/interview/seed/import", map[string]any{"domains": []string{"go"}}), 200)
	if imported.Created != 130 || len(imported.FolderIDs) != 1 {
		t.Fatal(imported)
	}
	var n int64
	f.db.Table("interview_concept_edges").Count(&n)
	// Only 72 legacy edges have both endpoints in the corrected dictionary.
	if n != 72 {
		t.Fatal("missing seed edges", n)
	}
	again := decode[interview.ImportResult](t, f.request(f.user, "POST", "/interview/seed/import", map[string]any{"domains": []string{"go"}}), 200)
	if again.Created != 0 || again.Updated != 0 || again.Skipped != 130 {
		t.Fatal("non-idempotent import", again)
	}
	var profile interview.Profile
	if err := f.db.Table("interview_question_profiles").Where("folder_id=? AND seed_key='GO002'", imported.FolderIDs[0]).Take(&profile).Error; err != nil {
		t.Fatal(err)
	}
	path := "/folders/" + imported.FolderIDs[0].String() + "/interview/questions/" + profile.MaterialID.String() + "/profile"
	got := decode[interview.Profile](t, f.request(f.user, "GET", path, nil), 200)
	got.Status = "ready"
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{short_answer}','"."') WHERE id=?`, got.MaterialID)
	request := interview.ProfileRequest{Profile: got, ExpectedVersion: got.ProfileVersion}
	decode[map[string]any](t, f.request(f.user, "PUT", path, request), 400)
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{answer}','"A goroutine is scheduled by the Go runtime."') WHERE id=?`, got.MaterialID)
	ready := decode[interview.Profile](t, f.request(f.user, "PUT", path, request), 200)
	if ready.Status != "ready" {
		t.Fatal(ready)
	}
	result := decode[interview.ImportResult](t, f.request(f.user, "POST", "/interview/seed/import", map[string]any{"domains": []string{"go"}}), 200)
	if result.Updated != 1 || result.Created != 0 || result.Skipped != 129 {
		t.Fatal("seed metadata not refreshed", result)
	}
	refreshed := decode[interview.Profile](t, f.request(f.user, "GET", path, nil), 200)
	if refreshed.Status != "ready" || refreshed.MaterialID != got.MaterialID {
		t.Fatal("published seed identity/status overwritten")
	}
	var answer string
	if err := f.db.Raw("SELECT values->>'answer' FROM materials WHERE id=?", got.MaterialID).Scan(&answer).Error; err != nil {
		t.Fatal(err)
	}
	if answer != "A goroutine is scheduled by the Go runtime." {
		t.Fatal("manual answer overwritten", answer)
	}
	catalog := decode[interview.Catalog](t, f.request(f.user, "GET", "/interview/concepts", nil), 200)
	c := catalog.Concepts[0]
	c.DisplayName = "Edited name"
	saved := decode[interview.Concept](t, f.request(f.user, "PUT", "/interview/concepts/"+c.Slug, c), 200)
	saved.DisplayName = "Edited twice"
	twice := decode[interview.Concept](t, f.request(f.user, "PUT", "/interview/concepts/"+c.Slug, saved), 200)
	if twice.Version != saved.Version+1 {
		t.Fatal("concept version")
	}
}
