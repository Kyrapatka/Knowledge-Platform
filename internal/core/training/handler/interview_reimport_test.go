package handler_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/core/dashboard"
	folderhandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/handler"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
)

func TestSeedMissingMaterialIsRecreated(t *testing.T) {
	for _, legacyOrphan := range []bool{false, true} {
		t.Run(map[bool]string{false: "hard delete cascades profile", true: "legacy orphan profile"}[legacyOrphan], func(t *testing.T) {
			f := bankHTTPFixture(t)
			first := importDomain(t, f, f.user, "algorithms")
			ids := assertBankCount(t, f, f.user, first.FolderIDs[0], "algorithms", 8)
			var id uuid.UUID
			var key string
			for k, v := range ids {
				key, id = k, v
				break
			}
			// Only the legacy fixture omits the FK. The actual schema cascades a
			// hard delete; both states must be reconciled without an orphan.
			if legacyOrphan {
				f.exec(t, `ALTER TABLE interview_question_profiles DROP CONSTRAINT interview_question_profiles_material_id_fkey`)
			}
			f.exec(t, `DELETE FROM materials WHERE id=?`, id)
			r := importDomain(t, f, f.user, "algorithms")
			if r.Created != 1 || r.Updated != 0 || r.Skipped != 7 {
				t.Fatal(r)
			}
			after := assertBankCount(t, f, f.user, r.FolderIDs[0], "algorithms", 8)
			if legacyOrphan && after[key] != id {
				t.Fatal("legacy profile identity changed")
			}
		})
	}
}

func TestSeedConcurrentRestoreDoesNotDuplicate(t *testing.T) {
	f := bankHTTPFixture(t)
	first := importDomain(t, f, f.user, "algorithms")
	if err := folderpg.NewRepository(f.db).Delete(context.Background(), first.FolderIDs[0]); err != nil {
		t.Fatal(err)
	}
	store := interview.NewStore(f.db)
	var results [2]interview.ImportResult
	var failures [2]error
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], failures[i] = store.ImportSeed(context.Background(), f.user, []string{"algorithms"})
		}(i)
	}
	wg.Wait()
	for _, err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if results[0].Created+results[1].Created != 8 || results[0].Skipped+results[1].Skipped != 8 || results[0].FolderIDs[0] != results[1].FolderIDs[0] {
		t.Fatal("concurrent restore duplicated state", results)
	}
	assertBankCount(t, f, f.user, results[0].FolderIDs[0], "algorithms", 8)
}

func bankHTTPFixture(t *testing.T) *fixture {
	f := newFixture(t)
	api := f.router.Group("/api/v1")
	api.Use(auth.AuthMiddleware(testParser{}))
	interview.NewHandler(f.db).RegisterRoutes(api)
	dashboard.NewHandler(f.db).RegisterRoutes(api)
	folders := folderservice.NewService(folderpg.NewRepository(f.db), foldertemplate.NewRegistry(foldertemplate.DefaultTemplates()))
	api.DELETE("/folders/:folderID", folderhandler.NewHandler(folders).Delete)
	return f
}

func importDomain(t *testing.T, f *fixture, user uuid.UUID, domain string) interview.ImportResult {
	t.Helper()
	return decode[interview.ImportResult](t, f.request(user, "POST", "/interview/seed/import", map[string]any{"domains": []string{domain}}), 200)
}

func assertBankCount(t *testing.T, f *fixture, user, folder uuid.UUID, domain string, n int) map[string]uuid.UUID {
	t.Helper()
	library := decode[dashboard.LibraryView](t, f.request(user, "GET", "/library", nil), 200)
	found := false
	for _, x := range library.Folders {
		if x.ID == folder {
			found = true
			if x.MaterialCount != int64(n) {
				t.Fatalf("library count %d, expected %d", x.MaterialCount, n)
			}
		}
	}
	if !found {
		t.Fatal("imported folder missing from library")
	}
	var rows []struct {
		MaterialID uuid.UUID
		SeedKey    string
	}
	if err := f.db.Raw(`SELECT q.material_id,q.seed_key FROM interview_question_profiles q JOIN materials m ON m.id=q.material_id JOIN folders f ON f.id=m.folder_id WHERE q.owner_id=? AND q.domain=? AND f.owner_id=? AND f.id=? AND f.deleted_at IS NULL AND m.deleted_at IS NULL AND q.folder_id=f.id`, user, domain, user, folder).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	ids := map[string]uuid.UUID{}
	for _, r := range rows {
		if r.SeedKey == "" || ids[r.SeedKey] != uuid.Nil {
			t.Fatal("missing/duplicate seed key", r)
		}
		ids[r.SeedKey] = r.MaterialID
	}
	if len(ids) != n {
		t.Fatalf("active profiles %d, expected %d", len(ids), n)
	}
	var total int64
	if err := f.db.Table("interview_question_profiles").Where("owner_id=? AND domain=?", user, domain).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != int64(n) {
		t.Fatal("duplicate profiles", total)
	}
	return ids
}

func TestSeedDeleteFolderAndReimport(t *testing.T) {
	for domain, n := range map[string]int{"algorithms": 8, "go": 130} {
		t.Run(domain, func(t *testing.T) {
			f := bankHTTPFixture(t)
			first := importDomain(t, f, f.user, domain)
			if first.Created != n || first.Updated != 0 || first.Skipped != 0 || len(first.FolderIDs) != 1 {
				t.Fatal(first)
			}
			oldFolder := first.FolderIDs[0]
			ids := assertBankCount(t, f, f.user, oldFolder, domain, n)
			again := importDomain(t, f, f.user, domain)
			if again.Created != 0 || again.Updated != 0 || again.Skipped != n || again.FolderIDs[0] != oldFolder {
				t.Fatal("non-idempotent", again)
			}
			if !reflect.DeepEqual(ids, assertBankCount(t, f, f.user, oldFolder, domain, n)) {
				t.Fatal("active IDs changed")
			}
			var material uuid.UUID
			for _, id := range ids {
				material = id
				break
			}
			p := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{oldFolder}, HorizonDays: 150}), 201)
			graphDue(t, f, p, material, true)
			progress := graphProgress(t, f, material)
			f.exec(t, `UPDATE materials SET values=values || '{"short_answer":"My edited answer","answer":"My notes","sources":"My reference"}'::jsonb WHERE id=?`, material)
			deleted := f.request(f.user, "DELETE", "/folders/"+oldFolder.String(), nil)
			if deleted.Code != 204 {
				t.Fatal(deleted.Code, deleted.Body.String())
			}
			var active int64
			if err := f.db.Table("materials").Where("folder_id=? AND deleted_at IS NULL", oldFolder).Count(&active).Error; err != nil {
				t.Fatal(err)
			}
			if active != 0 {
				t.Fatal("folder deletion left active materials")
			}
			restored := importDomain(t, f, f.user, domain)
			if restored.Created != n || restored.Updated != 0 || restored.Skipped != 0 || restored.FolderIDs[0] == oldFolder {
				t.Fatal("did not restore", restored)
			}
			if !reflect.DeepEqual(ids, assertBankCount(t, f, f.user, restored.FolderIDs[0], domain, n)) {
				t.Fatal("restoration changed material IDs")
			}
			if graphProgress(t, f, material) != progress {
				t.Fatal("restoration changed SRS")
			}
			var answer string
			if err := f.db.Raw(`SELECT values->>'short_answer' FROM materials WHERE id=?`, material).Scan(&answer).Error; err != nil {
				t.Fatal(err)
			}
			if answer != "My edited answer" {
				t.Fatal("restoration overwrote manual content")
			}
			var wrongFolder int64
			if err := f.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Where("f.owner_id=? AND m.deleted_at IS NULL AND f.deleted_at IS NOT NULL", f.user).Count(&wrongFolder).Error; err != nil {
				t.Fatal(err)
			}
			if wrongFolder != 0 {
				t.Fatal("active material attached to deleted folder")
			}
			last := importDomain(t, f, f.user, domain)
			if last.Created != 0 || last.Skipped != n {
				t.Fatal("repeated restore duplicates", last)
			}
		})
	}
}

func TestSeedPartialDeletionIsolationAndMetadataUpdates(t *testing.T) {
	f := bankHTTPFixture(t)
	other := uuid.New()
	f.exec(t, `INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'otherbank','otherbank','unused')`, other)
	first := importDomain(t, f, f.user, "algorithms")
	second := importDomain(t, f, other, "algorithms")
	ids := assertBankCount(t, f, f.user, first.FolderIDs[0], "algorithms", 8)
	otherIDs := assertBankCount(t, f, other, second.FolderIDs[0], "algorithms", 8)
	var before string
	if err := f.db.Raw(`SELECT jsonb_agg(to_jsonb(q) ORDER BY material_id)::text FROM interview_question_profiles q WHERE owner_id=?`, other).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	for _, v := range ids {
		id = v
		break
	}
	f.exec(t, `UPDATE materials SET deleted_at=NOW() WHERE id=?`, id)
	r := importDomain(t, f, f.user, "algorithms")
	if r.Created != 1 || r.Updated != 0 || r.Skipped != 7 || r.FolderIDs[0] != first.FolderIDs[0] {
		t.Fatal(r)
	}
	assertBankCount(t, f, f.user, r.FolderIDs[0], "algorithms", 8)
	f.exec(t, `UPDATE interview_question_profiles SET frequency=CASE WHEN frequency=1 THEN 2 ELSE 1 END WHERE material_id=?`, id)
	r = importDomain(t, f, f.user, "algorithms")
	if r.Created != 0 || r.Updated != 1 || r.Skipped != 7 {
		t.Fatal("seed metadata not reconciled", r)
	}
	if x := f.request(other, "DELETE", "/folders/"+first.FolderIDs[0].String(), nil); x.Code != 404 {
		t.Fatal("cross-user deletion", x.Code)
	}
	if x := f.request(f.user, "DELETE", "/folders/"+first.FolderIDs[0].String(), nil); x.Code != 204 {
		t.Fatal(x.Code)
	}
	importDomain(t, f, f.user, "algorithms")
	if !reflect.DeepEqual(otherIDs, assertBankCount(t, f, other, second.FolderIDs[0], "algorithms", 8)) {
		t.Fatal("other user's materials changed")
	}
	var after string
	if err := f.db.Raw(`SELECT jsonb_agg(to_jsonb(q) ORDER BY material_id)::text FROM interview_question_profiles q WHERE owner_id=?`, other).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("other user's profiles changed")
	}
}

func TestSeedRestoreRollsBackAtomically(t *testing.T) {
	f := bankHTTPFixture(t)
	first := importDomain(t, f, f.user, "algorithms")
	old := first.FolderIDs[0]
	if err := folderpg.NewRepository(f.db).Delete(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `CREATE FUNCTION reject_reimport() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test restoration rollback'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_restore BEFORE UPDATE ON interview_question_profiles FOR EACH ROW EXECUTE FUNCTION reject_reimport()`)
	decode[map[string]any](t, f.request(f.user, "POST", "/interview/seed/import", map[string]any{"domains": []string{"algorithms"}}), 500)
	var count int64
	if err := f.db.Table("materials m").Joins("JOIN interview_question_profiles q ON q.material_id=m.id").Where("q.owner_id=? AND m.deleted_at IS NULL", f.user).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial restoration committed")
	}
	var row struct{ FolderID uuid.UUID }
	if err := f.db.Table("interview_seed_folders").Select("folder_id").Where("user_id=? AND domain='algorithms'", f.user).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.FolderID != old {
		t.Fatal("folder mapping partially changed")
	}
	f.exec(t, `DROP TRIGGER reject_restore ON interview_question_profiles`)
	r := importDomain(t, f, f.user, "algorithms")
	if r.Created != 8 {
		t.Fatal(r)
	}
}
