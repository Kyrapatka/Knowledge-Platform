package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	trainingpg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testParser struct{}

func (testParser) ParseAccessToken(_ context.Context, value string) (token.AccessTokenClaims, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return token.AccessTokenClaims{}, token.ErrInvalidAccessToken
	}
	return token.AccessTokenClaims{UserID: id}, nil
}

type fixture struct {
	db     *gorm.DB
	router *gin.Engine
	user   uuid.UUID
	folder uuid.UUID
	now    time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL for isolated PostgreSQL/HTTP tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	baseSQL, err := base.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := "training_http_" + uuid.New().String()[:8]
	if err = base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	// Only this uniquely-created schema is removed. Never truncate application tables.
	t.Cleanup(func() {
		if err := base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Error(err)
		}
		baseSQL.Close()
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	scopedDSN := dsn + " search_path=" + schema
	if u.Host != "" {
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		scopedDSN = u.String()
	}
	db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	files, err := filepath.Glob("../../../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("missing migrations", err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = db.Exec(string(b)).Error; err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	f := &fixture{db: db, user: uuid.New(), folder: uuid.New(), now: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)}
	f.exec(t, "INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'training','training','unused')", f.user)
	config := `{"schema":{"fields":[{"key":"front","label":"Question","active":true},{"key":"back","label":"Answer","active":true}]},"metadata_schema":{"fields":[]},"card":{"question_fields":["front"],"answer_fields":["back"]}}`
	f.exec(t, "INSERT INTO folders(id,owner_id,title,template_key,config,training_config) VALUES(?,?,'Test','custom_origin',?,'{\"default_algorithm_key\":\"english_basic\",\"pool_size\":2}')", f.folder, f.user, config)
	f.restart()
	return f
}

func (f *fixture) restart() {
	gin.SetMode(gin.TestMode)
	f.router = gin.New()
	f.router.Use(gin.Recovery())
	api := f.router.Group("/api/v1")
	api.Use(auth.AuthMiddleware(testParser{}))
	s := service.NewServiceWithClock(trainingpg.NewRuntimeStore(f.db), func() time.Time { return f.now })
	handler.NewHandler(s).RegisterRoutes(api)
}
func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if err := f.db.Exec(q, args...).Error; err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) material(t *testing.T, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	b, _ := json.Marshal(map[string]string{"front": label, "back": "answer " + label})
	f.exec(t, `INSERT INTO materials(id,folder_id,values,created_at) VALUES(?,?,?,?)`, id, f.folder, string(b), f.now)
	return id
}
func (f *fixture) request(user uuid.UUID, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if user != uuid.Nil {
		r.Header.Set("Authorization", "Bearer "+user.String())
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	return w
}
func decode[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if w.Code != status {
		t.Fatalf("HTTP %d, wanted %d: %s", w.Code, status, w.Body.String())
	}
	var result T
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func (f *fixture) plan(t *testing.T) model.TrainingPlan {
	t.Helper()
	return decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}}), 201)
}
func actionFor(p *model.Presentation, a algorithm.Action) service.ActionRequest {
	return service.ActionRequest{CommandID: uuid.New(), PresentationID: p.ID, ExpectedVersion: p.ProgressVersion, Action: a}
}

func TestEnglishHTTPFlow(t *testing.T) {
	f := newFixture(t)
	f.material(t, "A")
	f.material(t, "B")
	plan := f.plan(t)
	if plan.Config.PoolSize != 2 || plan.AlgorithmKey != "english_basic" {
		t.Fatal("defaults lost")
	}
	startPath := "/training/plans/" + plan.ID.String() + "/sessions"
	view := decode[model.SessionView](t, f.request(f.user, "POST", startPath, nil), 200)
	if view.PoolSize != 2 || view.Current == nil {
		t.Fatal("pool not populated")
	}
	sessionPath := "/training/sessions/" + view.Session.ID.String()
	again := decode[model.SessionView](t, f.request(f.user, "POST", startPath, nil), 200)
	if again.Session.ID != view.Session.ID || again.Current.ID != view.Current.ID {
		t.Fatal("start did not resume")
	}
	// Ownership and duplicate active sources are checked on the server.
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}}), 409)
	decode[map[string]any](t, f.request(uuid.New(), "GET", sessionPath, nil), 404)
	decode[map[string]any](t, f.request(uuid.Nil, "GET", sessionPath, nil), 401)
	// Folder defaults and card edits cannot mutate the plan's snapshot.
	decode[service.FolderDefaults](t, f.request(f.user, "PATCH", "/folders/"+f.folder.String()+"/training-config", service.FolderDefaults{
		Version: 1, Config: folderconfig.TrainingConfig{DefaultAlgorithmKey: "english_adaptive", PoolSize: 8}}), 200)
	f.exec(t, `UPDATE folders SET config=jsonb_set(config,'{card,question_fields}','["back"]') WHERE id=?`, f.folder)
	f.exec(t, `UPDATE materials SET values=jsonb_set(values,'{front}','"edited"'),difficulty='hard' WHERE id=?`, view.Current.MaterialID)
	f.restart()
	restored := decode[model.SessionView](t, f.request(f.user, "GET", sessionPath, nil), 200)
	if !reflect.DeepEqual(view.Current, restored.Current) {
		t.Fatal("displayed content changed across restart/edit")
	}
	request := actionFor(view.Current, algorithm.Correct)
	first := decode[model.ActionResult](t, f.request(f.user, "POST", sessionPath+"/actions", request), 200)
	retry := decode[model.ActionResult](t, f.request(f.user, "POST", sessionPath+"/actions", request), 200)
	if !reflect.DeepEqual(first, retry) {
		t.Fatal("idempotent response changed")
	}
	request.Action = algorithm.Wrong
	decode[map[string]any](t, f.request(f.user, "POST", sessionPath+"/actions", request), 409)
	stale := actionFor(view.Current, algorithm.Correct)
	decode[map[string]any](t, f.request(f.user, "POST", sessionPath+"/actions", stale), 409)
	if first.Session.Current.MaterialID == view.Current.MaterialID {
		t.Fatal("answered material did not rotate")
	}
	// A new material is picked up as soon as a slot opens.
	newID := f.material(t, "late C")
	view = first.Session
	found := false
	for n := 0; n < 20 && view.Current != nil; n++ {
		if view.Current.MaterialID == newID {
			found = true
		}
		result := decode[model.ActionResult](t, f.request(f.user, "POST", sessionPath+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
		view = result.Session
	}
	if !found || view.Current != nil || view.PoolSize != 0 || view.Summary.Correct != 9 || view.Summary.StagePromotions != 3 {
		t.Fatalf("unexpected completed pool: %+v", view)
	}
	currentPlan := decode[model.TrainingPlan](t, f.request(f.user, "GET", "/training/plans/"+plan.ID.String(), nil), 200)
	if currentPlan.Status != model.StatusActive || currentPlan.Config.PoolSize != 2 {
		t.Fatal("empty pool finished or mutated plan")
	}
	done := decode[model.SessionView](t, f.request(f.user, "POST", sessionPath+"/finish", nil), 200)
	if done.Session.Status != model.StatusCompleted || done.Session.FinishedAt == nil {
		t.Fatal("session not finished")
	}
	decode[map[string]any](t, f.request(f.user, "POST", sessionPath+"/actions", stale), 409)
	// A retry still works after the session is finished.
	request.Action = algorithm.Correct
	decode[model.ActionResult](t, f.request(f.user, "POST", sessionPath+"/actions", request), 200)
	f.now = f.now.AddDate(0, 0, 1)
	next := decode[model.SessionView](t, f.request(f.user, "POST", startPath, nil), 200)
	if next.Session.ID == view.Session.ID || next.Current == nil || next.Current.Stage != 2 {
		t.Fatal("next Stage did not become due")
	}
}

func TestConcurrentActionsAndAtomicRollback(t *testing.T) {
	f := newFixture(t)
	f.material(t, "A")
	p := f.plan(t)
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	// Force the event insert to fail, after the attempted progress write.
	f.exec(t, `CREATE FUNCTION reject_training_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$`)
	f.exec(t, `CREATE TRIGGER reject_event BEFORE INSERT ON training_events FOR EACH ROW EXECUTE FUNCTION reject_training_event()`)
	req := actionFor(v.Current, algorithm.Correct)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", req), 500)
	after := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if after.Current.ID != v.Current.ID || after.Current.ProgressVersion != v.Current.ProgressVersion || after.Summary.Correct != 0 {
		t.Fatal("partial answer committed")
	}
	f.exec(t, `DROP TRIGGER reject_event ON training_events`)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Correct)).Code
		}()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		}
		if code == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("two devices: success=%d conflict=%d", success, conflict)
	}
	after = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if after.Summary.Correct != 1 {
		t.Fatal("answer counted twice")
	}
	// Identical concurrent commands replay exactly one committed event.
	req = actionFor(after.Current, algorithm.Correct)
	results := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- f.request(f.user, "POST", path+"/actions", req) }()
	}
	wg.Wait()
	close(results)
	var event uuid.UUID
	for w := range results {
		r := decode[model.ActionResult](t, w, 200)
		if event != uuid.Nil && event != r.Event.ID {
			t.Fatal("duplicate event")
		}
		event = r.Event.ID
	}
	after = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if after.Summary.Correct != 2 {
		t.Fatal("idempotent commands counted twice")
	}
}

func TestStageCancelsRecoveryHTTP(t *testing.T) {
	f := newFixture(t)
	mat := f.material(t, "A")
	p := f.plan(t)
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	// Move the fixture to a known Stage with an ordinary review due now.
	f.exec(t, `UPDATE user_material_progress SET stage=8,stage_last_review_at=?,stage_review_at=?,version=version+1 WHERE material_id=?`, f.now.AddDate(0, 0, -46), f.now, mat)
	v = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	r := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Wrong)), 200)
	if r.Session.Current == nil || r.Session.Current.Kind != model.ReviewRehab {
		t.Fatal("wrong did not enter rehab")
	}
	view := r.Session
	for _, days := range []int{0, 2} {
		if days != 0 {
			f.now = f.now.AddDate(0, 0, days)
			view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
		}
		for i := 0; i < 3; i++ {
			result := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
			view = result.Session
		}
		if view.Current != nil {
			t.Fatal("rehab day did not leave pool")
		}
	}
	// Make ordinary Stage due before the pending ten-day follow-up.
	f.exec(t, `UPDATE user_material_progress SET stage_review_at=?,version=version+1 WHERE material_id=?`, f.now, mat)
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current.Kind != model.ReviewStage {
		t.Fatal("ordinary Stage did not take priority")
	}
	r = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
	var progress model.UserMaterialProgress
	if err := f.db.Table("user_material_progress").Where("material_id=?", mat).Take(&progress).Error; err != nil {
		t.Fatal(err)
	}
	if progress.ExtraReviewAt != nil || progress.RehabActive || progress.ConsecutiveCorrect != 1 {
		t.Fatal("Stage failed to cancel recovery independently of mastery")
	}
}

func TestCancellationEmptyCardsAndDefaults(t *testing.T) {
	f := newFixture(t)
	// Empty cards never enter the pool and do not starve later valid materials.
	for i := 0; i < 3; i++ {
		id := f.material(t, "empty")
		f.exec(t, `UPDATE materials SET values='{"front":" ","back":"answer"}' WHERE id=?`, id)
	}
	valid := f.material(t, "valid")
	p := f.plan(t)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	if view.Current == nil || view.Current.MaterialID != valid || view.PoolSize != 1 {
		t.Fatal("empty cards entered or starved the pool")
	}
	path := "/training/sessions/" + view.Session.ID.String()
	// Partial mastery survives a finished session and a new one.
	r := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 200)
	decode[model.SessionView](t, f.request(f.user, "POST", path+"/finish", nil), 200)
	view = decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	if view.Current.ConsecutiveCorrect != 1 || view.Current.ProgressVersion != r.Event.ProgressVersionAfter {
		t.Fatal("new session lost mastery")
	}
	path = "/training/sessions/" + view.Session.ID.String()
	decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/cancel", nil), 200)
	cancelled := decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if cancelled.Session.Status != model.StatusCancelled || cancelled.Current != nil {
		t.Fatal("plan cancellation left active session")
	}
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)), 409)
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 409)
	newPlan := f.plan(t)
	if newPlan.ID == p.ID {
		t.Fatal("new plan reused old identity")
	}
	// Defaults have a separate optimistic-lock version.
	defaultsPath := "/folders/" + f.folder.String() + "/training-config"
	d := service.FolderDefaults{Config: folderconfig.TrainingConfig{DefaultAlgorithmKey: "english_basic", PoolSize: 4}, Version: 1}
	decode[service.FolderDefaults](t, f.request(f.user, "PATCH", defaultsPath, d), 200)
	decode[map[string]any](t, f.request(f.user, "PATCH", defaultsPath, d), 409)
	d.Version = 2
	d.Config.PoolSize = 0
	decode[map[string]any](t, f.request(f.user, "PATCH", defaultsPath, d), 400)
	// Cross-owner sources cannot be used, even with an explicit algorithm.
	decode[map[string]any](t, f.request(uuid.New(), "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}}), 404)
}

func TestStaleRecoveryPresentationAndManualStatistics(t *testing.T) {
	f := newFixture(t)
	mat := f.material(t, "A")
	p := f.plan(t)
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	f.exec(t, `UPDATE user_material_progress SET stage=8,stage_last_review_at=?,stage_review_at=?,version=version+1 WHERE material_id=?`, f.now.AddDate(0, 0, -46), f.now, mat)
	v = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	r := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Wrong)), 200)
	stale := actionFor(r.Session.Current, algorithm.Correct)
	f.now = f.now.AddDate(0, 0, 46)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", stale), 409)
	v = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if v.Current.Kind != model.ReviewStage || v.Current.ID == stale.PresentationID {
		t.Fatal("Stage did not invalidate old rehab presentation")
	}
	r = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Rollback)), 200)
	if r.Session.Summary.Wrong != 1 || r.Session.Summary.Correct != 0 || r.Session.Summary.Rollback != 1 {
		t.Fatal("manual rollback was counted as an answer")
	}
	r = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(r.Session.Current, algorithm.SkipRehab)), 200)
	if r.Session.Current != nil || r.Session.Summary.SkipRehab != 1 {
		t.Fatal("skip did not clear recovery")
	}
}

func TestConflictingDefaultsRequireExplicitChoice(t *testing.T) {
	f := newFixture(t)
	second := uuid.New()
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config,training_config)
		SELECT ?,owner_id,'second',template_key,config,'{"default_algorithm_key":"english_adaptive","pool_size":5}' FROM folders WHERE id=?`, second, f.folder)
	req := service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder, second}}
	decode[map[string]any](t, f.request(f.user, "POST", "/training/plans", req), 400)
	key := "english_basic"
	pool := 3
	req.AlgorithmKey = &key
	req.PoolSize = &pool
	p := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", req), 201)
	if p.Config.PoolSize != 3 || len(p.SourceFolderIDs) != 2 {
		t.Fatal("explicit settings did not resolve conflict")
	}
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	if v.Current != nil || v.PoolSize != 0 || v.Session.Status != model.StatusActive {
		t.Fatal("empty source cannot start a session")
	}
	f.material(t, "new")
	v = decode[model.SessionView](t, f.request(f.user, "GET", "/training/sessions/"+v.Session.ID.String(), nil), 200)
	if v.Current == nil {
		t.Fatal("new source material was not discovered")
	}
}

func TestSkipBetweenRehabDaysPreservesStageTimer(t *testing.T) {
	f := newFixture(t)
	mat := f.material(t, "A")
	p := f.plan(t)
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	f.exec(t, `UPDATE user_material_progress SET stage=8,stage_review_at=?,version=version+1 WHERE material_id=?`, f.now, mat)
	v = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	r := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Wrong)), 200)
	for i := 0; i < 3; i++ {
		r = decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(r.Session.Current, algorithm.Correct)), 200)
	}
	if r.Session.Current != nil {
		t.Fatal("second rehab day should be in the future")
	}
	req := service.SkipRequest{CommandID: uuid.New(), ExpectedVersion: r.Event.ProgressVersionAfter}
	skipPath := path + "/materials/" + mat.String() + "/skip-rehab"
	skipped := decode[model.ActionResult](t, f.request(f.user, "POST", skipPath, req), 200)
	want := f.now.AddDate(0, 0, 46).Add(-30 * time.Minute)
	if skipped.NextReviewAt == nil || !skipped.NextReviewAt.Equal(want) || skipped.Event.PresentationID != nil {
		t.Fatal("manual skip changed Stage schedule or invented a presentation")
	}
	retry := decode[model.ActionResult](t, f.request(f.user, "POST", skipPath, req), 200)
	if !reflect.DeepEqual(skipped, retry) {
		t.Fatal("manual skip was not idempotent")
	}
}

func TestAdaptiveHTTPMastery(t *testing.T) {
	f := newFixture(t)
	mat := f.material(t, "hard word")
	f.exec(t, `UPDATE materials SET difficulty='hard' WHERE id=?`, mat)
	key := "english_adaptive"
	p := decode[model.TrainingPlan](t, f.request(f.user, "POST", "/training/plans", service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}, AlgorithmKey: &key}), 201)
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+p.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + v.Session.ID.String()
	for i := 0; i < 5; i++ {
		if v.Current == nil || v.Current.RequiredCorrect != 5 || v.Current.Stage != 1 {
			t.Fatal("adaptive hard mastery changed early")
		}
		r := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", actionFor(v.Current, algorithm.Correct)), 200)
		v = r.Session
	}
	if v.Current != nil || v.Summary.Correct != 5 || v.Summary.StagePromotions != 1 {
		t.Fatal("adaptive mastery not completed")
	}
}
