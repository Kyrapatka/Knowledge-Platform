package handler_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	trainingpg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
)

func importedInterview(t *testing.T) (*fixture, uuid.UUID) {
	t.Helper()
	f := newFixture(t)
	result, err := interview.NewStore(f.db).ImportSeed(context.Background(), f.user, []string{"go"})
	if err != nil || result.Created == 0 || len(result.FolderIDs) != 1 {
		t.Fatalf("bank import: %+v, %v", result, err)
	}
	f.folder = result.FolderIDs[0]
	var row struct{ MaterialID uuid.UUID }
	if err := f.db.Table("interview_question_profiles").Select("material_id").Where("folder_id=? AND seed_key='GO001' AND status='draft'", f.folder).Take(&row).Error; err != nil || row.MaterialID == uuid.Nil {
		t.Fatal("missing imported draft", err)
	}
	return f, row.MaterialID
}

func TestBankDraftNormalTrainingAndIndependentMock(t *testing.T) {
	f, id := importedInterview(t)
	// Isolate one real imported question, preserving its draft profile/content.
	f.exec(t, `UPDATE materials SET deleted_at=? WHERE folder_id=? AND id<>?`, f.now, f.folder, id)
	f.exec(t, `UPDATE materials SET difficulty='easy' WHERE id=?`, id)
	var count int64
	if err := f.db.Table("user_material_progress").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("import eagerly created progress", err)
	}
	cfg := graph.DefaultConfig()
	cfg.IncludeDraft = true
	mock := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}}, Config: &cfg}), 200)
	if mock.Current == nil || mock.Current.MaterialID != id {
		t.Fatal("mock did not select imported question")
	}
	graphSnapshot := func() string {
		var out string
		if err := f.db.Raw(`SELECT to_jsonb(s)::text FROM interview_graph_session_state s WHERE session_id=?`, mock.Session.ID).Scan(&out).Error; err != nil {
			t.Fatal(err)
		}
		return out
	}
	beforeGraph := graphSnapshot()
	// Combined is the normal UI entry point, also for previously imported folders.
	f.restart()
	v := f.combined(t, service.CombinedSource{FolderID: f.folder})
	if v.Current == nil || v.Current.Presentation.MaterialID != id || v.Current.Presentation.InterviewGraph != nil {
		t.Fatalf("draft missing from normal recall: %+v", v)
	}
	card := v.Current.Presentation
	if len(card.Question) == 0 || len(card.Answer) < 2 || card.Answer[0].Key != "short_answer" || card.Answer[1].Key != "answer" {
		t.Fatalf("short/full answer lost: %+v", card)
	}
	path := "/training/sessions/" + v.Current.SessionID.String()
	result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(card, algorithm.Correct)), 200)
	var progress model.UserMaterialProgress
	load := func() {
		if err := f.db.Table("user_material_progress").Where("material_id=? AND track='long_term'", id).Take(&progress).Error; err != nil {
			t.Fatal(err)
		}
	}
	load()
	if progress.Stage != 2 || progress.CorrectCount != 1 || progress.NextReviewAt() == nil || !progress.NextReviewAt().After(f.now) || result.Session.Current != nil {
		t.Fatalf("normal scheduling failed: %+v", progress)
	}
	if beforeGraph != graphSnapshot() {
		t.Fatal("normal answer mutated mock state")
	}
	v = f.combinedCurrent(t, v)
	if v.Current != nil || v.EmptyReason != "not_due" {
		t.Fatal("question returned before due", v)
	}
	beforeProgress := graphProgress(t, f, id)
	mockPath := "/training/sessions/" + mock.Session.ID.String()
	decode[model.ActionResult](t, f.request(f.user, "POST", mockPath+"/actions", actionFor(mock.Current, algorithm.Correct)), 200)
	if beforeProgress != graphProgress(t, f, id) {
		t.Fatal("mock answer changed normal progress")
	}
	decode[model.SessionView](t, f.request(f.user, "POST", mockPath+"/finish", nil), 200)
	mock = decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}}, Config: &cfg}), 200)
	if mock.Current == nil || mock.Current.MaterialID != id || mock.Current.InterviewGraph.ReviewCredit {
		t.Fatal("SRS not-due hid question from mock")
	}
	beforeGraph = graphSnapshot()
	f.now = *progress.NextReviewAt()
	v = f.combinedCurrent(t, v)
	if v.Current == nil || v.Current.Presentation.MaterialID != id {
		t.Fatal("due question missing")
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+v.Current.SessionID.String()+"/actions", actionFor(v.Current.Presentation, algorithm.Wrong)), 200)
	load()
	if progress.Stage != 1 || progress.WrongCount != 1 || !progress.RehabActive {
		t.Fatalf("rehab not applied: %+v", progress)
	}
	if beforeGraph != graphSnapshot() {
		t.Fatal("normal wrong mutated mock state")
	}
}

func TestBankNormalMixedSourcesAndCram(t *testing.T) {
	for _, cram := range []bool{false, true} {
		t.Run(map[bool]string{false: "long_term", true: "cram"}[cram], func(t *testing.T) {
			f, _ := importedInterview(t)
			interviewFolder := f.folder
			var folder struct{ ID uuid.UUID }
			if err := f.db.Table("folders").Select("id").Where("owner_id=? AND template_key='custom_origin'", f.user).Take(&folder).Error; err != nil {
				t.Fatal(err)
			}
			f.folder = folder.ID
			f.exec(t, `UPDATE folders SET template_key='english_words' WHERE id=?`, f.folder)
			f.material(t, "English")
			englishFolder := f.folder
			formulaFolder := uuid.New()
			f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'Formula','formulas',config,'{"default_algorithm_key":"formula_adaptive","pool_size":2}' FROM folders WHERE id=?`, formulaFolder, englishFolder)
			f.folder = formulaFolder
			f.exercise(t, f.material(t, "Velocity"), "100 km in 2 hours")
			source := service.CombinedSource{FolderID: interviewFolder}
			if cram {
				key, days := "interview_cram", 5
				source.AlgorithmKey = &key
				source.HorizonDays = &days
			}
			v := f.combined(t, service.CombinedSource{FolderID: englishFolder}, service.CombinedSource{FolderID: formulaFolder}, source)
			if len(v.Sessions) != 3 || v.Current == nil {
				t.Fatal("mixed sources unavailable")
			}
			seen := map[uuid.UUID]bool{}
			for i := 0; i < 6 && v.Current != nil; i++ {
				seen[v.Current.Presentation.FolderID] = true
				decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+v.Current.SessionID.String()+"/actions", actionFor(v.Current.Presentation, algorithm.Correct)), 200)
				v = f.combinedCurrent(t, v)
			}
			if !seen[interviewFolder] || !seen[englishFolder] || !seen[formulaFolder] {
				t.Fatal("mixed queue excluded interview", seen)
			}
		})
	}
}

func TestLegacyInterviewShortAnswerIsTrainableWithoutReimport(t *testing.T) {
	f, id := importedInterview(t)
	// Old stored material without progress/profile, only a short answer. The
	// neighboring blank card must still be rejected by normal content eligibility.
	f.exec(t, `UPDATE materials SET values='{"question":""}' WHERE folder_id=? AND id<>?`, f.folder, id)
	f.exec(t, `UPDATE materials SET values='{"question":"Question","short_answer":"Short only"}',created_at=? WHERE id=?`, f.now.AddDate(-1, 0, 0), id)
	f.exec(t, `DELETE FROM interview_question_profiles WHERE material_id=?`, id)
	f.restart()
	v := f.combined(t, service.CombinedSource{FolderID: f.folder})
	if v.Current == nil || v.Current.Presentation.MaterialID != id || len(v.Current.Presentation.Answer) != 1 || v.Current.Presentation.Answer[0].Value != "Short only" {
		t.Fatalf("legacy short answer unavailable: %+v", v)
	}
	var count int64
	if err := f.db.Table("user_material_progress").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("blank cards received progress", count, err)
	}
}

func TestNormalInterviewAnswerAnalytics(t *testing.T) {
	f, id := importedInterview(t)
	f.exec(t, `UPDATE materials SET deleted_at=? WHERE folder_id=? AND id<>?`, f.now, f.folder, id)
	s := service.NewServiceWithClock(trainingpg.NewRuntimeStore(f.db), func() time.Time { return f.now })
	var events []analytics.Event
	s.SetPublisher(publishFunc(func(_ context.Context, e analytics.Event) { events = append(events, e) }))
	p, err := s.CreatePlan(context.Background(), f.user, service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, HorizonDays: 150})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.StartSession(context.Background(), f.user, p.ID)
	if err != nil || v.Current == nil {
		t.Fatal("no normal question", err)
	}
	events = nil
	req := actionFor(v.Current, algorithm.Correct)
	out, err := s.Act(context.Background(), f.user, v.Session.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventName != analytics.TrainingAnswered || events[0].Template != "interview_questions" || events[0].Mode == "mock" {
		t.Fatalf("wrong analytics: %+v", events)
	}
	replay, err := s.Act(context.Background(), f.user, v.Session.ID, req)
	if err != nil || !reflect.DeepEqual(out, replay) || len(events) != 1 {
		t.Fatal("answer replay duplicated effect", err)
	}
}
