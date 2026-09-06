package service

import (
	"context"
	"errors"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
	"testing"
)

func TestDifficultyOnlyPatchAndPreservation(t *testing.T) {
	user, folderID, matID := uuid.New(), uuid.New(), uuid.New()
	stored := materialmodel.Material{ID: matID, FolderID: folderID, Difficulty: materialmodel.DifficultyEasy}
	r := &fakeMaterialRepository{
		getByIDFunc: func(context.Context, uuid.UUID) (materialmodel.Material, error) { return stored, nil },
		updateFunc:  func(_ context.Context, m materialmodel.Material) error { stored = m; return nil },
	}
	f := &fakeFolderRepository{getByIDFunc: func(context.Context, uuid.UUID) (foldermodel.Folder, error) {
		return englishFolder(folderID, user), nil
	}}
	s := NewService(r, f)
	hard := materialmodel.DifficultyHard
	got, err := s.UpdateWithDifficulty(context.Background(), user, folderID, matID, nil, nil, &hard)
	if err != nil || got.Difficulty != hard || stored.Difficulty != hard {
		t.Fatal("difficulty-only patch failed", err)
	}
	got, err = s.Update(context.Background(), user, folderID, matID, nil, nil)
	if err != nil || got.Difficulty != hard {
		t.Fatal("omitting difficulty reset it", err)
	}
	invalid := materialmodel.Difficulty("unknown")
	_, err = s.UpdateWithDifficulty(context.Background(), user, folderID, matID, nil, nil, &invalid)
	if !errors.Is(err, material.ErrInvalidDifficulty) || stored.Difficulty != hard {
		t.Fatal("invalid difficulty was accepted", err)
	}
}

func TestMaterialCreateDifficulty(t *testing.T) {
	user, folderID := uuid.New(), uuid.New()
	var stored materialmodel.Material
	r := &fakeMaterialRepository{createFunc: func(_ context.Context, m materialmodel.Material) error {
		stored = m
		return nil
	}}
	f := &fakeFolderRepository{getByIDFunc: func(context.Context, uuid.UUID) (foldermodel.Folder, error) {
		return englishFolder(folderID, user), nil
	}}
	s := NewService(r, f)
	foreign, native := "word", "translation"
	values := map[string]*string{"foreign": &foreign, "native": &native}
	if _, err := s.Create(context.Background(), user, folderID, values, nil); err != nil {
		t.Fatal(err)
	}
	if stored.Difficulty != materialmodel.DifficultyMedium {
		t.Fatal("default difficulty must be medium")
	}
	if _, err := s.CreateWithDifficulty(context.Background(), user, folderID, values, nil, materialmodel.DifficultyHard); err != nil {
		t.Fatal(err)
	}
	if stored.Difficulty != materialmodel.DifficultyHard {
		t.Fatal("explicit difficulty lost")
	}
}
