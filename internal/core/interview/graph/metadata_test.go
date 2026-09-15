package graph

import (
	"encoding/json"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestMetadataAnswerPriorityAndHookFallback(t *testing.T) {
	s, cs, cat := graphFixture()
	s = started(s, cs[0])
	cs[0].Concepts = append(cs[0].Concepts, Link{Slug: "context_switch", Role: "answer", Weight: 1}, Link{Slug: "stack", Role: "hook", Weight: 2})
	for i := 0; i < 100; i++ {
		s.RandomSeed = int64(i)
		got := SelectMetadata(s, cs[0], "correct", cs, cat)
		if got.Candidate == nil || got.Candidate.MaterialID != cs[1].MaterialID || got.Matches[0].Source != "answer" {
			t.Fatal("hook displaced answer metadata", got)
		}
	}
	cs[1].Status = "archived"
	got := SelectMetadata(s, cs[0], "correct", cs, cat)
	if got.Candidate == nil || got.Candidate.MaterialID != cs[2].MaterialID || got.Matches[0].Source != "hook" {
		t.Fatal("missing hook fallback", got)
	}
}

func TestMetadataWrongPrefersSafeFallback(t *testing.T) {
	s, cs, cat := graphFixture()
	s = started(s, cs[0])
	s.CurrentDepth = 3
	cs[0].Specificity = 3
	cs[0].InterviewDifficulty = 3
	cs[0].Concepts = append(cs[0].Concepts, Link{Slug: "context_switch", Role: "wrong_fallback"}, Link{Slug: "stack", Role: "prerequisite"})
	cs[1].Specificity = 4
	cs[2].Specificity = 2
	cs[2].InterviewDifficulty = 2
	got := SelectMetadata(s, cs[0], "wrong", cs, cat)
	if got.Candidate == nil || got.Candidate.MaterialID != cs[2].MaterialID || got.State.CurrentDepth >= s.CurrentDepth || got.Matches[0].Source != "prerequisite" {
		t.Fatal("unsafe remediation", got)
	}
}

func TestMetadataFiltersAndDraftCredit(t *testing.T) {
	s, cs, _ := graphFixture()
	s.Config.Profile = "go_core"
	s.Config.Level = 3
	c := cs[0]
	c.Profiles = []string{"go_core", "backend_core"}
	c.LevelMin = 2
	c.LevelMax = 4
	if !Eligible(c, s) {
		t.Fatal("M:N membership not eligible")
	}
	s.Config.Profile = "backend_full"
	if Eligible(c, s) {
		t.Fatal("profile leaked")
	}
	s.Config.Profile = "all"
	s.Config.Level = 1
	if Eligible(c, s) {
		t.Fatal("level leaked")
	}
	s.Config.Level = 3
	c.Status = "draft"
	c.HasAnswer = false
	if Eligible(c, s) {
		t.Fatal("draft leaked")
	}
	s.Config.IncludeDraft = true
	got := SelectRoot(s, []Candidate{c})
	if got.Candidate == nil || got.ReviewCredit {
		t.Fatal("draft inaccessible or credited")
	}
	c.Status = "ready"
	c.HasAnswer = true
	got = SelectRoot(s, []Candidate{c})
	if got.ReviewCredit {
		t.Fatal("bank testing credited ready material")
	}
}

func TestNextRouteFrontierThenNewRoot(t *testing.T) {
	s, cs, cat := graphFixture()
	s = started(s, cs[0])
	cs[0].Concepts = append(cs[0].Concepts, Link{Slug: "context_switch", Role: "answer"})
	s.Frontier = []FrontierEntry{{MaterialID: cs[2].MaterialID, RootIndex: 1, Branch: "1.0", SourceDepth: 0, Status: "available", Score: 5}}
	got := SelectMetadata(s, cs[0], "next_route", cs, cat)
	if got.Candidate == nil || got.Candidate.MaterialID != cs[2].MaterialID || got.Reason != "frontier" || got.State.CurrentBranch == s.CurrentBranch {
		t.Fatal("did not switch branch", got)
	}
	s.Frontier = nil
	got = SelectMetadata(s, cs[0], "next_route", cs, cat)
	if got.Candidate == nil || got.State.RootsUsed != 2 || got.State.CurrentDepth != 0 || got.Reason != "root" {
		t.Fatal("did not open new root", got)
	}
}

func TestMetadataDeterminismAndQualityBounds(t *testing.T) {
	s, cs, cat := graphFixture()
	s = started(s, cs[0])
	cs[0].Concepts = append(cs[0].Concepts, Link{Slug: "context_switch", Role: "answer"}, Link{Slug: "stack", Role: "answer", Ordinal: 1})
	a := SelectMetadata(s, cs[0], "correct", cs, cat)
	raw, _ := json.Marshal(s)
	var resumed State
	if err := json.Unmarshal(raw, &resumed); err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(cs)-1; i < j; i, j = i+1, j-1 {
		cs[i], cs[j] = cs[j], cs[i]
	}
	b := SelectMetadata(resumed, cs[len(cs)-1], "correct", cs, cat)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("resume or candidate order changed RNG result")
	}
	scores := []Score{}
	for i := 0; i < 20; i++ {
		scores = append(scores, Score{MaterialID: uuid.New(), Score: 10 - float64(i)})
	}
	filtered := qualityCandidates(scores)
	if len(filtered) != 2 {
		t.Fatal("quality floor", filtered)
	}
	for i := range scores {
		scores[i].Score = 10
	}
	if len(qualityCandidates(scores)) != 5 {
		t.Fatal("top-K not bounded")
	}
	s.Frontier = nil
	for i := 0; i < 25; i++ {
		s.Frontier = append(s.Frontier, FrontierEntry{MaterialID: uuid.New(), RootIndex: 1, Status: "available", Score: float64(i)})
	}
	boundFrontier(&s)
	if len(s.Frontier) != 12 || s.Frontier[0].Score != 24 || s.Frontier[11].Score != 13 {
		t.Fatal("frontier discarded strongest entries", s.Frontier)
	}
}
