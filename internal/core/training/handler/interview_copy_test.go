package handler_test

import (
	"context"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func copyFixture(t *testing.T) (*fixture, uuid.UUID, uuid.UUID) {
	t.Helper()
	f := bankHTTPFixture(t)
	imported := importDomain(t, f, f.user, "algorithms")
	f.folder = imported.FolderIDs[0]
	var row struct{ ID uuid.UUID }
	if err := f.db.Table("materials").Where("folder_id=?", f.folder).Order("id").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE materials SET deleted_at=? WHERE folder_id=? AND id<>?`, f.now, f.folder, row.ID)
	target := uuid.New()
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config) SELECT ?,owner_id,'Copy target',template_key,config,training_config FROM folders WHERE id=?`, target, f.folder)
	return f, row.ID, target
}
func copyPath(f *fixture, source uuid.UUID) string {
	return fmt.Sprintf("/folders/%s/interview/questions/%s/copy", f.folder, source)
}
func countTarget(t *testing.T, f *fixture, target uuid.UUID) int64 {
	t.Helper()
	var n int64
	if err := f.db.Table("materials").Where("folder_id=?", target).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func TestInterviewCopyContentProfileAndNoProgress(t *testing.T) {
	f, source, target := copyFixture(t)
	f.exec(t, `UPDATE materials SET difficulty='hard' WHERE id=?`, source)
	v := f.combined(t, service.CombinedSource{FolderID: f.folder})
	if v.Current == nil {
		t.Fatal("missing source presentation")
	}
	decode[model.ActionResult](t, f.request(f.user, "POST", "/training/sessions/"+v.Current.SessionID.String()+"/actions", actionFor(v.Current.Presentation, algorithm.Correct)), 200)
	cfg := graph.DefaultConfig()
	cfg.IncludeDraft = true
	decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", service.StartGraphRequest{CommandID: uuid.New(), Sources: []model.SessionSource{{FolderID: f.folder}}, Config: &cfg}), 200)
	snapshot := func() string {
		var value string
		err := f.db.Raw(`SELECT jsonb_build_object('progress',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.material_id) FROM user_material_progress p),'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM training_events e),'mock',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.session_id) FROM interview_graph_session_state s))::text`).Scan(&value).Error
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := snapshot()
	copied := decode[interview.CopyResult](t, f.request(f.user, "POST", copyPath(f, source), interview.CopyRequest{TargetFolderID: target}), 201)
	if copied.MaterialID == source || copied.FolderID != target {
		t.Fatal(copied)
	}
	repo := materialpg.NewRepository(f.db)
	original, err := repo.GetByID(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := repo.GetByID(context.Background(), copied.MaterialID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original.Values, clone.Values) || !reflect.DeepEqual(original.Metadata, clone.Metadata) || original.Difficulty != clone.Difficulty {
		t.Fatal("content/metadata/difficulty lost")
	}
	svc := interview.NewService(interview.NewStore(f.db))
	p, err := svc.Profile(context.Background(), f.user, f.folder, source)
	if err != nil {
		t.Fatal(err)
	}
	q, err := svc.Profile(context.Background(), f.user, target, copied.MaterialID)
	if err != nil {
		t.Fatal(err)
	}
	if q.SeedKey != nil || q.ProfileVersion != 1 || !q.CreatedAt.After(p.CreatedAt) {
		t.Fatal("copy inherited identity/revision")
	}
	p.MaterialID, p.FolderID, p.SeedKey, p.ProfileVersion, p.CreatedAt, p.UpdatedAt = q.MaterialID, q.FolderID, nil, q.ProfileVersion, q.CreatedAt, q.UpdatedAt
	if !reflect.DeepEqual(p, q) {
		t.Fatalf("profile metadata lost: %+v versus %+v", p, q)
	}
	if snapshot() != before {
		t.Fatal("copy mutated progress/events/mock state")
	}
	var n int64
	if err := f.db.Table("user_material_progress").Where("material_id=?", copied.MaterialID).Count(&n).Error; err != nil || n != 0 {
		t.Fatal("progress copied", err)
	}
}
func TestInterviewCopyDuplicateConcurrentAndForced(t *testing.T) {
	f, source, target := copyFixture(t)
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{question}',to_jsonb(?::text)) WHERE id=?`, "  Question\n  TEXT?  ", source)
	var results [2]*httptest.ResponseRecorder
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = f.request(f.user, "POST", copyPath(f, source), interview.CopyRequest{TargetFolderID: target})
		}(i)
	}
	wg.Wait()
	if results[0].Code+results[1].Code != 201+409 || countTarget(t, f, target) != 1 {
		t.Fatal("concurrent duplicate", results[0].Code, results[1].Code)
	}
	var existing uuid.UUID
	for _, result := range results {
		if result.Code == 409 {
			duplicate := decode[map[string]string](t, result, 409)
			existing = uuid.MustParse(duplicate["existing_material_id"])
		}
	}
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{question}',to_jsonb(?::text)) WHERE id=?`, "question text?", existing)
	decode[map[string]string](t, f.request(f.user, "POST", copyPath(f, source), interview.CopyRequest{TargetFolderID: target}), 409)
	decode[interview.CopyResult](t, f.request(f.user, "POST", copyPath(f, source), interview.CopyRequest{TargetFolderID: target, AllowDuplicate: true}), 201)
	if countTarget(t, f, target) != 2 {
		t.Fatal("explicit duplicate not created")
	}
}
func TestInterviewCopyPermissionsValidationAndAtomicRollback(t *testing.T) {
	f, source, target := copyFixture(t)
	path := copyPath(f, source)
	decode[map[string]any](t, f.request(uuid.Nil, "POST", path, interview.CopyRequest{TargetFolderID: target}), 401)
	other := uuid.New()
	f.exec(t, `INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')`, other)
	decode[map[string]any](t, f.request(other, "POST", path, interview.CopyRequest{TargetFolderID: target}), 404)
	f.exec(t, `UPDATE folders SET owner_id=? WHERE id=?`, other, target)
	decode[map[string]any](t, f.request(f.user, "POST", path, interview.CopyRequest{TargetFolderID: target}), 404)
	f.exec(t, `UPDATE folders SET owner_id=?,template_key='english_words' WHERE id=?`, f.user, target)
	decode[map[string]any](t, f.request(f.user, "POST", path, interview.CopyRequest{TargetFolderID: target}), 400)
	f.exec(t, `UPDATE folders SET template_key='interview_questions' WHERE id=?`, target)
	decode[map[string]any](t, f.request(f.user, "POST", path, map[string]any{"target_folder_id": target, "progress": map[string]int{"stage": 7}}), 400)
	decode[map[string]any](t, f.request(f.user, "POST", path, interview.CopyRequest{TargetFolderID: f.folder}), 400)
	// Inject a failure after content insertion to verify the outer transaction.
	f.exec(t, `CREATE FUNCTION reject_copy_profile() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'copy fixture failure'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_copy_profile BEFORE INSERT ON interview_question_profiles FOR EACH ROW EXECUTE FUNCTION reject_copy_profile()`)
	decode[map[string]any](t, f.request(f.user, "POST", path, interview.CopyRequest{TargetFolderID: target}), 500)
	if countTarget(t, f, target) != 0 {
		t.Fatal("partial material remained")
	}
	f.exec(t, `DROP TRIGGER reject_copy_profile ON interview_question_profiles`)
	f.exec(t, `UPDATE folders SET deleted_at=? WHERE id=?`, f.now, target)
	decode[map[string]any](t, f.request(f.user, "POST", path, interview.CopyRequest{TargetFolderID: target}), 404)
}
