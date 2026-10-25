package materialimport

import (
	"context"
	"encoding/json"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func importDB(t *testing.T) (*gorm.DB, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL for real atomicity, race and profile tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schema := "quick_import_" + uuid.NewString()[:8]
	if err = base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	sqlBase, _ := base.DB()
	t.Cleanup(func() { base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`); sqlBase.Close() })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	scoped := dsn + " search_path=" + schema
	if u.Host != "" {
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		scoped = u.String()
	}
	db, err := gorm.Open(postgres.Open(scoped), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	files, _ := filepath.Glob("../../../../migrations/*.up.sql")
	if len(files) < 26 {
		t.Fatal("missing migrations")
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = db.Exec(string(b)).Error; err != nil {
			t.Fatal(file, err)
		}
	}
	user := uuid.New()
	if err = db.Exec("INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'import','import','unused')", user).Error; err != nil {
		t.Fatal(err)
	}
	return db, user
}
func TestPostgresImportAtomicityReplayRaceAndProfiles(t *testing.T) {
	db, user := importDB(t)
	ctx := context.Background()
	folders := folderservice.NewService(folderpg.NewRepository(db), foldertemplate.NewRegistry(foldertemplate.DefaultTemplates()))
	f, err := folders.Create(ctx, user, "Words", "", "english_words")
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(db)
	req := Request{CommandID: uuid.New(), Items: []Item{item("a", "run"), item("b", "walk")}}
	preview, err := s.Validate(ctx, user, f.ID, req)
	if err != nil || !preview.Valid {
		t.Fatal(preview, err)
	}
	var count int64
	db.Table("materials").Count(&count)
	if count != 0 {
		t.Fatal("validate wrote materials")
	}
	result, err := s.Import(ctx, user, f.ID, req)
	if err != nil || result.Created != 2 {
		t.Fatal(result, err)
	}
	replay, err := s.Import(ctx, user, f.ID, req)
	a, _ := json.Marshal(result)
	b, _ := json.Marshal(replay)
	if err != nil || string(a) != string(b) {
		t.Fatal("unstable replay", err)
	}
	req.Items[0].Values["native"] = ptr("changed")
	if _, err = s.Import(ctx, user, f.ID, req); err != ErrConflict {
		t.Fatal("changed payload accepted", err)
	}
	if _, err = s.Import(ctx, uuid.New(), f.ID, Request{CommandID: uuid.New(), Items: []Item{item("x", "steal")}}); err == nil {
		t.Fatal("ownership bypass")
	}
	invalid := Request{CommandID: uuid.New(), Items: []Item{item("x", "good"), {ClientID: "y", Values: map[string]*string{"foreign": ptr("bad")}}}}
	if _, err = s.Import(ctx, user, f.ID, invalid); err == nil {
		t.Fatal("invalid batch committed")
	}
	db.Table("materials").Count(&count)
	if count != 2 {
		t.Fatal("partial commit", count)
	}
	// Force a failure after CreateMany: the receipt write must roll back materials too.
	if err = db.Exec("ALTER TABLE material_import_commands ADD CONSTRAINT fail_receipt CHECK (request_hash = 'impossible') NOT VALID").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Import(ctx, user, f.ID, Request{CommandID: uuid.New(), Items: []Item{item("z", "rollback")}}); err == nil {
		t.Fatal("expected receipt failure")
	}
	db.Table("materials").Count(&count)
	if count != 2 {
		t.Fatal("receipt failure did not roll back", count)
	}
	db.Exec("ALTER TABLE material_import_commands DROP CONSTRAINT fail_receipt")
	// Two separate commands after identical previews serialize at the user lock.
	r1 := Request{CommandID: uuid.New(), Items: []Item{item("x", "race")}}
	r2 := Request{CommandID: uuid.New(), Items: []Item{item("x", "race")}}
	var wg sync.WaitGroup
	results := make(chan Result, 2)
	errs := make(chan error, 2)
	for _, r := range []Request{r1, r2} {
		wg.Add(1)
		go func(r Request) { defer wg.Done(); out, e := s.Import(ctx, user, f.ID, r); results <- out; errs <- e }(r)
	}
	wg.Wait()
	close(results)
	close(errs)
	created, skipped := 0, 0
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for r := range results {
		created += r.Created
		skipped += r.Skipped
	}
	if created != 1 || skipped != 1 {
		t.Fatal("duplicate race", created, skipped)
	}
	interviewFolder, err := folders.Create(ctx, user, "Questions", "", "interview_questions")
	if err != nil {
		t.Fatal(err)
	}
	questions := Request{CommandID: uuid.New(), Items: []Item{{ClientID: "q", Values: map[string]*string{"question": ptr("What is Go?"), "short_answer": ptr("A programming language.")}}}}
	out, err := s.Import(ctx, user, interviewFolder.ID, questions)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err = db.Table("interview_question_profiles").Select("status").Where("material_id=?", out.Items[0].MaterialID).Scan(&status).Error; err != nil || status != "ready" {
		t.Fatal("profile not ready", status, err)
	}
	if err = db.Exec("ALTER TABLE interview_question_profiles ADD CONSTRAINT fail_profile CHECK (status='impossible') NOT VALID").Error; err != nil {
		t.Fatal(err)
	}
	questions.CommandID = uuid.New()
	questions.Items[0].Values["question"] = ptr("Will roll back?")
	if _, err = s.Import(ctx, user, interviewFolder.ID, questions); err == nil {
		t.Fatal("expected profile failure")
	}
	db.Table("materials").Where("folder_id=?", interviewFolder.ID).Count(&count)
	if count != 1 {
		t.Fatal("profile failure partially committed")
	}
}
