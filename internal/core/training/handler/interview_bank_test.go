package handler_test

import (
	"context"
	"encoding/json"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/seed"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestBankImportPreservesCanonicalIdentityAndProgress(t *testing.T) {
	f := newFixture(t)
	store := interview.NewStore(f.db)
	ctx := context.Background()
	b, err := seed.Load()
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.ImportSeed(ctx, f.user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 368 || len(first.FolderIDs) != 15 || first.ImportRunID == uuid.Nil {
		t.Fatal(first)
	}
	var rows []interview.Profile
	if err = f.db.Table("interview_question_profiles").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	ids := map[string]uuid.UUID{}
	for _, p := range rows {
		ids[*p.SeedKey] = p.MaterialID
	}
	q := b.Questions[1]
	id := ids[q.SeedKey]
	var existing interview.Profile
	if err = f.db.Table("interview_question_profiles").Where("material_id=?", id).Take(&existing).Error; err != nil {
		t.Fatal(err)
	}
	p, err := store.Profile(ctx, f.user, existing.FolderID, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Frequency != q.Frequency || p.InterviewDifficulty != q.Difficulty || p.Specificity != q.Specificity || p.FollowupWeight != q.FollowupWeight || p.LevelMin != q.LevelMin || p.LevelMax != q.LevelMax || p.Subtopic != q.Subtopic {
		t.Fatal("metadata not exact", p, q)
	}
	for _, role := range []struct {
		name  string
		slugs []string
	}{{"answer", q.AnswerConcepts}, {"wrong_fallback", q.WrongFallback}, {"hook", q.ExpectedHooks}, {"prerequisite", q.PrerequisiteConcepts}} {
		for i, slug := range role.slugs {
			found := false
			for _, link := range p.Concepts {
				if link.Role == role.name && link.Slug == slug && link.Ordinal == i {
					found = true
				}
			}
			if !found {
				t.Fatal("relation/order missing", role.name, slug, i)
			}
		}
	}
	var aliases, concepts, memberships int64
	f.db.Table("interview_concepts").Count(&concepts)
	f.db.Table("interview_concept_aliases").Count(&aliases)
	f.db.Table("interview_question_memberships").Count(&memberships)
	if concepts != 1470 || aliases != 2825 || memberships != int64(b.Report.ProfileMemberships) {
		t.Fatal("dictionary or memberships wrong", concepts, aliases, memberships)
	}
	plan := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{existing.FolderID}, HorizonDays: 150}), 201)
	graphDue(t, f, plan, id, true)
	before := graphProgress(t, f, id)
	f.exec(t, `UPDATE materials SET values=values || '{"short_answer":"My summary","answer":"My detailed answer","sources":"My source"}'::jsonb WHERE id=?`, id)
	p.Status = "ready"
	if _, err = store.SaveProfile(ctx, f.user, p.FolderID, id, interview.ProfileRequest{Profile: p, ExpectedVersion: p.ProfileVersion}); err != nil {
		t.Fatal(err)
	}
	again, err := store.ImportSeed(ctx, f.user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Created != 0 || again.Updated != 0 || again.Skipped != 368 || graphProgress(t, f, id) != before {
		t.Fatal("reimport duplicated cards or changed SRS", again)
	}
	var values struct{ Values []byte }
	if err = f.db.Raw("SELECT values FROM materials WHERE id=?", id).Scan(&values).Error; err != nil {
		t.Fatal(err)
	}
	var content map[string]string
	if err = json.Unmarshal(values.Values, &content); err != nil {
		t.Fatal(err)
	}
	if content["answer"] != "My detailed answer" || content["short_answer"] != "My summary" || content["sources"] != "My source" || content["full_answer"] != "" {
		t.Fatal("content overwritten or duplicated", content)
	}
	var n int64
	f.db.Table("interview_question_profiles").Where("owner_id=?", f.user).Count(&n)
	if n != 368 {
		t.Fatal("canonical duplicates", n)
	}
	// Failure halfway through the operation must roll back both metadata and audit row.
	f.exec(t, `CREATE FUNCTION reject_import_run() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test rollback'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_import BEFORE INSERT ON interview_bank_import_runs FOR EACH ROW EXECUTE FUNCTION reject_import_run()`)
	prior, _ := store.Profile(ctx, f.user, p.FolderID, id)
	if _, err = store.ImportSeed(ctx, f.user, []string{"go"}); err == nil {
		t.Fatal("expected failed import")
	}
	after, _ := store.Profile(ctx, f.user, p.FolderID, id)
	if !reflect.DeepEqual(prior, after) {
		t.Fatal("partial import committed")
	}
}

func TestBankTestingNextRouteUndoResumeWithoutSRS(t *testing.T) {
	f, p, ids := graphFixture(t)
	f.exec(t, `UPDATE interview_question_profiles SET status='draft',root_weight=10`)
	f.exec(t, `UPDATE materials SET values=values || '{"short_answer":".","answer":".","sources":"."}'::jsonb`)
	config := graph.DefaultConfig()
	config.IncludeDraft = true
	config.QuestionLimit = 3
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/interview-graph", service.StartGraphRequest{CommandID: uuid.New(), Config: &config}), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	if v.Current == nil || v.Current.ProgressVersion != 0 || !v.Current.InterviewGraph.AnswerIncomplete || !v.Current.InterviewGraph.BankVerification || len(v.Current.Answer) != 3 {
		t.Fatal("draft not displayed", v)
	}
	for _, field := range v.Current.Answer {
		if field.Value != "." {
			t.Fatal(field)
		}
	}
	initial := v.Graph.State
	a := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, "next_route")), 200)
	if a.Event.ReviewCredit || a.Event.EventMode != "graph_navigation" || a.Session.Summary.Correct != 0 || a.Session.Summary.Wrong != 0 || a.Session.Graph.Statistics.InterviewProbes != 0 || a.Session.Graph.State.CurrentRoot == initial.CurrentRoot {
		t.Fatal("navigation graded or failed to switch", a)
	}
	u := decode[model.SessionView](t, f.request(f.user, "POST", path+"/undo", service.GraphUndoRequest{CommandID: uuid.New(), EventID: a.Event.ID}), 200)
	if u.Current.MaterialID != v.Current.MaterialID || u.Graph.State.RandomIndex != initial.RandomIndex || !reflect.DeepEqual(u.Graph.State.AskedMaterialIDs, initial.AskedMaterialIDs) {
		t.Fatal("navigation undo did not restore state")
	}
	f.restart()
	resumed := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if !reflect.DeepEqual(u.Current, resumed.Current) {
		t.Fatal("resume changed snapshot")
	}
	for _, action := range []algorithm.Action{algorithm.Correct, algorithm.Wrong, "next_route"} {
		if resumed.Current == nil {
			break
		}
		a = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(resumed.Current, action)), 200)
		resumed = a.Session
	}
	for _, id := range ids {
		if graphProgress(t, f, id) != "" {
			t.Fatal("bank testing created SRS", id)
		}
	}
	var credited int64
	f.db.Table("training_events").Where("review_credit").Count(&credited)
	if credited != 0 {
		t.Fatal("draft received SRS credit")
	}
}
