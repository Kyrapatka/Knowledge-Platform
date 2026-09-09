package postgres

import (
	"context"
	"errors"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialpostgres "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This test never truncates existing tables. All migrations, fixtures and
// assertions run in a unique schema inside a transaction that is rolled back.
func TestProgressPostgres(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	schema := "training_test_" + uuid.New().String()[:8]
	if err = tx.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(`SET LOCAL search_path TO "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../../../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatal("missing migrations", err)
	}
	for _, file := range files {
		content, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Exec(string(content)).Error; e != nil {
			t.Fatalf("%s: %v", file, e)
		}
	}
	ctx := context.Background()
	user, folder, mat := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?, 'training', 'training', 'unused')", []any{user}},
		{"INSERT INTO folders(id,owner_id,title,template_key) VALUES(?,?,'training','english_words')", []any{folder, user}},
		{"INSERT INTO materials(id,folder_id) VALUES(?,?)", []any{mat, folder}},
	} {
		if err = tx.Exec(q.sql, q.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	materialRepo := materialpostgres.NewRepository(tx)
	m, err := materialRepo.GetByID(ctx, mat)
	if err != nil || m.Difficulty != materialmodel.DifficultyMedium {
		t.Fatal("material default difficulty did not round-trip", err)
	}
	m.Difficulty = materialmodel.DifficultyHard
	if err = materialRepo.Update(ctx, m); err != nil {
		t.Fatal(err)
	}
	m, err = materialRepo.GetByID(ctx, mat)
	if err != nil || m.Difficulty != materialmodel.DifficultyHard {
		t.Fatal("material difficulty update did not round-trip", err)
	}
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	stage := now.AddDate(0, 0, 40)
	p := model.UserMaterialProgress{UserID: user, MaterialID: mat, Track: model.ProgressTrackDefault,
		AlgorithmKey: "english_basic", AlgorithmVersion: 1, Stage: 8, Version: 1,
		LearningStartedAt: now, CreatedAt: now, UpdatedAt: now, StageLastReviewAt: &now, StageReviewAt: &stage}
	p.StartRehab(now)
	r := NewProgressRepository(tx)
	if err = r.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	key := repository.ProgressKey{UserID: user, MaterialID: mat, Track: p.Track}
	got, err := r.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !got.RehabActive || got.RehabStep != 1 {
		t.Fatal("rehab did not round-trip")
	}
	rows, err := r.ListDue(ctx, user, p.Track, now, 10)
	if err != nil || len(rows) != 1 {
		t.Fatal("due rehab missing", rows, err)
	}
	// Duplicate persistent identities must conflict even with nullable PlanID.
	err = tx.Transaction(func(nested *gorm.DB) error { return NewProgressRepository(nested).Create(ctx, p) })
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatal("duplicate identity accepted", err)
	}
	got.CancelRecovery()
	updated, err := r.Update(ctx, got, 1)
	if err != nil || updated.Version != 2 {
		t.Fatal("update failed", err)
	}
	if _, err = r.Update(ctx, got, 1); !errors.Is(err, repository.ErrConflict) {
		t.Fatal("stale write accepted", err)
	}
	got, err = r.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got.RehabActive || got.RehabStep != 0 || got.RehabReviewAt != nil {
		t.Fatal("zero/NULL values not persisted")
	}
	rows, err = r.ListDue(ctx, user, p.Track, now, 10)
	if err != nil || len(rows) != 0 {
		t.Fatal("generated next date is stale", err)
	}
	got.ExtraReviewAt = &now
	got, err = r.Update(ctx, got, 2)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = r.ListDue(ctx, user, p.Track, now, 10)
	if err != nil || len(rows) != 1 {
		t.Fatal("extra review missing", err)
	}
	// CRAM runs of the same material are isolated from each other and persistent progress.
	for i := 0; i < 2; i++ {
		plan := uuid.New()
		err = tx.Exec("INSERT INTO training_plans(id,user_id,track,algorithm_key,algorithm_version,status,config,started_at,created_at,updated_at) VALUES(?,?,'cram','interview_cram',1,'active','{}',?,?,?)", plan, user, now, now, now).Error
		if err != nil {
			t.Fatal(err)
		}
		cram := p
		cram.Track = model.ProgressTrackCram
		cram.PlanID = &plan
		cram.AlgorithmKey = "interview_cram"
		if err = r.Create(ctx, cram); err != nil {
			t.Fatal(err)
		}
	}
	// Validate rollback migrations too, still entirely inside the private schema.
	for _, name := range []string{"000012_training_changes_and_deletion.down.sql", "000011_formula_training.down.sql", "000010_interview_final.down.sql", "000009_training_runtime.down.sql", "000008_create_training_progress.down.sql", "000007_add_material_difficulty.down.sql"} {
		content, e := os.ReadFile(filepath.Join("../../../../../migrations", name))
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Exec(string(content)).Error; e != nil {
			t.Fatal(e)
		}
	}
}
