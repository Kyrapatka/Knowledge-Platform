package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fixture struct {
	db           *gorm.DB
	store        *Store
	user, folder uuid.UUID
	now          time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL for isolated PostgreSQL projection tests")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		t.Fatal("test DSN must be a PostgreSQL URL")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	baseSQL, err := base.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := "dashboard_test_" + uuid.New().String()[:8]
	if err := base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		baseSQL.Close()
		t.Fatal(err)
	}
	// This is the only destructive test operation, scoped to the unique schema
	// created above. Never truncate, migrate or drop application tables.
	t.Cleanup(func() {
		if err := base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Error(err)
		}
		baseSQL.Close()
	})
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	files, err := filepath.Glob("../../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("missing migrations", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(data)).Error; err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	f := &fixture{db: db, user: uuid.New(), folder: uuid.New(), now: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)}
	f.exec(t, `INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'dashboard','dashboard','unused')`, f.user)
	f.exec(t, `INSERT INTO folders(id,owner_id,title,template_key,config) VALUES(?,?,'Interview','interview_questions',
 '{"schema":{"fields":[]},"metadata_schema":{"fields":[]},"card":{"question_fields":["question"],"answer_fields":["answer"]}}')`, f.folder, f.user)
	f.store = NewStore(db)
	f.store.now = func() time.Time { return f.now }
	return f
}

func (f *fixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if err := f.db.Exec(query, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) material(t *testing.T, question, metadata string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	values, err := json.Marshal(map[string]string{"question": question, "answer": "The answer"})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO materials(id,folder_id,values,metadata) VALUES(?,?,?,?)`, id, f.folder, string(values), metadata)
	return id
}

func (f *fixture) plan(t *testing.T, track string, created time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	key, horizon := "interview_long_term", 150
	if track == "cram" {
		key, horizon = "interview_cram", 5
	}
	c, err := json.Marshal(map[string]any{"horizon_days": horizon, "pool_size": 5, "cards": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO training_plans(id,user_id,track,algorithm_key,algorithm_version,status,config,started_at,created_at,updated_at)
 VALUES(?,?,?,?,1,'active',?,?,?,?)`, id, f.user, track, key, string(c), created, created, created)
	f.exec(t, `INSERT INTO training_plan_sources(plan_id,folder_id) VALUES(?,?)`, id, f.folder)
	return id
}

func queryDefaults() MaterialsQuery {
	return MaterialsQuery{Sort: "created_at", Direction: "asc", Limit: 50}
}

func TestDefaultProgressFollowsSelectedAlgorithm(t *testing.T) {
	f := newFixture(t)
	longTerm := f.plan(t, "long_term", f.now.Add(-48*time.Hour))
	f.plan(t, "cram", f.now.Add(-24*time.Hour))
	f.exec(t, `UPDATE folders SET training_config='{"default_algorithm_key":"interview_long_term","pool_size":5}' WHERE id=?`, f.folder)
	library, err := f.store.Library(context.Background(), f.user)
	if err != nil || len(library.Folders) != 1 || library.Folders[0].SelectedPlan == nil || library.Folders[0].SelectedPlan.ID != longTerm {
		t.Fatalf("library did not follow selected algorithm: %+v, %v", library, err)
	}
	page, err := f.store.FolderMaterials(context.Background(), f.user, f.folder, queryDefaults())
	if err != nil || page.SelectedPlan == nil || page.SelectedPlan.ID != longTerm {
		t.Fatalf("material context did not follow selected algorithm: %+v, %v", page, err)
	}
}

func TestProjectionSelectsPlanWithoutStartingLearning(t *testing.T) {
	f := newFixture(t)
	a := f.material(t, "SQL joins", `{"topic":" SQL ","category":"Legacy"}`)
	f.material(t, "SQL index", `{"category":"SQL"}`)
	f.material(t, "Literal 100%", `{}`)
	deleted := f.material(t, "Deleted", `{"topic":"SQL"}`)
	f.exec(t, `UPDATE materials SET deleted_at=? WHERE id=?`, f.now, deleted)
	longTerm := f.plan(t, "long_term", f.now.Add(-48*time.Hour))
	cram := f.plan(t, "cram", f.now.Add(-24*time.Hour))
	f.exec(t, `INSERT INTO user_material_progress(user_id,material_id,track,algorithm_key,algorithm_version,stage,
 learning_started_at,target_at,stage_review_at,version,created_at,updated_at)
 VALUES(?,?,'long_term','interview_long_term',1,5,?,?,?,1,?,?)`,
		f.user, a, f.now.Add(-40*24*time.Hour), f.now.Add(110*24*time.Hour), f.now.Add(-time.Hour), f.now, f.now)
	f.exec(t, `INSERT INTO user_material_progress(user_id,material_id,track,plan_id,algorithm_key,algorithm_version,stage,
 learning_started_at,target_at,stage_review_at,version,created_at,updated_at)
 VALUES(?,?,'cram',?,'interview_cram',1,4,?,?,?,2,?,?)`,
		f.user, a, cram, f.now.Add(-4*24*time.Hour), f.now.Add(24*time.Hour), f.now.Add(24*time.Hour), f.now, f.now)
	ctx := context.Background()
	library, err := f.store.Library(ctx, f.user)
	if err != nil {
		t.Fatal(err)
	}
	if len(library.Folders) != 1 || library.Totals.MaterialCount != 3 || library.Totals.LearningCount != 1 || library.Totals.DueCount != 0 ||
		library.Folders[0].SelectedPlan == nil || library.Folders[0].SelectedPlan.ID != cram {
		t.Fatalf("unexpected summary: %+v", library)
	}
	if len(library.Folders[0].Topics) != 2 || library.Folders[0].Topics[1].Name != "SQL" || library.Folders[0].Topics[1].Count != 2 {
		t.Fatalf("topic fallback/counts: %+v", library.Folders[0].Topics)
	}
	q := queryDefaults()
	q.Topic = "SQL"
	q.Sort = "question"
	q.Limit = 1
	result, err := f.store.FolderMaterials(ctx, f.user, f.folder, q)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 1 || result.Items[0].ID == a || len(result.Plans) != 2 || result.SelectedPlan.ID != cram {
		t.Fatalf("pagination/selection: %+v", result)
	}
	q.Offset = 1
	result, err = f.store.FolderMaterials(ctx, f.user, f.folder, q)
	if err != nil {
		t.Fatal(err)
	}
	p := result.Items[0].Progress
	if p == nil || p.Stage != 4 || p.Version != 2 || !p.CanStartFinal || p.PlanID == nil || *p.PlanID != cram {
		t.Fatalf("cram progress: %+v", p)
	}
	q.PlanID = &longTerm
	result, err = f.store.FolderMaterials(ctx, f.user, f.folder, q)
	if err != nil || result.Items[0].Progress.Stage != 5 || result.Items[0].Progress.PlanID != nil {
		t.Fatalf("long-term progress: %+v, %v", result, err)
	}
	q = queryDefaults()
	q.Search = "%"
	result, err = f.store.FolderMaterials(ctx, f.user, f.folder, q)
	if err != nil || result.Total != 1 || result.Items[0].Topic != "" || result.Items[0].Progress != nil {
		t.Fatalf("literal search/unstarted material: %+v, %v", result, err)
	}
	q.Search, q.Topic = "", "__none__"
	result, err = f.store.FolderMaterials(ctx, f.user, f.folder, q)
	if err != nil || result.Total != 1 {
		t.Fatalf("no-topic filter: %+v, %v", result, err)
	}
	var progressCount, sessionCount int64
	f.db.Table("user_material_progress").Count(&progressCount)
	f.db.Table("training_sessions").Count(&sessionCount)
	if progressCount != 2 || sessionCount != 0 {
		t.Fatal("read projections changed training state")
	}
	if _, err := f.store.FolderMaterials(ctx, uuid.New(), f.folder, queryDefaults()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign folder exposed: %v", err)
	}
	foreignPlan := uuid.New()
	q = queryDefaults()
	q.PlanID = &foreignPlan
	if _, err := f.store.FolderMaterials(ctx, f.user, f.folder, q); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign plan accepted: %v", err)
	}
	f.exec(t, `UPDATE training_plans SET status='cancelled' WHERE id=?`, cram)
	library, err = f.store.Library(ctx, f.user)
	if err != nil || library.Totals.DueCount != 1 || library.Folders[0].SelectedPlan.ID != longTerm {
		t.Fatalf("cancelled plan remained default: %+v, %v", library, err)
	}
	f.exec(t, `UPDATE folders SET deleted_at=? WHERE id=?`, f.now, f.folder)
	library, err = f.store.Library(ctx, f.user)
	if err != nil || len(library.Folders) != 0 {
		t.Fatalf("deleted folder exposed: %+v, %v", library, err)
	}
	if _, err := f.store.FolderMaterials(ctx, f.user, f.folder, queryDefaults()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted folder materials exposed: %v", err)
	}
}

func TestStatisticsCountsOnlyAnswersAndUsesLocalDays(t *testing.T) {
	f := newFixture(t)
	a := f.material(t, "SQL", `{}`)
	b := f.material(t, "HTTP", `{}`)
	undone := f.material(t, "Undone card", `{}`)
	plan := f.plan(t, "long_term", f.now.Add(-10*24*time.Hour))
	session := uuid.New()
	f.exec(t, `INSERT INTO training_sessions(id,plan_id,user_id,status,started_at,created_at) VALUES(?,?,?,'active',?,?)`, session, plan, f.user, f.now, f.now)
	addEvent := func(material uuid.UUID, action string, created time.Time, after int) {
		t.Helper()
		f.exec(t, `INSERT INTO training_events(id,command_id,user_id,plan_id,session_id,material_id,presentation_id,action,kind,
 algorithm_key,algorithm_version,stage_before,stage_after,progress_version_before,progress_version_after,created_at)
 VALUES(?,?,?,?,?,?,?,?,'stage','interview_long_term',1,1,?,1,2,?)`,
			uuid.New(), uuid.New(), f.user, plan, session, material, uuid.New(), action, after, created)
	}
	addEvent(a, "correct", time.Date(2026, 9, 8, 22, 30, 0, 0, time.UTC), 2) // September 9 in Moscow.
	addEvent(a, "wrong", time.Date(2026, 9, 9, 22, 30, 0, 0, time.UTC), 1)   // September 10 in Moscow.
	addEvent(b, "correct", time.Date(2026, 9, 10, 7, 30, 0, 0, time.UTC), 1)
	addEvent(b, "advance", f.now.Add(-time.Hour), 2)
	addEvent(b, "start_final", f.now.Add(-time.Hour), 1)
	addEvent(b, "correct", f.now.Add(-20*24*time.Hour), 2)
	addEvent(b, "correct", f.now.Add(time.Hour), 2) // Future rows are not current activity.
	addEvent(undone, "correct", time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC), 2)
	addEvent(undone, "wrong", time.Date(2026, 9, 8, 12, 1, 0, 0, time.UTC), 1)
	f.exec(t, `UPDATE training_events SET undone_at=? WHERE material_id=?`, f.now, undone)
	f.exec(t, `UPDATE materials SET deleted_at=? WHERE id=?`, f.now, a)
	result, err := f.store.Statistics(context.Background(), f.user, 3, "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if result.Totals.Answers != 3 || result.Totals.Correct != 2 || result.Totals.Wrong != 1 ||
		result.Totals.MaterialsReviewed != 2 || result.Totals.Sessions != 1 || result.Totals.StagePromotions != 1 || result.Totals.ActiveDays != 2 {
		t.Fatalf("incorrect totals: %+v", result.Totals)
	}
	if len(result.Daily) != 3 || result.Daily[0].Date != "2026-09-08" || result.Daily[0].Answers != 0 ||
		result.Daily[1].Answers != 1 || result.Daily[2].Answers != 2 || result.Daily[2].MaterialsReviewed != 2 {
		t.Fatalf("local dates: %+v", result.Daily)
	}
	other, err := f.store.Statistics(context.Background(), uuid.New(), 3, "Europe/Moscow")
	if err != nil || other.Totals.Answers != 0 || len(other.Daily) != 3 {
		t.Fatalf("foreign history exposed: %+v, %v", other, err)
	}
}

func TestInvalidQueriesAndUnauthenticatedRoutes(t *testing.T) {
	q := queryDefaults()
	q.Sort = "stage; DELETE FROM users"
	if !errors.Is(q.validate(), ErrInvalid) {
		t.Fatal("sort must be allowlisted")
	}
	store := NewStore(nil)
	for _, zone := range []string{"Local", "Invalid/Timezone", ""} {
		if _, err := store.Statistics(context.Background(), uuid.New(), 30, zone); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid zone %q accepted: %v", zone, err)
		}
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil).RegisterRoutes(router.Group("/api/v1"))
	for _, path := range []string{"/library", "/library/folders/" + uuid.New().String() + "/materials", "/statistics"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1"+path, nil))
		if w.Code != 401 {
			t.Fatalf("%s lacks auth check: %d", path, w.Code)
		}
	}
}

type testParser struct{}

func (testParser) ParseAccessToken(_ context.Context, value string) (token.AccessTokenClaims, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return token.AccessTokenClaims{}, token.ErrInvalidAccessToken
	}
	return token.AccessTokenClaims{UserID: id}, nil
}

func TestHTTPValidationAndSerialization(t *testing.T) {
	f := newFixture(t)
	f.material(t, "Question", `{"topic":"SQL"}`)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	api.Use(auth.AuthMiddleware(testParser{}))
	h := NewHandler(f.db)
	h.store.now = f.store.now
	h.RegisterRoutes(api)
	request := func(path string, want int) []byte {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1"+path, nil)
		r.Header.Set("Authorization", "Bearer "+f.user.String())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: HTTP %d, expected %d: %s", path, w.Code, want, w.Body.String())
		}
		return w.Body.Bytes()
	}
	request("/library", 200)
	request("/statistics?timezone=UTC&days=7", 200)
	request("/statistics?timezone=bad", 400)
	request("/statistics?days=367", 400)
	path := "/library/folders/" + f.folder.String() + "/materials"
	request(path+"?limit=0", 400)
	request(path+"?direction=invalid", 400)
	request(path+"?plan_id=invalid", 400)
	var view map[string]any
	if err := json.Unmarshal(request(path, 200), &view); err != nil {
		t.Fatal(err)
	}
	if view["selected_plan"] != nil || len(view["plans"].([]any)) != 0 || len(view["items"].([]any)) != 1 {
		t.Fatalf("empty context serialization: %+v", view)
	}
}
