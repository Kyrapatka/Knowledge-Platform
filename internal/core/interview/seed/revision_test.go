package seed

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAuthoritativeRevision(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	r := b.Report
	if r.Questions != 368 || r.Concepts != 1470 || r.Aliases != 2825 || r.ScopedAliases != 42 || r.AmbiguousAliases != 17 || r.ConceptsWithoutAlias != 5 {
		t.Fatalf("unexpected report: %+v", r)
	}
	if !reflect.DeepEqual(r.CountsByPrefix, prefixCounts) {
		t.Fatal(r.CountsByPrefix)
	}
	if r.QuestionsSHA256 != "d22685b9967cf4ff3bdc25329824894d17f42105974b1dcf991f7973d22106ac" || r.ConceptsSHA256 != "e7418e6c131e5025c9801d25b64d1162894865813ffb5f6853a388579190b414" {
		t.Fatal("embedded bytes differ from supplied corrected revision")
	}
	for _, q := range b.Questions {
		if q.Question == "." || q.ShortAnswer != "." || q.FullAnswer != "." || q.Source != "." || q.Status != "draft" {
			t.Fatal("seed content changed", q.SeedKey)
		}
	}
	for _, a := range b.AmbiguousAliases {
		if _, ok := b.AliasMap[a.NormalizedAlias]; ok {
			t.Fatal("ambiguous alias made global", a)
		}
	}
}

func TestRejectBrokenSeedBeforeImport(t *testing.T) {
	cases := map[string]func(map[string]any){
		"duplicate key": func(q map[string]any) {
			rows := q["questions"].([]any)
			rows[1].(map[string]any)["seed_key"] = rows[0].(map[string]any)["seed_key"]
		},
		"placeholder question": func(q map[string]any) { q["questions"].([]any)[0].(map[string]any)["question"] = "." },
		"missing answer field": func(q map[string]any) { delete(q["questions"].([]any)[0].(map[string]any), "full_answer") },
		"unknown concept": func(q map[string]any) {
			q["questions"].([]any)[0].(map[string]any)["answer_concepts"] = []string{"not_in_dictionary"}
		},
		"invalid frequency": func(q map[string]any) { q["questions"].([]any)[0].(map[string]any)["frequency"] = 11 },
		"reversed levels": func(q map[string]any) {
			r := q["questions"].([]any)[0].(map[string]any)
			r["level_min"] = 5
			r["level_max"] = 1
		},
		"unknown profile": func(q map[string]any) {
			q["questions"].([]any)[0].(map[string]any)["interview_profiles"] = []string{"invented"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			raw, concepts := RawFiles()
			var q map[string]any
			if err := json.Unmarshal(raw, &q); err != nil {
				t.Fatal(err)
			}
			mutate(q)
			raw, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Parse(raw, concepts); err == nil {
				t.Fatal("accepted corrupt input")
			}
		})
	}
}
