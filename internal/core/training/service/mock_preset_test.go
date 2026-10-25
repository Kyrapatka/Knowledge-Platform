package service

import (
	"context"
	"encoding/json"
	"errors"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"testing"
	"time"
)

type presetMemory struct {
	repository.Tx
	user    uuid.UUID
	rows    map[uuid.UUID]model.MockPreset
	folders map[uuid.UUID]folder.Folder
}

func (m *presetMemory) Transact(_ context.Context, u uuid.UUID, fn func(repository.Tx) error) error {
	m.user = u
	return fn(m)
}
func (m *presetMemory) MockPresets() repository.MockPresetRepository { return m }
func (m *presetMemory) Folder(id uuid.UUID) (folder.Folder, error) {
	f, ok := m.folders[id]
	if !ok {
		return f, repository.ErrNotFound
	}
	return f, nil
}
func clonePreset(p model.MockPreset) model.MockPreset {
	b, _ := json.Marshal(p)
	var c model.MockPreset
	_ = json.Unmarshal(b, &c)
	c.UserID = p.UserID
	return c
}
func (m *presetMemory) Get(id uuid.UUID) (model.MockPreset, error) {
	p, ok := m.rows[id]
	if !ok || p.UserID != m.user {
		return model.MockPreset{}, repository.ErrNotFound
	}
	return clonePreset(p), nil
}
func (m *presetMemory) List() ([]model.MockPreset, error) {
	out := []model.MockPreset{}
	for _, p := range m.rows {
		if p.UserID == m.user {
			out = append(out, clonePreset(p))
		}
	}
	return out, nil
}
func (m *presetMemory) Create(p model.MockPreset) error { m.rows[p.ID] = clonePreset(p); return nil }
func (m *presetMemory) Update(p model.MockPreset, v int) error {
	old, e := m.Get(p.ID)
	if e != nil {
		return e
	}
	if old.Version != v {
		return repository.ErrConflict
	}
	m.rows[p.ID] = clonePreset(p)
	return nil
}
func (m *presetMemory) Delete(id uuid.UUID) error {
	if _, e := m.Get(id); e != nil {
		return e
	}
	delete(m.rows, id)
	return nil
}
func (m *presetMemory) Used(id uuid.UUID, at time.Time) error {
	p, e := m.Get(id)
	if e != nil {
		return e
	}
	p.LastUsedAt = &at
	m.rows[id] = p
	return nil
}
func TestMockPresetLifecycleAndSourceResolution(t *testing.T) {
	ctx := context.Background()
	user, other := uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	m := &presetMemory{rows: map[uuid.UUID]model.MockPreset{}, folders: map[uuid.UUID]folder.Folder{}}
	for _, id := range []uuid.UUID{a, b, c} {
		m.folders[id] = folder.Folder{ID: id, TemplateKey: "interview_questions"}
	}
	s := NewService(m)
	cfg := graph.DefaultConfig()
	cfg.QuestionLimit = 20
	p, err := s.SaveMockPreset(ctx, user, uuid.Nil, SaveMockPresetRequest{Name: " Go Theory ", FolderIDs: []uuid.UUID{a, b, b}, Settings: &cfg})
	if err != nil || len(p.FolderIDs) != 2 || p.Name != "Go Theory" || p.LastUsedAt != nil {
		t.Fatal(p, err)
	}
	list, err := s.MockPresets(ctx, user)
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	list, err = s.MockPresets(ctx, other)
	if err != nil || len(list) != 0 {
		t.Fatal("list ownership", err)
	}
	if _, err = s.MockPreset(ctx, other, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("get ownership", err)
	}
	if _, err = s.SaveMockPreset(ctx, other, p.ID, SaveMockPresetRequest{Name: "stolen", ExpectedVersion: p.Version}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("update ownership", err)
	}
	if err = s.DeleteMockPreset(ctx, other, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("delete ownership", err)
	}
	if _, err = s.DuplicateMockPreset(ctx, other, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("copy ownership", err)
	}
	copy, err := s.DuplicateMockPreset(ctx, user, p.ID)
	if err != nil || copy.ID == p.ID || copy.LastUsedAt != nil || copy.Settings.QuestionLimit != 20 {
		t.Fatal(copy, err)
	}
	copy, err = s.SaveMockPreset(ctx, user, copy.ID, SaveMockPresetRequest{Name: "SQL", FolderIDs: []uuid.UUID{b, c}, ExpectedVersion: copy.Version})
	if err != nil || copy.Settings != nil {
		t.Fatal("null settings", copy, err)
	}
	m.user = user
	resolved, err := resolveMockRequest(m, StartGraphRequest{PresetIDs: []uuid.UUID{p.ID, p.ID}})
	if err != nil || resolved.Config.QuestionLimit != 20 || len(resolved.Sources) != 2 || len(resolved.PresetIDs) != 1 {
		t.Fatal(resolved, err)
	}
	override := cfg
	override.QuestionLimit = 30
	override.InterviewMode = "deep"
	override.DepthLevel = 2
	resolved, err = resolveMockRequest(m, StartGraphRequest{PresetIDs: []uuid.UUID{p.ID}, Config: &override})
	if err != nil || resolved.Config.QuestionLimit != 30 || m.rows[p.ID].Settings.QuestionLimit != 20 {
		t.Fatal("override persisted", err)
	}
	resolved, err = resolveMockRequest(m, StartGraphRequest{PresetIDs: []uuid.UUID{p.ID, copy.ID}})
	if err != nil || len(resolved.Sources) != 3 || resolved.Config != nil {
		t.Fatal("multi merged settings or duplicated folders", resolved, err)
	}
	if _, err = resolveMockRequest(m, StartGraphRequest{PresetIDs: []uuid.UUID{p.ID}, Sources: []model.SessionSource{{FolderID: a, MaterialIDs: []uuid.UUID{uuid.New()}}}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("exact selection widened", err)
	}
	delete(m.folders, a)
	got, err := s.MockPreset(ctx, user, p.ID)
	if err != nil || len(got.FolderIDs) != 1 || len(got.UnavailableFolderIDs) != 1 {
		t.Fatal(got, err)
	}
	if len(m.rows[p.ID].FolderIDs) != 2 {
		t.Fatal("read mutated saved preset")
	}
	delete(m.folders, b)
	if _, err = resolveMockRequest(m, StartGraphRequest{PresetIDs: []uuid.UUID{p.ID}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty effective start", err)
	}
	got, err = s.SaveMockPreset(ctx, user, p.ID, SaveMockPresetRequest{Name: "Renamed", FolderIDs: []uuid.UUID{c}, Settings: &override, ExpectedVersion: p.Version})
	if err != nil || got.Name != "Renamed" || got.Settings.QuestionLimit != 30 {
		t.Fatal(got, err)
	}
	if _, err = s.SaveMockPreset(ctx, user, p.ID, SaveMockPresetRequest{Name: "stale writer", ExpectedVersion: p.Version}); !errors.Is(err, repository.ErrConflict) {
		t.Fatal("lost update", err)
	}
	if err = s.DeleteMockPreset(ctx, user, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MockPreset(ctx, user, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
}
