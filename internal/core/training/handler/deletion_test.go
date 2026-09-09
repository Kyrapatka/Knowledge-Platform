package handler_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderhandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/handler"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	folderservice "github.com/Kyrapatka/knowledge-platform/internal/core/folder/service"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialhandler "github.com/Kyrapatka/knowledge-platform/internal/core/material/handler"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	trainingpg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/google/uuid"
)

func (f *fixture) libraryRoutes() {
	api := f.router.Group("/api/v1")
	api.Use(auth.AuthMiddleware(testParser{}))
	folders := folderpg.NewRepository(f.db)
	m := materialhandler.NewHandler(materialservice.NewService(materialpg.NewRepository(f.db), folders))
	h := folderhandler.NewHandler(folderservice.NewService(folders, nil))
	api.DELETE("/folders/:folderID/materials/:materialID", m.Delete)
	api.GET("/folders/:folderID/materials/:materialID", m.GetByID)
	api.DELETE("/folders/:folderID", h.Delete)
}

func TestSoftDeletePreservesTrainingHistory(t *testing.T) {
	f := newFixture(t)
	f.libraryRoutes()
	m := f.material(t, "A")
	f.exercise(t, m, "Problem")
	plan := f.plan(t)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + view.Session.ID.String()
	req := actionFor(view.Current, algorithm.Correct)
	answered := decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", req), 200)
	materialPath := "/folders/" + f.folder.String() + "/materials/" + m.String()
	other := uuid.New()
	f.exec(t, "INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES(?,'other','other','unused')", other)
	if r := f.request(other, "DELETE", materialPath, nil); r.Code != 404 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := f.request(f.user, "DELETE", materialPath, nil); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	decode[map[string]any](t, f.request(f.user, "GET", materialPath, nil), 404)
	decode[map[string]any](t, f.request(f.user, "POST", path+"/actions", actionFor(answered.Session.Current, algorithm.Correct)), 409)
	decode[model.ActionResult](t, f.request(f.user, "POST", path+"/actions", req), 200)
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current != nil || view.Summary.Correct != 1 {
		t.Fatal(view)
	}
	ctx := context.Background()
	rows, err := materialpg.NewRepository(f.db).ListByFolder(ctx, f.folder)
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	due, err := trainingpg.NewProgressRepository(f.db).ListDue(ctx, f.user, model.ProgressTrackDefault, f.now, 10)
	if err != nil || len(due) != 0 {
		t.Fatal(due, err)
	}
	for _, table := range []string{"materials", "user_material_progress", "training_events", "formula_exercises"} {
		var n int64
		q := f.db.Table(table)
		if table == "materials" {
			q = q.Where("id=? AND deleted_at IS NOT NULL", m)
		} else {
			q = q.Where("material_id=?", m)
		}
		if err := q.Count(&n).Error; err != nil || n != 1 {
			t.Fatal("history lost", table, n, err)
		}
	}
}

func TestDeleteFolderAndAnswerRace(t *testing.T) {
	f := newFixture(t)
	f.libraryRoutes()
	m := f.material(t, "A")
	plan := f.plan(t)
	view := decode[model.SessionView](t, f.request(f.user, "POST", "/training/plans/"+plan.ID.String()+"/sessions", nil), 200)
	path := "/training/sessions/" + view.Session.ID.String()
	var wg sync.WaitGroup
	var deleteCode, answerCode int
	wg.Add(2)
	go func() {
		defer wg.Done()
		deleteCode = f.request(f.user, "DELETE", "/folders/"+f.folder.String(), nil).Code
	}()
	go func() {
		defer wg.Done()
		answerCode = f.request(f.user, "POST", path+"/actions", actionFor(view.Current, algorithm.Correct)).Code
	}()
	wg.Wait()
	if deleteCode != 204 || (answerCode != 200 && answerCode != 409) {
		t.Fatal(deleteCode, answerCode)
	}
	ctx := context.Background()
	if _, err := folderpg.NewRepository(f.db).GetByID(ctx, f.folder); !errors.Is(err, folder.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := materialpg.NewRepository(f.db).GetByID(ctx, m); !errors.Is(err, material.ErrNotFound) {
		t.Fatal(err)
	}
	view = decode[model.SessionView](t, f.request(f.user, "GET", path, nil), 200)
	if view.Current != nil {
		t.Fatal("deleted folder remains in pool")
	}
	// Plan/history remains accessible despite its now deleted source folder.
	decode[model.TrainingPlan](t, f.request(f.user, "GET", "/training/plans/"+plan.ID.String(), nil), 200)
	decode[map[string]any](t, f.request(f.user, "GET", "/folders/"+f.folder.String()+"/training-config", nil), 404)
}

func TestDeleteFolderAndCreateMaterialRace(t *testing.T) {
	f := newFixture(t)
	value := "Content"
	id := uuid.New()
	now := time.Now().UTC()
	m := materialmodel.Material{ID: id, FolderID: f.folder, Values: map[string]*string{"front": &value, "back": &value}, Metadata: map[string]*string{}, Difficulty: materialmodel.DifficultyMedium, CreatedAt: now, UpdatedAt: now}
	var wg sync.WaitGroup
	var createErr, deleteErr error
	wg.Add(2)
	go func() { defer wg.Done(); createErr = materialpg.NewRepository(f.db).Create(context.Background(), m) }()
	go func() {
		defer wg.Done()
		deleteErr = folderpg.NewRepository(f.db).Delete(context.Background(), f.folder)
	}()
	wg.Wait()
	if deleteErr != nil || (createErr != nil && !errors.Is(createErr, material.ErrFolderNotFound)) {
		t.Fatal(createErr, deleteErr)
	}
	var n int64
	if err := f.db.Table("materials").Where("folder_id=? AND deleted_at IS NULL", f.folder).Count(&n).Error; err != nil || n != 0 {
		t.Fatal("live material survived folder deletion", n, err)
	}
}
