package materialimport

import (
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }
func english() foldermodel.Folder {
	return foldermodel.Folder{TemplateKey: "english_words", Config: folderconfig.FolderConfig{Schema: folderconfig.MaterialSchema{Fields: []folderconfig.FieldDefinition{{Key: "foreign", Active: true, Required: true}, {Key: "native", Active: true, Required: true}, {Key: "transcription", Active: true}, {Key: "hidden"}}}}}
}
func item(id, word string) Item {
	return Item{ClientID: id, Values: map[string]*string{"foreign": ptr(word), "native": ptr("перевод")}}
}
func TestImportValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Request)
	}{
		{"required", func(r *Request) { delete(r.Items[0].Values, "native") }},
		{"inactive", func(r *Request) { r.Items[0].Values["hidden"] = ptr("x") }},
		{"pronunciation", func(r *Request) { r.Items[0].Values["transcription"] = ptr("x") }},
		{"empty", func(r *Request) { r.Items[0].Values["native"] = ptr("  ") }},
		{"unknown", func(r *Request) { r.Items[0].Values["typo"] = ptr("x") }},
		{"difficulty", func(r *Request) { r.Items[0].Difficulty = "wrong" }},
		{"identity", func(r *Request) { r.Items[0].ClientID = "" }},
		{"command", func(r *Request) { r.CommandID = uuid.Nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Request{CommandID: uuid.New(), Items: []Item{item("a", "run")}}
			tc.edit(&r)
			if out := validate(english(), r, nil); out.Valid || len(out.Issues) == 0 {
				t.Fatalf("accepted invalid: %+v", out)
			}
		})
	}
}
func TestDuplicatePolicies(t *testing.T) {
	existing := []materialmodel.Material{{ID: uuid.New(), Values: map[string]*string{"foreign": ptr(" Run  fast ")}}}
	for _, policy := range []string{"skip", "conflict", "allow"} {
		t.Run(policy, func(t *testing.T) {
			r := Request{CommandID: uuid.New(), DuplicatePolicy: policy, Items: []Item{item("a", "run fast"), item("b", "walk"), item("c", "WALK")}}
			out := validate(english(), r, existing)
			if len(out.Items) != 3 {
				t.Fatal(out)
			}
			if policy == "skip" && (out.Created != 1 || out.Skipped != 2 || !out.Valid) {
				t.Fatal(out)
			}
			if policy == "conflict" && out.Valid {
				t.Fatal(out)
			}
			if policy == "allow" && (out.Created != 3 || out.Skipped != 0 || !out.Valid) {
				t.Fatal(out)
			}
		})
	}
}

func TestBoundsAndProfiles(t *testing.T) {
	r := Request{CommandID: uuid.New(), Items: make([]Item, MaxItems+1)}
	if validate(english(), r, nil).Valid {
		t.Fatal("accepted item overflow")
	}
	r.Items = []Item{item("a", "run")}
	r.Items[0].Values["native"] = ptr(strings.Repeat("я", 50001))
	if validate(english(), r, nil).Valid {
		t.Fatal("accepted byte overflow")
	}
	good := Item{Values: map[string]*string{"question": ptr("Why?"), "short_answer": ptr("Because.")}}
	p, err := buildProfile(good)
	if err != nil || p.Status != "ready" || len(p.Concepts) != 2 {
		t.Fatal(p, err)
	}
	good.Values["short_answer"] = ptr(".")
	p, err = buildProfile(good)
	if err != nil || p.Status != "draft" {
		t.Fatal(p, err)
	}
	bad := 11
	good.Profile = &ProfileInput{Frequency: &bad}
	if _, err = buildProfile(good); err == nil {
		t.Fatal("accepted invalid profile")
	}
}
func TestCommandHashBindsOperationFolderAndPayload(t *testing.T) {
	r := Request{CommandID: uuid.New(), Items: []Item{item("a", "run")}}
	folder := uuid.New()
	h := requestHash(folder, r)
	if h == requestHash(uuid.New(), r) {
		t.Fatal("folder not bound")
	}
	r.Items[0].Values["native"] = ptr("другое")
	if h == requestHash(folder, r) {
		t.Fatal("payload not bound")
	}
}
