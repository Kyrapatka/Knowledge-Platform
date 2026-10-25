package handler_test

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestMockPresetHTTPPersistenceOwnershipAndLastUsed(t *testing.T) {
	f, _, _ := graphFixture(t)
	base := "/training/mock-presets"
	cfg := graph.DefaultConfig()
	cfg.QuestionLimit = 20
	req := service.SaveMockPresetRequest{Name: "Go Theory", FolderIDs: []uuid.UUID{f.folder, f.folder}, Settings: &cfg}
	p := decode[model.MockPreset](t, f.request(f.user, "POST", base, req), 201)
	if len(p.FolderIDs) != 1 || p.LastUsedAt != nil {
		t.Fatal(p)
	}
	path := base + "/" + p.ID.String()
	got := decode[model.MockPreset](t, f.request(f.user, "GET", path, nil), 200)
	if got.Settings.QuestionLimit != 20 {
		t.Fatal("settings lost")
	}
	if len(decode[[]model.MockPreset](t, f.request(f.user, "GET", base, nil), 200)) != 1 {
		t.Fatal("list")
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		decode[map[string]any](t, f.request(uuid.New(), method, path, req), 404)
	}
	req.ExpectedVersion = p.Version
	req.Name = "Renamed"
	req.Settings = nil
	p = decode[model.MockPreset](t, f.request(f.user, "PUT", path, req), 200)
	if p.Settings != nil || p.Name != "Renamed" {
		t.Fatal(p)
	}
	copy := decode[model.MockPreset](t, f.request(f.user, "POST", path+"/duplicate", nil), 201)
	if copy.ID == p.ID || copy.LastUsedAt != nil || copy.Settings != nil || !reflect.DeepEqual(copy.FolderIDs, p.FolderIDs) {
		t.Fatal(copy)
	}
	start := service.StartGraphRequest{CommandID: uuid.New(), PresetIDs: []uuid.UUID{p.ID, copy.ID}, Config: &cfg}
	decode[graph.InterviewPlan](t, f.request(f.user, "POST", "/training/mock-interviews/preview", start), 200)
	got = decode[model.MockPreset](t, f.request(f.user, "GET", path, nil), 200)
	if got.LastUsedAt != nil {
		t.Fatal("preview marked used")
	}
	v := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", start), 200)
	if len(v.Session.Selection) != 1 || v.Session.Selection[0].MaterialIDs != nil {
		t.Fatal("preset froze materials or duplicated folders")
	}
	got = decode[model.MockPreset](t, f.request(f.user, "GET", path, nil), 200)
	if got.LastUsedAt == nil || got.Settings != nil {
		t.Fatal("last used or session override persistence")
	}
	retried := decode[model.SessionView](t, f.request(f.user, "POST", "/training/mock-interviews", start), 200)
	if retried.Session.ID != v.Session.ID {
		t.Fatal("retry created session")
	}
	// Folder deletion is reflected on the next read without losing the preset.
	f.exec(t, "UPDATE folders SET deleted_at=now() WHERE id=?", f.folder)
	got = decode[model.MockPreset](t, f.request(f.user, "GET", path, nil), 200)
	if len(got.FolderIDs) != 0 || len(got.UnavailableFolderIDs) != 1 {
		t.Fatal(got)
	}
	start.CommandID = uuid.New()
	decode[map[string]any](t, f.request(f.user, "POST", "/training/mock-interviews", start), 400)
	decode[map[string]any](t, f.request(f.user, "DELETE", path, nil), 200)
	decode[map[string]any](t, f.request(f.user, "GET", path, nil), 404)
}
